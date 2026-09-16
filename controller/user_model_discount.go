package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// resolveManagedUserId 解析路由中的用户 id 并校验当前管理员是否有权管理该用户。
// 用户专属折扣跟随用户管理权限：与用户管理页其它操作使用同一套角色/委托规则。
func resolveManagedUserId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return 0, false
	}
	target, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return 0, false
	}
	if !canManageTargetRole(c, target.Role) || (isDelegatedAdmin(c) && !delegatedCanManageTarget(target)) {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return 0, false
	}
	return id, true
}

func resolveProviderManagedUserId(c *gin.Context) (int, bool) {
	provider, _, ok := getProviderUserAdminProvider(c)
	if !ok {
		return 0, false
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return 0, false
	}
	target, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return 0, false
	}
	if target.ProviderId != provider.Id || target.Role != common.RoleCommonUser {
		common.ApiErrorI18n(c, i18n.MsgUserNoPermissionSameLevel)
		return 0, false
	}
	return id, true
}

type userModelDiscountRequest struct {
	// ModelName 支持一次提交一个模型；前端逐条维护
	ModelName string `json:"model_name"`
	// Discount 折扣，范围 (0, 1]
	Discount float64 `json:"discount"`
}

type userModelDiscountListResponse struct {
	Discounts []model.UserModelDiscount `json:"discounts"`
	Models    []string                  `json:"models"`
}

func getUserModelDiscountCandidates(userId int) ([]string, error) {
	target, err := model.GetUserById(userId, false)
	if err != nil {
		return nil, err
	}

	visiblePricing := getMarketplaceVisiblePricingForUserGroup(target.Group)
	if target.ProviderId == 0 {
		models := make([]string, 0, len(visiblePricing))
		for _, item := range visiblePricing {
			modelName := strings.TrimSpace(item.ModelName)
			if modelName != "" {
				models = append(models, modelName)
			}
		}
		sort.Strings(models)
		return models, nil
	}

	visibleBaseModels := make(map[string]struct{}, len(visiblePricing))
	for _, item := range visiblePricing {
		visibleBaseModels[item.ModelName] = struct{}{}
	}
	var rules []model.ProviderModelPricing
	if err := model.DB.
		Select("public_model_name", "base_model_name").
		Where("provider_id = ? AND enabled = ?", target.ProviderId, true).
		Find(&rules).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(rules))
	models := make([]string, 0, len(rules))
	for _, rule := range rules {
		if _, ok := visibleBaseModels[rule.BaseModelName]; !ok {
			continue
		}
		modelName := strings.TrimSpace(rule.PublicModelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		models = append(models, modelName)
	}
	sort.Strings(models)
	return models, nil
}

func isUserModelDiscountModelAllowed(userId int, modelName string) (bool, error) {
	models, err := getUserModelDiscountCandidates(userId)
	if err != nil {
		return false, err
	}
	for _, candidate := range models {
		if candidate == modelName {
			return true, nil
		}
	}

	return false, nil
}

func getUserModelDiscountListResponse(userId int) (*userModelDiscountListResponse, error) {
	records, err := model.GetUserModelDiscountsByUser(userId)
	if err != nil {
		return nil, err
	}
	models, err := getUserModelDiscountCandidates(userId)
	if err != nil {
		return nil, err
	}
	return &userModelDiscountListResponse{Discounts: records, Models: models}, nil
}

// AdminGetUserModelDiscounts 获取某用户全部模型专属折扣
func AdminGetUserModelDiscounts(c *gin.Context) {
	userId, ok := resolveManagedUserId(c)
	if !ok {
		return
	}
	data, err := getUserModelDiscountListResponse(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}

// AdminUpsertUserModelDiscount 设置/更新某用户某模型的专属折扣
func AdminUpsertUserModelDiscount(c *gin.Context) {
	userId, ok := resolveManagedUserId(c)
	if !ok {
		return
	}
	var req userModelDiscountRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	req.ModelName = strings.TrimSpace(req.ModelName)
	if req.ModelName == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if req.Discount <= 0 || req.Discount > 1 {
		common.ApiErrorMsg(c, "折扣范围必须在 (0, 1] 之间（1 表示不打折）")
		return
	}
	allowed, err := isUserModelDiscountModelAllowed(userId, req.ModelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !allowed {
		common.ApiErrorI18n(c, i18n.MsgModelUnavailable)
		return
	}
	if err := model.UpsertUserModelDiscount(userId, req.ModelName, req.Discount); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminDeleteUserModelDiscount 删除某用户某模型的专属折扣（恢复原价）
func AdminDeleteUserModelDiscount(c *gin.Context) {
	userId, ok := resolveManagedUserId(c)
	if !ok {
		return
	}
	modelName := strings.TrimSpace(c.Query("model_name"))
	if modelName == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := model.DeleteUserModelDiscount(userId, modelName); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ProviderGetUserModelDiscounts(c *gin.Context) {
	userId, ok := resolveProviderManagedUserId(c)
	if !ok {
		return
	}
	data, err := getUserModelDiscountListResponse(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func ProviderUpsertUserModelDiscount(c *gin.Context) {
	userId, ok := resolveProviderManagedUserId(c)
	if !ok {
		return
	}
	var req userModelDiscountRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	req.ModelName = strings.TrimSpace(req.ModelName)
	if req.ModelName == "" || req.Discount <= 0 || req.Discount > 1 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	allowed, err := isUserModelDiscountModelAllowed(userId, req.ModelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !allowed {
		common.ApiErrorI18n(c, i18n.MsgModelUnavailable)
		return
	}
	if err := model.UpsertUserModelDiscount(userId, req.ModelName, req.Discount); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ProviderDeleteUserModelDiscount(c *gin.Context) {
	userId, ok := resolveProviderManagedUserId(c)
	if !ok {
		return
	}
	modelName := strings.TrimSpace(c.Query("model_name"))
	if modelName == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err := model.DeleteUserModelDiscount(userId, modelName); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
