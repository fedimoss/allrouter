package controller

// 本文件是微信服务号「消息与事件推送」的服务器接入端点。
//
// 官方文档：
//   - 消息与事件推送介绍：https://developers.weixin.qq.com/doc/service/guide/dev/push/
//   - 接收事件推送（含扫描带参二维码）：https://developers.weixin.qq.com/doc/service/guide/product/message/Receiving_event_pushes.html
//
// 两个入口（同一 URL，方法不同）：
//   - GET  VerifyWechatServer  ：服务器配置校验。微信提交配置时用 GET 探测，校验通过必须
//     原样返回 echostr（纯文本，不能包 JSON），否则后台会提示 token 校验失败。
//   - POST ReceiveWechatMessage：接收消息/事件。无论用户是否已关注，只要扫码（或关注、取关、
//     点菜单等）都会推送到这里；本实现会把解析后的内容打印到控制台，并在用户扫码后被动回复
//     一条文本消息（内容见 wechatScanReplyContent）。
//
// 凭据（Token / EncodingAESKey）来自 service.NewWechatMessageConfig()，目前是占位符，
// 测试时替换为微信后台填写的真实值即可。

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// wechatScanReplyContent 是用户扫码后被动回复的文本内容。
// 先写死便于联调；后续接入数据库查询后，改为按 openid 动态生成即可。
const wechatScanReplyContent = "xxxxxxxxxxxxxxxxxxx"

// VerifyWechatServer 处理微信服务器配置校验（GET）。
//
// 微信会在 URL 上带 signature、timestamp、nonce、echostr 四个参数：
//   - 用 Token + timestamp + nonce 计算 sha1 签名，与 signature 比对；
//   - 一致则把 echostr 原样返回（纯文本），后台显示配置成功；
//   - 不一致返回 403，后台显示校验失败。
func VerifyWechatServer(c *gin.Context) {
	config := service.NewWechatMessageConfig()
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	echoStr := c.Query("echostr")

	if !service.VerifyWechatPushSignature(config.Token, signature, timestamp, nonce) {
		logger.LogWarn(c.Request.Context(), "wechat server verify failed: signature mismatch")
		c.String(http.StatusForbidden, "")
		return
	}

	logger.LogInfo(c.Request.Context(), "wechat server verify success")
	// 必须原样返回 echostr，且为纯文本；不能走 common.ApiSuccess（那会包成 JSON）。
	c.String(http.StatusOK, echoStr)
}

// ReceiveWechatMessage 处理微信消息/事件推送（POST）。
//
// 处理流程：
//  1. 读取原始 XML 请求体；
//  2. 判断是明文还是密文：请求体含 Encrypt 字段即为安全/兼容模式；
//  3. 校验签名（安全模式用 msg_signature，其余用 signature）；
//  4. 安全模式先 AES 解密得到明文 XML，再解析为 WechatPushMessage；
//  5. 把推送内容打印到控制台；
//  6. 若是扫码事件（subscribe 带场景值 / SCAN），被动回复一条文本消息；
//     其他事件回复空串（微信不会重试）。
func ReceiveWechatMessage(c *gin.Context) {
	config := service.NewWechatMessageConfig()
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.LogError(c.Request.Context(), "wechat push read body failed: "+err.Error())
		c.String(http.StatusOK, "")
		return
	}

	plainXML, ok := resolveWechatPushPlainXML(c, config, body)
	if !ok {
		// 验签/解密失败，已记录日志；回复空串避免微信重试。
		c.String(http.StatusOK, "")
		return
	}

	message, err := service.ParseWechatPushMessage(plainXML)
	if err != nil {
		logger.LogError(c.Request.Context(), "wechat push parse failed: "+err.Error())
		c.String(http.StatusOK, "")
		return
	}

	logWechatPushMessage(c, message, plainXML)

	reply, ok := buildWechatScanReply(c, config, message)
	if !ok {
		c.String(http.StatusOK, "")
		return
	}
	// 被动回复必须是 text/xml；内容为明文 XML 或（安全模式）密文 XML。
	c.Data(http.StatusOK, "text/xml; charset=utf-8", reply)
}

// buildWechatScanReply 在扫码事件下构造被动回复内容。
//
// 命中条件（用户扫码）：
//   - 已关注用户扫码：Event=SCAN；
//   - 未关注用户扫码后关注：Event=subscribe 且带二维码场景值（EventKey 有值）。
//
// 普通关注（非扫码）也会是 subscribe 但没有 EventKey，不回复，避免骚扰。
// 返回的第二个值表示是否需要回复；需要时已按当前模式（明文/安全）加密处理。
func buildWechatScanReply(c *gin.Context, config *service.WechatMessageConfig, message *service.WechatPushMessage) ([]byte, bool) {
	if !isWechatScanEvent(message) {
		return nil, false
	}

	plainReply, err := service.BuildWechatTextReplyXML(&service.WechatTextReply{
		ToUserName:   message.FromUserName, // 回复给用户，方向与推送相反
		FromUserName: message.ToUserName,
		Content:      wechatScanReplyContent,
	})
	if err != nil {
		logger.LogError(c.Request.Context(), "wechat build text reply failed: "+err.Error())
		return nil, false
	}

	// 安全/兼容模式必须回密文；明文模式直接回明文 XML。
	if c.Query("encrypt_type") == "aes" {
		encrypted, err := service.BuildWechatEncryptedReply(config, plainReply)
		if err != nil {
			logger.LogError(c.Request.Context(), "wechat encrypt reply failed: "+err.Error())
			return nil, false
		}
		return encrypted, true
	}
	return plainReply, true
}

// isWechatScanEvent 判断是否为“用户扫码”事件。
func isWechatScanEvent(message *service.WechatPushMessage) bool {
	if message == nil {
		return false
	}
	if message.Event == "SCAN" {
		return true
	}
	// 未关注用户扫码后关注：subscribe 且携带二维码场景值。
	return message.Event == "subscribe" && strings.TrimSpace(message.EventKey) != ""
}

// resolveWechatPushPlainXML 完成验签与（必要时）解密，返回可用于解析的明文 XML。
// 失败时返回 ok=false，调用方据此回复空串。
func resolveWechatPushPlainXML(c *gin.Context, config *service.WechatMessageConfig, body []byte) ([]byte, bool) {
	signature := c.Query("signature")
	msgSignature := c.Query("msg_signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")

	// 先尝试取出 Encrypt。取得到说明是安全/兼容模式的密文包。
	encrypt, encryptErr := service.ExtractWechatPushEncrypt(body)
	if encryptErr == nil {
		if !service.VerifyWechatPushEncryptedSignature(config.Token, msgSignature, timestamp, nonce, encrypt) {
			logger.LogWarn(c.Request.Context(), "wechat push rejected: msg_signature mismatch")
			return nil, false
		}
		plainXML, err := service.DecryptWechatPushMessage(config.EncodingAESKey, encrypt)
		if err != nil {
			// msg_signature 已通过说明密文来自微信、Token 正确；此时解密失败通常是
			// EncodingAESKey 与后台不一致（或填的是占位值）。打印长度便于核对，但不回显密文。
			logger.LogError(c.Request.Context(), "wechat push decrypt failed: "+err.Error()+
				" body_len="+strconv.Itoa(len(body))+
				" encrypt_len="+strconv.Itoa(len(encrypt))+
				" aes_key_len="+strconv.Itoa(len(config.EncodingAESKey)))
			return nil, false
		}
		return []byte(plainXML), true
	}

	// 明文模式：直接用 signature 校验。
	if !service.VerifyWechatPushSignature(config.Token, signature, timestamp, nonce) {
		logger.LogWarn(c.Request.Context(), "wechat push rejected: signature mismatch")
		return nil, false
	}
	return body, true
}

// logWechatPushMessage 把推送内容打印到控制台（gin.DefaultWriter 同时输出到控制台与日志文件）。
//
// 打印内容覆盖关注/取关、扫描带参二维码两种事件，以及地理位置、菜单、文本消息等通用字段，
// 便于联调时直接观察微信实际推来的数据。扫码场景重点看 Event 与 EventKey：
//   - 未关注扫码并关注：Event=subscribe，EventKey=qrscene_<场景值>；
//   - 已关注直接扫码：  Event=SCAN，     EventKey=<场景值>。
func logWechatPushMessage(c *gin.Context, message *service.WechatPushMessage, plainXML []byte) {
	lines := []string{
		"========== wechat push received ==========",
		"MsgType=" + message.MsgType + " Event=" + message.Event,
		"FromUserName(OpenID)=" + message.FromUserName,
		"ToUserName=" + message.ToUserName,
		"CreateTime=" + strconv.FormatInt(message.CreateTime, 10),
	}
	if message.EventKey != "" {
		lines = append(lines,
			"EventKey="+message.EventKey,
			"SceneValue="+service.NormalizeWechatEventKey(message.EventKey),
		)
	}
	if message.Ticket != "" {
		lines = append(lines, "Ticket="+message.Ticket)
	}
	if message.Content != "" {
		lines = append(lines, "Content="+message.Content)
	}
	if message.MsgId != 0 {
		lines = append(lines, "MsgId="+strconv.FormatInt(message.MsgId, 10))
	}
	if message.Latitude != "" || message.Longitude != "" || message.Precision != "" {
		lines = append(lines, "Location="+message.Latitude+","+message.Longitude+","+message.Precision)
	}
	if message.MenuId != "" {
		lines = append(lines, "MenuId="+message.MenuId)
	}
	if message.Status != "" {
		lines = append(lines, "Status="+message.Status)
	}
	// 直接回显微信推送的原始明文 XML，避免自造结构掩盖真实字段。
	lines = append(lines, "RawXML="+strings.TrimSpace(string(plainXML)))
	lines = append(lines, "==========================================")

	logger.LogInfo(c.Request.Context(), strings.Join(lines, "\n"))
}
