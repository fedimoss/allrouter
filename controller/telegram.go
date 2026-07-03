package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func TelegramBind(c *gin.Context) {
	if !common.TelegramOAuthEnabled {
		c.JSON(200, gin.H{
			"message": "管理员未开启通过 Telegram 登录以及注册",
			"success": false,
		})
		return
	}
	params := c.Request.URL.Query()
	if !checkTelegramAuthorization(params, common.TelegramBotToken) {
		c.JSON(200, gin.H{
			"message": "无效的请求",
			"success": false,
		})
		return
	}
	telegramId := params["id"][0]
	if model.IsTelegramIdAlreadyTaken(telegramId) {
		c.JSON(200, gin.H{
			"message": "该 Telegram 账户已被绑定",
			"success": false,
		})
		return
	}

	session := sessions.Default(c)
	id := session.Get("id")
	user := model.User{Id: id.(int)}
	if err := user.FillUserById(); err != nil {
		c.JSON(200, gin.H{
			"message": err.Error(),
			"success": false,
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
	user.TelegramId = telegramId
	if err := user.Update(false); err != nil {
		c.JSON(200, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}

	c.Redirect(302, common.ThemeAwarePath("/console/personal"))
}

func TelegramLogin(c *gin.Context) {
	if !common.TelegramOAuthEnabled {
		c.JSON(200, gin.H{
			"message": "管理员未开启通过 Telegram 登录以及注册",
			"success": false,
		})
		return
	}
	params := c.Request.URL.Query()
	if !checkTelegramAuthorization(params, common.TelegramBotToken) {
		c.JSON(200, gin.H{
			"message": "无效的请求",
			"success": false,
		})
		return
	}

	telegramId := params["id"][0]
	user := model.User{TelegramId: telegramId}
	if err := user.FillUserByTelegramId(); err != nil {
		c.JSON(200, gin.H{
			"message": err.Error(),
			"success": false,
		})
		return
	}
	setupLogin(&user, c, false)
}

func checkTelegramAuthorization(params map[string][]string, token string) bool {
	strs := []string{}
	var hash = ""
	for k, v := range params {
		if k == "hash" {
			hash = v[0]
			continue
		}
		strs = append(strs, k+"="+v[0])
	}
	sort.Strings(strs)
	var imploded = ""
	for _, s := range strs {
		if imploded != "" {
			imploded += "\n"
		}
		imploded += s
	}
	sha256hash := sha256.New()
	io.WriteString(sha256hash, token)
	hmachash := hmac.New(sha256.New, sha256hash.Sum(nil))
	io.WriteString(hmachash, imploded)
	ss := hex.EncodeToString(hmachash.Sum(nil))
	return hash == ss
}

// TelegramWebhook 接收 Telegram 通过 webhook 投递的 Update。
// 在群聊场景下，用户给机器人发送的消息会由 Telegram 回调到本接口，
// 这里解析出 update.Message 并打印群 ID、用户 ID、用户名与消息文本。
func TelegramWebhook(c *gin.Context) {
	// 校验 secret_token：仅当配置了 TELEGRAM_WEBHOOK_SECRET 时启用，
	// 与 setWebhook 时传入的 secret_token 比对，防止第三方伪造回调。
	if common.TelegramWebhookSecret != "" {
		got := c.GetHeader("X-Telegram-Bot-Api-Secret-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(common.TelegramWebhookSecret)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid telegram webhook secret token"})
			return
		}
	}

	var update models.Update
	if err := c.ShouldBindJSON(&update); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if update.Message != nil {
		// log.Println("群ID:", update.Message.Chat.ID)
		// if update.Message.From != nil {
		// 	log.Println("用户ID:", update.Message.From.ID)
		// 	log.Println("用户名:", update.Message.From.Username)
		// }
		// log.Println("消息:", update.Message.Text)

		// 新成员入群：发送欢迎消息 + 打开 Mini App 的 web_app 按钮
		if len(update.Message.NewChatMembers) > 0 {
			if err := welcomeNewMembers(c, update.Message); err != nil {
				log.Println("[Telegram] 欢迎消息发送失败:", err)
			}
		}

		// 回复对应的群成员：把消息回发到群里，并以 reply 形式指向该成员的原消息
		// if update.Message.From != nil && update.Message.Text != "" {
		// 	if err := replyInGroup(c, update.Message); err != nil {
		// 		log.Println("[Telegram] 回复失败:", err)
		// 	}
		// }
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// replyInGroup 在群聊里回复发送者的原消息（reply-to）。
func replyInGroup(c *gin.Context, msg *models.Message) error {
	if msg == nil || msg.From == nil {
		return nil
	}
	b, err := getTelegramBot()
	if err != nil {
		return err
	}
	name := msg.From.Username
	if name == "" {
		name = msg.From.FirstName
	}
	_, err = b.SendMessage(c.Request.Context(), &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   fmt.Sprintf("%s，收到你的消息：%s", name, msg.Text),
		ReplyParameters: &models.ReplyParameters{
			MessageID:                msg.ID,
			ChatID:                   msg.Chat.ID,
			AllowSendingWithoutReply: true,
		},
	})
	return err
}

var (
	telegramBot    *bot.Bot
	telegramBotMu  sync.Mutex
	telegramBotKey string
)

// getTelegramBot 获取（必要时创建）复用的 bot 实例。
// common.TelegramBotToken 是后台可改的配置，token 变化时自动重建实例。
func getTelegramBot() (*bot.Bot, error) {
	token := common.TelegramBotToken
	if token == "" {
		return nil, fmt.Errorf("未配置 Telegram Bot Token，无法回复消息")
	}
	telegramBotMu.Lock()
	defer telegramBotMu.Unlock()
	if telegramBot != nil && telegramBotKey == token {
		return telegramBot, nil
	}
	b, err := bot.New(token)
	if err != nil {
		return nil, fmt.Errorf("创建 Telegram bot 失败: %w", err)
	}
	telegramBot = b
	telegramBotKey = token
	return telegramBot, nil
}

//go:embed telegram_miniapp.html
var telegramMiniAppPage []byte

// TelegramMiniAppPage 返回内嵌的 Mini App HTML 页面（web_app 按钮打开）。
func TelegramMiniAppPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", telegramMiniAppPage)
}

// welcomeNewMembers 向新入群成员发送欢迎消息，附带打开 Mini App 的按钮。
func welcomeNewMembers(c *gin.Context, msg *models.Message) error {
	if common.TelegramMiniAppURL == "" {
		return fmt.Errorf("未配置 TELEGRAM_MINIAPP_URL（Mini App 启动链接），无法发送绑定按钮")
	}
	var names []string
	for _, m := range msg.NewChatMembers {
		if m.IsBot {
			continue // 过滤机器人自身
		}
		n := m.Username
		if n == "" {
			n = m.FirstName
		}
		if n == "" {
			n = fmt.Sprintf("%d", m.ID)
		}
		names = append(names, n)
	}
	if len(names) == 0 {
		return nil // 仅机器人入群，不欢迎
	}
	b, err := getTelegramBot()
	if err != nil {
		return err
	}
	// 群聊不支持 web_app 按钮（仅私聊可用，否则 BUTTON_TYPE_INVALID），故用 URL 按钮。
	// 指向 @BotFather /newapp 生成的 Mini App 启动链接（t.me/<bot>/<appname>），
	// 该形式能稳定打开 Mini App 并拿到 initData。
	_, err = b.SendMessage(c.Request.Context(), &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   fmt.Sprintf("欢迎 %s 加入！点击下方按钮完成账号绑定：", strings.Join(names, "、")),
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{
				{{Text: "点击绑定账号", URL: common.TelegramMiniAppURL}},
			},
		},
	})
	return err
}

type telegramMiniAppBindRequest struct {
	Username string `json:"username" binding:"required"`
	InitData string `json:"initData" binding:"required"`
}

// TelegramMiniAppBind 解析 Mini App 的 initData，校验来源后在 telegram_user_bindings 表记录
// 该 TG 用户与该网站用户的对应关系（用于群内身份关联/发福利，不写入 users.telegram_id，不授予登录权限）。
// 注意：仅凭 initData 证明 Telegram 身份，不校验网站侧归属（用户已知情并接受该风险）。
func TelegramMiniAppBind(c *gin.Context) {
	var req telegramMiniAppBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请求参数无效"})
		return
	}

	// 1. 校验 initData（HMAC），拿到 Telegram 用户
	values, err := url.ParseQuery(req.InitData)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的 initData"})
		return
	}
	tgUser, ok := bot.ValidateWebappRequest(values, common.TelegramBotToken)
	if !ok || tgUser == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "initData 校验失败"})
		return
	}
	telegramId := strconv.FormatInt(tgUser.ID, 10)

	// 2. 查网站用户
	target := model.User{Username: req.Username}
	if err := target.FillUserByUsername(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 3. 冲突守卫（查 telegram_user_bindings 表，不再看 users.telegram_id）
	existingByTG, err := model.GetTelegramBindingByTGID(telegramId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "查询绑定信息失败"})
		return
	}
	// 该 telegram_id 已绑定到其他网站账号
	if existingByTG != nil && existingByTG.UserID != target.Id {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "该 Telegram 已绑定到其他网站账号"})
		return
	}

	existingByUser, err := model.GetTelegramBindingByUserID(target.Id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "查询绑定信息失败"})
		return
	}
	// 该网站账号已绑定到其他 Telegram
	if existingByUser != nil && existingByUser.TelegramUserID != telegramId {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "该账号已绑定其他 Telegram"})
		return
	}

	// 4. 幂等：已经是这个绑定关系
	if existingByTG != nil && existingByTG.UserID == target.Id {
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "验证成功"})
		return
	}

	// 5. 写入新表
	if err := model.CreateTelegramBinding(&model.TelegramUserBinding{
		TelegramUserID: telegramId,
		UserID:         target.Id,
		UserName:       req.Username,
	}); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "验证成功"})
}
