package controller

import (
	"errors"
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
	"gorm.io/gorm"
)

// providerOwnerOptionKeys is deliberately an allowlist. SMTP is included only
// because its response is separately redacted; a future administrator-only
// option must not become visible/editable through the owner API by default.
var providerOwnerOptionKeys = map[string]struct{}{
	"console_setting.announcements":         {},
	"console_setting.announcements_enabled": {},
	model.ProviderTopUpGiftEnabledOptionKey: {},
	model.ProviderTopUpGiftRulesOptionKey:   {},
	model.ProviderTopUpGiftTimedOptionKey:   {},
	model.ProviderMailOptionKey:             {},
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

func providerMailSecret(config map[string]any) string {
	for _, preferredKey := range []string{"password", "smtp_token"} {
		if value, ok := config[preferredKey].(string); ok && value != "" {
			return value
		}
		for key, value := range config {
			if strings.EqualFold(strings.TrimSpace(key), preferredKey) {
				if secret, ok := value.(string); ok && secret != "" {
					return secret
				}
			}
		}
	}
	return ""
}

var providerMailKnownInputKeys = []string{
	"enabled",
	"host",
	"port",
	"username",
	"password",
	"from_email",
	"from_name",
	"reply_to",
	"encryption",
	"force_auth_login",
	"timeout_seconds",
	"password_configured",
	// Legacy names accepted for old stored configurations and API clients.
	"smtp_server",
	"smtp_port",
	"smtp_account",
	"smtp_token",
	"smtp_from",
	"smtp_ssl_enabled",
	"ssl_enabled",
	"smtp_force_auth_login",
}

// validateProviderMailInputKeys rejects case/whitespace aliases of fields that
// encoding/json would otherwise match case-insensitively. Without this check,
// decoding the submitted object into a struct and then re-marshalling its map
// can change which duplicate spelling wins, allowing the validated SMTP target
// to differ from the target used when mail is sent.
func validateProviderMailInputKeys(config map[string]any) error {
	for key := range config {
		trimmedKey := strings.TrimSpace(key)
		for _, knownKey := range providerMailKnownInputKeys {
			if strings.EqualFold(trimmedKey, knownKey) && key != knownKey {
				return errors.New("邮箱配置包含大小写或空格不规范的字段")
			}
		}
	}
	return nil
}

type providerMailStoredConfig struct {
	Enabled            *bool   `json:"enabled"`
	Host               *string `json:"host"`
	Port               *int    `json:"port"`
	Username           *string `json:"username"`
	FromEmail          *string `json:"from_email"`
	FromName           *string `json:"from_name"`
	ReplyTo            *string `json:"reply_to"`
	Encryption         *string `json:"encryption"`
	ForceAuthLogin     *bool   `json:"force_auth_login"`
	TimeoutSeconds     *int    `json:"timeout_seconds"`
	SMTPServer         *string `json:"smtp_server"`
	SMTPPort           *int    `json:"smtp_port"`
	SMTPAccount        *string `json:"smtp_account"`
	SMTPFrom           *string `json:"smtp_from"`
	SMTPSSLEnabled     *bool   `json:"smtp_ssl_enabled"`
	SSLEnabled         *bool   `json:"ssl_enabled"`
	SMTPForceAuthLogin *bool   `json:"smtp_force_auth_login"`
}

// providerMailCanonicalConfig is the only shape persisted after an update.
// Keeping one exact spelling per field prevents JSON key-order changes from
// altering the connection parameters after they have been validated.
type providerMailCanonicalConfig struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	FromEmail      string `json:"from_email"`
	FromName       string `json:"from_name"`
	ReplyTo        string `json:"reply_to"`
	Encryption     string `json:"encryption"`
	ForceAuthLogin bool   `json:"force_auth_login"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type providerMailPublicConfig struct {
	Enabled            bool   `json:"enabled"`
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	FromEmail          string `json:"from_email"`
	FromName           string `json:"from_name"`
	ReplyTo            string `json:"reply_to"`
	Encryption         string `json:"encryption"`
	ForceAuthLogin     bool   `json:"force_auth_login"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	PasswordConfigured bool   `json:"password_configured"`
}

type providerMailConnectionConfig struct {
	Host           string
	Port           int
	Username       string
	Encryption     string
	ForceAuthLogin bool
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func providerMailString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func providerMailBool(value *bool) bool {
	return value != nil && *value
}

func canonicalizeProviderMailConfig(config providerMailStoredConfig) providerMailCanonicalConfig {
	host := providerMailString(config.Host)
	if host == "" {
		host = providerMailString(config.SMTPServer)
	}
	username := providerMailString(config.Username)
	if username == "" {
		username = providerMailString(config.SMTPAccount)
	}
	port := 0
	if config.Port != nil {
		port = *config.Port
	}
	if port == 0 {
		if config.SMTPPort != nil {
			port = *config.SMTPPort
		}
	}
	if port == 0 {
		port = 587
	}
	encryption := providerMailString(config.Encryption)
	if encryption == "" {
		if providerMailBool(config.SMTPSSLEnabled) || providerMailBool(config.SSLEnabled) {
			encryption = "ssl"
		} else {
			encryption = "starttls"
		}
	}
	forceAuthLogin := false
	if config.ForceAuthLogin != nil {
		forceAuthLogin = *config.ForceAuthLogin
	} else if config.SMTPForceAuthLogin != nil {
		forceAuthLogin = *config.SMTPForceAuthLogin
	}
	timeoutSeconds := 0
	if config.TimeoutSeconds != nil {
		timeoutSeconds = *config.TimeoutSeconds
	}
	return providerMailCanonicalConfig{
		Enabled:        providerMailBool(config.Enabled),
		Host:           strings.TrimSpace(host),
		Port:           port,
		Username:       strings.TrimSpace(username),
		FromEmail:      firstNonEmpty(providerMailString(config.FromEmail), providerMailString(config.SMTPFrom)),
		FromName:       providerMailString(config.FromName),
		ReplyTo:        providerMailString(config.ReplyTo),
		Encryption:     encryption,
		ForceAuthLogin: forceAuthLogin,
		TimeoutSeconds: timeoutSeconds,
	}
}

func providerMailConnection(config providerMailCanonicalConfig) providerMailConnectionConfig {
	return providerMailConnectionConfig{
		Host:           config.Host,
		Port:           config.Port,
		Username:       config.Username,
		Encryption:     config.Encryption,
		ForceAuthLogin: config.ForceAuthLogin,
	}
}

// providerMailStoredConnection mirrors the exact canonical fields consumed by
// service.ProviderMail. Legacy aliases are useful for display/migration, but
// must not silently activate an old credential against a target the running
// mail sender did not previously use.
func providerMailStoredConnection(config providerMailStoredConfig) providerMailConnectionConfig {
	port := 0
	if config.Port != nil {
		port = *config.Port
	}
	forceAuthLogin := false
	if config.ForceAuthLogin != nil {
		forceAuthLogin = *config.ForceAuthLogin
	}
	return providerMailConnectionConfig{
		Host:           providerMailString(config.Host),
		Port:           port,
		Username:       providerMailString(config.Username),
		Encryption:     providerMailString(config.Encryption),
		ForceAuthLogin: forceAuthLogin,
	}
}

// redactProviderMailOption makes the SMTP password/app-password write-only for
// provider owners. Administrators continue to use the separate admin endpoint.
func redactProviderMailOption(option *model.ProviderOption) *model.ProviderOption {
	redacted := *option
	configMap := map[string]any{}
	stored := providerMailStoredConfig{}
	if err := common.UnmarshalJsonStr(option.Value, &configMap); err != nil {
		// Never return malformed raw SMTP data because it may itself contain a
		// credential. The provider can replace it by submitting a valid config.
		redacted.Value = `{"password_configured":false}`
		return &redacted
	}
	if err := common.UnmarshalJsonStr(option.Value, &stored); err != nil {
		redacted.Value = `{"password_configured":false}`
		return &redacted
	}
	canonical := canonicalizeProviderMailConfig(stored)
	publicConfig := providerMailPublicConfig{
		Enabled:            canonical.Enabled,
		Host:               canonical.Host,
		Port:               canonical.Port,
		Username:           canonical.Username,
		FromEmail:          canonical.FromEmail,
		FromName:           canonical.FromName,
		ReplyTo:            canonical.ReplyTo,
		Encryption:         canonical.Encryption,
		ForceAuthLogin:     canonical.ForceAuthLogin,
		TimeoutSeconds:     canonical.TimeoutSeconds,
		PasswordConfigured: providerMailSecret(configMap) != "",
	}
	data, err := common.Marshal(publicConfig)
	if err != nil {
		redacted.Value = `{"password_configured":false}`
		return &redacted
	}
	redacted.Value = string(data)
	return &redacted
}

// mergeProviderMailPassword preserves the stored SMTP password/app-password
// when the provider submits an empty password field, and replaces it only when
// a new non-empty value is submitted.
func mergeProviderMailPassword(providerId int, submitted string) (string, error) {
	config := map[string]any{}
	if err := common.UnmarshalJsonStr(submitted, &config); err != nil {
		return "", fmt.Errorf("邮箱配置格式无效: %w", err)
	}
	if config == nil {
		return "", errors.New("邮箱配置格式无效")
	}
	if err := validateProviderMailInputKeys(config); err != nil {
		return "", err
	}
	submittedConfig := providerMailStoredConfig{}
	if err := common.UnmarshalJsonStr(submitted, &submittedConfig); err != nil {
		return "", fmt.Errorf("邮箱配置格式无效: %w", err)
	}
	canonical := canonicalizeProviderMailConfig(submittedConfig)

	secret := providerMailSecret(config)
	if secret == "" {
		existing, err := model.GetProviderOptionValue(providerId, model.ProviderMailOptionKey)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		if err == nil && existing != "" {
			existingConfig := map[string]any{}
			if unmarshalErr := common.UnmarshalJsonStr(existing, &existingConfig); unmarshalErr != nil {
				return "", errors.New("现有邮箱配置格式无效，请输入新的 SMTP 密码 / 授权码")
			}
			if validationErr := validateProviderMailInputKeys(existingConfig); validationErr != nil {
				return "", errors.New("现有邮箱配置字段不规范，请输入新的 SMTP 密码 / 授权码")
			}
			// Only the exact canonical password field is consumed by the current
			// mail sender. A legacy smtp_token can be migrated when the caller
			// explicitly submits it, but is not silently reused from storage.
			secret, _ = existingConfig["password"].(string)
			if secret != "" {
				existingStoredConfig := providerMailStoredConfig{}
				if unmarshalErr := common.UnmarshalJsonStr(existing, &existingStoredConfig); unmarshalErr != nil {
					return "", errors.New("现有邮箱配置格式无效，请输入新的 SMTP 密码 / 授权码")
				}
				if providerMailStoredConnection(existingStoredConfig) != providerMailConnection(canonical) {
					return "", errors.New("修改 SMTP 主机、端口、账号、加密方式或认证方式时，必须输入新的 SMTP 密码 / 授权码")
				}
			}
		}
	}

	if canonical.Enabled && secret == "" {
		return "", errors.New("SMTP 密码 / 授权码不能为空")
	}
	canonical.Password = secret
	data, err := common.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return string(data), nil
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

func canManageProviderOptions(c *gin.Context, providerId int, adminRequest bool) bool {
	provider, err := model.GetProviderById(providerId)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if adminRequest && c.GetInt("role") >= common.RoleAdminUser {
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
	if !canManageProviderOptions(c, providerId, adminRequest) {
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
				if option.Key == model.ProviderMailOptionKey {
					visibleOptions = append(visibleOptions, redactProviderMailOption(option))
				} else {
					visibleOptions = append(visibleOptions, option)
				}
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
	if !canManageProviderOptions(c, providerId, adminRequest) {
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
	case model.ProviderMailOptionKey:
		merged, mergeErr := mergeProviderMailPassword(providerId, option.Value.(string))
		if mergeErr != nil {
			common.ApiErrorMsg(c, mergeErr.Error())
			return
		}
		option.Value = merged
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
