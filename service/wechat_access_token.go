// 本文件封装微信“获取接口调用凭据”（getAccessToken）接口。
//
// 官方文档：https://developers.weixin.qq.com/doc/service/api/base/api_getaccesstoken.html
//
// 接口作用：
//   - 这是微信服务号/公众号几乎所有服务端接口的前置调用，用 AppID + AppSecret 换取
//     全局唯一的后台接口调用凭据 access_token。
//   - 后续接口（例如生成带参数二维码）都把 access_token 放在 URL 查询参数里。
//
// 关键规则（新接手同事务必了解）：
//   - access_token 有效期为 7200 秒（2 小时），文档要求开发者“妥善保存”。微信对同一
//     账号重复获取会返回同一个 token，并对旧 token 施加一个短暂的并存期；当前实现是
//     “每次调用都实时获取”，未做缓存。若后续调用频率高，应在此文件内加缓存（建议按
//     ExpiresIn 提前几十秒过期），而不是在各业务处各自加缓存。
//   - 不同应用类型的 access_token 互相隔离，且只能调用对应应用类型的接口。
//   - AppSecret 属于账号级密钥，泄露风险高；调用方 IP 需在微信后台白名单内，否则会返回
//     40164（IP 不在白名单）。
//   - appid / secret 的正式来源尚未确定，当前使用下方占位常量；确定来源后通过
//     SetCredentials 注入即可，无需改动调用方代码。

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
	// wechatOpenAPIHost 微信开放接口的公共域名。本文件与二维码接口共用该常量，
	// 因此不要在别处重复定义同名 Host。
	wechatOpenAPIHost = "https://api.weixin.qq.com"
	// wechatAccessTokenPath getAccessToken 的接口路径，完整地址为 host + path。
	wechatAccessTokenPath = "/cgi-bin/token"
	// wechatAccessTokenGrant grant_type 固定填写 client_credential，这是文档要求。
	wechatAccessTokenGrant = "client_credential"
	// wechatAccessTokenTimeout 单次 HTTP 请求超时。该接口本身很轻，10 秒足够；
	// 超时过短会误判微信偶发的 system error(-1)，过长则拖住上游请求。
	wechatAccessTokenTimeout = 10 * time.Second

	// 占位数据：appid / secret 的正式来源（.env、数据库、系统设置等）尚未确定。
	// 后续业务明确后，替换这两个常量，或在创建服务后调用 SetCredentials 注入。
	// 注意：正式环境的 AppSecret 属于敏感信息，不要硬编码或提交到仓库。
	wechatAccessTokenPlaceholderAppID  = "wx_placeholder_appid"
	wechatAccessTokenPlaceholderSecret = "placeholder_appsecret"
)

// WechatAccessToken 是 getAccessToken 成功时的返回体。
//
// 字段含义（对应微信文档“返回参数”）：
//   - AccessToken 获取到的凭据，后续接口调用使用。
//   - ExpiresIn   凭据有效时间，单位秒，目前为 7200。
type WechatAccessToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

// WechatAccessTokenError 表示微信返回的业务错误。
//
// 微信的“接口调用凭据”接口在业务失败时仍返回 HTTP 200，只在响应体里带 errcode/errmsg，
// 因此必须显式判断 errcode，而不能只看 HTTP 状态码。
// 常见错误码（详见官方文档“错误码”一节）：
//   - 40001 AppSecret 错误 / access_token 无效
//   - 40002 不合法的 grant_type
//   - 40013 不合法的 AppID
//   - 40125 不合法的 secret
//   - 40164 调用方 IP 不在白名单
//   - 40243 AppSecret 已被冻结
//   - 41004 缺少 secret 参数
type WechatAccessTokenError struct {
	ErrCode int
	ErrMsg  string
}

func (e *WechatAccessTokenError) Error() string {
	return fmt.Sprintf("wechat get access token failed: errcode=%d errmsg=%s", e.ErrCode, e.ErrMsg)
}

// WechatAccessTokenService 封装 getAccessToken 的调用细节。
//
// 字段说明：
//   - httpClient 可注入替换（测试、代理、复用连接池），为空时按超时兜底创建。
//   - host       接口域名，默认 wechatOpenAPIHost，测试时可指向 mock 服务。
//   - appID      微信应用 AppID。
//   - secret     微信应用 AppSecret，属于敏感信息，不要在日志中打印。
type WechatAccessTokenService struct {
	httpClient *http.Client
	host       string
	appID      string
	secret     string
}

// NewWechatAccessTokenService 创建服务实例，appid / secret 暂用占位数据。
// 调用方如需真实凭据，可在创建后调用 SetCredentials。
func NewWechatAccessTokenService() *WechatAccessTokenService {
	return &WechatAccessTokenService{
		httpClient: &http.Client{Timeout: wechatAccessTokenTimeout},
		host:       wechatOpenAPIHost,
		appID:      wechatAccessTokenPlaceholderAppID,
		secret:     wechatAccessTokenPlaceholderSecret,
	}
}

// SetHTTPClient 注入自定义 HTTP 客户端，便于测试或复用已有客户端配置。
func (s *WechatAccessTokenService) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.httpClient = client
	}
}

// SetHost 覆盖接口域名，默认使用正式环境域名。
func (s *WechatAccessTokenService) SetHost(host string) {
	if host != "" {
		s.host = host
	}
}

// SetCredentials 设置 appid 与 secret。
// 这是后续确定配置来源（.env / 数据库 / 系统设置）后的统一注入点。
func (s *WechatAccessTokenService) SetCredentials(appID string, secret string) {
	s.appID = strings.TrimSpace(appID)
	s.secret = strings.TrimSpace(secret)
}

// GetAccessToken 调用微信接口获取全局唯一后台接口调用凭据。
//
// 请求方式：GET host + /cgi-bin/token
//
//	查询参数 grant_type=client_credential、appid、secret。
//
// 处理要点：
//  1. 先做本地参数校验（appid / secret 不能为空），避免无意义的网络请求。
//  2. HTTP 非 2xx 视为传输层错误直接返回，附带响应体便于排查。
//  3. 响应体解析后优先判断 errcode：非 0 返回 WechatAccessTokenError。
//  4. access_token 为空同样视为失败，避免把无效凭据继续传给下游业务。
func (s *WechatAccessTokenService) GetAccessToken(ctx context.Context) (*WechatAccessToken, error) {
	if s == nil {
		return nil, fmt.Errorf("wechat access token service is nil")
	}
	if strings.TrimSpace(s.appID) == "" {
		return nil, fmt.Errorf("wechat appid is empty")
	}
	if strings.TrimSpace(s.secret) == "" {
		return nil, fmt.Errorf("wechat secret is empty")
	}

	// host / httpClient 允许为空，空值时回退到默认配置，保证零值服务也能工作。
	host := s.host
	if host == "" {
		host = wechatOpenAPIHost
	}
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: wechatAccessTokenTimeout}
	}

	requestURL, err := url.Parse(host + wechatAccessTokenPath)
	if err != nil {
		return nil, err
	}
	// 使用 url.Values 统一编码，避免 appid / secret 中的特殊字符破坏 URL。
	query := requestURL.Query()
	query.Set("grant_type", wechatAccessTokenGrant)
	query.Set("appid", s.appID)
	query.Set("secret", s.secret)
	requestURL.RawQuery = query.Encode()

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
		return nil, fmt.Errorf("wechat get access token http status %d: %s", httpResponse.StatusCode, strings.TrimSpace(string(respBody)))
	}

	// 成功与失败字段放在同一个匿名结构里解析：微信成功时不带 errcode，
	// 失败时不带 access_token，可用统一的 errcode 判断分支。
	var response struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := common.Unmarshal(respBody, &response); err != nil {
		return nil, err
	}
	if response.ErrCode != 0 {
		return nil, &WechatAccessTokenError{ErrCode: response.ErrCode, ErrMsg: response.ErrMsg}
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return nil, fmt.Errorf("wechat get access token returned empty access_token")
	}

	return &WechatAccessToken{
		AccessToken: response.AccessToken,
		ExpiresIn:   response.ExpiresIn,
	}, nil
}

// GetWechatAccessToken 使用默认服务实例获取接口调用凭据。
// 便捷入口，适合一次性调用；需要自定义 appid/secret 时请自行创建服务并注入。
func GetWechatAccessToken(ctx context.Context) (*WechatAccessToken, error) {
	return NewWechatAccessTokenService().GetAccessToken(ctx)
}
