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
	return true
}

// ---- User APIs ----

func GetSubscriptionPlans(c *gin.Context) {
	// "错误：支付、兑换码、订阅计划和邀请返利功能已禁用。管理员需先确认合规声明后方可启用。"
	// if !operation_setting.IsPaymentComplianceConfirmed() {
	// 	common.ApiSuccess(c, []SubscriptionPlanDTO{})
	// 	return
	// }

	var plans []model.SubscriptionPlan
	if err := model.DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		result = append(result, SubscriptionPlanDTO{
			Plan: p,
		})
	}
	common.ApiSuccess(c, result)
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

// ---- Admin APIs ----

func AdminListSubscriptionPlans(c *gin.Context) {
	var plans []model.SubscriptionPlan
	if err := model.DB.Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		result = append(result, SubscriptionPlanDTO{
			Plan: p,
		})
	}
	common.ApiSuccess(c, result)
}

type AdminUpsertSubscriptionPlanRequest struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

type adminUpsertSubscriptionPlanPresence struct {
	Enabled       bool
	AllowPurchase bool
}

type adminUpsertSubscriptionPlanRawRequest struct {
	Plan map[string]json.RawMessage `json:"plan"`
}

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
	}
	return req, presence, nil
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
			return tx.Model(&model.SubscriptionPlan{}).Where("id = ?", req.Plan.Id).Updates(updateMap).Error
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
	common.ApiSuccess(c, req.Plan)
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
	req, _, err := bindAdminUpsertSubscriptionPlanRequest(c)
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

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// update plan (allow zero values updates with map)
		updateMap := map[string]interface{}{
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

// AdminGrantAirdropSubscription 管理员向指定用户空投全局配置的订阅套餐。
//
// POST /api/admin/subscription/airdrop
//
// 请求体：{"user_id": 123}
// 响应：{"user_id": 123, "granted": true, "plan_title": "体验套餐", "success": true}
//
// 与 AdminBindSubscription 的区别：
//   - AdminBindSubscription 可以指定任意 planId
//   - AdminGrantAirdropSubscription 使用全局运营配置中的 AirdropSubscriptionPlanId
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
