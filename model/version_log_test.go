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

func setupVersionLogCacheTest(t *testing.T) {
	t.Helper()

	oldDB := DB
	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&VersionLog{}))
	DB = db

	redisServer := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	common.RedisEnabled = true

	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
		DB = oldDB
	})
}

func TestRefreshLatestVersionLogCacheFromDBUsesNextNewestAfterLatestDeleted(t *testing.T) {
	setupVersionLogCacheTest(t)

	v10 := VersionLog{Version: "1.0", Log: "first", CreatedAt: 100, UpdatedAt: 100}
	v11 := VersionLog{Version: "1.1", Log: "second", CreatedAt: 200, UpdatedAt: 200}
	require.NoError(t, DB.Create(&v10).Error)
	require.NoError(t, DB.Create(&v11).Error)
	require.NoError(t, SetLatestVersionLogCache(&v11))

	require.NoError(t, DB.Where("id = ?", v11.Id).Delete(&VersionLog{}).Error)
	require.NoError(t, RefreshLatestVersionLogCacheFromDB())

	latest, err := GetLatestVersionLogCached()
	require.NoError(t, err)
	require.NotNil(t, latest)
	require.Equal(t, "1.0", latest.Version)
	require.Equal(t, "first", latest.Log)
}

func TestRefreshLatestVersionLogCacheFromDBClearsCacheWhenNoLogsRemain(t *testing.T) {
	setupVersionLogCacheTest(t)

	v10 := VersionLog{Version: "1.0", Log: "first", CreatedAt: 100, UpdatedAt: 100}
	require.NoError(t, DB.Create(&v10).Error)
	require.NoError(t, SetLatestVersionLogCache(&v10))
	require.NoError(t, DB.Where("id = ?", v10.Id).Delete(&VersionLog{}).Error)

	require.NoError(t, RefreshLatestVersionLogCacheFromDB())

	latest, err := GetLatestVersionLogCached()
	require.NoError(t, err)
	require.Nil(t, latest)
}
