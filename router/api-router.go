package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	// Import oauth package to register providers via init()
	_ "github.com/QuantumNous/new-api/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression))
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", anonymousRequestBodyLimit, controller.PostSetup)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/uptime/status", controller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), controller.DashboardListModels)
		apiRouter.GET("/status/test", middleware.AdminAuth(), controller.TestStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/user-agreement", controller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", controller.GetPrivacyPolicy)
		apiRouter.GET("/about", controller.GetAbout)
		//apiRouter.GET("/midjourney", controller.GetMidjourney)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.GET("/provider/public_config", controller.GetProviderPublicConfig)
		apiRouter.GET("/web_colors", controller.GetWebColors) // 获取网站主题色（无需登录）
		apiRouter.GET("/pricing", middleware.TryUserAuth(), controller.GetPricing)
		apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.ResetPassword)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.GET("/oauth/state", middleware.CriticalRateLimit(), controller.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.EmailBind)
		// Non-standard OAuth (WeChat, Telegram) - keep original routes
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), controller.WeChatAuth)
		apiRouter.POST("/oauth/wechat/bind", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.WeChatBind)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), controller.TelegramLogin)
		apiRouter.GET("/oauth/telegram/bind", middleware.CriticalRateLimit(), controller.TelegramBind)
		// Standard OAuth providers (GitHub, Discord, OIDC, LinuxDO) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), controller.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), controller.GetRatioConfig)

		apiRouter.POST("/stripe/webhook", anonymousRequestBodyLimit, controller.StripeWebhook)
		apiRouter.POST("/creem/webhook", anonymousRequestBodyLimit, controller.CreemWebhook)
		apiRouter.POST("/waffo/webhook", anonymousRequestBodyLimit, controller.WaffoWebhook)
		apiRouter.POST("/telegram/webhook", anonymousRequestBodyLimit, controller.TelegramWebhook)
		apiRouter.POST("/telegram/miniapp/bind", anonymousRequestBodyLimit, controller.TelegramMiniAppBind)

		// Universal secure verification routes
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.UniversalVerify)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/register", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Register)
			userRoute.POST("/login", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Login)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.Verify2FALogin)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), controller.TokenLog)
			userRoute.GET("/logout", controller.Logout)
			userRoute.POST("/epay/notify", anonymousRequestBodyLimit, controller.EpayNotify)
			userRoute.GET("/epay/notify", controller.EpayNotify)
			userRoute.POST("/lakala/notify", anonymousRequestBodyLimit, controller.LakalaNotify) // 拉卡拉支付结果回调
			userRoute.GET("/groups", controller.GetUserGroups)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.UserAuth())
			{
				selfRoute.GET("/self/groups", controller.GetUserGroups)
				selfRoute.GET("/self", controller.GetSelf) // 首页看板数据
				selfRoute.GET("/models", controller.GetUserModels)
				selfRoute.PUT("/self", controller.UpdateSelf)
				selfRoute.DELETE("/self", controller.DeleteSelf)
				selfRoute.GET("/token", controller.GenerateAccessToken)
				selfRoute.GET("/passkey", controller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", controller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", controller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", controller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", controller.PasskeyVerifyFinish)
				selfRoute.DELETE("/passkey", controller.PasskeyDelete)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.GET("/aff/records", controller.GetUserAffRecords)              // 管理员邀请记录列表
				selfRoute.GET("/self/aff/records", controller.GetSelfAffRecords)         // 用户邀请记录列表
				selfRoute.GET("/topup/rebate/records", controller.GetTopUpRebateRecords) // 用户返利记录列表
				selfRoute.GET("/topup/info", controller.GetTopUpInfo)
				selfRoute.GET("/topup/self", controller.GetUserTopUps)
				selfRoute.GET("/lakala/status", controller.GetLakalaTopUpStatus)
				selfRoute.GET("/redemption/self", controller.GetSelfRedemptionRecords)
				selfRoute.POST("/topup", middleware.CriticalRateLimit(), controller.TopUp)
				selfRoute.POST("/pay", middleware.CriticalRateLimit(), controller.RequestEpay)
				selfRoute.POST("/amount", controller.RequestAmount)
				selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.RequestStripePay)
				selfRoute.POST("/stripe/amount", controller.RequestStripeAmount)
				selfRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.RequestCreemPay)
				selfRoute.POST("/waffo/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPay)
				selfRoute.POST("/crypto/pay", middleware.CriticalRateLimit(), controller.RequestCryptoPay) // 加密货币充值下单
				selfRoute.POST("/crypto/confirm", controller.RequestCryptoConfirm)                         // 加密货币充值确认
				selfRoute.POST("/aff_transfer", controller.TransferAffQuota)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)
				selfRoute.POST("/avatar", controller.UploadAvatar) // 上传头像

				// 2FA routes
				selfRoute.GET("/2fa/status", controller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", controller.Setup2FA)
				selfRoute.POST("/2fa/enable", controller.Enable2FA)
				selfRoute.POST("/2fa/disable", controller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", controller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", controller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), controller.DoCheckin)

				// Custom OAuth bindings
				selfRoute.GET("/oauth/bindings", controller.GetUserOAuthBindings)
				selfRoute.DELETE("/oauth/bindings/:provider_id", controller.UnbindCustomOAuth)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", controller.GetAllUsers)
				adminRoute.GET("/topup", controller.GetAllTopUps)
				adminRoute.POST("/topup/detail", controller.GetUserTopupDetails)
				adminRoute.POST("/topup/complete", controller.AdminCompleteTopUp)
				adminRoute.GET("/search", controller.SearchUsers)
				adminRoute.GET("/:id/oauth/bindings", controller.GetUserOAuthBindingsByAdmin)
				adminRoute.DELETE("/:id/oauth/bindings/:provider_id", controller.UnbindCustomOAuthByAdmin)
				adminRoute.DELETE("/:id/bindings/:binding_type", controller.AdminClearUserBinding)
				adminRoute.GET("/:id/invitees", controller.GetUserInvitees)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
				adminRoute.DELETE("/:id/reset_passkey", controller.AdminResetPasskey)

				// Admin 2FA routes
				adminRoute.GET("/2fa/stats", controller.Admin2FAStats)
				adminRoute.DELETE("/:id/2fa", controller.AdminDisable2FA)
			}
		}

		// Subscription billing (plans, purchase, admin management)
		subscriptionRoute := apiRouter.Group("/subscription")
		subscriptionRoute.Use(middleware.UserAuth())
		{
			subscriptionRoute.GET("/plans", controller.GetSubscriptionPlans)
			subscriptionRoute.GET("/self", controller.GetSubscriptionSelf)
			subscriptionRoute.PUT("/self/preference", controller.UpdateSubscriptionPreference)
			subscriptionRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestEpay)
			subscriptionRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestStripePay)
			subscriptionRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestCreemPay)
			subscriptionRoute.POST("/crypto/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestCryptoPay) // 加密货币订阅
			subscriptionRoute.POST("/crypto/confirm", controller.SubscriptionRequestCryptoConfirm)                         // 加密货币订阅确认
			subscriptionRoute.GET("/lakala/status", controller.GetSubscriptionLakalaStatus)                                // 拉卡拉订阅订单状态轮询
		}
		subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
		subscriptionAdminRoute.Use(middleware.AdminAuth())
		{
			subscriptionAdminRoute.GET("/plans", controller.AdminListSubscriptionPlans)
			subscriptionAdminRoute.POST("/plans", controller.AdminCreateSubscriptionPlan)
			subscriptionAdminRoute.PUT("/plans/:id", controller.AdminUpdateSubscriptionPlan)
			subscriptionAdminRoute.PATCH("/plans/:id", controller.AdminUpdateSubscriptionPlanStatus)
			subscriptionAdminRoute.POST("/bind", controller.AdminBindSubscription)

			// User subscription management (admin)
			subscriptionAdminRoute.GET("/users/:id/subscriptions", controller.AdminListUserSubscriptions)
			subscriptionAdminRoute.POST("/users/:id/subscriptions", controller.AdminCreateUserSubscription)
			subscriptionAdminRoute.POST("/user_subscriptions/:id/invalidate", controller.AdminInvalidateUserSubscription)
			subscriptionAdminRoute.DELETE("/user_subscriptions/:id", controller.AdminDeleteUserSubscription)
		}

		// Subscription payment callbacks (no auth)
		apiRouter.POST("/subscription/epay/notify", anonymousRequestBodyLimit, controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/return", controller.SubscriptionEpayReturn)
		apiRouter.POST("/subscription/epay/return", anonymousRequestBodyLimit, controller.SubscriptionEpayReturn)
		apiRouter.POST("/subscription/lakala/notify", anonymousRequestBodyLimit, controller.SubscriptionLakalaNotify) // 拉卡拉订阅支付结果回调
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.GET("/channel_affinity_cache", controller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", controller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", controller.ResetModelRatio)
			optionRoute.POST("/migrate_console_setting", controller.MigrateConsoleSetting)      // 用于迁移检测的旧键，下个版本会删除
			optionRoute.POST("/web_logo", controller.UploadWebLogo)                             // 上传网站logo
			optionRoute.POST("/web_colors", controller.SetWebColors)                            // 设置网站主色和辅色
			optionRoute.POST("/update_crypto_chain_config", controller.UpdateCryptoChainConfig) // 更新加密货币链配置
			optionRoute.POST("/update_crypto_rate", controller.UpdateCryptoRate)                // 更新加密货币汇率
			optionRoute.POST("/web_support", controller.SetWebSupport)                          // 设置网站客服
			optionRoute.POST("/add_version_log", controller.AddVersionLog)                      // 添加版本日志
			optionRoute.GET("/get_version_log", controller.GetAllVersionLog)                    // 获取所有版本日志
			optionRoute.GET("/get_version_log/:id", controller.GetVersionLogById)               // 获取版本日志详情
			optionRoute.PUT("/update_version_log/:id", controller.UpdateVersionLogById)         // 更新版本日志详情
			optionRoute.DELETE("/delete_version_log/:id", controller.DeleteVersionLogById)      // 删除版本日志
		}

		// Telegram webhook 管理接口（管理员）
		telegramAdminRoute := apiRouter.Group("/telegram/admin")
		telegramAdminRoute.Use(middleware.RootAuth())
		{
			telegramAdminRoute.POST("/setWebhook", controller.SetTelegramWebhook)
			telegramAdminRoute.POST("/deleteWebhook", controller.DeleteTelegramWebhook)
			telegramAdminRoute.POST("/getWebhookInfo", controller.GetTelegramWebhookInfo)
		}

		// 加密货币公开读取接口（普通用户登录即可调用）
		optionReadRoute := apiRouter.Group("/option")
		optionReadRoute.Use(middleware.UserAuth())
		{
			optionReadRoute.GET("/get_crypto_chain_config", controller.GetCryptoChainConfig) // 获取加密货币链配置
			optionReadRoute.GET("/get_crypto_rate", controller.GetCryptoRate)                // 获取加密货币汇率
			optionReadRoute.POST("/wechat_qrcode", controller.UploadWechatCustomerQrcode)    // 上传微信客服二维码
		}

		// 币种 Stripe 价格配置（管理后台）
		currencyStripeRoute := apiRouter.Group("/currency-stripe-config")
		currencyStripeRoute.Use(middleware.RootAuth())
		{
			currencyStripeRoute.GET("/", controller.GetAdminCurrencyStripeConfigs)
			currencyStripeRoute.PUT("/", controller.UpdateAdminCurrencyStripeConfig)
		}

		// Custom OAuth provider management (root only)
		customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
		customOAuthRoute.Use(middleware.RootAuth())
		{
			customOAuthRoute.POST("/discovery", controller.FetchCustomOAuthDiscovery)
			customOAuthRoute.GET("/", controller.GetCustomOAuthProviders)
			customOAuthRoute.GET("/:id", controller.GetCustomOAuthProvider)
			customOAuthRoute.POST("/", controller.CreateCustomOAuthProvider)
			customOAuthRoute.PUT("/:id", controller.UpdateCustomOAuthProvider)
			customOAuthRoute.DELETE("/:id", controller.DeleteCustomOAuthProvider)
		}
		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", controller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", controller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", controller.ResetPerformanceStats)
			performanceRoute.POST("/gc", controller.ForceGC)
			performanceRoute.GET("/logs", controller.GetLogFiles)
			performanceRoute.DELETE("/logs", controller.CleanupLogFiles)
		}
		ratioSyncRoute := apiRouter.Group("/ratio_sync")
		ratioSyncRoute.Use(middleware.RootAuth())
		{
			ratioSyncRoute.GET("/channels", controller.GetSyncableChannels)
			ratioSyncRoute.POST("/fetch", controller.FetchUpstreamRatios)
		}
		channelRoute := apiRouter.Group("/channel")
		channelRoute.Use(middleware.AdminAuth())
		{
			channelRoute.GET("/", controller.GetAllChannels)
			channelRoute.GET("/search", controller.SearchChannels)
			channelRoute.GET("/models", controller.ChannelListModels)
			channelRoute.GET("/models_enabled", controller.EnabledListModels)
			channelRoute.GET("/:id", controller.GetChannel)
			channelRoute.POST("/:id/key", middleware.RootAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.SecureVerificationRequired(), controller.GetChannelKey)
			channelRoute.GET("/test", controller.TestAllChannels)
			channelRoute.GET("/test/:id", controller.TestChannel)
			channelRoute.GET("/update_balance", controller.UpdateAllChannelsBalance)
			channelRoute.GET("/update_balance/:id", controller.UpdateChannelBalance)
			channelRoute.POST("/", controller.AddChannel)
			channelRoute.PUT("/", controller.UpdateChannel)
			channelRoute.DELETE("/disabled", controller.DeleteDisabledChannel)
			channelRoute.POST("/tag/disabled", controller.DisableTagChannels)
			channelRoute.POST("/tag/enabled", controller.EnableTagChannels)
			channelRoute.PUT("/tag", controller.EditTagChannels)
			channelRoute.DELETE("/:id", controller.DeleteChannel)
			channelRoute.POST("/batch", controller.DeleteChannelBatch)
			channelRoute.POST("/fix", controller.FixChannelsAbilities)
			channelRoute.GET("/fetch_models/:id", controller.FetchUpstreamModels)
			channelRoute.POST("/fetch_models", middleware.RootAuth(), controller.FetchModels)
			channelRoute.POST("/codex/oauth/start", controller.StartCodexOAuth)
			channelRoute.POST("/codex/oauth/complete", controller.CompleteCodexOAuth)
			channelRoute.POST("/:id/codex/oauth/start", controller.StartCodexOAuthForChannel)
			channelRoute.POST("/:id/codex/oauth/complete", controller.CompleteCodexOAuthForChannel)
			channelRoute.POST("/:id/codex/refresh", controller.RefreshCodexChannelCredential)
			channelRoute.GET("/:id/codex/usage", controller.GetCodexChannelUsage)
			channelRoute.POST("/ollama/pull", controller.OllamaPullModel)
			channelRoute.POST("/ollama/pull/stream", controller.OllamaPullModelStream)
			channelRoute.DELETE("/ollama/delete", controller.OllamaDeleteModel)
			channelRoute.GET("/ollama/version/:id", controller.OllamaVersion)
			channelRoute.POST("/batch/tag", controller.BatchSetChannelTag)
			channelRoute.GET("/tag/models", controller.GetTagModels)
			channelRoute.POST("/copy/:id", controller.CopyChannel)
			channelRoute.POST("/multi_key/manage", controller.ManageMultiKeys)
			channelRoute.POST("/upstream_updates/apply", controller.ApplyChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/apply_all", controller.ApplyAllChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect", controller.DetectChannelUpstreamModelUpdates)
			channelRoute.POST("/upstream_updates/detect_all", controller.DetectAllChannelUpstreamModelUpdates)
		}
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), controller.SearchTokens)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKey)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
			tokenRoute.POST("/batch", controller.DeleteTokenBatch)
			tokenRoute.POST("/batch/keys", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKeysBatch)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
			{
				tokenUsageRoute.GET("/", controller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", controller.GetAllRedemptions)
			redemptionRoute.GET("/search", controller.SearchRedemptions)
			redemptionRoute.GET("/:id", controller.GetRedemption)
			redemptionRoute.POST("/", controller.AddRedemption)
			redemptionRoute.PUT("/", controller.UpdateRedemption)
			redemptionRoute.DELETE("/invalid", controller.DeleteInvalidRedemption)
			redemptionRoute.DELETE("/:id", controller.DeleteRedemption)
		}
		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), controller.GetAllLogs) // 使用日志(管理员)
		logRoute.DELETE("/", middleware.AdminAuth(), controller.DeleteHistoryLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), controller.GetLogsStat)
		logRoute.GET("/calls", middleware.AdminAuth(), controller.GetAdminCallLogs)
		logRoute.GET("/calls/stat", middleware.AdminAuth(), controller.GetAdminCallLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), controller.GetChannelAffinityUsageCacheStats)
		logRoute.GET("/search", middleware.AdminAuth(), controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs) // 使用日志(用户)
		logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), controller.SearchUserLogs)

		dataRoute := apiRouter.Group("/data")
		dataRoute.GET("/users", middleware.AdminAuth(), controller.GetQuotaDatesByUser)
		dataRoute.GET("/self", middleware.UserAuth(), controller.GetUserQuotaDates)                        // 消耗与请求趋势(用户)
		dataRoute.GET("/", middleware.AdminAuth(), controller.GetAllQuotaDates)                            // 消耗与请求趋势(管理员)
		dataRoute.GET("/modelPopularRank", middleware.AdminAuth(), controller.GetAllModelPopularRank)      // 模型热度排行(管理员)
		dataRoute.GET("/self/modelPopularRank", middleware.UserAuth(), controller.GetUserModelPopularRank) // 模型热度排行(用户)
		dataRoute.GET("/modelQuotaRadio", middleware.AdminAuth(), controller.GetAllModelQuotaRadio)        // 模型额度占比(管理员)
		dataRoute.GET("/self/modelQuotaRadio", middleware.UserAuth(), controller.GetUserModelQuotaRadio)   // 模型额度占比(用户)

		billRoute := apiRouter.Group("/bill")
		billRoute.GET("/", middleware.AdminAuth(), controller.GetAllBill)     // 账单中心数据概览
		billRoute.GET("/self", middleware.UserAuth(), controller.GetSelfBill) // 账单中心数据概览

		billRoute.GET("/distributor", middleware.UserAuth(), controller.GetDistributorBill) // 账单中心数据概览(分销商)

		// 运营中心接口
		providerRoute := apiRouter.Group("/provider")
		providerRoute.Use(middleware.UserAuth())
		{
			providerRoute.GET("/self", controller.GetProviderSelf)
			providerRoute.PUT("/self", controller.UpdateProviderSelf)
			providerRoute.PUT("/domains", controller.SaveProviderSelfDomains)
			providerRoute.POST("/domains", controller.CreateProviderSelfDomain)
			providerRoute.PUT("/domains/:domain_id", controller.UpdateProviderSelfDomain)
			providerRoute.DELETE("/domains/:domain_id", controller.DeleteProviderSelfDomain)
			providerRoute.GET("/config", controller.GetProviderSelfConfig)
			providerRoute.POST("/logo", controller.UploadProviderLogo)
			providerRoute.PUT("/config", controller.UpsertProviderSelfConfig)
			providerRoute.GET("/reward/config", controller.GetProviderRewardConfig)
			providerRoute.PUT("/reward/config", controller.UpsertProviderRewardConfig)
			providerRoute.GET("/reward/summary", controller.GetProviderRewardSummary)
			providerRoute.GET("/redemption", controller.GetProviderRedemptions)
			providerRoute.GET("/redemption/search", controller.SearchProviderRedemptions)
			providerRoute.POST("/redemption", controller.AddProviderRedemption)
			providerRoute.PUT("/redemption", controller.UpdateProviderRedemption)
			providerRoute.DELETE("/redemption/invalid", controller.DeleteInvalidProviderRedemption)
			providerRoute.GET("/redemption/:id", controller.GetProviderRedemption)
			providerRoute.DELETE("/redemption/:id", controller.DeleteProviderRedemption)
			providerRoute.GET("/profits", controller.GetProviderProfits)
			providerRoute.GET("/logs", controller.GetProviderUserLogs)
			providerRoute.GET("/logs/stat", controller.GetProviderUserLogsStat)
			providerRoute.GET("/users", controller.GetProviderUsers)
			providerRoute.GET("/tree/users", controller.GetTreeProviderUsers) //服务商用户管理---tree型结构
			providerRoute.GET("/users/search", controller.SearchProviderUsers)
			providerRoute.GET("/users/:id/invitees", controller.GetProviderUserInvitees)
			providerRoute.GET("/users/:id", controller.GetProviderUser)
			providerRoute.POST("/users", controller.CreateProviderUser)
			providerRoute.PUT("/users", controller.UpdateProviderUser)
			providerRoute.POST("/users/manage", controller.ManageProviderUser)
			providerRoute.DELETE("/users/:id", controller.DeleteProviderUser)
			providerRoute.GET("/base_models", controller.ListProviderBaseModels)
			providerRoute.GET("/model_pricing", controller.ListProviderModelPricing)
			providerRoute.POST("/model_pricing", controller.UpsertProviderModelPricing)
			providerRoute.PUT("/model_pricing", controller.UpsertProviderModelPricing)
			providerRoute.DELETE("/model_pricing/:id", controller.DeleteProviderModelPricing)
			providerRoute.POST("/withdraw/request", controller.AddProviderWithdrawRequest)    // 添加提现申请
			providerRoute.GET("/withdraw/list", controller.GetProviderWithdrawList)           // 提现申请列表
			providerRoute.GET("/withdraw/dashboard", controller.GetProviderWithdrawDashboard) // 提现申请数据概览
			providerRoute.POST("/withdraw/cancel", controller.CancelProviderWithdrawRequest)  // 取消提现申请
			providerRoute.GET("/options/:id", controller.GetProviderOptions)                  // 获取服务商配置
			providerRoute.PUT("/options/:id", controller.UpdateProviderOption)                // 更新服务商配置
		}
		providerAdminRoute := apiRouter.Group("/provider/admin") //服务商管理
		providerAdminRoute.Use(middleware.AdminAuth())
		{
			providerAdminRoute.GET("", controller.AdminListProviders)
			providerAdminRoute.GET("/", controller.AdminListProviders)
			providerAdminRoute.GET("/owner_candidates", controller.AdminListProviderOwnerCandidates)
			providerAdminRoute.POST("", controller.AdminCreateProvider)
			providerAdminRoute.POST("/", controller.AdminCreateProvider)
			providerAdminRoute.PUT("/:id", controller.AdminUpdateProvider)
			providerAdminRoute.DELETE("/:id", controller.AdminDisableProvider)
			providerAdminRoute.DELETE("/:id/permanent", controller.AdminDeleteProvider)
			providerAdminRoute.PUT("/:id/enable", controller.AdminEnableProvider)
			providerAdminRoute.POST("/logo", controller.AdminUploadProviderLogo)
			providerAdminRoute.PUT("/:id/config", controller.AdminUpsertProviderConfig)
			providerAdminRoute.PUT("/:id/nav_modules", controller.AdminUpdateProviderNavModules)
			providerAdminRoute.GET("/:id/reward/config", controller.AdminGetProviderRewardConfig)
			providerAdminRoute.PUT("/:id/reward/config", controller.AdminUpsertProviderRewardConfig)
			providerAdminRoute.GET("/:id/reward/summary", controller.AdminGetProviderRewardSummary)
			providerAdminRoute.GET("/:id/profits", controller.AdminGetProviderProfits)
			providerAdminRoute.PUT("/:id/domains", controller.AdminSaveProviderDomains)
			providerAdminRoute.POST("/:id/domains", controller.AdminCreateProviderDomain)
			providerAdminRoute.PUT("/:id/domains/:domain_id", controller.AdminUpdateProviderDomain)
			providerAdminRoute.DELETE("/:id/domains/:domain_id", controller.AdminDeleteProviderDomain)
			providerAdminRoute.GET("/base_models", controller.AdminListProviderBaseModels)
			providerAdminRoute.GET("/:id/model_pricing", controller.AdminListProviderModelPricing)
			providerAdminRoute.POST("/:id/model_pricing", controller.AdminUpsertProviderModelPricing)
			providerAdminRoute.PUT("/:id/model_pricing", controller.AdminUpsertProviderModelPricing)
			providerAdminRoute.DELETE("/:id/model_pricing/:pricing_id", controller.AdminDeleteProviderModelPricing)
			providerAdminRoute.GET("/withdraw/list", controller.AdminGetProviderWithdrawList)            // 提现申请列表
			providerAdminRoute.GET("/withdraw/dashboard", controller.AdminGetProviderWithdrawDashboard)  // 提现申请数据概览
			providerAdminRoute.POST("/withdraw/approve", controller.AdminApproveProviderWithdrawRequest) // 提现申请审核
		}

		operationRoute := apiRouter.Group("/operation")
		operationRoute.Use(middleware.AdminAuth())
		{
			// 看板数据接口
			operationRoute.GET("/user/dashboard", controller.GetUserOperationDashboard)               // 用户
			operationRoute.GET("/distributor/dashboard", controller.GetDistributorOperationDashboard) // 代理商
			operationRoute.GET("/merchant/dashboard", controller.GetMerchantOperationDashboard)       // 商家
			operationRoute.GET("/platform/dashboard", controller.GetPlatformOperationDashboard)       // 平台
			// 列表数据接口
			operationRoute.GET("/user/records", controller.GetUserOperationRecords)               // 用户
			operationRoute.GET("/distributor/records", controller.GetDistributorOperationRecords) // 代理商
			operationRoute.GET("/merchant/records", controller.GetMerchantOperationRecords)       // 商家
			operationRoute.GET("/platform/records", controller.GetPlatformOperationRecords)       // 平台
		}

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), controller.GetLogByKey)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		{
			prefillGroupRoute.GET("/", controller.GetPrefillGroups)
			prefillGroupRoute.POST("/", controller.CreatePrefillGroup)
			prefillGroupRoute.PUT("/", controller.UpdatePrefillGroup)
			prefillGroupRoute.DELETE("/:id", controller.DeletePrefillGroup)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		mjRoute.GET("/", middleware.AdminAuth(), controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		{
			taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserTask)
			taskRoute.GET("/", middleware.AdminAuth(), controller.GetAllTask)
		}

		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		{
			vendorRoute.GET("/", controller.GetAllVendors)
			vendorRoute.GET("/search", controller.SearchVendors)
			vendorRoute.GET("/:id", controller.GetVendorMeta)
			vendorRoute.POST("/", controller.CreateVendorMeta)
			vendorRoute.PUT("/", controller.UpdateVendorMeta)
			vendorRoute.DELETE("/:id", controller.DeleteVendorMeta)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		{
			modelsRoute.GET("/sync_upstream/preview", controller.SyncUpstreamPreview)
			modelsRoute.POST("/sync_upstream", controller.SyncUpstreamModels)
			modelsRoute.GET("/missing", controller.GetMissingModels)
			modelsRoute.GET("/", controller.GetAllModelsMeta)
			modelsRoute.GET("/search", controller.SearchModelsMeta)
			modelsRoute.POST("/translate", controller.TranslateModelContent)
			modelsRoute.GET("/:id", controller.GetModelMeta)
			modelsRoute.POST("/", controller.CreateModelMeta)
			modelsRoute.PUT("/", controller.UpdateModelMeta)
			modelsRoute.DELETE("/:id", controller.DeleteModelMeta)
		}

		// Deployments (model deployment management)
		deploymentsRoute := apiRouter.Group("/deployments")
		deploymentsRoute.Use(middleware.AdminAuth())
		{
			deploymentsRoute.GET("/settings", controller.GetModelDeploymentSettings)
			deploymentsRoute.POST("/settings/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/", controller.GetAllDeployments)
			deploymentsRoute.GET("/search", controller.SearchDeployments)
			deploymentsRoute.POST("/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/hardware-types", controller.GetHardwareTypes)
			deploymentsRoute.GET("/locations", controller.GetLocations)
			deploymentsRoute.GET("/available-replicas", controller.GetAvailableReplicas)
			deploymentsRoute.POST("/price-estimation", controller.GetPriceEstimation)
			deploymentsRoute.GET("/check-name", controller.CheckClusterNameAvailability)
			deploymentsRoute.POST("/", controller.CreateDeployment)

			deploymentsRoute.GET("/:id", controller.GetDeployment)
			deploymentsRoute.GET("/:id/logs", controller.GetDeploymentLogs)
			deploymentsRoute.GET("/:id/containers", controller.ListDeploymentContainers)
			deploymentsRoute.GET("/:id/containers/:container_id", controller.GetContainerDetails)
			deploymentsRoute.PUT("/:id", controller.UpdateDeployment)
			deploymentsRoute.PUT("/:id/name", controller.UpdateDeploymentName)
			deploymentsRoute.POST("/:id/extend", controller.ExtendDeployment)
			deploymentsRoute.DELETE("/:id", controller.DeleteDeployment)
		}

		// 支付对账
		wechatTradeBillRoute := apiRouter.Group("/wechat_trade_bill")
		wechatTradeBillRoute.Use(middleware.AdminAuth())
		{
			wechatTradeBillRoute.GET("/stat", controller.GetWechatTradeBillStat)
			wechatTradeBillRoute.GET("/list", controller.GetWechatTradeBillList)
			wechatTradeBillRoute.GET("/:id", controller.GetWechatTradeBillDetail)
			wechatTradeBillRoute.POST("/run", controller.RunWechatTradeBill)
		}
		//CLI Proxy API接口集成进allrouter中
		voRoute := apiRouter.Group("/v0")
		voRoute.Use(middleware.UserAuth())
		{
			//
			managementRoute := voRoute.Group("/management")
			//managementRoute.Use(middleware.CriticalRateLimit()) //关键接口限流中间件”，用来防刷、防爆破
			managementRoute.GET("/qwen-auth-url", controller.GetQwenAuthUrl)
			managementRoute.GET("/codex-auth-url", controller.GetCodexAuthUrl)
			managementRoute.GET("/anthropic-auth-url", controller.GetAnthropicAuthUrl)
			managementRoute.GET("/antigravity-auth-url", controller.GetAntigravityAuthUrl)
			managementRoute.GET("/gemini-cli-auth-url", middleware.CriticalRateLimit(), controller.GetGeminiCliAuthUrl)
			managementRoute.GET("/kimi-auth-url", controller.GetKimiAuthUrl)
			managementRoute.POST("/iflow-auth-url", controller.IflowAuth)
			managementRoute.POST("/oauth-callback", controller.OAuthCallBack)
			managementRoute.GET("/get-auth-status", controller.GetAuthStatus)
			managementRoute.GET("/useroauths", middleware.CriticalRateLimit(), controller.GetUserOAuths)
			managementRoute.DELETE("/oauthDelete/:id", middleware.CriticalRateLimit(), controller.DeleteOAuth)
			managementRoute.GET("/downloadoauth", middleware.CriticalRateLimit(), controller.DownloadOauth)
			managementRoute.POST("/auth-files/status", middleware.CriticalRateLimit(), controller.UpdateAuthFileStatus)
			managementRoute.GET("/get-oauth-success-count", middleware.CriticalRateLimit(), controller.GetUserAuthSuccessCount)
		}

	}
}
