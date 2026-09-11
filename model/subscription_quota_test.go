package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newQuotaTestUser creates the smallest valid user row needed by the
// subscription issuance/pre-consume paths.  Keep this helper local to the
// quota tests so it does not alter the fixtures used by other model tests.
func newQuotaTestUser(t *testing.T) *User {
	t.Helper()
	user := &User{
		Username:   fmt.Sprintf("quota%d", time.Now().UnixNano()%1_000_000_000_000),
		Password:   "password123",
		ProviderId: 0,
		Group:      "default",
		Status:     1,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestSubscriptionWeeklyQuotaReset(t *testing.T) {
	truncateTables(t)

	// Use a deterministic Monday in UTC.  The reset implementation aligns a
	// weekly period to the next Monday at 00:00 in the base time's location.
	base := time.Date(2025, time.January, 6, 12, 0, 0, 0, time.UTC)
	plan := &SubscriptionPlan{
		ProviderId:       0,
		Title:            "weekly quota plan",
		Enabled:          true,
		AllowPurchase:    1,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      100,
		QuotaResetPeriod: SubscriptionResetWeekly,
	}
	require.NoError(t, DB.Create(plan).Error)

	user := newQuotaTestUser(t)
	firstReset := calcNextResetTime(base, plan, base.AddDate(0, 1, 0).Unix())
	require.Greater(t, firstReset, base.Unix())
	sub := &UserSubscription{
		UserId:        user.Id,
		PlanId:        plan.Id,
		ProviderId:    0,
		AmountTotal:   100,
		AmountUsed:    73,
		StartTime:     base.Unix(),
		EndTime:       base.AddDate(0, 1, 0).Unix(),
		Status:        "active",
		LastResetTime: base.Unix(),
		NextResetTime: firstReset,
	}
	require.NoError(t, DB.Create(sub).Error)

	// Drive the same transaction helper used by the periodic reset task with a
	// time just after the due boundary.  This avoids relying on wall-clock time
	// while still exercising the persisted reset state.
	now := firstReset + 1
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var locked UserSubscription
		if err := tx.Where("id = ?", sub.Id).First(&locked).Error; err != nil {
			return err
		}
		return maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now)
	}))

	var got UserSubscription
	require.NoError(t, DB.First(&got, sub.Id).Error)
	require.Equal(t, int64(0), got.AmountUsed, "weekly usage must be reset once the boundary passes")
	// The boundary is represented as a Unix timestamp; its wall-clock
	// rendering follows the server/database location.  Assert progression
	// rather than a location-specific literal so this remains valid across
	// deployments configured for UTC or another local zone.
	require.Greater(t, got.LastResetTime, sub.LastResetTime)
	require.LessOrEqual(t, got.LastResetTime, now)
	require.Greater(t, got.NextResetTime, now)

	// A second invocation in the same period must be a no-op.
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return maybeResetUserSubscriptionWithPlanTx(tx, &got, plan, now)
	}))
	var unchanged UserSubscription
	require.NoError(t, DB.First(&unchanged, sub.Id).Error)
	require.Equal(t, got.NextResetTime, unchanged.NextResetTime)
	require.Equal(t, int64(0), unchanged.AmountUsed)
}

func TestSubscriptionPlanQuotaPolicySnapshot(t *testing.T) {
	truncateTables(t)

	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId:            0,
		Title:                 "snapshotted dual quota plan",
		Enabled:               true,
		AllowPurchase:         1,
		DurationUnit:          SubscriptionDurationMonth,
		DurationValue:         1,
		TotalAmount:           1000,
		ModelLimits:           "gpt-test",
		QuotaWindowMode:       SubscriptionQuotaWindowDual,
		FiveHourAmount:        50,
		FiveHourWindowSeconds: 3600,
		WeeklyAmount:          200,
		QuotaResetPeriod:      SubscriptionResetWeekly,
	}
	require.NoError(t, DB.Create(plan).Error)

	var issued *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		issued, err = CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "quota-test")
		return err
	}))
	require.NotNil(t, issued)
	require.Equal(t, SubscriptionQuotaWindowDual, issued.QuotaWindowMode)
	require.Equal(t, int64(50), issued.FiveHourAmount)
	require.Equal(t, int64(3600), issued.FiveHourWindowSeconds)
	require.Equal(t, int64(200), issued.WeeklyAmount)
	require.Equal(t, SubscriptionResetWeekly, issued.QuotaResetPeriodSnapshot)

	// Editing a plan changes only future issuances.  Existing subscriptions
	// retain the limits copied at issuance time.
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"quota_window_mode":        SubscriptionQuotaWindowFiveHour,
		"five_hour_amount":         5,
		"five_hour_window_seconds": 120,
		"weekly_amount":            900,
		"quota_reset_period":       SubscriptionResetMonthly,
	}).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, issued.Id).Error)
	require.Equal(t, SubscriptionQuotaWindowDual, reloaded.QuotaWindowMode)
	require.Equal(t, int64(50), reloaded.FiveHourAmount)
	require.Equal(t, int64(3600), reloaded.FiveHourWindowSeconds)
	require.Equal(t, int64(200), reloaded.WeeklyAmount)
	require.Equal(t, SubscriptionResetWeekly, reloaded.QuotaResetPeriodSnapshot)

	// The snapshot is enforced, not merely displayed: consume almost all of
	// the original weekly allowance and verify the old limit still applies even
	// though the current plan advertises a larger allowance.
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", issued.Id).
		Update("amount_used", int64(190)).Error)
	_, err := PreConsumeUserSubscription("quota-snapshot-limit", user.Id, "gpt-test", 0, 20)
	require.Error(t, err)
	var after UserSubscription
	require.NoError(t, DB.First(&after, issued.Id).Error)
	require.Equal(t, int64(190), after.AmountUsed)
}

func TestLegacySubscriptionQuotaCompatibility(t *testing.T) {
	truncateTables(t)

	user := newQuotaTestUser(t)
	// This plan intentionally leaves all quota-window columns empty, matching
	// a row created before the new fields existed.  TotalAmount remains the
	// original aggregate quota.
	plan := &SubscriptionPlan{
		ProviderId:       0,
		Title:            "legacy quota plan",
		Enabled:          true,
		AllowPurchase:    1,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      100,
		ModelLimits:      "gpt-test",
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)

	now := time.Now().Unix()
	sub := &UserSubscription{
		UserId:      user.Id,
		PlanId:      plan.Id,
		ProviderId:  0,
		AmountTotal: 100,
		AmountUsed:  90,
		StartTime:   now - 60,
		EndTime:     now + 3600,
		Status:      "active",
	}
	require.NoError(t, DB.Create(sub).Error)

	// Simulate the values an old row would have after the additive migration.
	// Empty strings are accepted by the new columns and exercise the fallback
	// to TotalAmount + QuotaResetPeriod.
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).
		Update("quota_window_mode", "").Error)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", sub.Id).
		Updates(map[string]interface{}{
			"quota_window_mode":           "",
			"five_hour_amount":            0,
			"five_hour_window_seconds":    0,
			"weekly_amount":               0,
			"quota_reset_period_snapshot": "",
		}).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	res, err := PreConsumeUserSubscription("legacy-quota-request", user.Id, "gpt-test", 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(100), res.AmountUsedAfter)
	_, err = PreConsumeUserSubscription("legacy-quota-over-limit", user.Id, "gpt-test", 0, 1)
	require.Error(t, err)

	// The legacy post-consume delta path remains available for callers that do
	// not carry a request id (and still obeys the original total allowance).
	require.NoError(t, PostConsumeUserSubscriptionDelta(sub.Id, -20))
	var got UserSubscription
	require.NoError(t, DB.First(&got, sub.Id).Error)
	require.Equal(t, int64(80), got.AmountUsed)
}
