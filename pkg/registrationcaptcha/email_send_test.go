package registrationcaptcha

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEmailSendFailureReleasesBothCooldowns(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "redis"}[shared], func(t *testing.T) {
			configureStore(t, shared)
			ctx := context.Background()
			reservation, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
			require.NoError(t, err)
			require.Zero(t, wait)
			require.NotNil(t, reservation)
			require.NoError(t, reservation.Release(ctx))
			require.NoError(t, reservation.Release(ctx), "rollback must be idempotent")

			// Separate requests prove that both the IP and email locks were freed.
			byIP, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.1", "other@example.com")
			require.NoError(t, err)
			require.Zero(t, wait)
			require.NotNil(t, byIP)
			byEmail, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.2", " USER@EXAMPLE.COM ")
			require.NoError(t, err)
			require.Zero(t, wait)
			require.NotNil(t, byEmail)

			// Repeating the earlier rollback must not delete either new lock.
			require.NoError(t, reservation.Release(ctx))
			for _, request := range [][2]string{
				{"192.0.2.1", "third@example.com"},
				{"192.0.2.3", "user@example.com"},
			} {
				blocked, wait, err := ReserveEmailSend(ctx, 7, request[0], request[1])
				require.NoError(t, err)
				require.Nil(t, blocked)
				require.Greater(t, wait, 59*time.Second)
			}
		})
	}
}

func TestExpiredEmailSendRollbackPreservesNewerCooldowns(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "memory", true: "redis"}[shared], func(t *testing.T) {
			server := configureStore(t, shared)
			ctx := context.Background()
			previous, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
			require.NoError(t, err)
			require.Zero(t, wait)
			if shared {
				server.FastForward(EmailSendInterval)
			} else {
				memoryStore.Lock()
				for key, entry := range memoryStore.entries {
					entry.expiresAt = time.Now().Add(-time.Second)
					memoryStore.entries[key] = entry
				}
				memoryStore.Unlock()
			}
			current, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
			require.NoError(t, err)
			require.Zero(t, wait)
			require.NotNil(t, current)
			require.NoError(t, previous.Release(ctx), "a late SMTP failure must not roll back a newer send")
			blocked, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
			require.NoError(t, err)
			require.Nil(t, blocked)
			require.Greater(t, wait, 59*time.Second)
			require.NoError(t, current.Release(ctx))
			retry, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
			require.NoError(t, err)
			require.Zero(t, wait)
			require.NotNil(t, retry)
		})
	}
}

func TestEmailSendHonorsExistingRedisCooldown(t *testing.T) {
	server := configureStore(t, true)
	ctx := context.Background()
	key := keyNamespace + "mail_email:7:" + digest("user@example.com")
	// Before owner tokens, cooldowns were stored as numeric rate-limit counters.
	require.NoError(t, server.Set(key, "1"))
	server.SetTTL(key, 30*time.Second)
	reservation, wait, err := ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
	require.NoError(t, err)
	require.Nil(t, reservation)
	require.Equal(t, 30*time.Second, wait)
	require.False(t, server.Exists(keyNamespace+"mail_ip:"+digest("192.0.2.1")))
	server.FastForward(30 * time.Second)
	reservation, wait, err = ReserveEmailSend(ctx, 7, "192.0.2.1", "user@example.com")
	require.NoError(t, err)
	require.Zero(t, wait)
	require.NotNil(t, reservation)
	require.NoError(t, reservation.Release(ctx))
}
