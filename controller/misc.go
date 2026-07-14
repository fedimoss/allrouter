package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func TestStatus(c *gin.Context) {
	err := model.PingDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "数据库连接失败",
		})
		return
	}
	// 获取HTTP统计信息
	httpStats := middleware.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Server is running",
		"http_stats": httpStats,
	})
	return
}

func GetStatus(c *gin.Context) {

	cs := console_setting.GetConsoleSetting()
	// 倒计时加载可能触发旧配置写回，必须在下方持有 OptionMap 读锁之前完成，避免锁升级死锁。
	providerId := resolveProviderId(c)
	topUpGiftTimed, err := model.LoadTopUpGiftTimedConfig(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	passkeySetting := system_setting.GetPasskeySettings()
	legalSetting := system_setting.GetLegalSettings()

	// 组装版本信息结构体
	versionInfo := gin.H{
		"version": "",
		"log":     "",
	}

	// 获取最新版本日志
	if latest, err := model.GetLatestVersionLogCached(); err == nil && latest != nil {
		versionInfo["version"] = latest.Version
		versionInfo["log"] = latest.Log
	}

	data := gin.H{
		"version":                     versionInfo,
		"start_time":                  common.StartTime,
		"email_verification":          common.EmailVerificationEnabled,
		"github_oauth":                common.GitHubOAuthEnabled,
		"github_client_id":            common.GitHubClientId,
		"discord_oauth":               system_setting.GetDiscordSettings().Enabled,
		"discord_client_id":           system_setting.GetDiscordSettings().ClientId,
		"linuxdo_oauth":               common.LinuxDOOAuthEnabled,
		"linuxdo_client_id":           common.LinuxDOClientId,
		"linuxdo_minimum_trust_level": common.LinuxDOMinimumTrustLevel,
		"telegram_oauth":              common.TelegramOAuthEnabled,
		"telegram_bot_name":           common.TelegramBotName,
		"theme":                       system_setting.GetThemeSettings().Frontend,
		"home_page_theme":             common.OptionMap["HomePageTheme"],
		"system_name":                 common.SystemName,
		"logo":                        common.Logo,
		"wechat_support":              common.WechatSupport,       // 微信客服
		"wechat_support_desc":         common.WechatSupportDesc,   // 微信客服文本描述
		"qq_support":                  common.QQSupport,           // QQ客服
		"qq_support_qrcode":           common.QQSupportQrcode,     // QQ客服二维码
		"telegram_support":            common.TelegramSupport,     // Telegram客服
		"telegram_support_desc":       common.TelegramSupportDesc, // Telegram客服文本描述
		"footer_html":                 common.Footer,
		"wechat_qrcode":               common.WeChatAccountQRCodeImageURL,
		"wechat_login":                common.WeChatAuthEnabled,
		"server_address":              system_setting.ServerAddress,
		"turnstile_check":             common.TurnstileCheckEnabled,
		"turnstile_site_key":          common.TurnstileSiteKey,
		"docs_link":                   operation_setting.GetGeneralSetting().DocsLink,
		"quota_per_unit":              common.QuotaPerUnit,
		// 兼容旧前端：保留 display_in_currency，同时提供新的 quota_display_type
		"display_in_currency":           operation_setting.IsCurrencyDisplay(),
		"quota_display_type":            operation_setting.GetQuotaDisplayType(),
		"custom_currency_symbol":        operation_setting.GetGeneralSetting().CustomCurrencySymbol,
		"custom_currency_exchange_rate": operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate,
		"enable_batch_update":           common.BatchUpdateEnabled,
		"enable_drawing":                common.DrawingEnabled,
		"enable_task":                   common.TaskEnabled,
		"enable_data_export":            common.DataExportEnabled,
		"data_export_default_time":      common.DataExportDefaultTime,
		"default_collapse_sidebar":      common.DefaultCollapseSidebar,
		"mj_notify_enabled":             setting.MjNotifyEnabled,
		"chats":                         setting.Chats,
		"demo_site_enabled":             operation_setting.DemoSiteEnabled,
		"self_use_mode_enabled":         operation_setting.SelfUseModeEnabled,
		"register_enabled":              common.RegisterEnabled,
		"password_register_enabled":     common.PasswordRegisterEnabled,
		"default_use_auto_group":        setting.DefaultUseAutoGroup,

		"usd_exchange_rate": operation_setting.USDExchangeRate,
		"price":             operation_setting.Price,
		"stripe_unit_price": setting.StripeUnitPrice,

		// 面板启用开关
		"api_info_enabled":      cs.ApiInfoEnabled,
		"uptime_kuma_enabled":   cs.UptimeKumaEnabled,
		"announcements_enabled": cs.AnnouncementsEnabled,
		"faq_enabled":           cs.FAQEnabled,

		// 模块管理配置
		"HeaderNavModules":    common.OptionMap["HeaderNavModules"],
		"SidebarModulesAdmin": common.OptionMap["SidebarModulesAdmin"],

		"oidc_enabled":                system_setting.GetOIDCSettings().Enabled,
		"oidc_client_id":              system_setting.GetOIDCSettings().ClientId,
		"oidc_authorization_endpoint": system_setting.GetOIDCSettings().AuthorizationEndpoint,
		"passkey_login":               passkeySetting.Enabled,
		"passkey_display_name":        passkeySetting.RPDisplayName,
		"passkey_rp_id":               passkeySetting.RPID,
		"passkey_origins":             passkeySetting.Origins,
		"passkey_allow_insecure":      passkeySetting.AllowInsecureOrigin,
		"passkey_user_verification":   passkeySetting.UserVerification,
		"passkey_attachment":          passkeySetting.AttachmentPreference,
		"setup":                       constant.Setup,
		"user_agreement_enabled":      legalSetting.UserAgreement != "",
		"privacy_policy_enabled":      legalSetting.PrivacyPolicy != "",
		"checkin_enabled":             getStatusCheckinEnabled(c),
		"topup_gift_timed":            topUpGiftTimed,
	}

	if providerId > 0 {
		primaryColor, secondaryColor := providerThemeColors(nil)
		data["primary_color"] = primaryColor
		data["secondary_color"] = secondaryColor
	}

	// 根据启用状态注入可选内容
	if v, ok := c.Get("provider_config"); ok {
		if cfg, ok := v.(model.ProviderConfig); ok {
			if cfg.SiteName != "" {
				data["system_name"] = cfg.SiteName
			}
			if cfg.Logo != "" {
				data["logo"] = cfg.Logo
			}
			if cfg.FooterText != "" {
				data["footer_html"] = cfg.FooterText
			}
			primaryColor, secondaryColor := providerThemeColors(&cfg)
			data["primary_color"] = primaryColor
			data["secondary_color"] = secondaryColor
			data["provider_config"] = providerConfigResponse(c, &cfg) // 提供商配置
		}
	}
	data["provider_id"] = providerId
	data["provider_enabled"] = providerId > 0

	if cs.ApiInfoEnabled {
		data["api_info"] = console_setting.GetApiInfo()
	}
	// 系统公告：主站展示全部公告；服务商站点仅接收主站明确下发的公告，并合并服务商自有公告。
	if cs.AnnouncementsEnabled {
		var announcements []map[string]interface{}
		if providerId > 0 {
			announcements = console_setting.GetAnnouncementsForProviderSites()
			if providerAnnouncements := model.GetProviderAnnouncements(providerId); len(providerAnnouncements) > 0 {
				announcements = append(announcements, providerAnnouncements...)
			}
		} else {
			announcements = console_setting.GetAnnouncements()
		}
		// 合并后统一按发布时间（publishDate）倒序排列，保证新旧公告混排而非分组展示
		sort.SliceStable(announcements, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, getStringField(announcements[i], "publishDate"))
			tj, _ := time.Parse(time.RFC3339, getStringField(announcements[j], "publishDate"))
			return ti.After(tj)
		})
		data["announcements"] = announcements
	}
	if cs.FAQEnabled {
		data["faq"] = console_setting.GetFAQ()
	}

	// Add enabled custom OAuth providers
	customProviders := oauth.GetEnabledCustomProviders()
	if len(customProviders) > 0 {
		type CustomOAuthInfo struct {
			Id                    int    `json:"id"`
			Name                  string `json:"name"`
			Slug                  string `json:"slug"`
			Icon                  string `json:"icon"`
			ClientId              string `json:"client_id"`
			AuthorizationEndpoint string `json:"authorization_endpoint"`
			Scopes                string `json:"scopes"`
		}
		providersInfo := make([]CustomOAuthInfo, 0, len(customProviders))
		for _, p := range customProviders {
			config := p.GetConfig()
			providersInfo = append(providersInfo, CustomOAuthInfo{
				Id:                    config.Id,
				Name:                  config.Name,
				Slug:                  config.Slug,
				Icon:                  config.Icon,
				ClientId:              config.ClientId,
				AuthorizationEndpoint: config.AuthorizationEndpoint,
				Scopes:                config.Scopes,
			})
		}
		data["custom_oauth_providers"] = providersInfo
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
	return
}

func GetNotice(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["Notice"],
	})
	return
}

func GetAbout(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["About"],
	})
	return
}

func GetUserAgreement(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    system_setting.GetLegalSettings().UserAgreement,
	})
	return
}

func GetPrivacyPolicy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    system_setting.GetLegalSettings().PrivacyPolicy,
	})
	return
}

func GetMidjourney(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["Midjourney"],
	})
	return
}

func GetHomePageContent(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    common.OptionMap["HomePageContent"],
	})
	return
}

func getRequestSystemName(c *gin.Context) string {
	if v, ok := c.Get("provider_config"); ok {
		if cfg, ok := v.(model.ProviderConfig); ok && strings.TrimSpace(cfg.SiteName) != "" {
			return strings.TrimSpace(cfg.SiteName)
		}
	}
	if v, ok := c.Get("provider_name"); ok {
		if name, ok := v.(string); ok && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	return common.SystemName
}

func getRequestBaseURLForEmail(c *gin.Context) string {
	providerDomain := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyProviderDomain))
	if providerDomain != "" {
		return common.GetTrustedRequestBaseURLWithDomains(c, system_setting.ServerAddress, []string{providerDomain})
	}
	return strings.TrimRight(system_setting.ServerAddress, "/")
}

func SendEmailVerification(c *gin.Context) {
	email := c.Query("email")
	if err := common.Validate.Var(email, "required,email"); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的邮箱地址",
		})
		return
	}
	localPart := parts[0]
	domainPart := parts[1]
	if common.EmailDomainRestrictionEnabled {
		allowed := false
		for _, domain := range common.EmailDomainWhitelist {
			if domainPart == domain {
				allowed = true
				break
			}
		}
		if !allowed {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "The administrator has enabled the email domain name whitelist, and your email address is not allowed due to special symbols or it's not in the whitelist.",
			})
			return
		}
	}
	if common.EmailAliasRestrictionEnabled {
		containsSpecialSymbols := strings.Contains(localPart, "+") || strings.Contains(localPart, ".")
		if containsSpecialSymbols {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "管理员已启用邮箱地址别名限制，您的邮箱地址由于包含特殊符号而被拒绝。",
			})
			return
		}
	}

	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	if model.IsEmailAlreadyTakenInProvider(providerId, email) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "邮箱地址已被占用",
		})
		return
	}
	code := common.GenerateVerificationCode(6)
	common.RegisterVerificationCodeWithKey(email, code, common.EmailVerificationPurpose)
	systemName := getRequestSystemName(c)
	subject := fmt.Sprintf("%s邮箱验证邮件 / Email Verification", systemName)
	content, err := common.RenderEmailTemplate("verification.html", map[string]any{
		"SystemName":   systemName,
		"Code":         code,
		"ValidMinutes": common.VerificationValidMinutes,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	err = service.SendProviderMail(providerId, subject, email, content)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func SendPasswordResetEmail(c *gin.Context) {
	email := c.Query("email")
	if err := common.Validate.Var(email, "required,email"); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	if model.IsEmailAlreadyTakenInProvider(providerId, email) {
		code := common.GenerateVerificationCode(0)
		common.RegisterVerificationCodeWithKey(email, code, common.PasswordResetPurpose)
		link := fmt.Sprintf("%s/user/reset?email=%s&token=%s", getRequestBaseURLForEmail(c), url.QueryEscape(email), url.QueryEscape(code))
		systemName := getRequestSystemName(c)
		subject := fmt.Sprintf("%s密码重置 / Password Reset", systemName)
		content, tmplErr := common.RenderEmailTemplate("password_reset.html", map[string]any{
			"SystemName":   systemName,
			"ResetLink":    link,
			"ValidMinutes": common.VerificationValidMinutes,
		})
		if tmplErr != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("failed to render password reset email template: %s", tmplErr.Error()))
		} else {
			err := service.SendProviderMail(providerId, subject, email, content)
			if err != nil {
				logger.LogError(c.Request.Context(), fmt.Sprintf("failed to send password reset email to %s: %s", email, err.Error()))
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type PasswordResetRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

func ResetPassword(c *gin.Context) {
	var req PasswordResetRequest
	err := json.NewDecoder(c.Request.Body).Decode(&req)
	if req.Email == "" || req.Token == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	if !common.VerifyCodeWithKey(req.Email, req.Token, common.PasswordResetPurpose) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "重置链接非法或已过期",
		})
		return
	}
	password := common.GenerateVerificationCode(12)
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	err = model.ResetUserPasswordByEmailInProvider(providerId, req.Email, password)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.DeleteKey(req.Email, common.PasswordResetPurpose)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    password,
	})
	return
}

// resolveProviderId 解析服务商ID
// resolveProviderId 解析当前请求对应的服务商ID，采用两级查找策略：
//  1. 域名解析：通过 TenantResolver 中间件，从请求域名匹配已认证的服务商域名，获取 providerId。
//     适用于用户通过服务商独立域名访问的场景，无需登录即可获取。
//  2. Session 回退：如果域名解析未命中（主域名访问），则从用户 Session 中读取已登录用户ID，
//     再依次检查该用户的 ProviderId 字段（通过服务商注册的用户）或 OwnerUserId 关联
//     （服务商所有者），获取对应的服务商ID。
//
// 返回：服务商ID；若用户无服务商关联则返回 0。
func resolveProviderId(c *gin.Context) int {
	// 第一级：从域名上下文获取（TenantResolver 中间件设置）
	if id := common.GetContextKeyInt(c, constant.ContextKeyProviderId); id > 0 {
		return id
	}
	// 第二级：从已登录用户的 Session 中获取用户关联的服务商ID
	session := sessions.Default(c)
	if userId, ok := session.Get("id").(int); ok && userId > 0 {
		// 检查用户是否属于某个服务商（通过服务商域名注册的用户）
		var user model.User
		if err := model.DB.Select("id", "provider_id").First(&user, userId).Error; err == nil && user.ProviderId > 0 {
			return user.ProviderId
		}
		// 检查用户是否为服务商所有者
		if provider, err := model.GetProviderByOwnerUserId(userId); err == nil {
			return provider.Id
		}
	}
	return 0
}

// getStringField 从 map[string]interface{} 中安全提取指定 key 的字符串值。
// 若 key 不存在或值类型不是 string，返回空字符串。
func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getStatusCheckinEnabled(c *gin.Context) bool {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	if providerId > 0 {
		cfg, err := model.GetProviderRewardConfig(providerId)
		if err == nil {
			return cfg.CheckinEnabled
		}
		return false
	}
	return operation_setting.GetCheckinSetting().Enabled
}
