package controller

// 本文件是「普通链接二维码 + 网页授权(snsapi_base)」方案的服务端接口。
//
// 官方文档：https://developers.weixin.qq.com/doc/service/guide/h5/auth.html
//
// 与带参二维码方案（controller/wechat_message.go）的区别：
//   - 带参二维码：扫码后停在会话，服务器被动回复消息；
//   - 本方案：二维码内容是一个 URL，微信扫一扫直接打开网页，页面通过网页授权拿到 openid，
//     再调用本文件的接口换取用户信息并渲染。用户不需要再点一次，体验更顺。
//
// 前端页面（web/src/pages/WechatScan）流程：
//  1. 页面加载时判断 URL 是否带 code；
//  2. 没有 code → 向后端要授权链接（按当前站点取 appid）并跳转；
//  3. 微信回调本页面并带上 code → 前端调 /api/wechat/web_auth/token；
//  4. 后端按当前站点取 appid/secret，用 code 换 openid；
//  5. 该 openid 在当前站点没有账号时创建账号（复用 User.Insert，自带赠送额度等），
//     已有账号则直接返回，最后把展示信息返回给页面渲染。
//
// 多站点说明（重要）：
//   - 主站与分站各自配置自己的服务号，凭据统一存在 wechat_official_configs 表，
//     以 provider_id 区分：主站 provider_id=0，分站为对应 providers.id。
//   - 当前请求属于哪个站点，由 middleware.TenantResolver 按域名解析后写入上下文
//     (constant.ContextKeyProviderId)，这里直接读取，不再自己解析域名。
//   - 因此同一个微信用户在 A 站与 B 站会各自拿到（各自服务号下的）openid，进而各自建号。

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// WechatScanEntry 是扫码入口（GET /wechat/scan），用于在“加载前端之前”决定去向：
//   - URL 上没有 code：说明用户刚扫码，直接 302 跳到微信授权链接。
//     好处是这一次请求完全不下载前端资源，省掉整包 JS 的加载，跳转更快、白屏更短。
//   - URL 上带 code：说明已从微信授权回调回来，交给前端 SPA 处理（渲染页面、调接口取数）。
//   - URL 上带 debug=1：调试直通，不跳授权也不取数，直接返回 SPA 页面，
//     方便在普通浏览器里打开本页调试样式（页面会用演示数据渲染）。
//
// 该入口只做跳转判断，不参与取数，因此不依赖前端是否已加载。
func WechatScanEntry(c *gin.Context) {
	// 调试直通：?debug=1 时直接渲染页面，避免在浏览器里被 302 跳到微信授权页而无法查看页面。
	if strings.TrimSpace(c.Query("debug")) != "" {
		c.Next()
		return
	}

	// 已带 code：把请求交回 SPA（由 web/dist 的 index.html 承接，前端再调 web_auth/token）。
	if strings.TrimSpace(c.Query("code")) != "" {
		c.Next()
		return
	}

	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	config, err := loadWechatWebAuthConfig(providerId)
	if err != nil {
		// 配置缺失/停用时不跳微信，直接把原因告诉用户，避免跳到微信再失败。
		c.String(http.StatusOK, "扫码服务暂不可用：%s", err.Error())
		return
	}

	// 回调地址固定为当前扫码入口本身，用请求 Host 组装，与前端 window.location.origin 行为一致，
	// 确保落在已配置的网页授权域名下。
	redirectURI := "https://" + c.Request.Host + "/wechat/scan"

	authorizeURL, err := service.BuildWechatOAuth2AuthorizeURL(config.AppId, redirectURI, "", "")
	if err != nil {
		logger.LogError(c.Request.Context(), "wechat scan build authorize url failed: "+err.Error())
		c.String(http.StatusOK, "扫码服务暂不可用：%s", err.Error())
		return
	}
	c.Redirect(http.StatusFound, authorizeURL)
}

// 新建微信用户的默认值。
const (
	// wechatWebAuthPasswordRandomLen 随机密码长度，需落在 User.Password 校验的 8-20 之间。
	wechatWebAuthPasswordRandomLen = 16
	// wechatWebAuthDefaultTokenName 扫码用户默认令牌的名称，同名令牌会被复用。
	wechatWebAuthDefaultTokenName = "default"
	// wechatWebAuthDefaultTokenGroup 默认令牌的分组。
	wechatWebAuthDefaultTokenGroup = "default"
)

// WechatWebAuthToken 用网页授权 code 换取 openid，并按当前站点查询/创建用户。
//
// 请求：GET /api/wechat/web_auth/token?code=xxx
//
// 返回 data 字段：
//   - openid   ：当前用户的 openid（snsapi_base 静默授权只能拿到它）。
//   - found    ：调用前该 openid 在当前站点是否已有账号。
//   - created  ：本次是否新建了账号。
//   - username / password：账号与密码（password 仅在新建时返回一次，已存在时为空）。
//   - site_url / api_key：站点根地址与真实令牌 key（前端按协议再拼出各自的 Base URL）。
func WechatWebAuthToken(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		common.ApiErrorMsg(c, "缺少 code 参数")
		return
	}

	// 当前请求归属哪个站点：主站为 0，分站为 provider.id。
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)

	config, err := loadWechatWebAuthConfig(providerId)
	if err != nil {
		logger.LogError(c.Request.Context(), "wechat web auth load config failed: "+err.Error())
		common.ApiError(c, err)
		return
	}

	token, err := service.NewWechatOAuth2Service().ExchangeCodeForToken(
		c.Request.Context(),
		config.AppId,
		config.AppSecret,
		code,
	)
	if err != nil {
		logger.LogError(c.Request.Context(), "wechat web auth exchange code failed: "+err.Error())
		common.ApiError(c, err)
		return
	}
	// 快照页模式返回的是虚拟 openid，不能用于真实业务，直接拒绝。
	if token.IsSnapshotUser == 1 {
		logger.LogWarn(c.Request.Context(), "wechat web auth got snapshot user, openid is virtual")
		common.ApiErrorMsg(c, "当前网页处于快照模式，openid 为虚拟数据，请使用完整服务")
		return
	}

	user, created, plainPassword, err := findOrCreateWechatUser(c, providerId, token.OpenID)
	if err != nil {
		logger.LogError(c.Request.Context(), "wechat web auth find or create user failed: "+err.Error())
		common.ApiError(c, err)
		return
	}
	// 被封禁的账号不能建立登录态，与 completeWechatLogin 保持同一套准入判断。
	if user.Status != common.UserStatusEnabled {
		logger.LogWarn(c.Request.Context(), "wechat web auth user is disabled")
		common.ApiErrorMsg(c, "用户已被封禁")
		return
	}

	apiKey, err := ensureWechatDefaultToken(user.Id, providerId)
	if err != nil {
		logger.LogError(c.Request.Context(), "wechat web auth ensure default token failed: "+err.Error())
		common.ApiError(c, err)
		return
	}

	// 扫码即登录：拿到 openid 已唯一确定用户，无需再走用户名密码登录。
	// 这里直接建立 session，用户停留在本页即可点顶栏导航进入站内其它页面。
	if err := establishUserSession(c, user, false); err != nil {
		logger.LogError(c.Request.Context(), "wechat web auth establish session failed: "+err.Error())
		common.ApiErrorI18n(c, i18n.MsgUserSessionSaveFailed)
		return
	}

	common.ApiSuccess(c, buildWechatWebAuthPayload(c, token.OpenID, user, created, plainPassword, apiKey))
}

// ensureWechatDefaultToken 保证该用户在当前站点存在名为 default、分组为 default 的令牌，并返回其 key。
// 已存在则复用（取最早创建的一条），不存在则新建一个永不过期、不限额度的令牌。
func ensureWechatDefaultToken(userId int, providerId int) (string, error) {
	existing, err := model.GetTokenByUserIdProviderAndName(userId, providerId, wechatWebAuthDefaultTokenName)
	if err == nil && existing != nil {
		return existing.Key, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	key, err := common.GenerateKey()
	if err != nil {
		return "", err
	}
	now := common.GetTimestamp()
	token := &model.Token{
		ProviderId:         providerId,
		UserId:             userId,
		Key:                key,
		Status:             common.UserStatusEnabled,
		Name:               wechatWebAuthDefaultTokenName,
		CreatedTime:        now,
		AccessedTime:       now,
		ExpiredTime:        -1, // 永不过期
		RemainQuota:        0,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		Group:              wechatWebAuthDefaultTokenGroup,
	}
	if err := token.Insert(); err != nil {
		return "", err
	}
	return token.Key, nil
}

// loadWechatWebAuthConfig 读取当前站点的服务号凭据（wechat_official_configs，按 provider_id）。
// 未配置或未启用时返回明确错误，避免用错站点的 appid 换 openid。
func loadWechatWebAuthConfig(providerId int) (*model.WechatOfficialConfig, error) {
	config, err := model.GetWechatConfigByProvider(providerId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("当前站点尚未配置微信服务号（AppID/AppSecret）")
		}
		return nil, err
	}
	if strings.TrimSpace(config.AppId) == "" || strings.TrimSpace(config.AppSecret) == "" {
		return nil, errors.New("当前站点微信服务号配置不完整（AppID/AppSecret）")
	}
	if !config.Enabled {
		return nil, errors.New("当前站点微信服务号配置已停用")
	}
	return config, nil
}

// findOrCreateWechatUser 按 (provider_id, openid) 查询用户；不存在则创建。
// 返回：用户、是否本次新建、新建时的明文密码（已存在时为空串）。
//
// 创建走 model.User.Insert，复用其内部的赠送额度 / 赠送订阅 / 奖励流水 / 邀请记录等逻辑，
// 不能自己 DB.Create，否则会漏掉站点级的新用户奖励。
func findOrCreateWechatUser(c *gin.Context, providerId int, openID string) (*model.User, bool, string, error) {
	existing, err := model.GetUserByProviderAndWeChatId(providerId, openID)
	if err == nil && existing != nil {
		return existing, false, "", nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, "", err
	}

	// 与 /api/oauth/wechat/qr 共用同一套命名（见 newWechatUserIdentity）：
	// 首选 wechat_<ID>，该名字被占用时函数内部已退化为 wechat_<随机串>。
	username, displayName, err := newWechatUserIdentity()
	if err != nil {
		return nil, false, "", err
	}
	// 密码随机生成；用户不感知，仅在创建时回显一次，库中存的是哈希。
	plainPassword := common.GetRandomString(wechatWebAuthPasswordRandomLen)

	user := &model.User{
		ProviderId:  providerId,
		WeChatId:    openID,
		Username:    username,
		DisplayName: displayName,
		Password:    plainPassword,
		// email 留空：微信扫码拿不到邮箱。空值会跳过邮箱唯一性校验，
		// 也避免用随机假邮箱占用全局 email 唯一索引。
		Email:      "",
		Role:       common.RoleCommonUser,
		Status:     common.UserStatusEnabled,
		RegisterIp: getRegistrationIP(c),
	}

	if err := user.Insert(0); err != nil {
		return nil, false, "", err
	}
	return user, true, plainPassword, nil
}

// buildWechatWebAuthPayload 组装返回给前端的展示数据。
//
// site_url 取当前请求域名（即用户扫码访问的站点域名）；api_key 为真实令牌 key。
// 各协议实际使用的 Base URL 由前端按 SDK 规则拼接（OpenAI 系列含 /v1，Anthropic 不含），
// 因此这里只返回站点根地址，避免把某个协议的路径规则写死在服务端。
// password 仅在本次新建账号时返回（明文只此一次，库中为哈希，之后无法再取回）。
//
// 同时并入 buildLoginUserPayload 的登录用户字段：扫码即登录，前端据此写入
// localStorage.user（路由守卫、角色与模块权限都读它）。
func buildWechatWebAuthPayload(c *gin.Context, openID string, user *model.User, created bool, plainPassword string, apiKey string) gin.H {
	payload := buildLoginUserPayload(
		user,
		common.GetContextKeyInt(c, constant.ContextKeyProviderId),
	)
	payload["openid"] = openID
	payload["found"] = !created
	payload["created"] = created
	payload["site_url"] = wechatWebAuthSiteURL(c)
	payload["api_key"] = apiKey
	payload["password"] = plainPassword
	payload["email"] = user.Email
	return payload
}

// wechatWebAuthSiteURL 用当前请求 Host 组装站点根地址（不含任何 API 路径）。
// 与 302 回调地址保持同一来源，确保展示的地址与用户实际访问的站点一致。
func wechatWebAuthSiteURL(c *gin.Context) string {
	return "https://" + c.Request.Host
}

// WechatWebAuthDebugConfig 返回当前站点的网页授权跳转链接，便于前端跳转与管理员核对。
//
// 请求：GET /api/wechat/web_auth/config?redirect_uri=xxx
func WechatWebAuthDebugConfig(c *gin.Context) {
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	if redirectURI == "" {
		common.ApiErrorMsg(c, "缺少 redirect_uri 参数")
		return
	}

	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	config, err := loadWechatWebAuthConfig(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	authorizeURL, err := service.BuildWechatOAuth2AuthorizeURL(config.AppId, redirectURI, "", "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"appid":         config.AppId,
		"authorize_url": authorizeURL,
	})
}
