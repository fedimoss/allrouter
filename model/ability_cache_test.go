package model

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelListCacheTest(t *testing.T) {
	t.Helper()

	oldDB := DB
	oldRDB := common.RDB
	oldRedisEnabled := common.RedisEnabled
	oldSyncFrequency := common.SyncFrequency
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	DB = db

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	initCol()

	redisServer := miniredis.RunT(t)
	common.RDB = redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	common.RedisEnabled = true
	common.SyncFrequency = 60

	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RDB = oldRDB
		common.RedisEnabled = oldRedisEnabled
		common.SyncFrequency = oldSyncFrequency
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		initCol()
		DB = oldDB
	})
}

func modelListCacheKeys(t *testing.T, pattern string) []string {
	t.Helper()

	keys, err := common.RDB.Keys(context.Background(), pattern).Result()
	require.NoError(t, err)
	return keys
}

func TestGetGroupEnabledModelsUsesRedisCacheAndInvalidatesOnAbilityStatusUpdate(t *testing.T) {
	setupModelListCacheTest(t)

	priority := int64(0)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "gpt-cache",
		ChannelId: 1,
		Enabled:   true,
		Priority:  &priority,
	}).Error)

	require.ElementsMatch(t, []string{"gpt-cache"}, GetGroupEnabledModels("default"))
	require.NotEmpty(t, modelListCacheKeys(t, "model_list_cache:group_enabled_models:*"))

	require.NoError(t, DB.Where("channel_id = ?", 1).Delete(&Ability{}).Error)
	require.ElementsMatch(t, []string{"gpt-cache"}, GetGroupEnabledModels("default"))

	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "gpt-cache",
		ChannelId: 1,
		Enabled:   true,
		Priority:  &priority,
	}).Error)
	require.NoError(t, UpdateAbilityStatus(1, false))
	require.Empty(t, modelListCacheKeys(t, "model_list_cache:*"))
	require.Empty(t, GetGroupEnabledModels("default"))
}

func TestGetEnabledModelsUsesRedisCacheAndInvalidatesOnBatchDeleteChannels(t *testing.T) {
	setupModelListCacheTest(t)

	priority := int64(0)
	require.NoError(t, DB.Create(&Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "key-1",
		Status: common.ChannelStatusEnabled,
		Name:   "channel-1",
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "gpt-enabled",
		ChannelId: 1,
		Enabled:   true,
		Priority:  &priority,
	}).Error)

	require.ElementsMatch(t, []string{"gpt-enabled"}, GetEnabledModels())
	require.NotEmpty(t, modelListCacheKeys(t, "model_list_cache:enabled_models"))

	require.NoError(t, DB.Where("channel_id = ?", 1).Delete(&Ability{}).Error)
	require.ElementsMatch(t, []string{"gpt-enabled"}, GetEnabledModels())

	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     "gpt-enabled",
		ChannelId: 1,
		Enabled:   true,
		Priority:  &priority,
	}).Error)
	require.NoError(t, BatchDeleteChannels([]int{1}))
	require.Empty(t, modelListCacheKeys(t, "model_list_cache:*"))
	require.Empty(t, GetEnabledModels())
}

func TestGetPreferredModelOwnerChannelTypesUsesRedisCacheAndInvalidatesOnChannelUpdate(t *testing.T) {
	setupModelListCacheTest(t)

	insertPreferredOwnerCandidate(t, 1, "gpt-owner", "default", constant.ChannelTypeOpenAI, 0, 0, common.ChannelStatusEnabled, true)

	owners, err := GetPreferredModelOwnerChannelTypes([]string{"gpt-owner"}, []string{"default"})
	require.NoError(t, err)
	require.Equal(t, constant.ChannelTypeOpenAI, owners["gpt-owner"])
	require.NotEmpty(t, modelListCacheKeys(t, "model_list_cache:preferred_owner:*"))

	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)

	cachedOwners, err := GetPreferredModelOwnerChannelTypes([]string{"gpt-owner"}, []string{"default"})
	require.NoError(t, err)
	require.Equal(t, constant.ChannelTypeOpenAI, cachedOwners["gpt-owner"])

	channel := Channel{
		Id:     2,
		Type:   constant.ChannelTypeCodex,
		Key:    "key-2",
		Status: common.ChannelStatusEnabled,
		Name:   "channel-2",
		Models: "gpt-owner",
		Group:  "default",
	}
	require.NoError(t, channel.Insert())
	require.Empty(t, modelListCacheKeys(t, "model_list_cache:*"))

	refreshedOwners, err := GetPreferredModelOwnerChannelTypes([]string{"gpt-owner"}, []string{"default"})
	require.NoError(t, err)
	require.Equal(t, constant.ChannelTypeCodex, refreshedOwners["gpt-owner"])
}
