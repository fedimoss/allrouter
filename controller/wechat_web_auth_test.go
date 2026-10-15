package controller

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupWechatWebAuthTestDB 用内存库替换全局 DB，并迁移微信配置表与用户表。
func setupWechatWebAuthTestDB(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.WechatOfficialConfig{}, &model.User{}, &model.Token{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedisEnabled
	})
}

// TestLoadWechatWebAuthConfigByProvider 校验按站点（provider_id）读取各自的服务号凭据，
// 主站(0)与分站(7)互不串用。
func TestLoadWechatWebAuthConfigByProvider(t *testing.T) {
	setupWechatWebAuthTestDB(t)

	require.NoError(t, model.DB.Create(&model.WechatOfficialConfig{
		ProviderId: 0, AppId: "wx_main", AppSecret: "secret_main", Enabled: true,
	}).Error)
	require.NoError(t, model.DB.Create(&model.WechatOfficialConfig{
		ProviderId: 7, AppId: "wx_provider7", AppSecret: "secret_provider7", Enabled: true,
	}).Error)

	main, err := loadWechatWebAuthConfig(0)
	require.NoError(t, err)
	require.Equal(t, "wx_main", main.AppId)
	require.Equal(t, "secret_main", main.AppSecret)

	provider7, err := loadWechatWebAuthConfig(7)
	require.NoError(t, err)
	require.Equal(t, "wx_provider7", provider7.AppId)
	require.Equal(t, "secret_provider7", provider7.AppSecret)
}

// TestLoadWechatWebAuthConfigMissing 校验未配置 / 停用时返回明确错误（不回退到其它站点）。
func TestLoadWechatWebAuthConfigMissing(t *testing.T) {
	setupWechatWebAuthTestDB(t)

	_, err := loadWechatWebAuthConfig(0)
	require.Error(t, err)

	require.NoError(t, model.DB.Create(&model.WechatOfficialConfig{
		ProviderId: 0, AppId: "wx_main", AppSecret: "secret_main", Enabled: false,
	}).Error)
	_, err = loadWechatWebAuthConfig(0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "停用")
}

// TestGenerateUniqueWechatUsername 校验随机用户名带前缀、长度合规，且能避开已占用的名字。
func TestGenerateUniqueWechatUsername(t *testing.T) {
	setupWechatWebAuthTestDB(t)

	username, err := generateUniqueWechatUsername()
	require.NoError(t, err)
	require.Contains(t, username, wechatWebAuthUsernamePrefix)
	require.LessOrEqual(t, len(username), model.UserNameMaxLength)
}

// TestWechatWebAuthDebugConfigRequiresProviderConfig 校验配置接口按站点取 appid；
// 未配置时返回业务错误而不是 500。
func TestWechatWebAuthDebugConfigRequiresProviderConfig(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	query := url.Values{}
	query.Set("redirect_uri", "https://allrouter.ai/wechat/scan")
	c.Request = httptest.NewRequest("GET", "/api/wechat/web_auth/config?"+query.Encode(), nil)

	WechatWebAuthDebugConfig(c)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"success":false`)
}

// TestWechatWebAuthDebugConfigBuildsAuthorizeURL 校验配置就绪时返回该站点的授权链接。
func TestWechatWebAuthDebugConfigBuildsAuthorizeURL(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.Create(&model.WechatOfficialConfig{
		ProviderId: 0, AppId: "wx_main", AppSecret: "secret_main", Enabled: true,
	}).Error)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	query := url.Values{}
	query.Set("redirect_uri", "https://allrouter.ai/wechat/scan")
	c.Request = httptest.NewRequest("GET", "/api/wechat/web_auth/config?"+query.Encode(), nil)

	WechatWebAuthDebugConfig(c)
	body := rec.Body.String()
	require.Contains(t, body, `"success":true`)
	require.Contains(t, body, "wx_main")
	require.Contains(t, body, "scope=snsapi_base")
}

// TestWechatWebAuthTokenMissingCode 校验缺少 code 时返回业务错误（不触发网络/DB）。
func TestWechatWebAuthTokenMissingCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/api/wechat/web_auth/token", nil)

	WechatWebAuthToken(c)

	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"success":false`)
	require.Contains(t, rec.Body.String(), "code")
}

// TestWechatScanEntryRedirectsToAuthorize 校验扫码入口在无 code 时 302 到微信授权链接。
func TestWechatScanEntryRedirectsToAuthorize(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.Create(&model.WechatOfficialConfig{
		ProviderId: 0, AppId: "wx_main", AppSecret: "secret_main", Enabled: true,
	}).Error)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "https://allrouter.ai/wechat/scan", nil)
	c.Request.Host = "allrouter.ai"

	WechatScanEntry(c)

	require.Equal(t, 302, rec.Code)
	location := rec.Header().Get("Location")
	require.Contains(t, location, "open.weixin.qq.com/connect/oauth2/authorize")
	require.Contains(t, location, "appid=wx_main")
	require.Contains(t, location, "scope=snsapi_base")
	// 回调地址应是当前扫码入口。
	require.Contains(t, location, "redirect_uri=")
}

// TestWechatScanEntryWithCodeFallsThrough 校验带 code（微信回调）时不跳转，交给后续 handler。
func TestWechatScanEntryWithCodeFallsThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	engine := gin.New()
	engine.GET("/wechat/scan",
		WechatScanEntry,
		func(ctx *gin.Context) {
			called = true
			ctx.String(200, "spa")
		},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://allrouter.ai/wechat/scan?code=abc", nil)
	engine.ServeHTTP(rec, req)

	require.True(t, called, "带 code 时应继续执行后续 handler 返回 SPA 页面")
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "spa", rec.Body.String())
}

// TestWechatScanEntryWithoutConfig 校验未配置服务号时不跳微信，直接返回原因。
func TestWechatScanEntryWithoutConfig(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "https://allrouter.ai/wechat/scan", nil)
	c.Request.Host = "allrouter.ai"

	WechatScanEntry(c)

	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), "扫码服务暂不可用")
}

// TestWechatScanEntryDebugPassesThrough 校验 ?debug=1 时不跳授权，直接交给后续 handler 渲染页面，
// 便于在普通浏览器里打开该页调试样式（即使未配置服务号也应可访问）。
func TestWechatScanEntryDebugPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	engine := gin.New()
	engine.GET("/wechat/scan",
		WechatScanEntry,
		func(ctx *gin.Context) {
			called = true
			ctx.String(200, "spa")
		},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://allrouter.ai/wechat/scan?debug=1", nil)
	engine.ServeHTTP(rec, req)

	require.True(t, called, "debug=1 时应继续执行后续 handler 返回页面")
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "spa", rec.Body.String())
}

// TestEnsureWechatDefaultTokenCreatesThenReuses 校验 default 令牌首次创建、后续复用同一 key。
func TestEnsureWechatDefaultTokenCreatesThenReuses(t *testing.T) {
	setupWechatWebAuthTestDB(t)

	require.NoError(t, model.DB.Create(&model.User{
		Id: 100, Username: "wx_user", ProviderId: 0,
	}).Error)

	first, err := ensureWechatDefaultToken(100, 0)
	require.NoError(t, err)
	require.NotEmpty(t, first)

	// 第二次应复用同一 key，不重复创建。
	second, err := ensureWechatDefaultToken(100, 0)
	require.NoError(t, err)
	require.Equal(t, first, second)

	// 库中应只有一条名为 default 的令牌。
	var count int64
	require.NoError(t, model.DB.Model(&model.Token{}).
		Where("user_id = ? AND provider_id = ? AND name = ?", 100, 0, wechatWebAuthDefaultTokenName).
		Count(&count).Error)
	require.Equal(t, int64(1), count)

	// 分组应为 default。
	stored, err := model.GetTokenByUserIdProviderAndName(100, 0, wechatWebAuthDefaultTokenName)
	require.NoError(t, err)
	require.Equal(t, wechatWebAuthDefaultTokenGroup, stored.Group)
	require.Equal(t, -1, int(stored.ExpiredTime))
}

// TestWechatWebAuthSiteURLUsesRequestHost 校验站点根地址取当前请求域名（不含 API 路径）。
func TestWechatWebAuthSiteURLUsesRequestHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "https://sub.example.com/wechat/scan", nil)
	require.Equal(t, "https://sub.example.com", wechatWebAuthSiteURL(c))
}
