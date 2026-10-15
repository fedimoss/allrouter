package controller

import (
	"errors"
	"fmt"
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
	AppId         string  `json:"app_id" binding:"required"`
	AppSecret     string  `json:"app_secret"`
	ServerAddress *string `json:"server_address"`
	ServerToken   string  `json:"server_token"`
	Enabled       *bool   `json:"enabled"`
}

type wechatOfficialConfigResponse struct {
	ProviderId     int    `json:"provider_id"`
	AppId          string `json:"app_id"`
	ServerAddress  string `json:"server_address"`
	HasServerToken bool   `json:"has_server_token"`
	QRCodeURL      string `json:"qrcode_url"`
	HasAppSecret   bool   `json:"has_app_secret"`
	Enabled        bool   `json:"enabled"`
}

func respondWechatOfficialConfig(c *gin.Context, config *model.WechatOfficialConfig) {
	serverAddress := config.ServerAddress
	hasServerToken := config.ServerToken != ""
	if config.ProviderId == 0 {
		if strings.TrimSpace(serverAddress) == "" {
			serverAddress = common.WeChatServerAddress
		}
		hasServerToken = hasServerToken || strings.TrimSpace(common.WeChatServerToken) != ""
	}
	common.ApiSuccess(c, wechatOfficialConfigResponse{
		ProviderId:     config.ProviderId,
		AppId:          config.AppId,
		ServerAddress:  serverAddress,
		HasServerToken: hasServerToken,
		QRCodeURL:      getGeneratedWechatQRCodeURL(config.QRCodeURL),
		HasAppSecret:   config.AppSecret != "",
		Enabled:        config.Enabled,
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
	serverAddress := ""
	if req.ServerAddress != nil {
		serverAddress = strings.TrimRight(strings.TrimSpace(*req.ServerAddress), "/")
	}
	req.ServerToken = strings.TrimSpace(req.ServerToken)
	if req.AppId == "" {
		common.ApiErrorMsg(c, "app_id is required")
		return
	}
	if len(req.AppId) > 255 || len(req.AppSecret) > 512 || len(serverAddress) > 2048 || len(req.ServerToken) > 512 {
		common.ApiErrorMsg(c, "configuration field exceeds the maximum length")
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
				ProviderId:    providerId,
				AppId:         req.AppId,
				AppSecret:     req.AppSecret,
				ServerAddress: serverAddress,
				ServerToken:   req.ServerToken,
				Enabled:       req.Enabled == nil || *req.Enabled,
			}
			if providerId == 0 {
				if req.ServerAddress == nil && config.ServerAddress == "" {
					config.ServerAddress = strings.TrimRight(strings.TrimSpace(common.WeChatServerAddress), "/")
				}
				if config.ServerToken == "" {
					config.ServerToken = strings.TrimSpace(common.WeChatServerToken)
				}
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

	// Changing credentials invalidates the generated image until a QR code for
	// the new official account is generated.
	credentialsChanged := existingConfig.AppId != req.AppId || req.AppSecret != "" && existingConfig.AppSecret != req.AppSecret
	if credentialsChanged {
		existingConfig.QRCodeURL = ""
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
	if req.ServerAddress != nil {
		existingConfig.ServerAddress = serverAddress
	}
	if req.ServerToken != "" {
		existingConfig.ServerToken = req.ServerToken
	}
	if providerId == 0 {
		if req.ServerAddress == nil && existingConfig.ServerAddress == "" {
			existingConfig.ServerAddress = strings.TrimRight(strings.TrimSpace(common.WeChatServerAddress), "/")
		}
		if existingConfig.ServerToken == "" {
			existingConfig.ServerToken = strings.TrimSpace(common.WeChatServerToken)
		}
	}

	if err := model.UpdateWechatConfig(existingConfig); err != nil {
		common.ApiError(c, err)
		return
	}

	respondWechatOfficialConfig(c, existingConfig)
}

// GenerateWechatOfficialQRCode creates a permanent QR code for the selected
// site's official account. Credentials are loaded from that site's database
// record and never accepted from the client. The image is cached as a data URL
// so the login page can render it without a second public file endpoint.
func GenerateWechatOfficialQRCode(c *gin.Context) {
	providerId := determineWechatConfigProviderId(c)
	if providerId < 0 {
		return
	}
	config, err := model.GetWechatConfigByProvider(providerId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "请先保存微信公众号 AppId 和 AppSecret")
			return
		}
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(config.AppId) == "" || strings.TrimSpace(config.AppSecret) == "" {
		common.ApiErrorMsg(c, "请先填写完整的微信公众号 AppId 和 AppSecret")
		return
	}
	if getGeneratedWechatQRCodeURL(config.QRCodeURL) != "" {
		respondWechatOfficialConfig(c, config)
		return
	}

	// Use one stable scene value per site so scans can always be attributed to
	// the correct site. The saved image makes this endpoint idempotent.
	result, err := createWechatPermanentQRCodeForConfig(c.Request.Context(), config, fmt.Sprintf("new-api-login-%d", providerId))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := persistWechatQRCode(config, result); err != nil {
		common.ApiError(c, err)
		return
	}
	respondWechatOfficialConfig(c, config)
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
	route = strings.TrimSuffix(route, "/qrcode")
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
