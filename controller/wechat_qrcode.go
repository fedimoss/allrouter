package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// wechatQRCodeRequest 是生成二维码接口的统一请求体，永久/临时两种模式共用。
type wechatQRCodeRequest struct {
	// SceneStr 字符串场景值，长度 1-64；与 SceneID 二选一，优先使用。
	SceneStr string `json:"scene_str"`
	// SceneID 整型场景值：永久二维码范围 1-100000，临时二维码为 32 位非 0 整型。
	SceneID *int `json:"scene_id"`
	// AccessToken 可选：显式指定调用凭据；为空时自动通过“获取接口调用凭据”接口获取。
	AccessToken string `json:"access_token"`
}

// wechatQRCodePlaceholderSceneStr 是 scene_str 的占位值。
// 业务尚未确定真实场景值来源，先用它保证接口可调用；后续确定后替换此处，
// 或在请求里显式传入 scene_str 覆盖。永久二维码数量有限（最多 10 万个），
// 占位值仅用于联调，不要在正式业务里长期使用。
const wechatQRCodePlaceholderSceneStr = "xxxxxxxxxxxxxxxxxxx"

// canCreateWechatQRCode 与 UpdatePricingSort 的鉴权一致：
// 主站管理员（role>=管理员）或分站站长（provider owner）可调用。
func canCreateWechatQRCode(c *gin.Context) bool {
	if c.GetInt("role") >= common.RoleAdminUser {
		return true
	}
	return model.IsProviderOwner(c.GetInt("id"))
}

// CreateWechatPermanentQRCode 生成永久二维码（QR_LIMIT_SCENE / QR_LIMIT_STR_SCENE）。
//
// 正式环境使用该方法：二维码无过期时间，但同一公众号最多 10 万个。
func CreateWechatPermanentQRCode(c *gin.Context) {
	request, _, accessToken, ok := prepareWechatQRCodeCall(c)
	if !ok {
		return
	}

	// 生成永久二维码
	result, err := service.NewWechatQRCodeService().CreatePermanentQRCode(
		c.Request.Context(),
		accessToken,
		&service.WechatPermanentQRCodeRequest{
			SceneStr: request.SceneStr,
			SceneID:  request.SceneID,
		},
	)

	// 处理生成结果
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 返回成功响应
	common.ApiSuccess(c, gin.H{
		"ticket":          result.Ticket,
		"url":             result.URL,
		"image_mime_type": result.ImageMimeType,
		"image_base64":    base64.StdEncoding.EncodeToString(result.Image),
	})
}

// CreateWechatTemporaryQRCode 生成临时二维码（QR_SCENE / QR_STR_SCENE），默认 10 分钟过期。
//
// 仅用于联调测试。临时二维码数量不限，适合反复调试；正式环境请改用永久二维码。
func CreateWechatTemporaryQRCode(c *gin.Context) {
	request, _, accessToken, ok := prepareWechatQRCodeCall(c)
	if !ok {
		return
	}

	// 生成临时二维码
	result, err := service.NewWechatQRCodeService().CreateTemporaryQRCode(
		c.Request.Context(),
		accessToken,
		&service.WechatTemporaryQRCodeRequest{
			SceneStr: request.SceneStr,
			SceneID:  request.SceneID,
		},
	)

	// 处理生成结果
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 返回成功响应
	common.ApiSuccess(c, gin.H{
		"ticket":          result.Ticket,
		"url":             result.URL,
		"expire_seconds":  result.ExpireSeconds,
		"image_mime_type": result.ImageMimeType,
		"image_base64":    base64.StdEncoding.EncodeToString(result.Image),
	})
}

// prepareWechatQRCodeCall 完成永久/临时二维码两个入口的公共前置步骤：
// 鉴权、解析请求、准备 access_token。任一步失败时已写出响应并返回 ok=false。
func prepareWechatQRCodeCall(c *gin.Context) (*wechatQRCodeRequest, *model.WechatOfficialConfig, string, bool) {
	if !canCreateWechatQRCode(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权限生成二维码",
		})
		return nil, nil, "", false
	}
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	config, err := model.GetWechatConfigByProvider(providerId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "请先保存微信公众号 AppId 和 AppSecret")
		} else {
			common.ApiError(c, err)
		}
		return nil, nil, "", false
	}
	if strings.TrimSpace(config.AppId) == "" || strings.TrimSpace(config.AppSecret) == "" {
		common.ApiErrorMsg(c, "请先填写完整的微信公众号 AppId 和 AppSecret")
		return nil, nil, "", false
	}

	// 解析请求参数
	request, ok := parseWechatQRCodeRequest(c)
	if !ok {
		return nil, nil, "", false
	}

	// 准备 access_token
	accessToken := strings.TrimSpace(request.AccessToken)
	if accessToken == "" {
		accessTokenService := service.NewWechatAccessTokenService()
		accessTokenService.SetCredentials(config.AppId, config.AppSecret)
		token, err := accessTokenService.GetAccessToken(c.Request.Context())
		if err != nil {
			common.ApiError(c, err)
			return nil, nil, "", false
		}
		accessToken = token.AccessToken
	}
	return request, config, accessToken, true
}

func persistWechatQRCode(config *model.WechatOfficialConfig, result *service.WechatPermanentQRCode) error {
	if config == nil || result == nil || len(result.Image) == 0 {
		return errors.New("微信二维码生成结果为空")
	}
	mimeType := strings.TrimSpace(result.ImageMimeType)
	if separator := strings.IndexByte(mimeType, ';'); separator >= 0 {
		mimeType = strings.TrimSpace(mimeType[:separator])
	}
	if mimeType == "" {
		mimeType = "image/jpeg"
	} else if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return errors.New("微信二维码图片响应类型无效")
	}
	qrcodeURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(result.Image)
	if err := model.UpdateWechatConfigQRCode(config.ProviderId, qrcodeURL); err != nil {
		return err
	}
	config.QRCodeURL = qrcodeURL
	return nil
}

func getGeneratedWechatQRCodeURL(qrcodeURL string) string {
	qrcodeURL = strings.TrimSpace(qrcodeURL)
	if !strings.HasPrefix(strings.ToLower(qrcodeURL), "data:image/") {
		return ""
	}
	return qrcodeURL
}

func createWechatPermanentQRCodeForConfig(ctx context.Context, config *model.WechatOfficialConfig, scene string) (*service.WechatPermanentQRCode, error) {
	if config == nil || strings.TrimSpace(config.AppId) == "" || strings.TrimSpace(config.AppSecret) == "" {
		return nil, errors.New("微信公众号配置不完整")
	}
	accessTokenService := service.NewWechatAccessTokenService()
	accessTokenService.SetCredentials(config.AppId, config.AppSecret)
	accessToken, err := accessTokenService.GetAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	return service.CreateWechatPermanentQRCode(ctx, accessToken.AccessToken, &service.WechatPermanentQRCodeRequest{SceneStr: scene})
}

// parseWechatQRCodeRequest 解析请求参数：以 JSON 请求体为主，
// 同时允许通过查询参数 scene_str / scene_id 传入，便于直接测试。
//
// 场景值处理：
//   - 显式传 scene_str（body 或 query）→ 使用字符串场景值；
//   - 否则传 scene_id → 使用整型场景值；
//   - 两者都没传 → 回退到占位 scene_str，保证接口在业务参数未定前也能直接调用联调。
//
// 具体生成永久码还是临时码，由调用方选择的服务方法决定（见两个入口函数）。
func parseWechatQRCodeRequest(c *gin.Context) (*wechatQRCodeRequest, bool) {
	var request wechatQRCodeRequest
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ApiErrorMsg(c, "读取请求体失败")
		return nil, false
	}
	if len(strings.TrimSpace(string(rawBody))) > 0 {
		if err := common.Unmarshal(rawBody, &request); err != nil {
			common.ApiErrorMsg(c, "无效的请求参数")
			return nil, false
		}
	}

	// 解析查询参数
	if querySceneStr := strings.TrimSpace(c.Query("scene_str")); querySceneStr != "" {
		request.SceneStr = querySceneStr
	}

	// 解析查询参数 scene_id
	if querySceneID := strings.TrimSpace(c.Query("scene_id")); querySceneID != "" {
		sceneID, err := strconv.Atoi(querySceneID)
		if err != nil {
			common.ApiErrorMsg(c, "scene_id 必须是整数")
			return nil, false
		}
		request.SceneID = &sceneID
	}

	request.SceneStr = strings.TrimSpace(request.SceneStr)
	if request.SceneStr == "" && request.SceneID == nil {
		// 未显式提供场景值时回退到占位值。
		request.SceneStr = wechatQRCodePlaceholderSceneStr
	}
	return &request, true
}
