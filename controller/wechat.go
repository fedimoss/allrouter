package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type wechatLoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
	AppId   string `json:"app_id"`
}

const wechatLoginSessionTTLSeconds = int64(300)

type wechatLoginPollRequest struct {
	State    string `json:"state"`
	Timezone string `json:"timezone"`
}

func getWechatConfigForSite(providerId int) (*model.WechatOfficialConfig, error) {
	config, err := model.GetWechatConfigByProvider(providerId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("当前站点尚未配置微信公众号")
	}
	if err != nil {
		return nil, err
	}
	if !config.Enabled || config.AppId == "" || config.AppSecret == "" {
		return nil, errors.New("当前站点尚未启用微信登录或配置不完整")
	}
	return config, nil
}

func getWechatServerAddress(config *model.WechatOfficialConfig) string {
	if config == nil {
		return ""
	}
	address := strings.TrimSpace(config.ServerAddress)
	if address == "" && config.ProviderId == 0 {
		address = strings.TrimSpace(common.WeChatServerAddress)
	}
	return address
}

func getWechatServerToken(config *model.WechatOfficialConfig) string {
	if config == nil {
		return ""
	}
	token := strings.TrimSpace(config.ServerToken)
	if token == "" && config.ProviderId == 0 {
		token = strings.TrimSpace(common.WeChatServerToken)
	}
	return token
}

func isWechatLoginConfigReady(config *model.WechatOfficialConfig) bool {
	return config != nil &&
		config.Enabled &&
		strings.TrimSpace(config.AppId) != "" &&
		strings.TrimSpace(config.AppSecret) != ""
}

func getWechatLoginStatus(providerId int) (bool, string) {
	config, err := model.GetWechatConfigByProvider(providerId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog("failed to load site WeChat configuration: " + err.Error())
		}
		return false, ""
	}
	qrcodeURL := getGeneratedWechatQRCodeURL(config.QRCodeURL)
	enabled := isWechatLoginConfigReady(config)
	return enabled, qrcodeURL
}

func getWeChatIdByCode(code string, config *model.WechatOfficialConfig) (string, error) {
	if code == "" {
		return "", errors.New("无效的参数")
	}
	if config == nil || config.AppId == "" || config.AppSecret == "" {
		return "", errors.New("微信公众号配置不完整")
	}
	serverAddress := getWechatServerAddress(config)
	serverToken := getWechatServerToken(config)
	// Keep old main-site installations working until their site-level server
	// settings are migrated. A provider must always use its own server.
	if serverAddress == "" {
		return "", errors.New("当前站点未配置微信登录服务地址")
	}
	endpoint, err := url.Parse(strings.TrimRight(serverAddress, "/") + "/api/wechat/user")
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("code", code)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", serverToken)
	req.Header.Set("X-WeChat-App-Id", config.AppId)
	req.Header.Set("X-WeChat-App-Secret", config.AppSecret)
	req.Header.Set("X-WeChat-Provider-Id", strconv.Itoa(config.ProviderId))
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	httpResponse, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("WeChat Server returned HTTP %d", httpResponse.StatusCode)
	}
	var res wechatLoginResponse
	err = common.DecodeJson(httpResponse.Body, &res)
	if err != nil {
		return "", err
	}
	if !res.Success {
		return "", errors.New(res.Message)
	}
	if res.Data == "" {
		return "", errors.New("验证码错误或已过期")
	}
	if res.AppId != "" && res.AppId != config.AppId {
		return "", errors.New("验证码所属公众号与当前站点配置不匹配")
	}
	return res.Data, nil
}

// getWechatOpenIDByOAuthCode exchanges a code issued by WeChat's official
// account web authorization endpoint. It deliberately uses the credentials
// loaded for the current site; the legacy WeChat Server is not involved.
func getWechatOpenIDByOAuthCode(ctx context.Context, code string, config *model.WechatOfficialConfig) (string, error) {
	if config == nil || strings.TrimSpace(config.AppId) == "" || strings.TrimSpace(config.AppSecret) == "" {
		return "", errors.New("微信公众号配置不完整")
	}
	token, err := service.NewWechatOAuth2Service().ExchangeCodeForToken(
		ctx,
		config.AppId,
		config.AppSecret,
		code,
	)
	if err != nil {
		return "", err
	}
	if token.IsSnapshotUser == 1 {
		return "", errors.New("当前微信授权来自快照模式，无法用于登录")
	}
	return token.OpenID, nil
}

// WeChatLoginQR creates a short-lived, site-scoped state and returns the
// official-account web authorization URL to encode as a QR code in the UI.
func WeChatLoginQR(c *gin.Context) {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	config, err := getWechatConfigForSite(providerId)
	if err != nil || !isWechatLoginConfigReady(config) {
		common.ApiErrorMsg(c, "当前站点尚未启用微信登录或配置不完整")
		return
	}

	baseURL := strings.TrimRight(getRequestBaseURLForEmail(c), "/")
	callbackURL := baseURL + "/api/oauth/wechat/callback"
	parsedCallback, err := url.Parse(callbackURL)
	if err != nil || parsedCallback.Scheme != "https" || parsedCallback.Host == "" {
		common.ApiErrorMsg(c, "微信网页登录回调地址必须使用 HTTPS，请检查本站域名配置")
		return
	}

	state, err := common.GenerateRandomCharsKey(48)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	authorizeURL, err := service.BuildWechatOAuth2AuthorizeURL(config.AppId, callbackURL, "snsapi_base", state)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	expiresAt := common.GetTimestamp() + wechatLoginSessionTTLSeconds
	if err := model.CreateWechatLoginSession(providerId, state, expiresAt); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"state":         state,
		"authorize_url": authorizeURL,
		"expires_in":    wechatLoginSessionTTLSeconds,
	})
}

// WeChatLoginCallback completes the mobile WeChat authorization and marks
// the desktop browser's one-time login state as ready to consume.
func WeChatLoginCallback(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	code := strings.TrimSpace(c.Query("code"))
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	if state == "" || code == "" {
		c.Redirect(http.StatusFound, "/login?wechat=expired")
		return
	}

	loginSession, err := model.GetWechatLoginSession(providerId, state)
	if err != nil || loginSession.Status != model.WechatLoginSessionPending || loginSession.ExpiresAt <= common.GetTimestamp() {
		c.Redirect(http.StatusFound, "/login?wechat=expired")
		return
	}
	config, err := getWechatConfigForSite(providerId)
	if err != nil {
		c.Redirect(http.StatusFound, "/login?wechat=error")
		return
	}
	openID, err := getWechatOpenIDByOAuthCode(c.Request.Context(), code, config)
	if err != nil {
		common.SysLog("wechat OAuth callback failed: " + err.Error())
		c.Redirect(http.StatusFound, "/login?wechat=error")
		return
	}
	updated, err := model.ScanWechatLoginSession(providerId, state, openID, common.GetTimestamp())
	if err != nil || !updated {
		if err != nil {
			common.SysLog("wechat OAuth session update failed: " + err.Error())
		}
		c.Redirect(http.StatusFound, "/login?wechat=expired")
		return
	}
	c.Redirect(http.StatusFound, "/wechat/login/confirm?state="+url.QueryEscape(state))
}

func WeChatLoginConfirm(c *gin.Context) {
	var req struct {
		State string `json:"state"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || strings.TrimSpace(req.State) == "" {
		common.ApiErrorMsg(c, "无效的微信登录状态")
		return
	}
	confirmed, err := model.ConfirmWechatLoginSession(
		common.GetContextKeyInt(c, constant.ContextKeyProviderId),
		req.State,
		common.GetTimestamp(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !confirmed {
		common.ApiErrorMsg(c, "微信登录二维码已过期，请重新获取")
		return
	}
	common.ApiSuccess(c, nil)
}

// WeChatLoginPoll consumes an authorized state on the desktop browser and
// establishes the normal site session there.
func WeChatLoginPoll(c *gin.Context) {
	var req wechatLoginPollRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil || strings.TrimSpace(req.State) == "" {
		common.ApiErrorMsg(c, "无效的微信登录状态")
		return
	}
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	status, err := model.GetWechatLoginSessionStatus(providerId, req.State, common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if status != model.WechatLoginSessionAuthorized {
		common.ApiSuccess(c, gin.H{"status": status})
		return
	}
	openID, found, err := model.ConsumeWechatLoginSession(providerId, req.State, common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !found {
		common.ApiSuccess(c, gin.H{"status": model.WechatLoginSessionConsumed})
		return
	}
	completeWechatLogin(c, providerId, openID, req.Timezone)
}

func WeChatAuth(c *gin.Context) {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	config, err := getWechatConfigForSite(providerId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	code := c.Query("code")
	wechatId, err := getWeChatIdByCode(code, config)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	completeWechatLogin(c, providerId, wechatId, c.Query("timezone"))
}

func completeWechatLogin(c *gin.Context, providerId int, wechatId string, timezone string) {
	user := model.User{
		WeChatId:   wechatId,
		ProviderId: providerId,
		// 微信登录不经过统一 OAuth state，需直接从本次请求接收并校验注册时区。
		Timezone: normalizeRegistrationTimezone(timezone),
	}
	if model.IsWeChatIdAlreadyTakenInProvider(providerId, wechatId) {
		if err := user.FillUserByWeChatIdInProvider(providerId, wechatId); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "用户已注销",
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		if user.Id == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "用户已注销",
			})
			return
		}
	} else {
		if common.RegisterEnabled {
			username, displayName, err := newWechatUserIdentity()
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
			user.Username = username
			user.DisplayName = displayName
			user.Role = common.RoleCommonUser
			user.Status = common.UserStatusEnabled
			user.RegisterIp = getRegistrationIP(c)

			if err := user.Insert(0); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
		} else {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "管理员关闭了新用户注册",
			})
			return
		}
	}

	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusOK, gin.H{
			"message": "用户已被封禁",
			"success": false,
		})
		return
	}
	setupLogin(&user, c, false)
}

// 微信新建用户的默认值。
const (
	// wechatDefaultDisplayName 微信静默授权（snsapi_base）只能拿到 openid，没有上游昵称可用，
	// 因此 display_name 固定为该值，与其它 OAuth 渠道「Provider Name + User」的兜底形式一致。
	wechatDefaultDisplayName = "WeChat User"
	// wechatUsernameRandomLen 兜底随机用户名中随机部分的长度：前缀 7 + 随机 10 = 17，处于用户名上限 20 以内。
	wechatUsernameRandomLen = 10
	// wechatUsernameRandomAttempts 兜底随机用户名的最大尝试次数，避免极端情况下死循环。
	wechatUsernameRandomAttempts = 5
)

// newWechatUserIdentity 生成微信新建用户的 username 与 display_name，并保证用户名可用。
// 两个扫码入口（/api/oauth/wechat/qr 与 /wechat/scan）共用这一套命名。
//
// 首选「wechat_ + 即将插入的这条记录的 ID」：ID 由数据库在插入时分配，这里用当前表内
// 最大 ID + 1 预测（GetMaxUserId 含软删除用户，保证这个值不会被已用过的号覆盖）。
// 若该名字已被人工占用（用户自填或管理员改名），退化为「wechat_ + 随机串」，
// 避免整个扫码注册入口被这一个名字卡死。
func newWechatUserIdentity() (string, string, error) {
	preferred := "wechat_" + strconv.Itoa(model.GetMaxUserId()+1)
	conflict, _, err := model.UserIdentityConflictFieldsGlobally(0, preferred, "")
	if err != nil {
		return "", "", err
	}
	if !conflict {
		return preferred, wechatDefaultDisplayName, nil
	}

	// 用户名在库中是全局唯一（UserIdentityConflictFieldsGlobally 不带 provider），必须生成后校验。
	for i := 0; i < wechatUsernameRandomAttempts; i++ {
		candidate := "wechat_" + common.GetRandomString(wechatUsernameRandomLen)
		conflict, _, err := model.UserIdentityConflictFieldsGlobally(0, candidate, "")
		if err != nil {
			return "", "", err
		}
		if !conflict {
			return candidate, wechatDefaultDisplayName, nil
		}
	}
	return "", "", errors.New("生成微信用户名失败，请稍后重试")
}

type wechatBindRequest struct {
	Code string `json:"code"`
}

func isWechatProviderOwner(c *gin.Context, providerId int, userId int) bool {
	if providerId <= 0 || userId <= 0 {
		return false
	}
	if ownerUserId := common.GetContextKeyInt(c, constant.ContextKeyProviderOwnerUserId); ownerUserId > 0 {
		return ownerUserId == userId
	}
	provider, err := model.GetProviderById(providerId)
	return err == nil && provider.OwnerUserId == userId
}

func WeChatBind(c *gin.Context) {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	config, err := getWechatConfigForSite(providerId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "请先登录"})
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	providerOwner := isWechatProviderOwner(c, providerId, userId)
	if user.ProviderId != providerId && !providerOwner {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "当前用户不属于该站点"})
		return
	}
	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "用户已被封禁"})
		return
	}
	var req wechatBindRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的请求",
		})
		return
	}
	code := req.Code
	wechatId, err := getWeChatIdByCode(code, config)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	wechatIdTaken, err := model.IsWeChatIdAlreadyTakenByOtherInProvider(providerId, wechatId, user.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if wechatIdTaken {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该微信账号已被绑定",
		})
		return
	}
	if providerOwner && user.ProviderId != providerId {
		err = model.SaveWechatOfficialIdentity(providerId, user.Id, wechatId)
	} else {
		user.WeChatId = wechatId
		err = user.Update(false)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}
