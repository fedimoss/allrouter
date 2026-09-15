package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Subscription duration units
const (
	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"
)

// Subscription quota reset period
const (
	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetYearly  = "yearly"
	SubscriptionResetCustom  = "custom"
)

// Subscription quota window modes. Legacy plans retain the original
// TotalAmount + QuotaResetPeriod behaviour.
const (
	SubscriptionQuotaWindowLegacy   = "legacy"
	SubscriptionQuotaWindowFiveHour = "five_hour"
	SubscriptionQuotaWindowWeekly   = "weekly"
	SubscriptionQuotaWindowDual     = "dual"
	SubscriptionQuotaWindowGeneric  = "generic"

	// DefaultFiveHourWindowSeconds is the ChatGPT-like rolling window length.
	DefaultFiveHourWindowSeconds int64 = 5 * 60 * 60
)

// Generic quota window units. Values are persisted in quota_windows and
// exposed by the subscription API.
const (
	SubscriptionQuotaWindowHour   = "hour"
	SubscriptionQuotaWindowDay    = "day"
	SubscriptionQuotaWindowWeek   = "week"
	SubscriptionQuotaWindowMonth  = "month"
	SubscriptionQuotaWindowYear   = "year"
	SubscriptionQuotaWindowCustom = "custom"

	SubscriptionQuotaWindowRolling  = "rolling"
	SubscriptionQuotaWindowCalendar = "calendar"

	// Keep exact-second arithmetic and time.Duration conversions safely below
	// their overflow limits while still supporting long multi-year plans.
	MaxSubscriptionQuotaWindowSeconds int64 = 100 * 366 * 24 * 60 * 60
)

// SubscriptionQuotaWindow describes one independently enforced quota
// condition. Amount is the canonical limit (0 disables the condition).
// Type, Period, and Unit are accepted aliases; Limit and Quota are accepted
// aliases for Amount. Duration is a unit count, while WindowSeconds (or
// CustomSeconds) can specify an exact duration. ResetMode defaults to rolling;
// calendar mode aligns day/week/month/year windows to UTC boundaries.
type SubscriptionQuotaWindow struct {
	Type            string `json:"type,omitempty"`
	Period          string `json:"period,omitempty"`
	Unit            string `json:"unit,omitempty"`
	Amount          int64  `json:"amount,omitempty"`
	Limit           int64  `json:"limit,omitempty"`
	Quota           int64  `json:"quota,omitempty"`
	Duration        int64  `json:"duration,omitempty"`
	DurationValue   int64  `json:"duration_value,omitempty"`
	WindowSeconds   int64  `json:"window_seconds,omitempty"`
	CustomSeconds   int64  `json:"custom_seconds,omitempty"`
	DurationSeconds int64  `json:"duration_seconds,omitempty"`
	ResetMode       string `json:"reset_mode,omitempty"`
	Calendar        bool   `json:"calendar,omitempty"`
	Rolling         bool   `json:"rolling,omitempty"`
	Name            string `json:"name,omitempty"`
}

// SubscriptionQuotaWindowList is persisted as JSON TEXT, which is supported
// consistently by SQLite, MySQL, and PostgreSQL.
type SubscriptionQuotaWindowList []SubscriptionQuotaWindow

func (w SubscriptionQuotaWindowList) Value() (driver.Value, error) {
	if len(w) == 0 {
		return "[]", nil
	}
	b, err := common.Marshal(w)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (w *SubscriptionQuotaWindowList) Scan(value any) error {
	if value == nil {
		*w = nil
		return nil
	}
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into SubscriptionQuotaWindowList", value)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		*w = nil
		return nil
	}
	var parsed SubscriptionQuotaWindowList
	if err := common.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	*w = parsed
	return nil
}

func normalizeSubscriptionQuotaWindowUnit(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return "", true
	case "5h", "5hour", "5_hours", "five_hour", "five-hours",
		SubscriptionQuotaWindowHour, "hourly", "hours":
		return SubscriptionQuotaWindowHour, true
	case SubscriptionQuotaWindowDay, "daily", "days":
		return SubscriptionQuotaWindowDay, true
	case SubscriptionQuotaWindowWeek, "weekly", "weeks":
		return SubscriptionQuotaWindowWeek, true
	case SubscriptionQuotaWindowMonth, "monthly", "months":
		return SubscriptionQuotaWindowMonth, true
	case SubscriptionQuotaWindowYear, "yearly", "years", "annual", "annually":
		return SubscriptionQuotaWindowYear, true
	case SubscriptionQuotaWindowCustom, "seconds", "second", "duration":
		return SubscriptionQuotaWindowCustom, true
	default:
		return "", false
	}
}

func quotaWindowNominalUnitSeconds(unit string) int64 {
	switch unit {
	case SubscriptionQuotaWindowHour:
		return 60 * 60
	case SubscriptionQuotaWindowDay:
		return 24 * 60 * 60
	case SubscriptionQuotaWindowWeek:
		return 7 * 24 * 60 * 60
	case SubscriptionQuotaWindowMonth:
		return 30 * 24 * 60 * 60
	case SubscriptionQuotaWindowYear:
		return 365 * 24 * 60 * 60
	default:
		return 0
	}
}

func pickPositiveQuotaWindowAlias(values ...int64) (int64, bool) {
	var selected int64
	for _, value := range values {
		if value < 0 {
			return 0, false
		}
		if value == 0 {
			continue
		}
		if selected != 0 && selected != value {
			return 0, false
		}
		selected = value
	}
	return selected, true
}

// Normalize folds aliases into canonical values and returns false for an
// invalid or non-positive window. Calendar units default to UTC calendar
// boundaries; hour/custom units default to rolling windows.
func (w SubscriptionQuotaWindow) Normalize() (SubscriptionQuotaWindow, bool) {
	var unit string
	for _, raw := range []string{w.Type, w.Period, w.Unit} {
		normalized, ok := normalizeSubscriptionQuotaWindowUnit(raw)
		if !ok {
			return SubscriptionQuotaWindow{}, false
		}
		if normalized == "" {
			continue
		}
		if unit != "" && unit != normalized {
			return SubscriptionQuotaWindow{}, false
		}
		unit = normalized
	}
	if unit == "" {
		return SubscriptionQuotaWindow{}, false
	}

	amount, ok := pickPositiveQuotaWindowAlias(w.Amount, w.Limit, w.Quota)
	if !ok || amount <= 0 {
		return SubscriptionQuotaWindow{}, false
	}
	count, ok := pickPositiveQuotaWindowAlias(w.Duration, w.DurationValue)
	if !ok {
		return SubscriptionQuotaWindow{}, false
	}
	seconds, ok := pickPositiveQuotaWindowAlias(w.WindowSeconds, w.CustomSeconds, w.DurationSeconds)
	if !ok {
		return SubscriptionQuotaWindow{}, false
	}

	if w.Calendar && w.Rolling {
		return SubscriptionQuotaWindow{}, false
	}
	resetMode := strings.ToLower(strings.TrimSpace(w.ResetMode))
	if resetMode != "" && resetMode != SubscriptionQuotaWindowCalendar && resetMode != SubscriptionQuotaWindowRolling {
		return SubscriptionQuotaWindow{}, false
	}
	if w.Calendar {
		if resetMode == SubscriptionQuotaWindowRolling {
			return SubscriptionQuotaWindow{}, false
		}
		resetMode = SubscriptionQuotaWindowCalendar
	}
	if w.Rolling {
		if resetMode == SubscriptionQuotaWindowCalendar {
			return SubscriptionQuotaWindow{}, false
		}
		resetMode = SubscriptionQuotaWindowRolling
	}
	if resetMode == "" {
		switch unit {
		case SubscriptionQuotaWindowDay, SubscriptionQuotaWindowWeek,
			SubscriptionQuotaWindowMonth, SubscriptionQuotaWindowYear:
			resetMode = SubscriptionQuotaWindowCalendar
		default:
			resetMode = SubscriptionQuotaWindowRolling
		}
	}

	if unit == SubscriptionQuotaWindowCustom {
		if seconds <= 0 || count > 1 || seconds > MaxSubscriptionQuotaWindowSeconds {
			return SubscriptionQuotaWindow{}, false
		}
		count = 1
	} else {
		baseSeconds := quotaWindowNominalUnitSeconds(unit)
		if baseSeconds <= 0 {
			return SubscriptionQuotaWindow{}, false
		}
		if count <= 0 && seconds > 0 {
			if seconds%baseSeconds != 0 {
				return SubscriptionQuotaWindow{}, false
			}
			count = seconds / baseSeconds
		}
		if count <= 0 {
			count = 1
			for _, raw := range []string{w.Type, w.Period, w.Unit} {
				switch strings.ToLower(strings.TrimSpace(raw)) {
				case "5h", "5hour", "5_hours", "five_hour", "five-hours":
					count = 5
				}
			}
		}
		if count > MaxSubscriptionQuotaWindowSeconds/baseSeconds {
			return SubscriptionQuotaWindow{}, false
		}
		expectedSeconds := count * baseSeconds
		if seconds > 0 && seconds != expectedSeconds {
			return SubscriptionQuotaWindow{}, false
		}
		seconds = expectedSeconds
	}
	if count <= 0 || seconds <= 0 || seconds > MaxSubscriptionQuotaWindowSeconds {
		return SubscriptionQuotaWindow{}, false
	}

	return SubscriptionQuotaWindow{
		Type: unit, Period: unit, Unit: unit,
		Amount: amount, Limit: amount, Quota: amount,
		Duration: count, DurationValue: count,
		WindowSeconds: seconds, CustomSeconds: seconds, DurationSeconds: seconds,
		ResetMode: resetMode, Calendar: resetMode == SubscriptionQuotaWindowCalendar,
		Rolling: resetMode == SubscriptionQuotaWindowRolling,
		Name:    strings.TrimSpace(w.Name),
	}, true
}
func NormalizeQuotaWindowList(w SubscriptionQuotaWindowList) SubscriptionQuotaWindowList {
	if len(w) == 0 {
		return nil
	}
	result := make(SubscriptionQuotaWindowList, 0, len(w))
	seen := make(map[string]struct{}, len(w))
	for _, item := range w {
		normalized, ok := item.Normalize()
		if !ok {
			continue
		}
		key := fmt.Sprintf("%s:%d:%d:%s:%s", normalized.Type, normalized.WindowSeconds, normalized.Amount, normalized.ResetMode, normalized.Name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

var (
	ErrSubscriptionOrderNotFound      = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid = errors.New("subscription order status invalid")
	ErrSubscriptionPlanSoldOut        = errors.New("该套餐已发放完毕")
	ErrSubscriptionPurchaseLimit      = errors.New("已达到该套餐购买上限")
	ErrSubscriptionStockInvalid       = errors.New("subscription stock state invalid")
	ErrSubscriptionLimitTooSmall      = errors.New("全局发放上限不能小于已发放和预占数量")
	// ErrSubscriptionCheckoutChanged means the catalog payment configuration
	// changed after a controller prepared a checkout but before the order row
	// was reserved.  The caller must restart checkout with the new price/product
	// rather than mixing an old remote payment with a new entitlement snapshot.
	ErrSubscriptionCheckoutChanged = errors.New("subscription checkout configuration changed; retry checkout")
	// ErrSubscriptionPlanHasPendingOrders is returned when an administrator
	// attempts to move a plan between providers/purchase-limit groups while a
	// checkout is still pending.  The pending order has already reserved stock
	// in its original scope; allowing the move would make completion ambiguous
	// and could strand that reservation.
	ErrSubscriptionPlanHasPendingOrders = errors.New("subscription plan has pending orders")
)

const (
	SubscriptionStockStatusReserved = "reserved"
	SubscriptionStockStatusIssued   = "issued"
	SubscriptionStockStatusReleased = "released"

	// 支付订单默认预占 45 分钟。支付渠道的结账有效期应不晚于该时间。
	DefaultSubscriptionStockReservationSeconds int64 = 45 * 60
	// 外部结账会话使用 40 分钟，给支付成功回调预留 5 分钟宽限期。
	DefaultSubscriptionCheckoutSeconds int64 = 40 * 60
)

const (
	subscriptionPlanCacheNamespace     = "new-api:subscription_plan:v1"
	subscriptionPlanInfoCacheNamespace = "new-api:subscription_plan_info:v1"
)

var (
	subscriptionPlanCacheOnce     sync.Once
	subscriptionPlanInfoCacheOnce sync.Once

	subscriptionPlanCache     *cachex.HybridCache[SubscriptionPlan]
	subscriptionPlanInfoCache *cachex.HybridCache[SubscriptionPlanInfo]
)

func subscriptionPlanCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanInfoCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_TTL", 120)
	if ttlSeconds <= 0 {
		ttlSeconds = 120
	}
	return time.Duration(ttlSeconds) * time.Second
}

func subscriptionPlanCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_CACHE_CAP", 5000)
	if capacity <= 0 {
		capacity = 5000
	}
	return capacity
}

func subscriptionPlanInfoCacheCapacity() int {
	capacity := common.GetEnvOrDefault("SUBSCRIPTION_PLAN_INFO_CACHE_CAP", 10000)
	if capacity <= 0 {
		capacity = 10000
	}
	return capacity
}

func getSubscriptionPlanCache() *cachex.HybridCache[SubscriptionPlan] {
	subscriptionPlanCacheOnce.Do(func() {
		ttl := subscriptionPlanCacheTTL()
		subscriptionPlanCache = cachex.NewHybridCache[SubscriptionPlan](cachex.HybridCacheConfig[SubscriptionPlan]{
			Namespace: cachex.Namespace(subscriptionPlanCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlan]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlan] {
				return hot.NewHotCache[string, SubscriptionPlan](hot.LRU, subscriptionPlanCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanCache
}

func getSubscriptionPlanInfoCache() *cachex.HybridCache[SubscriptionPlanInfo] {
	subscriptionPlanInfoCacheOnce.Do(func() {
		ttl := subscriptionPlanInfoCacheTTL()
		subscriptionPlanInfoCache = cachex.NewHybridCache[SubscriptionPlanInfo](cachex.HybridCacheConfig[SubscriptionPlanInfo]{
			Namespace: cachex.Namespace(subscriptionPlanInfoCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[SubscriptionPlanInfo]{},
			Memory: func() *hot.HotCache[string, SubscriptionPlanInfo] {
				return hot.NewHotCache[string, SubscriptionPlanInfo](hot.LRU, subscriptionPlanInfoCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return subscriptionPlanInfoCache
}

// 将数值类型 ID 转为字符串类型
func subscriptionPlanCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func InvalidateSubscriptionPlanCache(planId int) {
	if planId <= 0 {
		return
	}
	cache := getSubscriptionPlanCache()
	_, _ = cache.DeleteMany([]string{subscriptionPlanCacheKey(planId)})
	infoCache := getSubscriptionPlanInfoCache()
	_ = infoCache.Purge()
}

// Subscription plan
type SubscriptionPlan struct {
	Id int `json:"id"`

	// ProviderId 订阅套餐归属服务商 ID。
	// 0 表示主站套餐（所有主站用户可见），>0 表示该服务商私有套餐（仅该服务商站点用户可见）。
	// 由本次"服务商私有订阅"特性新增，配套迁移见 docs/sql/20260708_provider_owned_subscriptions.sql。
	ProviderId int `json:"provider_id" gorm:"type:int;not null;default:0;index;index:idx_subscription_plan_purchase_group,priority:1"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

	// Display money amount (follow existing code style: float64 for money)
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(12,6);not null;default:0"`
	Currency    string  `json:"currency" gorm:"type:varchar(8);not null;default:'USD'"`

	DurationUnit  string `json:"duration_unit" gorm:"type:varchar(16);not null;default:'month'"`
	DurationValue int    `json:"duration_value" gorm:"type:int;not null;default:1"`
	CustomSeconds int64  `json:"custom_seconds" gorm:"type:bigint;not null;default:0"`

	Enabled   bool `json:"enabled" gorm:"default:true"`
	SortOrder int  `json:"sort_order" gorm:"type:int;default:0"`

	// AllowPurchase 控制套餐是否允许用户在前端自助购买。
	// 1=允许购买（默认），0=禁止购买（仅管理员可手动绑定，如 VIP 专属套餐）。
	// 该字段不影响管理员通过 AdminBindSubscription / GrantAirdropSubscription 等方式授予订阅。
	AllowPurchase int `json:"allow_purchase" gorm:"type:int;default:1"`

	// ModelLimits 限制该套餐可使用的模型白名单，逗号分隔的模型名称列表。
	// 为空表示不限制（所有模型均可使用）。
	// 非空时，PreConsumeUserSubscription 仅对列表中的模型扣费，其他模型跳过该订阅。
	// 格式示例: "gpt-4,gpt-4o,claude-sonnet-5"
	ModelLimits string `json:"model_limits" gorm:"type:text;default:''"`

	StripePriceId    string `json:"stripe_price_id" gorm:"type:varchar(128);default:''"`
	StripePriceCnyId string `json:"stripe_price_cny_id" gorm:"type:varchar(128);default:''"`

	CreemProductId        string `json:"creem_product_id" gorm:"type:varchar(128);default:''"`
	WaffoPancakeProductId string `json:"waffo_pancake_product_id" gorm:"type:varchar(128);default:''"`

	// Max purchases per user (0 = unlimited)
	MaxPurchasePerUser int `json:"max_purchase_per_user" gorm:"type:int;default:0"`
	// PurchaseLimitGroup makes per-user and global issuance limits span every
	// plan with the same non-empty provider_id + group key.
	PurchaseLimitGroup string `json:"purchase_limit_group" gorm:"type:varchar(64);not null;default:'';index:idx_subscription_plan_purchase_group,priority:2"`

	// TotalPurchaseLimit 是套餐全局发放上限，0 表示无限量。
	// 购买、管理员赠送、空投和注册赠送都会占用该额度。
	TotalPurchaseLimit int64 `json:"total_purchase_limit" gorm:"type:bigint;not null;default:0"`
	// IssuedCount 是已经成功创建过的用户订阅数量；订阅到期、取消后不返还。
	IssuedCount int64 `json:"issued_count" gorm:"type:bigint;not null;default:0"`
	// ReservedCount 是待支付订单临时占用的数量；失败或超时后释放。
	ReservedCount int64 `json:"reserved_count" gorm:"type:bigint;not null;default:0"`

	// Upgrade user group after purchase (empty = no change)
	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

	// Total quota (amount in quota units, 0 = unlimited)
	TotalAmount int64 `json:"total_amount" gorm:"type:bigint;not null;default:0"`

	// Quota reset period for plan
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	// Independent quota windows. Empty/legacy plans continue to use
	// TotalAmount and QuotaResetPeriod exactly as before.
	QuotaWindowMode       string `json:"quota_window_mode" gorm:"type:varchar(16);not null;default:'legacy'"`
	FiveHourAmount        int64  `json:"five_hour_amount" gorm:"type:bigint;not null;default:0"`
	FiveHourWindowSeconds int64  `json:"five_hour_window_seconds" gorm:"type:bigint;not null;default:0"`
	WeeklyAmount          int64  `json:"weekly_amount" gorm:"type:bigint;not null;default:0"`
	// TEXT is intentional: MySQL 5.7 does not allow a default on TEXT columns.
	QuotaWindows SubscriptionQuotaWindowList `json:"quota_windows" gorm:"type:text"`
	// quotaWindowsInvalid is set when a persisted row contains one or more
	// malformed quota-window entries.  It is deliberately not persisted or
	// exposed on the wire; malformed raw entries themselves are retained across
	// cache serialization so request admission keeps failing closed.
	quotaWindowsInvalid bool `json:"-" gorm:"-"`

	// Input aliases retained for clients that used earlier terminology. They
	// are folded into the canonical fields and are not database columns.
	FiveHourQuota int64 `json:"five_hour_quota,omitempty" gorm:"-"`
	WeeklyQuota   int64 `json:"weekly_quota,omitempty" gorm:"-"`
	FiveHourLimit int64 `json:"five_hour_limit,omitempty" gorm:"-"`
	WeeklyLimit   int64 `json:"weekly_limit,omitempty" gorm:"-"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

// NormalizeQuotaWindowMode accepts the public mode names and falls back to
// legacy for omitted/unknown values.  The fixed five_hour/weekly/dual modes
// are retained for wire compatibility; hour/day/month/year/custom (and their
// aliases) select the generic window representation.
func NormalizeQuotaWindowMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case SubscriptionQuotaWindowFiveHour:
		return SubscriptionQuotaWindowFiveHour
	case SubscriptionQuotaWindowWeekly:
		return SubscriptionQuotaWindowWeekly
	case SubscriptionQuotaWindowDual:
		return SubscriptionQuotaWindowDual
	case SubscriptionQuotaWindowGeneric:
		return SubscriptionQuotaWindowGeneric
	case SubscriptionQuotaWindowHour, "hourly", "hours", "5h", "5hour", "5_hours", "five-hours",
		SubscriptionQuotaWindowDay, "daily", "days",
		"week",
		SubscriptionQuotaWindowMonth, "monthly", "months",
		SubscriptionQuotaWindowYear, "yearly", "years", "annual", "annually",
		SubscriptionQuotaWindowCustom, "seconds", "second", "duration":
		return SubscriptionQuotaWindowGeneric
	default:
		return SubscriptionQuotaWindowLegacy
	}
}

// subscriptionQuotaWindowModeIsKnown distinguishes an omitted/legacy mode
// from an arbitrary persisted string. NormalizeQuotaWindowMode intentionally
// falls back to legacy for backwards compatibility, but using that fallback
// at an enforcement boundary would turn a corrupted fixed/generic policy into
// an unlimited legacy entitlement. Callers that read stored plans therefore
// use this predicate before applying the compatibility fallback.
func subscriptionQuotaWindowModeIsKnown(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" || raw == SubscriptionQuotaWindowLegacy {
		return true
	}
	return NormalizeQuotaWindowMode(raw) != SubscriptionQuotaWindowLegacy
}

func subscriptionResetPeriodIsKnown(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", SubscriptionResetNever, SubscriptionResetDaily,
		SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetYearly,
		"annual", "annually", SubscriptionResetCustom:
		return true
	default:
		return false
	}
}

// subscriptionQuotaPolicyScalarsValid validates the scalar compatibility
// modes after aliases/defaults have been folded. Generic windows are checked
// separately because their limits live in policy.windows. This guard is used
// when reading persisted rows as well as at issuance; controller validation
// alone cannot protect rows written by older migrations or direct SQL edits.
func subscriptionQuotaPolicyScalarsValid(policy subscriptionQuotaPolicy) bool {
	if policy.invalid {
		return false
	}
	switch policy.mode {
	case SubscriptionQuotaWindowLegacy:
		// TotalAmount==0 is the documented unlimited legacy form.
	case SubscriptionQuotaWindowFiveHour:
		if policy.fiveHourAmount <= 0 || policy.fiveHourWindowSeconds <= 0 ||
			policy.fiveHourWindowSeconds > MaxSubscriptionQuotaWindowSeconds {
			return false
		}
	case SubscriptionQuotaWindowWeekly:
		if policy.weeklyAmount <= 0 {
			return false
		}
	case SubscriptionQuotaWindowDual:
		if policy.fiveHourAmount <= 0 || policy.fiveHourWindowSeconds <= 0 ||
			policy.fiveHourWindowSeconds > MaxSubscriptionQuotaWindowSeconds || policy.weeklyAmount <= 0 {
			return false
		}
	case SubscriptionQuotaWindowGeneric:
		if len(policy.windows) == 0 {
			return false
		}
	default:
		return false
	}
	if policyTracksAggregate(policy) && policy.resetPeriod == SubscriptionResetCustom &&
		(policy.resetCustomSeconds <= 0 || policy.resetCustomSeconds > MaxSubscriptionQuotaWindowSeconds) {
		return false
	}
	return true
}

// NormalizeQuotaWindows folds compatibility aliases into canonical fields,
// infers a mode when only limits were supplied, and applies the five-hour
// default window. It is safe to call repeatedly.
func (p *SubscriptionPlan) NormalizeQuotaWindows() {
	if p == nil {
		return
	}
	// Capture malformed entries before applying the compatibility normalizer.
	// Rows loaded from storage are normalized in several places
	// (cache, API summaries, issuance); without this marker a generic plan
	// containing one bad entry could become an apparently valid, reduced
	// policy after the first read and accidentally admit unlimited usage.
	// Preserve an already-recorded invalid marker across repeated normalization
	// calls, including objects previously normalized by an older process that
	// filtered their malformed entries.
	rawWindowsInvalid := quotaWindowListHasInvalidEntry(p.QuotaWindows)
	p.quotaWindowsInvalid = p.quotaWindowsInvalid || rawWindowsInvalid
	// Never discard malformed entries.  The normalizer is called while plans
	// are loaded from the database, copied into API summaries, and serialized
	// into the plan cache.  Filtering a bad entry here would erase the only
	// durable evidence that the generic policy was corrupt; after a cache JSON
	// round-trip the in-memory invalid marker is gone and the remaining valid
	// entries could accidentally become an apparently usable (or unlimited)
	// policy.  Keep the original list intact whenever an invalid entry is
	// present.  quotaPolicyFromPlan() still fails closed by checking the list
	// (and the marker for the in-memory case), while a repaired row can be
	// normalized normally on the next read.
	if rawWindowsInvalid {
		if strings.TrimSpace(p.QuotaWindowMode) == "" ||
			NormalizeQuotaWindowMode(p.QuotaWindowMode) == SubscriptionQuotaWindowLegacy {
			// A non-empty malformed list itself opts the plan into the generic
			// representation.  Marking the mode explicitly prevents a later
			// round-trip from treating the list as legacy data.
			p.QuotaWindowMode = SubscriptionQuotaWindowGeneric
		}
		return
	}
	// If a caller carries an already-recorded marker but no longer has the raw
	// entries (for example an object normalized by an older process), retain a
	// generic fail-closed mode instead of reconstructing a legacy/unlimited
	// policy from scalar fields.  New malformed rows take the branch above and
	// preserve their raw entries, so this is only a compatibility safeguard.
	if p.quotaWindowsInvalid {
		p.QuotaWindowMode = SubscriptionQuotaWindowGeneric
		return
	}
	if p.FiveHourAmount <= 0 {
		if p.FiveHourQuota > 0 {
			p.FiveHourAmount = p.FiveHourQuota
		} else if p.FiveHourLimit > 0 {
			p.FiveHourAmount = p.FiveHourLimit
		}
	}
	if p.WeeklyAmount <= 0 {
		if p.WeeklyQuota > 0 {
			p.WeeklyAmount = p.WeeklyQuota
		} else if p.WeeklyLimit > 0 {
			p.WeeklyAmount = p.WeeklyLimit
		}
	}
	// Generic definitions take precedence over the fixed compatibility fields.
	p.QuotaWindows = NormalizeQuotaWindowList(p.QuotaWindows)
	if len(p.QuotaWindows) > 0 {
		p.QuotaWindowMode = SubscriptionQuotaWindowGeneric
		return
	}
	rawMode := strings.ToLower(strings.TrimSpace(p.QuotaWindowMode))
	// A direct period mode is a convenient compact form for administrators who
	// need only one daily/monthly/yearly/custom window.  Materialize it into the
	// generic list so enforcement and snapshots use one code path.
	if rawMode == SubscriptionQuotaWindowHour || rawMode == "hourly" || rawMode == "hours" ||
		rawMode == "5h" || rawMode == "5hour" || rawMode == "5_hours" || rawMode == "five-hours" ||
		rawMode == SubscriptionQuotaWindowDay || rawMode == "daily" || rawMode == "days" || rawMode == "week" ||
		rawMode == SubscriptionQuotaWindowMonth || rawMode == "monthly" || rawMode == "months" ||
		rawMode == SubscriptionQuotaWindowYear || rawMode == "yearly" || rawMode == "years" || rawMode == "annual" || rawMode == "annually" ||
		rawMode == SubscriptionQuotaWindowCustom || rawMode == "seconds" || rawMode == "second" || rawMode == "duration" {
		unit := rawMode
		duration := int64(0)
		if unit == "5h" || unit == "5hour" || unit == "5_hours" || unit == "five-hours" {
			unit = SubscriptionQuotaWindowHour
			p.FiveHourAmount = maxInt64(p.FiveHourAmount, p.TotalAmount)
			duration = DefaultFiveHourWindowSeconds
		} else {
			switch unit {
			case "hourly", "hours":
				unit = SubscriptionQuotaWindowHour
			case "daily", "days":
				unit = SubscriptionQuotaWindowDay
			case "week":
				unit = SubscriptionQuotaWindowWeek
			case "monthly", "months":
				unit = SubscriptionQuotaWindowMonth
			case "yearly", "years", "annual", "annually":
				unit = SubscriptionQuotaWindowYear
			case "seconds", "second", "duration":
				unit = SubscriptionQuotaWindowCustom
			}
		}
		amount := p.WeeklyAmount
		if amount <= 0 {
			amount = p.TotalAmount
		}
		if unit == SubscriptionQuotaWindowHour && p.FiveHourAmount > 0 {
			amount = p.FiveHourAmount
		}
		if duration <= 0 {
			if unit == SubscriptionQuotaWindowCustom {
				duration = p.QuotaResetCustomSeconds
				if duration <= 0 {
					duration = p.CustomSeconds
				}
			} else {
				duration = 0 // Normalize derives the unit's default duration.
			}
		}
		if amount > 0 {
			candidate := SubscriptionQuotaWindow{Type: unit, Amount: amount, WindowSeconds: duration}
			if unit == SubscriptionQuotaWindowCustom {
				candidate.ResetMode = SubscriptionQuotaWindowRolling
			}
			if normalized, ok := candidate.Normalize(); ok {
				p.QuotaWindows = SubscriptionQuotaWindowList{normalized}
				p.QuotaWindowMode = SubscriptionQuotaWindowGeneric
				return
			}
		}
	}
	if rawMode == SubscriptionQuotaWindowGeneric {
		// Be liberal when reading rows created by an early generic-window
		// migration that stored only the mode/scalar fields.
		unit := SubscriptionQuotaWindowWeek
		switch NormalizeResetPeriod(p.QuotaResetPeriod) {
		case SubscriptionResetDaily:
			unit = SubscriptionQuotaWindowDay
		case SubscriptionResetMonthly:
			unit = SubscriptionQuotaWindowMonth
		case SubscriptionResetYearly:
			unit = SubscriptionQuotaWindowYear
		case SubscriptionResetCustom:
			unit = SubscriptionQuotaWindowCustom
		}
		amount := p.WeeklyAmount
		if amount <= 0 {
			amount = p.TotalAmount
		}
		seconds := p.QuotaResetCustomSeconds
		if seconds <= 0 {
			seconds = p.CustomSeconds
		}
		if normalized, ok := (SubscriptionQuotaWindow{Type: unit, Amount: amount, WindowSeconds: seconds}).Normalize(); ok {
			p.QuotaWindows = SubscriptionQuotaWindowList{normalized}
			p.QuotaWindowMode = SubscriptionQuotaWindowGeneric
			return
		}
	}
	p.QuotaWindowMode = NormalizeQuotaWindowMode(p.QuotaWindowMode)
	if p.QuotaWindowMode == SubscriptionQuotaWindowLegacy {
		switch {
		case p.FiveHourAmount > 0 && p.WeeklyAmount > 0:
			p.QuotaWindowMode = SubscriptionQuotaWindowDual
		case p.FiveHourAmount > 0:
			p.QuotaWindowMode = SubscriptionQuotaWindowFiveHour
		case p.WeeklyAmount > 0:
			p.QuotaWindowMode = SubscriptionQuotaWindowWeekly
		}
	}
	if p.FiveHourAmount > 0 && p.FiveHourWindowSeconds <= 0 {
		p.FiveHourWindowSeconds = DefaultFiveHourWindowSeconds
	}
	// A limit explicitly described as weekly should reset weekly when omitted.
	if (p.QuotaWindowMode == SubscriptionQuotaWindowWeekly || p.QuotaWindowMode == SubscriptionQuotaWindowDual) &&
		NormalizeResetPeriod(p.QuotaResetPeriod) == SubscriptionResetNever {
		p.QuotaResetPeriod = SubscriptionResetWeekly
	}
}

// UsesQuotaWindows reports whether this plan opts into independent windows.
func (p *SubscriptionPlan) UsesQuotaWindows() bool {
	if p == nil {
		return false
	}
	p.NormalizeQuotaWindows()
	return p.QuotaWindowMode != SubscriptionQuotaWindowLegacy || len(p.QuotaWindows) > 0
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	group, ok := NormalizeSubscriptionPurchaseLimitGroup(p.PurchaseLimitGroup)
	if !ok {
		return errors.New("invalid subscription purchase limit group")
	}
	p.PurchaseLimitGroup = group
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(tx *gorm.DB) error {
	group, ok := NormalizeSubscriptionPurchaseLimitGroup(p.PurchaseLimitGroup)
	if !ok {
		return errors.New("invalid subscription purchase limit group")
	}
	p.PurchaseLimitGroup = group
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// BeforeDelete prevents a catalog row from being removed while a checkout is
// holding a reservation for it.  Issued subscriptions are snapshot-based and
// may safely outlive their source plan; pending orders are different because
// they still need the source plan to complete or release their reservation.
// Keep this guard at the model layer so direct/admin GORM deletes cannot leave
// ReservedCount permanently stranded.
func (p *SubscriptionPlan) BeforeDelete(tx *gorm.DB) error {
	if tx == nil || p == nil {
		return nil
	}
	// Keep plan CRUD usable during a rolling migration where the catalog table
	// may be created before subscription_orders.  Without this guard an update
	// or delete of an otherwise valid plan would fail solely because the order
	// table has not been added yet; in that state no pending reservation can
	// exist to protect.
	if !subscriptionOrdersTableExists(tx) {
		return nil
	}
	var count int64
	query := tx.Model(&SubscriptionOrder{}).Where("status = ?", common.TopUpStatusPending)
	if p.Id > 0 {
		query = query.Where("plan_id = ?", p.Id)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrSubscriptionPlanHasPendingOrders
	}
	return nil
}

// ParseSubscriptionPlanModelLimits 将逗号分隔的模型限制字符串解析为去重后的模型名称列表。
// 自动去除空白字符，跳过空字符串，保持顺序并按首次出现去重。
// 示例: "gpt-4, gpt-4o, gpt-4" => ["gpt-4", "gpt-4o"]
func ParseSubscriptionPlanModelLimits(modelLimits string) []string {
	parts := strings.Split(modelLimits, ",")
	models := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		modelName := strings.TrimSpace(part)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		models = append(models, modelName)
	}
	return models
}

// NormalizeSubscriptionPlanModelLimits 规范化模型限制字符串：去重、去空格、排序。
// 用于 AdminCreateSubscriptionPlan / AdminUpdateSubscriptionPlan 中保存前的数据清洗，
// 确保数据库中存储的格式始终一致。
func NormalizeSubscriptionPlanModelLimits(modelLimits string) string {
	return strings.Join(ParseSubscriptionPlanModelLimits(modelLimits), ",")
}

// AllowsModel 判断该套餐是否允许使用指定模型。
// 规则：
//   - 套餐为 nil 时返回 false
//   - ModelLimits 为空（白名单为空）时返回 true，表示不限制模型
//   - ModelLimits 非空时，仅在白名单中匹配到 modelName 时返回 true
//   - modelName 为空字符串时返回 false
//
// 该方法在 PreConsumeUserSubscription 中被调用，用于决定是否从该订阅中扣费。

// VisibleInProvider 判断套餐对指定 provider_id 是否可见/适用。
// 规则很简单：套餐的 ProviderId 必须与请求上下文中的 provider_id 完全相等。
// 主站套餐 ProviderId=0，只能被主站(provider_id=0)用户看到/订阅；
// 服务商私有套餐只能被对应服务商站点的用户看到/订阅。
// 用于 ensureSubscriptionPlanPurchasable 中的越权订阅拦截。
func (p *SubscriptionPlan) VisibleInProvider(providerId int) bool {
	if p == nil {
		return false
	}
	return p.ProviderId == providerId
}

// ListVisibleSubscriptionPlans 查询指定 provider_id 下已启用的套餐列表，按 sort_order、id 倒序。
// 主站(provider_id=0)取主站套餐，服务商站点取其私有套餐，实现套餐按服务商隔离展示。
// 被 controller.GetSubscriptionPlans 调用。
func ListVisibleSubscriptionPlans(providerId int) ([]SubscriptionPlan, error) {
	var plans []SubscriptionPlan
	err := DB.
		Where("enabled = ? AND provider_id = ?", true, providerId).
		Order("sort_order desc, id desc").
		Find(&plans).Error
	for i := range plans {
		plans[i].NormalizeQuotaWindows()
	}
	return plans, err
}

// ListProviderSubscriptionPlanModels 列出某服务商可加入套餐模型白名单的候选模型名称。
// 数据来源：provider_model_pricing 表中该服务商已启用(enabled=true)的 public_model_name。
// 处理：去空白、去重、按字母升序返回。providerId<=0(主站)时返回空列表（主站无此约束）。
// 被 controller.ProviderListSubscriptionPlanModels 调用，前端用于模型多选下拉。
func ListProviderSubscriptionPlanModels(providerId int) ([]string, error) {
	if providerId <= 0 {
		return []string{}, nil
	}
	var rows []ProviderModelPricing
	if err := DB.
		Select("public_model_name").
		Where("provider_id = ? AND enabled = ?", providerId, true).
		Order("public_model_name asc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		modelName := strings.TrimSpace(row.PublicModelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		result = append(result, modelName)
	}
	return result, nil
}

// SubscriptionPlanModelsAllowedForProvider 校验套餐模型白名单是否全部来自指定服务商的模型广场。
// 返回 (ok, missing, err)：
//   - providerId<=0 或未配置白名单时直接放行(ok=true)，因为主站套餐不做模型来源约束；
//   - 否则取该服务商可上架模型集合，逐个比对白名单，收集不在集合中的模型到 missing；
//   - ok = (len(missing)==0)，missing 用于前端展示"哪些模型不合规"。
//
// 被 controller.validateSubscriptionPlanModelLimitsForProvider 调用。
func SubscriptionPlanModelsAllowedForProvider(providerId int, modelLimits string) (bool, []string, error) {
	limits := ParseSubscriptionPlanModelLimits(modelLimits)
	if providerId <= 0 || len(limits) == 0 {
		return true, nil, nil
	}
	allowed, err := ListProviderSubscriptionPlanModels(providerId)
	if err != nil {
		return false, nil, err
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, modelName := range allowed {
		allowedSet[modelName] = struct{}{}
	}
	missing := make([]string, 0)
	for _, modelName := range limits {
		if _, ok := allowedSet[modelName]; !ok {
			missing = append(missing, modelName)
		}
	}
	return len(missing) == 0, missing, nil
}

func (p *SubscriptionPlan) AllowsModel(modelName string) bool {
	if p == nil {
		return false
	}
	limits := ParseSubscriptionPlanModelLimits(p.ModelLimits)
	if len(limits) == 0 {
		return true
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	for _, allowed := range limits {
		if allowed == modelName {
			return true
		}
	}
	return false
}

// Subscription order (payment -> webhook -> create UserSubscription)
type SubscriptionOrder struct {
	Id              int     `json:"id"`
	UserId          int     `json:"user_id" gorm:"index"`
	PlanId          int     `json:"plan_id" gorm:"index"`
	ProviderId      int     `json:"provider_id" gorm:"type:int;not null;default:0;index"` // 订单归属服务商 ID（0=主站订单，>0=服务商私有套餐订单，用于后续给服务商所有者结算订阅收入）
	Money           float64 `json:"money"`
	Currency        string  `json:"currency" gorm:"type:varchar(10);default:''"`        // 币种符号（￥/$）
	OriginalMoney   float64 `json:"original_money" gorm:"type:decimal(18,6);default:0"` // 用户实际支付的原始金额（用户币种）
	TradeNo         string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	// PaymentProductId is the immutable gateway product/price identifier used
	// when this checkout was created.  It is intentionally snapshotted on the
	// order (rather than read from the mutable plan during webhook handling),
	// so an administrator can edit a plan without invalidating an in-flight
	// payment.  For Stripe this stores the Price ID; for Creem and other
	// product-based gateways it stores the Product ID.
	PaymentProductId string `json:"payment_product_id" gorm:"type:varchar(128);default:''"`
	// PlanSnapshot stores the immutable entitlement catalog at checkout time.
	// It is populated from the authoritative plan row by
	// CreateSubscriptionOrderTx, never trusted from a client payload. This
	// prevents edits to duration, model limits, or quota windows from changing
	// an already-paid order's entitlement.
	PlanSnapshot   string `json:"plan_snapshot,omitempty" gorm:"type:text"`
	Status         string `json:"status"`
	CreateTime     int64  `json:"create_time"`
	CompleteTime   int64  `json:"complete_time"`
	StockStatus    string `json:"stock_status" gorm:"type:varchar(16);not null;default:'';index"`
	StockExpiresAt int64  `json:"stock_expires_at" gorm:"type:bigint;not null;default:0;index"`

	ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}

// ErrSubscriptionPlanSnapshotInvalid is returned when an order contains a
// malformed or identity-mismatched entitlement snapshot.  A non-empty
// snapshot is treated as authoritative for the order; silently falling back to
// the mutable catalog would allow a corrupted/forged order to receive a
// different entitlement than the one paid for.
var ErrSubscriptionPlanSnapshotInvalid = errors.New("invalid subscription plan snapshot")

// ErrSubscriptionTopUpMismatch indicates that a TopUp row already exists for
// a subscription trade number but does not belong to the same subscription
// order.  A trade number is the idempotency key shared by payment mirrors and
// gateway callbacks; silently overwriting a row from another business would
// corrupt its status/owner (and could make a normal recharge look like a
// subscription).  Callers should leave the transaction rolled back and
// investigate the collision instead of trying to repair it implicitly.
var ErrSubscriptionTopUpMismatch = errors.New("subscription topup identity mismatch")

// snapshotSubscriptionPlan serializes the authoritative catalog row captured
// at checkout time.  We intentionally keep the complete plan-shaped payload
// (rather than a handful of fields) so newly added entitlement fields remain
// immutable across the payment boundary.  Inventory counters/timestamps are
// excluded because they describe mutable catalog state and are never used when
// issuing from a snapshot.
func snapshotSubscriptionPlan(plan *SubscriptionPlan) (string, error) {
	if plan == nil || plan.Id <= 0 {
		return "", ErrSubscriptionPlanSnapshotInvalid
	}
	copyPlan := *plan
	copyPlan.NormalizeQuotaWindows()
	copyPlan.IssuedCount = 0
	copyPlan.ReservedCount = 0
	copyPlan.CreatedAt = 0
	copyPlan.UpdatedAt = 0
	if quotaPolicyFromPlan(&copyPlan).invalid {
		return "", invalidSubscriptionQuotaPolicyError()
	}
	b, err := common.Marshal(&copyPlan)
	if err != nil {
		return "", fmt.Errorf("marshal subscription plan snapshot: %w", err)
	}
	return string(b), nil
}

// decodeSubscriptionPlanSnapshot validates and decodes an order's immutable
// entitlement payload.  Empty snapshots are represented by nil for backwards
// compatibility with orders created before snapshot support was introduced.
func decodeSubscriptionPlanSnapshot(raw string, planID, providerID int) (*SubscriptionPlan, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var snapshot SubscriptionPlan
	if err := common.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSubscriptionPlanSnapshotInvalid, err)
	}
	if snapshot.Id <= 0 || snapshot.Id != planID {
		return nil, ErrSubscriptionPlanSnapshotInvalid
	}
	if snapshot.ProviderId != providerID {
		return nil, ErrSubscriptionPlanSnapshotInvalid
	}
	// Normalize before policy/duration validation.  This also folds aliases
	// written by early clients into the canonical five-hour/weekly fields.
	snapshot.NormalizeQuotaWindows()
	if _, err := calcPlanEndTime(time.Unix(0, 0), &snapshot); err != nil {
		return nil, ErrSubscriptionPlanSnapshotInvalid
	}
	if quotaPolicyFromPlan(&snapshot).invalid {
		return nil, ErrSubscriptionPlanSnapshotInvalid
	}
	return &snapshot, nil
}

func (o *SubscriptionOrder) Insert() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		return CreateSubscriptionOrderTx(tx, o)
	})
}

// CreateSubscriptionOrderWithTopUp 创建订阅订单，并在同一事务中同步创建或更新对应的充值记录。
func CreateSubscriptionOrderWithTopUp(order *SubscriptionOrder) error {
	if order == nil {
		return errors.New("subscription order is nil")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := CreateSubscriptionOrderTx(tx, order); err != nil {
			return err
		}
		return upsertSubscriptionTopUpTx(tx, order)
	})
}

func (o *SubscriptionOrder) Update() error {
	if o == nil || o.Id <= 0 {
		return errors.New("invalid subscription order")
	}
	if DB == nil {
		return errors.New("database is not initialized")
	}
	// Subscription orders have a small state machine: a pending/reserved
	// order can only become issued through CompleteSubscriptionOrder, and a
	// pending/reserved order can only become released through
	// ExpireSubscriptionOrder.  A plain Save used to let callers change
	// status/stock_status without adjusting the plan counters, which could
	// permanently leak (or mint) inventory.  Keep Update for legacy metadata
	// updates, but reject attempts to mutate either state field here.
	return DB.Transaction(func(tx *gorm.DB) error {
		var current SubscriptionOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", o.Id).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSubscriptionOrderNotFound
			}
			return err
		}
		if o.Status != current.Status {
			return ErrSubscriptionOrderStatusInvalid
		}
		if o.StockStatus != current.StockStatus {
			return ErrSubscriptionStockInvalid
		}
		// Stock expiry is part of reservation state as well.  It may be extended
		// only by the dedicated checkout creation path; allowing a stale Update
		// to overwrite it would make an order effectively un-expirable.
		if o.StockExpiresAt != current.StockExpiresAt {
			return ErrSubscriptionStockInvalid
		}
		// Preserve immutable identity/state columns from the locked row while
		// retaining the historical Save semantics for non-state metadata.  This
		// also prevents a stale object from reverting a concurrently updated
		// provider payload or completion timestamp.
		o.Id = current.Id
		o.UserId = current.UserId
		o.PlanId = current.PlanId
		o.ProviderId = current.ProviderId
		o.Money = current.Money
		o.Currency = current.Currency
		o.OriginalMoney = current.OriginalMoney
		o.TradeNo = current.TradeNo
		o.PaymentMethod = current.PaymentMethod
		o.PaymentProvider = current.PaymentProvider
		o.PaymentProductId = current.PaymentProductId
		o.PlanSnapshot = current.PlanSnapshot
		o.Status = current.Status
		o.CreateTime = current.CreateTime
		o.CompleteTime = current.CompleteTime
		o.StockStatus = current.StockStatus
		o.StockExpiresAt = current.StockExpiresAt
		return tx.Save(o).Error
	})
}

func GetSubscriptionOrderByTradeNo(tradeNo string) *SubscriptionOrder {
	if tradeNo == "" {
		return nil
	}
	var order SubscriptionOrder
	if err := DB.Where("trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

// applyProviderSubscriptionIncomeTx 在订阅订单完成事务内，为服务商私有套餐订单结算订阅收入：
// 把用户支付的金额按 QuotaPerUnit 换算成额度，发放给该服务商的"所有者用户"(provider.owner_user_id)。
//
// 入账逻辑：
//  1. 仅当 order.ProviderId>0 且 order.Id>0 时才处理（主站订单不分账）；
//  2. incomeQuota = order.Money × QuotaPerUnit，<=0 则跳过；
//  3. 用固定 tradeNo "PROVIDER-SUBSCRIPTION-{orderId}" 做幂等键：若已有对应 TopUp 记录，说明已入账过，直接返回已入账的 userId(不重复发钱)；
//  4. 查 provider.owner_user_id，必须 >0，否则报错；
//  5. 给该 owner 用户的 quota 字段原子加 incomeQuota，并写入一条 PaymentMethod=provider_subscription 的 TopUp 流水；
//  6. 返回 (ownerUserId, incomeQuota, created, err)，调用方据此在事务外更新缓存与日志。
//
// 注意：该函数在事务内调用，DB 操作要么全成功要么全回滚；幂等性靠 tradeNo 唯一保证，不会因回调重复而重复发钱。
func applyProviderSubscriptionIncomeTx(tx *gorm.DB, order *SubscriptionOrder) (int, int, bool, error) {
	if tx == nil || order == nil || order.ProviderId <= 0 || order.Id <= 0 {
		return 0, 0, false, nil
	}
	incomeQuota := int(decimal.NewFromFloat(order.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
	if incomeQuota <= 0 {
		return 0, 0, false, nil
	}
	// 幂等键：每个订阅订单最多生成一条服务商收入流水，避免重复回调重复入账。
	tradeNo := fmt.Sprintf("PROVIDER-SUBSCRIPTION-%d", order.Id)
	var provider Provider
	if err := tx.Select("id", "owner_user_id").Where("id = ?", order.ProviderId).First(&provider).Error; err != nil {
		return 0, 0, false, err
	}
	if provider.OwnerUserId <= 0 {
		return 0, 0, false, errors.New("provider owner user id is empty")
	}
	var existing TopUp
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("trade_no = ?", tradeNo).First(&existing).Error; err == nil {
		// The income trade number is also an idempotency key.  It must never be
		// treated as already settled when a colliding row belongs to another
		// user/provider/business.  In particular, returning an arbitrary row's
		// UserId here would skip the owner credit while making the callback look
		// successful.
		if existing.UserId != provider.OwnerUserId ||
			(existing.ProviderId != 0 && existing.ProviderId != order.ProviderId) ||
			(strings.TrimSpace(existing.PaymentMethod) != "" && existing.PaymentMethod != TopUpPaymentMethodProviderSubscription) ||
			(strings.TrimSpace(existing.PaymentProvider) != "" && strings.TrimSpace(order.PaymentProvider) != "" && !strings.EqualFold(existing.PaymentProvider, order.PaymentProvider)) ||
			(strings.TrimSpace(existing.BizType) != "" && existing.BizType != TopUpBizTypePayment) ||
			(existing.SourceID != 0 && existing.SourceID != order.Id) ||
			existing.Status != common.TopUpStatusSuccess {
			return 0, 0, false, ErrSubscriptionTopUpMismatch
		}
		// 已存在收入流水，视为本次"未新增入账"，返回已记录的 userId，created=false。
		return existing.UserId, 0, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, false, err
	}
	// 原子加额度，避免并发回调时额度丢失。
	if err := tx.Model(&User{}).Where("id = ?", provider.OwnerUserId).Update("quota", gorm.Expr("quota + ?", incomeQuota)).Error; err != nil {
		return 0, 0, false, err
	}
	now := common.GetTimestamp()
	// 记录一条服务商订阅收入流水，PaymentMethod=provider_subscription，
	// 便于在充值流水中与 provider_profit(分润) 区分，并在账单/报表中聚合展示。
	topUp := &TopUp{
		ProviderId:      order.ProviderId,
		UserId:          provider.OwnerUserId,
		Amount:          int64(incomeQuota),
		Money:           order.Money,
		TradeNo:         tradeNo,
		PaymentMethod:   TopUpPaymentMethodProviderSubscription,
		PaymentProvider: order.PaymentProvider,
		BizType:         TopUpBizTypePayment,
		SourceID:        order.Id,
		CreateTime:      now,
		CompleteTime:    now,
		Status:          common.TopUpStatusSuccess,
		Currency:        order.Currency,
		OriginalMoney:   order.OriginalMoney,
	}
	if topUp.Currency == "" {
		topUp.Currency = "USD"
	}
	if topUp.OriginalMoney == 0 {
		topUp.OriginalMoney = order.Money
	}
	if err := tx.Create(topUp).Error; err != nil {
		return 0, 0, false, err
	}
	return provider.OwnerUserId, incomeQuota, true, nil
}

// SubscriptionQuotaWindowSnapshot describes one independently enforced quota
// window for API consumers. A zero Limit means unlimited.
type SubscriptionQuotaWindowSnapshot struct {
	Type string `json:"type"`
	// Period/Duration/ResetMode preserve configured calendar semantics in API responses.
	Period        string  `json:"period,omitempty"`
	Duration      int64   `json:"duration,omitempty"`
	ResetMode     string  `json:"reset_mode,omitempty"`
	Name          string  `json:"name,omitempty"`
	Limit         int64   `json:"limit"`
	Used          int64   `json:"used"`
	Remaining     int64   `json:"remaining"`
	ResetAt       int64   `json:"reset_at"`
	WindowSeconds int64   `json:"window_seconds,omitempty"`
	UsedPercent   float64 `json:"used_percent"`
}

// User subscription instance
type UserSubscription struct {
	Id     int `json:"id"`
	UserId int `json:"user_id" gorm:"index;index:idx_user_sub_active,priority:1"`
	PlanId int `json:"plan_id" gorm:"index"`
	// ProviderId 用户订阅实例归属服务商 ID（0=主站，>0=服务商）。
	// 创建时从订单/用户 provider_id 继承，便于按服务商维度查询用户有效订阅、隔离计费。
	ProviderId int `json:"provider_id" gorm:"type:int;not null;default:0;index"`

	AmountTotal int64 `json:"amount_total" gorm:"type:bigint;not null;default:0"`
	AmountUsed  int64 `json:"amount_used" gorm:"type:bigint;not null;default:0"`

	StartTime int64  `json:"start_time" gorm:"bigint"`
	EndTime   int64  `json:"end_time" gorm:"bigint;index;index:idx_user_sub_active,priority:3"`
	Status    string `json:"status" gorm:"type:varchar(32);index;index:idx_user_sub_active,priority:2"` // active/expired/cancelled

	Source string `json:"source" gorm:"type:varchar(32);default:'order'"` // order/admin

	LastResetTime int64 `json:"last_reset_time" gorm:"type:bigint;default:0"`
	NextResetTime int64 `json:"next_reset_time" gorm:"type:bigint;default:0;index"`

	UpgradeGroup  string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`
	PrevUserGroup string `json:"prev_user_group" gorm:"type:varchar(64);default:''"`

	// Quota policy snapshot. These values are copied when the subscription is
	// issued so later plan edits do not change an existing customer's limits.
	QuotaWindowMode                 string                      `json:"quota_window_mode" gorm:"type:varchar(16);not null;default:'legacy'"`
	QuotaWindowModeSnapshot         string                      `json:"quota_window_mode_snapshot,omitempty" gorm:"-"`
	FiveHourAmount                  int64                       `json:"five_hour_amount" gorm:"type:bigint;not null;default:0"`
	FiveHourWindowSeconds           int64                       `json:"five_hour_window_seconds" gorm:"type:bigint;not null;default:0"`
	WeeklyAmount                    int64                       `json:"weekly_amount" gorm:"type:bigint;not null;default:0"`
	QuotaResetPeriodSnapshot        string                      `json:"quota_reset_period_snapshot" gorm:"type:varchar(16);default:''"`
	QuotaResetCustomSecondsSnapshot int64                       `json:"quota_reset_custom_seconds_snapshot" gorm:"type:bigint;default:0"`
	QuotaWindowsSnapshot            SubscriptionQuotaWindowList `json:"quota_windows_snapshot,omitempty" gorm:"type:text"`
	PlanPolicySnapshotVersion       int                         `json:"plan_policy_snapshot_version" gorm:"type:int;not null;default:0"`
	ModelLimitsSnapshot             string                      `json:"model_limits_snapshot,omitempty" gorm:"type:text"`
	PlanTitleSnapshot               string                      `json:"plan_title_snapshot,omitempty" gorm:"type:varchar(255);default:''"`

	// Derived usage fields are populated for API responses and are not stored.
	FiveHourUsed      int64                             `json:"five_hour_used" gorm:"-"`
	FiveHourRemaining int64                             `json:"five_hour_remaining" gorm:"-"`
	FiveHourResetAt   int64                             `json:"five_hour_reset_at" gorm:"-"`
	WeeklyUsed        int64                             `json:"weekly_used" gorm:"-"`
	WeeklyRemaining   int64                             `json:"weekly_remaining" gorm:"-"`
	WeeklyResetAt     int64                             `json:"weekly_reset_at" gorm:"-"`
	QuotaWindows      []SubscriptionQuotaWindowSnapshot `json:"quota_windows,omitempty" gorm:"-"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (s *UserSubscription) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	s.CreatedAt = now
	s.UpdatedAt = now
	return nil
}

func (s *UserSubscription) BeforeUpdate(tx *gorm.DB) error {
	s.UpdatedAt = common.GetTimestamp()
	return nil
}

// SubscriptionSummary 聚合用户订阅实例及其对应的套餐信息。
// Plan 字段为可选（omitempty），当套餐已被删除或查询不到时为 nil。
// 该结构用于 API 返回，前端可直接展示套餐标题、模型限制等信息而无需额外请求。
type SubscriptionSummary struct {
	Subscription *UserSubscription `json:"subscription"`
	Plan         *SubscriptionPlan `json:"plan,omitempty"`
}

func calcPlanEndTime(start time.Time, plan *SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}
	// time.Duration is stored as nanoseconds and overflows after roughly
	// 292 years.  Validate calendar and fixed-second durations before doing
	// any multiplication so a malformed/admin-supplied value cannot wrap into
	// an already-expired (or otherwise unrelated) subscription timestamp.
	const maxDurationSeconds int64 = int64(^uint64(0)>>1) / int64(time.Second)
	checkedUnixAdd := func(base, delta int64) (int64, bool) {
		const maxInt64 = int64(^uint64(0) >> 1)
		const minInt64 = -maxInt64 - 1
		if delta > 0 && base > maxInt64-delta {
			return 0, false
		}
		if delta < 0 && base < minInt64-delta {
			return 0, false
		}
		return base + delta, true
	}
	positiveCalendarEnd := func(end time.Time) (int64, error) {
		endUnix, ok := checkedUnixAdd(start.Unix(), end.Unix()-start.Unix())
		if !ok || endUnix <= start.Unix() {
			return 0, errors.New("subscription duration overflows timestamp")
		}
		return endUnix, nil
	}
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		value := int64(plan.DurationValue)
		if value > maxDurationSeconds/(366*24*60*60) {
			return 0, errors.New("duration_value is too large")
		}
		return positiveCalendarEnd(start.AddDate(plan.DurationValue, 0, 0))
	case SubscriptionDurationMonth:
		value := int64(plan.DurationValue)
		if value > maxDurationSeconds/(31*24*60*60) {
			return 0, errors.New("duration_value is too large")
		}
		return positiveCalendarEnd(start.AddDate(0, plan.DurationValue, 0))
	case SubscriptionDurationDay:
		value := int64(plan.DurationValue)
		if value > maxDurationSeconds/(24*60*60) {
			return 0, errors.New("duration_value is too large")
		}
		seconds := value * (24 * 60 * 60)
		endUnix, ok := checkedUnixAdd(start.Unix(), seconds)
		if !ok {
			return 0, errors.New("subscription duration overflows timestamp")
		}
		return endUnix, nil
	case SubscriptionDurationHour:
		value := int64(plan.DurationValue)
		if value > maxDurationSeconds/(60*60) {
			return 0, errors.New("duration_value is too large")
		}
		seconds := value * (60 * 60)
		endUnix, ok := checkedUnixAdd(start.Unix(), seconds)
		if !ok {
			return 0, errors.New("subscription duration overflows timestamp")
		}
		return endUnix, nil
	case SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		if plan.CustomSeconds > maxDurationSeconds {
			return 0, errors.New("custom_seconds is too large")
		}
		endUnix, ok := checkedUnixAdd(start.Unix(), plan.CustomSeconds)
		if !ok {
			return 0, errors.New("subscription duration overflows timestamp")
		}
		return endUnix, nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func NormalizeResetPeriod(period string) string {
	switch strings.ToLower(strings.TrimSpace(period)) {
	case SubscriptionResetDaily:
		return SubscriptionResetDaily
	case SubscriptionResetWeekly:
		return SubscriptionResetWeekly
	case SubscriptionResetMonthly:
		return SubscriptionResetMonthly
	case SubscriptionResetYearly, "annual", "annually":
		return SubscriptionResetYearly
	case SubscriptionResetCustom:
		return SubscriptionResetCustom
	default:
		return SubscriptionResetNever
	}
}

func calcNextResetTime(base time.Time, plan *SubscriptionPlan, endUnix int64) int64 {
	if plan == nil {
		return 0
	}
	period := NormalizeResetPeriod(plan.QuotaResetPeriod)
	if period == SubscriptionResetNever {
		return 0
	}
	var next time.Time
	switch period {
	case SubscriptionResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, 1)
	case SubscriptionResetWeekly:
		// Align to next Monday 00:00
		weekday := int(base.Weekday()) // Sunday=0
		// Convert to Monday=1..Sunday=7
		if weekday == 0 {
			weekday = 7
		}
		daysUntil := 8 - weekday
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).
			AddDate(0, 0, daysUntil)
	case SubscriptionResetMonthly:
		// Align to first day of next month 00:00
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).
			AddDate(0, 1, 0)
	case SubscriptionResetYearly:
		// Align to first day of next year 00:00.
		if base.Year() >= 9998 {
			return 0
		}
		next = time.Date(base.Year()+1, time.January, 1, 0, 0, 0, 0, base.Location())
	case SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 || plan.QuotaResetCustomSeconds > MaxSubscriptionQuotaWindowSeconds {
			return 0
		}
		const maxInt64 = int64(^uint64(0) >> 1)
		if base.Unix() > maxInt64-plan.QuotaResetCustomSeconds {
			return 0
		}
		next = time.Unix(base.Unix()+plan.QuotaResetCustomSeconds, 0).In(base.Location())
	default:
		return 0
	}
	if next.Unix() <= base.Unix() {
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
}

type subscriptionQuotaWindowPolicy struct {
	unit          string
	amount        int64
	duration      int64
	windowSeconds int64
	resetMode     string
	name          string
}

type subscriptionQuotaPolicy struct {
	mode                  string
	fiveHourAmount        int64
	fiveHourWindowSeconds int64
	weeklyAmount          int64
	resetPeriod           string
	resetCustomSeconds    int64
	windows               []subscriptionQuotaWindowPolicy
	// invalid is set when a persisted generic policy cannot be normalized.
	// Such a policy must fail closed instead of silently becoming an unlimited
	// legacy policy (especially when TotalAmount is zero).
	invalid bool
}

func invalidSubscriptionQuotaPolicyError() error {
	return errors.New("invalid subscription quota policy")
}

func normalizedQuotaWindowPolicies(list SubscriptionQuotaWindowList) []subscriptionQuotaWindowPolicy {
	list = NormalizeQuotaWindowList(list)
	if len(list) == 0 {
		return nil
	}
	result := make([]subscriptionQuotaWindowPolicy, 0, len(list))
	for _, item := range list {
		normalized, ok := item.Normalize()
		if !ok || normalized.Amount <= 0 || normalized.WindowSeconds <= 0 {
			continue
		}
		result = append(result, subscriptionQuotaWindowPolicy{
			unit: normalized.Type, amount: normalized.Amount, duration: normalized.Duration,
			windowSeconds: normalized.WindowSeconds, resetMode: normalized.ResetMode, name: normalized.Name,
		})
	}
	return result
}

func quotaWindowListHasInvalidEntry(list SubscriptionQuotaWindowList) bool {
	for _, item := range list {
		if _, ok := item.Normalize(); !ok {
			return true
		}
	}
	return false
}

func quotaPolicyFromPlan(plan *SubscriptionPlan) subscriptionQuotaPolicy {
	if plan == nil {
		return subscriptionQuotaPolicy{mode: SubscriptionQuotaWindowLegacy}
	}
	// Scalar quota fields are persisted administrator data.  Negative values
	// must never be treated as "disabled" (the positive-only predicates below
	// would otherwise turn a corrupt limited plan into an unlimited one).
	if plan.TotalAmount < 0 || plan.FiveHourAmount < 0 || plan.FiveHourWindowSeconds < 0 ||
		plan.WeeklyAmount < 0 || plan.QuotaResetCustomSeconds < 0 {
		return subscriptionQuotaPolicy{mode: NormalizeQuotaWindowMode(plan.QuotaWindowMode), invalid: true}
	}
	rawMode := strings.ToLower(strings.TrimSpace(plan.QuotaWindowMode))
	if !subscriptionQuotaWindowModeIsKnown(rawMode) {
		// NormalizeQuotaWindowMode deliberately maps unknown values to legacy
		// for old API clients. A persisted row with an explicit unknown mode is
		// different: accepting it here could silently turn a malformed fixed or
		// generic policy into an unlimited legacy entitlement.
		return subscriptionQuotaPolicy{mode: SubscriptionQuotaWindowGeneric, invalid: true}
	}
	if !subscriptionResetPeriodIsKnown(plan.QuotaResetPeriod) {
		return subscriptionQuotaPolicy{mode: NormalizeQuotaWindowMode(plan.QuotaWindowMode), invalid: true}
	}
	// Validate persisted window entries before compatibility normalization.
	// Duplicate valid entries are still allowed and are
	// deduplicated by the normalizer; one invalid entry makes the policy
	// unusable so a bad edit cannot turn a limited plan into an unlimited one.
	rawWindowsInvalid := plan.quotaWindowsInvalid || quotaWindowListHasInvalidEntry(plan.QuotaWindows)
	copyPlan := *plan
	copyPlan.NormalizeQuotaWindows()
	policy := subscriptionQuotaPolicy{
		mode:                  copyPlan.QuotaWindowMode,
		fiveHourAmount:        copyPlan.FiveHourAmount,
		fiveHourWindowSeconds: copyPlan.FiveHourWindowSeconds,
		weeklyAmount:          copyPlan.WeeklyAmount,
		resetPeriod:           NormalizeResetPeriod(copyPlan.QuotaResetPeriod),
		resetCustomSeconds:    copyPlan.QuotaResetCustomSeconds,
		windows:               normalizedQuotaWindowPolicies(copyPlan.QuotaWindows),
	}
	if rawWindowsInvalid {
		policy.invalid = true
		return policy
	}
	if len(policy.windows) > 0 {
		policy.mode = SubscriptionQuotaWindowGeneric
		return policy
	}
	if NormalizeQuotaWindowMode(rawMode) == SubscriptionQuotaWindowGeneric || policy.mode == SubscriptionQuotaWindowGeneric {
		// A malformed/partially migrated generic row must not silently become an
		// unlimited subscription. Administrators must repair the row before it can
		// be issued or used.
		policy.mode = SubscriptionQuotaWindowGeneric
		policy.invalid = true
		return policy
	}
	if policy.fiveHourAmount > 0 && policy.fiveHourWindowSeconds <= 0 {
		policy.fiveHourWindowSeconds = DefaultFiveHourWindowSeconds
	}
	if (policy.mode == SubscriptionQuotaWindowWeekly || policy.mode == SubscriptionQuotaWindowDual) &&
		policy.weeklyAmount <= 0 {
		policy.weeklyAmount = copyPlan.TotalAmount
	}
	if !subscriptionQuotaPolicyScalarsValid(policy) {
		policy.invalid = true
	}
	return policy
}

func quotaPolicyForSubscription(sub *UserSubscription, plan *SubscriptionPlan) subscriptionQuotaPolicy {
	policy := quotaPolicyFromPlan(plan)
	if sub == nil {
		return policy
	}
	// Subscription snapshots are untrusted persisted data as well.  Reject
	// negative scalar values before mode detection can interpret them as an
	// omitted/disabled quota and accidentally grant unlimited usage.
	if sub.AmountTotal < 0 || sub.AmountUsed < 0 || sub.FiveHourAmount < 0 ||
		sub.FiveHourWindowSeconds < 0 || sub.WeeklyAmount < 0 ||
		sub.QuotaResetCustomSecondsSnapshot < 0 {
		policy.invalid = true
		return policy
	}
	// A non-empty snapshot always wins over the current plan. This makes plan
	// edits affect only newly issued subscriptions.
	if len(sub.QuotaWindowsSnapshot) > 0 {
		if quotaWindowListHasInvalidEntry(sub.QuotaWindowsSnapshot) {
			policy.windows = nil
			policy.mode = SubscriptionQuotaWindowGeneric
			policy.invalid = true
			return policy
		}
		policy.windows = normalizedQuotaWindowPolicies(sub.QuotaWindowsSnapshot)
		if len(policy.windows) > 0 {
			policy.mode = SubscriptionQuotaWindowGeneric
			policy.invalid = false
			return policy
		}
	}
	snapshotMode := strings.TrimSpace(sub.QuotaWindowMode)
	if snapshotMode == "" {
		snapshotMode = strings.TrimSpace(sub.QuotaWindowModeSnapshot)
	}
	if !subscriptionQuotaWindowModeIsKnown(snapshotMode) {
		policy.mode = SubscriptionQuotaWindowGeneric
		policy.invalid = true
		return policy
	}
	if sub.PlanPolicySnapshotVersion > 0 && !subscriptionResetPeriodIsKnown(sub.QuotaResetPeriodSnapshot) {
		policy.mode = SubscriptionQuotaWindowGeneric
		policy.invalid = true
		return policy
	}
	// Versioned subscriptions must never fall back to the live catalog policy,
	// even when the snapshotted plan used the legacy (scalar) mode and therefore
	// has no five-hour/weekly fields populated.  The previous condition treated
	// that perfectly valid legacy snapshot as "missing" and returned `policy`
	// from the current plan; editing a legacy plan to dual/generic mode then
	// silently changed an already-issued customer's entitlement.  Only
	// unversioned historical rows are allowed to inherit the current plan.
	if sub.PlanPolicySnapshotVersion <= 0 && (snapshotMode == "" ||
		(NormalizeQuotaWindowMode(snapshotMode) == SubscriptionQuotaWindowLegacy &&
			sub.FiveHourAmount <= 0 && sub.WeeklyAmount <= 0 && sub.QuotaResetPeriodSnapshot == "")) {
		return policy
	}
	if snapshotMode == "" {
		// A versioned row with an omitted mode represents the original legacy
		// aggregate policy.  Treat it as such and keep the snapshot fail-safe.
		snapshotMode = SubscriptionQuotaWindowLegacy
	}
	// We are about to use the scalar snapshot fields.  Do not retain generic
	// windows inherited from the current plan; doing so would let a later plan
	// edit alter an already-issued subscription.
	policy.windows = nil
	policy.mode = NormalizeQuotaWindowMode(snapshotMode)
	policy.invalid = false
	// A generic mode without a persisted list can still be reconstructed from
	// the snapshotted scalar fields (for example rows written during an
	// intermediate migration).  Prefer the snapshot's reset period as the
	// source of the unit.
	if policy.mode == SubscriptionQuotaWindowGeneric && len(policy.windows) == 0 {
		amount := sub.WeeklyAmount
		if amount <= 0 {
			amount = sub.AmountTotal
		}
		unit := SubscriptionQuotaWindowWeek
		if sub.QuotaResetPeriodSnapshot != "" {
			switch NormalizeResetPeriod(sub.QuotaResetPeriodSnapshot) {
			case SubscriptionResetDaily:
				unit = SubscriptionQuotaWindowDay
			case SubscriptionResetMonthly:
				unit = SubscriptionQuotaWindowMonth
			case SubscriptionResetYearly:
				unit = SubscriptionQuotaWindowYear
			case SubscriptionResetCustom:
				unit = SubscriptionQuotaWindowCustom
			}
		}
		seconds := sub.QuotaResetCustomSecondsSnapshot
		candidate := SubscriptionQuotaWindow{Type: unit, Amount: amount, WindowSeconds: seconds}
		if normalized, ok := candidate.Normalize(); ok {
			policy.windows = []subscriptionQuotaWindowPolicy{{
				unit: normalized.Type, amount: normalized.Amount, duration: normalized.Duration,
				windowSeconds: normalized.WindowSeconds, resetMode: normalized.ResetMode, name: normalized.Name,
			}}
		}
		if len(policy.windows) == 0 {
			policy.mode = SubscriptionQuotaWindowGeneric
			policy.invalid = true
		}
	}
	policy.fiveHourAmount = sub.FiveHourAmount
	policy.fiveHourWindowSeconds = sub.FiveHourWindowSeconds
	policy.weeklyAmount = sub.WeeklyAmount
	if (policy.mode == SubscriptionQuotaWindowWeekly || policy.mode == SubscriptionQuotaWindowDual) &&
		policy.weeklyAmount <= 0 {
		policy.weeklyAmount = sub.AmountTotal
	}
	if sub.QuotaResetPeriodSnapshot != "" {
		policy.resetPeriod = NormalizeResetPeriod(sub.QuotaResetPeriodSnapshot)
		policy.resetCustomSeconds = sub.QuotaResetCustomSecondsSnapshot
	} else if sub.PlanPolicySnapshotVersion > 0 {
		// Empty is an intentional snapshot of "never" for legacy plans; do not
		// retain a reset period from a later-edited catalog row.
		policy.resetPeriod = SubscriptionResetNever
		policy.resetCustomSeconds = 0
	}
	if policy.fiveHourAmount > 0 && policy.fiveHourWindowSeconds <= 0 {
		policy.fiveHourWindowSeconds = DefaultFiveHourWindowSeconds
	}
	if !subscriptionQuotaPolicyScalarsValid(policy) {
		policy.invalid = true
	}
	return policy
}

func policyHasGenericWindows(policy subscriptionQuotaPolicy) bool {
	return len(policy.windows) > 0
}

// policyTracksAggregate reports whether AmountUsed is the authoritative
// counter for this policy.  Generic windows derive usage from request records
// and must never be mixed with the legacy aggregate, otherwise a request would
// be charged twice or a reset could erase a rolling window unexpectedly.
func policyTracksAggregate(policy subscriptionQuotaPolicy) bool {
	return policy.mode == SubscriptionQuotaWindowLegacy || policyHasWeekly(policy)
}

func policyWindowList(policy subscriptionQuotaPolicy) []subscriptionQuotaWindowPolicy {
	if policyHasGenericWindows(policy) {
		return policy.windows
	}
	windows := make([]subscriptionQuotaWindowPolicy, 0, 2)
	if policyHasFiveHour(policy) {
		windows = append(windows, subscriptionQuotaWindowPolicy{
			unit: SubscriptionQuotaWindowHour, amount: policy.fiveHourAmount,
			duration: 5, windowSeconds: policy.fiveHourWindowSeconds,
			resetMode: SubscriptionQuotaWindowRolling,
		})
	}
	if policyHasWeekly(policy) {
		seconds := int64(7 * 24 * 60 * 60)
		resetMode := SubscriptionQuotaWindowCalendar
		if policy.resetPeriod == SubscriptionResetCustom && policy.resetCustomSeconds > 0 {
			seconds = policy.resetCustomSeconds
			resetMode = SubscriptionQuotaWindowRolling
		}
		windows = append(windows, subscriptionQuotaWindowPolicy{
			unit: SubscriptionQuotaWindowWeek, amount: policy.weeklyAmount,
			duration: 1, windowSeconds: seconds, resetMode: resetMode,
		})
	}
	return windows
}

// aggregateRecordInCurrentPeriod determines whether a request created at
// recordTime still belongs to the persisted aggregate period.  It prevents a
// late settlement/refund from subtracting last week's usage from the current
// week's counter after a reset has already run.
func aggregateRecordInCurrentPeriod(sub *UserSubscription, policy subscriptionQuotaPolicy, recordTime, now int64) bool {
	if sub == nil || !policyTracksAggregate(policy) || recordTime <= 0 {
		return false
	}
	if policy.resetPeriod == SubscriptionResetNever {
		return true
	}
	if sub.LastResetTime > 0 && recordTime < sub.LastResetTime {
		return false
	}
	if sub.NextResetTime > 0 && recordTime >= sub.NextResetTime && sub.NextResetTime <= now {
		return false
	}
	return recordTime <= now
}

// aggregateQuotaPeriodBounds resolves the persisted aggregate period that
// contains recordTime. Unlike generic quota windows, legacy/fixed weekly
// periods may begin with a partial calendar bucket at subscription issuance.
func aggregateQuotaPeriodBounds(sub *UserSubscription, policy subscriptionQuotaPolicy, recordTime int64) (int64, int64, error) {
	if sub == nil || recordTime <= 0 {
		return 0, 0, errors.New("invalid subscription aggregate record time")
	}
	if sub.StartTime > 0 && recordTime < sub.StartTime {
		return 0, 0, errors.New("subscription aggregate record predates subscription")
	}
	if sub.EndTime > 0 && recordTime >= sub.EndTime {
		return 0, 0, errors.New("subscription aggregate record is outside subscription")
	}

	start := sub.StartTime
	end := sub.EndTime
	period := NormalizeResetPeriod(policy.resetPeriod)
	if period == SubscriptionResetNever {
		return start, end, nil
	}

	var calendarStart, calendarEnd time.Time
	switch period {
	case SubscriptionResetDaily:
		record := time.Unix(recordTime, 0)
		location := time.Unix(sub.StartTime, 0).Location()
		record = record.In(location)
		calendarStart = time.Date(record.Year(), record.Month(), record.Day(), 0, 0, 0, 0, location)
		calendarEnd = calendarStart.AddDate(0, 0, 1)
	case SubscriptionResetWeekly:
		record := time.Unix(recordTime, 0)
		location := time.Unix(sub.StartTime, 0).Location()
		record = record.In(location)
		weekday := int(record.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		calendarStart = time.Date(record.Year(), record.Month(), record.Day(), 0, 0, 0, 0, location).
			AddDate(0, 0, -(weekday - 1))
		calendarEnd = calendarStart.AddDate(0, 0, 7)
	case SubscriptionResetMonthly:
		record := time.Unix(recordTime, 0)
		location := time.Unix(sub.StartTime, 0).Location()
		record = record.In(location)
		calendarStart = time.Date(record.Year(), record.Month(), 1, 0, 0, 0, 0, location)
		calendarEnd = calendarStart.AddDate(0, 1, 0)
	case SubscriptionResetYearly:
		record := time.Unix(recordTime, 0)
		location := time.Unix(sub.StartTime, 0).Location()
		record = record.In(location)
		calendarStart = time.Date(record.Year(), time.January, 1, 0, 0, 0, 0, location)
		calendarEnd = calendarStart.AddDate(1, 0, 0)
	case SubscriptionResetCustom:
		seconds := policy.resetCustomSeconds
		if seconds <= 0 || seconds > MaxSubscriptionQuotaWindowSeconds {
			return 0, 0, errors.New("invalid subscription aggregate reset duration")
		}
		anchor := sub.StartTime
		// Do not let malformed timestamps wrap while assigning a historical
		// custom bucket.  A wrapped elapsed/start value could point at an
		// unrelated bucket and make settlement bypass the configured aggregate
		// limit.  Persisted subscriptions normally have positive Unix times,
		// but this helper is also used on legacy rows and must fail closed.
		elapsed, ok := subscriptionSafeSubInt64(recordTime, anchor)
		if !ok {
			return 0, 0, errors.New("subscription aggregate period overflows timestamp")
		}
		bucketOffset, ok := subscriptionSafeMulInt64(floorDivInt64(elapsed, seconds), seconds)
		if !ok {
			return 0, 0, errors.New("subscription aggregate period overflows timestamp")
		}
		start, ok = subscriptionSafeAddInt64(anchor, bucketOffset)
		if !ok {
			return 0, 0, errors.New("subscription aggregate period overflows timestamp")
		}
		end, ok = subscriptionSafeAddInt64(start, seconds)
		if !ok {
			end = int64(^uint64(0) >> 1)
		}
	default:
		return 0, 0, errors.New("invalid subscription aggregate reset period")
	}
	if period != SubscriptionResetCustom {
		start = calendarStart.Unix()
		end = calendarEnd.Unix()
		// time.Time can represent years far beyond the Unix int64 range, and
		// Unix() then wraps back into a negative value.  Reject a wrapped or
		// backwards calendar interval before the caller builds a SQL predicate;
		// otherwise the `end > 0` guard below would omit the upper bound and a
		// historical settlement could bypass its period quota.
		if end <= start {
			return 0, 0, errors.New("subscription aggregate period overflows timestamp")
		}
	}
	// The first aggregate period is partial when a subscription starts inside a
	// calendar bucket, and the final period ends with the subscription itself.
	if sub.StartTime > 0 && start < sub.StartTime {
		start = sub.StartTime
	}
	if sub.EndTime > 0 && (end <= 0 || end > sub.EndTime) {
		end = sub.EndTime
	}
	if end > 0 && (recordTime < start || recordTime >= end) {
		return 0, 0, errors.New("subscription aggregate period does not contain record")
	}
	return start, end, nil
}

func aggregateQuotaHistoricalTargetAllowedTx(tx *gorm.DB, sub *UserSubscription, policy subscriptionQuotaPolicy, record *SubscriptionPreConsumeRecord, target, now int64) (bool, error) {
	if tx == nil || sub == nil || record == nil {
		return false, errors.New("invalid subscription aggregate settlement")
	}
	limit := policyWeeklyLimit(policy, sub)
	if limit <= 0 {
		return true, nil
	}
	if target < 0 {
		return false, errors.New("actual amount must be >= 0")
	}
	if target > limit {
		return false, nil
	}
	start, end, err := aggregateQuotaPeriodBounds(sub, policy, record.CreatedAt)
	if err != nil {
		return false, err
	}
	query := tx.Where("user_subscription_id = ? AND status <> ? AND request_id <> ?", sub.Id, "refunded", record.RequestId).
		Where("created_at >= ? AND created_at <= ?", start, now)
	if end > 0 {
		query = query.Where("created_at < ?", end)
	}
	var records []SubscriptionPreConsumeRecord
	if err := query.Find(&records).Error; err != nil {
		return false, err
	}
	remainingForOthers := limit - target
	var used int64
	for i := range records {
		amount := effectivePreConsumeAmount(&records[i])
		if amount <= 0 {
			continue
		}
		if used > remainingForOthers || amount > remainingForOthers-used {
			return false, nil
		}
		used += amount
	}
	return true, nil
}
func policyHasFiveHour(policy subscriptionQuotaPolicy) bool {
	return (policy.mode == SubscriptionQuotaWindowFiveHour || policy.mode == SubscriptionQuotaWindowDual) &&
		policy.fiveHourAmount > 0 && policy.fiveHourWindowSeconds > 0
}

func policyHasWeekly(policy subscriptionQuotaPolicy) bool {
	return policy.mode == SubscriptionQuotaWindowWeekly || policy.mode == SubscriptionQuotaWindowDual
}

func policyWeeklyLimit(policy subscriptionQuotaPolicy, sub *UserSubscription) int64 {
	if sub == nil {
		return 0
	}
	if policy.mode == SubscriptionQuotaWindowLegacy {
		return sub.AmountTotal
	}
	if policyHasWeekly(policy) {
		return policy.weeklyAmount
	}
	return 0
}

func calcNextResetTimeForPolicy(base time.Time, policy subscriptionQuotaPolicy, endUnix int64) int64 {
	if policy.resetPeriod == SubscriptionResetNever {
		return 0
	}
	plan := &SubscriptionPlan{
		QuotaResetPeriod:        policy.resetPeriod,
		QuotaResetCustomSeconds: policy.resetCustomSeconds,
	}
	return calcNextResetTime(base, plan, endUnix)
}

func floorDivInt64(value, divisor int64) int64 {
	if divisor <= 0 {
		return 0
	}
	quotient := value / divisor
	if value%divisor < 0 {
		quotient--
	}
	return quotient
}

// subscriptionSafeAddInt64 and subscriptionSafeMulInt64 are used by quota
// window arithmetic.  Window definitions normally pass through Normalize,
// but persisted rows and internal callers can still contain malformed values;
// signed overflow here would turn a future boundary into a date in the past
// and could incorrectly admit unlimited requests.
func subscriptionSafeAddInt64(a, b int64) (int64, bool) {
	const maxInt64 = int64(^uint64(0) >> 1)
	const minInt64 = -maxInt64 - 1
	if b > 0 && a > maxInt64-b {
		return 0, false
	}
	if b < 0 && a < minInt64-b {
		return 0, false
	}
	return a + b, true
}

// subscriptionSafeSubInt64 is the subtraction counterpart used when deriving
// rolling-window lower bounds. Keeping the operation in one helper avoids a
// signed underflow at the int64 boundary (which would otherwise wrap into a
// future timestamp and make the usage query fail open).
func subscriptionSafeSubInt64(a, b int64) (int64, bool) {
	const minInt64 = -int64(^uint64(0)>>1) - 1
	if b == minInt64 {
		return 0, false
	}
	return subscriptionSafeAddInt64(a, -b)
}

func subscriptionSafeMulInt64(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	const minInt64 = -maxInt64 - 1
	if a == -1 {
		if b == minInt64 {
			return 0, false
		}
		return -b, true
	}
	if b == -1 {
		if a == minInt64 {
			return 0, false
		}
		return -a, true
	}
	if a > 0 {
		if b > 0 {
			if a > maxInt64/b {
				return 0, false
			}
		} else if b < minInt64/a {
			return 0, false
		}
	} else { // a < 0
		if b > 0 {
			if a < minInt64/b {
				return 0, false
			}
		} else if a < maxInt64/b {
			return 0, false
		}
	}
	return a * b, true
}

func subscriptionSafeWindowEnd(start, duration int64) int64 {
	end, ok := subscriptionSafeAddInt64(start, duration)
	if ok {
		return end
	}
	if duration >= 0 {
		return int64(^uint64(0) >> 1)
	}
	return -int64(^uint64(0)>>1) - 1
}

// isValidSubscriptionQuotaWindowUnit guards the arithmetic helpers against
// malformed rows assembled by older migrations or manually edited snapshots.
// Normalize() normally guarantees one of these units, but the low-level
// bounds/usage functions are also called while reading persisted data and
// therefore must not silently reinterpret an unknown unit as a rolling
// window.
func isValidSubscriptionQuotaWindowUnit(unit string) bool {
	switch unit {
	case SubscriptionQuotaWindowHour, SubscriptionQuotaWindowDay,
		SubscriptionQuotaWindowWeek, SubscriptionQuotaWindowMonth,
		SubscriptionQuotaWindowYear, SubscriptionQuotaWindowCustom:
		return true
	default:
		return false
	}
}

// quotaWindowBounds returns the current rolling boundary (start, now) or
// calendar interval [start,end). Rolling membership is (start,now].
func quotaWindowBounds(window subscriptionQuotaWindowPolicy, now, anchor int64) (int64, int64) {
	if now <= 0 {
		now = GetDBTimestamp()
	}
	// Unknown units/reset modes are invalid configuration.  A rolling window
	// does not need a unit at all (the exact windowSeconds value is sufficient),
	// and older in-memory/legacy callers intentionally omit it.  Preserve that
	// compatibility for an empty unit while still rejecting a non-empty unknown
	// unit and every malformed calendar window.  Normalized persisted policies
	// always carry a canonical unit, so this exception cannot make a malformed
	// stored policy admissible during quota enforcement.
	if (window.resetMode != SubscriptionQuotaWindowRolling && window.resetMode != SubscriptionQuotaWindowCalendar) ||
		(window.resetMode == SubscriptionQuotaWindowCalendar && !isValidSubscriptionQuotaWindowUnit(window.unit)) ||
		(window.resetMode == SubscriptionQuotaWindowRolling && window.unit != "" && !isValidSubscriptionQuotaWindowUnit(window.unit)) {
		return now, now
	}
	if window.windowSeconds <= 0 || window.windowSeconds > MaxSubscriptionQuotaWindowSeconds {
		return now, now
	}
	if window.resetMode != SubscriptionQuotaWindowCalendar {
		start, ok := subscriptionSafeAddInt64(now, -window.windowSeconds)
		if !ok {
			start = -int64(^uint64(0)>>1) - 1
		}
		return start, now
	}
	t := time.Unix(now, 0).UTC()
	count := window.duration
	if count <= 0 {
		count = 1
	}
	var start, end time.Time
	// Normalize enforces this relationship. Keep the check here as well for
	// snapshots/legacy rows assembled without Normalize, preventing period
	// multiplication and AddDate conversions from overflowing.
	maxCountForUnit := func(unit string) (int64, bool) {
		base := quotaWindowNominalUnitSeconds(unit)
		if base <= 0 {
			return 0, false
		}
		return MaxSubscriptionQuotaWindowSeconds / base, true
	}
	switch window.unit {
	case SubscriptionQuotaWindowHour:
		maxCount, ok := maxCountForUnit(window.unit)
		if !ok || count > maxCount {
			return now, now
		}
		period, ok := subscriptionSafeMulInt64(count, 60*60)
		if !ok || period <= 0 {
			return now, now
		}
		startUnix, ok := subscriptionSafeMulInt64(floorDivInt64(now, period), period)
		if !ok {
			return now, now
		}
		start = time.Unix(startUnix, 0).UTC()
		end = time.Unix(subscriptionSafeWindowEnd(startUnix, period), 0).UTC()
	case SubscriptionQuotaWindowDay:
		maxCount, ok := maxCountForUnit(window.unit)
		if !ok || count > maxCount {
			return now, now
		}
		period, ok := subscriptionSafeMulInt64(count, 24*60*60)
		if !ok || period <= 0 {
			return now, now
		}
		startUnix, ok := subscriptionSafeMulInt64(floorDivInt64(now, period), period)
		if !ok {
			return now, now
		}
		start = time.Unix(startUnix, 0).UTC()
		end = time.Unix(subscriptionSafeWindowEnd(startUnix, period), 0).UTC()
	case SubscriptionQuotaWindowWeek:
		maxCount, ok := maxCountForUnit(window.unit)
		if !ok || count > maxCount {
			return now, now
		}
		period, ok := subscriptionSafeMulInt64(count, 7*24*60*60)
		if !ok || period <= 0 {
			return now, now
		}
		// 1970-01-05 is the first Monday after the Unix epoch. Use it as
		// the stable anchor so one-week windows are Monday-to-Monday.
		mondayAnchor := time.Date(1970, time.January, 5, 0, 0, 0, 0, time.UTC).Unix()
		delta, ok := subscriptionSafeAddInt64(now, -mondayAnchor)
		if !ok {
			return now, now
		}
		aligned, ok := subscriptionSafeMulInt64(floorDivInt64(delta, period), period)
		if !ok {
			return now, now
		}
		startUnix, ok := subscriptionSafeAddInt64(mondayAnchor, aligned)
		if !ok {
			return now, now
		}
		start = time.Unix(startUnix, 0).UTC()
		end = time.Unix(subscriptionSafeWindowEnd(startUnix, period), 0).UTC()
	case SubscriptionQuotaWindowMonth:
		maxCount, ok := maxCountForUnit(window.unit)
		if !ok || count > maxCount || count > int64(^uint(0)>>1) {
			return now, now
		}
		yearPart, ok := subscriptionSafeMulInt64(int64(t.Year()), 12)
		if !ok {
			return now, now
		}
		monthIndex, ok := subscriptionSafeAddInt64(yearPart, int64(t.Month())-1)
		if !ok {
			return now, now
		}
		startIndex, ok := subscriptionSafeMulInt64(floorDivInt64(monthIndex, count), count)
		if !ok {
			return now, now
		}
		startYear64 := startIndex / 12
		startMonthIndex := startIndex % 12
		if startMonthIndex < 0 {
			startMonthIndex += 12
			startYear64, ok = subscriptionSafeAddInt64(startYear64, -1)
			if !ok {
				return now, now
			}
		}
		maxInt := int64(^uint(0) >> 1)
		minInt := -maxInt - 1
		if startYear64 > maxInt || startYear64 < minInt {
			return now, now
		}
		startYear := int(startYear64)
		start = time.Date(startYear, time.Month(startMonthIndex+1), 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(0, int(count), 0)
	case SubscriptionQuotaWindowYear:
		maxCount, ok := maxCountForUnit(window.unit)
		if !ok || count > maxCount || count > int64(^uint(0)>>1) {
			return now, now
		}
		startYear64, ok := subscriptionSafeMulInt64(floorDivInt64(int64(t.Year()), count), count)
		if !ok {
			return now, now
		}
		maxInt := int64(^uint(0) >> 1)
		minInt := -maxInt - 1
		if startYear64 > maxInt || startYear64 < minInt {
			return now, now
		}
		startYear := int(startYear64)
		start = time.Date(startYear, time.January, 1, 0, 0, 0, 0, time.UTC)
		end = start.AddDate(int(count), 0, 0)
	case SubscriptionQuotaWindowCustom:
		// Custom calendar windows are anchored to the subscription issuance
		// timestamp. An absent/future anchor is malformed; returning an empty
		// interval lets callers fail closed instead of silently re-anchoring at
		// the settlement time and bypassing historical usage.
		if anchor <= 0 || anchor > now {
			return now, now
		}
		elapsed, ok := subscriptionSafeAddInt64(now, -anchor)
		if !ok {
			return now, now
		}
		aligned, ok := subscriptionSafeMulInt64(floorDivInt64(elapsed, window.windowSeconds), window.windowSeconds)
		if !ok {
			return now, now
		}
		startUnix, ok := subscriptionSafeAddInt64(anchor, aligned)
		if !ok {
			return now, now
		}
		start = time.Unix(startUnix, 0).UTC()
		end = time.Unix(subscriptionSafeWindowEnd(startUnix, window.windowSeconds), 0).UTC()
	default:
		startUnix, ok := subscriptionSafeAddInt64(now, -window.windowSeconds)
		if !ok {
			startUnix = -int64(^uint64(0)>>1) - 1
		}
		return startUnix, now
	}
	startUnix, endUnix := start.Unix(), end.Unix()
	if endUnix <= startUnix {
		// A date operation at the edge of time.Time's representable range can
		// normalize backwards.  Returning an empty interval is fail-closed for
		// callers that use these bounds to query usage.
		return now, now
	}
	return startUnix, endUnix
}

// quotaWindowEffectiveSeconds reports the actual length of the current
// interval for API snapshots.  Month/year calendar windows do not have a
// fixed 30/365-day length (and leap years/months vary), while rolling/custom
// windows do.  Enforcement continues to use quotaWindowBounds as the source
// of truth; this helper is presentation-only.
func quotaWindowEffectiveSeconds(window subscriptionQuotaWindowPolicy, now, anchor int64) int64 {
	if window.resetMode == SubscriptionQuotaWindowCalendar {
		start, end := quotaWindowBounds(window, now, anchor)
		if end > start {
			return end - start
		}
	}
	return window.windowSeconds
}

func quotaWindowContainsRecord(window subscriptionQuotaWindowPolicy, now, anchor, recordTime int64) bool {
	if recordTime <= 0 || recordTime > now {
		return false
	}
	if (window.resetMode != SubscriptionQuotaWindowRolling && window.resetMode != SubscriptionQuotaWindowCalendar) ||
		(window.resetMode == SubscriptionQuotaWindowCalendar && !isValidSubscriptionQuotaWindowUnit(window.unit)) ||
		(window.resetMode == SubscriptionQuotaWindowRolling && window.unit != "" && !isValidSubscriptionQuotaWindowUnit(window.unit)) ||
		window.windowSeconds <= 0 || window.windowSeconds > MaxSubscriptionQuotaWindowSeconds {
		return false
	}
	start, end := quotaWindowBounds(window, now, anchor)
	if window.resetMode == SubscriptionQuotaWindowCalendar {
		return recordTime >= start && recordTime < end
	}
	return recordTime > start && recordTime <= end
}

func recentSubscriptionWindowUsageTx(tx *gorm.DB, subscriptionID int, now int64, window subscriptionQuotaWindowPolicy, anchor int64) (int64, int64, error) {
	return recentSubscriptionWindowUsageExcludingTx(tx, subscriptionID, now, window, anchor, "")
}

// recentSubscriptionWindowUsageExcludingTx is used during settlement, where
// the request being settled is already present in the records table.
func recentSubscriptionWindowUsageExcludingTx(tx *gorm.DB, subscriptionID int, now int64, window subscriptionQuotaWindowPolicy, anchor int64, excludeRequestID string) (int64, int64, error) {
	if tx == nil || subscriptionID <= 0 {
		return 0, 0, nil
	}
	if window.windowSeconds <= 0 || window.windowSeconds > MaxSubscriptionQuotaWindowSeconds {
		return 0, 0, errors.New("invalid subscription quota window duration")
	}
	// Rolling windows can be represented by an exact duration alone; preserve
	// the empty-unit form used by legacy in-memory callers. Calendar windows
	// require a canonical unit, and any non-empty rolling unit must be known so
	// malformed data cannot be silently reinterpreted.
	if (window.resetMode == SubscriptionQuotaWindowCalendar && !isValidSubscriptionQuotaWindowUnit(window.unit)) ||
		(window.resetMode == SubscriptionQuotaWindowRolling && window.unit != "" && !isValidSubscriptionQuotaWindowUnit(window.unit)) {
		return 0, 0, errors.New("invalid subscription quota window unit")
	}
	if window.resetMode != SubscriptionQuotaWindowRolling && window.resetMode != SubscriptionQuotaWindowCalendar {
		return 0, 0, errors.New("invalid subscription quota window reset mode")
	}
	if window.unit == SubscriptionQuotaWindowCustom &&
		window.resetMode == SubscriptionQuotaWindowCalendar &&
		(anchor <= 0 || anchor > now) {
		return 0, 0, errors.New("invalid subscription quota window anchor")
	}
	start, end := quotaWindowBounds(window, now, anchor)
	var records []SubscriptionPreConsumeRecord
	query := tx.Where("user_subscription_id = ? AND status <> ?", subscriptionID, "refunded")
	if window.resetMode == SubscriptionQuotaWindowCalendar {
		query = query.Where("created_at >= ? AND created_at < ? AND created_at <= ?", start, end, now)
	} else {
		query = query.Where("created_at > ? AND created_at <= ?", start, now)
	}
	if strings.TrimSpace(excludeRequestID) != "" {
		query = query.Where("request_id <> ?", excludeRequestID)
	}
	if err := query.Find(&records).Error; err != nil {
		return 0, 0, err
	}
	var used int64
	resetAt := end
	if window.resetMode != SubscriptionQuotaWindowCalendar {
		resetAt = 0
	}
	for i := range records {
		amount := effectivePreConsumeAmount(&records[i])
		if amount <= 0 {
			continue
		}
		if used > int64(^uint64(0)>>1)-amount {
			used = int64(^uint64(0) >> 1)
		} else {
			used += amount
		}
		if window.resetMode != SubscriptionQuotaWindowCalendar {
			expires, ok := subscriptionSafeAddInt64(records[i].CreatedAt, window.windowSeconds)
			if !ok {
				// A record timestamp at the upper int64 boundary remains in the
				// rolling window for all representable future times.
				expires = int64(^uint64(0) >> 1)
			}
			if resetAt == 0 || expires < resetAt {
				resetAt = expires
			}
		}
	}
	return used, resetAt, nil
}

// recentSubscriptionUsageTx is retained for fixed five-hour callers.
func recentSubscriptionUsageTx(tx *gorm.DB, subscriptionID int, now int64, windowSeconds int64) (int64, int64, error) {
	return recentSubscriptionWindowUsageTx(tx, subscriptionID, now, subscriptionQuotaWindowPolicy{
		unit: SubscriptionQuotaWindowHour, amount: 0, duration: 5, windowSeconds: windowSeconds, resetMode: SubscriptionQuotaWindowRolling,
	}, 0)
}

func effectivePreConsumeAmount(record *SubscriptionPreConsumeRecord) int64 {
	if record == nil {
		return 0
	}
	if record.SettledAmount != nil {
		if *record.SettledAmount < 0 {
			return 0
		}
		return *record.SettledAmount
	}
	if record.PreConsumed < 0 {
		return 0
	}
	return record.PreConsumed
}

func quotaUsedPercent(used, limit int64) float64 {
	if limit <= 0 || used <= 0 {
		return 0
	}
	percent := float64(used) * 100 / float64(limit)
	if percent > 100 {
		return 100
	}
	return percent
}

// PopulateSubscriptionUsage fills the derived usage fields used by the self
// subscription API. Errors are intentionally returned so callers that need
// strict consistency can surface them; list endpoints may choose to ignore
// them for backward compatibility.
func PopulateSubscriptionUsage(sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if sub == nil {
		return errors.New("subscription is nil")
	}
	if now <= 0 {
		now = GetDBTimestamp()
	}
	policy := quotaPolicyForSubscription(sub, plan)
	if policy.invalid {
		return invalidSubscriptionQuotaPolicyError()
	}
	weeklyLimit := policyWeeklyLimit(policy, sub)
	weeklyUsed := sub.AmountUsed
	weeklyResetAt := sub.NextResetTime
	if policy.resetPeriod != SubscriptionResetNever {
		// A zero next_reset_time is meaningful: when a subscription ends before
		// the next calendar boundary (for example, a short monthly-reset plan
		// issued mid-month), calcNextResetTime stores zero because there is no
		// future reset inside the subscription.  Do not treat that sentinel as
		// an already-expired boundary or the usage display would be cleared while
		// the aggregate counter is still authoritative.
		if weeklyResetAt > 0 && weeklyResetAt <= now {
			baseUnix := sub.LastResetTime
			if baseUnix <= 0 {
				baseUnix = sub.StartTime
			}
			weeklyResetAt = calcNextResetTimeForPolicy(time.Unix(baseUnix, 0), policy, sub.EndTime)
			if weeklyResetAt > 0 && weeklyResetAt <= now {
				base := time.Unix(weeklyResetAt, 0)
				for weeklyResetAt > 0 && weeklyResetAt <= now {
					base = time.Unix(weeklyResetAt, 0)
					weeklyResetAt = calcNextResetTimeForPolicy(base, policy, sub.EndTime)
				}
			}
			// The persisted boundary has definitely passed, so the aggregate
			// usage belongs to a previous period even when there is no subsequent
			// boundary before the subscription end (in which case weeklyResetAt is
			// left at zero by calcNextResetTimeForPolicy).
			weeklyUsed = 0
		}
	}
	if weeklyLimit > 0 && weeklyUsed > weeklyLimit {
		weeklyUsed = weeklyLimit
	}
	sub.WeeklyUsed = weeklyUsed
	sub.WeeklyResetAt = weeklyResetAt
	if weeklyLimit > 0 {
		sub.WeeklyRemaining = maxInt64(0, weeklyLimit-weeklyUsed)
	} else {
		sub.WeeklyRemaining = 0
	}

	if policyHasFiveHour(policy) && DB != nil {
		used, resetAt, err := recentSubscriptionUsageTx(DB, sub.Id, now, policy.fiveHourWindowSeconds)
		if err != nil {
			return err
		}
		sub.FiveHourUsed = used
		sub.FiveHourResetAt = resetAt
		sub.FiveHourRemaining = maxInt64(0, policy.fiveHourAmount-used)
	} else {
		sub.FiveHourUsed = 0
		sub.FiveHourResetAt = 0
		sub.FiveHourRemaining = 0
	}

	windows := make([]SubscriptionQuotaWindowSnapshot, 0, 2)
	if policyHasGenericWindows(policy) {
		for _, window := range policy.windows {
			used, resetAt := int64(0), int64(0)
			if DB != nil && sub.Id > 0 {
				var err error
				used, resetAt, err = recentSubscriptionWindowUsageTx(DB, sub.Id, now, window, sub.StartTime)
				if err != nil {
					return err
				}
			} else {
				_, resetAt = quotaWindowBounds(window, now, sub.StartTime)
			}
			windows = append(windows, SubscriptionQuotaWindowSnapshot{
				Type: window.unit, Period: window.unit, Duration: window.duration,
				ResetMode: window.resetMode, Name: window.name, Limit: window.amount, Used: used,
				Remaining: maxInt64(0, window.amount-used), ResetAt: resetAt,
				WindowSeconds: quotaWindowEffectiveSeconds(window, now, sub.StartTime), UsedPercent: quotaUsedPercent(used, window.amount),
			})
		}
	}
	if policyHasFiveHour(policy) {
		windows = append(windows, SubscriptionQuotaWindowSnapshot{
			Type: "five_hour", Period: SubscriptionQuotaWindowHour, Duration: 5,
			ResetMode: SubscriptionQuotaWindowRolling,
			Limit:     policy.fiveHourAmount, Used: sub.FiveHourUsed,
			Remaining: sub.FiveHourRemaining, ResetAt: sub.FiveHourResetAt,
			WindowSeconds: policy.fiveHourWindowSeconds,
			UsedPercent:   quotaUsedPercent(sub.FiveHourUsed, policy.fiveHourAmount),
		})
	}
	if policyHasWeekly(policy) || (policy.mode == SubscriptionQuotaWindowLegacy && weeklyLimit > 0) {
		windows = append(windows, SubscriptionQuotaWindowSnapshot{
			Type: "weekly", Period: SubscriptionQuotaWindowWeek, Duration: 1,
			ResetMode: SubscriptionQuotaWindowCalendar,
			Limit:     weeklyLimit, Used: sub.WeeklyUsed,
			Remaining: sub.WeeklyRemaining, ResetAt: sub.WeeklyResetAt,
			UsedPercent: quotaUsedPercent(sub.WeeklyUsed, weeklyLimit),
		})
	}
	sub.QuotaWindows = windows
	return nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// GetSubscriptionPlanById 根据套餐 ID 获取订阅套餐详情
func GetSubscriptionPlanById(id int) (*SubscriptionPlan, error) {
	return getSubscriptionPlanByIdTx(nil, id)
}

// getSubscriptionPlanByIdTx 根据套餐 ID 从缓存中获取订阅套餐详情
func getSubscriptionPlanByIdTx(tx *gorm.DB, id int) (*SubscriptionPlan, error) {
	if id <= 0 {
		return nil, errors.New("invalid plan id")
	}
	key := subscriptionPlanCacheKey(id)
	// 事务内读取必须绕开缓存：库存和归属校验需要使用数据库中的最新值。
	if tx == nil && key != "" {
		// 将套餐 ID 作为键从缓存中获取订阅套餐详情
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
			cached.NormalizeQuotaWindows()
			return &cached, nil
		}
	}
	var plan SubscriptionPlan
	query := DB
	if tx != nil {
		query = tx
	}
	if err := query.Where("id = ?", id).First(&plan).Error; err != nil {
		return nil, err
	}
	// Keep the storage representation in the cache, not the already-filtered
	// compatibility view.  NormalizeQuotaWindows intentionally drops malformed
	// entries after recording an in-memory fail-closed marker; that marker is
	// transient and is not part of the public JSON shape.  Caching only the
	// normalized value would therefore lose the marker across a JSON/Redis
	// round-trip, allowing a malformed generic/period policy to be reinterpreted
	// as a valid (or unlimited) policy on the next cache hit.  A raw copy retains
	// the offending entry so every cache reader can detect it again before
	// normalization.  The returned value remains normalized for callers.
	rawPlan := plan
	plan.NormalizeQuotaWindows()
	if tx == nil {
		_ = getSubscriptionPlanCache().SetWithTTL(key, rawPlan, subscriptionPlanCacheTTL())
	}
	return &plan, nil
}

// getSubscriptionPlanOrSnapshotTx keeps issued subscriptions usable after an
// administrator removes their source plan. Only versioned snapshots qualify;
// old rows without a complete policy snapshot remain fail-closed.
func getSubscriptionPlanOrSnapshotTx(tx *gorm.DB, sub *UserSubscription) (*SubscriptionPlan, error) {
	if sub == nil {
		return nil, errors.New("invalid subscription plan snapshot")
	}
	// Rows created by the original subscription implementation could omit a
	// plan id and still be settled/refunded through the aggregate AmountUsed
	// counter.  Keep those legacy accounting paths operational; request-aware
	// pre-consume still requires a real plan before admitting a new request.
	if sub.PlanId <= 0 {
		if sub.PlanPolicySnapshotVersion > 0 {
			return nil, errors.New("invalid subscription plan snapshot")
		}
		return nil, nil
	}
	plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
	if err == nil {
		if sub.PlanPolicySnapshotVersion > 0 {
			// Issued subscriptions keep the provider/model fence that was sold to
			// them. Quota fields are resolved from the subscription snapshot by
			// quotaPolicyForSubscription; copy these remaining policy fields here
			// so a later plan edit cannot silently change existing entitlements.
			snapshotPlan := *plan
			snapshotPlan.ProviderId = sub.ProviderId
			snapshotPlan.ModelLimits = sub.ModelLimitsSnapshot
			// The policy snapshot also carries the title shown to the customer at
			// purchase time.  Keep it stable when an administrator edits the
			// catalog row after issuance; this is especially important for billing
			// logs and for provider plans whose source row may be repurposed.
			if strings.TrimSpace(sub.PlanTitleSnapshot) != "" {
				snapshotPlan.Title = sub.PlanTitleSnapshot
			}
			return &snapshotPlan, nil
		}
		return plan, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) || sub.PlanPolicySnapshotVersion <= 0 {
		return nil, err
	}
	return subscriptionPlanSnapshotFallback(sub), nil
}

// subscriptionPlanSnapshotFallback builds the minimum plan-shaped object
// needed by billing and API consumers after the source plan has been deleted.
// Issued subscriptions carry the policy snapshot, so deleting a catalog row
// must not turn an active subscription into an unusable or anonymous one.
func subscriptionPlanSnapshotFallback(sub *UserSubscription) *SubscriptionPlan {
	if sub == nil {
		return nil
	}
	mode := sub.QuotaWindowMode
	if strings.TrimSpace(mode) == "" {
		mode = sub.QuotaWindowModeSnapshot
	}
	title := strings.TrimSpace(sub.PlanTitleSnapshot)
	if title == "" {
		title = fmt.Sprintf("已删除套餐 #%d", sub.PlanId)
	}
	return &SubscriptionPlan{
		Id: sub.PlanId, ProviderId: sub.ProviderId,
		Title:    title,
		Subtitle: "原套餐已删除（按已发行策略继续生效）",
		Enabled:  true, AllowPurchase: 0,
		ModelLimits: sub.ModelLimitsSnapshot,
		TotalAmount: sub.AmountTotal, QuotaWindowMode: mode,
		FiveHourAmount: sub.FiveHourAmount, FiveHourWindowSeconds: sub.FiveHourWindowSeconds,
		WeeklyAmount: sub.WeeklyAmount, QuotaWindows: NormalizeQuotaWindowList(sub.QuotaWindowsSnapshot),
		QuotaResetPeriod:        sub.QuotaResetPeriodSnapshot,
		QuotaResetCustomSeconds: sub.QuotaResetCustomSecondsSnapshot,
	}
}

// subscriptionPlanForSummary returns a per-subscription copy of the catalog
// plan with all issuance-time fields overlaid.  A subscription can outlive
// edits to its source plan; exposing the live catalog object in the user/admin
// summary would otherwise make the displayed title, model fence, or quota
// policy disagree with the policy enforced by billing.  The copy is also
// important when several subscriptions share one plan but have different
// snapshots.
func subscriptionPlanForSummary(plan *SubscriptionPlan, sub *UserSubscription) *SubscriptionPlan {
	if plan == nil || sub == nil || sub.PlanPolicySnapshotVersion <= 0 {
		return plan
	}
	snapshot := *plan
	if title := strings.TrimSpace(sub.PlanTitleSnapshot); title != "" {
		snapshot.Title = title
	}
	// ModelLimitsSnapshot is authoritative even when it is intentionally empty
	// (an empty snapshot means the issued subscription allowed every model).
	snapshot.ModelLimits = sub.ModelLimitsSnapshot
	mode := strings.TrimSpace(sub.QuotaWindowMode)
	if mode == "" {
		mode = strings.TrimSpace(sub.QuotaWindowModeSnapshot)
	}
	if mode != "" {
		snapshot.QuotaWindowMode = mode
	}
	snapshot.FiveHourAmount = sub.FiveHourAmount
	snapshot.FiveHourWindowSeconds = sub.FiveHourWindowSeconds
	snapshot.WeeklyAmount = sub.WeeklyAmount
	// An empty reset-period snapshot is meaningful: it records the legacy
	// "never" policy.  Always overwrite the live catalog value for versioned
	// subscriptions so the summary cannot display a reset schedule that billing
	// does not enforce after an administrator edits the source plan.
	snapshot.QuotaResetPeriod = sub.QuotaResetPeriodSnapshot
	snapshot.QuotaResetCustomSeconds = sub.QuotaResetCustomSecondsSnapshot
	snapshot.QuotaWindows = NormalizeQuotaWindowList(sub.QuotaWindowsSnapshot)
	snapshot.NormalizeQuotaWindows()
	return &snapshot
}

func CountUserSubscriptionsByPlan(userId int, planId int) (int64, error) {
	return countUserSubscriptionsByPurchaseScope(userId, planId)
}

func normalizeSubscriptionOrderCurrency(currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "", "$", "USD":
		return "USD"
	case "€", "EUR":
		return "EUR"
	case "￥", "¥", "CNY", "RMB":
		return "CNY"
	default:
		return strings.ToUpper(strings.TrimSpace(currency))
	}
}

func subscriptionAccountingMoneyMatches(quoted, authoritative float64) bool {
	if math.IsNaN(quoted) || math.IsInf(quoted, 0) ||
		math.IsNaN(authoritative) || math.IsInf(authoritative, 0) {
		return false
	}
	// price_amount is DECIMAL(10,6) on MySQL/PostgreSQL.  Comparing at that
	// scale avoids rejecting an unchanged value solely because it passed
	// through a float64 representation, while still detecting a real catalog
	// edit before the order reserves inventory.
	return decimal.NewFromFloat(quoted).Round(6).Equal(
		decimal.NewFromFloat(authoritative).Round(6),
	)
}

func expectedSubscriptionPaymentProductID(order *SubscriptionOrder, plan *SubscriptionPlan) string {
	if order == nil || plan == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(order.PaymentProvider)) {
	case PaymentProviderStripe:
		if normalizeSubscriptionOrderCurrency(order.Currency) == "CNY" {
			return strings.TrimSpace(plan.StripePriceCnyId)
		}
		return strings.TrimSpace(plan.StripePriceId)
	case PaymentProviderCreem:
		return strings.TrimSpace(plan.CreemProductId)
	case PaymentProviderWaffoPancake:
		return strings.TrimSpace(plan.WaffoPancakeProductId)
	default:
		return strings.TrimSpace(order.PaymentProductId)
	}
}

// normalizeSubscriptionOrderPaymentSnapshot binds an order's accounting and
// gateway identity to the plan row already locked by CreateSubscriptionOrderTx.
// For ordinary gateways the controller has already prepared a remote quote;
// any mismatch means that quote is stale and must be restarted.  Crypto quotes
// are intentionally calculated after this transaction and can adopt the
// authoritative price directly.
func normalizeSubscriptionOrderPaymentSnapshot(order *SubscriptionOrder, plan *SubscriptionPlan) error {
	if order == nil || plan == nil || plan.PriceAmount <= 0 ||
		math.IsNaN(plan.PriceAmount) || math.IsInf(plan.PriceAmount, 0) {
		return ErrSubscriptionCheckoutChanged
	}

	if order.PaymentMethod == PaymentMethodCrypto ||
		strings.EqualFold(strings.TrimSpace(order.PaymentProvider), PaymentProviderCrypto) {
		order.Money = plan.PriceAmount
		order.OriginalMoney = plan.PriceAmount
		return nil
	}

	if !subscriptionAccountingMoneyMatches(order.Money, plan.PriceAmount) {
		return ErrSubscriptionCheckoutChanged
	}
	expectedProductID := expectedSubscriptionPaymentProductID(order, plan)
	if strings.TrimSpace(order.PaymentProductId) != expectedProductID {
		return ErrSubscriptionCheckoutChanged
	}

	order.Money = plan.PriceAmount
	if math.IsNaN(order.OriginalMoney) || math.IsInf(order.OriginalMoney, 0) || order.OriginalMoney < 0 {
		return ErrSubscriptionCheckoutChanged
	}
	currency := normalizeSubscriptionOrderCurrency(order.Currency)
	if order.OriginalMoney == 0 {
		// Old/internal USD callers did not always populate OriginalMoney.  It is
		// unambiguous in USD, so fill the immutable callback snapshot here.  A CNY
		// amount requires the checkout-time exchange rate and therefore must be
		// supplied by the controller rather than guessed in the model layer.
		if currency != "USD" {
			return ErrSubscriptionCheckoutChanged
		}
		order.OriginalMoney = plan.PriceAmount
	}
	if currency == "USD" && math.Abs(order.OriginalMoney-plan.PriceAmount) > 0.0050001 {
		// USD gateways commonly round to the minor unit (Waffo does so
		// explicitly), hence a half-cent tolerance.  Keep the controller's exact
		// charged value on the order for webhook verification.
		return ErrSubscriptionCheckoutChanged
	}
	return nil
}

// CreateSubscriptionOrderTx 创建待支付订阅订单并原子预占一份套餐额度。
// 所有支付渠道必须通过此方法下单，不能在控制器中直接 tx.Create(order)。
func CreateSubscriptionOrderTx(tx *gorm.DB, order *SubscriptionOrder) error {
	if tx == nil || order == nil {
		return errors.New("invalid subscription order")
	}
	if common.UsingSQLite {
		// SQLite's deferred transactions can deadlock while upgrading a read
		// snapshot to a writer.  Serialize the complete order validation and
		// reservation path in this process; the database-level busy timeout still
		// handles writers from other processes.
		subscriptionSQLiteStockMu.Lock()
		defer subscriptionSQLiteStockMu.Unlock()
	}
	if order.UserId <= 0 || order.PlanId <= 0 || strings.TrimSpace(order.TradeNo) == "" {
		return errors.New("invalid subscription order")
	}
	if order.CreateTime == 0 {
		order.CreateTime = common.GetTimestamp()
	}
	if order.Status == "" {
		order.Status = common.TopUpStatusPending
	} else if order.Status != common.TopUpStatusPending {
		// Orders are created as pending and transition to success/expired only
		// through the completion/expiry state machines.  Accepting a caller
		// supplied success status here would reserve stock without a payment
		// record and bypass the idempotent completion path.
		return ErrSubscriptionOrderStatusInvalid
	}
	if order.StockStatus != "" && order.StockStatus != SubscriptionStockStatusReserved {
		return ErrSubscriptionStockInvalid
	}

	// 同一用户的订单创建和赠送统一锁住 user 行，使每用户购买上限在多实例下也严格生效。
	var user User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "provider_id").Where("id = ?", order.UserId).First(&user).Error; err != nil {
		return err
	}
	if user.ProviderId != order.ProviderId {
		return errors.New("subscription provider does not match user provider")
	}

	// Load and lock the authoritative plan/group row before validating any
	// order fields.  A controller may have read a cached plan just before this
	// transaction; using that stale object for provider/enabled checks could
	// reserve stock for a plan that has since moved tenants or been disabled.
	scope, err := loadSubscriptionPurchaseScopeTx(tx, order.PlanId)
	if err != nil {
		return err
	}
	plan := scope.target
	if plan.ProviderId != order.ProviderId {
		return errors.New("subscription plan does not belong to user provider")
	}
	if !plan.Enabled || plan.AllowPurchase != 1 {
		return errors.New("该套餐暂不允许订阅")
	}
	if err := checkUserSubscriptionPurchaseLimitTx(tx, order.UserId, order.PlanId, plan.MaxPurchasePerUser); err != nil {
		return err
	}
	// SubscriptionOrder.Money is always the authoritative USD accounting price.
	// Controllers necessarily read a plan before entering this transaction to
	// prepare their gateway-specific checkout.  Reject a stale non-crypto quote
	// instead of silently combining its remote amount/product with the newly
	// locked entitlement snapshot.  Crypto is different: its token amount is
	// calculated only after this function returns and is stored separately in
	// CryptoTransaction.UsdtAmount, so it can safely adopt the locked price.
	if err := normalizeSubscriptionOrderPaymentSnapshot(order, &plan); err != nil {
		return err
	}
	// Capture entitlement policy from the authoritative locked row.  Never
	// trust a caller-supplied snapshot: controllers may receive a stale plan
	// object (or a malicious payload), while this row is the source of truth at
	// checkout time.  The snapshot is written before the stock reservation and
	// order insert so any serialization error rolls the whole transaction back.
	planSnapshot, err := snapshotSubscriptionPlan(&plan)
	if err != nil {
		return err
	}
	order.PlanSnapshot = planSnapshot
	if err := reserveSubscriptionPlanStockTx(tx, order.PlanId); err != nil {
		return err
	}

	order.StockStatus = SubscriptionStockStatusReserved
	if order.StockExpiresAt <= 0 {
		order.StockExpiresAt = order.CreateTime + DefaultSubscriptionStockReservationSeconds
	}
	return tx.Create(order).Error
}

// checkUserSubscriptionPurchaseLimitTx 的调用方必须先锁定对应用户行，避免并发下重复突破每用户上限。
func checkUserSubscriptionPurchaseLimitTx(tx *gorm.DB, userId int, planId int, _ int) error {
	return checkUserSubscriptionPurchaseScopeLimitTx(tx, userId, planId)
}

func reserveSubscriptionPlanStockTx(tx *gorm.DB, planId int) error {
	return incrementSubscriptionPurchaseScopeStockTx(tx, planId, true)
}

func issueSubscriptionPlanStockTx(tx *gorm.DB, planId int) error {
	return incrementSubscriptionPurchaseScopeStockTx(tx, planId, false)
}

func commitReservedSubscriptionPlanStockTx(tx *gorm.DB, planId int) error {
	const maxInt64 = int64(^uint64(0) >> 1)
	res := tx.Model(&SubscriptionPlan{}).
		Where("id = ? AND reserved_count > 0 AND issued_count >= 0 AND issued_count < ?", planId, maxInt64).
		Updates(map[string]interface{}{
			"reserved_count": gorm.Expr("reserved_count - ?", 1),
			"issued_count":   gorm.Expr("issued_count + ?", 1),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrSubscriptionStockInvalid
	}
	return nil
}

func releaseReservedSubscriptionPlanStockTx(tx *gorm.DB, planId int) error {
	res := tx.Model(&SubscriptionPlan{}).
		Where("id = ? AND reserved_count > 0", planId).
		UpdateColumn("reserved_count", gorm.Expr("reserved_count - ?", 1))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrSubscriptionStockInvalid
	}
	return nil
}

func getUserGroupByIdTx(tx *gorm.DB, userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("invalid userId")
	}
	if tx == nil {
		tx = DB
	}
	var group string
	if err := tx.Model(&User{}).Where("id = ?", userId).Select(commonGroupCol).Find(&group).Error; err != nil {
		return "", err
	}
	return group, nil
}

// getUserProviderIdByIdTx 查询指定用户绑定的 provider_id（0 表示主站用户，>0 表示归属某服务商）。
// 在 CreateUserSubscriptionFromPlanTx 中用于：当调用方未显式传入 providerId 时，按用户归属自动继承，
// 保证订阅实例与用户在同一个服务商上下文中。
func getUserProviderIdByIdTx(tx *gorm.DB, userId int) (int, error) {
	if userId <= 0 {
		return 0, errors.New("invalid userId")
	}
	if tx == nil {
		tx = DB
	}
	var providerId int
	if err := tx.Model(&User{}).Where("id = ?", userId).Select("provider_id").Find(&providerId).Error; err != nil {
		return 0, err
	}
	return providerId, nil
}

func downgradeUserGroupForSubscriptionTx(tx *gorm.DB, sub *UserSubscription, now int64) (string, error) {
	if tx == nil || sub == nil {
		return "", errors.New("invalid downgrade args")
	}
	upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
	if upgradeGroup == "" {
		return "", nil
	}
	currentGroup, err := getUserGroupByIdTx(tx, sub.UserId)
	if err != nil {
		return "", err
	}
	if currentGroup != upgradeGroup {
		return "", nil
	}
	var activeSub UserSubscription
	// A subscription is usable only after its start time.  COALESCE keeps
	// legacy rows with a NULL/zero start_time backwards compatible while
	// preventing a future-dated entitlement from blocking group downgrade.
	activeQuery := tx.Where("user_id = ? AND status = ? AND COALESCE(start_time, 0) <= ? AND end_time > ? AND id <> ? AND upgrade_group <> ''",
		sub.UserId, "active", now, now, sub.Id).
		Order("end_time desc, id desc").
		Limit(1).
		Find(&activeSub)
	if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
		return "", nil
	}
	prevGroup := strings.TrimSpace(sub.PrevUserGroup)
	if prevGroup == "" || prevGroup == currentGroup {
		return "", nil
	}
	if err := tx.Model(&User{}).Where("id = ?", sub.UserId).
		Update("group", prevGroup).Error; err != nil {
		return "", err
	}
	return prevGroup, nil
}

// CreateUserSubscriptionFromPlanTx 基于套餐创建用户订阅实例（事务内）。
// providerIds 为可选可变参数：显式传入时用传入值作为订阅实例 ProviderId（如订单完成时用 order.ProviderId）；
// 未传入时回退到从用户表查 provider_id，保证订阅实例归属与用户一致。
// 这样既支持"按订单归属"也支持"按用户归属"两种语义，且保持向后兼容(原签名仍可用)。
func CreateUserSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string, providerIds ...int) (*UserSubscription, error) {
	return createUserSubscriptionFromPlanTxWithSnapshot(tx, userId, plan, nil, source, false, providerIds...)
}

// createUserSubscriptionFromReservedPlanTx 用于支付订单完成：订单在下单时已经预占库存，
// 此处只把预占原子转换为已发放，不能再次占用。
func createUserSubscriptionFromReservedPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string, providerIds ...int) (*UserSubscription, error) {
	return createUserSubscriptionFromPlanTxWithSnapshot(tx, userId, plan, nil, source, true, providerIds...)
}

func createUserSubscriptionFromPlanTx(tx *gorm.DB, userId int, plan *SubscriptionPlan, source string, stockReserved bool, providerIds ...int) (*UserSubscription, error) {
	return createUserSubscriptionFromPlanTxWithSnapshot(tx, userId, plan, nil, source, stockReserved, providerIds...)
}

// createUserSubscriptionFromPlanSnapshotTx issues an entitlement from the
// immutable plan payload captured on a paid order.  The current catalog row is
// still locked and used for provider/scope validation and stock accounting;
// only mutable entitlement fields (duration, limits, quota policy, title,
// upgrade group, etc.) come from the snapshot.
func createUserSubscriptionFromPlanSnapshotTx(tx *gorm.DB, userId int, snapshot *SubscriptionPlan, source string, providerIds ...int) (*UserSubscription, error) {
	return createUserSubscriptionFromPlanSnapshotWithStockTx(tx, userId, snapshot, source, true, providerIds...)
}

// createUserSubscriptionFromPlanSnapshotUnreservedTx is the compatibility
// path for pre-snapshot orders that have an entitlement payload but no stock
// reservation marker. It issues from the payload while atomically allocating a
// fresh stock slot, mirroring CreateUserSubscriptionFromPlanTx semantics.
func createUserSubscriptionFromPlanSnapshotUnreservedTx(tx *gorm.DB, userId int, snapshot *SubscriptionPlan, source string, providerIds ...int) (*UserSubscription, error) {
	return createUserSubscriptionFromPlanSnapshotWithStockTx(tx, userId, snapshot, source, false, providerIds...)
}

func createUserSubscriptionFromPlanSnapshotWithStockTx(tx *gorm.DB, userId int, snapshot *SubscriptionPlan, source string, stockReserved bool, providerIds ...int) (*UserSubscription, error) {
	if snapshot == nil {
		return nil, ErrSubscriptionPlanSnapshotInvalid
	}
	return createUserSubscriptionFromPlanTxWithSnapshot(tx, userId, snapshot, snapshot, source, stockReserved, providerIds...)
}

func createUserSubscriptionFromPlanTxWithSnapshot(tx *gorm.DB, userId int, plan *SubscriptionPlan, entitlementSnapshot *SubscriptionPlan, source string, stockReserved bool, providerIds ...int) (*UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if common.UsingSQLite {
		// See subscriptionSQLiteStockMu's comment: issuance updates the same
		// catalog counters as reservation and must be serialized as well.
		subscriptionSQLiteStockMu.Lock()
		defer subscriptionSQLiteStockMu.Unlock()
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	var lockedUser User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "provider_id").Where("id = ?", userId).First(&lockedUser).Error; err != nil {
		return nil, err
	}
	// Never snapshot a caller-supplied/cached plan directly.  Admin binds,
	// airdrops, and payment completion may load a plan before entering this
	// transaction; an administrator can edit provider/group/quota fields in
	// between those operations.  loadSubscriptionPurchaseScopeTx obtains the
	// authoritative row lock (and the complete shared-group lock set), so the
	// provider validation, quota snapshot, and stock transition all describe
	// exactly the same catalog version.
	scope, err := loadSubscriptionPurchaseScopeTx(tx, plan.Id)
	if err != nil {
		return nil, err
	}
	authoritativePlan := scope.target
	authoritativePlan.NormalizeQuotaWindows()
	// The catalog row remains authoritative for identity, provider ownership,
	// purchase scope, and inventory counters.  When an order carries a valid
	// entitlement snapshot, overlay only the policy fields onto a local copy so
	// edits made after checkout cannot change what is issued.
	if entitlementSnapshot != nil {
		snapshot := *entitlementSnapshot
		snapshot.NormalizeQuotaWindows()
		if snapshot.Id != authoritativePlan.Id || snapshot.ProviderId != authoritativePlan.ProviderId {
			return nil, ErrSubscriptionPlanSnapshotInvalid
		}
		currentGroup, currentGroupOK := NormalizeSubscriptionPurchaseLimitGroup(authoritativePlan.PurchaseLimitGroup)
		snapshotGroup, snapshotGroupOK := NormalizeSubscriptionPurchaseLimitGroup(snapshot.PurchaseLimitGroup)
		if !currentGroupOK || !snapshotGroupOK || currentGroup != snapshotGroup {
			// A provider/group move while a reservation is pending would strand
			// stock in the old scope.  Treat it as an invalid snapshot rather than
			// silently committing a different scope.
			return nil, ErrSubscriptionPlanSnapshotInvalid
		}
		plan = &snapshot
	} else {
		plan = &authoritativePlan
	}
	nowUnix := getDBTimestampTx(tx)
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	resetBase := now
	policy := quotaPolicyFromPlan(plan)
	if policy.invalid {
		return nil, invalidSubscriptionQuotaPolicyError()
	}
	nextReset := int64(0)
	if policyTracksAggregate(policy) {
		nextReset = calcNextResetTimeForPolicy(resetBase, policy, endUnix)
	}
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}
	// 解析订阅实例归属服务商：优先用显式传入的 providerIds[0]，
	// 否则按用户表的 provider_id 自动继承，保证订阅与用户在同一服务商上下文。
	userProviderId := lockedUser.ProviderId
	providerId := userProviderId
	if len(providerIds) > 0 {
		providerId = providerIds[0]
	}
	if providerId != userProviderId {
		return nil, errors.New("subscription provider does not match user provider")
	}
	if plan.ProviderId != providerId {
		return nil, errors.New("subscription plan does not belong to user provider")
	}
	if stockReserved {
		if err := commitReservedSubscriptionPlanStockTx(tx, plan.Id); err != nil {
			return nil, err
		}
	} else {
		if err := checkUserSubscriptionPurchaseLimitTx(tx, userId, plan.Id, authoritativePlan.MaxPurchasePerUser); err != nil {
			return nil, err
		}
		if err := issueSubscriptionPlanStockTx(tx, plan.Id); err != nil {
			return nil, err
		}
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	prevGroup := ""
	if upgradeGroup != "" {
		currentGroup, err := getUserGroupByIdTx(tx, userId)
		if err != nil {
			return nil, err
		}
		if currentGroup != upgradeGroup {
			prevGroup = currentGroup
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", upgradeGroup).Error; err != nil {
				return nil, err
			}
		}
	}
	sub := &UserSubscription{
		UserId:                          userId,
		PlanId:                          plan.Id,
		ProviderId:                      providerId,
		AmountTotal:                     plan.TotalAmount,
		AmountUsed:                      0,
		QuotaWindowMode:                 policy.mode,
		QuotaWindowModeSnapshot:         policy.mode,
		FiveHourAmount:                  policy.fiveHourAmount,
		FiveHourWindowSeconds:           policy.fiveHourWindowSeconds,
		WeeklyAmount:                    policy.weeklyAmount,
		QuotaResetPeriodSnapshot:        policy.resetPeriod,
		QuotaResetCustomSecondsSnapshot: policy.resetCustomSeconds,
		QuotaWindowsSnapshot:            NormalizeQuotaWindowList(plan.QuotaWindows),
		PlanPolicySnapshotVersion:       1,
		ModelLimitsSnapshot:             plan.ModelLimits,
		PlanTitleSnapshot:               plan.Title,
		StartTime:                       now.Unix(),
		EndTime:                         endUnix,
		Status:                          "active",
		Source:                          source,
		LastResetTime:                   lastReset,
		NextResetTime:                   nextReset,
		UpgradeGroup:                    upgradeGroup,
		PrevUserGroup:                   prevGroup,
		CreatedAt:                       common.GetTimestamp(),
		UpdatedAt:                       common.GetTimestamp(),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

// subscriptionCompletionResult carries the post-commit side effects produced
// while completing an order.  Keeping these values separate from the database
// transaction lets crypto confirmation update the order and its chain record
// atomically without writing logs/caches before the commit succeeds.
type subscriptionCompletionResult struct {
	logUserID             int
	logPlanTitle          string
	logMoney              float64
	logPaymentMethod      string
	upgradeGroup          string
	providerIncomeOwnerID int
	providerIncomeQuota   int
	providerIncomeCreated bool
}

// completeSubscriptionOrderTx performs the order completion inside an
// existing transaction.  The caller owns the transaction and may lock related
// payment records before invoking it (the crypto path does exactly that).
func completeSubscriptionOrderTx(tx *gorm.DB, tradeNo string, providerPayload string, expectedPaymentMethod string, expectedPaymentProvider ...string) (*subscriptionCompletionResult, error) {
	if tx == nil || strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	result := &subscriptionCompletionResult{}
	var order SubscriptionOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
		return nil, ErrSubscriptionOrderNotFound
	}
	// 订阅补单/回调处理时，必须确认当前回调网关和本地订单支付方式一致。
	if expectedPaymentMethod != "" && order.PaymentMethod != expectedPaymentMethod {
		return nil, ErrPaymentMethodMismatch
	}
	if len(expectedPaymentProvider) > 0 && expectedPaymentProvider[0] != "" && order.PaymentProvider != expectedPaymentProvider[0] {
		// Orders created before payment_provider was introduced have an empty
		// provider snapshot. Their payment method is still checked above, so
		// allow that narrow legacy case for the corresponding gateway while
		// rejecting any explicit cross-provider value.
		if strings.TrimSpace(order.PaymentProvider) != "" {
			return nil, ErrPaymentMethodMismatch
		}
	}
	if order.Status == common.TopUpStatusSuccess {
		// 订单此前已完成（重复回调/补单）：仍要保证幂等地补齐充值流水与服务商收入，避免漏发钱。
		if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return nil, err
		}
		incomeOwnerID, incomeQuota, incomeCreated, err := applyProviderSubscriptionIncomeTx(tx, &order)
		if err != nil {
			return nil, err
		}
		result.providerIncomeOwnerID = incomeOwnerID
		result.providerIncomeQuota = incomeQuota
		result.providerIncomeCreated = incomeCreated
		return result, nil
	}
	if order.Status != common.TopUpStatusPending {
		return nil, ErrSubscriptionOrderStatusInvalid
	}
	// A reservation has a finite lifetime.  Webhook retries arriving after the
	// reservation sweeper's deadline must not issue a subscription unless the
	// order is still pending and its reservation is valid; otherwise stock can
	// be committed after it has already been logically released.  Legacy orders
	// with no stock snapshot (empty status/expiry) retain their historical path.
	if order.StockStatus == SubscriptionStockStatusReserved && order.StockExpiresAt > 0 && order.StockExpiresAt <= common.GetTimestamp() {
		return nil, ErrSubscriptionStockInvalid
	}
	plan, err := getSubscriptionPlanByIdTx(tx, order.PlanId)
	if err != nil {
		return nil, err
	}
	planSnapshot, err := decodeSubscriptionPlanSnapshot(order.PlanSnapshot, order.PlanId, order.ProviderId)
	if err != nil {
		return nil, err
	}
	// 新订单在下单时已预占；历史订单没有库存状态，完成时再原子占用一份。
	var issuedSub *UserSubscription
	if order.StockStatus == SubscriptionStockStatusReserved {
		if planSnapshot != nil {
			issuedSub, err = createUserSubscriptionFromPlanSnapshotTx(tx, order.UserId, planSnapshot, "order", order.ProviderId)
		} else {
			issuedSub, err = createUserSubscriptionFromReservedPlanTx(tx, order.UserId, plan, "order", order.ProviderId)
		}
	} else if order.StockStatus == "" {
		if planSnapshot != nil {
			issuedSub, err = createUserSubscriptionFromPlanSnapshotUnreservedTx(tx, order.UserId, planSnapshot, "order", order.ProviderId)
		} else {
			issuedSub, err = CreateUserSubscriptionFromPlanTx(tx, order.UserId, plan, "order", order.ProviderId)
		}
	} else {
		return nil, ErrSubscriptionStockInvalid
	}
	if err != nil {
		return nil, err
	}
	// createUserSubscriptionFromPlanTx snapshots the authoritative locked plan,
	// not the potentially stale plan loaded above.  Use the resulting snapshot
	// for all post-commit effects and logs as well.
	if issuedSub != nil {
		result.upgradeGroup = strings.TrimSpace(issuedSub.UpgradeGroup)
		result.logPlanTitle = issuedSub.PlanTitleSnapshot
	}
	paidQuota := int(decimal.NewFromFloat(order.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
	if paidQuota > 0 {
		if err := tx.Model(&User{}).Where("id = ?", order.UserId).
			Update("used_quota", gorm.Expr("used_quota + ?", paidQuota)).Error; err != nil {
			return nil, err
		}
	}
	order.Status = common.TopUpStatusSuccess
	order.StockStatus = SubscriptionStockStatusIssued
	order.CompleteTime = common.GetTimestamp()
	if providerPayload != "" {
		order.ProviderPayload = providerPayload
	}
	if err := tx.Save(&order).Error; err != nil {
		return nil, err
	}
	if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
		return nil, err
	}
	// 首次完成订单时同样结算服务商订阅收入，写入 owner 用户额度与一条 provider_subscription 流水。
	incomeOwnerID, incomeQuota, incomeCreated, err := applyProviderSubscriptionIncomeTx(tx, &order)
	if err != nil {
		return nil, err
	}
	result.providerIncomeOwnerID = incomeOwnerID
	result.providerIncomeQuota = incomeQuota
	result.providerIncomeCreated = incomeCreated
	result.logUserID = order.UserId
	if result.logPlanTitle == "" {
		result.logPlanTitle = plan.Title
	}
	result.logMoney = order.Money
	result.logPaymentMethod = order.PaymentMethod
	return result, nil
}

func finishSubscriptionCompletionSideEffects(tradeNo string, result *subscriptionCompletionResult) {
	if result == nil {
		return
	}
	if result.upgradeGroup != "" && result.logUserID > 0 {
		_ = UpdateUserGroupCache(result.logUserID, result.upgradeGroup)
	}
	if result.logUserID > 0 {
		msg := fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", result.logPlanTitle, result.logMoney, result.logPaymentMethod)
		RecordLog(result.logUserID, LogTypeTopup, msg)
	}
	// 事务提交成功后：异步刷新服务商 owner 用户的额度缓存，并记录一条收入到账日志。
	if result.providerIncomeCreated && result.providerIncomeOwnerID > 0 && result.providerIncomeQuota > 0 {
		asyncIncrUserQuotaCache(result.providerIncomeOwnerID, result.providerIncomeQuota)
		RecordLog(result.providerIncomeOwnerID, LogTypeTopup, fmt.Sprintf("provider subscription income credited %s, source user ID %d, trade no %s", logger.LogQuota(result.providerIncomeQuota), result.logUserID, tradeNo))
	}
}

// CompleteSubscriptionOrder 完成一个订阅订单（幂等）。从套餐创建 UserSubscription 快照。
// Complete a subscription order (idempotent). Creates a UserSubscription snapshot from the plan.
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentMethod string, expectedPaymentProvider ...string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return errors.New("tradeNo is empty")
	}
	var result *subscriptionCompletionResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = completeSubscriptionOrderTx(tx, tradeNo, providerPayload, expectedPaymentMethod, expectedPaymentProvider...)
		return err
	})
	if err != nil {
		return err
	}
	finishSubscriptionCompletionSideEffects(tradeNo, result)
	return nil
}

// CompleteSubscriptionCryptoOrder atomically binds a verified chain transfer
// to its subscription order and activates the entitlement.  The previous
// controller flow completed the subscription and crypto row in two separate
// transactions; a crash between them left a paid order with an unbound chain
// record and allowed a later confirmation to attach a different transfer.
// This method locks both rows, enforces tx-hash uniqueness, and commits them in
// one transaction. Repeating the same (trade_no, tx_hash) is idempotent.
func CompleteSubscriptionCryptoOrder(tradeNo, providerPayload, txHash, payerAddress string, blockNumber uint64, confirmations int) error {
	tradeNo = strings.TrimSpace(tradeNo)
	txHash = normalizeTxHash(txHash)
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	if txHash == "" {
		return errors.New("tx hash is empty")
	}
	var result *subscriptionCompletionResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Lock the subscription payment record first, matching the order lock
		// order used by the ordinary completion path.
		var cryptoTx CryptoTransaction
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("trade_no = ?", tradeNo).First(&cryptoTx).Error; err != nil {
			return errors.New("crypto subscription transaction not found")
		}
		if cryptoTx.SubscriptionOrderId <= 0 || cryptoTx.TopUpId != 0 {
			return errors.New("crypto transaction is not a subscription payment")
		}
		if cryptoTx.UserId <= 0 {
			return errors.New("crypto transaction user is invalid")
		}
		if cryptoTx.TxHash != nil && strings.TrimSpace(*cryptoTx.TxHash) != "" {
			if normalizeTxHash(*cryptoTx.TxHash) != txHash || cryptoTx.Status != CryptoTransactionStatusSuccess {
				if normalizeTxHash(*cryptoTx.TxHash) != txHash {
					return ErrCryptoTransactionHashUsed
				}
				return ErrCryptoTransactionStatusInvalid
			}
			// The chain row is already complete.  CompleteSubscriptionOrder is
			// idempotent and repairs any legacy missing top-up/income records.
			var completeErr error
			result, completeErr = completeSubscriptionOrderTx(tx, tradeNo, providerPayload, PaymentMethodCrypto, PaymentProviderCrypto)
			return completeErr
		}
		// An unbound row is only eligible for the first confirmation while it is
		// explicitly pending.  Treat empty/failed/success states as invalid rather
		// than allowing a stale or partially-written row to be completed.
		if cryptoTx.Status != CryptoTransactionStatusPending {
			return ErrCryptoTransactionStatusInvalid
		}

		// Verify the order/transaction relationship before granting anything.
		var order SubscriptionOrder
		if err := tx.Where("id = ?", cryptoTx.SubscriptionOrderId).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		if order.TradeNo != tradeNo || order.UserId != cryptoTx.UserId {
			return errors.New("crypto transaction does not match subscription order")
		}
		if order.PaymentMethod != PaymentMethodCrypto || (order.PaymentProvider != "" && order.PaymentProvider != PaymentProviderCrypto) {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		// Check the unique hash constraint while holding the current transaction.
		// The database unique index remains the final race-proof guard across
		// concurrent confirmations for different orders.
		var duplicateCount int64
		if err := tx.Model(&CryptoTransaction{}).
			Where("LOWER(tx_hash) = LOWER(?) AND id <> ?", txHash, cryptoTx.Id).Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return ErrCryptoTransactionHashUsed
		}

		var err error
		result, err = completeSubscriptionOrderTx(tx, tradeNo, providerPayload, PaymentMethodCrypto, PaymentProviderCrypto)
		if err != nil {
			return err
		}
		now := common.GetTimestamp()
		cryptoTx.TxHash = &txHash
		cryptoTx.PayerAddress = strings.ToLower(strings.TrimSpace(payerAddress))
		cryptoTx.BlockNumber = blockNumber
		cryptoTx.Confirmations = confirmations
		cryptoTx.Status = CryptoTransactionStatusSuccess
		cryptoTx.CompleteTime = now
		if err := tx.Save(&cryptoTx).Error; err != nil {
			if IsCryptoTransactionHashUniqueViolation(err) {
				return ErrCryptoTransactionHashUsed
			}
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	finishSubscriptionCompletionSideEffects(tradeNo, result)
	return nil
}

func upsertSubscriptionTopUpTx(tx *gorm.DB, order *SubscriptionOrder) error {
	if tx == nil || order == nil {
		return errors.New("invalid subscription order")
	}
	now := common.GetTimestamp()
	// 订阅订单未显式设置状态时，充值记录按待支付处理。
	status := order.Status
	if status == "" {
		status = common.TopUpStatusPending
	}
	// 支付成功但订单未记录完成时间时，使用当前时间补齐。
	completeTime := order.CompleteTime
	if status == common.TopUpStatusSuccess && completeTime == 0 {
		completeTime = now
	}
	var topup TopUp
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			topup = TopUp{
				UserId: order.UserId,
				// 新建充值流水时带上服务商归属，保证流水与订单在同一服务商上下文。
				ProviderId:      order.ProviderId,
				Amount:          0,
				Money:           order.Money,
				TradeNo:         order.TradeNo,
				PaymentMethod:   order.PaymentMethod,
				PaymentProvider: order.PaymentProvider,
				BizType:         TopUpBizTypeSubscription,
				SourceID:        order.Id,
				CreateTime:      order.CreateTime,
				CompleteTime:    completeTime,
				Status:          status,
				Currency:        order.Currency,      // 传递币种符号
				OriginalMoney:   order.OriginalMoney, // 传递实际支付金额
			}
			return tx.Create(&topup).Error
		}
		return err
	}
	// A TopUp row can pre-date the subscription mirror (or, in the worst case,
	// have been created by another payment flow with the same trade number).
	// Only rows whose explicit identity fields agree with this order may be
	// updated.  Zero/empty values are tolerated and backfilled for legacy
	// subscription rows; non-empty conflicting values fail closed so a payment
	// callback can never rewrite another user's recharge record.
	if topup.UserId != 0 && topup.UserId != order.UserId {
		return ErrSubscriptionTopUpMismatch
	}
	if topup.ProviderId != 0 && topup.ProviderId != order.ProviderId {
		return ErrSubscriptionTopUpMismatch
	}
	if method := strings.TrimSpace(topup.PaymentMethod); method != "" &&
		!strings.EqualFold(method, strings.TrimSpace(order.PaymentMethod)) {
		return ErrSubscriptionTopUpMismatch
	}
	if provider := strings.TrimSpace(topup.PaymentProvider); provider != "" {
		// An explicit provider on the existing row must be present on the
		// order as well.  Allowing it when the order snapshot is empty would
		// let a callback for one gateway mutate a row created by another.
		if strings.TrimSpace(order.PaymentProvider) == "" ||
			!strings.EqualFold(provider, strings.TrimSpace(order.PaymentProvider)) {
			return ErrSubscriptionTopUpMismatch
		}
	}
	legacySubscriptionShape := topup.Amount == 0 && topup.SourceID == 0 &&
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(topup.TradeNo)), "sub")
	if bizType := strings.TrimSpace(topup.BizType); bizType != "" && bizType != TopUpBizTypeSubscription {
		// The original top_ups schema defaults BizType to "payment".  Some
		// pre-subscription-mirror rows therefore have the legacy shape
		// (zero amount, sub_* trade number, no source id) even though their
		// business type was never explicitly written.  Permit that narrow
		// shape and convert it to an explicit subscription row below; any
		// amount-bearing or non-subscription trade remains a hard collision.
		if !legacySubscriptionShape {
			return ErrSubscriptionTopUpMismatch
		}
	}
	if topup.SourceID != 0 && topup.SourceID != order.Id {
		return ErrSubscriptionTopUpMismatch
	}
	// An amount-bearing row is a normal recharge record, not the zero-amount
	// subscription mirror.  Do not convert it into a subscription even when
	// older rows left BizType blank.
	if topup.Amount != 0 {
		return ErrSubscriptionTopUpMismatch
	}
	if topup.Money != 0 && !subscriptionAccountingMoneyMatches(topup.Money, order.Money) {
		return ErrSubscriptionTopUpMismatch
	}
	if topup.OriginalMoney != 0 && order.OriginalMoney != 0 &&
		!subscriptionAccountingMoneyMatches(topup.OriginalMoney, order.OriginalMoney) {
		return ErrSubscriptionTopUpMismatch
	}
	if storedCurrency := normalizeSubscriptionOrderCurrency(topup.Currency); storedCurrency != "USD" || strings.TrimSpace(topup.Currency) != "" {
		expectedCurrency := normalizeSubscriptionOrderCurrency(order.Currency)
		if strings.TrimSpace(topup.Currency) != "" && storedCurrency != expectedCurrency {
			return ErrSubscriptionTopUpMismatch
		}
	}
	if topup.UserId == 0 {
		topup.UserId = order.UserId
	}
	topup.Money = order.Money
	// 兼容历史数据：若旧流水未带 provider_id，则按订单补齐，避免老订单缺少服务商归属。
	if topup.ProviderId == 0 {
		topup.ProviderId = order.ProviderId
	}
	// 补充币种信息（仅在 TopUp 尚未设置时）
	if topup.Currency == "" {
		topup.Currency = order.Currency
	}
	// 补充实际支付金额（仅在 TopUp 尚未设置时）
	if topup.OriginalMoney == 0 {
		topup.OriginalMoney = order.OriginalMoney
	}
	if topup.PaymentMethod == "" {
		topup.PaymentMethod = order.PaymentMethod
	}
	if topup.PaymentProvider == "" {
		topup.PaymentProvider = order.PaymentProvider
	}
	if topup.BizType == "" || legacySubscriptionShape {
		topup.BizType = TopUpBizTypeSubscription
	}
	if topup.SourceID == 0 {
		topup.SourceID = order.Id
	}
	if topup.CreateTime == 0 {
		topup.CreateTime = order.CreateTime
	}
	topup.CompleteTime = completeTime
	topup.Status = status
	return tx.Save(&topup).Error
}

// expireSubscriptionOrderTx transitions one pending order to expired and
// releases its reservation.  The bool reports whether this invocation
// actually performed the transition; callers that sweep in parallel can use
// it for accurate metrics without treating an already-completed order as a
// newly expired one.
func expireSubscriptionOrderTx(tx *gorm.DB, tradeNo string, expectedPaymentMethod string) (bool, error) {
	if tx == nil {
		return false, errors.New("database transaction is nil")
	}
	if tradeNo == "" {
		return false, errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	var order SubscriptionOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
		return false, ErrSubscriptionOrderNotFound
	}
	// 过期/关闭订单也要做同样的支付方式校验，避免不同支付通道互相影响订单状态。
	if expectedPaymentMethod != "" && order.PaymentMethod != expectedPaymentMethod {
		return false, ErrPaymentMethodMismatch
	}
	if order.Status != common.TopUpStatusPending {
		return false, nil
	}
	if order.StockStatus == SubscriptionStockStatusReserved {
		if err := releaseReservedSubscriptionPlanStockTx(tx, order.PlanId); err != nil {
			return false, err
		}
		order.StockStatus = SubscriptionStockStatusReleased
	}
	order.Status = common.TopUpStatusExpired
	order.CompleteTime = common.GetTimestamp()
	if err := tx.Save(&order).Error; err != nil {
		return false, err
	}
	if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
		return false, err
	}
	return true, nil
}

func ExpireSubscriptionOrder(tradeNo string, expectedPaymentMethod string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		_, err := expireSubscriptionOrderTx(tx, tradeNo, expectedPaymentMethod)
		return err
	})
}

// ExpireReservedSubscriptionOrders 释放过期但尚未支付的库存预占。
// 每个订单都通过 ExpireSubscriptionOrder 单独事务处理，因此多实例重复执行也安全。
func ExpireReservedSubscriptionOrders(limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	now := common.GetTimestamp()
	var tradeNos []string
	if err := DB.Model(&SubscriptionOrder{}).
		Where("status = ? AND stock_status = ? AND stock_expires_at > 0 AND stock_expires_at <= ?", common.TopUpStatusPending, SubscriptionStockStatusReserved, now).
		Order("stock_expires_at asc, id asc").
		Limit(limit).
		Pluck("trade_no", &tradeNos).Error; err != nil {
		return 0, err
	}
	expired := 0
	for _, tradeNo := range tradeNos {
		transitioned, err := func() (bool, error) {
			var result bool
			err := DB.Transaction(func(tx *gorm.DB) error {
				var err error
				result, err = expireSubscriptionOrderTx(tx, tradeNo, "")
				return err
			})
			return result, err
		}()
		if err != nil {
			return expired, err
		}
		if transitioned {
			expired++
		}
	}
	return expired, nil
}

// AdminBindSubscription 管理员手动为用户绑定订阅套餐（无需支付）。
//
// 与 GrantAirdropSubscription 的区别：
//   - AdminBindSubscription：管理员指定任意 planId 绑定给用户，source 为 "admin"
//   - GrantAirdropSubscription：使用全局配置的空投套餐 ID，source 为 "airdrop"
//
// 返回值：
//   - 如果套餐配置了 UpgradeGroup，返回 "用户分组将升级到 xxx" 的提示消息
//   - 否则返回空字符串
func AdminBindSubscription(userId int, planId int, sourceNote string) (string, error) {
	if userId <= 0 || planId <= 0 {
		return "", errors.New("invalid userId or planId")
	}
	// The issuance helper loads and locks the authoritative catalog row inside
	// the transaction.  Do not retain a cached plan object here: an
	// administrator may edit the upgrade group (or delete the plan) between a
	// preflight read and the bind operation, and the post-bind side effects must
	// describe the exact snapshot that was actually issued.
	plan := &SubscriptionPlan{Id: planId}
	var issued *UserSubscription
	var err error
	err = DB.Transaction(func(tx *gorm.DB) error {
		var bindErr error
		issued, bindErr = CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		if bindErr != nil {
			return bindErr
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	// Use the immutable issuance snapshot rather than the stale caller object
	// for cache updates and the user-facing result.
	upgradeGroup := ""
	if issued != nil {
		upgradeGroup = strings.TrimSpace(issued.UpgradeGroup)
	}
	if upgradeGroup != "" {
		_ = UpdateUserGroupCache(userId, upgradeGroup)
		return fmt.Sprintf("用户分组将升级到 %s", upgradeGroup), nil
	}
	return "", nil
}

// GrantAirdropSubscription 向指定用户授予其所属站点配置的空投订阅计划。
//
// 功能说明：
//   - 主站使用 AirdropSubscriptionPlanId；服务商使用 ProviderRewardConfig 中的对应字段。
//   - 调用此函数后，直接为该用户创建一个来源为 "airdrop" 的活跃订阅，无需支付。
//
// 使用场景：
//   - 管理员在后台手动向特定用户空投订阅（通过 AdminGrantAirdropSubscription API）。
//   - 可作为促销活动的运营工具（批量发放体验订阅）。
//
// 返回值：
//   - 返回授予的套餐标题（planTitle），如果未配置空投套餐或套餐不可用则返回空字符串。
//   - 错误仅在数据库操作失败时返回。
//
// 副作用：
//   - 如果套餐配置了 UpgradeGroup，会更新用户的缓存分组。
//   - 操作记录写入用户日志（LogTypeSystem）。
func GrantAirdropSubscription(userId int) (string, error) {

	if userId <= 0 {
		return "", errors.New("invalid user id")
	}
	var planTitle string
	var upgradeGroup string
	err := DB.Transaction(func(tx *gorm.DB) error {
		providerId, err := getUserProviderIdByIdTx(tx, userId)
		if err != nil {
			return err
		}
		planId, err := getSubscriptionRewardPlanIdTx(tx, providerId, false)
		if err != nil || planId <= 0 {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, planId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if plan == nil || !plan.Enabled {
			return nil
		}
		if plan.ProviderId != providerId {
			return nil
		}
		issued, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "airdrop")
		if err != nil {
			return err
		}
		// CreateUserSubscriptionFromPlanTx reloads and locks the authoritative
		// catalog row before issuing the entitlement.  Use the returned
		// issuance snapshot for the user-facing title/group rather than the
		// potentially stale plan object read during reward-config resolution.
		if issued != nil {
			planTitle = issued.PlanTitleSnapshot
			upgradeGroup = strings.TrimSpace(issued.UpgradeGroup)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if upgradeGroup != "" {
		_ = UpdateUserGroupCache(userId, upgradeGroup)
	}
	if planTitle != "" {
		RecordLog(userId, LogTypeSystem, fmt.Sprintf("airdrop subscription reward %s", planTitle))
	}
	return planTitle, nil
}

// getSubscriptionRewardPlanIdTx 按站点解析奖励套餐配置。
// 服务商站点永远不会继承主站的订阅奖励套餐。
func getSubscriptionRewardPlanIdTx(tx *gorm.DB, providerId int, registerGift bool) (int, error) {
	if providerId <= 0 {
		if registerGift {
			return common.RegisterGiftSubscriptionPlanId, nil
		}
		return common.AirdropSubscriptionPlanId, nil
	}
	var cfg ProviderRewardConfig
	query := tx.Where("provider_id = ?", providerId).First(&cfg)
	if errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if query.Error != nil {
		return 0, query.Error
	}
	if registerGift {
		return cfg.RegisterGiftSubscriptionPlanId, nil
	}
	return cfg.AirdropSubscriptionPlanId, nil
}

// parseOptionalSubscriptionProvider parses the optional provider scope used by
// subscription read APIs.  Keeping the argument variadic preserves the old
// call signature for callers that need a user's subscriptions across all
// providers, while allowing request-scoped callers to enforce tenant
// isolation.  Provider id zero is a valid (main-site) scope, so presence must
// be tracked separately from the value itself.
func parseOptionalSubscriptionProvider(providerIds []int) (int, bool, error) {
	switch len(providerIds) {
	case 0:
		return 0, false, nil
	case 1:
		if providerIds[0] < 0 {
			return 0, false, errors.New("invalid providerId")
		}
		return providerIds[0], true, nil
	default:
		return 0, false, errors.New("multiple providerIds are not supported")
	}
}

// GetAllActiveUserSubscriptions returns all active subscriptions for a user.
// When providerIds is supplied, results are scoped to that provider.  The
// variadic form keeps backwards compatibility with existing admin/internal
// callers that intentionally need the complete history.
func GetAllActiveUserSubscriptions(userId int, providerIds ...int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	providerId, scoped, err := parseOptionalSubscriptionProvider(providerIds)
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	var subs []UserSubscription
	// Treat NULL/zero start_time as immediately active for pre-migration rows,
	// but do not expose subscriptions scheduled for a future start.
	query := DB.Where("user_id = ? AND status = ? AND COALESCE(start_time, 0) <= ? AND end_time > ?", userId, "active", now, now)
	if scoped {
		query = query.Where("provider_id = ?", providerId)
	}
	err = query.Order("end_time desc, id desc").Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// HasActiveUserSubscription returns whether the user has any active subscription.
// This is a lightweight existence check to avoid heavy pre-consume transactions.
func HasActiveUserSubscription(userId int, providerIds ...int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	providerId, scoped, err := parseOptionalSubscriptionProvider(providerIds)
	if err != nil {
		return false, err
	}
	now := common.GetTimestamp()
	var count int64
	query := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND COALESCE(start_time, 0) <= ? AND end_time > ?", userId, "active", now, now)
	if scoped {
		query = query.Where("provider_id = ?", providerId)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAllUserSubscriptions returns all subscriptions (active and expired) for a user.
func GetAllUserSubscriptions(userId int, providerIds ...int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	providerId, scoped, err := parseOptionalSubscriptionProvider(providerIds)
	if err != nil {
		return nil, err
	}
	var subs []UserSubscription
	query := DB.Where("user_id = ?", userId)
	if scoped {
		query = query.Where("provider_id = ?", providerId)
	}
	err = query.Order("end_time desc, id desc").Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// buildSubscriptionSummaries 批量构建订阅摘要列表，附带套餐信息。
// 优化：先收集所有订阅中涉及的去重 planId，然后一次性批量查询套餐，
// 避免 N+1 查询问题。对于已删除的套餐，使用发行时快照构造最小 Plan
// 对象，确保 active subscription API 仍能展示标题/策略并继续计费。
func buildSubscriptionSummaries(subs []UserSubscription) []SubscriptionSummary {
	if len(subs) == 0 {
		return []SubscriptionSummary{}
	}
	// 第一步：收集去重后的 planId 列表
	planIds := make([]int, 0, len(subs))
	seenPlanIds := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.PlanId <= 0 {
			continue
		}
		if _, ok := seenPlanIds[sub.PlanId]; ok {
			continue
		}
		seenPlanIds[sub.PlanId] = struct{}{}
		planIds = append(planIds, sub.PlanId)
	}
	// 第二步：批量查询套餐，构建 id -> plan 映射
	planMap := make(map[int]*SubscriptionPlan, len(planIds))
	if len(planIds) > 0 {
		var plans []SubscriptionPlan
		if err := DB.Where("id IN ?", planIds).Find(&plans).Error; err == nil {
			for i := range plans {
				plans[i].NormalizeQuotaWindows()
				planMap[plans[i].Id] = &plans[i]
			}
		}
	}
	// 第三步：组装结果，每个订阅带上对应的套餐信息
	result := make([]SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		plan := planMap[sub.PlanId]
		if plan == nil && sub.PlanPolicySnapshotVersion > 0 {
			plan = subscriptionPlanSnapshotFallback(&subCopy)
		} else {
			plan = subscriptionPlanForSummary(plan, &subCopy)
		}
		// Usage is derived from request records so list responses match the
		// same rolling-window accounting used by pre-consume.
		_ = PopulateSubscriptionUsage(&subCopy, plan, GetDBTimestamp())
		result = append(result, SubscriptionSummary{
			Subscription: &subCopy,
			Plan:         plan,
		})
	}
	return result
}

// AdminInvalidateUserSubscription marks a user subscription as cancelled and ends it immediately.
func AdminInvalidateUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		if err := tx.Model(&sub).Updates(map[string]interface{}{
			"status":     "cancelled",
			"end_time":   now,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(userSubscriptionId int) (string, error) {
	if userSubscriptionId <= 0 {
		return "", errors.New("invalid userSubscriptionId")
	}
	now := common.GetTimestamp()
	cacheGroup := ""
	downgradeGroup := ""
	var userId int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		userId = sub.UserId
		target, err := downgradeUserGroupForSubscriptionTx(tx, &sub, now)
		if err != nil {
			return err
		}
		if target != "" {
			cacheGroup = target
			downgradeGroup = target
		}
		if err := tx.Where("id = ?", userSubscriptionId).Delete(&UserSubscription{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if cacheGroup != "" && userId > 0 {
		_ = UpdateUserGroupCache(userId, cacheGroup)
	}
	if downgradeGroup != "" {
		return fmt.Sprintf("用户分组将回退到 %s", downgradeGroup), nil
	}
	return "", nil
}

type SubscriptionPreConsumeResult struct {
	UserSubscriptionId int
	PreConsumed        int64
	AmountTotal        int64
	AmountUsedBefore   int64
	AmountUsedAfter    int64
	QuotaWindowMode    string
	WeeklyLimit        int64
	WeeklyUsed         int64
	FiveHourLimit      int64
	FiveHourUsed       int64
	QuotaWindows       []SubscriptionQuotaWindowSnapshot `json:"quota_windows,omitempty"`
}

// ExpireDueSubscriptions marks expired subscriptions and handles group downgrade.
func ExpireDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("status = ? AND end_time > 0 AND end_time <= ?", "active", now).
		Order("end_time asc, id asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	expiredCount := 0
	userIds := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.UserId > 0 {
			userIds[sub.UserId] = struct{}{}
		}
	}
	for userId := range userIds {
		cacheGroup := ""
		err := DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&UserSubscription{}).
				Where("user_id = ? AND status = ? AND end_time > 0 AND end_time <= ?", userId, "active", now).
				Updates(map[string]interface{}{
					"status":     "expired",
					"updated_at": common.GetTimestamp(),
				})
			if res.Error != nil {
				return res.Error
			}
			expiredCount += int(res.RowsAffected)

			// If there's an active upgraded subscription, keep current group.
			var activeSub UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND COALESCE(start_time, 0) <= ? AND end_time > ? AND upgrade_group <> ''",
				userId, "active", now, now).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
				return nil
			}

			// No active upgraded subscription, downgrade to previous group if needed.
			var lastExpired UserSubscription
			expiredQuery := tx.Where("user_id = ? AND status = ? AND upgrade_group <> ''",
				userId, "expired").
				Order("end_time desc, id desc").
				Limit(1).
				Find(&lastExpired)
			if expiredQuery.Error != nil || expiredQuery.RowsAffected == 0 {
				return nil
			}
			upgradeGroup := strings.TrimSpace(lastExpired.UpgradeGroup)
			prevGroup := strings.TrimSpace(lastExpired.PrevUserGroup)
			if upgradeGroup == "" || prevGroup == "" {
				return nil
			}
			currentGroup, err := getUserGroupByIdTx(tx, userId)
			if err != nil {
				return err
			}
			if currentGroup != upgradeGroup || currentGroup == prevGroup {
				return nil
			}
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("group", prevGroup).Error; err != nil {
				return err
			}
			cacheGroup = prevGroup
			return nil
		})
		if err != nil {
			return expiredCount, err
		}
		if cacheGroup != "" {
			_ = UpdateUserGroupCache(userId, cacheGroup)
		}
	}
	return expiredCount, nil
}

// SubscriptionPreConsumeRecord stores idempotent pre-consume operations per request.
type SubscriptionPreConsumeRecord struct {
	Id                 int    `json:"id"`
	RequestId          string `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	UserId             int    `json:"user_id" gorm:"index"`
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index;index:idx_sub_pre_consume_window,priority:1"`
	ModelName          string `json:"model_name" gorm:"type:varchar(255);default:'';index"`
	QuotaType          int    `json:"quota_type" gorm:"type:int;default:0"`
	PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	// SettledAmount is the request's current effective usage. NULL on old rows
	// means that PreConsumed is the best available estimate.
	SettledAmount *int64 `json:"settled_amount,omitempty" gorm:"type:bigint"`
	SettledAt     int64  `json:"settled_at,omitempty" gorm:"bigint;default:0"`
	Status        string `json:"status" gorm:"type:varchar(32);index;index:idx_sub_pre_consume_window,priority:2"` // consumed/refunded
	CreatedAt     int64  `json:"created_at" gorm:"bigint;index:idx_sub_pre_consume_window,priority:3"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := getDBTimestampTx(tx)
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = getDBTimestampTx(tx)
	return nil
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	policy := quotaPolicyForSubscription(sub, plan)
	if policy.invalid {
		return invalidSubscriptionQuotaPolicyError()
	}
	if policy.resetPeriod == SubscriptionResetNever {
		return nil
	}
	// Rolling/generic windows derive usage from request records and must not
	// stay queued forever in the aggregate reset task.
	if !policyTracksAggregate(policy) {
		if sub.NextResetTime != 0 || sub.LastResetTime != 0 {
			sub.NextResetTime = 0
			sub.LastResetTime = 0
			return tx.Save(sub).Error
		}
		return nil
	}
	if sub.NextResetTime > 0 && sub.NextResetTime > now {
		return nil
	}
	baseUnix := sub.LastResetTime
	if baseUnix <= 0 {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextResetTimeForPolicy(base, policy, sub.EndTime)
	advanced := false
	for next > 0 && next <= now {
		advanced = true
		base = time.Unix(next, 0)
		next = calcNextResetTimeForPolicy(base, policy, sub.EndTime)
	}
	if !advanced {
		if sub.NextResetTime == 0 && next > 0 {
			sub.NextResetTime = next
			sub.LastResetTime = base.Unix()
			return tx.Save(sub).Error
		}
		return nil
	}
	sub.AmountUsed = 0
	sub.LastResetTime = base.Unix()
	sub.NextResetTime = next
	return tx.Save(sub).Error
}

// genericQuotaAvailableTx checks every configured generic window.  Windows
// are conjunctive: a request must fit in all of them (for example, both a
// daily and a monthly allowance).  Usage is derived from request records so
// rolling and calendar windows remain correct across process restarts.
func genericQuotaAvailableTx(tx *gorm.DB, sub *UserSubscription, policy subscriptionQuotaPolicy, now, amount int64) (bool, error) {
	if !policyHasGenericWindows(policy) {
		return true, nil
	}
	if amount <= 0 {
		return false, errors.New("amount must be > 0")
	}
	anchor := int64(0)
	if sub != nil {
		anchor = sub.StartTime
	}
	for _, window := range policy.windows {
		if window.amount <= 0 {
			continue
		}
		used, _, err := recentSubscriptionWindowUsageTx(tx, sub.Id, now, window, anchor)
		if err != nil {
			return false, err
		}
		if used > window.amount || amount > window.amount-used {
			return false, nil
		}
	}
	return true, nil
}

// quotaWindowTargetAllowedTx validates the final amount of an already-created
// request in every historical interval to which that request belongs. This is
// deliberately different from an admission check at settlement time: a slow
// request may settle after its original calendar/rolling window has ended, but
// increasing its final amount must not make that historical window exceed its
// configured limit.
func quotaWindowTargetAllowedTx(tx *gorm.DB, subscriptionID int, window subscriptionQuotaWindowPolicy, anchor int64, record *SubscriptionPreConsumeRecord, target, now int64) (bool, error) {
	if tx == nil || subscriptionID <= 0 || record == nil {
		return false, errors.New("invalid subscription quota settlement window")
	}
	if target < 0 {
		return false, errors.New("actual amount must be >= 0")
	}
	if window.amount <= 0 {
		return true, nil
	}
	if window.windowSeconds <= 0 || window.windowSeconds > MaxSubscriptionQuotaWindowSeconds {
		return false, errors.New("invalid subscription quota window duration")
	}
	if window.resetMode != SubscriptionQuotaWindowRolling && window.resetMode != SubscriptionQuotaWindowCalendar {
		return false, errors.New("invalid subscription quota window reset mode")
	}
	// Rolling windows historically omitted the unit because the exact
	// windowSeconds value is sufficient (the fixed five-hour compatibility
	// path still constructs such policies in older callers).  Preserve that
	// empty-unit form, but reject every non-empty unknown unit and all calendar
	// windows without a canonical unit.  Without this guard a malformed
	// calendar row reaches quotaWindowBounds, receives an empty [now, now)
	// interval, and the historical settlement check can incorrectly admit an
	// amount after silently querying no usage.
	if (window.resetMode == SubscriptionQuotaWindowCalendar && !isValidSubscriptionQuotaWindowUnit(window.unit)) ||
		(window.resetMode == SubscriptionQuotaWindowRolling && window.unit != "" && !isValidSubscriptionQuotaWindowUnit(window.unit)) {
		return false, errors.New("invalid subscription quota window unit")
	}
	if record.CreatedAt <= 0 || record.CreatedAt > now {
		return false, errors.New("invalid subscription quota record time")
	}
	if window.unit == SubscriptionQuotaWindowCustom &&
		window.resetMode == SubscriptionQuotaWindowCalendar &&
		(anchor <= 0 || anchor > record.CreatedAt) {
		return false, errors.New("invalid subscription quota window anchor")
	}
	if target > window.amount {
		return false, nil
	}
	remainingForOthers := window.amount - target

	if window.resetMode == SubscriptionQuotaWindowCalendar {
		// Resolve the bucket using the request timestamp, not the settlement
		// timestamp. Query through now so later requests in the same historical
		// bucket are included as well.
		start, end := quotaWindowBounds(window, record.CreatedAt, anchor)
		var records []SubscriptionPreConsumeRecord
		err := tx.Where("user_subscription_id = ? AND status <> ? AND request_id <> ?", subscriptionID, "refunded", record.RequestId).
			Where("created_at >= ? AND created_at < ? AND created_at <= ?", start, end, now).
			Find(&records).Error
		if err != nil {
			return false, err
		}
		var used int64
		for i := range records {
			amount := effectivePreConsumeAmount(&records[i])
			if amount <= 0 {
				continue
			}
			if used > remainingForOthers || amount > remainingForOthers-used {
				return false, nil
			}
			used += amount
		}
		return true, nil
	}

	// A rolling request created at r contributes to every interval evaluated at
	// t where r <= t < r+W. Usage changes only when another request is created
	// (expiry events only decrease it), so it is sufficient to check t=r and
	// every later request creation timestamp while the current request remains
	// in the rolling window.
	recordTime := record.CreatedAt
	lower, lowerOK := subscriptionSafeSubInt64(recordTime, window.windowSeconds)
	if !lowerOK {
		// Saturating to MinInt64 preserves the complete representable history;
		// wrapping to a positive timestamp would incorrectly omit prior usage.
		lower = -int64(^uint64(0)>>1) - 1
	}
	upper, upperOK := subscriptionSafeAddInt64(recordTime, window.windowSeconds)
	if !upperOK {
		upper = int64(^uint64(0) >> 1)
	}
	var records []SubscriptionPreConsumeRecord
	err := tx.Where("user_subscription_id = ? AND status <> ? AND request_id <> ?", subscriptionID, "refunded", record.RequestId).
		Where("created_at > ? AND created_at < ? AND created_at <= ?", lower, upper, now).
		Order("created_at ASC, id ASC").Find(&records).Error
	if err != nil {
		return false, err
	}

	left, right := 0, 0
	var used int64
	addUntil := func(timestamp int64) bool {
		for right < len(records) && records[right].CreatedAt <= timestamp {
			amount := effectivePreConsumeAmount(&records[right])
			if amount > 0 {
				if used > remainingForOthers || amount > remainingForOthers-used {
					return false
				}
				used += amount
			}
			right++
		}
		return true
	}

	// At the request timestamp, every queried record at or before r is inside
	// (r-W, r].
	if !addUntil(recordTime) {
		return false, nil
	}
	for right < len(records) {
		criticalTime := records[right].CreatedAt
		windowStart, windowStartOK := subscriptionSafeSubInt64(criticalTime, window.windowSeconds)
		if !windowStartOK {
			windowStart = -int64(^uint64(0)>>1) - 1
		}
		for left < right && records[left].CreatedAt <= windowStart {
			amount := effectivePreConsumeAmount(&records[left])
			if amount > 0 {
				used -= amount
				if used < 0 {
					used = 0
				}
			}
			left++
		}
		if !addUntil(criticalTime) {
			return false, nil
		}
	}
	return true, nil
}

func genericQuotaTargetAllowedTx(tx *gorm.DB, sub *UserSubscription, policy subscriptionQuotaPolicy, record *SubscriptionPreConsumeRecord, target, now int64) (bool, error) {
	if !policyHasGenericWindows(policy) {
		return true, nil
	}
	if target < 0 {
		return false, errors.New("actual amount must be >= 0")
	}
	if sub == nil || record == nil {
		return false, errors.New("invalid subscription quota settlement")
	}
	anchor := sub.StartTime
	for _, window := range policy.windows {
		if window.amount <= 0 {
			continue
		}
		allowed, err := quotaWindowTargetAllowedTx(tx, sub.Id, window, anchor, record, target, now)
		if err != nil {
			return false, err
		}
		if !allowed {
			return false, nil
		}
	}
	return true, nil
}

// populatePreConsumeResultTx fills the quota snapshot returned by a
// pre-consume operation.  Generic windows are record-backed (rather than
// represented by UserSubscription.AmountUsed), so the result must query the
// same window usage that admission/settlement use.  Keeping this in the
// transaction also makes the returned snapshot include the request that was
// just inserted.
func populatePreConsumeResultTx(tx *gorm.DB, result *SubscriptionPreConsumeResult, sub *UserSubscription, policy subscriptionQuotaPolicy, now int64) error {
	if result == nil || sub == nil {
		return nil
	}
	if now <= 0 {
		now = getDBTimestampTx(tx)
	}
	result.QuotaWindowMode = policy.mode
	result.WeeklyLimit = policyWeeklyLimit(policy, sub)
	result.WeeklyUsed = sub.AmountUsed
	result.FiveHourLimit = policy.fiveHourAmount
	result.FiveHourUsed = 0
	if policyHasFiveHour(policy) && tx != nil && sub.Id > 0 {
		used, _, err := recentSubscriptionUsageTx(tx, sub.Id, now, policy.fiveHourWindowSeconds)
		if err != nil {
			return err
		}
		result.FiveHourUsed = used
	}
	if policyHasGenericWindows(policy) {
		result.QuotaWindows = make([]SubscriptionQuotaWindowSnapshot, 0, len(policy.windows))
		for _, window := range policy.windows {
			used, resetAt := int64(0), int64(0)
			if tx != nil && sub.Id > 0 {
				var err error
				used, resetAt, err = recentSubscriptionWindowUsageTx(tx, sub.Id, now, window, sub.StartTime)
				if err != nil {
					return err
				}
			} else {
				_, resetAt = quotaWindowBounds(window, now, sub.StartTime)
			}
			result.QuotaWindows = append(result.QuotaWindows, SubscriptionQuotaWindowSnapshot{
				Type: window.unit, Period: window.unit, Duration: window.duration,
				ResetMode: window.resetMode, Name: window.name, Limit: window.amount, Used: used,
				Remaining: maxInt64(0, window.amount-used), ResetAt: resetAt,
				WindowSeconds: quotaWindowEffectiveSeconds(window, now, sub.StartTime), UsedPercent: quotaUsedPercent(used, window.amount),
			})
		}
	}
	return nil
}

// populatePreConsumeResult is retained for callers that only have an
// in-memory policy.  Transactional pre-consume paths use the Tx variant above
// so returned usage is accurate.
func populatePreConsumeResult(result *SubscriptionPreConsumeResult, sub *UserSubscription, policy subscriptionQuotaPolicy) {
	_ = populatePreConsumeResultTx(nil, result, sub, policy, 0)
}

func validateSubscriptionPreConsumeRetry(record *SubscriptionPreConsumeRecord, userID int, modelName string, quotaType int, amount int64) error {
	if record == nil {
		return errors.New("subscription pre-consume record is nil")
	}
	if record.UserId != userID {
		return errors.New("subscription request id belongs to another user")
	}
	// Rows created before model/quota-type snapshots were introduced have both
	// fields empty and SettledAt=0. Preserve idempotency for those legacy rows;
	// all new rows must match the complete request shape exactly.
	legacyShape := record.ModelName == "" && record.QuotaType == 0 && record.SettledAt == 0
	if !legacyShape && record.ModelName != modelName {
		return errors.New("subscription request id belongs to another model")
	}
	if !legacyShape && record.QuotaType != quotaType {
		return errors.New("subscription request id belongs to another quota type")
	}
	if record.PreConsumed != amount {
		return fmt.Errorf("subscription request amount mismatch, existing=%d requested=%d", record.PreConsumed, amount)
	}
	switch record.Status {
	case "consumed":
		return nil
	case "refunded":
		return errors.New("subscription pre-consume already refunded")
	default:
		return fmt.Errorf("subscription pre-consume status invalid: %s", record.Status)
	}
}

func populateIdempotentPreConsumeResultTx(tx *gorm.DB, result *SubscriptionPreConsumeResult, record *SubscriptionPreConsumeRecord) error {
	if tx == nil || result == nil || record == nil {
		return errors.New("invalid subscription pre-consume result")
	}
	var sub UserSubscription
	if err := tx.Where("id = ?", record.UserSubscriptionId).First(&sub).Error; err != nil {
		return err
	}
	result.UserSubscriptionId = sub.Id
	result.PreConsumed = effectivePreConsumeAmount(record)
	result.AmountTotal = sub.AmountTotal
	result.AmountUsedBefore = sub.AmountUsed
	result.AmountUsedAfter = sub.AmountUsed
	if plan, planErr := getSubscriptionPlanOrSnapshotTx(tx, &sub); planErr == nil {
		if err := populatePreConsumeResultTx(tx, result, &sub, quotaPolicyForSubscription(&sub, plan), getDBTimestampTx(tx)); err != nil {
			return err
		}
	}
	return nil
}

// PreConsumeUserSubscription 从用户的活跃订阅中预扣配额（幂等）。
//
// 扣费逻辑（按 end_time asc 顺序遍历所有活跃订阅）：
//  1. 首先检查幂等：同一 requestId 已存在预扣记录则直接返回。
//  2. 遍历用户的活跃订阅列表（按到期时间升序，优先消耗先到期的订阅）。
//  3. 对每个订阅，通过 AllowsModel 检查该套餐是否允许当前请求的模型。
//     - 如果套餐有 ModelLimits 白名单且不包含当前模型，跳过该订阅。
//  4. 检查是否需要重置配额（maybeResetUserSubscriptionWithPlanTx）。
//  5. 检查剩余配额是否足够（AmountTotal > 0 时 remain >= amount）。
//  6. 创建 SubscriptionPreConsumeRecord 幂等记录并更新 AmountUsed。
//
// 返回值 SubscriptionPreConsumeResult 包含扣费前后的状态快照，供 SettleBilling 使用。
func PreConsumeUserSubscription(requestId string, userId int, modelName string, quotaType int, amount int64) (*SubscriptionPreConsumeResult, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestId) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}
	modelName = strings.TrimSpace(modelName)
	now := GetDBTimestamp()
	result := &SubscriptionPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		// request_id is globally unique. Validate its owner and request shape
		// before returning an idempotent result so it cannot be replayed by a
		// different user or model.
		var existing SubscriptionPreConsumeRecord
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if err := validateSubscriptionPreConsumeRetry(&existing, userId, modelName, quotaType, amount); err != nil {
				return err
			}
			return populateIdempotentPreConsumeResultTx(tx, result, &existing)
		}
		userProviderId, err := getUserProviderIdByIdTx(tx, userId)
		if err != nil {
			return err
		}
		var subs []UserSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND provider_id = ? AND status = ? AND COALESCE(start_time, 0) <= ? AND end_time > ?",
				userId, userProviderId, "active", now, now).
			Order("end_time asc, id asc").Find(&subs).Error; err != nil {
			// Preserve the database error.  Treating an unavailable database as
			// "no active subscription" makes subscription_first/wallet fallback
			// silently charge the wallet (or report an exhausted quota) and hides a
			// transient outage from operators.  Only an empty, successful query is
			// the genuine no-subscription case below.
			return err
		}
		if len(subs) == 0 {
			return errors.New("no active subscription")
		}

		for i := range subs {
			sub := &subs[i]
			plan, err := getSubscriptionPlanOrSnapshotTx(tx, sub)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if plan == nil || plan.ProviderId != sub.ProviderId || !plan.AllowsModel(modelName) {
				continue
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, sub, plan, now); err != nil {
				return err
			}
			policy := quotaPolicyForSubscription(sub, plan)
			if policy.invalid {
				return invalidSubscriptionQuotaPolicyError()
			}
			weeklyLimit := policyWeeklyLimit(policy, sub)
			tracksAggregate := policyTracksAggregate(policy)
			if tracksAggregate && sub.AmountUsed < 0 {
				// A negative persisted aggregate would effectively grant extra
				// quota (and can make limit-sub.AmountUsed overflow).  Treat the
				// corrupt row as unusable rather than normalizing it silently.
				return errors.New("invalid subscription aggregate usage")
			}
			if tracksAggregate && weeklyLimit > 0 && sub.AmountUsed > weeklyLimit {
				// A plan edit cannot make an already-issued subscription spend
				// above its snapshotted limit.
				sub.AmountUsed = weeklyLimit
			}
			if tracksAggregate && weeklyLimit > 0 && (sub.AmountUsed > weeklyLimit || amount > weeklyLimit-sub.AmountUsed) {
				continue
			}
			if policyHasGenericWindows(policy) {
				available, err := genericQuotaAvailableTx(tx, sub, policy, now, amount)
				if err != nil {
					return err
				}
				if !available {
					continue
				}
			}
			if policyHasFiveHour(policy) {
				fiveUsed, _, err := recentSubscriptionUsageTx(tx, sub.Id, now, policy.fiveHourWindowSeconds)
				if err != nil {
					return err
				}
				if fiveUsed > policy.fiveHourAmount || amount > policy.fiveHourAmount-fiveUsed {
					continue
				}
			}

			usedBefore := sub.AmountUsed
			settledAmount := amount
			record := &SubscriptionPreConsumeRecord{
				RequestId:          requestId,
				UserId:             userId,
				UserSubscriptionId: sub.Id,
				ModelName:          modelName,
				QuotaType:          quotaType,
				PreConsumed:        amount,
				SettledAmount:      &settledAmount,
				SettledAt:          now,
				Status:             "consumed",
			}
			createResult := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "request_id"}},
				DoNothing: true,
			}).Create(record)
			if createResult.Error != nil {
				return createResult.Error
			}
			if createResult.RowsAffected == 0 {
				// OnConflict/DoNothing keeps PostgreSQL transactions usable after a
				// concurrent request_id retry; a raw duplicate-key error would abort
				// the transaction before the idempotent row could be loaded.
				var duplicate SubscriptionPreConsumeRecord
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("request_id = ?", requestId).First(&duplicate).Error; err != nil {
					return err
				}
				if err := validateSubscriptionPreConsumeRetry(&duplicate, userId, modelName, quotaType, amount); err != nil {
					return err
				}
				return populateIdempotentPreConsumeResultTx(tx, result, &duplicate)
			}
			if tracksAggregate {
				nextUsed, ok := subscriptionSafeAddInt64(sub.AmountUsed, amount)
				if !ok {
					return errors.New("subscription aggregate usage overflows int64")
				}
				sub.AmountUsed = nextUsed
				if err := tx.Save(sub).Error; err != nil {
					return err
				}
			}
			result.UserSubscriptionId = sub.Id
			result.PreConsumed = amount
			result.AmountTotal = sub.AmountTotal
			result.AmountUsedBefore = usedBefore
			result.AmountUsedAfter = sub.AmountUsed
			if err := populatePreConsumeResultTx(tx, result, sub, policy, now); err != nil {
				return err
			}
			return nil
		}
		return fmt.Errorf("subscription quota insufficient, need=%d", amount)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SettleSubscriptionPreConsume records the final effective amount for one
// request. Repeating the same final amount is idempotent; changing it applies
// only the difference to the applicable aggregate/window usage.
func SettleSubscriptionPreConsume(requestID string, actualAmount int64) error {
	if strings.TrimSpace(requestID) == "" {
		return errors.New("requestId is empty")
	}
	if actualAmount < 0 {
		return errors.New("actual amount must be >= 0")
	}
	now := GetDBTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", requestID).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return errors.New("subscription pre-consume already refunded")
		}
		return settleSubscriptionPreConsumeTx(tx, &record, actualAmount, now)
	})
}

// AdjustSubscriptionPreConsume applies a delta to the current effective amount.
// It is kept for asynchronous/legacy callers that naturally operate in deltas.
func AdjustSubscriptionPreConsume(requestID string, delta int64) error {
	if strings.TrimSpace(requestID) == "" {
		return errors.New("requestId is empty")
	}
	if delta == 0 {
		return nil
	}
	now := GetDBTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_id = ?", requestID).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return errors.New("subscription pre-consume already refunded")
		}
		current := effectivePreConsumeAmount(&record)
		target, ok := subscriptionSafeAddInt64(current, delta)
		if !ok {
			return errors.New("subscription adjustment overflows int64")
		}
		if target < 0 {
			target = 0
		}
		return settleSubscriptionPreConsumeTx(tx, &record, target, now)
	})
}

func settleSubscriptionPreConsumeTx(tx *gorm.DB, record *SubscriptionPreConsumeRecord, target, now int64) error {
	if tx == nil || record == nil {
		return errors.New("invalid subscription settlement")
	}
	if target < 0 {
		return errors.New("actual amount must be >= 0")
	}
	current := effectivePreConsumeAmount(record)
	if current == target {
		if record.SettledAmount == nil || record.SettledAt == 0 {
			value := target
			record.SettledAmount = &value
			record.SettledAt = now
			return tx.Save(record).Error
		}
		return nil
	}
	var sub UserSubscription
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", record.UserSubscriptionId).First(&sub).Error; err != nil {
		return err
	}
	plan, err := getSubscriptionPlanOrSnapshotTx(tx, &sub)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		plan = nil // the subscription snapshot is sufficient for settlement
	}
	if plan != nil {
		// A long-running request may cross a reset boundary. Advance the
		// aggregate state before applying a settlement delta.
		if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
			return err
		}
	}
	policy := quotaPolicyForSubscription(&sub, plan)
	if policy.invalid {
		return invalidSubscriptionQuotaPolicyError()
	}
	if policyTracksAggregate(policy) && sub.AmountUsed < 0 {
		return errors.New("invalid subscription aggregate usage")
	}
	delta := target - current
	tracksAggregatePolicy := policyTracksAggregate(policy)
	tracksAggregate := tracksAggregatePolicy && aggregateRecordInCurrentPeriod(&sub, policy, record.CreatedAt, now)
	if delta > 0 && tracksAggregatePolicy {
		if tracksAggregate {
			limit := policyWeeklyLimit(policy, &sub)
			if limit > 0 && (sub.AmountUsed > limit || delta > limit-sub.AmountUsed) {
				return fmt.Errorf("subscription aggregate quota insufficient, need=%d remaining=%d", delta, maxInt64(0, limit-sub.AmountUsed))
			}
		} else {
			allowed, err := aggregateQuotaHistoricalTargetAllowedTx(tx, &sub, policy, record, target, now)
			if err != nil {
				return err
			}
			if !allowed {
				return fmt.Errorf("subscription historical aggregate quota insufficient, need=%d", delta)
			}
		}
	}
	if delta > 0 && policyHasFiveHour(policy) {
		window := subscriptionQuotaWindowPolicy{unit: SubscriptionQuotaWindowHour, amount: policy.fiveHourAmount,
			duration: 5, windowSeconds: policy.fiveHourWindowSeconds, resetMode: SubscriptionQuotaWindowRolling}
		allowed, err := quotaWindowTargetAllowedTx(tx, sub.Id, window, sub.StartTime, record, target, now)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("subscription five-hour quota insufficient, need=%d", delta)
		}
	}
	if delta > 0 && policyHasGenericWindows(policy) {
		allowed, err := genericQuotaTargetAllowedTx(tx, &sub, policy, record, target, now)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("subscription quota insufficient, window limit reached, need=%d", delta)
		}
	}
	if tracksAggregate {
		nextUsed, ok := subscriptionSafeAddInt64(sub.AmountUsed, delta)
		if !ok {
			return errors.New("subscription aggregate usage overflows int64")
		}
		if nextUsed < 0 {
			nextUsed = 0
		}
		sub.AmountUsed = nextUsed
	}
	value := target
	record.SettledAmount = &value
	record.SettledAt = now
	if err := tx.Save(record).Error; err != nil {
		return err
	}
	if tracksAggregate {
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
	}
	return nil
}

// refundSubscriptionPreConsumeTx performs the idempotent subscription refund
// inside an existing transaction.  Keeping the row locks and usage update in
// the caller's transaction is important for asynchronous tasks such as
// Midjourney: the task marker, subscription usage, and token balance must be
// committed (or rolled back) together so a crash cannot produce a duplicate
// refund or strand a partially completed one.
func refundSubscriptionPreConsumeTx(tx *gorm.DB, requestID string, now int64) error {
	if tx == nil {
		return errors.New("database transaction is nil")
	}
	if strings.TrimSpace(requestID) == "" {
		return errors.New("requestId is empty")
	}
	var record SubscriptionPreConsumeRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("request_id = ?", requestID).First(&record).Error; err != nil {
		return err
	}
	if record.Status == "refunded" {
		return nil
	}
	var sub UserSubscription
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", record.UserSubscriptionId).First(&sub).Error; err != nil {
		// An administrator may hard-delete a subscription while an upstream
		// request is still in flight. The pre-consume record is intentionally
		// retained for idempotency, but there is no aggregate row left to roll
		// back. Treat that state as reclaimed and mark the record refunded so
		// retries cannot strand it forever.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			zero := int64(0)
			record.SettledAmount = &zero
			record.SettledAt = now
			record.Status = "refunded"
			return tx.Save(&record).Error
		}
		return err
	}
	amount := effectivePreConsumeAmount(&record)
	plan, err := getSubscriptionPlanOrSnapshotTx(tx, &sub)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		plan = nil
	}
	if plan != nil {
		if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
			return err
		}
	}
	policy := quotaPolicyForSubscription(&sub, plan)
	if policyTracksAggregate(policy) && sub.AmountUsed < 0 {
		return errors.New("invalid subscription aggregate usage")
	}
	if amount > 0 && policyTracksAggregate(policy) && aggregateRecordInCurrentPeriod(&sub, policy, record.CreatedAt, now) {
		nextUsed, ok := subscriptionSafeAddInt64(sub.AmountUsed, -amount)
		if !ok {
			return errors.New("subscription aggregate usage overflows int64")
		}
		sub.AmountUsed = maxInt64(0, nextUsed)
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
	}
	zero := int64(0)
	record.SettledAmount = &zero
	record.SettledAt = now
	record.Status = "refunded"
	return tx.Save(&record).Error
}

// RefundSubscriptionPreConsume is idempotent and refunds pre-consumed
// subscription quota by requestId.
func RefundSubscriptionPreConsume(requestID string) error {
	if strings.TrimSpace(requestID) == "" {
		return errors.New("requestId is empty")
	}
	now := GetDBTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		return refundSubscriptionPreConsumeTx(tx, requestID, now)
	})
}

// ResetDueSubscriptions resets subscriptions whose next_reset_time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ? AND COALESCE(start_time, 0) <= ?", now, "active", now).
		Order("next_reset_time asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}
	resetCount := 0
	for _, sub := range subs {
		// Re-read the subscription and its plan while holding the row lock.  The
		// previous implementation resolved the plan (often from cache) before
		// opening this transaction, so a concurrent plan edit or subscription
		// cancellation could reset the row using stale policy data.  Resolving the
		// snapshot inside the transaction keeps reset semantics consistent with
		// pre-consume and settlement.
		subID := sub.Id
		err := DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND status = ? AND COALESCE(start_time, 0) <= ? AND next_reset_time > 0 AND next_reset_time <= ?", subID, "active", now, now).
				First(&locked).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			plan, planErr := getSubscriptionPlanOrSnapshotTx(tx, &locked)
			if planErr != nil || plan == nil {
				// Legacy rows without a plan/snapshot cannot be reset safely; leave
				// them untouched and let the ordinary expiry/accounting paths handle
				// them, matching the historical best-effort sweeper behaviour.
				return nil
			}
			beforeAmount, beforeLast, beforeNext := locked.AmountUsed, locked.LastResetTime, locked.NextResetTime
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now); err != nil {
				return err
			}
			if locked.AmountUsed != beforeAmount || locked.LastResetTime != beforeLast || locked.NextResetTime != beforeNext {
				resetCount++
			}
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

const defaultSubscriptionPreConsumeRetentionSeconds int64 = 7 * 24 * 60 * 60

func quotaRecordBackedWindows(policy subscriptionQuotaPolicy) []subscriptionQuotaWindowPolicy {
	if policyHasGenericWindows(policy) {
		return policy.windows
	}
	if policyHasFiveHour(policy) {
		return []subscriptionQuotaWindowPolicy{{
			unit: SubscriptionQuotaWindowHour, amount: policy.fiveHourAmount,
			duration: 5, windowSeconds: policy.fiveHourWindowSeconds,
			resetMode: SubscriptionQuotaWindowRolling,
		}}
	}
	return nil
}

// subscriptionRecordSafeBefore returns the earliest timestamp that can still
// contribute to one active subscription's record-backed quota windows. It is
// calculated per subscription so one multi-year plan does not force unrelated
// subscriptions to retain all request records for the same duration.
func subscriptionRecordSafeBefore(sub *UserSubscription, plan *SubscriptionPlan, now int64) int64 {
	if sub == nil || sub.Status != "active" || (sub.EndTime > 0 && sub.EndTime <= now) {
		return now
	}
	policy := quotaPolicyForSubscription(sub, plan)
	safeBefore := now
	for _, window := range quotaRecordBackedWindows(policy) {
		if window.windowSeconds <= 0 || window.windowSeconds > MaxSubscriptionQuotaWindowSeconds {
			continue
		}
		start, _ := quotaWindowBounds(window, now, sub.StartTime)
		if start < safeBefore {
			safeBefore = start
		}
	}
	return safeBefore
}

// CleanupSubscriptionPreConsumeRecords removes records only after both:
//  1. their idempotency retention period has elapsed; and
//  2. they can no longer contribute to that subscription's current rolling or
//     calendar quota windows.
//
// The second cutoff is subscription-specific. This avoids the previous global
// maximum-window retention, where one long yearly/custom plan kept every
// customer's request history indefinitely.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if DB == nil {
		return 0, errors.New("database is not initialized")
	}
	if olderThanSeconds <= 0 {
		olderThanSeconds = defaultSubscriptionPreConsumeRetentionSeconds
	}
	now := GetDBTimestamp()
	idempotencyCutoff, cutoffOK := subscriptionSafeSubInt64(now, olderThanSeconds)
	if !cutoffOK {
		// A caller-supplied retention larger than the representable timestamp
		// range must never wrap to a future cutoff (which would delete recent
		// idempotency records). Saturating at MinInt64 keeps the cleanup
		// conservative: only records older than the minimum representable time
		// could be removed.
		idempotencyCutoff = -int64(^uint64(0)>>1) - 1
	}

	var subs []UserSubscription
	if err := DB.Find(&subs).Error; err != nil {
		return 0, err
	}
	planIDs := make([]int, 0, len(subs))
	seen := make(map[int]struct{}, len(subs))
	for i := range subs {
		if subs[i].PlanId <= 0 {
			continue
		}
		if _, ok := seen[subs[i].PlanId]; ok {
			continue
		}
		seen[subs[i].PlanId] = struct{}{}
		planIDs = append(planIDs, subs[i].PlanId)
	}
	plansByID := make(map[int]*SubscriptionPlan, len(planIDs))
	if len(planIDs) > 0 {
		var plans []SubscriptionPlan
		if err := DB.Where("id IN ?", planIDs).Find(&plans).Error; err != nil {
			return 0, err
		}
		for i := range plans {
			plans[i].NormalizeQuotaWindows()
			plansByID[plans[i].Id] = &plans[i]
		}
	}

	var deleted int64
	for i := range subs {
		plan := plansByID[subs[i].PlanId]
		if plan == nil && subs[i].PlanPolicySnapshotVersion > 0 {
			// The source plan may have been deleted after issuance.  Resolve the
			// record-retention window from the subscription snapshot rather than
			// falling back to the ordinary idempotency cutoff.
			plan = subscriptionPlanSnapshotFallback(&subs[i])
		}
		safeBefore := subscriptionRecordSafeBefore(&subs[i], plan, now)
		res := DB.Where("user_subscription_id = ? AND updated_at < ? AND created_at < ?", subs[i].Id, idempotencyCutoff, safeBefore).
			Delete(&SubscriptionPreConsumeRecord{})
		if res.Error != nil {
			return deleted, res.Error
		}
		deleted += res.RowsAffected
	}

	// Deleted subscription rows have no window that can consume their records;
	// retain them only for the ordinary idempotency period.
	res := DB.Where("updated_at < ?", idempotencyCutoff).
		Where("NOT EXISTS (SELECT 1 FROM user_subscriptions WHERE user_subscriptions.id = subscription_pre_consume_records.user_subscription_id)").
		Delete(&SubscriptionPreConsumeRecord{})
	if res.Error != nil {
		return deleted, res.Error
	}
	return deleted + res.RowsAffected, nil
}

type SubscriptionPlanInfo struct {
	PlanId    int
	PlanTitle string
}

func GetSubscriptionPlanInfoByUserSubscriptionId(userSubscriptionId int) (*SubscriptionPlanInfo, error) {
	if userSubscriptionId <= 0 {
		return nil, errors.New("invalid userSubscriptionId")
	}
	cacheKey := fmt.Sprintf("sub:%d", userSubscriptionId)
	if cached, found, err := getSubscriptionPlanInfoCache().Get(cacheKey); err == nil && found {
		return &cached, nil
	}
	var sub UserSubscription
	if err := DB.Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
		return nil, err
	}
	plan, err := getSubscriptionPlanOrSnapshotTx(nil, &sub)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, errors.New("subscription plan is unavailable")
	}
	info := &SubscriptionPlanInfo{
		PlanId:    sub.PlanId,
		PlanTitle: plan.Title,
	}
	_ = getSubscriptionPlanInfoCache().SetWithTTL(cacheKey, *info, subscriptionPlanInfoCacheTTL())
	return info, nil
}

// Update subscription used amount by delta (positive consume more, negative refund).
func PostConsumeUserSubscriptionDelta(userSubscriptionId int, delta int64) error {
	if userSubscriptionId <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", userSubscriptionId).First(&sub).Error; err != nil {
			return err
		}
		var plan *SubscriptionPlan
		if loaded, err := getSubscriptionPlanOrSnapshotTx(tx, &sub); err == nil {
			plan = loaded
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		policy := quotaPolicyForSubscription(&sub, plan)
		if policy.invalid {
			return invalidSubscriptionQuotaPolicyError()
		}
		if policyHasGenericWindows(policy) && delta != 0 {
			return errors.New("request-aware subscription settlement required for quota windows")
		}
		if delta > 0 && policyHasFiveHour(policy) {
			return errors.New("request-aware subscription settlement required for five-hour window")
		}
		tracksWeekly := policy.mode == SubscriptionQuotaWindowLegacy || policyHasWeekly(policy)
		newUsed := sub.AmountUsed
		if tracksWeekly {
			var ok bool
			newUsed, ok = subscriptionSafeAddInt64(newUsed, delta)
			if !ok {
				return errors.New("subscription aggregate usage overflows int64")
			}
			if newUsed < 0 {
				newUsed = 0
			}
			limit := policyWeeklyLimit(policy, &sub)
			if limit > 0 && newUsed > limit {
				return fmt.Errorf("subscription used exceeds weekly limit, used=%d limit=%d", newUsed, limit)
			}
			sub.AmountUsed = newUsed
		}
		return tx.Save(&sub).Error
	})
}
