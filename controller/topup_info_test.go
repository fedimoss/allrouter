package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTopupInfoTestDB(t *testing.T) {
	t.Helper()

	oldDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.CurrencyStripeConfig{},
		&model.TimezoneCurrencyMap{},
	))

	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
	})
}

func getTopupInfoContext(t *testing.T, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/topup/info", nil)
	ctx.Set("id", userID)
	return ctx, recorder
}

func postStripeAmountContext(t *testing.T, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	payload, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/stripe/amount", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func TestGetTopUpInfoDoesNotExposeStripeWhenPayMethodsExcludeStripe(t *testing.T) {
	setupTopupInfoTestDB(t)
	gin.SetMode(gin.TestMode)

	oldPayMethods := operation_setting.PayMethods
	oldStripeApiSecret := setting.StripeApiSecret
	oldStripeWebhookSecret := setting.StripeWebhookSecret
	oldStripePriceId := setting.StripePriceId
	t.Cleanup(func() {
		operation_setting.PayMethods = oldPayMethods
		setting.StripeApiSecret = oldStripeApiSecret
		setting.StripeWebhookSecret = oldStripeWebhookSecret
		setting.StripePriceId = oldStripePriceId
	})

	operation_setting.PayMethods = []map[string]string{{
		"color": "rgba(var(--semi-green-5), 1)",
		"name":  "微信",
		"type":  "wxpay",
	}}
	setting.StripeApiSecret = "sk_test_123"
	setting.StripeWebhookSecret = "whsec_test_123"
	setting.StripePriceId = "price_usd"

	require.NoError(t, model.DB.Create(&model.User{
		Id:       1,
		Username: "u1",
		Timezone: "America/New_York",
	}).Error)
	require.NoError(t, model.DB.Create(&model.CurrencyStripeConfig{
		Currency:      "USD",
		StripePriceID: "price_usd",
		UnitPrice:     1,
		Symbol:        "$",
	}).Error)

	ctx, recorder := getTopupInfoContext(t, 1)
	GetTopUpInfo(ctx)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			EnableStripeTopup bool                `json:"enable_stripe_topup"`
			PayMethods        []map[string]string `json:"pay_methods"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.False(t, resp.Data.EnableStripeTopup)
	require.Len(t, resp.Data.PayMethods, 1)
	require.Equal(t, "wxpay", resp.Data.PayMethods[0]["type"])
	for _, method := range resp.Data.PayMethods {
		require.NotEqual(t, PaymentMethodStripe, method["type"])
	}
}

func TestRequestStripeAmountRequiresConfiguredPayMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.PayMethods = oldPayMethods
	})

	operation_setting.PayMethods = []map[string]string{{
		"name": "微信",
		"type": "wxpay",
	}}

	ctx, recorder := postStripeAmountContext(t, StripePayRequest{
		Amount:        10,
		PaymentMethod: PaymentMethodStripe,
	})
	RequestStripeAmount(ctx)

	var resp struct {
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "error", resp.Message)
	require.Equal(t, "管理员未开启Stripe充值", resp.Data)
}
