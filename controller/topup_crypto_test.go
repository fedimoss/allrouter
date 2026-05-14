package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCryptoTopupTestDB(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.CryptoTransaction{},
		&model.TimezoneCurrencyMap{},
		&model.CryptoChainConfig{},
	))
	model.DB = db
	t.Cleanup(func() {
		model.DB = oldDB
	})
}

func postCryptoTopupContext(t *testing.T, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/crypto/pay", bytes.NewReader(payload))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)
	return ctx, recorder
}

func TestCalcCryptoAmounts(t *testing.T) {
	usdAmount, usdtAmount, symbol, err := calcCryptoAmounts(5, "USD")
	require.NoError(t, err)
	require.Equal(t, "5", usdAmount.String())
	require.Equal(t, "5", usdtAmount.String())
	require.Equal(t, "$", symbol)

	usdAmount, usdtAmount, symbol, err = calcCryptoAmounts(4, "CNY")
	require.NoError(t, err)
	require.Equal(t, "0.5884", usdAmount.String())
	require.Equal(t, "0.5884", usdtAmount.String())
	require.Equal(t, "¥", symbol)
}

func TestResolveCryptoUserCurrencyUsesExactTimezoneOrUSD(t *testing.T) {
	setupCryptoTopupTestDB(t)
	require.NoError(t, model.DB.Create(&model.TimezoneCurrencyMap{Timezone: "Asia/Shanghai", Currency: "CNY"}).Error)

	require.Equal(t, "CNY", resolveCryptoUserCurrency(&model.User{Timezone: "Asia/Shanghai"}))
	require.Equal(t, "USD", resolveCryptoUserCurrency(&model.User{Timezone: "Asia/Tokyo"}))
	require.Equal(t, "USD", resolveCryptoUserCurrency(&model.User{}))
}

func TestRequestCryptoPayRequiresReceiverAddress(t *testing.T) {
	setupCryptoTopupTestDB(t)
	gin.SetMode(gin.TestMode)
	// 插入一条收款地址为空的配置，模拟未配置场景
	require.NoError(t, model.DB.Create(&model.CryptoChainConfig{
		Network:          "Sepolia",
		ChainID:          11155111,
		TokenSymbol:      "USDT",
		TokenDecimals:    6,
		TokenContract:    "0xcE5DD515c545bEe30EF9a0E42a5da3211A79D983",
		ReceiverAddress:  "", // 空地址
		RPCURL:           "https://example.com",
		MinConfirmations: 2,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "u1", Timezone: "America/New_York"}).Error)

	ctx, recorder := postCryptoTopupContext(t, CryptoPayRequest{Amount: 5, Network: "Sepolia", PaymentMethod: "crypto"}, 1)
	RequestCryptoPay(ctx)

	var resp map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, false, resp["success"])
	require.Contains(t, resp["message"], "收款地址未配置")
}

func TestRequestCryptoPayCreatesPendingOrder(t *testing.T) {
	setupCryptoTopupTestDB(t)
	gin.SetMode(gin.TestMode)
	// 插入一条有效配置
	require.NoError(t, model.DB.Create(&model.CryptoChainConfig{
		Network:          "Sepolia",
		ChainID:          11155111,
		TokenSymbol:      "USDT",
		TokenDecimals:    6,
		TokenContract:    "0xcE5DD515c545bEe30EF9a0E42a5da3211A79D983",
		ReceiverAddress:  "0x1111111111111111111111111111111111111111",
		RPCURL:           "https://example.com",
		MinConfirmations: 2,
	}).Error)
	require.NoError(t, model.DB.Create(&model.TimezoneCurrencyMap{Timezone: "Asia/Shanghai", Currency: "CNY"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "u1", Timezone: "Asia/Shanghai"}).Error)

	ctx, recorder := postCryptoTopupContext(t, CryptoPayRequest{Amount: 4, Network: "Sepolia"}, 1)
	RequestCryptoPay(ctx)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			TradeNo         string `json:"trade_no"`
			PayAmount       string `json:"pay_amount"`
			DisplayCurrency string `json:"display_currency"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Equal(t, "0.588400", resp.Data.PayAmount)
	require.Equal(t, "CNY", resp.Data.DisplayCurrency)
	require.NotEmpty(t, resp.Data.TradeNo)

	var topUp model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", resp.Data.TradeNo).First(&topUp).Error)
	require.Equal(t, PaymentMethodCrypto, topUp.PaymentMethod)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)
	require.Equal(t, int64(4), topUp.Amount)
	require.InDelta(t, 0.5884, topUp.Money, 0.000001)

	var cryptoTx model.CryptoTransaction
	require.NoError(t, model.DB.Where("trade_no = ?", resp.Data.TradeNo).First(&cryptoTx).Error)
	require.Equal(t, topUp.Id, cryptoTx.TopUpId)
	require.Equal(t, "0.588400", cryptoTx.UsdtAmount)
	require.Equal(t, model.CryptoTransactionStatusPending, cryptoTx.Status)
}

func TestParseCryptoTransferLog(t *testing.T) {
	cfg := cryptoChainConfig{
		Network:         "BSC",
		ChainID:         56,
		TokenSymbol:     "USDT",
		TokenDecimals:   18,
		TokenContract:   "0x55d398326f99059fF775485246999027B3197955",
		ReceiverAddress: "0x1111111111111111111111111111111111111111",
	}

	logItem := cryptoLog{
		Address: cfg.TokenContract,
		Topics: []string{
			cryptoTransferTopic,
			"0x0000000000000000000000002222222222222222222222222222222222222222",
			"0x0000000000000000000000001111111111111111111111111111111111111111",
		},
		Data: "0x0000000000000000000000000000000000000000000000004563918244f40000",
	}

	from, to, value, ok := parseCryptoTransferLog(&cfg, logItem)
	require.True(t, ok)
	require.Equal(t, "0x2222222222222222222222222222222222222222", from)
	require.Equal(t, "0x1111111111111111111111111111111111111111", to)
	require.Equal(t, "5000000000000000000", value.String())
}
