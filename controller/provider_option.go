package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
)

// providerOwnerOptionKeys is deliberately an allowlist. Besides protecting the
// current SMTP credentials, it prevents a future administrator-only option
// from becoming visible/editable through the provider-owner API by default.
var providerOwnerOptionKeys = map[string]struct{}{
	"console_setting.announcements":         {},
	"console_setting.announcements_enabled": {},
	model.ProviderTopUpGiftEnabledOptionKey: {},
	model.ProviderTopUpGiftRulesOptionKey:   {},
	model.ProviderTopUpGiftTimedOptionKey:   {},
}

func providerOwnerCanAccessOption(key string) bool {
	// Do not normalize an incoming key before persistence. Reject aliases
	// instead, avoiding case/space/collation tricks such as MAIL.SMTP on MySQL.
	if key != strings.TrimSpace(key) {
		return false
	}
	_, ok := providerOwnerOptionKeys[key]
	return ok
}

// authorizeAdminProviderOptions applies a stricter boundary than the generic
// AdminAuth middleware because these options may contain SMTP credentials. The
// caller must be a current (not merely session-cached) main-site administrator.
func authorizeAdminProviderOptions(c *gin.Context) bool {
	if common.GetContextKeyInt(c, constant.ContextKeyProviderId) != 0 || c.GetInt("role") < common.RoleAdminUser {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "该配置仅限主站系统管理员操作",
		})
		return false
	}

	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if user.ProviderId != 0 || user.Role < common.RoleAdminUser || user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "该配置仅限主站系统管理员操作",
		})
		return false
	}
	var ownedProviderCount int64
	if err := model.DB.Model(&model.Provider{}).Where("owner_user_id = ?", user.Id).Count(&ownedProviderCount).Error; err != nil {
		common.ApiError(c, err)
		return false
	}
	if ownedProviderCount > 0 {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "服务商账号无权访问邮箱配置",
		})
		return false
	}
	return true
}

func canManageProviderOptions(c *gin.Context, providerId int) bool {
	provider, err := model.GetProviderById(providerId)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if c.GetInt("role") >= common.RoleAdminUser {
		return true
	}
	if provider.OwnerUserId == c.GetInt("id") {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": "无权访问该服务商配置",
	})
	return false
}

func getProviderOptions(c *gin.Context, adminRequest bool) {
	// 服务商ID
	providerId, err := strconv.Atoi(c.Param("id"))

	// 验证服务商ID是否有效
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的服务商ID",
		})
		return
	}
	if !canManageProviderOptions(c, providerId) {
		return
	}

	// 获取服务商配置
	options, err := model.GetProviderOptions(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !adminRequest {
		visibleOptions := make([]*model.ProviderOption, 0, len(options))
		for _, option := range options {
			if option != nil && providerOwnerCanAccessOption(option.Key) {
				visibleOptions = append(visibleOptions, option)
			}
		}
		options = visibleOptions
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    options,
	})
}

// GetProviderOptions returns only the explicitly provider-manageable options.
func GetProviderOptions(c *gin.Context) {
	getProviderOptions(c, false)
}

// AdminGetProviderOptions returns all options, including administrator-owned
// SMTP credentials. Its route is protected by middleware.AdminAuth.
func AdminGetProviderOptions(c *gin.Context) {
	if !authorizeAdminProviderOptions(c) {
		return
	}
	// The response may contain SMTP credentials. Explicitly prevent browsers
	// and intermediary caches from retaining or replaying it.
	c.Header("Cache-Control", "no-store, private")
	c.Header("Pragma", "no-cache")
	getProviderOptions(c, true)
}

func updateProviderOption(c *gin.Context, adminRequest bool) {
	providerId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的服务商ID",
		})
		return
	}
	if !canManageProviderOptions(c, providerId) {
		return
	}

	var option OptionUpdateRequest
	err = common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	if !adminRequest && !providerOwnerCanAccessOption(option.Key) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "该配置仅限系统管理员操作",
		})
		return
	}

	// 转换值为字符串
	switch option.Value.(type) {
	case bool:
		option.Value = common.Interface2String(option.Value.(bool))
	case float64:
		option.Value = common.Interface2String(option.Value.(float64))
	case int:
		option.Value = common.Interface2String(option.Value.(int))
	default:
		option.Value = fmt.Sprintf("%v", option.Value)
	}

	switch option.Key {
	// 系统公告
	case "console_setting.announcements":
		// 验证系统公告是否符合要求
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case model.ProviderTopUpGiftTimedOptionKey:
		// 服务商倒计时独立存储，但与主站共用校验及服务端时间锚定规则。
		normalized, normalizeErr := model.NormalizeTopUpGiftTimedConfig(option.Value.(string), time.Now())
		if normalizeErr != nil {
			common.ApiErrorMsg(c, normalizeErr.Error())
			return
		}
		option.Value = normalized
	}

	// 更新服务商配置
	err = model.UpdateProviderOption(providerId, option.Key, option.Value.(string))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// UpdateProviderOption updates only options owned by the provider itself.
func UpdateProviderOption(c *gin.Context) {
	updateProviderOption(c, false)
}

// AdminUpdateProviderOption may update administrator-owned options such as
// mail.smtp. Its route is protected by middleware.AdminAuth.
func AdminUpdateProviderOption(c *gin.Context) {
	if !authorizeAdminProviderOptions(c) {
		return
	}
	updateProviderOption(c, true)
}
