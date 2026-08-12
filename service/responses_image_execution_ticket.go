package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	responsesImageExecutionTicketVersion       = 1
	responsesImageExecutionTicketDefaultTTL    = 5 * time.Minute
	responsesImageExecutionTicketMaximumTTL    = 15 * time.Minute
	responsesImageExecutionTicketMaximumLength = 24 * 1024
	responsesImageExecutionTicketAAD           = "responses-image-client-stream:v1"
)

var (
	ErrResponsesImageExecutionTicketInvalid = errors.New("invalid responses image execution ticket")
	ErrResponsesImageExecutionTicketExpired = errors.New("responses image execution ticket expired")
	ErrResponsesImageExecutionTicketReplay  = errors.New("responses image execution ticket already consumed")
	responsesImageExecutionTicketNow        = time.Now
)

// ResponsesImageExecutionTicketClaims 是 client_stream 的加密执行载荷。
// UserID/TokenID 仅用于执行时重新读取并验证当前账号状态；票据不包含用户令牌、
// 上游 API key 或图片渠道信息。ImageRequest 只来自网关内部规划结果。
type ResponsesImageExecutionTicketClaims struct {
	Version    int              `json:"v"`
	IssuedAt   int64            `json:"iat"`
	ExpiresAt  int64            `json:"exp"`
	Nonce      string           `json:"nonce"`
	UserID     int              `json:"user_id"`
	TokenID    int              `json:"token_id"`
	RequestID  string           `json:"request_id,omitempty"`
	CallID     string           `json:"call_id,omitempty"`
	CallNumber int              `json:"call_number"`
	Request    dto.ImageRequest `json:"request"`
}

var responsesImageExecutionTicketReplayCache = struct {
	sync.Mutex
	consumed map[string]int64
}{
	consumed: make(map[string]int64),
}

// IssueResponsesImageExecutionTicket 创建自包含的 AES-GCM 票据。
// 密文使用 Raw URL Base64，适合放入 shell 请求体；图片提示词不会出现在 URL 或命令中。
func IssueResponsesImageExecutionTicket(
	claims ResponsesImageExecutionTicketClaims,
	ttl time.Duration,
) (string, error) {
	if claims.UserID <= 0 || claims.TokenID <= 0 || claims.CallNumber < 0 ||
		strings.TrimSpace(claims.Request.Model) == "" || strings.TrimSpace(claims.Request.Prompt) == "" {
		return "", fmt.Errorf("%w: incomplete claims", ErrResponsesImageExecutionTicketInvalid)
	}

	ttl = normalizeResponsesImageExecutionTicketTTL(ttl)
	now := responsesImageExecutionTicketNow()
	claims.Version = responsesImageExecutionTicketVersion
	claims.IssuedAt = now.Unix()
	claims.ExpiresAt = now.Add(ttl).Unix()

	nonceBytes := make([]byte, 18)
	if _, err := io.ReadFull(rand.Reader, nonceBytes); err != nil {
		return "", fmt.Errorf("generate execution nonce: %w", err)
	}
	claims.Nonce = base64.RawURLEncoding.EncodeToString(nonceBytes)

	plaintext, err := common.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal execution ticket: %w", err)
	}
	gcm, err := responsesImageExecutionTicketGCM()
	if err != nil {
		return "", err
	}
	gcmNonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, gcmNonce); err != nil {
		return "", fmt.Errorf("generate execution ticket nonce: %w", err)
	}

	sealed := gcm.Seal(nil, gcmNonce, plaintext, []byte(responsesImageExecutionTicketAAD))
	packed := make([]byte, 1+len(gcmNonce)+len(sealed))
	packed[0] = byte(responsesImageExecutionTicketVersion)
	copy(packed[1:], gcmNonce)
	copy(packed[1+len(gcmNonce):], sealed)
	ticket := base64.RawURLEncoding.EncodeToString(packed)
	if len(ticket) > responsesImageExecutionTicketMaximumLength {
		return "", fmt.Errorf("%w: ticket exceeds command transport limit", ErrResponsesImageExecutionTicketInvalid)
	}
	return ticket, nil
}

// ConsumeResponsesImageExecutionTicket 校验、解密并原子消费票据。
// nonce 在图片渠道调用前写入短期内存集合，防止并发重放导致重复生成和重复计费。
func ConsumeResponsesImageExecutionTicket(ticket string) (*ResponsesImageExecutionTicketClaims, error) {
	ticket = strings.TrimSpace(ticket)
	if ticket == "" || len(ticket) > responsesImageExecutionTicketMaximumLength {
		return nil, ErrResponsesImageExecutionTicketInvalid
	}
	packed, err := base64.RawURLEncoding.DecodeString(ticket)
	if err != nil {
		return nil, ErrResponsesImageExecutionTicketInvalid
	}
	gcm, err := responsesImageExecutionTicketGCM()
	if err != nil {
		return nil, err
	}
	if len(packed) <= 1+gcm.NonceSize() || int(packed[0]) != responsesImageExecutionTicketVersion {
		return nil, ErrResponsesImageExecutionTicketInvalid
	}

	gcmNonce := packed[1 : 1+gcm.NonceSize()]
	ciphertext := packed[1+gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, gcmNonce, ciphertext, []byte(responsesImageExecutionTicketAAD))
	if err != nil {
		return nil, ErrResponsesImageExecutionTicketInvalid
	}
	var claims ResponsesImageExecutionTicketClaims
	if err := common.Unmarshal(plaintext, &claims); err != nil {
		return nil, ErrResponsesImageExecutionTicketInvalid
	}

	now := responsesImageExecutionTicketNow().Unix()
	if claims.Version != responsesImageExecutionTicketVersion || claims.UserID <= 0 ||
		claims.TokenID <= 0 || claims.CallNumber < 0 || claims.Nonce == "" ||
		strings.TrimSpace(claims.Request.Model) == "" || strings.TrimSpace(claims.Request.Prompt) == "" ||
		claims.IssuedAt <= 0 || claims.IssuedAt > now+30 || claims.ExpiresAt <= claims.IssuedAt ||
		claims.ExpiresAt-claims.IssuedAt > int64(responsesImageExecutionTicketMaximumTTL/time.Second) {
		return nil, ErrResponsesImageExecutionTicketInvalid
	}
	if claims.ExpiresAt <= now {
		return nil, ErrResponsesImageExecutionTicketExpired
	}
	if !consumeResponsesImageExecutionNonce(claims.Nonce, claims.ExpiresAt, now) {
		return nil, ErrResponsesImageExecutionTicketReplay
	}
	return &claims, nil
}

func normalizeResponsesImageExecutionTicketTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return responsesImageExecutionTicketDefaultTTL
	}
	if ttl > responsesImageExecutionTicketMaximumTTL {
		return responsesImageExecutionTicketMaximumTTL
	}
	return ttl
}

func responsesImageExecutionTicketGCM() (cipher.AEAD, error) {
	secret := strings.TrimSpace(common.CryptoSecret)
	if secret == "" {
		return nil, fmt.Errorf("responses image execution ticket secret is empty")
	}
	key := sha256.Sum256([]byte(responsesImageExecutionTicketAAD + "\x00" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create execution ticket cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create execution ticket AEAD: %w", err)
	}
	return gcm, nil
}

func consumeResponsesImageExecutionNonce(nonce string, expiresAt, now int64) bool {
	responsesImageExecutionTicketReplayCache.Lock()
	defer responsesImageExecutionTicketReplayCache.Unlock()

	for cachedNonce, cachedExpiry := range responsesImageExecutionTicketReplayCache.consumed {
		if cachedExpiry <= now {
			delete(responsesImageExecutionTicketReplayCache.consumed, cachedNonce)
		}
	}
	if _, exists := responsesImageExecutionTicketReplayCache.consumed[nonce]; exists {
		return false
	}
	responsesImageExecutionTicketReplayCache.consumed[nonce] = expiresAt
	return true
}
