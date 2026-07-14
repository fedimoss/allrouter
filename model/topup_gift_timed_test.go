package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNormalizeTopUpGiftTimedConfig(t *testing.T) {
	now := time.Date(2026, time.July, 14, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))

	raw, err := NormalizeTopUpGiftTimedConfig(`{"enabled":true,"day":3,"end_time":1}`, now)
	require.NoError(t, err)

	config, err := ParseTopUpGiftTimedConfig(raw)
	require.NoError(t, err)
	require.True(t, config.Enabled)
	require.Equal(t, 3, config.Day)
	require.Equal(t, now.AddDate(0, 0, 3).Unix(), config.EndTime)

	raw, err = NormalizeTopUpGiftTimedConfig(`{"enabled":false,"day":7,"end_time":1}`, now)
	require.NoError(t, err)
	config, err = ParseTopUpGiftTimedConfig(raw)
	require.NoError(t, err)
	require.False(t, config.Enabled)
	require.Equal(t, 7, config.Day)
	require.Zero(t, config.EndTime)
}

func TestNormalizeTopUpGiftTimedConfigRejectsInvalidInput(t *testing.T) {
	now := time.Now()

	_, err := NormalizeTopUpGiftTimedConfig(`{"enabled":true,"day":0}`, now)
	require.ErrorContains(t, err, "至少为 1")

	_, err = NormalizeTopUpGiftTimedConfig(`{"enabled":false,"day":-1}`, now)
	require.ErrorContains(t, err, "不能小于 0")

	_, err = NormalizeTopUpGiftTimedConfig(`not-json`, now)
	require.ErrorContains(t, err, "格式无效")
}

func TestLoadTopUpGiftTimedConfigUsesProviderScopeWithoutGlobalFallback(t *testing.T) {
	oldDB := DB
	oldGlobal := common.TopUpGiftTimed
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}, &ProviderOption{}))
	DB = db
	common.TopUpGiftTimed = `{"enabled":true,"day":5}`
	t.Cleanup(func() {
		DB = oldDB
		common.TopUpGiftTimed = oldGlobal
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	globalEndBeforeLoad := time.Now().AddDate(0, 0, 5).Unix()
	global, err := LoadTopUpGiftTimedConfig(0)
	require.NoError(t, err)
	require.True(t, global.Enabled)
	require.GreaterOrEqual(t, global.EndTime, globalEndBeforeLoad)
	require.LessOrEqual(t, global.EndTime, time.Now().AddDate(0, 0, 5).Unix())
	var globalOption Option
	require.NoError(t, DB.Where("key = ?", "TopUpGiftTimed").First(&globalOption).Error)
	persistedGlobal, err := ParseTopUpGiftTimedConfig(globalOption.Value)
	require.NoError(t, err)
	require.Equal(t, global.EndTime, persistedGlobal.EndTime)
	globalAgain, err := LoadTopUpGiftTimedConfig(0)
	require.NoError(t, err)
	require.Equal(t, global.EndTime, globalAgain.EndTime)

	missingProvider, err := LoadTopUpGiftTimedConfig(42)
	require.NoError(t, err)
	require.False(t, missingProvider.Enabled)
	require.Zero(t, missingProvider.EndTime)

	require.NoError(t, UpdateProviderOption(42, ProviderTopUpGiftTimedOptionKey, `{"enabled":true,"day":2}`))
	providerEndBeforeLoad := time.Now().AddDate(0, 0, 2).Unix()
	providerConfig, err := LoadTopUpGiftTimedConfig(42)
	require.NoError(t, err)
	require.True(t, providerConfig.Enabled)
	require.Equal(t, 2, providerConfig.Day)
	require.GreaterOrEqual(t, providerConfig.EndTime, providerEndBeforeLoad)
	providerRaw, err := GetProviderOptionValue(42, ProviderTopUpGiftTimedOptionKey)
	require.NoError(t, err)
	persistedProvider, err := ParseTopUpGiftTimedConfig(providerRaw)
	require.NoError(t, err)
	require.Equal(t, providerConfig.EndTime, persistedProvider.EndTime)
	providerAgain, err := LoadTopUpGiftTimedConfig(42)
	require.NoError(t, err)
	require.Equal(t, providerConfig.EndTime, providerAgain.EndTime)
}
