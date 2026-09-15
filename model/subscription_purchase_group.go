package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SQLite uses deferred transactions and cannot reliably upgrade two
// concurrent read transactions to writers (the loser reports SQLITE_BUSY /
// "database is deadlocked" even when busy_timeout is configured).  Serialize
// subscription stock mutations in-process; PostgreSQL/MySQL retain their
// normal row-lock concurrency.  The lock is intentionally package-local and
// only held around the short reservation/issuance operation.
var subscriptionSQLiteStockMu sync.Mutex

var errSubscriptionPurchaseScopeChanged = errors.New("subscription purchase limit group changed concurrently")

// subscriptionOrdersTableExists reports whether the subscription order table
// is available on the current connection.  A few model/controller call sites
// (and, more importantly, older installations during a rolling migration)
// may update a subscription_plans row before the subscription_orders migration
// has run.  In that situation there cannot be a pending reservation to guard,
// so the pending-order checks should be skipped rather than making an
// otherwise unrelated plan update fail with "table does not exist".
func subscriptionOrdersTableExists(tx *gorm.DB) bool {
	if tx == nil {
		return false
	}
	return tx.Migrator().HasTable(&SubscriptionOrder{})
}

// NormalizeSubscriptionPurchaseLimitGroup returns one database-independent
// group key. Limiting keys to lower-case ASCII avoids MySQL's commonly
// case-insensitive collations disagreeing with PostgreSQL/SQLite comparisons.
func NormalizeSubscriptionPurchaseLimitGroup(raw string) (string, bool) {
	group := strings.ToLower(strings.TrimSpace(raw))
	if group == "" {
		return "", true
	}
	if len(group) > 64 {
		return "", false
	}
	for i := 0; i < len(group); i++ {
		ch := group[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.' || ch == ':' {
			continue
		}
		return "", false
	}
	return group, true
}

// subscriptionPurchaseScope is the set of plans that share purchase limits.
// An empty purchase_limit_group keeps the historical, single-plan behaviour.
type subscriptionPurchaseScope struct {
	target             SubscriptionPlan
	plans              []SubscriptionPlan
	planIDs            []int
	maxPerUser         int
	totalPurchaseLimit int64
	issuedCount        int64
	reservedCount      int64
}

func minPositiveInt(current, candidate int) int {
	if candidate <= 0 {
		return current
	}
	if current <= 0 || candidate < current {
		return candidate
	}
	return current
}

func minPositiveInt64(current, candidate int64) int64 {
	if candidate <= 0 {
		return current
	}
	if current <= 0 || candidate < current {
		return candidate
	}
	return current
}

func addNonNegativeSaturated(left, right int64) int64 {
	if left < 0 {
		left = 0
	}
	if right <= 0 {
		return left
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if left > maxInt64-right {
		return maxInt64
	}
	return left + right
}

// loadSubscriptionPurchaseScopeTx locks every plan in a purchase-limit group
// in ascending ID order. Using one deterministic order prevents two buyers of
// different plans in the same group from exceeding the shared stock limit.
func loadSubscriptionPurchaseScopeTx(tx *gorm.DB, planID int) (*subscriptionPurchaseScope, error) {
	if tx == nil || planID <= 0 {
		return nil, errors.New("invalid subscription purchase scope")
	}

	var target SubscriptionPlan
	if err := tx.Where("id = ?", planID).First(&target).Error; err != nil {
		return nil, err
	}
	group, ok := NormalizeSubscriptionPurchaseLimitGroup(target.PurchaseLimitGroup)
	if !ok {
		return nil, errors.New("invalid subscription purchase limit group")
	}
	target.PurchaseLimitGroup = group

	plans := make([]SubscriptionPlan, 0, 1)
	if target.PurchaseLimitGroup == "" {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", target.Id).Find(&plans).Error; err != nil {
			return nil, err
		}
	} else {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("provider_id = ? AND purchase_limit_group = ?", target.ProviderId, target.PurchaseLimitGroup).
			Order("id asc").Find(&plans).Error; err != nil {
			return nil, err
		}
	}
	if len(plans) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	scope := &subscriptionPurchaseScope{
		target:  target,
		plans:   plans,
		planIDs: make([]int, 0, len(plans)),
	}
	targetFound := false
	for i := range plans {
		plan := &plans[i]
		if plan.IssuedCount < 0 || plan.ReservedCount < 0 {
			return nil, ErrSubscriptionStockInvalid
		}
		scope.planIDs = append(scope.planIDs, plan.Id)
		scope.maxPerUser = minPositiveInt(scope.maxPerUser, plan.MaxPurchasePerUser)
		scope.totalPurchaseLimit = minPositiveInt64(scope.totalPurchaseLimit, plan.TotalPurchaseLimit)
		scope.issuedCount = addNonNegativeSaturated(scope.issuedCount, plan.IssuedCount)
		scope.reservedCount = addNonNegativeSaturated(scope.reservedCount, plan.ReservedCount)
		if plan.Id == target.Id {
			lockedGroup, valid := NormalizeSubscriptionPurchaseLimitGroup(plan.PurchaseLimitGroup)
			if !valid || plan.ProviderId != target.ProviderId || lockedGroup != target.PurchaseLimitGroup {
				return nil, errSubscriptionPurchaseScopeChanged
			}
			plan.PurchaseLimitGroup = lockedGroup
			scope.target = *plan
			targetFound = true
		}
	}
	if !targetFound {
		return nil, gorm.ErrRecordNotFound
	}
	return scope, nil
}

// validateSubscriptionPlanPurchaseLimitsTx validates a proposed plan update
// against both its own issued stock and the destination group's shared stock.
// The strictest positive limit configured by any group member is authoritative;
// zero continues to mean unlimited.
func ValidateSubscriptionPlanPurchaseLimitsTx(tx *gorm.DB, proposed *SubscriptionPlan) error {
	if tx == nil || proposed == nil {
		return errors.New("invalid subscription plan")
	}
	if proposed.TotalPurchaseLimit < 0 || proposed.MaxPurchasePerUser < 0 {
		return ErrSubscriptionLimitTooSmall
	}
	proposedGroup, ok := NormalizeSubscriptionPurchaseLimitGroup(proposed.PurchaseLimitGroup)
	if !ok {
		return errors.New("invalid subscription purchase limit group")
	}
	proposed.PurchaseLimitGroup = proposedGroup

	var current SubscriptionPlan
	hasCurrent := false
	originalProviderID := 0
	originalGroup := ""
	if proposed.Id > 0 {
		query := tx.Where("id = ?", proposed.Id).First(&current)
		if query.Error != nil && !errors.Is(query.Error, gorm.ErrRecordNotFound) {
			return query.Error
		}
		hasCurrent = query.Error == nil
		if hasCurrent {
			originalProviderID = current.ProviderId
		}
	}
	currentGroup, currentGroupOK := NormalizeSubscriptionPurchaseLimitGroup(current.PurchaseLimitGroup)
	if hasCurrent && !currentGroupOK {
		return errors.New("invalid existing subscription purchase limit group")
	}
	originalGroup = currentGroup

	// Lock the union of the old and destination scopes once, in one ascending
	// ID order. Locking target first and lower-ID members afterwards can
	// deadlock against purchase transactions that always lock the whole group.
	var lockedPlans []SubscriptionPlan
	lockQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&SubscriptionPlan{})
	hasLockCondition := false
	if proposed.Id > 0 {
		lockQuery = lockQuery.Where("id = ?", proposed.Id)
		hasLockCondition = true
	}
	if hasCurrent && currentGroup != "" {
		if hasLockCondition {
			lockQuery = lockQuery.Or("provider_id = ? AND purchase_limit_group = ?", current.ProviderId, currentGroup)
		} else {
			lockQuery = lockQuery.Where("provider_id = ? AND purchase_limit_group = ?", current.ProviderId, currentGroup)
			hasLockCondition = true
		}
	}
	if proposedGroup != "" {
		if hasLockCondition {
			lockQuery = lockQuery.Or("provider_id = ? AND purchase_limit_group = ?", proposed.ProviderId, proposedGroup)
		} else {
			lockQuery = lockQuery.Where("provider_id = ? AND purchase_limit_group = ?", proposed.ProviderId, proposedGroup)
			hasLockCondition = true
		}
	}
	if hasLockCondition {
		if err := lockQuery.Order("id asc").Find(&lockedPlans).Error; err != nil {
			return err
		}
	}
	if proposed.Id > 0 {
		hasCurrent = false
		for i := range lockedPlans {
			if lockedPlans[i].Id == proposed.Id {
				current = lockedPlans[i]
				hasCurrent = true
				break
			}
		}
		if !hasCurrent {
			return gorm.ErrRecordNotFound
		}
		lockedGroup, valid := NormalizeSubscriptionPurchaseLimitGroup(current.PurchaseLimitGroup)
		if !valid || current.ProviderId != originalProviderID || lockedGroup != originalGroup {
			return errSubscriptionPurchaseScopeChanged
		}
		// A pending checkout has already reserved stock against this plan's
		// provider/group scope.  Moving the plan while that order is pending
		// would make completion use a different tenant/scope and can strand the
		// reservation.  Keep provider and purchase-limit-group immutable until
		// all pending orders have either completed or expired.  Other catalog
		// fields remain editable as usual.
		if current.ProviderId != proposed.ProviderId || lockedGroup != proposedGroup {
			if !subscriptionOrdersTableExists(tx) {
				// During a rolling migration there is no order table and thus no
				// reservation that could be stranded by this change.
				return nil
			}
			var pendingCount int64
			if err := tx.Model(&SubscriptionOrder{}).
				Where("plan_id = ? AND status = ?", proposed.Id, common.TopUpStatusPending).
				Count(&pendingCount).Error; err != nil {
				return err
			}
			if pendingCount > 0 {
				return ErrSubscriptionPlanHasPendingOrders
			}
		}
	}

	issued := int64(0)
	reserved := int64(0)
	effectiveLimit := proposed.TotalPurchaseLimit
	if hasCurrent {
		issued = addNonNegativeSaturated(issued, current.IssuedCount)
		reserved = addNonNegativeSaturated(reserved, current.ReservedCount)
	}

	if proposedGroup != "" {
		for i := range lockedPlans {
			member := &lockedPlans[i]
			memberGroup, valid := NormalizeSubscriptionPurchaseLimitGroup(member.PurchaseLimitGroup)
			if !valid || member.Id == proposed.Id || member.ProviderId != proposed.ProviderId || memberGroup != proposedGroup {
				continue
			}
			issued = addNonNegativeSaturated(issued, member.IssuedCount)
			reserved = addNonNegativeSaturated(reserved, member.ReservedCount)
			effectiveLimit = minPositiveInt64(effectiveLimit, member.TotalPurchaseLimit)
		}
	}

	allocated := addNonNegativeSaturated(issued, reserved)
	if effectiveLimit > 0 && allocated > effectiveLimit {
		return ErrSubscriptionLimitTooSmall
	}
	return nil
}

// countUserSubscriptionsByPurchaseScope returns issued subscriptions only,
// matching the historical CountUserSubscriptionsByPlan contract while making
// a non-empty purchase_limit_group span all plans in the same provider.
func countUserSubscriptionsByPurchaseScope(userID, planID int) (int64, error) {
	if userID <= 0 || planID <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var plan SubscriptionPlan
	if err := DB.Select("id", "provider_id", "purchase_limit_group").Where("id = ?", planID).First(&plan).Error; err != nil {
		return 0, err
	}
	group, ok := NormalizeSubscriptionPurchaseLimitGroup(plan.PurchaseLimitGroup)
	if !ok {
		return 0, fmt.Errorf("invalid subscription purchase limit group for plan %d", planID)
	}
	query := DB.Model(&UserSubscription{}).Where("user_id = ?", userID)
	if group == "" {
		query = query.Where("plan_id = ?", planID)
	} else {
		var planIDs []int
		if err := DB.Model(&SubscriptionPlan{}).
			Where("provider_id = ? AND purchase_limit_group = ?", plan.ProviderId, group).
			Pluck("id", &planIDs).Error; err != nil {
			return 0, err
		}
		if len(planIDs) == 0 {
			return 0, nil
		}
		query = query.Where("plan_id IN ?", planIDs)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func checkUserSubscriptionPurchaseScopeLimitTx(tx *gorm.DB, userID, planID int) error {
	if tx == nil || userID <= 0 || planID <= 0 {
		return errors.New("invalid subscription purchase limit")
	}
	scope, err := loadSubscriptionPurchaseScopeTx(tx, planID)
	if err != nil {
		return err
	}
	if scope.maxPerUser <= 0 {
		return nil
	}

	var issuedCount int64
	if err := tx.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id IN ?", userID, scope.planIDs).
		Count(&issuedCount).Error; err != nil {
		return err
	}
	var reservedCount int64
	if err := tx.Model(&SubscriptionOrder{}).
		Where("user_id = ? AND plan_id IN ? AND status = ? AND stock_status = ?",
			userID, scope.planIDs, common.TopUpStatusPending, SubscriptionStockStatusReserved).
		Count(&reservedCount).Error; err != nil {
		return err
	}
	if addNonNegativeSaturated(issuedCount, reservedCount) >= int64(scope.maxPerUser) {
		return ErrSubscriptionPurchaseLimit
	}
	return nil
}

func incrementSubscriptionPurchaseScopeStockTx(tx *gorm.DB, planID int, reserve bool) error {
	scope, err := loadSubscriptionPurchaseScopeTx(tx, planID)
	if err != nil {
		return err
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	// Guard the per-row counter as well as the shared allocation sum.  An
	// unlimited plan can otherwise wrap BIGINT at MaxInt64 even though the
	// saturated scope total still appears to have capacity.
	if reserve {
		if scope.target.ReservedCount < 0 || scope.target.ReservedCount >= maxInt64 {
			return ErrSubscriptionStockInvalid
		}
	} else if scope.target.IssuedCount < 0 || scope.target.IssuedCount >= maxInt64 {
		return ErrSubscriptionStockInvalid
	}
	allocated := addNonNegativeSaturated(scope.issuedCount, scope.reservedCount)
	if scope.totalPurchaseLimit > 0 && allocated >= scope.totalPurchaseLimit {
		return ErrSubscriptionPlanSoldOut
	}
	column := "issued_count"
	if reserve {
		column = "reserved_count"
	}
	res := tx.Model(&SubscriptionPlan{}).Where("id = ?", scope.target.Id).
		Where(column+" >= 0 AND "+column+" < ?", maxInt64).
		UpdateColumn(column, gorm.Expr(column+" + ?", 1))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
