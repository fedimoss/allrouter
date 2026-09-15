package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

// TestModelPriceHelperRatioPreConsumeUsesTokenUnits verifies the unit contract
// for ratio-priced models.  A model ratio is quota units per token (ratio=1 is
// one quota per token), while QuotaPerUnit is only used by fixed-dollar prices.
// Accidentally multiplying this branch by QuotaPerUnit would reject otherwise
// affordable subscription requests by several orders of magnitude.
func TestModelPriceHelperRatioPreConsumeUsesTokenUnits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const modelName = "__price_helper_ratio_unit_regression__"
	originalRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalRatios))
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"__price_helper_ratio_unit_regression__":0.055}`))

	newContext := func() *gin.Context {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx.Set("group", "default")
		return ctx
	}
	newInfo := func() *relaycommon.RelayInfo {
		return &relaycommon.RelayInfo{
			OriginModelName: modelName,
			UserGroup:       "default",
			UsingGroup:      "default",
		}
	}

	// The minimum pre-consume floor is 500 tokens.  600 tokens at ratio .055
	// therefore costs 33 quota units, not 16,500 (33 * QuotaPerUnit).
	priceData, err := ModelPriceHelper(newContext(), newInfo(), 100, &types.TokenCountMeta{MaxTokens: 100})
	require.NoError(t, err)
	require.Equal(t, 33, priceData.QuotaToPreConsume)

	// Once the prompt exceeds the floor, max_tokens is added to the prompt
	// estimate before applying the ratio.
	priceData, err = ModelPriceHelper(newContext(), newInfo(), 1000, &types.TokenCountMeta{MaxTokens: 200})
	require.NoError(t, err)
	require.Equal(t, 66, priceData.QuotaToPreConsume)
}

// TestModelPriceHelperRatioPreConsumeClampsTokenEstimate guards the checked
// conversion introduced around max_tokens addition.  A maximum-size uint
// accepted by DTO validation must not wrap an int and turn the pre-consume
// amount negative (or zero).
func TestModelPriceHelperRatioPreConsumeClampsTokenEstimate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const modelName = "__price_helper_ratio_overflow_regression__"
	originalRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(originalRatios))
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"__price_helper_ratio_overflow_regression__":0.055}`))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("group", "default")
	info := &relaycommon.RelayInfo{
		OriginModelName: modelName,
		UserGroup:       "default",
		UsingGroup:      "default",
	}

	priceData, err := ModelPriceHelper(ctx, info, 100, &types.TokenCountMeta{MaxTokens: common.MaxQuota})
	require.NoError(t, err)
	want := common.QuotaFromFloat(float64(common.MaxQuota) * 0.055)
	require.Equal(t, want, priceData.QuotaToPreConsume)
	require.Greater(t, priceData.QuotaToPreConsume, 0)
}
