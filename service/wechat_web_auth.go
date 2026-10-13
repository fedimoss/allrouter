// 本文件封装微信服务号「网页授权」（H5 授权）的服务器端逻辑。
//
// 官方文档：https://developers.weixin.qq.com/doc/service/guide/h5/auth.html
//
// 与带参二维码方案的区别（新接手同事先读这段）：
//   - 带参二维码：用户扫码后停留在公众号会话，微信把事件推给服务器，由服务器被动回复消息。
//   - 普通链接二维码 + 网页授权：二维码内容直接是一个 URL，微信扫一扫会用内置浏览器打开该
//     URL，页面通过网页授权拿到 openid，再向后端换取用户信息在页面上渲染。
//     即“扫码即打开网页”，不需要用户再点一次。
//
// 网页授权流程（本文件实现第 2、3 步，第 1 步由前端拼接跳转链接）：
//  1. 前端引导用户打开授权链接（只负责跳转，不参与取数）；
//     https://open.weixin.qq.com/connect/oauth2/authorize?appid=APPID&redirect_uri=REDIRECT&
//     response_type=code&scope=snsapi_base&state=STATE#wechat_redirect
//  2. 用户授权后微信回调 redirect_uri，并在 URL 上带 code；
//  3. 后端用 code 换网页授权 access_token，同时拿到 openid（本文件 OAuth2AccessToken）；
//  4. 本场景只需要 openid（snsapi_base 只能拿到 openid，且无需用户点击同意，静默授权）。
//
// 适用范围与前提：
//   - 仅已认证服务号可用；
//   - 需在「设置与开发 - 账号设置 - 功能设置」配置「网页授权域名」，且回调链接域名必须与之一致；
//   - 网页授权 access_token 与“获取接口调用凭据”的全局 access_token 不是同一个，不能混用。

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	// wechatOAuth2AuthorizeHost 是网页授权跳转域名（前端拼接授权链接时使用）。
	wechatOAuth2AuthorizeHost = "https://open.weixin.qq.com"
	// wechatOAuth2AuthorizePath 是网页授权跳转路径。
	wechatOAuth2AuthorizePath = "/connect/oauth2/authorize"
	// wechatOAuth2AccessTokenPath 是用 code 换取网页授权 access_token 的接口路径。
	wechatOAuth2AccessTokenPath = "/sns/oauth2/access_token"
	// wechatOAuth2ScopeBase 静默授权作用域，只返回 openid，用户无感知。
	wechatOAuth2ScopeBase = "snsapi_base"
	// wechatOAuth2ScopeUserInfo 弹窗授权作用域，可获取昵称头像等信息。
	wechatOAuth2ScopeUserInfo = "snsapi_userinfo"
	// wechatOAuth2Timeout 单次 HTTP 请求超时。
	wechatOAuth2Timeout = 10 * time.Second
)

// WechatOAuth2Token 是用 code 换取到的网页授权凭证。
//
// 字段含义（对应微信文档“换取用户授权 Token”一节）：
//   - AccessToken 网页授权接口调用凭证，与全局 access_token 不同，仅能用于该用户信息相关接口。
//   - ExpiresIn   凭证有效期（秒）。
//   - RefreshToken 刷新令牌，有效期 30 天。
//   - OpenID      用户在当前服务号下的唯一标识，本业务的核心字段。
//   - Scope       实际授权作用域，如 snsapi_base。
//   - IsSnapshotUser 快照页模式虚拟账号标记（值为 1 表示是），此时 openid 为虚拟数据，不可用于真实业务。
type WechatOAuth2Token struct {
	AccessToken    string
	ExpiresIn      int
	RefreshToken   string
	OpenID         string
	Scope          string
	IsSnapshotUser int
}

// WechatOAuth2Error 表示微信返回的业务错误（HTTP 200 但 errcode 非 0）。
// 常见：40029 code 无效；40163 code 已被使用；40013 不合法的 AppID。
type WechatOAuth2Error struct {
	ErrCode int
	ErrMsg  string
}

func (e *WechatOAuth2Error) Error() string {
	return fmt.Sprintf("wechat oauth2 failed: errcode=%d errmsg=%s", e.ErrCode, e.ErrMsg)
}

// WechatOAuth2Service 封装网页授权中“用 code 换 access_token/openid”的调用。
type WechatOAuth2Service struct {
	httpClient *http.Client
	host       string
}

// NewWechatOAuth2Service 创建服务实例。
func NewWechatOAuth2Service() *WechatOAuth2Service {
	return &WechatOAuth2Service{
		httpClient: &http.Client{Timeout: wechatOAuth2Timeout},
		host:       wechatOpenAPIHost,
	}
}

// SetHTTPClient 注入自定义 HTTP 客户端，便于测试或复用已有客户端配置。
func (s *WechatOAuth2Service) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.httpClient = client
	}
}

// SetHost 覆盖接口域名，默认使用正式环境域名，测试时可指向 mock 服务。
func (s *WechatOAuth2Service) SetHost(host string) {
	if host != "" {
		s.host = host
	}
}

// BuildWechatOAuth2AuthorizeURL 构造网页授权跳转链接（前端可直接使用）。
//
// 参数：
//   - appID       服务号 AppID。
//   - redirectURI 授权后回调地址，必须是已配置的网页授权域名下的地址，且需 https。
//   - scope       授权作用域，留空默认 snsapi_base（静默授权，仅拿 openid）。
//   - state       防 CSRF 的透传参数，可留空。
//
// 注意：微信对授权链接做正则强匹配，参数顺序不能随意调整，因此这里按官方顺序拼接。
func BuildWechatOAuth2AuthorizeURL(appID string, redirectURI string, scope string, state string) (string, error) {
	appID = strings.TrimSpace(appID)
	redirectURI = strings.TrimSpace(redirectURI)
	if appID == "" {
		return "", fmt.Errorf("wechat oauth2 appid is empty")
	}
	if redirectURI == "" {
		return "", fmt.Errorf("wechat oauth2 redirect_uri is empty")
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = wechatOAuth2ScopeBase
	}

	query := url.Values{}
	query.Set("appid", appID)
	// redirect_uri 需要 urlEncode 后再拼接，url.Values 会自动编码。
	query.Set("redirect_uri", redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", scope)
	if state != "" {
		query.Set("state", state)
	}

	// 微信要求链接末尾必须带 #wechat_redirect，且必须在 query 之后。
	return fmt.Sprintf(
		"%s%s?%s#wechat_redirect",
		wechatOAuth2AuthorizeHost,
		wechatOAuth2AuthorizePath,
		query.Encode(),
	), nil
}

// ExchangeCodeForToken 用授权 code 换取网页授权 access_token 与 openid。
//
// 说明：
//   - code 只能使用一次，5 分钟未使用自动过期；重复使用会返回 40163。
//   - 本接口必须由服务端发起，secret 不能下发到客户端。
//   - snsapi_base 情况下只能拿到 openid，无法也无需再调用其他接口获取资料。
func (s *WechatOAuth2Service) ExchangeCodeForToken(ctx context.Context, appID string, secret string, code string) (*WechatOAuth2Token, error) {
	if s == nil {
		return nil, fmt.Errorf("wechat oauth2 service is nil")
	}
	appID = strings.TrimSpace(appID)
	secret = strings.TrimSpace(secret)
	code = strings.TrimSpace(code)
	if appID == "" {
		return nil, fmt.Errorf("wechat oauth2 appid is empty")
	}
	if secret == "" {
		return nil, fmt.Errorf("wechat oauth2 secret is empty")
	}
	if code == "" {
		return nil, fmt.Errorf("wechat oauth2 code is empty")
	}

	host := s.host
	if host == "" {
		host = wechatOpenAPIHost
	}
	requestURL, err := url.Parse(host + wechatOAuth2AccessTokenPath)
	if err != nil {
		return nil, err
	}
	query := requestURL.Query()
	query.Set("appid", appID)
	query.Set("secret", secret)
	query.Set("code", code)
	query.Set("grant_type", "authorization_code")
	requestURL.RawQuery = query.Encode()

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: wechatOAuth2Timeout}
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()

	respBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, fmt.Errorf("wechat oauth2 http status %d: %s", httpResponse.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var response struct {
		AccessToken    string `json:"access_token"`
		ExpiresIn      int    `json:"expires_in"`
		RefreshToken   string `json:"refresh_token"`
		OpenID         string `json:"openid"`
		Scope          string `json:"scope"`
		IsSnapshotUser int    `json:"is_snapshotuser"`
		ErrCode        int    `json:"errcode"`
		ErrMsg         string `json:"errmsg"`
	}
	if err := common.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}
	if response.ErrCode != 0 {
		return nil, &WechatOAuth2Error{ErrCode: response.ErrCode, ErrMsg: response.ErrMsg}
	}
	if strings.TrimSpace(response.OpenID) == "" {
		return nil, fmt.Errorf("wechat oauth2 returned empty openid")
	}

	return &WechatOAuth2Token{
		AccessToken:    response.AccessToken,
		ExpiresIn:      response.ExpiresIn,
		RefreshToken:   response.RefreshToken,
		OpenID:         response.OpenID,
		Scope:          response.Scope,
		IsSnapshotUser: response.IsSnapshotUser,
	}, nil
}
