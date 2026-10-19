package controller

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
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

// TestNewWechatUserIdentity 校验两个扫码入口共用的命名：
// username 首选「wechat_ + 本记录的 ID」，display_name 固定为 WeChat User。
func TestNewWechatUserIdentity(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 41, Username: "existing"}).Error)

	username, displayName, err := newWechatUserIdentity()
	require.NoError(t, err)
	require.Equal(t, "wechat_42", username)
	require.Equal(t, "WeChat User", displayName)
	require.LessOrEqual(t, len(username), model.UserNameMaxLength)
	require.LessOrEqual(t, len(displayName), model.UserNameMaxLength)
}

// TestNewWechatUserIdentityFallsBackToRandom 校验首选名字被人工占用时退化为随机兜底，
// 而不是让扫码注册入口被同一个名字卡死。
func TestNewWechatUserIdentityFallsBackToRandom(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	// aff_code 有 (provider_id, aff_code) 唯一索引，测试里的多条记录需要各自不同的值。
	require.NoError(t, model.DB.Create(&model.User{Id: 41, Username: "existing", AffCode: "aff-existing"}).Error)
	// 人工占用了本该分给 42 号的用户名。
	require.NoError(t, model.DB.Create(&model.User{Id: 10, Username: "wechat_42", AffCode: "aff-squatter"}).Error)

	username, displayName, err := newWechatUserIdentity()
	require.NoError(t, err)
	require.NotEqual(t, "wechat_42", username)
	require.Regexp(t, `^wechat_[0-9A-Za-z]{10}$`, username)
	require.Equal(t, "WeChat User", displayName)
	require.LessOrEqual(t, len(username), model.UserNameMaxLength)

	conflict, _, err := model.UserIdentityConflictFieldsGlobally(0, username, "")
	require.NoError(t, err)
	require.False(t, conflict)
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

// TestEstablishUserSessionWritesLoginState 校验扫码链路复用的 session 写入会把
// 用户 id / 用户名 / 角色 / 状态 / 分组与站点 provider_id 都落进 session。
// 这些字段是 authCheck 放行后续接口的依据，缺任何一项都会导致登录态不可用。
func TestEstablishUserSessionWritesLoginState(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	gin.SetMode(gin.TestMode)

	var got map[string]any
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("wechat-web-auth-test"))))
	router.GET("/wechat/scan", func(c *gin.Context) {
		c.Set(string(constant.ContextKeyProviderId), 7)
		user := &model.User{
			Id:       42,
			Username: "wx_user",
			Role:     common.RoleCommonUser,
			Status:   common.UserStatusEnabled,
			Group:    "default",
		}
		require.NoError(t, establishUserSession(c, user, false))

		session := sessions.Default(c)
		got = map[string]any{
			"id":          session.Get("id"),
			"provider_id": session.Get("provider_id"),
			"username":    session.Get("username"),
			"role":        session.Get("role"),
			"status":      session.Get("status"),
			"group":       session.Get("group"),
		}
		c.Status(200)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/wechat/scan", nil))

	require.Equal(t, 200, rec.Code)
	require.Equal(t, 42, got["id"])
	require.Equal(t, 7, got["provider_id"])
	require.Equal(t, "wx_user", got["username"])
	require.Equal(t, common.RoleCommonUser, got["role"])
	require.Equal(t, common.UserStatusEnabled, got["status"])
	require.Equal(t, "default", got["group"])
	// 必须种下会话 cookie，否则浏览器侧不构成登录态。
	require.NotEmpty(t, rec.Result().Cookies())
}

// TestBuildWechatWebAuthPayloadIncludesLoginFields 校验展示 payload 同时带上
// 登录用户字段（前端据此写 localStorage.user）与扫码展示字段。
func TestBuildWechatWebAuthPayloadIncludesLoginFields(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "https://sub.example.com/wechat/scan", nil)
	c.Set(string(constant.ContextKeyProviderId), 7)

	user := &model.User{
		Id:          42,
		Username:    "wx_user",
		DisplayName: "wx_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
	}
	payload := buildWechatWebAuthPayload(c, "openid_demo", user, true, "plain_pwd", "sk-key")

	// 登录用户字段：前端写 localStorage 后，路由守卫与顶栏据此判断已登录。
	require.Equal(t, 42, payload["id"])
	require.Equal(t, 7, payload["provider_id"])
	require.Equal(t, "wx_user", payload["username"])
	require.Equal(t, common.RoleCommonUser, payload["role"])
	// 展示字段保持原有语义。
	require.Equal(t, true, payload["created"])
	require.Equal(t, false, payload["found"])
	require.Equal(t, "openid_demo", payload["openid"])
	require.Equal(t, "sk-key", payload["api_key"])
	require.Equal(t, "plain_pwd", payload["password"])
	require.Equal(t, "https://sub.example.com", payload["site_url"])
}

// TestEstablishedSessionPassesUserAuth 端到端校验：扫码链路建立的 session
// 能通过 middleware.UserAuth()，即扫完码后访问站内受保护接口不再被拦截。
// 这是本次需求的核心行为，必须由真实中间件验证，而不是只看 session 字段。
func TestEstablishedSessionPassesUserAuth(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("wechat-web-auth-e2e"))))
	// 模拟 TenantResolver 写入当前站点，authCheck 会比对 session 里的 provider_id。
	router.Use(func(c *gin.Context) {
		c.Set(string(constant.ContextKeyProviderId), 0)
		c.Next()
	})
	router.GET("/wechat/scan", func(c *gin.Context) {
		user := &model.User{
			Id:       42,
			Username: "wx_user",
			Role:     common.RoleCommonUser,
			Status:   common.UserStatusEnabled,
			Group:    "default",
		}
		require.NoError(t, establishUserSession(c, user, false))
		c.Status(200)
	})
	router.GET("/api/user/self", middleware.UserAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})

	// 第一步：扫码建立登录态，拿到会话 cookie。
	scanRec := httptest.NewRecorder()
	router.ServeHTTP(scanRec, httptest.NewRequest("GET", "/wechat/scan", nil))
	require.Equal(t, 200, scanRec.Code)
	cookies := scanRec.Result().Cookies()
	require.NotEmpty(t, cookies, "扫码后应种下会话 cookie")

	// 第二步：带 cookie 与 New-Api-User 请求受保护接口，应放行而不是 401。
	req := httptest.NewRequest("GET", "/api/user/self", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	req.Header.Set("New-Api-User", "42")
	authRec := httptest.NewRecorder()
	router.ServeHTTP(authRec, req)

	require.Equal(t, 200, authRec.Code)
	require.Contains(t, authRec.Body.String(), `"success":true`)
}

// TestEstablishedSessionRejectedForDisabledUser 校验被封禁账号不会被放行，
// 与 completeWechatLogin 的准入判断保持一致。
func TestEstablishedSessionRejectedForDisabledUser(t *testing.T) {
	setupWechatWebAuthTestDB(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("wechat-web-auth-banned"))))
	router.Use(func(c *gin.Context) {
		c.Set(string(constant.ContextKeyProviderId), 0)
		c.Next()
	})
	router.GET("/wechat/scan", func(c *gin.Context) {
		user := &model.User{
			Id:       43,
			Username: "wx_banned",
			Role:     common.RoleCommonUser,
			Status:   common.UserStatusDisabled,
			Group:    "default",
		}
		require.NoError(t, establishUserSession(c, user, false))
		c.Status(200)
	})
	router.GET("/api/user/self", middleware.UserAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"success": true})
	})

	scanRec := httptest.NewRecorder()
	router.ServeHTTP(scanRec, httptest.NewRequest("GET", "/wechat/scan", nil))
	cookies := scanRec.Result().Cookies()

	req := httptest.NewRequest("GET", "/api/user/self", nil)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	req.Header.Set("New-Api-User", "43")
	authRec := httptest.NewRecorder()
	router.ServeHTTP(authRec, req)

	// 封禁用户不应拿到成功响应。
	require.NotContains(t, authRec.Body.String(), `"success":true`)
}
