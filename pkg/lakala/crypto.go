// Package lakala 拉卡拉 API 签名/加密工具
package lakala

import (
	"bytes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/tjfoc/gmsm/sm4"
)

const (
	// Sm4KeySize SM4 密钥长度（16字节 / 128位）
	Sm4KeySize = 16
)

// GenerateSm4Key 生成一个 Base64 编码的 SM4 密钥（128位）
func GenerateSm4Key() (string, error) {
	key := make([]byte, Sm4KeySize)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("生成SM4密钥失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// GenerateSm4KeyBytes 生成 SM4 密钥字节（128位）
func GenerateSm4KeyBytes() ([]byte, error) {
	key := make([]byte, Sm4KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成SM4密钥失败: %w", err)
	}
	return key, nil
}

// Sm4EncryptECB SM4/ECB/PKCS5Padding 加密
// key: Base64 编码的密钥
// plaintext: 明文字符串
// 返回: Base64 编码的密文
func Sm4EncryptECB(keyBase64 string, plaintext string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return "", fmt.Errorf("密钥Base64解码失败: %w", err)
	}
	return sm4EncryptECB(key, []byte(plaintext))
}

// Sm4EncryptECBBytes SM4/ECB/PKCS5Padding 加密（字节版本）
func Sm4EncryptECBBytes(key []byte, plaintext []byte) (string, error) {
	return sm4EncryptECB(key, plaintext)
}

func sm4EncryptECB(key []byte, plaintext []byte) (string, error) {
	if len(key) != Sm4KeySize {
		return "", fmt.Errorf("SM4密钥长度必须为%d字节，当前为%d字节", Sm4KeySize, len(key))
	}

	block, err := sm4.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建SM4 cipher失败: %w", err)
	}

	// PKCS5 padding
	padded := pkcs5Padding(plaintext, block.BlockSize())

	// ECB 加密
	encrypted, err := ecbEncrypt(block, padded)
	if err != nil {
		return "", fmt.Errorf("SM4 ECB加密失败: %w", err)
	}

	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// Sm4DecryptECB SM4/ECB/PKCS5Padding 解密
// keyBase64: Base64 编码的密钥
// cipherBase64: Base64 编码的密文
// 返回: 明文字符串
func Sm4DecryptECB(keyBase64 string, cipherBase64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return "", fmt.Errorf("密钥Base64解码失败: %w", err)
	}

	cipherData, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return "", fmt.Errorf("密文Base64解码失败: %w", err)
	}

	plaintext, err := sm4DecryptECB(key, cipherData)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Sm4DecryptECBBytes SM4/ECB/PKCS5Padding 解密（字节版本）
func Sm4DecryptECBBytes(key []byte, cipherData []byte) ([]byte, error) {
	return sm4DecryptECB(key, cipherData)
}

func sm4DecryptECB(key []byte, cipherData []byte) ([]byte, error) {
	if len(key) != Sm4KeySize {
		return nil, fmt.Errorf("SM4密钥长度必须为%d字节，当前为%d字节", Sm4KeySize, len(key))
	}

	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建SM4 cipher失败: %w", err)
	}

	// ECB 解密
	decrypted, err := ecbDecrypt(block, cipherData)
	if err != nil {
		return nil, fmt.Errorf("SM4 ECB解密失败: %w", err)
	}

	// 去除 PKCS5 padding
	unpadded, err := pkcs5Unpadding(decrypted)
	if err != nil {
		return nil, fmt.Errorf("PKCS5去填充失败: %w", err)
	}

	return unpadded, nil
}

// pkcs5Padding PKCS5 填充
func pkcs5Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs5Unpadding 去除 PKCS5 填充
func pkcs5Unpadding(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("数据为空，无法去除填充")
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding > sm4.BlockSize {
		return nil, fmt.Errorf("无效的PKCS5填充长度: %d", padding)
	}
	return data[:len(data)-padding], nil
}

// ecbEncrypt ECB 模式加密
func ecbEncrypt(block cipher.Block, src []byte) ([]byte, error) {
	blockSize := block.BlockSize()
	if len(src)%blockSize != 0 {
		return nil, fmt.Errorf("ECB加密输入长度%d不是块大小%d的倍数", len(src), blockSize)
	}

	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += blockSize {
		block.Encrypt(dst[i:i+blockSize], src[i:i+blockSize])
	}
	return dst, nil
}

// ecbDecrypt ECB 模式解密
func ecbDecrypt(block cipher.Block, src []byte) ([]byte, error) {
	blockSize := block.BlockSize()
	if len(src)%blockSize != 0 {
		return nil, fmt.Errorf("ECB解密输入长度%d不是块大小%d的倍数", len(src), blockSize)
	}

	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += blockSize {
		block.Decrypt(dst[i:i+blockSize], src[i:i+blockSize])
	}
	return dst, nil
}
