package controller

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	// "错误：支付、兑换码、订阅计划和邀请返利功能已禁用。管理员需先确认合规声明后方可启用。"
	// "github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---- Shared types ----

type SubscriptionPlanDTO struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

type BillingPreferenceRequest struct {
	BillingPreference string `json:"billing_preference"`
}

// ensureSubscriptionPlanPurchasable 校验套餐是否允许用户自助购买。
// 检查三项：套餐是否存在、是否启用、是否允许购买（AllowPurchase）。
//
// 注意：此函数仅用于用户端自助购买流程的入口校验（支付控制器中调用）。
// 管理员操作（AdminBindSubscription、AdminGrantAirdropSubscription）和
// 系统自动授予（注册赠送、空投）不受 AllowPurchase 限制。
// 返回 false 时已通过 ApiErrorMsg 向前端返回错误信息，调用方直接 return 即可。
func ensureSubscriptionPlanPurchasable(c *gin.Context, plan *model.SubscriptionPlan) bool {
	if plan == nil {
		common.ApiErrorMsg(c, "套餐不存在")
		return false
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return false
	}
	if plan.AllowPurchase != 1 {
		common.ApiErrorMsg(c, "该套餐暂不允许订阅")
		return false
	}
	// 服务商归属校验：套餐必须归属当前请求上下文中的 provider_id（主站为 0）。
	// 这样可以避免主站用户订阅服务商私有套餐、或服务商 A 的用户订阅服务商 B 的套餐，
	// 实现套餐按服务商隔离的"站点级"可见性。
	if !plan.VisibleInProvider(c.GetInt("provider_id")) {
		common.ApiErrorMsg(c, "该套餐不适用于当前站点")
		return false
	}
	return true
}

// buildSubscriptionPlanDTO 将单个 SubscriptionPlan 包装成统一的 DTO 结构 { plan: {...} }。
// 统一出口结构便于前端按 record.plan.* 的方式读取字段，复用 Admin / Provider 两套接口。
func buildSubscriptionPlanDTO(plan model.SubscriptionPlan) SubscriptionPlanDTO {
	return SubscriptionPlanDTO{Plan: plan}
}

// buildSubscriptionPlanDTOs 批量将 SubscriptionPlan 列表包装成 DTO 列表，预分配容量避免扩容。
func buildSubscriptionPlanDTOs(plans []model.SubscriptionPlan) []SubscriptionPlanDTO {
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		result = append(result, buildSubscriptionPlanDTO(p))
	}
	return result
}

// ---- User APIs ----

func GetSubscriptionPlans(c *gin.Context) {
	// "错误：支付、兑换码、订阅计划和邀请返利功能已禁用。管理员需先确认合规声明后方可启用。"
	// if !operation_setting.IsPaymentComplianceConfirmed() {
	// 	common.ApiSuccess(c, []SubscriptionPlanDTO{})
	// 	return
	// }

	// 仅返回"对当前请求上下文 provider_id 可见且已启用"的套餐：
	// 主站(provider_id=0)拿到主站套餐，服务商站点只拿到该服务商私有的套餐。
	// 详见 model.ListVisibleSubscriptionPlans。
	plans, err := model.ListVisibleSubscriptionPlans(c.GetInt("provider_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildSubscriptionPlanDTOs(plans))
}

func GetSubscriptionSelf(c *gin.Context) {
	userId := c.GetInt("id")
	settingMap, _ := model.GetUserSetting(userId, false)
	pref := common.NormalizeBillingPreference(settingMap.BillingPreference)

	// Get all subscriptions (including expired)
	allSubscriptions, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		allSubscriptions = []model.SubscriptionSummary{}
	}

	// Get active subscriptions for backward compatibility
	activeSubscriptions, err := model.GetAllActiveUserSubscriptions(userId)
	if err != nil {
		activeSubscriptions = []model.SubscriptionSummary{}
	}

	common.ApiSuccess(c, gin.H{
		"billing_preference": pref,
		"subscriptions":      activeSubscriptions, // all active subscriptions
		"all_subscriptions":  allSubscriptions,    // all subscriptions including expired
	})
}

func UpdateSubscriptionPreference(c *gin.Context) {
	userId := c.GetInt("id")
	var req BillingPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	pref := common.NormalizeBillingPreference(req.BillingPreference)

	user, err := model.GetUserById(userId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	current := user.GetSetting()
	current.BillingPreference = pref
	user.SetSetting(current)
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"billing_preference": pref})
}

// ---- Provider owner APIs ----
//
// 以下接口面向"服务商所有者"（provider owner）：服务商在自己的控制台里管理其私有的订阅套餐。
// 与 Admin 接口的区别：
//   - 通过 getOwnedProvider(c) 解析当前登录用户所拥有/绑定的服务商，只能操作 provider_id == provider.Id 的套餐；
//   - 套餐被强制归属当前服务商，不允许跨服务商修改；
//   - 套餐所配置的模型白名单(model_limits)必须全部来自该服务商在"模型广场"上上架的模型。
//
// 这些接口路由注册在 router/api-router.go 的 providerRoute 下，路径前缀 /api/provider/subscription。

// ProviderListSubscriptionPlans 列出当前服务商所有者名下的全部套餐（含未启用）。
func ProviderListSubscriptionPlans(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	var plans []model.SubscriptionPlan
	if err := model.DB.Where("provider_id = ?", provider.Id).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildSubscriptionPlanDTOs(plans))
}

// ProviderListSubscriptionPlanModels 返回当前服务商可加入套餐模型白名单的候选模型列表。
// 数据源是该服务商在"模型广场"(provider_model_pricing)中已启用的 public_model_name，去重并按字母序返回。
// 前端用于在新增/编辑套餐弹窗中提供模型多选下拉。
func ProviderListSubscriptionPlanModels(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	models, err := model.ListProviderSubscriptionPlanModels(provider.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, models)
}

// ProviderCreateSubscriptionPlan 服务商所有者创建一个归属于自己的私有订阅套餐。
// 与 AdminCreateSubscriptionPlan 的差异：provider_id 被强制为当前服务商，
// 并额外校验套餐模型白名单必须来自该服务商模型广场。
func ProviderCreateSubscriptionPlan(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	req, presence, err := bindAdminUpsertSubscriptionPlanRequest(c)
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	// 强制新建：Id 清零由 GORM 重新分配，ProviderId 绑定当前服务商，避免越权创建到他人名下。
	req.Plan.Id = 0
	req.Plan.ProviderId = provider.Id
	if !normalizeSubscriptionPlanFields(c, &req.Plan) {
		return
	}
	if !validateSubscriptionPlanModelLimitsForProvider(c, provider.Id, req.Plan.ModelLimits) {
		return
	}
	// 由于 Create 不会写入布尔字段的"显式零值"，与 Admin 逻辑保持一致：
	// 记录请求中是否显式传了 allow_purchase/enabled，事务内再用 Updates 写回正确值。
	explicitAllowPurchase := req.Plan.AllowPurchase
	explicitEnabled := req.Plan.Enabled
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&req.Plan).Error; err != nil {
			return err
		}
		// 二次更新保证 allow_purchase/enabled 的"显式 false"也能落库（Create 会把 0/false 当零值跳过）。
		updateMap := map[string]interface{}{
			"provider_id":                provider.Id,
			"quota_reset_period":         req.Plan.QuotaResetPeriod,
			"quota_reset_custom_seconds": req.Plan.QuotaResetCustomSeconds,
		}
		if presence.AllowPurchase {
			updateMap["allow_purchase"] = explicitAllowPurchase
		}
		if presence.Enabled {
			updateMap["enabled"] = explicitEnabled
		}
		return tx.Model(&model.SubscriptionPlan{}).Where("id = ?", req.Plan.Id).Updates(updateMap).Error
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 用请求中显式传入的值回填返回体，保证返回给前端的布尔字段与落库一致。
	if presence.AllowPurchase {
		req.Plan.AllowPurchase = explicitAllowPurchase
	}
	if presence.Enabled {
		req.Plan.Enabled = explicitEnabled
	}
	// 新建套餐后失效对应缓存，避免读到旧的聚合缓存。
	model.InvalidateSubscriptionPlanCache(req.Plan.Id)
	common.ApiSuccess(c, buildSubscriptionPlanDTO(req.Plan))
}

// ProviderUpdateSubscriptionPlan 服务商所有者更新自己名下的套餐全量字段。
// 通过 id + provider_id 双重条件查询，保证服务商只能改自己的套餐，越权改他人套餐会因查不到记录而报错。
func ProviderUpdateSubscriptionPlan(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	req, _, err := bindAdminUpsertSubscriptionPlanRequest(c)
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	// 先确认该套餐确实归属当前服务商，否则直接返回错误，避免越权。
	var existing model.SubscriptionPlan
	if err := model.DB.Select("id", "provider_id").Where("id = ? AND provider_id = ?", id, provider.Id).First(&existing).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	req.Plan.Id = id
	req.Plan.ProviderId = provider.Id
	if !normalizeSubscriptionPlanFields(c, &req.Plan) {
		return
	}
	if !validateSubscriptionPlanModelLimitsForProvider(c, provider.Id, req.Plan.ModelLimits) {
		return
	}
	// 使用 map 形式 Updates 以支持零值字段写入；并强制 provider_id 始终指向当前服务商防止篡改归属。
	updateMap := map[string]interface{}{
		"provider_id":                provider.Id,
		"title":                      req.Plan.Title,
		"subtitle":                   req.Plan.Subtitle,
		"price_amount":               req.Plan.PriceAmount,
		"currency":                   req.Plan.Currency,
		"duration_unit":              req.Plan.DurationUnit,
		"duration_value":             req.Plan.DurationValue,
		"custom_seconds":             req.Plan.CustomSeconds,
		"enabled":                    req.Plan.Enabled,
		"sort_order":                 req.Plan.SortOrder,
		"allow_purchase":             req.Plan.AllowPurchase,
		"model_limits":               req.Plan.ModelLimits,
		"stripe_price_id":            req.Plan.StripePriceId,
		"stripe_price_cny_id":        req.Plan.StripePriceCnyId,
		"creem_product_id":           req.Plan.CreemProductId,
		"waffo_pancake_product_id":   req.Plan.WaffoPancakeProductId,
		"max_purchase_per_user":      req.Plan.MaxPurchasePerUser,
		"total_amount":               req.Plan.TotalAmount,
		"upgrade_group":              req.Plan.UpgradeGroup,
		"quota_reset_period":         req.Plan.QuotaResetPeriod,
		"quota_reset_custom_seconds": req.Plan.QuotaResetCustomSeconds,
		"updated_at":                 common.GetTimestamp(),
	}
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ? AND provider_id = ?", id, provider.Id).Updates(updateMap).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

// ProviderUpdateSubscriptionPlanStatus 服务商所有者单独切换套餐启用状态（enabled）。
// 用 RowsAffected 判断：若受影响行数为 0，说明该 id 不属于当前服务商，返回"套餐不存在"以隐藏存在性。
func ProviderUpdateSubscriptionPlanStatus(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpdateSubscriptionPlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	res := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ? AND provider_id = ?", id, provider.Id).Update("enabled", *req.Enabled)
	if res.Error != nil {
		common.ApiError(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		common.ApiErrorMsg(c, "套餐不存在")
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

// ---- Admin APIs ----

func AdminListSubscriptionPlans(c *gin.Context) {
	var plans []model.SubscriptionPlan
	if err := model.DB.Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildSubscriptionPlanDTOs(plans))
}

type AdminUpsertSubscriptionPlanRequest struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

type adminUpsertSubscriptionPlanPresence struct {
	Enabled       bool
	AllowPurchase bool
	// ProviderId 标记请求体中是否显式传入了 provider_id 字段。
	// AdminUpdateSubscriptionPlan 用它判断：未显式传入时需保留原归属，避免误把套餐改回主站。
	ProviderId bool
}

type adminUpsertSubscriptionPlanRawRequest struct {
	Plan map[string]json.RawMessage `json:"plan"`
}

// bindAdminUpsertSubscriptionPlanRequest 绑定套餐增改请求，同时记录关键字段是否"显式存在"。
// 因为 JSON 解析后无法区分"字段未传"与"字段传了零值"，所以这里把原始 body 再解析成
// map[string]json.RawMessage，通过 key 是否存在来判断 presence。返回的 presence 用于：
//   - AllowPurchase/Enabled：决定是否在二次 Updates 中强制写回显式零值（见 Provider/Admin Create）；
//   - ProviderId：决定 Admin 更新时是否保留原归属（见 AdminUpdateSubscriptionPlan）。
func bindAdminUpsertSubscriptionPlanRequest(c *gin.Context) (AdminUpsertSubscriptionPlanRequest, adminUpsertSubscriptionPlanPresence, error) {
	var req AdminUpsertSubscriptionPlanRequest
	var presence adminUpsertSubscriptionPlanPresence

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return req, presence, err
	}
	if err := common.Unmarshal(body, &req); err != nil {
		return req, presence, err
	}

	var rawReq adminUpsertSubscriptionPlanRawRequest
	if err := common.Unmarshal(body, &rawReq); err != nil {
		return req, presence, err
	}
	if rawReq.Plan != nil {
		_, presence.Enabled = rawReq.Plan["enabled"]
		_, presence.AllowPurchase = rawReq.Plan["allow_purchase"]
		_, presence.ProviderId = rawReq.Plan["provider_id"]
	}
	return req, presence, nil
}

// validateAdminSubscriptionPlanProvider 校验套餐归属服务商 ID 是否合法，管理员侧使用：
//   - ProviderId < 0：非法，直接拒绝；
//   - ProviderId == 0：主站套餐，无需进一步校验，返回 true；
//   - ProviderId > 0：必须为已存在的服务商，且其 model_limits 中的模型必须全部来自该服务商模型广场。
//
// 返回 false 时已写入错误响应，调用方直接 return。
func validateAdminSubscriptionPlanProvider(c *gin.Context, plan *model.SubscriptionPlan) bool {
	if plan == nil {
		common.ApiErrorMsg(c, "参数错误")
		return false
	}
	if plan.ProviderId < 0 {
		common.ApiErrorMsg(c, "服务商ID不能为负数")
		return false
	}
	if plan.ProviderId == 0 {
		return true
	}
	if plan.ProviderId > 0 {
		var count int64
		if err := model.DB.Model(&model.Provider{}).Where("id = ?", plan.ProviderId).Count(&count).Error; err != nil {
			common.ApiError(c, err)
			return false
		}
		if count == 0 {
			common.ApiErrorMsg(c, "服务商不存在")
			return false
		}
		// 服务商存在，再校验套餐模型白名单是否全部属于该服务商的模型广场。
		return validateSubscriptionPlanModelLimitsForProvider(c, plan.ProviderId, plan.ModelLimits)
	}
	return false
}

// validateSubscriptionPlanModelLimitsForProvider 校验套餐模型白名单是否全部来自指定服务商的模型广场。
// 不通过时返回 false 并把缺失的模型名拼到错误信息里，方便前端定位。
func validateSubscriptionPlanModelLimitsForProvider(c *gin.Context, providerId int, modelLimits string) bool {
	ok, missing, err := model.SubscriptionPlanModelsAllowedForProvider(providerId, modelLimits)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if !ok {
		common.ApiErrorMsg(c, "套餐模型必须来自服务商模型广场: "+strings.Join(missing, ", "))
		return false
	}
	return true
}

// normalizeSubscriptionPlanFields 规范化并校验套餐的可输入字段，Admin 与 Provider 两条路径共用。
// 包含：标题非空、价格区间(0,9999]、币种强制 USD、模型白名单去重排序、时长单位默认、
// 购买上限/总额度非负、升级分组存在性、重置周期合法性。返回 false 时已写入错误响应。
func normalizeSubscriptionPlanFields(c *gin.Context, plan *model.SubscriptionPlan) bool {
	if plan == nil {
		common.ApiErrorMsg(c, "参数错误")
		return false
	}
	if strings.TrimSpace(plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return false
	}
	if plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return false
	}
	if plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return false
	}
	plan.Currency = "USD"
	plan.ModelLimits = model.NormalizeSubscriptionPlanModelLimits(plan.ModelLimits)
	if plan.DurationUnit == "" {
		plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != model.SubscriptionDurationCustom {
		plan.DurationValue = 1
	}
	if plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return false
	}
	if plan.TotalAmount < 0 {
		common.ApiErrorMsg(c, "总额度不能为负数")
		return false
	}
	plan.UpgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
	if plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return false
		}
	}
	plan.QuotaResetPeriod = model.NormalizeResetPeriod(plan.QuotaResetPeriod)
	if plan.QuotaResetPeriod == model.SubscriptionResetCustom && plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return false
	}
	return true
}

func AdminCreateSubscriptionPlan(c *gin.Context) {
	// 创建套餐订阅计划
	// "错误：支付、兑换码、订阅计划和邀请返利功能已禁用。管理员需先确认合规声明后方可启用。"
	// if !requirePaymentCompliance(c) {
	// 	return
	// }

	req, presence, err := bindAdminUpsertSubscriptionPlanRequest(c)
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Plan.Id = 0
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return
	}
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	req.Plan.Currency = "USD"
	// 规范化模型限制：去重、去空格、排序后再存储，保证数据库格式一致
	req.Plan.ModelLimits = model.NormalizeSubscriptionPlanModelLimits(req.Plan.ModelLimits)
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return
	}
	if req.Plan.TotalAmount < 0 {
		common.ApiErrorMsg(c, "总额度不能为负数")
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return
	}
	// 新增校验：管理员创建套餐时也必须校验归属服务商合法性及模型白名单来源。
	if !validateAdminSubscriptionPlanProvider(c, &req.Plan) {
		return
	}
	explicitAllowPurchase := req.Plan.AllowPurchase
	explicitEnabled := req.Plan.Enabled
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&req.Plan).Error; err != nil {
			return err
		}

		// GORM applies struct default tags on create for zero values. Preserve
		// explicit admin choices such as allow_purchase=0 and enabled=false.
		updateMap := map[string]interface{}{}
		if presence.AllowPurchase {
			updateMap["allow_purchase"] = explicitAllowPurchase
		}
		if presence.Enabled {
			updateMap["enabled"] = explicitEnabled
		}
		if len(updateMap) > 0 {
			if err := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", req.Plan.Id).Updates(updateMap).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if presence.AllowPurchase {
		req.Plan.AllowPurchase = explicitAllowPurchase
	}
	if presence.Enabled {
		req.Plan.Enabled = explicitEnabled
	}
	model.InvalidateSubscriptionPlanCache(req.Plan.Id)
	common.ApiSuccess(c, buildSubscriptionPlanDTO(req.Plan))
}

func AdminUpdateSubscriptionPlan(c *gin.Context) {
	// 更新套餐订阅计划
	// "错误：支付、兑换码、订阅计划和邀请返利功能已禁用。管理员需先确认合规声明后方可启用。"
	// if !requirePaymentCompliance(c) {
	// 	return
	// }

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	req, presence, err := bindAdminUpsertSubscriptionPlanRequest(c)
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorMsg(c, "套餐标题不能为空")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorMsg(c, "价格不能为负数")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorMsg(c, "价格不能超过9999")
		return
	}
	req.Plan.Id = id
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	req.Plan.Currency = "USD"
	// 规范化模型限制：去重、去空格、排序后再存储，保证数据库格式一致
	req.Plan.ModelLimits = model.NormalizeSubscriptionPlanModelLimits(req.Plan.ModelLimits)
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorMsg(c, "购买上限不能为负数")
		return
	}
	if req.Plan.TotalAmount < 0 {
		common.ApiErrorMsg(c, "总额度不能为负数")
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorMsg(c, "升级分组不存在")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorMsg(c, "自定义重置周期需大于0秒")
		return
	}
	// 当请求未显式传入 provider_id 时，沿用该套餐原有的归属服务商，避免管理员漏传字段
	// 而把已有服务商私有套餐误改成主站套餐(provider_id=0)。
	if !presence.ProviderId {
		var existing model.SubscriptionPlan
		if err := model.DB.Select("provider_id").Where("id = ?", id).First(&existing).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		req.Plan.ProviderId = existing.ProviderId
	}

	// 统一校验套餐归属服务商合法性（含模型白名单必须来自该服务商模型广场）。
	if !validateAdminSubscriptionPlanProvider(c, &req.Plan) {
		return
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// update plan (allow zero values updates with map)
		// 用 map 形式 Updates 以支持零值字段写入，并把 provider_id 一并写入，保证归属可被管理员显式调整。
		updateMap := map[string]interface{}{
			"provider_id":                req.Plan.ProviderId,
			"title":                      req.Plan.Title,
			"subtitle":                   req.Plan.Subtitle,
			"price_amount":               req.Plan.PriceAmount,
			"currency":                   req.Plan.Currency,
			"duration_unit":              req.Plan.DurationUnit,
			"duration_value":             req.Plan.DurationValue,
			"custom_seconds":             req.Plan.CustomSeconds,
			"enabled":                    req.Plan.Enabled,
			"sort_order":                 req.Plan.SortOrder,
			"allow_purchase":             req.Plan.AllowPurchase,
			"model_limits":               req.Plan.ModelLimits,
			"stripe_price_id":            req.Plan.StripePriceId,    // Stripe 美元价格 ID
			"stripe_price_cny_id":        req.Plan.StripePriceCnyId, // Stripe 人民币价格 ID
			"creem_product_id":           req.Plan.CreemProductId,
			"waffo_pancake_product_id":   req.Plan.WaffoPancakeProductId,
			"max_purchase_per_user":      req.Plan.MaxPurchasePerUser,
			"total_amount":               req.Plan.TotalAmount,
			"upgrade_group":              req.Plan.UpgradeGroup,
			"quota_reset_period":         req.Plan.QuotaResetPeriod,
			"quota_reset_custom_seconds": req.Plan.QuotaResetCustomSeconds,
			"updated_at":                 common.GetTimestamp(),
		}
		if err := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Updates(updateMap).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

type AdminUpdateSubscriptionPlanStatusRequest struct {
	Enabled *bool `json:"enabled"`
}

func AdminUpdateSubscriptionPlanStatus(c *gin.Context) {
	// "错误：支付、兑换码、订阅计划和邀请返利功能已禁用。管理员需先确认合规声明后方可启用。"
	// if !requirePaymentCompliance(c) {
	// 	return
	// }

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	var req AdminUpdateSubscriptionPlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Update("enabled", *req.Enabled).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

type AdminBindSubscriptionRequest struct {
	UserId int `json:"user_id"`
	PlanId int `json:"plan_id"`
}

// AdminGrantAirdropSubscriptionRequest 管理员空投订阅请求体。
// 仅需指定目标用户 ID，套餐由全局配置 AirdropSubscriptionPlanId 决定。
type AdminGrantAirdropSubscriptionRequest struct {
	UserId int `json:"user_id"`
}

// AdminBindSubscription 管理员手动为用户绑定订阅套餐（无需支付，可指定任意套餐）。
// POST /api/admin/subscription/bind
// 请求体：{"user_id": 123, "plan_id": 5}
func AdminBindSubscription(c *gin.Context) {
	// "错误：支付、兑换码、订阅计划和邀请返利功能已禁用。管理员需先确认合规声明后方可启用。"
	// if !requirePaymentCompliance(c) {
	// 	return
	// }

	var req AdminBindSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	msg, err := model.AdminBindSubscription(req.UserId, req.PlanId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminGrantAirdropSubscription 管理员向指定用户空投其所属站点配置的订阅套餐。
//
// POST /api/admin/subscription/airdrop
//
// 请求体：{"user_id": 123}
// 响应：{"user_id": 123, "granted": true, "plan_title": "体验套餐", "success": true}
//
// 与 AdminBindSubscription 的区别：
//   - AdminBindSubscription 可以指定任意 planId
//   - AdminGrantAirdropSubscription 按目标用户所属站点解析空投套餐
func AdminGrantAirdropSubscription(c *gin.Context) {
	var req AdminGrantAirdropSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	title, err := model.GrantAirdropSubscription(req.UserId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"user_id":    req.UserId,
		"granted":    title != "",
		"plan_title": title,
	})
}

// ProviderGrantAirdropSubscription 允许服务提供商所有者将其已配置的空投计划授予其自有用户之一。
// POST /api/provider/subscription/airdrop
func ProviderGrantAirdropSubscription(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	var req AdminGrantAirdropSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var target model.User
	if err := model.DB.Select("id", "provider_id").Where("id = ?", req.UserId).Take(&target).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if target.ProviderId != provider.Id {
		common.ApiErrorMsg(c, "目标用户不属于当前服务商")
		return
	}
	title, err := model.GrantAirdropSubscription(target.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"user_id":    target.Id,
		"granted":    title != "",
		"plan_title": title,
	})
}

// ---- Admin: user subscription management ----

func AdminListUserSubscriptions(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	subs, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, subs)
}

type AdminCreateUserSubscriptionRequest struct {
	PlanId int `json:"plan_id"`
}

// AdminCreateUserSubscription creates a new user subscription from a plan (no payment).
func AdminCreateUserSubscription(c *gin.Context) {
	// "错误：支付、兑换码、订阅计划和邀请返利功能已禁用。管理员需先确认合规声明后方可启用。"
	// if !requirePaymentCompliance(c) {
	// 	return
	// }

	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	var req AdminCreateUserSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	msg, err := model.AdminBindSubscription(userId, req.PlanId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminInvalidateUserSubscription cancels a user subscription immediately.
func AdminInvalidateUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	msg, err := model.AdminInvalidateUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorMsg(c, "无效的订阅ID")
		return
	}
	msg, err := model.AdminDeleteUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if msg != "" {
		common.ApiSuccess(c, gin.H{"message": msg})
		return
	}
	common.ApiSuccess(c, nil)
}
