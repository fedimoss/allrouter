package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type telegramSetWebhookRequest struct {
	BotToken     string `json:"bot_token"`
	CpolarDomain string `json:"cpolar_domain"`
	SecretToken  string `json:"secret_token"`
}

type telegramTokenRequest struct {
	BotToken string `json:"bot_token"`
}

// callTelegramAPI 代理调用 Telegram Bot API，结果以 raw JSON []byte 返回。
func callTelegramAPI(method, botToken string, formValues url.Values) ([]byte, error) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/%s", botToken, method)

	var resp *http.Response
	var err error

	if formValues != nil {
		resp, err = http.PostForm(apiURL, formValues)
	} else {
		resp, err = http.Get(apiURL)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// resolveToken 优先用请求里传来的 bot_token，为空时回退到 DB 里配置的 Token。
func resolveToken(reqToken string) string {
	if t := strings.TrimSpace(reqToken); t != "" {
		return t
	}
	return common.TelegramBotToken
}

// SetTelegramWebhook 由管理员在后台调用，向 Telegram 注册/更新 webhook。
func SetTelegramWebhook(c *gin.Context) {
	var req telegramSetWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数无效"})
		return
	}

	domain := strings.TrimPrefix(req.CpolarDomain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	webhookURL := fmt.Sprintf("https://%s/api/telegram/webhook", domain)
	form := url.Values{}
	form.Set("url", webhookURL)
	if req.SecretToken != "" {
		form.Set("secret_token", req.SecretToken)
	}

	body, err := callTelegramAPI("setWebhook", resolveToken(req.BotToken), form)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("请求 Telegram API 失败: %v", err)})
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}

// DeleteTelegramWebhook 由管理员在后台调用，删除 bot 的 webhook（切回 getUpdates 轮询模式）。
func DeleteTelegramWebhook(c *gin.Context) {
	var req telegramTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数无效"})
		return
	}

	body, err := callTelegramAPI("deleteWebhook", resolveToken(req.BotToken), nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("请求 Telegram API 失败: %v", err)})
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}

// GetTelegramWebhookInfo 查询当前 webhook 状态（url、pending_update_count、last_error_message 等）。
func GetTelegramWebhookInfo(c *gin.Context) {
	var req telegramTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数无效"})
		return
	}

	body, err := callTelegramAPI("getWebhookInfo", resolveToken(req.BotToken), nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": fmt.Sprintf("请求 Telegram API 失败: %v", err)})
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}
