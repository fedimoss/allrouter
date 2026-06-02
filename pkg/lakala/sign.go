// Package lakala 提供拉卡拉支付网关的签名、验签和证书加载功能。
//
// 签名算法：SHA256-with-RSA，使用商户私钥对请求体签名，生成 Authorization 头。
// 验签算法：使用拉卡拉平台公钥证书验证回调签名，防止数据被篡改和伪造。
//
// Authorization 头格式：
//
//	LKLAPI-SHA256withRSA appid="xxx",serial_no="xxx",timestamp="xxx",nonce_str="xxx",signature="xxx"
//
// 待签名字符串（请求）：
//
//	appid\nserial_no\ntimestamp\nnonce_str\nbody\n
//
// 待签名字符串（回调通知）：
//
//	timestamp\nnonce_str\nbody\n
package lakala

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

// nonceChars 是生成随机字符串（Nonce）时使用的字符集，
// 包含大小写英文字母和数字。
const nonceChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateNonceStr 生成一个12位的随机字符串，用于签名的 Nonce 字段。
// 使用 crypto/rand 保证密码学安全的随机性。
func GenerateNonceStr() (string, error) {
	const nonceLen = 12 // Nonce 固定长度12位
	result := make([]byte, nonceLen)
	for i := 0; i < nonceLen; i++ {
		// 从字符集中随机选取一个字符
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(nonceChars))))
		if err != nil {
			return "", fmt.Errorf("generate nonce failed: %w", err)
		}
		result[i] = nonceChars[n.Int64()]
	}
	return string(result), nil
}

// BuildSignString 构造拉卡拉请求签名的待签名字符串。
//
// 格式：appid\nserial_no\ntimestamp\nnonce_str\nbody\n
// 时间戳由当前时间自动生成。
func BuildSignString(appID, serialNo, nonceStr, body string) string {
	timeStamp := strconv.FormatInt(time.Now().Unix(), 10)
	return BuildSignStringWithTimestamp(appID, serialNo, timeStamp, nonceStr, body)
}

// BuildSignStringWithTimestamp 使用指定时间戳构造待签名字符串。
// 与 BuildSignString 的区别在于时间戳由调用方传入，通常用于验证签名时重现签名字符串。
func BuildSignStringWithTimestamp(appID, serialNo, timeStamp, nonceStr, body string) string {
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n",
		appID,     // 接入应用ID
		serialNo,  // 证书序列号
		timeStamp, // Unix时间戳（秒）
		nonceStr,  // 随机字符串
		body,      // 请求体JSON
	)
}

// BuildNotifySignString 构造拉卡拉回调通知的待签名字符串。
//
// 格式：timestamp\nnonce_str\nbody\n
// 回调通知的签名字符串不包含 appID 和 serialNo，与请求签名格式不同。
func BuildNotifySignString(timeStamp, nonceStr, body string) string {
	return fmt.Sprintf("%s\n%s\n%s\n", timeStamp, nonceStr, body)
}

// LoadRSAPrivateKey 加载RSA商户私钥。
//
// 参数 path 可以是文件路径，也可以是PEM格式的字符串（内联配置）。
// 支持 PKCS#8 和 PKCS#1 两种私钥格式。
func LoadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	// 加载密钥数据：优先按文件路径读取，文件不存在时视为内联PEM字符串
	data, err := loadPathOrInlineData(path)
	if err != nil {
		return nil, fmt.Errorf("read private key failed: %w", err)
	}

	// 解码PEM块
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid private key PEM")
	}

	// 优先尝试 PKCS#8 格式解析（现代标准格式）
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}

	// PKCS#8 解析失败时，回退到 PKCS#1 格式（传统RSA私钥格式）
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key failed: %w", err)
	}
	return rsaKey, nil
}

// SignResult 是签名操作的结果，包含完整的签名信息和生成的 Authorization 头。
type SignResult struct {
	AppID         string `json:"appid"`         // 接入应用ID
	SerialNo      string `json:"serial_no"`     // 证书序列号
	TimeStamp     string `json:"timestamp"`     // 签名时的Unix时间戳（秒）
	NonceStr      string `json:"nonce_str"`     // 随机字符串（防重放）
	Signature     string `json:"signature"`     // Base64编码的RSA签名值
	Authorization string `json:"authorization"` // 完整的Authorization请求头值
	SignBody      string `json:"sign_body"`     // 用于签名的原始待签名字符串（调试用）
}

// LoadCertificate 加载X.509公钥证书。
//
// 参数 path 可以是文件路径，也可以是PEM格式的字符串（内联配置）。
func LoadCertificate(path string) (*x509.Certificate, error) {
	// 加载证书数据：优先按文件路径读取，文件不存在时视为内联PEM字符串
	data, err := loadPathOrInlineData(path)
	if err != nil {
		return nil, fmt.Errorf("read certificate failed: %w", err)
	}

	// 解码PEM块，提取DER格式的证书二进制数据
	block, _ := pem.Decode(data)
	if block != nil {
		data = block.Bytes
	}

	// 解析X.509证书
	cert, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, fmt.Errorf("parse certificate failed: %w", err)
	}
	return cert, nil
}

// loadPathOrInlineData 加载配置数据：先尝试作为文件路径读取，
// 文件不存在时视为内联的原始数据字符串。
func loadPathOrInlineData(value string) ([]byte, error) {
	// 先尝试作为文件路径读取
	if data, err := os.ReadFile(value); err == nil {
		return data, nil
	}
	// 空值直接返回错误
	if value == "" {
		return nil, fmt.Errorf("empty config value")
	}
	// 文件读取失败则将value本身视为PEM数据
	return []byte(value), nil
}

// VerifySignHeaders 是接口响应验签所需的请求头字段集合。
type VerifySignHeaders struct {
	AppID     string // 应用ID（Lklapi-Appid）
	SerialNo  string // 证书序列号（Lklapi-Serial）
	TimeStamp string // 时间戳（Lklapi-Timestamp）
	NonceStr  string // 随机字符串（Lklapi-Nonce）
	Signature string // 签名值（Lklapi-Signature）
	TraceID   string // 链路追踪ID（Lklapi-Traceid）
}

// AuthorizationInfo 是从 Authorization 头中解析出的签名参数。
type AuthorizationInfo struct {
	AppID     string // 应用ID
	SerialNo  string // 证书序列号
	TimeStamp string // 时间戳
	NonceStr  string // 随机字符串
	Signature string // 签名值
}

// ParseAuthorization 解析拉卡拉 Authorization 请求头。
//
// 格式示例：
//
//	LKLAPI-SHA256withRSA appid="xxx",serial_no="xxx",timestamp="xxx",nonce_str="xxx",signature="xxx"
//
// 返回值包含解析出的各个签名字段，缺少必填字段（timestamp/nonce_str/signature）时返回错误。
func ParseAuthorization(value string) (*AuthorizationInfo, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("missing Authorization")
	}

	// 校验 scheme 前缀（不区分大小写）
	const scheme = "LKLAPI-SHA256withRSA"
	if !strings.HasPrefix(strings.ToUpper(value), strings.ToUpper(scheme)) {
		return nil, fmt.Errorf("unsupported Authorization scheme")
	}

	// 去除 scheme 前缀，得到 key=value 字段列表
	fields := strings.TrimSpace(value[len(scheme):])
	if fields == "" {
		return nil, fmt.Errorf("empty Authorization fields")
	}

	// 按逗号分割，逐个解析 key="value" 格式的字段
	parsed := map[string]string{}
	for _, field := range strings.Split(fields, ",") {
		// 按等号分割为 key 和 value
		name, rawValue, ok := strings.Cut(strings.TrimSpace(field), "=")
		if !ok {
			return nil, fmt.Errorf("invalid Authorization field %q", field)
		}
		// key 转小写，value 去外层引号
		name = strings.ToLower(strings.TrimSpace(name))
		rawValue = strings.Trim(strings.TrimSpace(rawValue), `"`)
		parsed[name] = rawValue
	}

	// 提取各字段到结构体
	info := &AuthorizationInfo{
		AppID:     parsed["appid"],
		SerialNo:  parsed["serial_no"],
		TimeStamp: parsed["timestamp"],
		NonceStr:  parsed["nonce_str"],
		Signature: parsed["signature"],
	}

	// timestamp、nonce_str、signature 为必填字段
	if info.TimeStamp == "" || info.NonceStr == "" || info.Signature == "" {
		return nil, fmt.Errorf("Authorization missing timestamp, nonce_str or signature")
	}
	return info, nil
}

// Verify 验证拉卡拉接口响应的签名。
//
// 参数：
//   - certPath: 拉卡拉平台公钥证书（文件路径或PEM字符串）
//   - headers: 响应头中的验签相关字段
//   - body: 响应体原文
//
// 验证通过返回 nil，失败返回错误。
func Verify(certPath string, headers VerifySignHeaders, body string) error {
	// 使用响应头中的字段重新构造待签名字符串
	verifyString := BuildSignStringWithTimestamp(
		headers.AppID,
		headers.SerialNo,
		headers.TimeStamp,
		headers.NonceStr,
		body,
	)
	// 执行RSA签名验证
	return verifySignString(certPath, headers.Signature, verifyString)
}

// VerifyNotify 验证拉卡拉异步回调通知的签名。
//
// 与 Verify 的区别：回调通知使用不同的待签名字符串格式（不含 appID 和 serialNo），
// 签名参数从 Authorization 头中解析。
func VerifyNotify(certPath string, authorization string, body string) error {
	// 解析 Authorization 头，提取签名参数
	info, err := ParseAuthorization(authorization)
	if err != nil {
		return err
	}

	// 构造回调专用的待签名字符串（timestamp\nnonce_str\nbody\n）
	verifyString := BuildNotifySignString(info.TimeStamp, info.NonceStr, body)

	// 执行RSA签名验证
	return verifySignString(certPath, info.Signature, verifyString)
}

// verifySignString 执行RSA签名验证的核心逻辑。
//
// 流程：
//  1. 加载平台公钥证书
//  2. 从证书中提取RSA公钥
//  3. 对待签名字符串做SHA256哈希
//  4. 对签名值做Base64解码
//  5. 使用 RSA PKCS1v15 算法验证签名
func verifySignString(certPath string, signatureBase64 string, signString string) error {
	// 加载拉卡拉平台公钥证书
	cert, err := LoadCertificate(certPath)
	if err != nil {
		return fmt.Errorf("load certificate failed: %w", err)
	}

	// 提取RSA公钥
	rsaPubKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("certificate public key is not RSA")
	}

	// 对待签名字符串做SHA256哈希
	hash := sha256.Sum256([]byte(signString))

	// Base64解码签名值
	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("decode signature failed: %w", err)
	}

	// RSA PKCS1v15 签名验证
	if err := rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, hash[:], signature); err != nil {
		return fmt.Errorf("verify signature failed: %w", err)
	}
	return nil
}

// Sign 使用商户私钥对请求体进行RSA签名，返回包含 Authorization 头的完整签名结果。
//
// 流程：
//  1. 加载商户RSA私钥
//  2. 生成随机 Nonce 字符串
//  3. 获取当前Unix时间戳
//  4. 构造待签名字符串
//  5. 对待签名字符串做SHA256哈希
//  6. 使用RSA PKCS1v15 + SHA256 签名
//  7. Base64编码签名值
//  8. 拼装 Authorization 头
func Sign(appID, serialNo, privateKeyPath, body string) (*SignResult, error) {
	// 加载商户RSA私钥
	privateKey, err := LoadRSAPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load private key failed: %w", err)
	}

	// 生成12位随机Nonce字符串
	nonceStr, err := GenerateNonceStr()
	if err != nil {
		return nil, err
	}

	// 生成当前Unix时间戳（秒）
	timeStamp := strconv.FormatInt(time.Now().Unix(), 10)

	// 构造待签名字符串：appid\nserial_no\ntimestamp\nnonce_str\nbody\n
	signString := BuildSignStringWithTimestamp(appID, serialNo, timeStamp, nonceStr, body)

	// 对待签名字符串做SHA256哈希
	hash := sha256.Sum256([]byte(signString))

	// 使用 RSA PKCS1v15 + SHA256 算法签名
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("RSA sign failed: %w", err)
	}

	// Base64编码签名值
	signatureBase64 := base64.StdEncoding.EncodeToString(signature)

	// 拼装完整的 Authorization 头
	authorization := fmt.Sprintf(
		`LKLAPI-SHA256withRSA appid="%s",serial_no="%s",timestamp="%s",nonce_str="%s",signature="%s"`,
		appID, serialNo, timeStamp, nonceStr, signatureBase64,
	)

	return &SignResult{
		AppID:         appID,
		SerialNo:      serialNo,
		TimeStamp:     timeStamp,
		NonceStr:      nonceStr,
		Signature:     signatureBase64,
		Authorization: authorization,
		SignBody:      signString,
	}, nil
}
