package common

import (
	"crypto/tls"
	//"os"
	//"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var StartTime = time.Now().Unix() // unit: second
var Version = "v0.0.0"            // this hard coding will be replaced automatically when building, no need to manually change
var SystemName = "All Router"
var Footer = ""
var Logo = ""
var WechatSupport = ""       // 微信客服
var WechatSupportDesc = ""   // 微信客服文本描述
var QQSupport = ""           // QQ客服
var QQSupportQrcode = ""     // QQ客服二维码
var TelegramSupport = ""     // Telegram客服
var TelegramSupportDesc = "" // Telegram客服文本描述
var TopUpLink = ""

var themeValue atomic.Value // stores string; safe for concurrent read/write

func init() {
	themeValue.Store("classic")
}

func GetTheme() string {
	return themeValue.Load().(string)
}

// SetTheme updates the frontend theme atomically.
// Only "default" and "classic" are accepted; other values are silently ignored.
func SetTheme(t string) {
	if t == "default" || t == "classic" {
		themeValue.Store(t)
	}
}

// ThemeAwarePath rewrites legacy /console/* paths to the default-theme
// equivalents when the active theme is "default".  For "classic" (or any
// other theme) the path is returned unchanged.  The function only touches
// known prefixes so it is safe to call with arbitrary suffixes and query
// strings.
func ThemeAwarePath(suffix string) string {
	if GetTheme() != "default" {
		return suffix
	}
	switch {
	case strings.HasPrefix(suffix, "/console/topup"):
		return strings.Replace(suffix, "/console/topup", "/wallet", 1)
	case strings.HasPrefix(suffix, "/console/log"):
		return strings.Replace(suffix, "/console/log", "/usage-logs", 1)
	case strings.HasPrefix(suffix, "/console/personal"):
		return strings.Replace(suffix, "/console/personal", "/profile", 1)
	}
	return suffix
}

// var ChatLink = ""
// var ChatLink2 = ""
var QuotaPerUnit = 500 * 1000.0 // $0.002 / 1K tokens
// 保留旧变量以兼容历史逻辑，实际展示由 general_setting.quota_display_type 控制
var DisplayInCurrencyEnabled = true
var DisplayTokenStatEnabled = true
var DrawingEnabled = true
var TaskEnabled = true
var DataExportEnabled = true
var DataExportInterval = 5         // unit: minute
var DataExportDefaultTime = "hour" // unit: minute
var DefaultCollapseSidebar = false // default value of collapse sidebar

// Any options with "Secret", "Token" in its key won't be return by GetOptions

var SessionSecret = uuid.New().String()
var CryptoSecret = uuid.New().String()
var SessionCookieSecure = false
var SessionCookieTrustedURLs []string

var OptionMap map[string]string
var OptionMapRWMutex sync.RWMutex

var ItemsPerPage = 10
var MaxRecentItems = 1000

var PasswordLoginEnabled = true
var PasswordRegisterEnabled = true
var EmailVerificationEnabled = false
var GitHubOAuthEnabled = false
var LinuxDOOAuthEnabled = false
var WeChatAuthEnabled = false
var TelegramOAuthEnabled = false
var TurnstileCheckEnabled = false
var RegisterEnabled = true

var EmailDomainRestrictionEnabled = false // 是否启用邮箱域名限制
var EmailAliasRestrictionEnabled = false  // 是否启用邮箱别名限制
var EmailDomainWhitelist = []string{
	"gmail.com",
	"163.com",
	"126.com",
	"qq.com",
	"outlook.com",
	"hotmail.com",
	"icloud.com",
	"yahoo.com",
	"foxmail.com",
}
var EmailLoginAuthServerList = []string{
	"smtp.sendcloud.net",
	"smtp.azurecomm.net",
}

var DebugEnabled bool
var MemoryCacheEnabled bool

var LogConsumeEnabled = true

var TLSInsecureSkipVerify bool
var InsecureTLSConfig = &tls.Config{InsecureSkipVerify: true}

var SMTPServer = ""
var SMTPPort = 587
var SMTPSSLEnabled = false
var SMTPForceAuthLogin = false
var SMTPAccount = ""
var SMTPFrom = ""
var SMTPToken = ""

var GitHubClientId = ""
var GitHubClientSecret = ""
var LinuxDOClientId = ""
var LinuxDOClientSecret = ""
var LinuxDOMinimumTrustLevel = 0

var WeChatServerAddress = ""
var WeChatServerToken = ""
var WeChatAccountQRCodeImageURL = ""

var TurnstileSiteKey = ""
var TurnstileSecretKey = ""

var TelegramBotToken = ""
var TelegramBotName = ""
var TelegramWebhookSecret = "" // Telegram webhook 的 secret_token（与 setWebhook 时传的保持一致）；为空则不校验
var TelegramMiniAppURL = ""    // Mini App 启动链接（@BotFather /newapp 生成的 t.me/<bot>/<appname>），群里"绑定账号"按钮指向它；为空则不发送

var QuotaForNewUser = 0

// RegisterGiftSubscriptionPlanId 新用户注册时自动赠送的订阅套餐 ID。
// 在运营设置中配置，0 表示不赠送。用户注册事务中调用 grantRegisterGiftSubscriptionTx 实现。
var RegisterGiftSubscriptionPlanId = 0

// AirdropSubscriptionPlanId 管理员空投订阅时使用的全局套餐 ID。
// 在运营设置中配置，0 表示未配置。通过 AdminGrantAirdropSubscription API 或
// GrantAirdropSubscription 函数向指定用户授予该套餐。
var AirdropSubscriptionPlanId = 0

var QuotaForInviter = 0
var QuotaForInvitee = 0

// InviteTopupRebateRatio 邀请充值返利比例
// 配置参数：当被邀请人充值成功后，按此比例给邀请人返利
// 示例：设置为 10.0 表示返利 10%（充值 100 美元，邀请人获得 10 美元价值的额度）
// 设置为 0.0 表示关闭返利功能
var InviteTopupRebateRatio = 10.0
var InviteConsumeRebateRatioLevel2 = 1.0
var ChannelDisableThreshold = 5.0
var AutomaticDisableChannelEnabled = false
var AutomaticEnableChannelEnabled = false
var QuotaRemindThreshold = 1000
var PreConsumedQuota = 500

var RetryTimes = 0

//var RootUserEmail = ""

var IsMasterNode bool

// NodeName 节点名称，从 NODE_NAME 环境变量读取；
// 用于审计日志中标识节点身份，在容器/K8s 部署时比自动探测到的容器内网 IP 更具可读性。
var NodeName = ""

var requestInterval int
var RequestInterval time.Duration

var SyncFrequency int // unit is second

var BatchUpdateEnabled = false
var BatchUpdateInterval int

var RelayTimeout int // unit is second

var RelayIdleConnTimeout int // unit is second
var RelayMaxIdleConns int
var RelayMaxIdleConnsPerHost int

var GeminiSafetySetting string

// https://docs.cohere.com/docs/safety-modes Type; NONE/CONTEXTUAL/STRICT
var CohereSafetySetting string

const (
	RequestIdKey         = "X-Oneapi-Request-Id"
	UpstreamRequestIdKey = "X-Upstream-Request-Id"
)

const (
	RoleGuestUser  = 0
	RoleCommonUser = 1
	RoleAdminUser  = 10
	RoleRootUser   = 100
)

func IsValidateRole(role int) bool {
	return role == RoleGuestUser || role == RoleCommonUser || role == RoleAdminUser || role == RoleRootUser
}

var (
	FileUploadPermission    = RoleGuestUser
	FileDownloadPermission  = RoleGuestUser
	ImageUploadPermission   = RoleGuestUser
	ImageDownloadPermission = RoleGuestUser
)

// Session duration constants (seconds)
const (
	SessionMaxAgeDefault  = 604800  // 7 days
	SessionMaxAgeExtended = 2592000 // 30 days
)

// All duration's unit is seconds
// Shouldn't larger then RateLimitKeyExpirationDuration
var (
	GlobalApiRateLimitEnable   bool
	GlobalApiRateLimitNum      int
	GlobalApiRateLimitDuration int64

	GlobalWebRateLimitEnable   bool
	GlobalWebRateLimitNum      int
	GlobalWebRateLimitDuration int64

	CriticalRateLimitEnable   bool
	CriticalRateLimitNum            = 20
	CriticalRateLimitDuration int64 = 20 * 60

	UploadRateLimitNum            = 10
	UploadRateLimitDuration int64 = 60

	DownloadRateLimitNum            = 10
	DownloadRateLimitDuration int64 = 60

	// Per-user search rate limit (applies after authentication, keyed by user ID)
	SearchRateLimitEnable         = true
	SearchRateLimitNum            = 10
	SearchRateLimitDuration int64 = 60
)

var RateLimitKeyExpirationDuration = 20 * time.Minute

const (
	UserStatusEnabled  = 1 // don't use 0, 0 is the default value!
	UserStatusDisabled = 2 // also don't use 0
)

const (
	TokenStatusEnabled   = 1 // don't use 0, 0 is the default value!
	TokenStatusDisabled  = 2 // also don't use 0
	TokenStatusExpired   = 3
	TokenStatusExhausted = 4
)

const (
	RedemptionCodeStatusEnabled  = 1 // don't use 0, 0 is the default value!
	RedemptionCodeStatusDisabled = 2 // also don't use 0
	RedemptionCodeStatusUsed     = 3 // also don't use 0
)

const (
	ChannelStatusUnknown          = 0
	ChannelStatusEnabled          = 1 // don't use 0, 0 is the default value!
	ChannelStatusManuallyDisabled = 2 // also don't use 0
	ChannelStatusAutoDisabled     = 3
)

const (
	TopUpStatusPending = "pending"
	TopUpStatusSuccess = "success"
	TopUpStatusFailed  = "failed"
	TopUpStatusExpired = "expired"
)
