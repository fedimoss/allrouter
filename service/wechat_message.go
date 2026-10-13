// 本文件封装微信服务号“消息与事件推送”的服务器接入逻辑。
//
// 官方文档：
//   - 消息与事件推送介绍：https://developers.weixin.qq.com/doc/service/guide/dev/push/
//   - 接收事件推送：https://developers.weixin.qq.com/doc/service/guide/product/message/Receiving_event_pushes.html
//
// 接入分两步（新接手同事先读这段）：
//
//  1. 服务器配置校验（GET）。在微信公众平台「开发 - 消息与事件推送」填写服务器 URL、Token、
//     EncodingAESKey 并提交时，微信会向该 URL 发一个 GET 请求，携带 signature/timestamp/
//     nonce/echostr。开发者用 Token 按“字典序排序 + sha1”算出签名，与 signature 比对；一致
//     则原样返回 echostr，接入生效，否则接入失败。
//  2. 事件接收（POST）。接入生效后，用户发消息或触发事件（关注/取关、扫描带参二维码、点击
//     菜单等）时，微信会 POST XML 到该 URL。本文件负责验签、（安全模式下）解密、解析 XML。
//
// 加解密方式说明：
//   - 明文模式：POST 体就是明文 XML，仅用 signature 校验；
//   - 兼容模式：明文与密文共存，本文件按“含 Encrypt 字段即密文”处理；
//   - 安全模式：POST 体是 <xml><Encrypt>...</Encrypt></xml>，需用 msg_signature 验签，
//     再用 EncodingAESKey 做 AES-256-CBC 解密。
//
// Token 与 EncodingAESKey 暂用占位符，正式测试时改成微信后台配置的真实值即可。

package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// 占位数据：正式测试时替换为微信公众平台「消息与事件推送」里配置的真实值。
	//
	//   - wechatMessageTokenPlaceholder：即后台填写的 Token，用于生成/校验签名。
	//   - wechatMessageEncodingAESKeyPlaceholder：即后台的 EncodingAESKey，43 位字符串
	//     （base64 字符集，无 "=" 填充），base64 解码后是 32 字节，用作 AES-256 密钥。
	//     下面这个占位值刻意做成合法的 43 位，保证解码流程不会因格式问题提前报错；
	//     但占位值本身解密必然失败，只有换成真实值才能通过。
	wechatMessageTokenPlaceholder          = "xxxxxxxxxxxxxx"
	wechatMessageEncodingAESKeyPlaceholder = "xxxxxxxxxxxxxx" // 43 字符
	// wechatMessageAppIDPlaceholder 是公众号 AppID。
	// 加密回包时明文里必须带上真实 AppID（微信据此识别账号），否则回包可能被丢弃。
	// 请替换为与 EncodingAESKey 同一公众号的 AppID。
	wechatMessageAppIDPlaceholder = "wx_placeholder_appid"

	// wechatPushEventKeyScenePrefix 是未关注用户扫码后 subscribe 事件的 EventKey 前缀。
	wechatPushEventKeyScenePrefix = "qrscene_"

	// wechatPKCS7BlockSize 是微信消息体 PKCS#7 填充的块大小。
	// 注意：这是微信的特殊约定，取 32（而不是 AES 的 16 字节块），
	// 因此去填充时合法的填充值范围是 1-32。
	wechatPKCS7BlockSize = 32
)

// WechatMessageConfig 是服务器配置所需的凭据。
//
// 字段说明：
//   - Token          ：用于签名校验，需与微信后台填写的 Token 完全一致。
//   - EncodingAESKey ：安全/兼容模式下用于 AES 解密，需与后台填写的一致。
//
// 占位符集中在 NewWechatMessageConfig，替换真实值时只需改这一处（或调用方直接覆盖字段）。
type WechatMessageConfig struct {
	Token          string
	EncodingAESKey string
	AppID          string
}

// NewWechatMessageConfig 返回带占位凭据的配置。测试前请替换为微信后台的真实值。
func NewWechatMessageConfig() *WechatMessageConfig {
	return &WechatMessageConfig{
		Token:          wechatMessageTokenPlaceholder,
		EncodingAESKey: wechatMessageEncodingAESKeyPlaceholder,
		AppID:          wechatMessageAppIDPlaceholder,
	}
}

// WechatPushMessage 是微信消息/事件推送 XML 的解析结果。
//
// 不同 MsgType / Event 只会填充对应字段，未出现的字段零值即可忽略：
//   - 关注/取关事件：MsgType=event，Event=subscribe/unsubscribe；
//   - 扫描带参二维码（未关注）：MsgType=event，Event=subscribe，EventKey=qrscene_xxx，
//     并带 Ticket；
//   - 扫描带参二维码（已关注）：MsgType=event，Event=SCAN，EventKey=xxx，并带 Ticket；
//   - 上报地理位置：Event=LOCATION，带 Latitude/Longitude/Precision；
//   - 菜单事件：Event=CLICK/VIEW，带 EventKey；
//   - 普通文本消息：MsgType=text，带 Content 与 MsgId。
type WechatPushMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`   // 开发者微信号（接收方）
	FromUserName string   `xml:"FromUserName"` // 发送方 OpenID
	CreateTime   int64    `xml:"CreateTime"`   // 消息创建时间（秒级时间戳）
	MsgType      string   `xml:"MsgType"`      // 消息类型：event / text / image ...
	Event        string   `xml:"Event"`        // 事件类型：subscribe / unsubscribe / SCAN / LOCATION / CLICK / VIEW
	EventKey     string   `xml:"EventKey"`     // 事件 KEY：带参二维码场景值；关注事件会带 qrscene_ 前缀
	Ticket       string   `xml:"Ticket"`       // 二维码 ticket
	Content      string   `xml:"Content"`      // 文本消息内容
	MsgId        int64    `xml:"MsgId"`        // 消息 ID（普通消息才有）
	Latitude     string   `xml:"Latitude"`     // 地理位置纬度
	Longitude    string   `xml:"Longitude"`    // 地理位置经度
	Precision    string   `xml:"Precision"`    // 地理位置精度
	MenuId       string   `xml:"MenuId"`       // 菜单 ID
	Status       string   `xml:"Status"`       // 事件状态（部分事件才有）
}

// wechatPushEnvelope 用于在安全/兼容模式下取出密文 XML 的 Encrypt 字段。
type wechatPushEnvelope struct {
	XMLName xml.Name `xml:"xml"`
	Encrypt string   `xml:"Encrypt"`
}

// BuildWechatPushSignature 按微信规则生成签名：
// 将 token、timestamp、nonce 三个参数按字典序排序后拼接，再做 sha1，输出十六进制字符串。
//
// 这是微信服务器配置校验（GET）与明文/兼容模式推送（POST）共用的签名算法。
func BuildWechatPushSignature(token string, timestamp string, nonce string) string {
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}

// BuildWechatPushEncryptedSignature 生成安全模式下的签名：
// 与上面类似，但参与排序拼接的第四个参数是密文 Encrypt。
// 安全模式下微信传的是 msg_signature，需用此函数校验。
func BuildWechatPushEncryptedSignature(token string, timestamp string, nonce string, encrypt string) string {
	parts := []string{token, timestamp, nonce, encrypt}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}

// VerifyWechatPushSignature 校验明文/兼容模式签名，十六进制比对不区分大小写。
func VerifyWechatPushSignature(token string, signature string, timestamp string, nonce string) bool {
	expected := BuildWechatPushSignature(token, timestamp, nonce)
	return strings.EqualFold(strings.TrimSpace(signature), expected)
}

// VerifyWechatPushEncryptedSignature 校验安全模式签名。
func VerifyWechatPushEncryptedSignature(token string, signature string, timestamp string, nonce string, encrypt string) bool {
	expected := BuildWechatPushEncryptedSignature(token, timestamp, nonce, encrypt)
	return strings.EqualFold(strings.TrimSpace(signature), expected)
}

// ExtractWechatPushEncrypt 从安全/兼容模式的密文 XML 中取出 Encrypt 字段。
func ExtractWechatPushEncrypt(data []byte) (string, error) {
	var envelope wechatPushEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("parse wechat push encrypt envelope failed: %w", err)
	}
	encrypt := strings.TrimSpace(envelope.Encrypt)
	if encrypt == "" {
		return "", fmt.Errorf("wechat push encrypt field is empty")
	}
	return encrypt, nil
}

// ParseWechatPushMessage 解析明文推送 XML 为结构体。
func ParseWechatPushMessage(data []byte) (*WechatPushMessage, error) {
	var message WechatPushMessage
	if err := xml.Unmarshal(data, &message); err != nil {
		return nil, fmt.Errorf("parse wechat push xml failed: %w", err)
	}
	return &message, nil
}

// NormalizeWechatEventKey 去掉关注事件的 qrscene_ 前缀，得到纯场景值。
//
// 微信约定：未关注用户扫码并关注后推送的 subscribe 事件，EventKey 形如 qrscene_xxx；
// 已关注用户扫码推送的 SCAN 事件，EventKey 直接就是 xxx。业务侧通常希望拿到纯场景值，
// 因此统一用本函数去掉前缀；非该前缀的值原样返回。
func NormalizeWechatEventKey(eventKey string) string {
	trimmed := strings.TrimSpace(eventKey)
	return strings.TrimPrefix(trimmed, wechatPushEventKeyScenePrefix)
}

// DecryptWechatPushMessage 解密安全/兼容模式下的密文消息，返回明文 XML 字符串。
//
// 解密流程（微信约定）：
//  1. EncodingAESKey（43 位）补一个 "=" 后做 base64 解码，得到 32 字节 AES 密钥；
//  2. 密文 base64 解码后得到字节串；
//  3. AES-256-CBC 解密，IV 取密钥前 16 字节；
//  4. 去掉 PKCS#7 填充（注意：微信按 32 字节块填充，填充值范围为 1-32，不是 16）；
//  5. 明文结构为：16 字节随机数 + 4 字节网络序消息长度 + 消息明文 + AppID，
//     按长度取出中间的消息明文部分。
func DecryptWechatPushMessage(encodingAESKey string, encrypted string) (string, error) {
	key, err := decodeWechatAESKey(encodingAESKey)
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return "", fmt.Errorf("decode wechat push ciphertext failed: %w", err)
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid wechat push ciphertext length %d", len(ciphertext))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create wechat aes cipher failed: %w", err)
	}
	// IV 固定取密钥前 16 字节，这是微信消息加解密的约定。
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plaintext, ciphertext)

	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return "", err
	}
	// 16 字节随机数 + 4 字节长度，至少要有 20 字节头部。
	if len(plaintext) < 20 {
		return "", fmt.Errorf("wechat push plaintext too short: %d", len(plaintext))
	}
	msgLen := int(binary.BigEndian.Uint32(plaintext[16:20]))
	if msgLen <= 0 || 20+msgLen > len(plaintext) {
		return "", fmt.Errorf("invalid wechat push message length %d", msgLen)
	}
	return string(plaintext[20 : 20+msgLen]), nil
}

// decodeWechatAESKey 把 43 位 EncodingAESKey 还原为 32 字节 AES 密钥。
func decodeWechatAESKey(encodingAESKey string) ([]byte, error) {
	key := strings.TrimSpace(encodingAESKey)
	if key == "" {
		return nil, fmt.Errorf("wechat encoding aes key is empty")
	}
	// 微信 EncodingAESKey 是“去掉填充”的 base64，需要补回 "=" 才能解码。
	if !strings.HasSuffix(key, "=") {
		key += "="
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("decode wechat encoding aes key failed: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("wechat encoding aes key must decode to 32 bytes, got %d", len(decoded))
	}
	return decoded, nil
}

// pkcs7Unpad 去掉 PKCS#7 填充，并校验填充合法性。
//
// 注意微信的特殊约定：消息体按 32 字节块填充（wechatPKCS7BlockSize），
// 因此填充值可能是 1-32，不能按 AES 的 16 字节块去限制，否则填充值大于 16
// 的消息（例如恰好需要 17-32 填充的关注事件）会被误判为非法而解密失败。
func pkcs7Unpad(data []byte) ([]byte, error) {
	// 解密结果仍是 AES 块（16 字节）的整数倍。
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid pkcs7 data length %d", len(data))
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > wechatPKCS7BlockSize || padding > len(data) {
		return nil, fmt.Errorf("invalid pkcs7 padding value %d", padding)
	}
	for _, b := range data[len(data)-padding:] {
		if int(b) != padding {
			return nil, fmt.Errorf("invalid pkcs7 padding bytes")
		}
	}
	return data[:len(data)-padding], nil
}

// ========================= 被动回复 =========================
//
// 被动回复：在收到微信推送的那个 HTTP 响应里直接返回一段 XML，用户就会在会话里看到消息。
// 关键约束：
//   - 必须在 5 秒内返回，否则微信会给用户提示“该公众号暂时无法提供服务”并重试 3 次；
//   - 安全模式下回包必须是密文（见 BuildWechatEncryptedReply），明文回包会被微信丢弃；
//   - 文本内容 UTF-8 编码最长 2048 字节，超出不展示；
//   - 不需要回复时返回空串即可，微信不会重试。
//
// 注意方向：回复里 ToUserName 是用户 OpenID（原推送的 FromUserName），
// FromUserName 是公众号原始 ID（原推送的 ToUserName），不要写反。

// WechatTextReply 描述一条被动回复文本消息。
type WechatTextReply struct {
	ToUserName   string // 接收方：用户 OpenID（原推送 FromUserName）
	FromUserName string // 发送方：公众号原始 ID（原推送 ToUserName）
	Content      string // 文本内容，UTF-8 最长 2048 字节
}

// BuildWechatTextReplyXML 按微信要求构造被动回复文本消息的明文 XML。
//
// CreateTime 由本函数取当前时间；内容用 CDATA 包裹，避免特殊字符破坏 XML。
func BuildWechatTextReplyXML(reply *WechatTextReply) ([]byte, error) {
	if reply == nil {
		return nil, fmt.Errorf("wechat text reply is nil")
	}
	if strings.TrimSpace(reply.ToUserName) == "" {
		return nil, fmt.Errorf("wechat text reply ToUserName(openid) is empty")
	}
	if strings.TrimSpace(reply.FromUserName) == "" {
		return nil, fmt.Errorf("wechat text reply FromUserName(official account) is empty")
	}
	if reply.Content == "" {
		return nil, fmt.Errorf("wechat text reply content is empty")
	}

	// 内容用 CDATA 包裹：拼接字符串而非 xml.Marshal，避免内容被实体转义后破坏展示。
	return []byte(fmt.Sprintf(
		"<xml><ToUserName><![CDATA[%s]]></ToUserName><FromUserName><![CDATA[%s]]></FromUserName><CreateTime>%d</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[%s]]></Content></xml>",
		reply.ToUserName,
		reply.FromUserName,
		time.Now().Unix(),
		reply.Content,
	)), nil
}

// BuildWechatEncryptedReply 把明文回复 XML 加密成微信要求的密文回包 XML。
//
// 安全模式下回包格式（XML）：
//
//	<xml>
//	  <Encrypt><![CDATA[密文]]></Encrypt>
//	  <MsgSignature><![CDATA[签名]]></MsgSignature>
//	  <TimeStamp>时间戳</TimeStamp>
//	  <Nonce><![CDATA[随机数]]></Nonce>
//	</xml>
//
// 其中 MsgSignature = sha1(sort(token, timestamp, nonce, encrypt))，与请求校验同算法。
// timestamp/nonce 由本函数生成；微信校验时只要求签名自洽，不要求与请求相同。
func BuildWechatEncryptedReply(config *WechatMessageConfig, plainReplyXML []byte) ([]byte, error) {
	if config == nil {
		return nil, fmt.Errorf("wechat message config is nil")
	}
	encrypt, err := EncryptWechatPushMessage(config.EncodingAESKey, config.AppID, string(plainReplyXML))
	if err != nil {
		return nil, err
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := generateWechatNonce()
	if err != nil {
		return nil, err
	}
	msgSignature := BuildWechatPushEncryptedSignature(config.Token, timestamp, nonce, encrypt)

	return []byte(fmt.Sprintf(
		"<xml><Encrypt><![CDATA[%s]]></Encrypt><MsgSignature><![CDATA[%s]]></MsgSignature><TimeStamp>%s</TimeStamp><Nonce><![CDATA[%s]]></Nonce></xml>",
		encrypt,
		msgSignature,
		timestamp,
		nonce,
	)), nil
}

// EncryptWechatPushMessage 是解密（DecryptWechatPushMessage）的逆过程：
// 明文结构为 16 字节随机数 + 4 字节网络序长度 + 明文 + AppID，
// 按 32 字节块 PKCS#7 填充后 AES-256-CBC 加密，IV 取密钥前 16 字节，最后 base64。
func EncryptWechatPushMessage(encodingAESKey string, appID string, plainMessage string) (string, error) {
	key, err := decodeWechatAESKey(encodingAESKey)
	if err != nil {
		return "", err
	}

	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate wechat random prefix failed: %w", err)
	}
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(plainMessage)))

	raw := make([]byte, 0, 16+4+len(plainMessage)+len(appID))
	raw = append(raw, random...)
	raw = append(raw, length...)
	raw = append(raw, []byte(plainMessage)...)
	raw = append(raw, []byte(appID)...)

	padded := pkcs7Pad(raw, wechatPKCS7BlockSize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create wechat aes cipher failed: %w", err)
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ciphertext, padded)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// pkcs7Pad 按指定块大小做 PKCS#7 填充（微信约定 32 字节块）。
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	for i := 0; i < padding; i++ {
		data = append(data, byte(padding))
	}
	return data
}

// generateWechatNonce 生成一个随机 nonce（十进制数字串），用于回包签名。
func generateWechatNonce() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate wechat nonce failed: %w", err)
	}
	return strconv.FormatUint(binary.BigEndian.Uint64(buf), 10), nil
}
