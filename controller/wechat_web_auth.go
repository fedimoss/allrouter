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
//  2. 没有 code → 跳转微信授权链接（snsapi_base 静默授权）；
//  3. 微信回调本页面并带上 code → 前端调 /api/wechat/web_auth/token；
//  4. 后端用 code 换 openid，按 openid 查用户，返回要展示的信息。
//
// 说明：体验阶段返回固定文案；接入真实业务时，把 buildWechatWebAuthPayload 里的内容
// 改为根据 model 查询结果动态生成即可（代码中已标注位置）。

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// wechatWebAuthAppID / wechatWebAuthSecret 是网页授权所需的服务号凭据。
// 与消息推送、二维码等其它微信能力使用的是同一个服务号，但此处独立成常量，
// 便于单独替换；正式接入时应统一收敛到同一处配置。
const (
	wechatWebAuthAppID  = "wx_placeholder_appid"
	wechatWebAuthSecret = "placeholder_appsecret"
)

// WechatWebAuthToken 用网页授权 code 换取 openid，并按 openid 查询用户信息。
//
// 请求：GET /api/wechat/web_auth/token?code=xxx
//
// 返回 data 字段：
//   - openid   ：当前用户的 openid（snsapi_base 静默授权只能拿到它）。
//   - found    ：该 openid 是否已绑定站内用户。
//   - user_*   ：绑定用户的基础信息（未绑定时为空）。
//   - base_url / api_key：当前体验阶段固定展示的内容。
func WechatWebAuthToken(c *gin.Context) {
	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		common.ApiErrorMsg(c, "缺少 code 参数")
		return
	}

	token, err := service.NewWechatOAuth2Service().ExchangeCodeForToken(
		c.Request.Context(),
		wechatWebAuthAppID,
		wechatWebAuthSecret,
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

	// 按 openid 查站内用户（wechat_id 绑定的用户）。
	var user model.User
	found := model.IsWeChatIdAlreadyTaken(token.OpenID)
	if found {
		user.WeChatId = token.OpenID
		if err := user.FillUserByWeChatId(); err != nil {
			logger.LogError(c.Request.Context(), "wechat web auth fill user failed: "+err.Error())
			common.ApiError(c, err)
			return
		}
	}

	common.ApiSuccess(c, buildWechatWebAuthPayload(token.OpenID, found, &user))
}

// buildWechatWebAuthPayload 组装返回给前端的展示数据。
//
// 体验阶段：base_url / api_key 固定显示。
// 接入真实业务时：把 base_url / api_key 换成按 user 查询到的真实配置即可。
func buildWechatWebAuthPayload(openID string, found bool, user *model.User) gin.H {
	payload := gin.H{
		"openid":   openID,
		"found":    found,
		"base_url": "https://allrouter.ai/v1",
		"api_key":  "123456789",
	}
	if found && user != nil {
		payload["user_id"] = user.Id
		payload["username"] = user.Username
		payload["display_name"] = user.DisplayName
		payload["email"] = user.Email
	}
	return payload
}

// WechatWebAuthDebugConfig 返回网页授权跳转链接，便于前端在未配置 AppID 时给出提示，
// 也方便管理员人工核对授权地址是否正确。
//
// 请求：GET /api/wechat/web_auth/config?redirect_uri=xxx
func WechatWebAuthDebugConfig(c *gin.Context) {
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	if redirectURI == "" {
		common.ApiErrorMsg(c, "缺少 redirect_uri 参数")
		return
	}
	authorizeURL, err := service.BuildWechatOAuth2AuthorizeURL(wechatWebAuthAppID, redirectURI, "", "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"appid":         wechatWebAuthAppID,
		"authorize_url": authorizeURL,
	})
}
