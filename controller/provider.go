package controller

import (
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type providerAdminResponse struct {
	model.Provider
	Domains      []model.ProviderDomain       `json:"domains"`
	Config       *model.ProviderConfig        `json:"config,omitempty"`
	ModelPricing []model.ProviderModelPricing `json:"model_pricing,omitempty"`
}

type providerAdminRequest struct {
	OwnerUserId int    `json:"owner_user_id"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
}

type providerOwnerCandidate struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

const (
	defaultProviderPrimaryColor   = "#09FEF7"
	defaultProviderSecondaryColor = "#BAFF29"
)

var providerHexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{3}$|^#[0-9a-fA-F]{6}$`)

func normalizeProviderHexColor(color string) (string, bool) {
	color = strings.TrimSpace(color)
	if color == "" {
		return "", true
	}
	return color, providerHexColorPattern.MatchString(color)
}

func providerThemeColors(cfg *model.ProviderConfig) (string, string) {
	primary := defaultProviderPrimaryColor
	secondary := defaultProviderSecondaryColor
	if cfg == nil {
		return primary, secondary
	}
	if color, ok := normalizeProviderHexColor(cfg.ThemeColor); ok && color != "" {
		primary = color
	}
	if color, ok := normalizeProviderHexColor(cfg.SecondaryColor); ok && color != "" {
		secondary = color
	}
	return primary, secondary
}

// providerConfigResponse 构建提供商配置响应
func providerConfigResponse(c *gin.Context, cfg *model.ProviderConfig) gin.H {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	resp := gin.H{"provider_id": providerId, "enabled": providerId > 0}
	primaryColor, secondaryColor := providerThemeColors(cfg)
	resp["primary_color"] = primaryColor
	resp["secondary_color"] = secondaryColor
	if cfg == nil {
		if providerName := c.GetString("provider_name"); providerName != "" {
			resp["site_name"] = providerName
		}
		return resp
	}
	resp["site_name"] = cfg.SiteName
	resp["logo"] = cfg.Logo
	resp["theme_color"] = cfg.ThemeColor
	resp["login_background"] = cfg.LoginBackground
	resp["home_modules"] = cfg.HomeModules
	resp["nav_modules"] = cfg.NavModules
	resp["pricing_display"] = cfg.PricingDisplay
	resp["announcement"] = cfg.Announcement
	resp["footer_text"] = cfg.FooterText
	resp["support_url"] = cfg.SupportUrl
	resp["wechat_support"] = cfg.WechatSupport // 微信客服
	resp["qq_support"] = cfg.QQSupport         // QQ客服
	return resp
}

func parseProviderAdminId(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid provider id")
		return 0, false
	}
	return id, true
}

func normalizeProviderStatus(status int) int {
	if status == model.ProviderStatusDisabled {
		return model.ProviderStatusDisabled
	}
	return model.ProviderStatusEnabled
}

func normalizeProviderDomainStatus(status int) int {
	if status == model.ProviderDomainStatusVerified {
		return model.ProviderDomainStatusVerified
	}
	return model.ProviderDomainStatusPending
}

func validateProviderRebateRatio(ratio float64) bool {
	return ratio >= 0 && ratio <= 100
}

func buildProviderAdminResponses(providers []model.Provider, withPricing bool) ([]providerAdminResponse, error) {
	responses := make([]providerAdminResponse, 0, len(providers))
	if len(providers) == 0 {
		return responses, nil
	}
	ids := make([]int, 0, len(providers))
	for _, provider := range providers {
		ids = append(ids, provider.Id)
	}

	var domains []model.ProviderDomain
	if err := model.DB.Where("provider_id IN ?", ids).Order("id desc").Find(&domains).Error; err != nil {
		return nil, err
	}
	domainMap := make(map[int][]model.ProviderDomain)
	for _, domain := range domains {
		domainMap[domain.ProviderId] = append(domainMap[domain.ProviderId], domain)
	}

	var configs []model.ProviderConfig
	if err := model.DB.Where("provider_id IN ?", ids).Find(&configs).Error; err != nil {
		return nil, err
	}
	configMap := make(map[int]model.ProviderConfig)
	for _, cfg := range configs {
		configMap[cfg.ProviderId] = cfg
	}

	pricingMap := make(map[int][]model.ProviderModelPricing)
	if withPricing {
		var pricing []model.ProviderModelPricing
		if err := model.DB.Where("provider_id IN ?", ids).Order("id desc").Find(&pricing).Error; err != nil {
			return nil, err
		}
		for _, row := range pricing {
			pricingMap[row.ProviderId] = append(pricingMap[row.ProviderId], row)
		}
	}

	for _, provider := range providers {
		resp := providerAdminResponse{
			Provider:     provider,
			Domains:      make([]model.ProviderDomain, 0),
			ModelPricing: make([]model.ProviderModelPricing, 0),
		}
		if domains, ok := domainMap[provider.Id]; ok {
			resp.Domains = domains
		}
		if pricing, ok := pricingMap[provider.Id]; ok {
			resp.ModelPricing = pricing
		}
		if cfg, ok := configMap[provider.Id]; ok {
			resp.Config = &cfg
		}
		responses = append(responses, resp)
	}
	return responses, nil
}

func AdminListProviders(c *gin.Context) {
	var providers []model.Provider
	if err := model.DB.Order("id desc").Find(&providers).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	responses, err := buildProviderAdminResponses(providers, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, responses)
}

func AdminListProviderOwnerCandidates(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	currentProviderId, _ := strconv.Atoi(c.Query("current_provider_id"))

	var providers []model.Provider
	if err := model.DB.Select("id, owner_user_id").Find(&providers).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	usedOwnerIds := make([]int, 0, len(providers))
	for _, provider := range providers {
		if currentProviderId > 0 && provider.Id == currentProviderId {
			continue
		}
		usedOwnerIds = append(usedOwnerIds, provider.OwnerUserId)
	}

	query := model.DB.Model(&model.User{}).
		Select("id, username, display_name, email").
		Where("provider_id = ? AND role < ?", 0, common.RoleAdminUser)
	if len(usedOwnerIds) > 0 {
		query = query.Where("id NOT IN ?", usedOwnerIds)
	}
	if keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		if idKeyword, err := strconv.Atoi(keyword); err == nil && idKeyword > 0 {
			query = query.Where("LOWER(username) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(email) LIKE ? OR id = ?", like, like, like, idKeyword)
		} else {
			query = query.Where("LOWER(username) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(email) LIKE ?", like, like, like)
		}
	}

	var users []providerOwnerCandidate
	if err := query.Order("id desc").Limit(30).Find(&users).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, users)
}

func validateProviderOwnerCandidate(userId int, currentProviderId int) bool {
	var user model.User
	if err := model.DB.Select("id, provider_id, role").Where("id = ?", userId).First(&user).Error; err != nil {
		return false
	}
	if user.ProviderId != 0 || user.Role >= common.RoleAdminUser {
		return false
	}
	var count int64
	query := model.DB.Model(&model.Provider{}).Where("owner_user_id = ?", userId)
	if currentProviderId > 0 {
		query = query.Where("id <> ?", currentProviderId)
	}
	if err := query.Count(&count).Error; err != nil {
		return false
	}
	return count == 0
}

func AdminCreateProvider(c *gin.Context) {
	var req providerAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.OwnerUserId <= 0 || req.Name == "" {
		common.ApiErrorMsg(c, "provider name and owner user id are required")
		return
	}
	if !validateProviderOwnerCandidate(req.OwnerUserId, 0) {
		common.ApiErrorMsg(c, "owner user is not eligible")
		return
	}
	now := common.GetTimestamp()
	provider := model.Provider{
		OwnerUserId: req.OwnerUserId,
		Name:        req.Name,
		Status:      normalizeProviderStatus(req.Status),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := model.DB.Create(&provider).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, provider)
}

func AdminUpdateProvider(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	var req providerAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.OwnerUserId <= 0 || req.Name == "" {
		common.ApiErrorMsg(c, "provider name and owner user id are required")
		return
	}
	if !validateProviderOwnerCandidate(req.OwnerUserId, id) {
		common.ApiErrorMsg(c, "owner user is not eligible")
		return
	}
	if err := model.DB.Model(&model.Provider{}).Where("id = ?", id).Updates(map[string]interface{}{
		"owner_user_id": req.OwnerUserId,
		"name":          req.Name,
		"status":        normalizeProviderStatus(req.Status),
		"updated_at":    common.GetTimestamp(),
	}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminDisableProvider(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	if err := model.DB.Model(&model.Provider{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     model.ProviderStatusDisabled,
		"updated_at": common.GetTimestamp(),
	}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func upsertProviderConfig(c *gin.Context, providerId int) {
	var req model.ProviderConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.ProviderId = providerId
	themeColor, ok := normalizeProviderHexColor(req.ThemeColor)
	if !ok {
		common.ApiErrorMsg(c, "invalid theme color")
		return
	}
	secondaryColor, ok := normalizeProviderHexColor(req.SecondaryColor)
	if !ok {
		common.ApiErrorMsg(c, "invalid secondary color")
		return
	}
	req.ThemeColor = themeColor
	req.SecondaryColor = secondaryColor
	updates := map[string]interface{}{
		"site_name":        strings.TrimSpace(req.SiteName),
		"logo":             strings.TrimSpace(req.Logo),
		"theme_color":      req.ThemeColor,
		"secondary_color":  req.SecondaryColor,
		"login_background": strings.TrimSpace(req.LoginBackground),
		"home_modules":     req.HomeModules,
		"nav_modules":      req.NavModules,
		"pricing_display":  req.PricingDisplay,
		"announcement":     strings.TrimSpace(req.Announcement),
		"footer_text":      strings.TrimSpace(req.FooterText),
		"support_url":      strings.TrimSpace(req.SupportUrl),
		"updated_at":       common.GetTimestamp(),
		"wechat_support":   strings.TrimSpace(req.WechatSupport), // 微信客服
		"qq_support":       strings.TrimSpace(req.QQSupport),     // QQ客服
	}
	var cfg model.ProviderConfig
	err := model.DB.Where("provider_id = ?", providerId).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		req.Id = 0
		req.CreatedAt = common.GetTimestamp()
		req.UpdatedAt = req.CreatedAt
		if err := model.DB.Create(&req).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, req)
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DB.Model(&cfg).Updates(updates).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminUpsertProviderConfig(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	upsertProviderConfig(c, id)
}

func AdminUploadProviderLogo(c *gin.Context) {
	logoURL, ok := saveUploadedLogo(c)
	if !ok {
		return
	}
	common.ApiSuccess(c, gin.H{"url": logoURL})
}

func createProviderDomain(c *gin.Context, providerId int, allowVerified bool) {
	var req model.ProviderDomain
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Domain = strings.TrimSpace(strings.ToLower(req.Domain))
	if req.Domain == "" {
		common.ApiErrorMsg(c, "domain is required")
		return
	}
	status := model.ProviderDomainStatusPending
	if allowVerified {
		status = normalizeProviderDomainStatus(req.Status)
	}
	now := common.GetTimestamp()
	domain := model.ProviderDomain{
		ProviderId:  providerId,
		Domain:      req.Domain,
		Status:      status,
		VerifyToken: strings.TrimSpace(req.VerifyToken),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := model.DB.Create(&domain).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, domain)
}

func updateProviderDomain(c *gin.Context, providerId int, allowVerified bool) {
	domainId, err := strconv.Atoi(c.Param("domain_id"))
	if err != nil || domainId <= 0 {
		common.ApiErrorMsg(c, "invalid domain id")
		return
	}
	var req model.ProviderDomain
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Domain = strings.TrimSpace(strings.ToLower(req.Domain))
	if req.Domain == "" {
		common.ApiErrorMsg(c, "domain is required")
		return
	}
	status := model.ProviderDomainStatusPending
	if allowVerified {
		status = normalizeProviderDomainStatus(req.Status)
	}
	if err := model.DB.Model(&model.ProviderDomain{}).
		Where("id = ? AND provider_id = ?", domainId, providerId).
		Updates(map[string]interface{}{
			"domain":       req.Domain,
			"status":       status,
			"verify_token": strings.TrimSpace(req.VerifyToken),
			"updated_at":   common.GetTimestamp(),
		}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func deleteProviderDomain(c *gin.Context, providerId int) {
	domainId, err := strconv.Atoi(c.Param("domain_id"))
	if err != nil || domainId <= 0 {
		common.ApiErrorMsg(c, "invalid domain id")
		return
	}
	if err := model.DB.Where("id = ? AND provider_id = ?", domainId, providerId).Delete(&model.ProviderDomain{}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminCreateProviderDomain(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	createProviderDomain(c, id, true)
}

func AdminUpdateProviderDomain(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	updateProviderDomain(c, id, true)
}

func AdminDeleteProviderDomain(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	deleteProviderDomain(c, id)
}

func listProviderModelPricing(c *gin.Context, providerId int) {
	var rows []model.ProviderModelPricing
	if err := model.DB.Where("provider_id = ?", providerId).Order("id desc").Find(&rows).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func listProviderBaseModels(c *gin.Context) {
	models := model.GetEnabledModels()
	sort.Strings(models)
	common.ApiSuccess(c, models)
}

func upsertProviderModelPricing(c *gin.Context, providerId int) {
	var req model.ProviderModelPricing
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.ProviderId = providerId
	req.PublicModelName = strings.TrimSpace(req.PublicModelName)
	req.BaseModelName = strings.TrimSpace(req.BaseModelName)
	if req.PublicModelName == "" || req.BaseModelName == "" {
		common.ApiErrorMsg(c, "model name is required")
		return
	}
	if req.PricingType != model.ProviderPricingTypeDelta {
		req.PricingType = model.ProviderPricingTypeRatio
	}
	if req.Ratio == 0 {
		req.Ratio = 1
	}
	if !validateProviderRebateRatio(req.ConsumeRebateRatioLevel1) || !validateProviderRebateRatio(req.ConsumeRebateRatioLevel2) {
		common.ApiErrorMsg(c, "consume rebate ratio must be between 0 and 100")
		return
	}
	if req.Id == 0 {
		if err := model.DB.Create(&req).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, req)
		return
	}
	if err := model.DB.Model(&model.ProviderModelPricing{}).
		Where("id = ? AND provider_id = ?", req.Id, providerId).
		Updates(map[string]interface{}{
			"public_model_name":           req.PublicModelName,
			"base_model_name":             req.BaseModelName,
			"enabled":                     req.Enabled,
			"pricing_type":                req.PricingType,
			"ratio":                       req.Ratio,
			"delta_model_ratio":           req.DeltaModelRatio,
			"delta_model_price":           req.DeltaModelPrice,
			"consume_rebate_ratio_level1": req.ConsumeRebateRatioLevel1,
			"consume_rebate_ratio_level2": req.ConsumeRebateRatioLevel2,
			"updated_at":                  common.GetTimestamp(),
		}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func deleteProviderModelPricing(c *gin.Context, providerId int, paramName string) {
	id, err := strconv.Atoi(c.Param(paramName))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid pricing id")
		return
	}
	if err := model.DB.Where("id = ? AND provider_id = ?", id, providerId).Delete(&model.ProviderModelPricing{}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func AdminListProviderModelPricing(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	listProviderModelPricing(c, id)
}

func AdminListProviderBaseModels(c *gin.Context) {
	listProviderBaseModels(c)
}

func AdminUpsertProviderModelPricing(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	upsertProviderModelPricing(c, id)
}

func AdminDeleteProviderModelPricing(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	deleteProviderModelPricing(c, id, "pricing_id")
}

func GetProviderPublicConfig(c *gin.Context) {
	if v, ok := c.Get("provider_config"); ok {
		if cfg, ok := v.(model.ProviderConfig); ok {
			common.ApiSuccess(c, providerConfigResponse(c, &cfg))
			return
		}
	}
	common.ApiSuccess(c, providerConfigResponse(c, nil))
}

func getOwnedProvider(c *gin.Context) (*model.Provider, bool) {
	userId := c.GetInt("id")
	provider, err := model.GetProviderByOwnerUserId(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "current user is not a provider owner"})
			return nil, false
		}
		common.ApiError(c, err)
		return nil, false
	}
	return provider, true
}

func GetProviderSelf(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	responses, err := buildProviderAdminResponses([]model.Provider{*provider}, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(responses) == 0 {
		common.ApiErrorMsg(c, "provider not found")
		return
	}
	common.ApiSuccess(c, responses[0])
}

func UpdateProviderSelf(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	var req providerAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		common.ApiErrorMsg(c, "provider name is required")
		return
	}
	if err := model.DB.Model(&model.Provider{}).Where("id = ? AND owner_user_id = ?", provider.Id, c.GetInt("id")).Updates(map[string]interface{}{
		"name":       req.Name,
		"updated_at": common.GetTimestamp(),
	}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func CreateProviderSelfDomain(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	createProviderDomain(c, provider.Id, true)
}

func UpdateProviderSelfDomain(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	updateProviderDomain(c, provider.Id, true)
}

func DeleteProviderSelfDomain(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	deleteProviderDomain(c, provider.Id)
}

func GetProviderSelfConfig(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	var cfg model.ProviderConfig
	if err := model.DB.Where("provider_id = ?", provider.Id).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiSuccess(c, gin.H{"provider_id": provider.Id, "site_name": provider.Name})
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cfg)
}

func UpsertProviderSelfConfig(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	upsertProviderConfig(c, provider.Id)
}

func UploadProviderLogo(c *gin.Context) {
	if _, ok := getOwnedProvider(c); !ok {
		return
	}
	logoURL, ok := saveUploadedLogo(c)
	if !ok {
		return
	}
	common.ApiSuccess(c, gin.H{"url": logoURL})
}

func ListProviderModelPricing(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	listProviderModelPricing(c, provider.Id)
}

func ListProviderBaseModels(c *gin.Context) {
	if _, ok := getOwnedProvider(c); !ok {
		return
	}
	listProviderBaseModels(c)
}

func UpsertProviderModelPricing(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	upsertProviderModelPricing(c, provider.Id)
}

func DeleteProviderModelPricing(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	deleteProviderModelPricing(c, provider.Id, "id")
}

// AddProviderWithdrawRequest 添加提现申请
func AddProviderWithdrawRequest(c *gin.Context) {
	// 解析请求参数
	amount, err := strconv.ParseFloat(c.Query("amount"), 64)
	if err != nil || amount <= 0 {
		common.ApiErrorMsg(c, "amount must be greater than 0")
		return
	}

	// 根据用户ID获取Provider信息
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}

	// 根据用户时区确定币种，并校验可用余额
	userId := c.GetInt("id")
	var user model.User
	timezone := "America/New_York"
	if err := model.DB.Select("quota, reward_quota, timezone").Where("id = ?", userId).First(&user).Error; err == nil && user.Timezone != "" {
		timezone = user.Timezone
	}
	currencyCode := model.GetCurrencyByTimezoneWithFallback(timezone, "usd")

	// 获取美元人民币汇率
	usdToCnyRate := operation_setting.USDExchangeRate

	// 计算可用余额并与提现金额比较
	availableBalance := float64(user.Quota-user.RewardQuota) / common.QuotaPerUnit

	// 如果用户币种是 CNY，availableBalance 转为人民币
	if strings.ToLower(currencyCode) == "cny" {
		availableBalance = availableBalance * usdToCnyRate
	}

	// 此时 amount 和 availableBalance 已经是同一币种，直接比较
	if amount > availableBalance {
		common.ApiErrorMsg(c, "insufficient balance")
		return
	}

	// 根据币种计算 USD/CNY 金额并转换为符号
	currencyCode = strings.ToLower(currencyCode)
	var currency string
	var usdAmount, cnyAmount float64
	switch currencyCode {
	case "cny":
		currency = "￥"
		cnyAmount = amount
		usdAmount = amount / usdToCnyRate
	default:
		currency = "$"
		usdAmount = amount
		cnyAmount = amount * usdToCnyRate
	}

	// 构建提现申请
	withdraw := model.ProviderWithdraw{
		ProviderId:   provider.Id,                         // 服务商ID
		Amount:       amount,                              // 提现金额
		Currency:     currency,                            // 提现币种(符号)
		UsdAmount:    usdAmount,                           // 提现金额(美元)
		CnyAmount:    cnyAmount,                           // 提现金额(人民币)
		UsdToCnyRate: usdToCnyRate,                        // 美元人民币汇率
		Status:       model.ProviderWithdrawStatusPending, // 待审核
		CreatedAt:    common.GetTimestamp(),               // 创建时间
		UpdatedAt:    common.GetTimestamp(),               // 更新时间
	}

	// 创建提现申请
	if err := model.CreateProviderWithdraw(&withdraw); err != nil {
		common.ApiError(c, err)
		return
	}

	// 返回提现申请
	common.ApiSuccess(c, withdraw)
}

// 提现申请列表
func GetProviderWithdrawList(c *gin.Context) {
	// 根据用户ID获取Provider信息
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}

	// 分页查询
	pageInfo := common.GetPageQuery(c)

	// 提现申请状态
	status, _ := strconv.Atoi(c.Query("status"))

	// 查询提现申请列表
	records, total, err := model.GetProviderWithdraws(provider.Id, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 返回提现申请列表
	pageInfo.SetItems(records)
	pageInfo.SetTotal(int(total))
	common.ApiSuccess(c, pageInfo)
}

// 提现申请数据概览
func GetProviderWithdrawDashboard(c *gin.Context) {
	if _, ok := getOwnedProvider(c); !ok {
		return
	}

	// 获取当前用户ID
	userId := c.GetInt("id")

	// 根据用户ID查询User表
	var user model.User
	if err := model.DB.Select("quota, reward_quota, timezone").Where("id = ?", userId).First(&user).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	// 计算可用余额（美元），保留尽可能多的小数位
	availableBalance := float64(user.Quota-user.RewardQuota) / common.QuotaPerUnit

	// 根据用户时区确定币种
	timezone := user.Timezone
	if timezone == "" {
		timezone = "America/New_York"
	}

	// 获取时区对应的币种，默认使用美元
	currency := model.GetCurrencyByTimezoneWithFallback(timezone, "usd")

	// 如果币种为 CNY，根据汇率转换为人民币
	if strings.ToLower(currency) == "cny" {
		availableBalance = availableBalance * operation_setting.USDExchangeRate
	}

	if strings.ToLower(currency) == "usd" {
		currency = "$"
	} else {
		currency = "￥"
	}

	common.ApiSuccess(c, gin.H{
		"available_balance": availableBalance,
		"currency":          currency,
	})
}

// 提现申请列表 (管理员接口)
func AdminGetProviderWithdrawList(c *gin.Context) {
	// 分页查询
	pageInfo := common.GetPageQuery(c)

	// 可选筛选参数
	providerId, _ := strconv.Atoi(c.Query("provider_id")) // 服务商ID
	providerName := c.Query("provider_name")              // 服务商名称
	status, _ := strconv.Atoi(c.Query("status"))          // 提现申请状态

	// 查询提现申请列表（JOIN providers 表以支持供应商名称筛选）
	records, total, err := model.SearchProviderWithdraws(providerId, providerName, status, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 返回提现申请列表
	pageInfo.SetItems(records)
	pageInfo.SetTotal(int(total))
	common.ApiSuccess(c, pageInfo)
}

// 提现申请审核 (管理员接口)
func AdminApproveProviderWithdrawRequest(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id")) // 获取提现申请ID
	action := c.Query("action")          // 获取提现申请操作

	// 查询提现申请记录
	var withdraw model.ProviderWithdraw
	if err := model.DB.Where("id = ?", id).First(&withdraw).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	// 根据操作处理
	var status int
	switch action {
	case "approve":
		status = model.ProviderWithdrawStatusApproved

		// 查询服务商对应的用户
		var provider model.Provider
		if err := model.DB.Select("owner_user_id").Where("id = ?", withdraw.ProviderId).First(&provider).Error; err != nil {
			common.ApiError(c, err)
			return
		}

		// 计算需要扣除的美元金额
		var usdAmount float64
		if withdraw.Currency == "￥" {
			usdAmount = withdraw.Amount / withdraw.UsdToCnyRate
		} else {
			usdAmount = withdraw.Amount
		}

		// 转换为内部额度并扣除
		deduction := int(usdAmount * common.QuotaPerUnit)
		// 检查服务商余额是否足够
		result := model.DB.Model(&model.User{}).Where("id = ? AND quota >= ?", provider.OwnerUserId, deduction).
			UpdateColumn("quota", gorm.Expr("quota - ?", deduction))
		if result.Error != nil {
			common.ApiError(c, result.Error)
			return
		}
		if result.RowsAffected == 0 {
			common.ApiErrorMsg(c, i18n.T(c, i18n.MsgProviderWithdrawInsufficientBalance))
			return
		}

	case "reject":
		status = model.ProviderWithdrawStatusRejected
	default:
		common.ApiErrorMsg(c, "invalid action")
		return
	}

	// 修改提现申请状态
	if err := model.UpdateProviderWithdrawStatus(id, status); err != nil {
		common.ApiError(c, err)
		return
	}

	// 返回成功
	common.ApiSuccess(c, gin.H{
		"message": "success",
	})
}

// 取消提现申请（服务商在自己的提现管理中取消待审核的申请）
func CancelProviderWithdrawRequest(c *gin.Context) {
	// 1. 解析提现申请 ID
	id, _ := strconv.Atoi(c.Query("id"))

	// 2. 获取当前登录服务商，校验身份
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}

	// 3. 查询提现申请是否存在
	var withdraw model.ProviderWithdraw
	if err := model.DB.Where("id = ?", id).First(&withdraw).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	// 4. 校验该申请是否属于当前服务商
	if withdraw.ProviderId != provider.Id {
		common.ApiErrorMsg(c, i18n.T(c, i18n.MsgProviderWithdrawNotYours))
		return
	}

	// 5. 只允许取消"待审核"状态的申请
	if withdraw.Status != model.ProviderWithdrawStatusPending {
		common.ApiErrorMsg(c, i18n.T(c, i18n.MsgProviderWithdrawCannotCancel))
		return
	}

	// 6. 更新状态为"已取消"
	if err := model.UpdateProviderWithdrawStatus(id, model.ProviderWithdrawStatusCancelled); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"message": "success",
	})
}

// 提现申请数据概览 (管理员接口)
func AdminGetProviderWithdrawDashboard(c *gin.Context) {
	var totalCount, todayCount int64

	model.DB.Model(&model.ProviderWithdraw{}).Count(&totalCount)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	model.DB.Model(&model.ProviderWithdraw{}).Where("created_at >= ?", todayStart).Count(&todayCount)

	common.ApiSuccess(c, gin.H{
		"total_count": totalCount,
		"today_count": todayCount,
	})
}
