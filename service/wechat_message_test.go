package service

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildWechatPushSignature(t *testing.T) {
	// 微信官方规则：token、timestamp、nonce 字典序排序后拼接再 sha1。
	// 用一组可手算的输入验证顺序无关性与稳定性。
	got := BuildWechatPushSignature("token", "123", "456")
	require.Equal(t, BuildWechatPushSignature("token", "456", "123"), got)
	require.Len(t, got, 40) // sha1 十六进制固定 40 位

	require.True(t, VerifyWechatPushSignature("token", got, "123", "456"))
	require.True(t, VerifyWechatPushSignature("token", got, "456", "123"))
	require.False(t, VerifyWechatPushSignature("token", "deadbeef", "123", "456"))
}

func TestVerifyWechatPushEncryptedSignature(t *testing.T) {
	signature := BuildWechatPushEncryptedSignature("token", "123", "456", "ENCRYPT")
	require.True(t, VerifyWechatPushEncryptedSignature("token", signature, "123", "456", "ENCRYPT"))
	require.False(t, VerifyWechatPushEncryptedSignature("token", signature, "123", "456", "OTHER"))
}

func TestParseWechatPushMessageSubscribeWithScene(t *testing.T) {
	xml := `<xml><ToUserName><![CDATA[gh_xxx]]></ToUserName>
<FromUserName><![CDATA[openid_123]]></FromUserName>
<CreateTime>1790000000</CreateTime>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[subscribe]]></Event>
<EventKey><![CDATA[qrscene_allrouter.ai]]></EventKey>
<Ticket><![CDATA[TICKET_VALUE]]></Ticket>
</xml>`
	message, err := ParseWechatPushMessage([]byte(xml))
	require.NoError(t, err)
	require.Equal(t, "event", message.MsgType)
	require.Equal(t, "subscribe", message.Event)
	require.Equal(t, "openid_123", message.FromUserName)
	require.Equal(t, int64(1790000000), message.CreateTime)
	require.Equal(t, "qrscene_allrouter.ai", message.EventKey)
	require.Equal(t, "TICKET_VALUE", message.Ticket)
	require.Equal(t, "allrouter.ai", NormalizeWechatEventKey(message.EventKey))
}

func TestParseWechatPushMessageScanFriend(t *testing.T) {
	xml := `<xml><FromUserName><![CDATA[openid_456]]></FromUserName>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[SCAN]]></Event>
<EventKey><![CDATA[allrouter.ai]]></EventKey>
<Ticket><![CDATA[T2]]></Ticket>
</xml>`
	message, err := ParseWechatPushMessage([]byte(xml))
	require.NoError(t, err)
	require.Equal(t, "SCAN", message.Event)
	// 已关注用户扫码没有 qrscene_ 前缀，Normalize 后仍是原值。
	require.Equal(t, "allrouter.ai", NormalizeWechatEventKey(message.EventKey))
}

func TestNormalizeWechatEventKey(t *testing.T) {
	require.Equal(t, "abc", NormalizeWechatEventKey("qrscene_abc"))
	require.Equal(t, "abc", NormalizeWechatEventKey("abc"))
	require.Equal(t, "", NormalizeWechatEventKey(""))
}

func TestExtractWechatPushEncrypt(t *testing.T) {
	encrypt, err := ExtractWechatPushEncrypt([]byte(`<xml><ToUserName><![CDATA[gh]]></ToUserName><Encrypt><![CDATA[CIPHER]]></Encrypt></xml>`))
	require.NoError(t, err)
	require.Equal(t, "CIPHER", encrypt)

	_, err = ExtractWechatPushEncrypt([]byte(`<xml><ToUserName><![CDATA[gh]]></ToUserName></xml>`))
	require.Error(t, err)
}

func TestDecryptWechatPushMessageRoundTrip(t *testing.T) {
	// 用与微信相同的算法构造一段密文，验证解密能还原出消息明文。
	encodingAESKey := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG" // 43 位
	plainMessage := `<xml><FromUserName><![CDATA[openid]]></FromUserName><Event><![CDATA[SCAN]]></Event></xml>`
	encrypted := encryptWechatPushForTest(t, encodingAESKey, plainMessage, "wx_appid")

	got, err := DecryptWechatPushMessage(encodingAESKey, encrypted)
	require.NoError(t, err)
	require.Equal(t, plainMessage, got)
}

// TestDecryptWechatPushMessageLargePadding 覆盖填充值大于 16 的情况：
// 微信按 32 字节块填充，填充值可能是 17-32；若按 AES 的 16 字节块去限制，
// 这类消息（如部分关注事件）会被误判为非法而解密失败。
func TestDecryptWechatPushMessageLargePadding(t *testing.T) {
	encodingAESKey := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	// 构造消息长度，使 (16 随机 + 4 长度 + msg + appid) 对 32 取模为 14，需填 18 字节，
	// 正好复现线上关注的 invalid pkcs7 padding value 18。
	appID := "wx_appid_1234" // 13 字节
	msgLen := 13             // 20 + 13 + 13 = 46，46 % 32 = 14，填充 18
	plainMessage := strings.Repeat("a", msgLen)
	encrypted := encryptWechatPushForTest(t, encodingAESKey, plainMessage, appID)

	got, err := DecryptWechatPushMessage(encodingAESKey, encrypted)
	require.NoError(t, err)
	require.Equal(t, plainMessage, got)
}

func TestBuildWechatTextReplyXML(t *testing.T) {
	reply, err := BuildWechatTextReplyXML(&WechatTextReply{
		ToUserName:   "openid_123",
		FromUserName: "gh_abc",
		Content:      "Base URL: https://allrouter.ai/v1\nAPI 密钥: 123456789",
	})
	require.NoError(t, err)

	text := string(reply)
	require.Contains(t, text, "<ToUserName><![CDATA[openid_123]]></ToUserName>")
	require.Contains(t, text, "<FromUserName><![CDATA[gh_abc]]></FromUserName>")
	require.Contains(t, text, "<MsgType><![CDATA[text]]></MsgType>")
	require.Contains(t, text, "Base URL: https://allrouter.ai/v1")

	parsed, err := ParseWechatPushMessage(reply)
	require.NoError(t, err)
	require.Equal(t, "openid_123", parsed.ToUserName)
	require.Equal(t, "text", parsed.MsgType)
	require.Contains(t, parsed.Content, "API 密钥: 123456789")
}

func TestBuildWechatTextReplyXMLRejectsEmpty(t *testing.T) {
	_, err := BuildWechatTextReplyXML(&WechatTextReply{FromUserName: "gh", Content: "hi"})
	require.Error(t, err)
	_, err = BuildWechatTextReplyXML(&WechatTextReply{ToUserName: "o", Content: "hi"})
	require.Error(t, err)
	_, err = BuildWechatTextReplyXML(&WechatTextReply{ToUserName: "o", FromUserName: "gh"})
	require.Error(t, err)
}

// TestBuildWechatEncryptedReplyRoundTrip 验证加密回包能被同密钥解密回原明文，
// 且签名自洽（微信侧就是用同样算法校验回包）。
func TestBuildWechatEncryptedReplyRoundTrip(t *testing.T) {
	config := &WechatMessageConfig{
		Token:          "test_token",
		EncodingAESKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
		AppID:          "wx_test_appid",
	}
	plainReply := `<xml><ToUserName><![CDATA[openid]]></ToUserName><MsgType><![CDATA[text]]></MsgType></xml>`

	encryptedReply, err := BuildWechatEncryptedReply(config, []byte(plainReply))
	require.NoError(t, err)

	envelope, err := ExtractWechatPushEncrypt(encryptedReply)
	require.NoError(t, err)

	// 解密回原明文。
	decrypted, err := DecryptWechatPushMessage(config.EncodingAESKey, envelope)
	require.NoError(t, err)
	require.Equal(t, plainReply, decrypted)

	// 回包里的 MsgSignature 应与 token+timestamp+nonce+encrypt 自洽。
	var reply struct {
		MsgSignature string `xml:"MsgSignature"`
		TimeStamp    string `xml:"TimeStamp"`
		Nonce        string `xml:"Nonce"`
	}
	require.NoError(t, xml.Unmarshal(encryptedReply, &reply))
	require.True(t, VerifyWechatPushEncryptedSignature(config.Token, reply.MsgSignature, reply.TimeStamp, reply.Nonce, envelope))
}

// encryptWechatPushForTest 复刻微信消息加密格式，仅用于测试解密逻辑：
// 16 字节随机数 + 4 字节网络序长度 + 明文 + AppID，PKCS#7 填充后 AES-256-CBC 加密再 base64。
func encryptWechatPushForTest(t *testing.T, encodingAESKey string, plainMessage string, appID string) string {
	t.Helper()

	key, err := decodeWechatAESKey(encodingAESKey)
	require.NoError(t, err)

	random := make([]byte, 16)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(plainMessage)))

	raw := make([]byte, 0, 16+4+len(plainMessage)+len(appID))
	raw = append(raw, random...)
	raw = append(raw, length...)
	raw = append(raw, []byte(plainMessage)...)
	raw = append(raw, []byte(appID)...)

	// 微信按 32 字节块做 PKCS#7 填充，测试也按同样规则构造，才能覆盖大填充值场景。
	padded := pkcs7Pad(raw, wechatPKCS7BlockSize)
	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ciphertext, padded)
	return base64.StdEncoding.EncodeToString(ciphertext)
}
