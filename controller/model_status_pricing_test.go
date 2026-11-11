package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	testVisibleModelName = "zz-model-status-visible"
	testHiddenModelName  = "zz-model-status-hidden"
	testProviderID       = 8101
	testProviderOwnerID  = 8102
)

type pricingResponse struct {
	Success           bool            `json:"success"`
	Data              []model.Pricing `json:"data"`
	Vendors           []struct{}      `json:"vendors"`
	SupportedEndpoint map[string]struct {
		Path   string `json:"path"`
		Method string `json:"method"`
	} `json:"supported_endpoint"`
	GroupRatio     map[string]float64 `json:"group_ratio"`
	UsableGroup    map[string]string  `json:"usable_group"`
	PricingVersion string             `json:"pricing_version"`
}

type baseModelsResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Models []string `json:"models"`
		Items  []struct {
			ModelName   string `json:"model_name"`
			BillingMode string `json:"billing_mode"`
			BillingExpr string `json:"billing_expr"`
		} `json:"items"`
	} `json:"data"`
}

// setupModelStatusPricingTest 构造一个可见模型和一个仅模型广场隐藏的模型。
func setupModelStatusPricingTest(t *testing.T) *gorm.DB {
	t.Helper()

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Provider{},
		&model.ProviderConfig{},
		&model.ProviderOption{},
		&model.ProviderModelPricing{},
	))

	originalSelfUseMode := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = false
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = originalSelfUseMode
	})

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		if key == "billing_setting.billing_mode" || key == "billing_setting.billing_expr" {
			saved[key] = value
		}
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
		model.InvalidatePricingCache()
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"` + testHiddenModelName + `":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"` + testHiddenModelName + `":"tier(\"base\", p * 1 + c * 2)"}`,
	}))
	model.InvalidatePricingCache()

	require.NoError(t, db.Create(&model.User{
		Id:       testProviderOwnerID,
		Username: "provider-owner",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		AffCode:  "status-owner-aff",
	}).Error)
	require.NoError(t, db.Create(&model.User{
		Id:       8103,
		Username: "pricing-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
		AffCode:  "status-pricing-aff",
	}).Error)
	require.NoError(t, db.Create(&model.Provider{
		Id:          testProviderID,
		OwnerUserId: testProviderOwnerID,
		Name:        "status-provider",
		Status:      model.ProviderStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "status-channel",
		Group:  "default",
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: testVisibleModelName, ChannelId: 1, Enabled: true},
		{Group: "default", Model: testHiddenModelName, ChannelId: 1, Enabled: true},
	}).Error)
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: testVisibleModelName, Status: model.ModelStatusVisible, NameRule: model.NameRuleExact},
		{ModelName: testHiddenModelName, Status: 0, NameRule: model.NameRuleExact},
	}).Error)
	// GORM 会对零值应用 default:1，必须显式回写 0 才能构造隐藏状态。
	require.NoError(t, db.Model(&model.Model{}).
		Where("model_name = ?", testHiddenModelName).
		Update("status", 0).Error)
	model.InvalidatePricingCache()

	return db
}

func modelStatusPricingContext(t *testing.T, providerID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 8103)
	common.SetContextKey(ctx, constant.ContextKeyProviderId, providerID)
	return ctx, recorder
}

func decodePricingResponse(t *testing.T, recorder *httptest.ResponseRecorder) pricingResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var response pricingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

func pricingResponseByModelName(response pricingResponse) map[string]model.Pricing {
	result := make(map[string]model.Pricing, len(response.Data))
	for _, item := range response.Data {
		result[item.ModelName] = item
	}
	return result
}

// 隐藏模型不应从注册表消失，否则分站候选、成本价和同步都会失去依据。
func TestModelStatusHiddenModelStaysInRegistry(t *testing.T) {
	setupModelStatusPricingTest(t)

	pricingByName := pricingByModelName(model.GetPricing())
	hidden, ok := pricingByName[testHiddenModelName]
	require.True(t, ok)
	require.True(t, hidden.MarketplaceHidden)
	require.Equal(t, []string{"default"}, hidden.EnableGroup)
	require.NotEmpty(t, hidden.SupportedEndpointTypes)
	require.Equal(t, "tiered_expr", hidden.BillingMode)
	require.NotEmpty(t, hidden.BillingExpr)
	require.Equal(t, []int{0}, model.GetModelQuotaTypes(testHiddenModelName))

	visible, ok := pricingByName[testVisibleModelName]
	require.True(t, ok)
	require.False(t, visible.MarketplaceHidden)

	eligibleNames := getMarketplaceVisibleModelNamesForUserGroup("default")
	require.Contains(t, eligibleNames, testHiddenModelName)
	require.Contains(t, eligibleNames, testVisibleModelName)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 8103)
	ListModels(ctx, constant.ChannelTypeOpenAI)
	require.Contains(t, decodeListModelsResponse(t, recorder), testHiddenModelName)
}

// 主站只在 /api/pricing 的 data 中隐藏模型，版本标记不能污染共享缓存。
func TestModelStatusHiddenModelExcludedFromMainPricingData(t *testing.T) {
	setupModelStatusPricingTest(t)

	ctx, recorder := modelStatusPricingContext(t, 0)
	GetPricing(ctx)
	response := decodePricingResponse(t, recorder)
	byName := pricingResponseByModelName(response)
	require.Contains(t, byName, testVisibleModelName)
	require.NotContains(t, byName, testHiddenModelName)
	require.Equal(t, "b2c5fbb11278d477f8141196fd56208a", response.PricingVersion)
	require.NotEmpty(t, response.GroupRatio)
	require.NotEmpty(t, response.UsableGroup)
	require.NotEmpty(t, response.SupportedEndpoint)
	require.Empty(t, pricingByModelName(model.GetPricing())[testVisibleModelName].PricingVersion)

	var versionCount int
	for _, item := range response.Data {
		require.False(t, item.MarketplaceHidden)
		if item.PricingVersion != "" {
			versionCount++
		}
	}
	require.Equal(t, 1, versionCount)
	for _, item := range response.Data {
		if item.ModelName == testVisibleModelName {
			require.Equal(t, "b2c5fbb11278d477f8141196fd56208a31cb25dfdb04e0dc866e3fc4b8f0ae21", item.PricingVersion)
		}
	}
}

// 分站同样只在模型广场隐藏，已启用的分站映射不应把隐藏 base 带进响应。
func TestModelStatusHiddenModelExcludedFromProviderPricingData(t *testing.T) {
	db := setupModelStatusPricingTest(t)
	require.NoError(t, db.Create(&model.ProviderModelPricing{
		ProviderId:      testProviderID,
		PublicModelName: testHiddenModelName + "-public",
		BaseModelName:   testHiddenModelName,
		Enabled:         true,
		PricingType:     model.ProviderPricingTypeRatio,
		Ratio:           1.5,
	}).Error)

	ctx, recorder := modelStatusPricingContext(t, testProviderID)
	GetPricing(ctx)
	response := decodePricingResponse(t, recorder)
	byName := pricingResponseByModelName(response)
	require.NotContains(t, byName, testHiddenModelName)
	require.NotContains(t, byName, testHiddenModelName+"-public")
	require.NotContains(t, pricingByModelName(model.GetPricing()), testHiddenModelName+"-public")
	require.NotEmpty(t, response.SupportedEndpoint)
}

// 手动新增依赖 base_models 候选和启用校验使用完整注册表。
func TestModelStatusHiddenModelCanBeManuallyAddedToProvider(t *testing.T) {
	db := setupModelStatusPricingTest(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/provider/admin/base_models?with_price=true", nil)
	listProviderBaseModels(ctx, testProviderID)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response baseModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Contains(t, response.Data.Models, testHiddenModelName)
	require.Contains(t, response.Data.Models, testVisibleModelName)

	var hiddenPriceFound bool
	for _, item := range response.Data.Items {
		if item.ModelName == testHiddenModelName {
			hiddenPriceFound = true
			require.Equal(t, "tiered_expr", item.BillingMode)
			require.NotEmpty(t, item.BillingExpr)
		}
	}
	require.True(t, hiddenPriceFound)

	payload, err := common.Marshal(model.ProviderModelPricing{
		ProviderId:      testProviderID,
		PublicModelName: testHiddenModelName + "-public",
		BaseModelName:   testHiddenModelName,
		Enabled:         true,
		PricingType:     model.ProviderPricingTypeRatio,
		Ratio:           1.5,
	})
	require.NoError(t, err)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/provider/admin/model_pricing", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	upsertProviderModelPricing(ctx, testProviderID, true)
	require.Equal(t, http.StatusOK, recorder.Code)
	var apiResponse struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &apiResponse))
	require.True(t, apiResponse.Success, apiResponse.Message)

	rule, err := model.GetProviderModelPricing(testProviderID, testHiddenModelName+"-public")
	require.NoError(t, err)
	require.True(t, rule.Enabled)
	require.Equal(t, testHiddenModelName, rule.BaseModelName)
	require.NoError(t, db.Where("provider_id = ?", testProviderID).Delete(&model.ProviderModelPricing{}).Error)
}

// 自动同步继续把隐藏模型视为已注册模型：不软禁用，并照常补建和恢复。
func TestModelStatusHiddenModelKeepsProviderSyncBehavior(t *testing.T) {
	setupModelStatusPricingTest(t)
	mainModels := getMarketplaceVisibleModelNamesForUserGroup("default")

	plan := planProviderModelPricingSync(mainModels, []model.ProviderModelPricing{
		pricingRow(1, testHiddenModelName, testHiddenModelName, true, false),
		pricingRow(2, testVisibleModelName, testVisibleModelName, false, true),
		pricingRow(3, "zz-manual-off", "zz-manual-off", false, false),
	})
	require.Empty(t, plan.Disable)
	require.Equal(t, []string{testVisibleModelName}, updatedBaseNames(plan, true))
	freshModelName := "zz-model-status-fresh"
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     freshModelName,
		ChannelId: 1,
		Enabled:   true,
	}).Error)
	freshPlan := planProviderModelPricingSync(
		append(getMarketplaceVisibleModelNamesForUserGroup("default"), freshModelName),
		[]model.ProviderModelPricing{},
	)
	require.Contains(t, createdBaseNames(freshPlan), freshModelName)
	for _, created := range plan.Creates {
		require.True(t, created.Enabled)
	}
	for _, created := range freshPlan.Creates {
		require.True(t, created.Enabled)
	}

	manualPlan := planProviderModelPricingSync([]string{testHiddenModelName}, []model.ProviderModelPricing{
		pricingRow(3, "zz-manual-off", "zz-manual-off", false, false),
	})
	require.Empty(t, manualPlan.Reenable)
}
