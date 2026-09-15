package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newQuotaTestUser creates the smallest valid user row needed by the
// subscription issuance/pre-consume paths.  Keep this helper local to the
// quota tests so it does not alter the fixtures used by other model tests.
func newQuotaTestUser(t *testing.T) *User {
	t.Helper()
	unique := time.Now().UnixNano()
	user := &User{
		Username:   fmt.Sprintf("quota%d", unique%1_000_000_000_000),
		Password:   "password123",
		ProviderId: 0,
		Group:      "default",
		Status:     1,
		AffCode:    fmt.Sprintf("q%x", unique),
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

func TestPopulateSubscriptionUsagePreservesFinalPartialCalendarPeriod(t *testing.T) {
	// A subscription issued in the middle of a calendar bucket can end before
	// the next reset boundary.  In that case next_reset_time is intentionally
	// persisted as zero (there is no future reset while the subscription is
	// active), but the aggregate amount_used still belongs to the current
	// period and must remain visible to the API.
	start := time.Date(2025, time.January, 15, 12, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 10) // before the February monthly boundary
	now := start.AddDate(0, 0, 3)
	plan := &SubscriptionPlan{
		Title:            "short monthly-reset plan",
		Enabled:          true,
		AllowPurchase:    1,
		DurationUnit:     SubscriptionDurationDay,
		DurationValue:    10,
		TotalAmount:      100,
		QuotaResetPeriod: SubscriptionResetMonthly,
	}
	sub := &UserSubscription{
		AmountTotal:   100,
		AmountUsed:    37,
		StartTime:     start.Unix(),
		EndTime:       end.Unix(),
		Status:        "active",
		NextResetTime: 0,
	}

	require.NoError(t, PopulateSubscriptionUsage(sub, plan, now.Unix()))
	require.Equal(t, int64(37), sub.WeeklyUsed)
	require.Equal(t, int64(63), sub.WeeklyRemaining)
	require.Equal(t, int64(0), sub.WeeklyResetAt)
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
	require.Equal(t, 1, issued.PlanPolicySnapshotVersion)
	require.Equal(t, "gpt-test", issued.ModelLimitsSnapshot)

	// Editing a plan changes only future issuances.  Existing subscriptions
	// retain the limits copied at issuance time.
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"quota_window_mode":        SubscriptionQuotaWindowFiveHour,
		"five_hour_amount":         5,
		"five_hour_window_seconds": 120,
		"weekly_amount":            900,
		"quota_reset_period":       SubscriptionResetMonthly,
		"model_limits":             "other-model",
	}).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, issued.Id).Error)
	require.Equal(t, SubscriptionQuotaWindowDual, reloaded.QuotaWindowMode)
	require.Equal(t, int64(50), reloaded.FiveHourAmount)
	require.Equal(t, int64(3600), reloaded.FiveHourWindowSeconds)
	require.Equal(t, int64(200), reloaded.WeeklyAmount)
	require.Equal(t, SubscriptionResetWeekly, reloaded.QuotaResetPeriodSnapshot)
	summaries, err := GetAllActiveUserSubscriptions(user.Id)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.NotNil(t, summaries[0].Plan)
	require.Equal(t, "snapshotted dual quota plan", summaries[0].Plan.Title)
	require.Equal(t, "gpt-test", summaries[0].Plan.ModelLimits)
	getSubscriptionPlanInfoCache().Purge()
	planInfo, err := GetSubscriptionPlanInfoByUserSubscriptionId(issued.Id)
	require.NoError(t, err)
	require.Equal(t, "snapshotted dual quota plan", planInfo.PlanTitle,
		"issued subscription title must not follow later catalog edits")

	// Model eligibility is part of the issued policy as well. The old model
	// remains available, while a model added to the source plan later does not
	// become available to an existing subscription.
	_, err = PreConsumeUserSubscription("quota-snapshot-model", user.Id, "gpt-test", 0, 1)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("quota-snapshot-new-model", user.Id, "other-model", 0, 1)
	require.Error(t, err)

	// The snapshot is enforced, not merely displayed: consume almost all of
	// the original weekly allowance and verify the old limit still applies even
	// though the current plan advertises a larger allowance.
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", issued.Id).
		Update("amount_used", int64(190)).Error)
	_, err = PreConsumeUserSubscription("quota-snapshot-limit", user.Id, "gpt-test", 0, 20)
	require.Error(t, err)
	var after UserSubscription
	require.NoError(t, DB.First(&after, issued.Id).Error)
	require.Equal(t, int64(190), after.AmountUsed)
}

// A hard-deleted subscription must not leave an in-flight pre-consume record
// permanently stuck in the consumed state.  Refund is still idempotent even
// though there is no aggregate row left from which to subtract usage.
func TestRefundSubscriptionPreConsumeAfterSubscriptionHardDelete(t *testing.T) {
	truncateTables(t)

	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId:       0,
		Title:            "hard-delete refund plan",
		Enabled:          true,
		AllowPurchase:    1,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      100,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)

	var sub *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		sub, err = CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "quota-test")
		return err
	}))
	require.NotNil(t, sub)

	requestID := fmt.Sprintf("hard-delete-refund-%d", time.Now().UnixNano())
	_, err := PreConsumeUserSubscription(requestID, user.Id, "gpt-test", 0, 10)
	require.NoError(t, err)

	// Simulate the administrator hard-delete path.  Keep the pre-consume row
	// so the refund operation can exercise the missing-subscription branch.
	require.NoError(t, DB.Where("id = ?", sub.Id).Delete(&UserSubscription{}).Error)
	require.NoError(t, RefundSubscriptionPreConsume(requestID))
	// Repeating the refund must remain a no-op.
	require.NoError(t, RefundSubscriptionPreConsume(requestID))

	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", requestID).First(&record).Error)
	require.Equal(t, "refunded", record.Status)
	require.NotNil(t, record.SettledAmount)
	require.Equal(t, int64(0), *record.SettledAmount)
}

func TestLegacySubscriptionSnapshotDoesNotInheritEditedPlanWindows(t *testing.T) {
	truncateTables(t)
	getSubscriptionPlanInfoCache().Purge()
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId: 0, Title: "legacy snapshot", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		TotalAmount: 100, QuotaResetPeriod: SubscriptionResetNever,
		ModelLimits: "gpt-legacy",
	}
	sub := createQuotaTestSubscription(t, user, plan)
	require.Equal(t, 1, sub.PlanPolicySnapshotVersion)
	require.Equal(t, SubscriptionQuotaWindowLegacy, sub.QuotaWindowMode)

	// Mutate the catalog after issuance.  The issued legacy subscription must
	// retain its aggregate, never-reset policy rather than inheriting a newly
	// configured dual window.
	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]interface{}{
		"quota_window_mode":        SubscriptionQuotaWindowDual,
		"five_hour_amount":         10,
		"five_hour_window_seconds": 3600,
		"weekly_amount":            10,
		"quota_reset_period":       SubscriptionResetWeekly,
	}).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	// Consume the full legacy allowance in two requests.  If the edited plan
	// leaked into the snapshot, the second request would be rejected by the new
	// five-hour/weekly limits instead of the original 100-unit aggregate.
	_, err := PreConsumeUserSubscription("legacy-snapshot-first", user.Id, "gpt-legacy", 0, 60)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("legacy-snapshot-second", user.Id, "gpt-legacy", 0, 40)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("legacy-snapshot-over", user.Id, "gpt-legacy", 0, 1)
	require.Error(t, err)
	summaries, err := GetAllActiveUserSubscriptions(user.Id)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.NotNil(t, summaries[0].Plan)
	require.Equal(t, SubscriptionResetNever, summaries[0].Plan.QuotaResetPeriod,
		"subscription summary must preserve the snapshotted reset policy")
}

func TestSubscriptionPolicySnapshotSurvivesPlanDeletion(t *testing.T) {
	truncateTables(t)
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId: 0, Title: "deleted source plan", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		ModelLimits: "gpt-snapshot",
		QuotaWindows: SubscriptionQuotaWindowList{{
			Type: SubscriptionQuotaWindowDay, Amount: 100, Duration: 1,
			ResetMode: SubscriptionQuotaWindowCalendar,
		}},
	}
	sub := createQuotaTestSubscription(t, user, plan)
	require.Equal(t, 1, sub.PlanPolicySnapshotVersion)
	require.NoError(t, DB.Delete(&SubscriptionPlan{}, plan.Id).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	_, err := PreConsumeUserSubscription("deleted-plan-allowed", user.Id, "gpt-snapshot", 0, 60)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("deleted-plan-blocked-model", user.Id, "other-model", 0, 1)
	require.Error(t, err)
	_, err = PreConsumeUserSubscription("deleted-plan-over-window", user.Id, "gpt-snapshot", 0, 41)
	require.Error(t, err)
}

func TestSubscriptionPreConsumeRetryValidatesRequestShape(t *testing.T) {
	truncateTables(t)
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId: 0, Title: "idempotent subscription", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		TotalAmount: 100, ModelLimits: "gpt-test",
	}
	sub := createQuotaTestSubscription(t, user, plan)

	first, err := PreConsumeUserSubscription("subscription-idempotent", user.Id, "gpt-test", 7, 10)
	require.NoError(t, err)
	require.Equal(t, sub.Id, first.UserSubscriptionId)

	retry, err := PreConsumeUserSubscription("subscription-idempotent", user.Id, "gpt-test", 7, 10)
	require.NoError(t, err)
	require.Equal(t, sub.Id, retry.UserSubscriptionId)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, sub.Id).Error)
	require.Equal(t, int64(10), reloaded.AmountUsed, "an idempotent retry must not consume twice")

	_, err = PreConsumeUserSubscription("subscription-idempotent", user.Id, "gpt-test", 7, 11)
	require.Error(t, err)
	_, err = PreConsumeUserSubscription("subscription-idempotent", user.Id, "other-model", 7, 10)
	require.Error(t, err)
	_, err = PreConsumeUserSubscription("subscription-idempotent", user.Id, "gpt-test", 8, 10)
	require.Error(t, err)
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

func TestSubscriptionGenericQuotaWindowEnforcedAndSettled(t *testing.T) {
	truncateTables(t)
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId:    0,
		Title:         "generic rolling plan",
		Enabled:       true,
		AllowPurchase: 1,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		QuotaWindows: SubscriptionQuotaWindowList{{
			Type:          SubscriptionQuotaWindowCustom,
			Amount:        100,
			WindowSeconds: 3600,
			ResetMode:     SubscriptionQuotaWindowRolling,
		}},
	}
	require.NoError(t, DB.Create(plan).Error)
	var sub *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		sub, err = CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "quota-test")
		return err
	}))
	require.Equal(t, SubscriptionQuotaWindowGeneric, sub.QuotaWindowMode)
	require.Len(t, sub.QuotaWindowsSnapshot, 1)

	first, err := PreConsumeUserSubscription("generic-window-1", user.Id, "", 0, 60)
	require.NoError(t, err)
	require.Equal(t, int64(0), first.AmountUsedAfter, "generic windows derive usage from request records")
	_, err = PreConsumeUserSubscription("generic-window-too-much", user.Id, "", 0, 41)
	require.Error(t, err)

	// Settling below the pre-consumed amount updates the request record and
	// immediately frees the difference in the rolling window.
	require.NoError(t, SettleSubscriptionPreConsume("generic-window-1", 40))
	second, err := PreConsumeUserSubscription("generic-window-2", user.Id, "", 0, 60)
	require.NoError(t, err)
	require.Equal(t, int64(60), second.PreConsumed)

	require.NoError(t, RefundSubscriptionPreConsume("generic-window-1"))
	_, err = PreConsumeUserSubscription("generic-window-3", user.Id, "", 0, 40)
	require.NoError(t, err)
}

func TestMalformedGenericQuotaFailsClosed(t *testing.T) {
	truncateTables(t)
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId:      0,
		Title:           "malformed generic plan",
		Enabled:         true,
		AllowPurchase:   1,
		DurationUnit:    SubscriptionDurationMonth,
		DurationValue:   1,
		TotalAmount:     0,
		QuotaWindowMode: SubscriptionQuotaWindowGeneric,
	}
	require.NoError(t, DB.Create(plan).Error)

	// A malformed generic plan must not be issuable. In particular, zero
	// TotalAmount must never turn the missing window list into unlimited usage.
	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "quota-test")
		return err
	})
	require.ErrorContains(t, err, "invalid subscription quota policy")

	// Existing rows with the same corrupted policy also fail closed at request
	// admission instead of bypassing the quota check.
	sub := &UserSubscription{
		UserId: user.Id, PlanId: plan.Id, ProviderId: 0,
		StartTime: time.Now().Unix() - 60, EndTime: time.Now().Unix() + 3600,
		Status: "active", QuotaWindowMode: SubscriptionQuotaWindowGeneric,
		QuotaWindowModeSnapshot: SubscriptionQuotaWindowGeneric,
	}
	require.NoError(t, DB.Create(sub).Error)
	_, err = PreConsumeUserSubscription("malformed-generic-request", user.Id, "", 0, 1)
	require.ErrorContains(t, err, "invalid subscription quota policy")
}

// Normalization is used by caches and API summaries and historically dropped
// malformed entries.  The raw evidence must survive those passes so a corrupt
// generic policy cannot silently become an unlimited plan.
func TestMalformedQuotaWindowRemainsInvalidAfterNormalization(t *testing.T) {
	plan := &SubscriptionPlan{
		QuotaWindowMode: SubscriptionQuotaWindowGeneric,
		QuotaWindows: SubscriptionQuotaWindowList{
			{Type: SubscriptionQuotaWindowDay, Amount: 100, WindowSeconds: 1}, // invalid: day must use its nominal duration
			{Type: SubscriptionQuotaWindowCustom, Amount: 50, WindowSeconds: 60},
		},
	}
	plan.NormalizeQuotaWindows()
	// Keep the raw list, including the malformed entry.  Dropping it here
	// would lose the only durable corruption signal before a cache round-trip.
	require.Len(t, plan.QuotaWindows, 2)
	require.Equal(t, int64(1), plan.QuotaWindows[0].WindowSeconds)
	policy := quotaPolicyFromPlan(plan)
	require.True(t, policy.invalid)

	// The plan cache uses JSON serialization, which intentionally omits the
	// in-memory quotaWindowsInvalid marker.  The raw malformed entry must remain
	// sufficient for the decoded plan to fail closed on every subsequent read.
	codec := cachex.JSONCodec[SubscriptionPlan]{}
	raw, err := codec.Encode(*plan)
	require.NoError(t, err)
	roundTripped, err := codec.Decode(raw)
	require.NoError(t, err)
	require.False(t, roundTripped.quotaWindowsInvalid, "cache JSON intentionally omits the in-memory marker")
	roundTripped.NormalizeQuotaWindows()
	require.True(t, roundTripped.quotaWindowsInvalid)
	require.Len(t, roundTripped.QuotaWindows, 2)
	roundTrippedPolicy := quotaPolicyFromPlan(&roundTripped)
	require.True(t, roundTrippedPolicy.invalid)

	// A second normalization pass must not turn the object into a valid or
	// legacy/unlimited policy either.
	roundTripped.NormalizeQuotaWindows()
	require.True(t, quotaPolicyFromPlan(&roundTripped).invalid)
}

func TestSubscriptionGenericCalendarDailyMonthlyYearlyLimits(t *testing.T) {
	truncateTables(t)
	getSubscriptionPlanInfoCache().Purge()

	tests := []struct {
		name string
		unit string
	}{
		{name: "daily", unit: SubscriptionQuotaWindowDay},
		{name: "monthly", unit: SubscriptionQuotaWindowMonth},
		{name: "yearly", unit: SubscriptionQuotaWindowYear},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user := newQuotaTestUser(t)
			plan := &SubscriptionPlan{
				ProviderId: 0, Title: tc.name + " calendar plan", Enabled: true, AllowPurchase: 1,
				DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
				QuotaWindows: SubscriptionQuotaWindowList{{
					Type: tc.unit, Amount: 100, Duration: 1,
					ResetMode: SubscriptionQuotaWindowCalendar,
				}},
			}
			sub := createQuotaTestSubscription(t, user, plan)
			createdAt := time.Now().UTC().Unix() - 30
			createQuotaRecordAt(t, sub, tc.name+"-existing", 90, createdAt)

			result, err := PreConsumeUserSubscription(tc.name+"-over-limit", user.Id, "", 0, 11)
			require.Error(t, err, "%s quota must reject a request over the current calendar bucket", tc.name)
			require.Nil(t, result)

			// A smaller request is admitted and the returned generic-window
			// snapshot must report actual usage, not the configured limit.
			result, err = PreConsumeUserSubscription(tc.name+"-within-limit", user.Id, "", 0, 10)
			require.NoError(t, err)
			require.Len(t, result.QuotaWindows, 1)
			require.Equal(t, int64(100), result.QuotaWindows[0].Limit)
			require.Equal(t, int64(100), result.QuotaWindows[0].Used)
			require.Equal(t, int64(0), result.QuotaWindows[0].Remaining)
		})
	}
}

func TestSubscriptionCalendarMonthAndYearUseActualIntervalLength(t *testing.T) {
	month := subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowMonth, duration: 1, windowSeconds: 30 * 24 * 60 * 60,
		resetMode: SubscriptionQuotaWindowCalendar,
	}
	february := time.Date(2024, time.February, 15, 12, 0, 0, 0, time.UTC).Unix()
	require.Equal(t, int64(29*24*60*60), quotaWindowEffectiveSeconds(month, february, 0))

	year := subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowYear, duration: 1, windowSeconds: 365 * 24 * 60 * 60,
		resetMode: SubscriptionQuotaWindowCalendar,
	}
	leapYear := time.Date(2024, time.July, 1, 12, 0, 0, 0, time.UTC).Unix()
	require.Equal(t, int64(366*24*60*60), quotaWindowEffectiveSeconds(year, leapYear, 0))
}

func TestSubscriptionPlanSnapshotSummarySurvivesDeletion(t *testing.T) {
	truncateTables(t)
	getSubscriptionPlanInfoCache().Purge()
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId: 0, Title: "snapshot summary plan", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		ModelLimits: "gpt-snapshot",
		QuotaWindows: SubscriptionQuotaWindowList{{
			Type: SubscriptionQuotaWindowDay, Amount: 100, Duration: 1,
			ResetMode: SubscriptionQuotaWindowCalendar,
		}},
	}
	sub := createQuotaTestSubscription(t, user, plan)
	require.NoError(t, DB.Delete(&SubscriptionPlan{}, plan.Id).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	summaries, err := GetAllActiveUserSubscriptions(user.Id)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.NotNil(t, summaries[0].Plan)
	// The issued title is part of the subscription snapshot and remains
	// customer-facing even after the catalog row is deleted.
	require.Equal(t, "snapshot summary plan", summaries[0].Plan.Title)
	require.Equal(t, "gpt-snapshot", summaries[0].Plan.ModelLimits)

	info, err := GetSubscriptionPlanInfoByUserSubscriptionId(sub.Id)
	require.NoError(t, err)
	require.Equal(t, sub.PlanId, info.PlanId)
	require.Equal(t, "snapshot summary plan", info.PlanTitle)
}

func createQuotaTestSubscription(t *testing.T, user *User, plan *SubscriptionPlan) *UserSubscription {
	t.Helper()
	require.NoError(t, DB.Create(plan).Error)
	var sub *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		sub, err = CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "quota-test")
		return err
	}))
	require.NotNil(t, sub)
	return sub
}

func createQuotaRecordAt(t *testing.T, sub *UserSubscription, requestID string, amount, createdAt int64) *SubscriptionPreConsumeRecord {
	t.Helper()
	settled := amount
	record := &SubscriptionPreConsumeRecord{
		RequestId: requestID, UserId: sub.UserId, UserSubscriptionId: sub.Id,
		PreConsumed: amount, SettledAmount: &settled, SettledAt: createdAt,
		Status: "consumed",
	}
	require.NoError(t, DB.Create(record).Error)
	require.NoError(t, DB.Model(record).UpdateColumns(map[string]interface{}{
		"created_at": createdAt,
		"updated_at": createdAt,
		"settled_at": createdAt,
	}).Error)
	record.CreatedAt = createdAt
	record.UpdatedAt = createdAt
	return record
}

func TestSubscriptionQuotaWindowNormalizeAndCalendarBounds(t *testing.T) {
	normalized, ok := (SubscriptionQuotaWindow{
		Type: SubscriptionQuotaWindowHour, Amount: 100, WindowSeconds: DefaultFiveHourWindowSeconds,
	}).Normalize()
	require.True(t, ok)
	require.Equal(t, int64(5), normalized.Duration)

	_, ok = (SubscriptionQuotaWindow{
		Type: SubscriptionQuotaWindowHour, Amount: 100, Duration: 2, WindowSeconds: DefaultFiveHourWindowSeconds,
	}).Normalize()
	require.False(t, ok, "conflicting duration aliases must be rejected")

	_, ok = (SubscriptionQuotaWindow{
		Type: SubscriptionQuotaWindowCustom, Amount: 100, WindowSeconds: MaxSubscriptionQuotaWindowSeconds + 1,
	}).Normalize()
	require.False(t, ok, "oversized windows must be rejected")

	week := subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowWeek, duration: 1, windowSeconds: 7 * 24 * 60 * 60,
		resetMode: SubscriptionQuotaWindowCalendar,
	}
	wednesday := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC).Unix()
	start, end := quotaWindowBounds(week, wednesday, 0)
	require.Equal(t, time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC).Unix(), start)
	require.Equal(t, time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC).Unix(), end)

	quarter := subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowMonth, duration: 3, windowSeconds: 90 * 24 * 60 * 60,
		resetMode: SubscriptionQuotaWindowCalendar,
	}
	may := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC).Unix()
	start, end = quotaWindowBounds(quarter, may, 0)
	require.Equal(t, time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC).Unix(), start)
	require.Equal(t, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC).Unix(), end)

	twoYears := subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowYear, duration: 2, windowSeconds: 2 * 365 * 24 * 60 * 60,
		resetMode: SubscriptionQuotaWindowCalendar,
	}
	mid2027 := time.Date(2027, time.June, 1, 0, 0, 0, 0, time.UTC).Unix()
	start, end = quotaWindowBounds(twoYears, mid2027, 0)
	require.Equal(t, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Unix(), start)
	require.Equal(t, time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC).Unix(), end)

	rolling := subscriptionQuotaWindowPolicy{windowSeconds: 100, resetMode: SubscriptionQuotaWindowRolling}
	require.False(t, quotaWindowContainsRecord(rolling, 10_000, 0, 9_900), "rolling lower boundary is open")
	require.True(t, quotaWindowContainsRecord(rolling, 10_000, 0, 9_901))
}

func TestSubscriptionQuotaWindowArithmeticGuardsOverflow(t *testing.T) {
	const maxInt64 = int64(^uint64(0) >> 1)
	const minInt64 = -maxInt64 - 1

	if got, ok := subscriptionSafeAddInt64(maxInt64, 1); ok || got != 0 {
		t.Fatalf("expected overflowing add to be rejected, got %d (ok=%v)", got, ok)
	}
	if got, ok := subscriptionSafeAddInt64(minInt64, -1); ok || got != 0 {
		t.Fatalf("expected underflowing add to be rejected, got %d (ok=%v)", got, ok)
	}
	if got, ok := subscriptionSafeMulInt64(maxInt64, 2); ok || got != 0 {
		t.Fatalf("expected overflowing multiplication to be rejected, got %d (ok=%v)", got, ok)
	}
	if got, ok := subscriptionSafeMulInt64(minInt64, -1); ok || got != 0 {
		t.Fatalf("expected min-int multiplication to be rejected, got %d (ok=%v)", got, ok)
	}

	// A malformed persisted policy can contain a huge duration even though the
	// public normalizer rejects it. Bounds resolution must remain deterministic
	// and must not wrap a period into a negative divisor or panic in time.Date.
	now := int64(1_900_000_000)
	malformed := subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowHour, duration: maxInt64,
		windowSeconds: 3600, resetMode: SubscriptionQuotaWindowCalendar,
	}
	start, end := quotaWindowBounds(malformed, now, 0)
	require.Equal(t, now, start)
	require.Equal(t, now, end)

	// Extreme timestamps and anchors must not wrap while calculating rolling or
	// custom boundaries. The upper endpoint is saturated at MaxInt64.
	rolling := subscriptionQuotaWindowPolicy{
		windowSeconds: 100, resetMode: SubscriptionQuotaWindowRolling,
	}
	start, end = quotaWindowBounds(rolling, maxInt64, 0)
	require.Equal(t, maxInt64-100, start)
	require.Equal(t, maxInt64, end)

	custom := subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowCustom, windowSeconds: 100,
		duration: 1, resetMode: SubscriptionQuotaWindowCalendar,
	}
	// Keep calendar calculations within time.Time's practical range;
	// Unix MaxInt64 is not representable as a normal calendar year and can
	// make time.Unix/AddDate extremely expensive on some platforms.
	nearTimeLimit := time.Date(9990, time.December, 31, 23, 59, 0, 0, time.UTC).Unix()
	start, end = quotaWindowBounds(custom, nearTimeLimit, nearTimeLimit-50)
	require.Equal(t, nearTimeLimit-50, start)
	require.Equal(t, nearTimeLimit+50, end)

	// Calendar calculations should also fail closed for timestamps near the
	// int64 boundary instead of allowing time.Date/AddDate to normalize into a
	// wrapped interval.
	calendar := subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowYear, duration: 1,
		windowSeconds: 365 * 24 * 60 * 60,
		resetMode:     SubscriptionQuotaWindowCalendar,
	}
	start, end = quotaWindowBounds(calendar, nearTimeLimit, 0)
	require.LessOrEqual(t, start, end)
}

func TestSubscriptionCalendarSettlementChecksHistoricalBucket(t *testing.T) {
	truncateTables(t)
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId: 0, Title: "historical calendar settlement", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		QuotaWindows: SubscriptionQuotaWindowList{{
			Type: SubscriptionQuotaWindowDay, Amount: 100, Duration: 1,
			ResetMode: SubscriptionQuotaWindowCalendar,
		}},
	}
	sub := createQuotaTestSubscription(t, user, plan)
	previousDay := time.Now().UTC().Truncate(24 * time.Hour).Add(-24 * time.Hour)
	current := createQuotaRecordAt(t, sub, "calendar-history-current", 60, previousDay.Add(time.Hour).Unix())
	createQuotaRecordAt(t, sub, "calendar-history-other", 40, previousDay.Add(2*time.Hour).Unix())

	err := SettleSubscriptionPreConsume(current.RequestId, 70)
	require.Error(t, err, "late settlement must validate the request's historical calendar bucket")
	var got SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", current.RequestId).First(&got).Error)
	require.Equal(t, int64(60), effectivePreConsumeAmount(&got))
}

func TestSubscriptionRollingSettlementChecksHistoricalPeak(t *testing.T) {
	truncateTables(t)
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId: 0, Title: "historical rolling settlement", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		QuotaWindows: SubscriptionQuotaWindowList{{
			Type: SubscriptionQuotaWindowCustom, Amount: 100, WindowSeconds: 3600,
			ResetMode: SubscriptionQuotaWindowRolling,
		}},
	}
	sub := createQuotaTestSubscription(t, user, plan)
	now := time.Now().Unix()
	recordTime := now - 7200
	createQuotaRecordAt(t, sub, "rolling-history-before", 40, recordTime-1000)
	current := createQuotaRecordAt(t, sub, "rolling-history-current", 20, recordTime)
	createQuotaRecordAt(t, sub, "rolling-history-after", 40, recordTime+1000)

	err := SettleSubscriptionPreConsume(current.RequestId, 30)
	require.Error(t, err, "late settlement must validate every historical rolling peak")
	var got SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", current.RequestId).First(&got).Error)
	require.Equal(t, int64(20), effectivePreConsumeAmount(&got))
}

func TestSubscriptionFiveHourSettlementChecksCompleteTarget(t *testing.T) {
	truncateTables(t)
	user := newQuotaTestUser(t)
	plan := &SubscriptionPlan{
		ProviderId: 0, Title: "fixed rolling settlement", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		QuotaWindowMode: SubscriptionQuotaWindowFiveHour,
		FiveHourAmount:  100, FiveHourWindowSeconds: 3600,
	}
	createQuotaTestSubscription(t, user, plan)
	_, err := PreConsumeUserSubscription("five-hour-other", user.Id, "", 0, 40)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("five-hour-current", user.Id, "", 0, 60)
	require.NoError(t, err)

	err = SettleSubscriptionPreConsume("five-hour-current", 90)
	require.Error(t, err, "settlement must validate other usage plus the complete final target")
	var got SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", "five-hour-current").First(&got).Error)
	require.Equal(t, int64(60), effectivePreConsumeAmount(&got))
}

func TestCleanupSubscriptionPreConsumeRecordsUsesPerSubscriptionCutoff(t *testing.T) {
	truncateTables(t)
	user := newQuotaTestUser(t)
	longPlan := &SubscriptionPlan{
		ProviderId: 0, Title: "long rolling cleanup", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 2,
		QuotaWindows: SubscriptionQuotaWindowList{{
			Type: SubscriptionQuotaWindowCustom, Amount: 100, WindowSeconds: 30 * 24 * 60 * 60,
			ResetMode: SubscriptionQuotaWindowRolling,
		}},
	}
	longSub := createQuotaTestSubscription(t, user, longPlan)
	legacyPlan := &SubscriptionPlan{
		ProviderId: 0, Title: "legacy cleanup", Enabled: true, AllowPurchase: 1,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 2,
		TotalAmount: 100, QuotaResetPeriod: SubscriptionResetNever,
	}
	legacySub := createQuotaTestSubscription(t, user, legacyPlan)
	old := time.Now().Unix() - 10*24*60*60
	longRecord := createQuotaRecordAt(t, longSub, "cleanup-long-window", 1, old)
	legacyRecord := createQuotaRecordAt(t, legacySub, "cleanup-unrelated", 1, old)

	deleted, err := CleanupSubscriptionPreConsumeRecords(7 * 24 * 60 * 60)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
	require.NoError(t, DB.First(&SubscriptionPreConsumeRecord{}, longRecord.Id).Error,
		"record inside this subscription's rolling window must be retained")
	require.ErrorIs(t, DB.First(&SubscriptionPreConsumeRecord{}, legacyRecord.Id).Error, gorm.ErrRecordNotFound,
		"a long window on another subscription must not retain unrelated history")
}

func TestSubscriptionAggregateSettlementChecksHistoricalPeriod(t *testing.T) {
	truncateTables(t)
	now := time.Now()
	location := now.Location()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	currentMonday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location).
		AddDate(0, 0, -(weekday - 1))
	previousMonday := currentMonday.AddDate(0, 0, -7)

	cases := []struct {
		name string
		plan *SubscriptionPlan
	}{
		{
			name: "fixed-weekly",
			plan: &SubscriptionPlan{
				ProviderId: 0, Title: "fixed weekly historical", Enabled: true, AllowPurchase: 1,
				DurationUnit: SubscriptionDurationMonth, DurationValue: 2, TotalAmount: 100,
				QuotaWindowMode: SubscriptionQuotaWindowWeekly, WeeklyAmount: 100,
				QuotaResetPeriod: SubscriptionResetWeekly,
			},
		},
		{
			name: "legacy-weekly",
			plan: &SubscriptionPlan{
				ProviderId: 0, Title: "legacy weekly historical", Enabled: true, AllowPurchase: 1,
				DurationUnit: SubscriptionDurationMonth, DurationValue: 2, TotalAmount: 100,
				QuotaResetPeriod: SubscriptionResetWeekly,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := newQuotaTestUser(t)
			sub := createQuotaTestSubscription(t, user, tc.plan)
			require.NoError(t, DB.Model(sub).UpdateColumns(map[string]interface{}{
				"start_time":      previousMonday.AddDate(0, 0, -7).Unix(),
				"end_time":        currentMonday.AddDate(0, 0, 21).Unix(),
				"last_reset_time": currentMonday.Unix(),
				"next_reset_time": currentMonday.AddDate(0, 0, 7).Unix(),
				"amount_used":     0,
			}).Error)
			sub.StartTime = previousMonday.AddDate(0, 0, -7).Unix()
			sub.EndTime = currentMonday.AddDate(0, 0, 21).Unix()
			sub.LastResetTime = currentMonday.Unix()
			sub.NextResetTime = currentMonday.AddDate(0, 0, 7).Unix()
			current := createQuotaRecordAt(t, sub, tc.name+"-current", 60, previousMonday.Add(12*time.Hour).Unix())
			createQuotaRecordAt(t, sub, tc.name+"-other", 40, previousMonday.Add(24*time.Hour).Unix())

			err := SettleSubscriptionPreConsume(current.RequestId, 90)
			require.Error(t, err, "late settlement must validate the historical aggregate reset period")
			var got SubscriptionPreConsumeRecord
			require.NoError(t, DB.Where("request_id = ?", current.RequestId).First(&got).Error)
			require.Equal(t, int64(60), effectivePreConsumeAmount(&got))
		})
	}
}
