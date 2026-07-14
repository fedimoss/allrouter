package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetTopUpGiftConfigUsesCurrentSiteProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 使用独立内存库保存服务商配置，避免测试污染真实数据库。
	oldDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ProviderOption{}))
	model.DB = db

	// 主站配置位于进程级 OptionMap，测试结束后必须完整恢复。
	common.OptionMapRWMutex.Lock()
	oldEnabled := common.TopUpGiftEnabled
	oldRules := common.TopUpGiftRules
	oldTimed := common.TopUpGiftTimed
	common.TopUpGiftEnabled = true
	common.TopUpGiftRules = `[{"id":"main","threshold":10,"bonus":1}]`
	common.TopUpGiftTimed = `{"enabled":true,"day":7,"end_time":2000}`
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		model.DB = oldDB
		common.OptionMapRWMutex.Lock()
		common.TopUpGiftEnabled = oldEnabled
		common.TopUpGiftRules = oldRules
		common.TopUpGiftTimed = oldTimed
		common.OptionMapRWMutex.Unlock()
	})

	require.NoError(t, model.UpdateProviderOption(42, model.ProviderTopUpGiftEnabledOptionKey, "false"))
	require.NoError(t, model.UpdateProviderOption(42, model.ProviderTopUpGiftRulesOptionKey, `[{"id":"provider","threshold":20,"bonus":3}]`))
	require.NoError(t, model.UpdateProviderOption(42, model.ProviderTopUpGiftTimedOptionKey, `{"enabled":true,"day":2,"end_time":3000}`))

	// 主站上下文只读取全局配置。
	mainConfig := requestTopUpGiftConfig(t, 0)
	require.True(t, mainConfig.Enabled)
	require.Len(t, mainConfig.Rules, 1)
	require.Equal(t, "main", mainConfig.Rules[0].Id)
	require.True(t, mainConfig.Timed.Enabled)
	require.Equal(t, 7, mainConfig.Timed.Day)

	// 服务商上下文只读取 provider_options，不继承主站的开关、规则或倒计时。
	providerConfig := requestTopUpGiftConfig(t, 42)
	require.False(t, providerConfig.Enabled)
	require.Len(t, providerConfig.Rules, 1)
	require.Equal(t, "provider", providerConfig.Rules[0].Id)
	require.True(t, providerConfig.Timed.Enabled)
	require.Equal(t, 2, providerConfig.Timed.Day)

	// 未配置的服务商返回关闭状态与空数组，保持公开接口响应结构稳定。
	missingProviderConfig := requestTopUpGiftConfig(t, 99)
	require.False(t, missingProviderConfig.Enabled)
	require.NotNil(t, missingProviderConfig.Rules)
	require.Empty(t, missingProviderConfig.Rules)
	require.False(t, missingProviderConfig.Timed.Enabled)
}

func requestTopUpGiftConfig(t *testing.T, providerId int) TopUpGiftPublicConfig {
	t.Helper()
	// 直接注入 TenantResolver 的产物，聚焦验证控制器的站点分流与响应内容。
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/topup/gift_config", nil)
	common.SetContextKey(ctx, constant.ContextKeyProviderId, providerId)

	GetTopUpGiftConfig(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                  `json:"success"`
		Data    TopUpGiftPublicConfig `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response.Data
}
