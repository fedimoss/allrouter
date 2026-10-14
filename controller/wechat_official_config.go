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

type wechatOfficialConfigRequest struct {
	AppId     string  `json:"app_id" binding:"required"`
	AppSecret string  `json:"app_secret"`
	QRCodeURL *string `json:"qrcode_url"`
	Enabled   *bool   `json:"enabled"`
}

type wechatOfficialConfigResponse struct {
	ProviderId   int    `json:"provider_id"`
	AppId        string `json:"app_id"`
	QRCodeURL    string `json:"qrcode_url"`
	HasAppSecret bool   `json:"has_app_secret"`
	Enabled      bool   `json:"enabled"`
}

func respondWechatOfficialConfig(c *gin.Context, config *model.WechatOfficialConfig) {
	common.ApiSuccess(c, wechatOfficialConfigResponse{
		ProviderId:   config.ProviderId,
		AppId:        config.AppId,
		QRCodeURL:    config.QRCodeURL,
		HasAppSecret: config.AppSecret != "",
		Enabled:      config.Enabled,
	})
}

// GetWechatOfficialConfig 获取微信服务号配置
func GetWechatOfficialConfig(c *gin.Context) {
	providerId := determineWechatConfigProviderId(c)
	if providerId < 0 {
		return
	}

	config, err := model.GetWechatConfigByProvider(providerId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondWechatOfficialConfig(c, &model.WechatOfficialConfig{ProviderId: providerId, Enabled: true})
			return
		}
		common.ApiError(c, err)
		return
	}

	respondWechatOfficialConfig(c, config)
}

// SaveWechatOfficialConfig 保存微信服务号配置
func SaveWechatOfficialConfig(c *gin.Context) {
	providerId := determineWechatConfigProviderId(c)
	if providerId < 0 {
		return
	}

	var req wechatOfficialConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	req.AppId = strings.TrimSpace(req.AppId)
	req.AppSecret = strings.TrimSpace(req.AppSecret)
	if req.QRCodeURL != nil {
		*req.QRCodeURL = strings.TrimSpace(*req.QRCodeURL)
	}
	if req.AppId == "" {
		common.ApiErrorMsg(c, "app_id is required")
		return
	}
	if len(req.AppId) > 255 || len(req.AppSecret) > 512 {
		common.ApiErrorMsg(c, "app_id or app_secret exceeds the maximum length")
		return
	}
	if req.QRCodeURL != nil && len(*req.QRCodeURL) > 2048 {
		common.ApiErrorMsg(c, "qrcode_url exceeds the maximum length")
		return
	}

	// 查询是否已存在配置
	existingConfig, err := model.GetWechatConfigByProvider(providerId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新配置
			if req.AppSecret == "" {
				common.ApiErrorMsg(c, "app_secret is required when creating new config")
				return
			}
			config := &model.WechatOfficialConfig{
				ProviderId: providerId,
				AppId:      req.AppId,
				AppSecret:  req.AppSecret,
				Enabled:    req.Enabled == nil || *req.Enabled,
			}
			if req.QRCodeURL != nil {
				config.QRCodeURL = *req.QRCodeURL
			}
			if err := model.CreateWechatConfig(config); err != nil {
				common.ApiError(c, err)
				return
			}
			respondWechatOfficialConfig(c, config)
			return
		}
		common.ApiError(c, err)
		return
	}

	if req.AppSecret == "" && (existingConfig.AppSecret == "" || existingConfig.AppId != req.AppId) {
		common.ApiErrorMsg(c, "app_secret is required when changing app_id or when no secret is configured")
		return
	}

	// 更新配置；省略 enabled 时保留原值。
	existingConfig.AppId = req.AppId
	if req.Enabled != nil {
		existingConfig.Enabled = *req.Enabled
	}
	// 如果 AppSecret 不为空，则更新；否则保持原值
	if req.AppSecret != "" {
		existingConfig.AppSecret = req.AppSecret
	}
	if req.QRCodeURL != nil {
		existingConfig.QRCodeURL = *req.QRCodeURL
	}

	if err := model.UpdateWechatConfig(existingConfig); err != nil {
		common.ApiError(c, err)
		return
	}

	respondWechatOfficialConfig(c, existingConfig)
}

// DeleteWechatOfficialConfig 删除微信服务号配置
func DeleteWechatOfficialConfig(c *gin.Context) {
	providerId := determineWechatConfigProviderId(c)
	if providerId < 0 {
		return
	}

	if err := model.DeleteWechatConfig(providerId); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}

func determineWechatConfigProviderId(c *gin.Context) int {
	c.Header("Cache-Control", "no-store, private")
	c.Header("Pragma", "no-cache")
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	siteProviderId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)

	switch {
	case route == "/api/option/wechat_official_config":
		if c.GetInt("role") >= common.RoleRootUser && siteProviderId == 0 {
			return 0
		}
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "only the main-site root administrator can manage this configuration"})
		return -1
	case route == "/api/provider/wechat_official_config":
		provider, isOwner, ok := getPermittedProvider(c, "provider")
		if !ok {
			return -1
		}
		if !isOwner {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "only provider owner can manage this configuration"})
			return -1
		}
		return provider.Id
	case route == "/api/provider/admin/:id/wechat_official_config" || strings.HasPrefix(route, "/api/provider/admin/") && strings.HasSuffix(route, "/wechat_official_config"):
		if !authorizeWechatConfigAdmin(c) {
			return -1
		}
		providerId, err := strconv.Atoi(c.Param("id"))
		if err != nil || providerId <= 0 {
			common.ApiErrorMsg(c, "invalid provider id")
			return -1
		}
		if _, err := model.GetProviderById(providerId); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				common.ApiErrorMsg(c, "provider not found")
			} else {
				common.ApiError(c, err)
			}
			return -1
		}
		return providerId
	default:
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "invalid configuration scope"})
		return -1
	}
}

func authorizeWechatConfigAdmin(c *gin.Context) bool {
	if common.GetContextKeyInt(c, constant.ContextKeyProviderId) != 0 {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "this configuration can only be managed from the main site"})
		return false
	}
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if user.ProviderId != 0 || user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "this configuration can only be managed by a main-site administrator"})
		return false
	}
	if user.Role < common.RoleAdminUser && !delegatedHasModule(c, "provider") {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "no permission to manage provider WeChat configuration"})
		return false
	}
	return true
}
