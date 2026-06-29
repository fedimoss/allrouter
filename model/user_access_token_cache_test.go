package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserAccessTokenCacheTest(t *testing.T) {
	t.Helper()

	oldDB := DB
	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled
	oldSyncFrequency := common.SyncFrequency

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	DB = db

	redisServer := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	common.RedisEnabled = true
	common.SyncFrequency = 60

	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
		common.SyncFrequency = oldSyncFrequency
		DB = oldDB
	})
}

func TestValidateAccessTokenInProviderCachesLookup(t *testing.T) {
	setupUserAccessTokenCacheTest(t)

	accessToken := "management-token"
	user := User{
		ProviderId:  7,
		Username:    "alice",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		AccessToken: &accessToken,
	}
	require.NoError(t, DB.Create(&user).Error)

	first, err := ValidateAccessTokenInProvider("Bearer "+accessToken, 7)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, user.Id, first.Id)

	require.NoError(t, DB.Unscoped().Delete(&User{}, user.Id).Error)

	cached, err := ValidateAccessTokenInProvider(accessToken, 7)
	require.NoError(t, err)
	require.NotNil(t, cached)
	require.Equal(t, user.Id, cached.Id)
	require.Equal(t, "alice", cached.Username)
}

func TestUserUpdateInvalidatesPreviousAccessTokenCache(t *testing.T) {
	setupUserAccessTokenCacheTest(t)

	oldToken := "old-management-token"
	user := User{
		ProviderId:  7,
		Username:    "alice",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		AccessToken: &oldToken,
	}
	require.NoError(t, DB.Create(&user).Error)
	_, err := ValidateAccessTokenInProvider(oldToken, 7)
	require.NoError(t, err)

	newToken := "new-management-token"
	user.SetAccessToken(newToken)
	require.NoError(t, user.Update(false))

	oldLookup, err := ValidateAccessTokenInProvider(oldToken, 7)
	require.NoError(t, err)
	require.Nil(t, oldLookup)

	newLookup, err := ValidateAccessTokenInProvider(newToken, 7)
	require.NoError(t, err)
	require.NotNil(t, newLookup)
	require.Equal(t, user.Id, newLookup.Id)
}

func TestUserDeleteInvalidatesAccessTokenCache(t *testing.T) {
	setupUserAccessTokenCacheTest(t)

	accessToken := "delete-management-token"
	user := User{
		ProviderId:  7,
		Username:    "alice",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		AccessToken: &accessToken,
	}
	require.NoError(t, DB.Create(&user).Error)
	_, err := ValidateAccessTokenInProvider(accessToken, 7)
	require.NoError(t, err)

	require.NoError(t, user.Delete())

	deletedLookup, err := ValidateAccessTokenInProvider(accessToken, 7)
	require.NoError(t, err)
	require.Nil(t, deletedLookup)
}
