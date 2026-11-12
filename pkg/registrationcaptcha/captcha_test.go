package registrationcaptcha

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func configureStore(t *testing.T, shared bool) *miniredis.Miniredis {
	t.Helper()
	oldEnabled, oldClient := common.RedisEnabled, common.RDB
	memoryStore.Lock()
	oldEntries, oldSweep := memoryStore.entries, memoryStore.nextSweep
	memoryStore.entries = make(map[string]memoryEntry)
	memoryStore.nextSweep = time.Time{}
	memoryStore.Unlock()
	common.RedisEnabled = shared
	var server *miniredis.Miniredis
	if shared {
		server = miniredis.RunT(t)
		common.RDB = redis.NewClient(&redis.Options{
			Addr: server.Addr(), MaxRetries: -1,
			DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second,
		})
		client := common.RDB
		t.Cleanup(func() { _ = client.Close() })
	}
	t.Cleanup(func() {
		common.RedisEnabled, common.RDB = oldEnabled, oldClient
		memoryStore.Lock()
		memoryStore.entries, memoryStore.nextSweep = oldEntries, oldSweep
		memoryStore.Unlock()
	})
	return server
}

func browserScope() Scope {
	return Scope{ProviderID: 7, Session: "browser-one", IP: "192.0.2.7", Email: "user@example.com"}
}

func readyChallenge(t *testing.T, scope Scope) (*Challenge, []Click) {
	t.Helper()
	ctx := context.Background()
	challenge, err := Create(ctx, scope)
	require.NoError(t, err)
	encoded, ok, err := readToken(ctx, "challenge", challenge.ID, scope, false)
	require.NoError(t, err)
	require.True(t, ok)
	var expected answer
	require.NoError(t, common.UnmarshalJsonStr(encoded, &expected))
	// Advance only the issue timestamp to avoid a real wait in unit tests.
	expected.IssuedAt = time.Now().Add(-2500 * time.Millisecond).UnixMilli()
	encodedBytes, err := common.Marshal(expected)
	require.NoError(t, err)
	require.NoError(t, putToken(ctx, "challenge", challenge.ID, scope, string(encodedBytes), ChallengeTTL))
	clicks := make([]Click, len(expected.Targets))
	for i, target := range expected.Targets {
		clicks[i] = Click{X: target.X, Y: target.Y, TimeMS: int64(i+1) * 250}
	}
	return challenge, clicks
}

func TestChallengeImagesDoNotExposeAnswers(t *testing.T) {
	configureStore(t, false)
	challenge, err := Create(context.Background(), browserScope())
	require.NoError(t, err)
	require.True(t, validToken(challenge.ID))
	require.Equal(t, RequiredClicks, challenge.RequiredClicks)
	require.Equal(t, 120, challenge.ExpiresIn)
	encoded, err := common.Marshal(challenge)
	require.NoError(t, err)
	for _, privateField := range []string{"Targets", "IssuedAt", "Radius", "answer", "seed", "glyph"} {
		require.NotContains(t, string(encoded), privateField)
	}
	for i, data := range []string{challenge.Image, challenge.PromptImage} {
		require.True(t, strings.HasPrefix(data, "data:image/png;base64,"))
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(data, "data:image/png;base64,"))
		require.NoError(t, err)
		img, err := png.Decode(bytes.NewReader(decoded))
		require.NoError(t, err)
		require.Equal(t, imageWidth*imageScale, img.Bounds().Dx())
		if i == 0 {
			require.Equal(t, imageHeight*imageScale, img.Bounds().Dy())
		}
	}
	second, err := Create(context.Background(), browserScope())
	require.NoError(t, err)
	require.NotEqual(t, challenge.ID, second.ID)
	require.NotEqual(t, challenge.Image, second.Image)
	require.NotEqual(t, challenge.PromptImage, second.PromptImage)
}

func TestCaptchaOneAttemptAndSeparateOneUseGrants(t *testing.T) {
	for _, shared := range []bool{false, true} {
		name := "memory"
		if shared {
			name = "redis"
		}
		t.Run(name, func(t *testing.T) {
			configureStore(t, shared)
			ctx, scope := context.Background(), browserScope()
			challenge, clicks := readyChallenge(t, scope)
			grants, err := Verify(ctx, challenge.ID, scope, clicks)
			require.NoError(t, err)
			require.NotEqual(t, grants.EmailToken, grants.RegistrationToken)
			_, err = Verify(ctx, challenge.ID, scope, clicks)
			require.ErrorIs(t, err, ErrInvalidChallenge)
			ok, err := ConsumeGrant(ctx, grants.EmailToken, scope, RegistrationPurpose)
			require.NoError(t, err)
			require.False(t, ok, "an email grant cannot authorize registration")
			ok, err = ConsumeGrant(ctx, grants.EmailToken, scope, EmailPurpose)
			require.NoError(t, err)
			require.True(t, ok)
			ok, err = ConsumeGrant(ctx, grants.EmailToken, scope, EmailPurpose)
			require.NoError(t, err)
			require.False(t, ok)
			ok, err = CheckGrant(ctx, grants.RegistrationToken, scope)
			require.NoError(t, err)
			require.True(t, ok, "email sending must preserve the registration grant")
			ok, err = ConsumeGrant(ctx, grants.RegistrationToken, scope, RegistrationPurpose)
			require.NoError(t, err)
			require.True(t, ok)
			ok, err = CheckGrant(ctx, grants.RegistrationToken, scope)
			require.NoError(t, err)
			require.False(t, ok)

			challenge, clicks = readyChallenge(t, scope)
			wrong := append([]Click(nil), clicks...)
			wrong[0].X = 0
			wrong[0].Y = 0
			_, err = Verify(ctx, challenge.ID, scope, wrong)
			require.ErrorIs(t, err, ErrInvalidChallenge)
			_, err = Verify(ctx, challenge.ID, scope, clicks)
			require.ErrorIs(t, err, ErrInvalidChallenge, "incorrect guesses must consume the challenge")
		})
	}
}

func TestCaptchaScopeBinding(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "redis"}[shared], func(t *testing.T) {
			configureStore(t, shared)
			ctx, scope := context.Background(), browserScope()
			challenge, clicks := readyChallenge(t, scope)
			wrongScopes := []Scope{scope, scope, scope, scope}
			wrongScopes[0].Session = "another-browser"
			wrongScopes[1].Email = "another@example.com"
			wrongScopes[2].ProviderID++
			wrongScopes[3].IP = "192.0.2.8"
			for _, wrong := range wrongScopes {
				_, err := Verify(ctx, challenge.ID, wrong, clicks)
				require.ErrorIs(t, err, ErrInvalidChallenge)
			}
			grants, err := Verify(ctx, challenge.ID, scope, clicks)
			require.NoError(t, err, "wrong scopes must not destroy the legitimate challenge")
			for _, wrong := range wrongScopes {
				ok, err := ConsumeGrant(ctx, grants.RegistrationToken, wrong, RegistrationPurpose)
				require.NoError(t, err)
				require.False(t, ok)
			}
			scope.Email = " USER@EXAMPLE.COM "
			ok, err := ConsumeGrant(ctx, grants.RegistrationToken, scope, RegistrationPurpose)
			require.NoError(t, err)
			require.True(t, ok, "email normalization must match verification-code storage")
		})
	}
}

func TestConcurrentGrantConsumption(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "redis"}[shared], func(t *testing.T) {
			configureStore(t, shared)
			ctx, scope := context.Background(), browserScope()
			for _, purpose := range []string{EmailPurpose, RegistrationPurpose} {
				token, err := NewSessionID()
				require.NoError(t, err)
				require.NoError(t, putToken(ctx, purpose, token, scope, "ok", RegistrationTTL))
				var successes atomic.Int64
				var workers sync.WaitGroup
				errorsCh := make(chan error, 32)
				for i := 0; i < 32; i++ {
					workers.Add(1)
					go func() {
						defer workers.Done()
						ok, err := ConsumeGrant(ctx, token, scope, purpose)
						if err != nil {
							errorsCh <- err
						}
						if ok {
							successes.Add(1)
						}
					}()
				}
				workers.Wait()
				close(errorsCh)
				for err := range errorsCh {
					require.NoError(t, err)
				}
				require.EqualValues(t, 1, successes.Load())
			}
		})
	}
}

func TestCaptchaAndGrantExpiration(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "redis"}[shared], func(t *testing.T) {
			server := configureStore(t, shared)
			ctx, scope := context.Background(), browserScope()
			challenge, clicks := readyChallenge(t, scope)
			emailToken, err := NewSessionID()
			require.NoError(t, err)
			registerToken, err := NewSessionID()
			require.NoError(t, err)
			require.NoError(t, putToken(ctx, EmailPurpose, emailToken, scope, "ok", EmailTokenTTL))
			require.NoError(t, putToken(ctx, RegistrationPurpose, registerToken, scope, "ok", RegistrationTTL))
			if shared {
				server.FastForward(ChallengeTTL + time.Second)
			} else {
				memoryStore.Lock()
				for _, pair := range [][2]string{{"challenge", challenge.ID}, {EmailPurpose, emailToken}} {
					key := keyNamespace + pair[0] + ":" + digest(pair[1])
					entry := memoryStore.entries[key]
					entry.expiresAt = time.Now().Add(-time.Second)
					memoryStore.entries[key] = entry
				}
				memoryStore.Unlock()
			}
			_, err = Verify(ctx, challenge.ID, scope, clicks)
			require.ErrorIs(t, err, ErrInvalidChallenge)
			ok, err := ConsumeGrant(ctx, emailToken, scope, EmailPurpose)
			require.NoError(t, err)
			require.False(t, ok)
			ok, err = CheckGrant(ctx, registerToken, scope)
			require.NoError(t, err)
			require.True(t, ok, "registration remains valid for the email-code entry step")
			if shared {
				server.FastForward(RegistrationTTL)
			} else {
				memoryStore.Lock()
				key := keyNamespace + RegistrationPurpose + ":" + digest(registerToken)
				entry := memoryStore.entries[key]
				entry.expiresAt = time.Now().Add(-time.Second)
				memoryStore.entries[key] = entry
				memoryStore.Unlock()
			}
			ok, err = CheckGrant(ctx, registerToken, scope)
			require.NoError(t, err)
			require.False(t, ok)
		})
	}
}

func TestAnswerValidation(t *testing.T) {
	now := time.Now()
	expected := answer{IssuedAt: now.Add(-2 * time.Second).UnixMilli()}
	clicks := make([]Click, RequiredClicks)
	for i := range clicks {
		expected.Targets = append(expected.Targets, target{X: 30 + 60*float64(i), Y: 50, Radius: 26})
		clicks[i] = Click{X: expected.Targets[i].X, Y: 50, TimeMS: int64(i+1) * 250}
	}
	require.True(t, matchesAnswer(expected, clicks, now))
	for name, change := range map[string]func([]Click) []Click{
		"count":                       func(value []Click) []Click { return value[:4] },
		"order":                       func(value []Click) []Click { value[0], value[1] = value[1], value[0]; return value },
		"out of bounds":               func(value []Click) []Click { value[0].X = -1; return value },
		"not finite":                  func(value []Click) []Click { value[0].Y = math.Inf(1); return value },
		"nan":                         func(value []Click) []Click { value[0].Y = math.NaN(); return value },
		"fabricated future timestamp": func(value []Click) []Click { value[4].TimeMS = 90000; return value },
		"nonmonotonic timestamps":     func(value []Click) []Click { value[2].TimeMS = value[1].TimeMS; return value },
	} {
		t.Run(name, func(t *testing.T) {
			require.False(t, matchesAnswer(expected, change(append([]Click(nil), clicks...)), now))
		})
	}
	expected.IssuedAt = now.Add(-200 * time.Millisecond).UnixMilli()
	require.False(t, matchesAnswer(expected, clicks, now))
	expected.IssuedAt = now.Add(-3 * time.Minute).UnixMilli()
	require.False(t, matchesAnswer(expected, clicks, now))
}

func TestChallengeRateLimitCannotBeResetByEmailOrTenant(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "redis"}[shared], func(t *testing.T) {
			configureStore(t, shared)
			for i := 0; i < 5; i++ {
				scope := browserScope()
				scope.ProviderID = i
				scope.Email = strings.Repeat("a", i+1) + "@example.com"
				_, err := Create(context.Background(), scope)
				require.NoError(t, err)
			}
			_, err := Create(context.Background(), browserScope())
			var limited *RateLimitError
			require.True(t, errors.As(err, &limited))
			require.Greater(t, limited.RetryAfter, 55*time.Second)
		})
	}
}

func TestEmailCooldownByIPAndNormalizedEmail(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "redis"}[shared], func(t *testing.T) {
			server := configureStore(t, shared)
			ctx := context.Background()
			reservation, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
			require.NoError(t, err)
			require.Zero(t, wait)
			require.NotNil(t, reservation)
			for _, request := range [][2]string{
				{"192.0.2.2", " USER@EXAMPLE.COM "},
				{"192.0.2.1", "another@example.com"},
			} {
				reservation, wait, err = ReserveEmailSend(ctx, 7, request[0], request[1])
				require.NoError(t, err)
				require.Nil(t, reservation)
				require.Greater(t, wait, 59*time.Second)
				require.LessOrEqual(t, wait, EmailSendInterval)
			}
			// A blocked IP must not lock the different email it attempted to use.
			reservation, wait, err = ReserveEmailSend(ctx, 7, "192.0.2.2", "another@example.com")
			require.NoError(t, err)
			require.Zero(t, wait)
			require.NotNil(t, reservation)
			if shared {
				server.FastForward(59 * time.Second)
			} else {
				memoryStore.Lock()
				for key, entry := range memoryStore.entries {
					entry.expiresAt = time.Now().Add(time.Second)
					memoryStore.entries[key] = entry
				}
				memoryStore.Unlock()
			}
			reservation, wait, err = ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
			require.NoError(t, err)
			require.Nil(t, reservation)
			require.Greater(t, wait, time.Duration(0))
			require.LessOrEqual(t, wait, time.Second)
			if shared {
				server.FastForward(time.Second)
			} else {
				memoryStore.Lock()
				for key, entry := range memoryStore.entries {
					entry.expiresAt = time.Now().Add(-time.Second)
					memoryStore.entries[key] = entry
				}
				memoryStore.Unlock()
			}
			reservation, wait, err = ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
			require.NoError(t, err)
			require.NotNil(t, reservation)
			require.Zero(t, wait, "rejected attempts must not extend the cooldown")
		})
	}
}

func TestConcurrentEmailCooldown(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "redis"}[shared], func(t *testing.T) {
			configureStore(t, shared)
			var successes atomic.Int64
			var workers sync.WaitGroup
			errorsCh := make(chan error, 24)
			for i := 0; i < 24; i++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					reservation, wait, err := ReserveEmailSend(context.Background(), 7, "192.0.2.1", "user@example.com")
					if err != nil {
						errorsCh <- err
					} else if wait == 0 && reservation != nil {
						successes.Add(1)
					}
				}()
			}
			workers.Wait()
			close(errorsCh)
			for err := range errorsCh {
				require.NoError(t, err)
			}
			require.EqualValues(t, 1, successes.Load())
		})
	}
}

func TestConfiguredRedisFailsClosed(t *testing.T) {
	configureStore(t, false)
	ctx, scope := context.Background(), browserScope()
	token, err := NewSessionID()
	require.NoError(t, err)
	require.NoError(t, putToken(ctx, RegistrationPurpose, token, scope, "ok", RegistrationTTL))
	common.RedisEnabled, common.RDB = true, nil
	_, err = CheckGrant(ctx, token, scope)
	require.ErrorIs(t, err, ErrStoreUnavailable)
	_, err = ConsumeGrant(ctx, token, scope, RegistrationPurpose)
	require.ErrorIs(t, err, ErrStoreUnavailable)
	_, err = Create(ctx, scope)
	require.ErrorIs(t, err, ErrStoreUnavailable)
	_, _, err = ReserveEmailSend(ctx, 7, scope.IP, scope.Email)
	require.ErrorIs(t, err, ErrStoreUnavailable)
	common.RedisEnabled = false
	ok, err := CheckGrant(ctx, token, scope)
	require.NoError(t, err)
	require.True(t, ok, "Redis failures must never consume a process-local fallback")
}

func TestMemoryStoreBoundedWithoutEvictingValidGrants(t *testing.T) {
	configureStore(t, false)
	memoryStore.Lock()
	for i := 0; i < maxMemoryEntries; i++ {
		memoryStore.entries[string(rune(i))] = memoryEntry{expiresAt: time.Now().Add(time.Minute)}
	}
	memoryStore.Unlock()
	token, err := NewSessionID()
	require.NoError(t, err)
	err = putToken(context.Background(), RegistrationPurpose, token, browserScope(), "ok", RegistrationTTL)
	require.ErrorIs(t, err, ErrStoreUnavailable)
	memoryStore.Lock()
	for key, entry := range memoryStore.entries {
		entry.expiresAt = time.Now().Add(-time.Second)
		memoryStore.entries[key] = entry
	}
	memoryStore.Unlock()
	require.NoError(t, putToken(context.Background(), RegistrationPurpose, token, browserScope(), "ok", RegistrationTTL))
	memoryStore.Lock()
	defer memoryStore.Unlock()
	require.Len(t, memoryStore.entries, 1)
}
