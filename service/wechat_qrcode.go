// 本文件封装微信“生成带参数的二维码”（createQRCode）接口。
//
// 官方文档：https://developers.weixin.qq.com/doc/service/api/qrcode/qrcodes/api_createqrcode.html
//
// 业务背景（新接手同事先读这段）：
//   - 微信二维码分两类：
//       临时二维码：QR_SCENE / QR_STR_SCENE，必须带 expire_seconds，最长 30 天，数量不限；
//       永久二维码：QR_LIMIT_SCENE / QR_LIMIT_STR_SCENE，无过期时间，但同一账号最多 10 万个。
//     本文件同时提供两套方法：
//       CreatePermanentQRCode → 正式环境使用（永久码，不发送 expire_seconds）；
//       CreateTemporaryQRCode → 联调测试使用（临时码，默认 600 秒即 10 分钟过期）。
//     两者共用同一套“创建 ticket + ticket 换图片”流程，只是请求体不同，因此不会出现两套重复实现。
//     注意：临时二维码的请求体必须带 expire_seconds，永久二维码绝不能再带该字段。
//   - 用户扫码后的行为由微信决定：未关注则引导关注并回推带场景值的关注事件；已关注则直接
//     进入会话并回推带场景值的扫码事件。场景值 scene_id / scene_str 就是业务侧区分用户来
//     源的标识，本文件只负责把场景值原样提交给微信，具体含义由调用方定义。
//   - access_token 必须已经在调用本文件前拿到（见 wechat_access_token.go），本文件只接收它
//     作为参数，不负责获取/缓存凭据。
//
// 完整调用链（两步）：
//  1. POST https://api.weixin.qq.com/cgi-bin/qrcode/create?access_token=xxx
//     请求体为 JSON，成功后返回 ticket 与 url。
//  2. GET  https://mp.weixin.qq.com/cgi-bin/showqrcode?ticket=xxx
//     用 ticket 换取真正的二维码图片，ticket 需要 URL 编码（本文件用 url.Values 自动处理）。
//     该换取接口无须登录态，注意它的域名与第 1 步不同。

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	// wechatQRCodeImageHost 换取二维码图片的域名 mp.weixin.qq.com，
	// 与创建二维码接口的 api.weixin.qq.com 不是同一个域名，不要混用。
	wechatQRCodeImageHost = "https://mp.weixin.qq.com"
	// wechatQRCodeCreatePath 创建二维码接口路径，host 使用 wechatOpenAPIHost（见 access_token 文件）。
	wechatQRCodeCreatePath = "/cgi-bin/qrcode/create"
	// wechatQRCodeShowPath 用 ticket 换取图片的接口路径。
	wechatQRCodeShowPath = "/cgi-bin/showqrcode"
	// wechatQRCodeTimeout 单次 HTTP 请求超时，覆盖“创建二维码”和“换图片”两步。
	wechatQRCodeTimeout = 15 * time.Second

	// 四种 action_name：
	//   临时：QR_SCENE（整型 scene_id）/ QR_STR_SCENE（字符串 scene_str）；
	//   永久：QR_LIMIT_SCENE（整型 scene_id）/ QR_LIMIT_STR_SCENE（字符串 scene_str）。
	wechatQRCodeActionScene         = "QR_SCENE"
	wechatQRCodeActionStrScene      = "QR_STR_SCENE"
	wechatQRCodeActionLimitScene    = "QR_LIMIT_SCENE"
	wechatQRCodeActionLimitStrScene = "QR_LIMIT_STR_SCENE"

	// wechatTemporaryQRCodeExpireSeconds 临时二维码默认有效期，600 秒 = 10 分钟。
	// 仅供联调测试使用；正式环境的永久二维码不设置过期时间。
	wechatTemporaryQRCodeExpireSeconds = 600
	// wechatQRCodeExpireSecondsMax 临时二维码有效期上限，2592000 秒 = 30 天（微信限制）。
	wechatQRCodeExpireSecondsMax = 2592000

	// 场景值范围限制，来自微信文档“请求参数”一节：
	//   scene_id 永久二维码最大 100000，且只支持 1--100000；临时二维码为 32 位非 0 整型；
	//   scene_str 字符串长度限制为 1 到 64。
	wechatQRCodeSceneIDMaxValue = 100000
	wechatQRCodeSceneStrMaxLen  = 64
)

// WechatPermanentQRCodeRequest 描述要生成的永久二维码场景值。
//
// 两个字段二选一：
//   - SceneStr 非空时优先生效，生成 QR_LIMIT_STR_SCENE（字符串场景）；
//   - 否则使用 SceneID，生成 QR_LIMIT_SCENE（整型场景）。
//
// SceneID 用指针，是为了区分“未传场景ID”和“传了 0”两种情况：nil 表示未传，
// 非 nil 才进入整型场景分支（0 会被范围校验拦下）。
type WechatPermanentQRCodeRequest struct {
	SceneStr string
	SceneID  *int
}

// WechatTemporaryQRCodeRequest 描述要生成的临时二维码场景值。
//
// 字段含义与永久二维码一致，额外多一个有效期：
//   - ExpireSeconds 为 nil 时使用默认值 wechatTemporaryQRCodeExpireSeconds（600 秒）。
//     用指针是为了区分“未指定”和“显式指定”，避免 0 被当成有效值传给微信。
type WechatTemporaryQRCodeRequest struct {
	SceneStr      string
	SceneID       *int
	ExpireSeconds *int64
}

// WechatPermanentQRCode 是永久二维码的生成结果。
//
// 字段说明：
//   - Ticket       创建二维码接口返回的票据，可用于后续重新换取图片。
//   - URL          微信返回的二维码解析地址（含 ticket），可直接给开发者使用。
//   - Image        通过 ticket 换取的二维码图片二进制内容（JPEG）。
//   - ImageMimeType 图片 MIME 类型，取自换取图片接口的 Content-Type，缺省为 image/jpeg。
type WechatPermanentQRCode struct {
	Ticket        string
	URL           string
	Image         []byte
	ImageMimeType string
}

// WechatTemporaryQRCode 是临时二维码的生成结果，比永久二维码多一个实际有效期。
//
// 字段说明：
//   - Ticket        创建二维码接口返回的票据。
//   - URL           微信返回的二维码解析地址（含 ticket）。
//   - ExpireSeconds 本次二维码的实际有效期（秒），等于请求值或默认 600。
//   - Image         通过 ticket 换取的二维码图片二进制内容（JPEG）。
//   - ImageMimeType 图片 MIME 类型，缺省为 image/jpeg。
type WechatTemporaryQRCode struct {
	Ticket        string
	URL           string
	ExpireSeconds int64
	Image         []byte
	ImageMimeType string
}

// WechatQRCodeError 表示微信返回的业务错误（HTTP 200 但 errcode 非 0）。
// 常见错误码：40001 access_token 无效；40052 action_name 有误；40053 action_info 有误。
type WechatQRCodeError struct {
	ErrCode int
	ErrMsg  string
}

func (e *WechatQRCodeError) Error() string {
	return fmt.Sprintf("wechat create qrcode failed: errcode=%d errmsg=%s", e.ErrCode, e.ErrMsg)
}

// WechatQRCodeService 封装微信“生成带参数的二维码”接口。
//
// 字段说明：
//   - httpClient 可注入替换（测试、代理、复用连接池），为空时按超时兜底创建。
//   - host       创建二维码接口域名，默认 wechatOpenAPIHost。
//   - imageHost  换取图片接口域名，默认 wechatQRCodeImageHost。
type WechatQRCodeService struct {
	httpClient *http.Client
	host       string
	imageHost  string
}

// NewWechatQRCodeService 创建服务实例。access_token 不在这里配置，
// 而是在调用 CreatePermanentQRCode / CreateTemporaryQRCode 时由调用方传入
// （因为凭据由 access_token 文件负责获取）。
func NewWechatQRCodeService() *WechatQRCodeService {
	return &WechatQRCodeService{
		httpClient: &http.Client{Timeout: wechatQRCodeTimeout},
		host:       wechatOpenAPIHost,
		imageHost:  wechatQRCodeImageHost,
	}
}

// SetHTTPClient 注入自定义 HTTP 客户端，便于测试或复用已有客户端配置。
func (s *WechatQRCodeService) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.httpClient = client
	}
}

// SetHost 覆盖创建二维码接口域名，默认使用正式环境域名。
func (s *WechatQRCodeService) SetHost(host string) {
	if host != "" {
		s.host = host
	}
}

// SetImageHost 覆盖换取二维码图片的域名，默认 mp.weixin.qq.com。
func (s *WechatQRCodeService) SetImageHost(host string) {
	if host != "" {
		s.imageHost = host
	}
}

// 以下三个结构体对应微信 createQRCode 的请求体结构：
//
//	{
//	  "expire_seconds": 600,
//	  "action_name": "QR_STR_SCENE",
//	  "action_info": { "scene": { "scene_str": "test" } }
//	}
//
// SceneID / SceneStr 都带 omitempty，保证最终 JSON 里只出现实际使用的那一个字段；
// ExpireSeconds 也带 omitempty，永久二维码不设置该字段时不会出现在请求体里。

// wechatQRCodeScene 二维码场景信息，scene_id 与 scene_str 二选一。
type wechatQRCodeScene struct {
	SceneID  *int   `json:"scene_id,omitempty"`
	SceneStr string `json:"scene_str,omitempty"`
}

// wechatQRCodeActionInfo 二维码详细信息，目前只用到 scene。
type wechatQRCodeActionInfo struct {
	Scene wechatQRCodeScene `json:"scene"`
}

// wechatQRCodeCreateRequest createQRCode 的请求体。
// 临时二维码必填 expire_seconds，永久二维码不发送该字段。
type wechatQRCodeCreateRequest struct {
	ExpireSeconds *int64                 `json:"expire_seconds,omitempty"`
	ActionName    string                 `json:"action_name"`
	ActionInfo    wechatQRCodeActionInfo `json:"action_info"`
}

// CreatePermanentQRCode 创建永久二维码，并用返回的 ticket 换取二维码图片。
//
// 参数：
//   - accessToken 必填，由“获取接口调用凭据”接口获取。
//   - request     必填，提供 SceneStr 或 SceneID。
//
// 返回：包含 ticket、url 与图片内容的 WechatPermanentQRCode。
//
// 流程：
//  1. 本地校验 accessToken 与场景值，构造永久二维码请求体；
//  2. POST host + /cgi-bin/qrcode/create?access_token=xxx；
//  3. 解析响应，先判 errcode，再校验 ticket 非空；
//  4. 调 FetchQRCodeImage 用 ticket 换取图片，一并返回。
func (s *WechatQRCodeService) CreatePermanentQRCode(ctx context.Context, accessToken string, request *WechatPermanentQRCodeRequest) (*WechatPermanentQRCode, error) {
	createBody, err := buildWechatPermanentQRCodeBody(request)
	if err != nil {
		return nil, err
	}
	ticket, qrURL, err := s.createQRCodeTicket(ctx, accessToken, createBody)
	if err != nil {
		return nil, err
	}
	image, mimeType, err := s.FetchQRCodeImage(ctx, ticket)
	if err != nil {
		return nil, err
	}
	return &WechatPermanentQRCode{
		Ticket:        ticket,
		URL:           qrURL,
		Image:         image,
		ImageMimeType: mimeType,
	}, nil
}

// CreateTemporaryQRCode 创建临时二维码（默认 10 分钟过期），并用返回的 ticket 换取图片。
//
// 仅用于联调测试：正式环境应使用 CreatePermanentQRCode。
//
// 参数：
//   - accessToken 必填，由“获取接口调用凭据”接口获取。
//   - request     必填，提供 SceneStr 或 SceneID；ExpireSeconds 为空时默认 600 秒。
//
// 返回：包含 ticket、url、实际有效期与图片内容的 WechatTemporaryQRCode。
func (s *WechatQRCodeService) CreateTemporaryQRCode(ctx context.Context, accessToken string, request *WechatTemporaryQRCodeRequest) (*WechatTemporaryQRCode, error) {
	createBody, expireSeconds, err := buildWechatTemporaryQRCodeBody(request)
	if err != nil {
		return nil, err
	}
	ticket, qrURL, err := s.createQRCodeTicket(ctx, accessToken, createBody)
	if err != nil {
		return nil, err
	}
	image, mimeType, err := s.FetchQRCodeImage(ctx, ticket)
	if err != nil {
		return nil, err
	}
	return &WechatTemporaryQRCode{
		Ticket:        ticket,
		URL:           qrURL,
		ExpireSeconds: expireSeconds,
		Image:         image,
		ImageMimeType: mimeType,
	}, nil
}

// createQRCodeTicket 是永久/临时二维码共用的第一步：向微信申请 ticket。
//
// 两种二维码只有请求体不同，发送与解析流程完全一致，因此抽到这里避免重复实现。
// 返回 ticket 与微信给出的二维码解析地址 url。
func (s *WechatQRCodeService) createQRCodeTicket(ctx context.Context, accessToken string, createBody *wechatQRCodeCreateRequest) (string, string, error) {
	if s == nil {
		return "", "", fmt.Errorf("wechat qrcode service is nil")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return "", "", fmt.Errorf("wechat access token is empty")
	}

	payload, err := common.Marshal(createBody)
	if err != nil {
		return "", "", err
	}

	host := s.host
	if host == "" {
		host = wechatOpenAPIHost
	}
	requestURL, err := url.Parse(host + wechatQRCodeCreatePath)
	if err != nil {
		return "", "", err
	}
	// access_token 走查询参数，用 url.Values 统一编码。
	query := requestURL.Query()
	query.Set("access_token", accessToken)
	requestURL.RawQuery = query.Encode()

	client := s.ensureHTTPClient()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Accept", "application/json")

	respBody, err := doWechatRequest(client, httpRequest)
	if err != nil {
		return "", "", err
	}

	// 成功返回 ticket/url，失败返回 errcode/errmsg，统一结构解析。
	var response struct {
		Ticket  string `json:"ticket"`
		URL     string `json:"url"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := common.Unmarshal(respBody, &response); err != nil {
		return "", "", err
	}
	if response.ErrCode != 0 {
		return "", "", &WechatQRCodeError{ErrCode: response.ErrCode, ErrMsg: response.ErrMsg}
	}
	if strings.TrimSpace(response.Ticket) == "" {
		return "", "", fmt.Errorf("wechat create qrcode returned empty ticket")
	}
	return response.Ticket, response.URL, nil
}

// FetchQRCodeImage 通过 ticket 换取二维码图片内容。
//
// 说明：
//   - 该接口无须登录态即可调用，ticket 需要 URL 编码（url.Values 会自动编码）。
//   - ticket 非法时微信返回 HTTP 404，因此非 2xx 直接报错。
//   - 返回图片 MIME 类型，缺省兜底为 image/jpeg（微信返回的就是 JPEG）。
func (s *WechatQRCodeService) FetchQRCodeImage(ctx context.Context, ticket string) ([]byte, string, error) {
	if s == nil {
		return nil, "", fmt.Errorf("wechat qrcode service is nil")
	}
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return nil, "", fmt.Errorf("wechat qrcode ticket is empty")
	}

	host := s.imageHost
	if host == "" {
		host = wechatQRCodeImageHost
	}
	requestURL, err := url.Parse(host + wechatQRCodeShowPath)
	if err != nil {
		return nil, "", err
	}
	query := requestURL.Query()
	query.Set("ticket", ticket)
	requestURL.RawQuery = query.Encode()

	client := s.ensureHTTPClient()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, "", err
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, "", err
	}
	defer httpResponse.Body.Close()

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, "", err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, "", fmt.Errorf("wechat show qrcode http status %d: %s", httpResponse.StatusCode, strings.TrimSpace(string(body)))
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("wechat show qrcode returned empty image")
	}

	mimeType := httpResponse.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	return body, mimeType, nil
}

// ensureHTTPClient 保证服务在真正发请求前有可用的 HTTP 客户端。
func (s *WechatQRCodeService) ensureHTTPClient() *http.Client {
	if s.httpClient == nil {
		s.httpClient = &http.Client{Timeout: wechatQRCodeTimeout}
	}
	return s.httpClient
}

// resolveWechatQRCodeScene 校验并返回场景值，永久/临时二维码共用。
//
// 规则（场景值二选一，scene_str 优先）：
//   - sceneStr 非空 → 返回 scene_str，limitToPermanentRange 时校验长度 1-64；
//   - 否则 sceneID 非 nil → 返回 scene_id，permanent 时范围 1-100000（微信永久码限制），
//     临时码时仅要求 32 位非 0 正整数；
//   - 两者都没有 → 报错，避免向微信发送无场景的无效请求。
func resolveWechatQRCodeScene(sceneStr string, sceneID *int, permanent bool) (wechatQRCodeScene, error) {
	trimmedSceneStr := strings.TrimSpace(sceneStr)
	if trimmedSceneStr != "" {
		if len(trimmedSceneStr) > wechatQRCodeSceneStrMaxLen {
			return wechatQRCodeScene{}, fmt.Errorf("wechat qrcode scene_str length must be between 1 and %d", wechatQRCodeSceneStrMaxLen)
		}
		return wechatQRCodeScene{SceneStr: trimmedSceneStr}, nil
	}

	if sceneID != nil {
		id := *sceneID
		if id <= 0 {
			return wechatQRCodeScene{}, fmt.Errorf("wechat qrcode scene_id must be a non-zero positive integer")
		}
		if permanent {
			if id > wechatQRCodeSceneIDMaxValue {
				return wechatQRCodeScene{}, fmt.Errorf("wechat qrcode scene_id must be between 1 and %d", wechatQRCodeSceneIDMaxValue)
			}
		} else if id > math.MaxInt32 {
			// 临时二维码的 scene_id 为 32 位非 0 整型。
			return wechatQRCodeScene{}, fmt.Errorf("wechat qrcode scene_id must be within 32-bit integer range")
		}
		return wechatQRCodeScene{SceneID: &id}, nil
	}

	return wechatQRCodeScene{}, fmt.Errorf("wechat qrcode requires scene_str or scene_id")
}

// buildWechatPermanentQRCodeBody 校验场景值并构造永久二维码请求体。
// 固定使用 QR_LIMIT_SCENE / QR_LIMIT_STR_SCENE，且不设置 expire_seconds。
func buildWechatPermanentQRCodeBody(request *WechatPermanentQRCodeRequest) (*wechatQRCodeCreateRequest, error) {
	if request == nil {
		return nil, fmt.Errorf("wechat qrcode request is nil")
	}
	scene, err := resolveWechatQRCodeScene(request.SceneStr, request.SceneID, true)
	if err != nil {
		return nil, err
	}
	actionName := wechatQRCodeActionLimitScene
	if scene.SceneStr != "" {
		actionName = wechatQRCodeActionLimitStrScene
	}
	return &wechatQRCodeCreateRequest{
		ActionName: actionName,
		ActionInfo: wechatQRCodeActionInfo{Scene: scene},
	}, nil
}

// buildWechatTemporaryQRCodeBody 校验场景值并构造临时二维码请求体。
// 固定使用 QR_SCENE / QR_STR_SCENE，必须带 expire_seconds（默认 600 秒）。
// 返回的第二个值是本次实际使用的有效期，便于调用方回显。
func buildWechatTemporaryQRCodeBody(request *WechatTemporaryQRCodeRequest) (*wechatQRCodeCreateRequest, int64, error) {
	if request == nil {
		return nil, 0, fmt.Errorf("wechat qrcode request is nil")
	}
	scene, err := resolveWechatQRCodeScene(request.SceneStr, request.SceneID, false)
	if err != nil {
		return nil, 0, err
	}

	expireSeconds := int64(wechatTemporaryQRCodeExpireSeconds)
	if request.ExpireSeconds != nil {
		expireSeconds = *request.ExpireSeconds
	}
	if expireSeconds <= 0 || expireSeconds > wechatQRCodeExpireSecondsMax {
		return nil, 0, fmt.Errorf("wechat temporary qrcode expire_seconds must be between 1 and %d", wechatQRCodeExpireSecondsMax)
	}

	actionName := wechatQRCodeActionScene
	if scene.SceneStr != "" {
		actionName = wechatQRCodeActionStrScene
	}
	return &wechatQRCodeCreateRequest{
		ExpireSeconds: &expireSeconds,
		ActionName:    actionName,
		ActionInfo:    wechatQRCodeActionInfo{Scene: scene},
	}, expireSeconds, nil
}

// doWechatRequest 是微信 JSON 接口的通用发送逻辑：发请求、读响应体、
// 把非 2xx 视为传输层错误并附带响应体，供调用方解析业务字段。
func doWechatRequest(client *http.Client, request *http.Request) ([]byte, error) {
	httpResponse, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, fmt.Errorf("wechat http status %d: %s", httpResponse.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// CreateWechatPermanentQRCode 使用默认服务实例创建永久二维码。
// 便捷入口，适合一次性调用；需要自定义域名/客户端时请自行创建服务。
func CreateWechatPermanentQRCode(ctx context.Context, accessToken string, request *WechatPermanentQRCodeRequest) (*WechatPermanentQRCode, error) {
	return NewWechatQRCodeService().CreatePermanentQRCode(ctx, accessToken, request)
}

// CreateWechatTemporaryQRCode 使用默认服务实例创建临时二维码。
// 便捷入口，适合一次性调用；需要自定义域名/客户端时请自行创建服务。
func CreateWechatTemporaryQRCode(ctx context.Context, accessToken string, request *WechatTemporaryQRCodeRequest) (*WechatTemporaryQRCode, error) {
	return NewWechatQRCodeService().CreateTemporaryQRCode(ctx, accessToken, request)
}
