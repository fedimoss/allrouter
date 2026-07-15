package common

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

type verificationValue struct {
	code string
	time time.Time
}

const (
	EmailVerificationPurpose = "v"
	PasswordResetPurpose     = "r"

	verificationKeyNamespace = "new-api:verification_code:v1"
)

var (
	verificationMutex        sync.Mutex
	verificationMap          map[string]verificationValue
	verificationMapMaxSize   = 10
	VerificationValidMinutes = 10

	// ErrVerificationStoreUnavailable lets callers distinguish an infrastructure
	// failure from an invalid or expired verification code.
	ErrVerificationStoreUnavailable = errors.New("verification code store unavailable")

	// A successful verification must consume the code atomically. GET followed by
	// DEL would allow two concurrent requests to use the same code successfully.
	verificationConsumeScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value or value ~= ARGV[1] then
    return 0
end
redis.call("DEL", KEYS[1])
return 1
`)
)

func GenerateVerificationCode(length int) string {
	code := uuid.New().String()
	code = strings.Replace(code, "-", "", -1)
	if length == 0 {
		return code
	}
	return code[:length]
}

// verificationStorageKey scopes codes by provider and purpose. The normalized
// email is hashed so Redis key listings do not expose users' email addresses.
func verificationStorageKey(providerId int, key string, purpose string) string {
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	digest := sha256.Sum256([]byte(normalizedKey))
	return fmt.Sprintf("%s:%d:%s:%x", verificationKeyNamespace, providerId, purpose, digest)
}

func verificationCodeTTL() time.Duration {
	return time.Duration(VerificationValidMinutes) * time.Minute
}

// RegisterVerificationCodeWithKey stores a verification code in the shared
// Redis store when Redis is enabled. Redis failures are returned to the caller;
// falling back to this process' map would reintroduce cross-node inconsistency.
func RegisterVerificationCodeWithKey(ctx context.Context, providerId int, key string, code string, purpose string) error {
	storageKey := verificationStorageKey(providerId, key, purpose)
	if RedisEnabled {
		if RDB == nil {
			return fmt.Errorf("%w: redis client is nil", ErrVerificationStoreUnavailable)
		}
		if err := RDB.Set(ctx, storageKey, code, verificationCodeTTL()).Err(); err != nil {
			return fmt.Errorf("%w: set code: %w", ErrVerificationStoreUnavailable, err)
		}
		return nil
	}

	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap[storageKey] = verificationValue{
		code: code,
		time: time.Now(),
	}
	if len(verificationMap) > verificationMapMaxSize {
		removeExpiredPairs()
	}
	return nil
}

// VerifyCodeWithKey atomically verifies and consumes a code. It returns
// (false, nil) for a missing, expired, or incorrect code, and a non-nil error
// only when the configured shared store is unavailable.
func VerifyCodeWithKey(ctx context.Context, providerId int, key string, code string, purpose string) (bool, error) {
	return consumeVerificationCode(ctx, providerId, key, code, purpose)
}

// InvalidateVerificationCodeWithKey removes code only if it still has the
// expected value. This is used to roll back a failed email send without deleting
// a newer code written by a concurrent resend request.
func InvalidateVerificationCodeWithKey(ctx context.Context, providerId int, key string, code string, purpose string) error {
	_, err := consumeVerificationCode(ctx, providerId, key, code, purpose)
	return err
}

func consumeVerificationCode(ctx context.Context, providerId int, key string, code string, purpose string) (bool, error) {
	storageKey := verificationStorageKey(providerId, key, purpose)
	if RedisEnabled {
		if RDB == nil {
			return false, fmt.Errorf("%w: redis client is nil", ErrVerificationStoreUnavailable)
		}
		matched, err := verificationConsumeScript.Run(ctx, RDB, []string{storageKey}, code).Int()
		if err != nil {
			return false, fmt.Errorf("%w: consume code: %w", ErrVerificationStoreUnavailable, err)
		}
		return matched == 1, nil
	}

	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	value, ok := verificationMap[storageKey]
	if !ok {
		return false, nil
	}
	if time.Since(value.time) >= verificationCodeTTL() {
		delete(verificationMap, storageKey)
		return false, nil
	}
	if subtle.ConstantTimeCompare([]byte(code), []byte(value.code)) != 1 {
		return false, nil
	}
	delete(verificationMap, storageKey)
	return true, nil
}

// no lock inside, so the caller must lock the verificationMap before calling!
func removeExpiredPairs() {
	now := time.Now()
	ttl := verificationCodeTTL()
	for key := range verificationMap {
		if now.Sub(verificationMap[key].time) >= ttl {
			delete(verificationMap, key)
		}
	}
}

func init() {
	verificationMutex.Lock()
	defer verificationMutex.Unlock()
	verificationMap = make(map[string]verificationValue)
}
