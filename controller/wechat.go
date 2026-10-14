package controller

import (
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

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type wechatLoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    string `json:"data"`
	AppId   string `json:"app_id"`
}

func getWechatConfigForSite(providerId int) (*model.WechatOfficialConfig, error) {
	config, err := model.GetWechatConfigByProvider(providerId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("当前站点尚未配置微信公众号")
	}
	if err != nil {
		return nil, err
	}
	if providerId == 0 && !common.WeChatAuthEnabled {
		return nil, errors.New("管理员未开启通过微信登录以及注册")
	}
	if !config.Enabled || config.AppId == "" || config.AppSecret == "" || strings.TrimSpace(common.WeChatServerAddress) == "" {
		return nil, errors.New("当前站点尚未启用微信登录或配置不完整")
	}
	return config, nil
}

func getWechatLoginStatus(providerId int) (bool, string) {
	config, err := model.GetWechatConfigByProvider(providerId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			common.SysLog("failed to load site WeChat configuration: " + err.Error())
		}
		return false, ""
	}
	qrcodeURL := strings.TrimSpace(config.QRCodeURL)
	if providerId == 0 && qrcodeURL == "" {
		qrcodeURL = strings.TrimSpace(common.WeChatAccountQRCodeImageURL)
	}
	enabled := config.Enabled && config.AppId != "" && config.AppSecret != "" &&
		strings.TrimSpace(common.WeChatServerAddress) != "" && qrcodeURL != ""
	if providerId == 0 && !common.WeChatAuthEnabled {
		enabled = false
	}
	return enabled, qrcodeURL
}

func getWeChatIdByCode(code string, config *model.WechatOfficialConfig) (string, error) {
	if code == "" {
		return "", errors.New("无效的参数")
	}
	if config == nil || config.AppId == "" || config.AppSecret == "" {
		return "", errors.New("微信公众号配置不完整")
	}
	endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(common.WeChatServerAddress), "/") + "/api/wechat/user")
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
	req.Header.Set("Authorization", common.WeChatServerToken)
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
	user := model.User{
		WeChatId:   wechatId,
		ProviderId: providerId,
		// 微信登录不经过统一 OAuth state，需直接从本次请求接收并校验注册时区。
		Timezone: normalizeRegistrationTimezone(c.Query("timezone")),
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
			user.Username = "wechat_" + strconv.Itoa(model.GetMaxUserId()+1)
			user.DisplayName = "WeChat User"
			user.Role = common.RoleCommonUser
			user.Status = common.UserStatusEnabled
			user.RegisterIp = getRegistrationIP(c)
			if !ensureGlobalUserIdentityAvailable(c, 0, user.Username, user.Email) {
				return
			}

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
