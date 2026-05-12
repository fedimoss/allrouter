package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
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

func providerConfigResponse(c *gin.Context, cfg *model.ProviderConfig) gin.H {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	resp := gin.H{"provider_id": providerId, "enabled": providerId > 0}
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
	updates := map[string]interface{}{
		"site_name":        strings.TrimSpace(req.SiteName),
		"logo":             strings.TrimSpace(req.Logo),
		"theme_color":      strings.TrimSpace(req.ThemeColor),
		"login_background": strings.TrimSpace(req.LoginBackground),
		"home_modules":     req.HomeModules,
		"nav_modules":      req.NavModules,
		"pricing_display":  req.PricingDisplay,
		"announcement":     strings.TrimSpace(req.Announcement),
		"footer_text":      strings.TrimSpace(req.FooterText),
		"support_url":      strings.TrimSpace(req.SupportUrl),
		"updated_at":       common.GetTimestamp(),
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
			"public_model_name": req.PublicModelName,
			"base_model_name":   req.BaseModelName,
			"enabled":           req.Enabled,
			"pricing_type":      req.PricingType,
			"ratio":             req.Ratio,
			"delta_model_ratio": req.DeltaModelRatio,
			"delta_model_price": req.DeltaModelPrice,
			"updated_at":        common.GetTimestamp(),
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
