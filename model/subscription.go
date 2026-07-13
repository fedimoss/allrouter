package model

import (
	"errors"
	"fmt"
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
	SubscriptionResetCustom  = "custom"
)

var (
	ErrSubscriptionOrderNotFound      = errors.New("subscription order not found")
	ErrSubscriptionOrderStatusInvalid = errors.New("subscription order status invalid")
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
	ProviderId int `json:"provider_id" gorm:"type:int;not null;default:0;index"`

	Title    string `json:"title" gorm:"type:varchar(128);not null"`
	Subtitle string `json:"subtitle" gorm:"type:varchar(255);default:''"`

	// Display money amount (follow existing code style: float64 for money)
	PriceAmount float64 `json:"price_amount" gorm:"type:decimal(10,6);not null;default:0"`
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

	// Upgrade user group after purchase (empty = no change)
	UpgradeGroup string `json:"upgrade_group" gorm:"type:varchar(64);default:''"`

	// Total quota (amount in quota units, 0 = unlimited)
	TotalAmount int64 `json:"total_amount" gorm:"type:bigint;not null;default:0"`

	// Quota reset period for plan
	QuotaResetPeriod        string `json:"quota_reset_period" gorm:"type:varchar(16);default:'never'"`
	QuotaResetCustomSeconds int64  `json:"quota_reset_custom_seconds" gorm:"type:bigint;default:0"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (p *SubscriptionPlan) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	p.CreatedAt = now
	p.UpdatedAt = now
	return nil
}

func (p *SubscriptionPlan) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
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
	Status          string  `json:"status"`
	CreateTime      int64   `json:"create_time"`
	CompleteTime    int64   `json:"complete_time"`

	ProviderPayload string `json:"provider_payload" gorm:"type:text"`
}

func (o *SubscriptionOrder) Insert() error {
	if o.CreateTime == 0 {
		o.CreateTime = common.GetTimestamp()
	}
	return DB.Create(o).Error
}

// CreateSubscriptionOrderWithTopUp 创建订阅订单，并在同一事务中同步创建或更新对应的充值记录。
func CreateSubscriptionOrderWithTopUp(order *SubscriptionOrder) error {
	if order == nil {
		return errors.New("subscription order is nil")
	}
	if order.CreateTime == 0 {
		order.CreateTime = common.GetTimestamp()
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		return upsertSubscriptionTopUpTx(tx, order)
	})
}

func (o *SubscriptionOrder) Update() error {
	return DB.Save(o).Error
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
	var existing TopUp
	if err := tx.Where("trade_no = ?", tradeNo).First(&existing).Error; err == nil {
		// 已存在收入流水，视为本次"未新增入账"，返回已记录的 userId，created=false。
		return existing.UserId, 0, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, false, err
	}
	var provider Provider
	if err := tx.Select("id", "owner_user_id").Where("id = ?", order.ProviderId).First(&provider).Error; err != nil {
		return 0, 0, false, err
	}
	if provider.OwnerUserId <= 0 {
		return 0, 0, false, errors.New("provider owner user id is empty")
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
	switch plan.DurationUnit {
	case SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func NormalizeResetPeriod(period string) string {
	switch strings.TrimSpace(period) {
	case SubscriptionResetDaily, SubscriptionResetWeekly, SubscriptionResetMonthly, SubscriptionResetCustom:
		return strings.TrimSpace(period)
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
	case SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 {
			return 0
		}
		next = base.Add(time.Duration(plan.QuotaResetCustomSeconds) * time.Second)
	default:
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
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
	if key != "" {
		// 将套餐 ID 作为键从缓存中获取订阅套餐详情
		if cached, found, err := getSubscriptionPlanCache().Get(key); err == nil && found {
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
	_ = getSubscriptionPlanCache().SetWithTTL(key, plan, subscriptionPlanCacheTTL())
	return &plan, nil
}

func CountUserSubscriptionsByPlan(userId int, planId int) (int64, error) {
	if userId <= 0 || planId <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", userId, planId).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
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
	activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND id <> ? AND upgrade_group <> ''",
		sub.UserId, "active", now, sub.Id).
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
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	if plan.MaxPurchasePerUser > 0 {
		var count int64
		if err := tx.Model(&UserSubscription{}).
			Where("user_id = ? AND plan_id = ?", userId, plan.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("已达到该套餐购买上限")
		}
	}
	nowUnix := getDBTimestampTx(tx)
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	resetBase := now
	nextReset := calcNextResetTime(resetBase, plan, endUnix)
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}
	// 解析订阅实例归属服务商：优先用显式传入的 providerIds[0]，
	// 否则按用户表的 provider_id 自动继承，保证订阅与用户在同一服务商上下文。
	userProviderId, err := getUserProviderIdByIdTx(tx, userId)
	if err != nil {
		return nil, err
	}
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
		UserId:        userId,
		PlanId:        plan.Id,
		ProviderId:    providerId,
		AmountTotal:   plan.TotalAmount,
		AmountUsed:    0,
		StartTime:     now.Unix(),
		EndTime:       endUnix,
		Status:        "active",
		Source:        source,
		LastResetTime: lastReset,
		NextResetTime: nextReset,
		UpgradeGroup:  upgradeGroup,
		PrevUserGroup: prevGroup,
		CreatedAt:     common.GetTimestamp(),
		UpdatedAt:     common.GetTimestamp(),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

// CompleteSubscriptionOrder 完成一个订阅订单（幂等）。从套餐创建 UserSubscription 快照。
// Complete a subscription order (idempotent). Creates a UserSubscription snapshot from the plan.
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentMethod string, expectedPaymentProvider ...string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	var logUserId int
	var logPlanTitle string
	var logMoney float64
	var logPaymentMethod string
	var upgradeGroup string
	// 服务商订阅收入结算结果：在事务内由 applyProviderSubscriptionIncomeTx 填充，
	// 事务成功后再在事务外更新额度缓存并写日志。
	var providerIncomeOwnerId int
	var providerIncomeQuota int
	var providerIncomeCreated bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		// 订阅补单/回调处理时，必须确认当前回调网关和本地订单支付方式一致。
		if expectedPaymentMethod != "" && order.PaymentMethod != expectedPaymentMethod {
			return ErrPaymentMethodMismatch
		}
		if len(expectedPaymentProvider) > 0 && expectedPaymentProvider[0] != "" && order.PaymentProvider != expectedPaymentProvider[0] {
			return ErrPaymentMethodMismatch
		}
		if order.Status == common.TopUpStatusSuccess {
			// 订单此前已完成（重复回调/补单）：仍要保证幂等地补齐充值流水与服务商收入，避免漏发钱。
			if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
				return err
			}
			incomeOwnerId, incomeQuota, incomeCreated, err := applyProviderSubscriptionIncomeTx(tx, &order)
			if err != nil {
				return err
			}
			providerIncomeOwnerId = incomeOwnerId
			providerIncomeQuota = incomeQuota
			providerIncomeCreated = incomeCreated
			return nil
		}
		if order.Status != common.TopUpStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		plan, err := getSubscriptionPlanByIdTx(tx, order.PlanId)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			// still allow completion for already purchased orders
		}
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
		// 用订单上的 ProviderId 创建订阅实例，确保实例归属与订单一致（服务商私有套餐 → 服务商站点用户实例）。
		_, err = CreateUserSubscriptionFromPlanTx(tx, order.UserId, plan, "order", order.ProviderId)
		if err != nil {
			return err
		}
		paidQuota := int(decimal.NewFromFloat(order.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
		if paidQuota > 0 {
			if err := tx.Model(&User{}).Where("id = ?", order.UserId).
				Update("used_quota", gorm.Expr("used_quota + ?", paidQuota)).Error; err != nil {
				return err
			}
		}
		order.Status = common.TopUpStatusSuccess
		order.CompleteTime = common.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		if err := upsertSubscriptionTopUpTx(tx, &order); err != nil {
			return err
		}
		// 首次完成订单时同样结算服务商订阅收入，写入 owner 用户额度与一条 provider_subscription 流水。
		incomeOwnerId, incomeQuota, incomeCreated, err := applyProviderSubscriptionIncomeTx(tx, &order)
		if err != nil {
			return err
		}
		providerIncomeOwnerId = incomeOwnerId
		providerIncomeQuota = incomeQuota
		providerIncomeCreated = incomeCreated
		logUserId = order.UserId
		logPlanTitle = plan.Title
		logMoney = order.Money
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if upgradeGroup != "" && logUserId > 0 {
		_ = UpdateUserGroupCache(logUserId, upgradeGroup)
	}
	if logUserId > 0 {
		msg := fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", logPlanTitle, logMoney, logPaymentMethod)
		RecordLog(logUserId, LogTypeTopup, msg)
	}
	// 事务提交成功后：异步刷新服务商 owner 用户的额度缓存，并记录一条收入到账日志。
	// 放在事务外是为了避免缓存/日志失败回滚数据库事务（缓存最终一致性即可）。
	if providerIncomeCreated && providerIncomeOwnerId > 0 && providerIncomeQuota > 0 {
		asyncIncrUserQuotaCache(providerIncomeOwnerId, providerIncomeQuota)
		RecordLog(providerIncomeOwnerId, LogTypeTopup, fmt.Sprintf("provider subscription income credited %s, source user ID %d, trade no %s", logger.LogQuota(providerIncomeQuota), logUserId, tradeNo))
	}
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
	if err := tx.Where("trade_no = ?", order.TradeNo).First(&topup).Error; err != nil {
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
	if topup.BizType == "" {
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

func ExpireSubscriptionOrder(tradeNo string, expectedPaymentMethod string) error {
	if tradeNo == "" {
		return errors.New("tradeNo is empty")
	}
	refCol := "`trade_no`"
	if common.UsingPostgreSQL {
		refCol = `"trade_no"`
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var order SubscriptionOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(refCol+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrSubscriptionOrderNotFound
		}
		// 过期/关闭订单也要做同样的支付方式校验，避免不同支付通道互相影响订单状态。
		if expectedPaymentMethod != "" && order.PaymentMethod != expectedPaymentMethod {
			return ErrPaymentMethodMismatch
		}
		if order.Status != common.TopUpStatusPending {
			return nil
		}
		order.Status = common.TopUpStatusExpired
		order.CompleteTime = common.GetTimestamp()
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		return upsertSubscriptionTopUpTx(tx, &order)
	})
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
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return "", err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "admin")
		return err
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(plan.UpgradeGroup) != "" {
		_ = UpdateUserGroupCache(userId, plan.UpgradeGroup)
		return fmt.Sprintf("用户分组将升级到 %s", plan.UpgradeGroup), nil
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
			return nil
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
		if _, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "airdrop"); err != nil {
			return err
		}
		planTitle = plan.Title
		upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
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

// getSubscriptionRewardPlanIdTx 通过站点解析奖励配置。
// 提供商站点永远不会继承主站点的订阅奖励计划。
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

// GetAllActiveUserSubscriptions returns all active subscriptions for a user.
func GetAllActiveUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var subs []UserSubscription
	err := DB.Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// HasActiveUserSubscription returns whether the user has any active subscription.
// This is a lightweight existence check to avoid heavy pre-consume transactions.
func HasActiveUserSubscription(userId int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("invalid userId")
	}
	now := common.GetTimestamp()
	var count int64
	if err := DB.Model(&UserSubscription{}).
		Where("user_id = ? AND status = ? AND end_time > ?", userId, "active", now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetAllUserSubscriptions returns all subscriptions (active and expired) for a user.
func GetAllUserSubscriptions(userId int) ([]SubscriptionSummary, error) {
	if userId <= 0 {
		return nil, errors.New("invalid userId")
	}
	var subs []UserSubscription
	err := DB.Where("user_id = ?", userId).
		Order("end_time desc, id desc").
		Find(&subs).Error
	if err != nil {
		return nil, err
	}
	return buildSubscriptionSummaries(subs), nil
}

// buildSubscriptionSummaries 批量构建订阅摘要列表，附带套餐信息。
// 优化：先收集所有订阅中涉及的去重 planId，然后一次性批量查询套餐，
// 避免 N+1 查询问题。对于已删除或查询不到的套餐，Plan 字段为 nil。
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
				plan := plans[i]
				planMap[plan.Id] = &plan
			}
		}
	}
	// 第三步：组装结果，每个订阅带上对应的套餐信息
	result := make([]SubscriptionSummary, 0, len(subs))
	for _, sub := range subs {
		subCopy := sub
		result = append(result, SubscriptionSummary{
			Subscription: &subCopy,
			Plan:         planMap[sub.PlanId],
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
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
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
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
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
			activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group <> ''",
				userId, "active", now).
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
	UserSubscriptionId int    `json:"user_subscription_id" gorm:"index"`
	PreConsumed        int64  `json:"pre_consumed" gorm:"type:bigint;not null;default:0"`
	Status             string `json:"status" gorm:"type:varchar(32);index"` // consumed/refunded
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *SubscriptionPreConsumeRecord) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *SubscriptionPreConsumeRecord) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *UserSubscription, plan *SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	if sub.NextResetTime > 0 && sub.NextResetTime > now {
		return nil
	}
	if NormalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever {
		return nil
	}
	baseUnix := sub.LastResetTime
	if baseUnix <= 0 {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextResetTime(base, plan, sub.EndTime)
	advanced := false
	for next > 0 && next <= now {
		advanced = true
		base = time.Unix(next, 0)
		next = calcNextResetTime(base, plan, sub.EndTime)
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
	now := GetDBTimestamp()

	returnValue := &SubscriptionPreConsumeResult{}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing SubscriptionPreConsumeRecord
		query := tx.Where("request_id = ?", requestId).Limit(1).Find(&existing)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if existing.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			var sub UserSubscription
			if err := tx.Where("id = ?", existing.UserSubscriptionId).First(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = existing.PreConsumed
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = sub.AmountUsed
			returnValue.AmountUsedAfter = sub.AmountUsed
			return nil
		}

		userProviderId, err := getUserProviderIdByIdTx(tx, userId)
		if err != nil {
			return err
		}
		var subs []UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND provider_id = ? AND status = ? AND end_time > ?", userId, userProviderId, "active", now).
			Order("end_time asc, id asc").
			Find(&subs).Error; err != nil {
			return errors.New("no active subscription")
		}
		if len(subs) == 0 {
			return errors.New("no active subscription")
		}
		for _, candidate := range subs {
			sub := candidate
			plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
			if err != nil {
				return err
			}
			if plan.ProviderId != sub.ProviderId {
				continue
			}
			if !plan.AllowsModel(modelName) {
				continue
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
				return err
			}
			usedBefore := sub.AmountUsed
			if sub.AmountTotal > 0 {
				remain := sub.AmountTotal - usedBefore
				if remain < amount {
					continue
				}
			}
			record := &SubscriptionPreConsumeRecord{
				RequestId:          requestId,
				UserId:             userId,
				UserSubscriptionId: sub.Id,
				PreConsumed:        amount,
				Status:             "consumed",
			}
			if err := tx.Create(record).Error; err != nil {
				var dup SubscriptionPreConsumeRecord
				if err2 := tx.Where("request_id = ?", requestId).First(&dup).Error; err2 == nil {
					if dup.Status == "refunded" {
						return errors.New("subscription pre-consume already refunded")
					}
					returnValue.UserSubscriptionId = sub.Id
					returnValue.PreConsumed = dup.PreConsumed
					returnValue.AmountTotal = sub.AmountTotal
					returnValue.AmountUsedBefore = sub.AmountUsed
					returnValue.AmountUsedAfter = sub.AmountUsed
					return nil
				}
				return err
			}
			sub.AmountUsed += amount
			if err := tx.Save(&sub).Error; err != nil {
				return err
			}
			returnValue.UserSubscriptionId = sub.Id
			returnValue.PreConsumed = amount
			returnValue.AmountTotal = sub.AmountTotal
			returnValue.AmountUsedBefore = usedBefore
			returnValue.AmountUsedAfter = sub.AmountUsed
			return nil
		}
		return fmt.Errorf("subscription quota insufficient, need=%d", amount)
	})
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

// RefundSubscriptionPreConsume is idempotent and refunds pre-consumed subscription quota by requestId.
func RefundSubscriptionPreConsume(requestId string) error {
	if strings.TrimSpace(requestId) == "" {
		return errors.New("requestId is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var record SubscriptionPreConsumeRecord
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("request_id = ?", requestId).First(&record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return nil
		}
		if record.PreConsumed <= 0 {
			record.Status = "refunded"
			return tx.Save(&record).Error
		}
		if err := PostConsumeUserSubscriptionDelta(record.UserSubscriptionId, -record.PreConsumed); err != nil {
			return err
		}
		record.Status = "refunded"
		return tx.Save(&record).Error
	})
}

// ResetDueSubscriptions resets subscriptions whose next_reset_time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	now := GetDBTimestamp()
	var subs []UserSubscription
	if err := DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", now, "active").
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
		subCopy := sub
		plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
		if err != nil || plan == nil {
			continue
		}
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked UserSubscription
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND next_reset_time > 0 AND next_reset_time <= ?", subCopy.Id, now).
				First(&locked).Error; err != nil {
				return nil
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &locked, plan, now); err != nil {
				return err
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// CleanupSubscriptionPreConsumeRecords removes old idempotency records to keep table small.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ?", cutoff).Delete(&SubscriptionPreConsumeRecord{})
	return res.RowsAffected, res.Error
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
	plan, err := getSubscriptionPlanByIdTx(nil, sub.PlanId)
	if err != nil {
		return nil, err
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
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("id = ?", userSubscriptionId).
			First(&sub).Error; err != nil {
			return err
		}
		newUsed := sub.AmountUsed + delta
		if newUsed < 0 {
			newUsed = 0
		}
		if sub.AmountTotal > 0 && newUsed > sub.AmountTotal {
			return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newUsed, sub.AmountTotal)
		}
		sub.AmountUsed = newUsed
		return tx.Save(&sub).Error
	})
}
