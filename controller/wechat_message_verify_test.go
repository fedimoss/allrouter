package controller

import (
	"crypto/sha1"
	"encoding/hex"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func wechatTestSignature(token string, timestamp string, nonce string) string {
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}

// TestVerifyWechatServerReturnsEchoStr 校验 GET 入口在有正确签名时原样返回 echostr。
func TestVerifyWechatServerReturnsEchoStr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	token := service.NewWechatMessageConfig().Token
	signature := wechatTestSignature(token, "111", "222")
	query := url.Values{}
	query.Set("signature", signature)
	query.Set("timestamp", "111")
	query.Set("nonce", "222")
	query.Set("echostr", "hello_weixin")
	c.Request = httptest.NewRequest("GET", "/api/wechat/message?"+query.Encode(), nil)

	VerifyWechatServer(c)

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "hello_weixin", rec.Body.String())
}

// TestVerifyWechatServerRejectsBadSignature 校验签名错误时返回 403 且不回显。
func TestVerifyWechatServerRejectsBadSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	query := url.Values{}
	query.Set("signature", "wrong")
	query.Set("timestamp", "111")
	query.Set("nonce", "222")
	query.Set("echostr", "hello_weixin")
	c.Request = httptest.NewRequest("GET", "/api/wechat/message?"+query.Encode(), nil)

	VerifyWechatServer(c)

	require.Equal(t, 403, rec.Code)
	require.Empty(t, rec.Body.String())
}

// TestReceiveWechatMessagePlainMode 校验明文模式下能收到事件并回复空串。
func TestReceiveWechatMessagePlainMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	token := service.NewWechatMessageConfig().Token
	signature := wechatTestSignature(token, "111", "222")
	query := url.Values{}
	query.Set("signature", signature)
	query.Set("timestamp", "111")
	query.Set("nonce", "222")
	body := `<xml><FromUserName><![CDATA[openid_x]]></FromUserName><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[SCAN]]></Event><EventKey><![CDATA[allrouter.ai]]></EventKey><Ticket><![CDATA[T]]></Ticket></xml>`
	c.Request = httptest.NewRequest("POST", "/api/wechat/message?"+query.Encode(), strings.NewReader(body))

	ReceiveWechatMessage(c)

	require.Equal(t, 200, rec.Code)
	require.Empty(t, rec.Body.String())
}

// TestReceiveWechatMessageRejectsBadSignature 校验明文模式签名错误时不解析也不报错（回空串）。
func TestReceiveWechatMessageRejectsBadSignature(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	query := url.Values{}
	query.Set("signature", "wrong")
	query.Set("timestamp", "111")
	query.Set("nonce", "222")
	body := `<xml><Event><![CDATA[SCAN]]></Event></xml>`
	c.Request = httptest.NewRequest("POST", "/api/wechat/message?"+query.Encode(), strings.NewReader(body))

	ReceiveWechatMessage(c)

	require.Equal(t, 200, rec.Code)
	require.Empty(t, rec.Body.String())
}

// TestReceiveWechatMessageRepliesOnScan 验证已关注用户扫码（SCAN）时被动回复文本消息。
func TestReceiveWechatMessageRepliesOnScan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	token := service.NewWechatMessageConfig().Token
	signature := wechatTestSignature(token, "111", "222")
	query := url.Values{}
	query.Set("signature", signature)
	query.Set("timestamp", "111")
	query.Set("nonce", "222")
	body := `<xml><ToUserName><![CDATA[gh_abc]]></ToUserName><FromUserName><![CDATA[openid_x]]></FromUserName><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[SCAN]]></Event><EventKey><![CDATA[allrouter.ai]]></EventKey></xml>`
	c.Request = httptest.NewRequest("POST", "/api/wechat/message?"+query.Encode(), strings.NewReader(body))

	ReceiveWechatMessage(c)

	require.Equal(t, 200, rec.Code)
	// 明文模式下应回明文 XML 文本消息。
	resp := rec.Body.String()
	require.Contains(t, resp, "<MsgType><![CDATA[text]]></MsgType>")
	// 引用常量而非硬编码文案，避免文案调整后测试失效。
	require.Contains(t, resp, wechatScanReplyContent)
	// 回复方向：发给用户 openid，来自公众号。
	require.Contains(t, resp, "<ToUserName><![CDATA[openid_x]]></ToUserName>")
	require.Contains(t, resp, "<FromUserName><![CDATA[gh_abc]]></FromUserName>")
}

// TestReceiveWechatMessageNoReplyOnPlainSubscribe 验证普通关注（无场景值）不回复，避免骚扰。
func TestReceiveWechatMessageNoReplyOnPlainSubscribe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	token := service.NewWechatMessageConfig().Token
	signature := wechatTestSignature(token, "111", "222")
	query := url.Values{}
	query.Set("signature", signature)
	query.Set("timestamp", "111")
	query.Set("nonce", "222")
	body := `<xml><ToUserName><![CDATA[gh_abc]]></ToUserName><FromUserName><![CDATA[openid_x]]></FromUserName><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[subscribe]]></Event></xml>`
	c.Request = httptest.NewRequest("POST", "/api/wechat/message?"+query.Encode(), strings.NewReader(body))

	ReceiveWechatMessage(c)

	require.Equal(t, 200, rec.Code)
	require.Empty(t, rec.Body.String())
}
