package lakala

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// generateTestRSAKey 生成测试用的 RSA 私钥并写入临时 PEM 文件
func generateTestRSAKey(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pkcs1Bytes := x509.MarshalPKCS1PrivateKey(privateKey)
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pkcs1Bytes,
	}

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "test_private_key.pem")
	err = os.WriteFile(keyPath, pem.EncodeToMemory(pemBlock), 0600)
	require.NoError(t, err)

	return keyPath
}

func TestGenerateSm4Key(t *testing.T) {
	key, err := GenerateSm4Key()
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(key)
	require.NoError(t, err)
	require.Len(t, decoded, Sm4KeySize)
}

func TestSm4EncryptDecryptECB(t *testing.T) {
	key := "dRzPaYd7z6vYn9sL/JTZ3A=="

	tests := []struct {
		name      string
		plaintext string
	}{
		{"短文本", "hello world"},
		{"中文文本", "阿萨德哈的哦已我居然挤公交大幅度AAAADDF"},
		{"JSON报文", `{"req_data":{"member_id":"AAA200154561278"}}`},
		{"空字符串", ""},
		{"长文本", strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cipherText, err := Sm4EncryptECB(key, tt.plaintext)
			require.NoError(t, err)
			require.NotEmpty(t, cipherText)
			require.NotEqual(t, tt.plaintext, cipherText)

			decrypted, err := Sm4DecryptECB(key, cipherText)
			require.NoError(t, err)
			require.Equal(t, tt.plaintext, decrypted)
		})
	}
}

func TestSm4DecryptECB_InvalidKey(t *testing.T) {
	_, err := Sm4DecryptECB("not-base64!@#$", "dummy")
	require.Error(t, err)
}

func TestSm4DecryptECB_InvalidCipher(t *testing.T) {
	key := "dRzPaYd7z6vYn9sL/JTZ3A=="
	_, err := Sm4DecryptECB(key, "not-base64!@#$")
	require.Error(t, err)
}

func TestSm4EncryptECB_InvalidKeyLength(t *testing.T) {
	_, err := Sm4EncryptECB("dG9vLXNob3J0", "hello") // 7 bytes, not 16
	require.Error(t, err)
}

func TestGenerateNonceStr(t *testing.T) {
	for range 100 {
		nonce, err := GenerateNonceStr()
		require.NoError(t, err)
		require.Len(t, nonce, 12)

		for _, c := range nonce {
			require.True(t,
				(c >= 'a' && c <= 'z') ||
					(c >= 'A' && c <= 'Z') ||
					(c >= '0' && c <= '9'),
				"nonce 包含非法字符: %c", c)
		}
	}
}

func TestBuildSignString(t *testing.T) {
	signStr := BuildSignStringWithTimestamp(
		"8000000000001",
		"1610334026688401311",
		"1621690412",
		"123456789012",
		`{"reqData":{"member_id":"AAA200154561278"}}`,
	)

	// 验证格式：5行，每行以 \n 结尾（包括最后一行）
	lines := strings.Split(signStr, "\n")
	require.Len(t, lines, 6)
	require.Empty(t, lines[5])

	require.Equal(t, "8000000000001", lines[0])
	require.Equal(t, "1610334026688401311", lines[1])
	require.Equal(t, "1621690412", lines[2])
	require.Equal(t, "123456789012", lines[3])
	require.Equal(t, `{"reqData":{"member_id":"AAA200154561278"}}`, lines[4])

	require.True(t, strings.HasSuffix(signStr, "\n"),
		"待签名报文必须以 \\n 结尾")
}

func TestSign_SHA256withRSA(t *testing.T) {
	keyPath := generateTestRSAKey(t)

	result, err := Sign("8000000000001", "1610334026688401311", keyPath, `{"reqData":{}}`)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 基本字段
	require.Equal(t, "8000000000001", result.AppID)
	require.Equal(t, "1610334026688401311", result.SerialNo)
	require.NotEmpty(t, result.TimeStamp)
	require.Len(t, result.NonceStr, 12)
	require.NotEmpty(t, result.Signature)

	// Authorization header 格式
	require.Contains(t, result.Authorization, "LKLAPI-SHA256withRSA")
	require.Contains(t, result.Authorization, `appid="8000000000001"`)
	require.Contains(t, result.Authorization, `serial_no="1610334026688401311"`)
	require.Contains(t, result.Authorization, `timestamp="`)
	require.Contains(t, result.Authorization, `nonce_str="`)
	require.Contains(t, result.Authorization, `signature="`)

	// 待签名报文必须以 \n 结尾
	require.True(t, strings.HasSuffix(result.SignBody, "\n"),
		"待签名报文必须以 \\n 结尾")

	// 使用同一私钥生成的签名可以 Base64 解码
	_, err = base64.StdEncoding.DecodeString(result.Signature)
	require.NoError(t, err, "签名应为有效 Base64")
}

func TestSign_InvalidKeyPath(t *testing.T) {
	_, err := Sign("appid", "serial", "/nonexistent/key.pem", "body")
	require.Error(t, err)
}

func TestSign_InvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "bad.pem")
	err := os.WriteFile(badPath, []byte("not a pem file"), 0600)
	require.NoError(t, err)

	_, err = Sign("appid", "serial", badPath, "body")
	require.Error(t, err)
}

// generateTestCerts 生成测试用的 RSA 密钥对和证书
// 返回: (私钥.pem路径, 证书.cer路径, *rsa.PrivateKey)
func generateTestCerts(t *testing.T) (string, string, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// 创建自签名证书
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test.lakala.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	tmpDir := t.TempDir()

	// 写私钥 PEM
	pkcs1Bytes := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPath := filepath.Join(tmpDir, "test_private_key.pem")
	err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pkcs1Bytes,
	}), 0600)
	require.NoError(t, err)

	// 写证书 CER（DER 格式）
	certPath := filepath.Join(tmpDir, "test_cert.cer")
	err = os.WriteFile(certPath, certDER, 0600)
	require.NoError(t, err)

	return keyPath, certPath, privateKey
}

func TestSignAndVerify_RoundTrip(t *testing.T) {
	keyPath, certPath, _ := generateTestCerts(t)

	body := `{"respData":{"member_id":"AAA200154561278"}}`

	// 签名
	result, err := Sign("OP00000003", "00dfba8194c41b84cf", keyPath, body)
	require.NoError(t, err)
	require.NotEmpty(t, result.Signature)

	// 验签
	headers := VerifySignHeaders{
		AppID:     result.AppID,
		SerialNo:  result.SerialNo,
		TimeStamp: result.TimeStamp,
		NonceStr:  result.NonceStr,
		Signature: result.Signature,
	}
	err = Verify(certPath, headers, body)
	require.NoError(t, err, "签名→验签闭环应通过")
}

func TestSignAndVerify_RoundTripWithPEMContent(t *testing.T) {
	keyPath, certPath, _ := generateTestCerts(t)
	keyPEM, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	certDER, err := os.ReadFile(certPath)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	body := `{"resp_data":{"out_trade_no":"LKL-test"}}`

	result, err := Sign("OP00000003", "00dfba8194c41b84cf", string(keyPEM), body)
	require.NoError(t, err)
	require.NotEmpty(t, result.Signature)

	headers := VerifySignHeaders{
		AppID:     result.AppID,
		SerialNo:  result.SerialNo,
		TimeStamp: result.TimeStamp,
		NonceStr:  result.NonceStr,
		Signature: result.Signature,
	}
	err = Verify(string(certPEM), headers, body)
	require.NoError(t, err)
}

func TestVerify_WrongBody(t *testing.T) {
	keyPath, certPath, _ := generateTestCerts(t)

	result, err := Sign("OP00000003", "00dfba8194c41b84cf", keyPath, `{"right":true}`)
	require.NoError(t, err)

	headers := VerifySignHeaders{
		AppID:     result.AppID,
		SerialNo:  result.SerialNo,
		TimeStamp: result.TimeStamp,
		NonceStr:  result.NonceStr,
		Signature: result.Signature,
	}
	err = Verify(certPath, headers, `{"wrong":true}`)
	require.Error(t, err, "报文被篡改应验签失败")
}

func TestVerify_TamperedSignature(t *testing.T) {
	_, certPath, _ := generateTestCerts(t)

	headers := VerifySignHeaders{
		AppID:     "OP00000003",
		SerialNo:  "00dfba8194c41b84cf",
		TimeStamp: "1621690412",
		NonceStr:  "123456789012",
		Signature: "tampered",
	}
	err := Verify(certPath, headers, `{}`)
	require.Error(t, err, "签名被篡改应验签失败")
}

func TestLoadCertificate_DER(t *testing.T) {
	_, certPath, _ := generateTestCerts(t)

	cert, err := LoadCertificate(certPath)
	require.NoError(t, err)
	require.NotNil(t, cert)
	require.Equal(t, "test.lakala.com", cert.Subject.CommonName)
}

func TestVerify_InvalidCertPath(t *testing.T) {
	err := Verify("/nonexistent/cert.cer", VerifySignHeaders{}, "{}")
	require.Error(t, err)
}
