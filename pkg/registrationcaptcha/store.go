package registrationcaptcha

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

const (
	ChallengeTTL        = 2 * time.Minute
	EmailTokenTTL       = 2 * time.Minute
	RegistrationTTL     = 10 * time.Minute
	EmailSendInterval   = 60 * time.Second
	maxMemoryEntries    = 32768
	keyNamespace        = "new-api:registration_captcha:v1:"
	EmailPurpose        = "email"
	RegistrationPurpose = "register"
)

var ErrStoreUnavailable = errors.New("registration captcha store unavailable")

// Scope binds a challenge and its grants to one browser, IP, tenant and email.
// Only the opaque Session identifier is put in the signed session cookie;
// answers and grants are always kept server-side.
type Scope struct {
	ProviderID int
	Session    string
	IP         string
	Email      string
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (scope Scope) binding() string {
	value := fmt.Sprintf("%d\x00%s\x00%s\x00%s", scope.ProviderID, scope.Session, scope.IP, NormalizeEmail(scope.Email))
	return digest(value)
}

func digest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", hash)
}

func NewSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func validToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == token
}

type memoryEntry struct {
	binding   string
	value     string
	count     int
	expiresAt time.Time
}

var memoryStore = struct {
	sync.Mutex
	entries   map[string]memoryEntry
	nextSweep time.Time
}{entries: make(map[string]memoryEntry)}

// Redis failures deliberately fail closed. A per-process fallback when Redis
// is configured would permit replay and cooldown bypass on another node.
var readTokenScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value or string.sub(value, 1, 65) ~= ARGV[1] then
    return false
end
if ARGV[2] == "1" then
    redis.call("DEL", KEYS[1])
end
return string.sub(value, 66)
`)

// All keys are checked before any counter is changed. Rejected requests never
// extend the window; INCR and PEXPIRE are performed atomically.
var reserveWindowScript = redis.NewScript(`
local wait = 0
for _, key in ipairs(KEYS) do
    local count = tonumber(redis.call("GET", key) or "0")
    if count >= tonumber(ARGV[1]) then
        local ttl = redis.call("PTTL", key)
        if ttl < 0 then
            ttl = tonumber(ARGV[2])
            redis.call("PEXPIRE", key, ttl)
        end
        wait = math.max(wait, math.max(1, ttl))
    end
end
if wait > 0 then
    return wait
end
for _, key in ipairs(KEYS) do
    if redis.call("INCR", key) == 1 then
        redis.call("PEXPIRE", key, ARGV[2])
    end
end
return 0
`)

// Email sends reserve both keys with an owner token so a failed request can
// release only its own reservation, even if it finishes after the window ends.
// Existing numeric cooldown keys also remain reserved until their TTL expires.
var reserveEmailSendScript = redis.NewScript(`
local wait = 0
for _, key in ipairs(KEYS) do
    if redis.call("EXISTS", key) == 1 then
        local ttl = redis.call("PTTL", key)
        if ttl < 0 then
            ttl = tonumber(ARGV[2])
            redis.call("PEXPIRE", key, ttl)
        end
        wait = math.max(wait, math.max(1, ttl))
    end
end
if wait > 0 then
    return wait
end
for _, key in ipairs(KEYS) do
    redis.call("SET", key, ARGV[1], "PX", ARGV[2])
end
return 0
`)

var releaseEmailSendScript = redis.NewScript(`
for _, key in ipairs(KEYS) do
    if redis.call("GET", key) == ARGV[1] then
        redis.call("DEL", key)
    end
end
return 1
`)

func sweepMemoryLocked(now time.Time, additions int) error {
	if now.After(memoryStore.nextSweep) || len(memoryStore.entries)+additions > maxMemoryEntries {
		for key, entry := range memoryStore.entries {
			if !now.Before(entry.expiresAt) {
				delete(memoryStore.entries, key)
			}
		}
		memoryStore.nextSweep = now.Add(time.Minute)
	}
	if len(memoryStore.entries)+additions > maxMemoryEntries {
		return fmt.Errorf("%w: in-memory capacity reached", ErrStoreUnavailable)
	}
	return nil
}

func putToken(ctx context.Context, kind, token string, scope Scope, value string, ttl time.Duration) error {
	key := keyNamespace + kind + ":" + digest(token)
	binding := scope.binding()
	if common.RedisEnabled {
		if common.RDB == nil {
			return ErrStoreUnavailable
		}
		if err := common.RDB.Set(ctx, key, binding+":"+value, ttl).Err(); err != nil {
			return fmt.Errorf("%w: %w", ErrStoreUnavailable, err)
		}
		return nil
	}

	memoryStore.Lock()
	defer memoryStore.Unlock()
	now := time.Now()
	if err := sweepMemoryLocked(now, 1); err != nil {
		return err
	}
	memoryStore.entries[key] = memoryEntry{binding: binding, value: value, expiresAt: now.Add(ttl)}
	return nil
}

func readToken(ctx context.Context, kind, token string, scope Scope, consume bool) (string, bool, error) {
	if !validToken(token) || scope.Session == "" || scope.IP == "" {
		return "", false, nil
	}
	key := keyNamespace + kind + ":" + digest(token)
	binding := scope.binding()
	if common.RedisEnabled {
		if common.RDB == nil {
			return "", false, ErrStoreUnavailable
		}
		consumeArg := "0"
		if consume {
			consumeArg = "1"
		}
		value, err := readTokenScript.Run(ctx, common.RDB, []string{key}, binding+":", consumeArg).Text()
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		if err != nil {
			return "", false, fmt.Errorf("%w: %w", ErrStoreUnavailable, err)
		}
		return value, true, nil
	}

	memoryStore.Lock()
	defer memoryStore.Unlock()
	entry, exists := memoryStore.entries[key]
	if !exists {
		return "", false, nil
	}
	if !time.Now().Before(entry.expiresAt) {
		delete(memoryStore.entries, key)
		return "", false, nil
	}
	if entry.binding != binding {
		return "", false, nil
	}
	if consume {
		delete(memoryStore.entries, key)
	}
	return entry.value, true, nil
}

func CheckGrant(ctx context.Context, token string, scope Scope) (bool, error) {
	_, ok, err := readToken(ctx, RegistrationPurpose, token, scope, false)
	return ok, err
}

func ConsumeGrant(ctx context.Context, token string, scope Scope, purpose string) (bool, error) {
	if purpose != EmailPurpose && purpose != RegistrationPurpose {
		return false, nil
	}
	_, ok, err := readToken(ctx, purpose, token, scope, true)
	return ok, err
}

func reserveWindow(ctx context.Context, keys []string, limit int, ttl time.Duration) (time.Duration, error) {
	if common.RedisEnabled {
		if common.RDB == nil {
			return 0, ErrStoreUnavailable
		}
		wait, err := reserveWindowScript.Run(ctx, common.RDB, keys, limit, ttl.Milliseconds()).Int64()
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrStoreUnavailable, err)
		}
		return time.Duration(wait) * time.Millisecond, nil
	}

	memoryStore.Lock()
	defer memoryStore.Unlock()
	now := time.Now()
	var wait time.Duration
	additions := 0
	for _, key := range keys {
		entry, exists := memoryStore.entries[key]
		if !exists || !now.Before(entry.expiresAt) {
			delete(memoryStore.entries, key)
			additions++
			continue
		}
		if entry.count >= limit && entry.expiresAt.Sub(now) > wait {
			wait = entry.expiresAt.Sub(now)
		}
	}
	if wait > 0 {
		return wait, nil
	}
	if err := sweepMemoryLocked(now, additions); err != nil {
		return 0, err
	}
	for _, key := range keys {
		entry, exists := memoryStore.entries[key]
		if !exists {
			entry.expiresAt = now.Add(ttl)
		}
		entry.count++
		memoryStore.entries[key] = entry
	}
	return 0, nil
}

type EmailSendReservation struct {
	keys        []string
	owner       string
	redisClient *redis.Client
}

// Release rolls back an unsuccessful send without clearing a newer request's
// cooldown. Successful sends retain the reservation until its normal expiry.
func (reservation *EmailSendReservation) Release(ctx context.Context) error {
	if reservation == nil {
		return nil
	}
	if reservation.redisClient != nil {
		if err := releaseEmailSendScript.Run(ctx, reservation.redisClient, reservation.keys, reservation.owner).Err(); err != nil {
			return fmt.Errorf("%w: %w", ErrStoreUnavailable, err)
		}
		return nil
	}

	memoryStore.Lock()
	defer memoryStore.Unlock()
	for _, key := range reservation.keys {
		if entry, exists := memoryStore.entries[key]; exists && entry.value == reservation.owner {
			delete(memoryStore.entries, key)
		}
	}
	return nil
}

// ReserveEmailSend enforces the same 60-second interval for registration and
// account email binding. Both the IP and normalized tenant/email must be free.
// A rejected request receives no reservation and changes neither cooldown.
func ReserveEmailSend(ctx context.Context, providerID int, ip, email string) (*EmailSendReservation, time.Duration, error) {
	keys := []string{
		keyNamespace + "mail_ip:" + digest(ip),
		keyNamespace + "mail_email:" + strconv.Itoa(providerID) + ":" + digest(NormalizeEmail(email)),
	}
	owner, err := NewSessionID()
	if err != nil {
		return nil, 0, err
	}
	reservation := &EmailSendReservation{keys: keys, owner: owner}
	if common.RedisEnabled {
		if common.RDB == nil {
			return nil, 0, ErrStoreUnavailable
		}
		reservation.redisClient = common.RDB
		wait, err := reserveEmailSendScript.Run(ctx, reservation.redisClient, keys, owner, EmailSendInterval.Milliseconds()).Int64()
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %w", ErrStoreUnavailable, err)
		}
		if wait > 0 {
			return nil, time.Duration(wait) * time.Millisecond, nil
		}
		return reservation, 0, nil
	}

	memoryStore.Lock()
	defer memoryStore.Unlock()
	now := time.Now()
	var wait time.Duration
	for _, key := range keys {
		if entry, exists := memoryStore.entries[key]; exists && now.Before(entry.expiresAt) && entry.expiresAt.Sub(now) > wait {
			wait = entry.expiresAt.Sub(now)
		}
	}
	if wait > 0 {
		return nil, wait, nil
	}
	for _, key := range keys {
		delete(memoryStore.entries, key)
	}
	if err := sweepMemoryLocked(now, len(keys)); err != nil {
		return nil, 0, err
	}
	for _, key := range keys {
		memoryStore.entries[key] = memoryEntry{value: owner, expiresAt: now.Add(EmailSendInterval)}
	}
	return reservation, 0, nil
}
