package controller

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/lakala"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupLakalaTopupTestDB(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldQuotaPerUnit := common.QuotaPerUnit
	oldUsingSQLite := common.UsingSQLite
	oldRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.Log{},
		&model.CurrencyStripeConfig{},
		&model.TimezoneCurrencyMap{},
	))
	model.DB = db
	model.LOG_DB = db
	common.QuotaPerUnit = 100000
	common.UsingSQLite = true
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.QuotaPerUnit = oldQuotaPerUnit
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
	})
}

func setupLakalaSettingsForTest(t *testing.T) (keyPEM string, certPEM string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPEMBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	template := &x509.Certificate{
		SerialNumber:          bigIntOneForLakalaTest(),
		Subject:               pkix.Name{CommonName: "lakala.test"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	certPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	oldPayMethods := operation_setting.PayMethods
	oldPrice := operation_setting.Price
	oldServerAddress := system_setting.ServerAddress
	oldCallbackAddress := operation_setting.CustomCallbackAddress
	common.OptionMapRWMutex.Lock()
	oldOptionMap := common.OptionMap
	optionMap := make(map[string]string, len(oldOptionMap)+5)
	for key, value := range oldOptionMap {
		optionMap[key] = value
	}
	common.OptionMap = optionMap
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		operation_setting.PayMethods = oldPayMethods
		operation_setting.Price = oldPrice
		system_setting.ServerAddress = oldServerAddress
		operation_setting.CustomCallbackAddress = oldCallbackAddress
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	operation_setting.PayMethods = []map[string]string{{"name": "微信", "type": model.PaymentProviderLakala}}
	operation_setting.Price = 7.3
	common.OptionMapRWMutex.Lock()
	common.OptionMap["LakalaAppID"] = "OP00000003"
	common.OptionMap["LakalaSerialNo"] = "00dfba8194c41b84cf"
	common.OptionMap["LakalaPrivateKey"] = string(keyPEMBytes)
	common.OptionMap["LakalaPublicCert"] = string(certPEMBytes)
	common.OptionMap["LakalaMerchantNo"] = "822290059430BF9"
	common.OptionMapRWMutex.Unlock()
	system_setting.ServerAddress = "http://127.0.0.1:3000"
	operation_setting.CustomCallbackAddress = "https://callback.example.com"
	return string(keyPEMBytes), string(certPEMBytes)
}

func bigIntOneForLakalaTest() *big.Int {
	return big.NewInt(1)
}

func postLakalaTopupContext(t *testing.T, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	payload, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/pay", bytes.NewReader(payload))
	ctx.Request.Host = "127.0.0.1:3000"
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)
	return ctx, recorder
}

func getLakalaStatusContext(tradeNo string, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/lakala/status?trade_no="+tradeNo, nil)
	ctx.Set("id", userID)
	return ctx, recorder
}

func signLakalaNotifyForTest(t *testing.T, keyPEM string, body string) string {
	t.Helper()
	privateKey, err := lakala.LoadRSAPrivateKey(keyPEM)
	require.NoError(t, err)
	timeStamp := "1780380000"
	nonceStr := "notifyNonce1"
	signString := lakala.BuildNotifySignString(timeStamp, nonceStr, body)
	hash := sha256.Sum256([]byte(signString))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
	require.NoError(t, err)
	return fmt.Sprintf(
		`LKLAPI-SHA256withRSA timestamp="%s",nonce_str="%s",signature="%s"`,
		timeStamp,
		nonceStr,
		base64.StdEncoding.EncodeToString(signature),
	)
}

func TestRequestEpayUsesLakalaPaymentMethod(t *testing.T) {
	setupLakalaTopupTestDB(t)
	keyPEM, _ := setupLakalaSettingsForTest(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "u1", Group: "default"}).Error)

	var upstreamBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/preorder", r.URL.Path)
		require.NotEmpty(t, r.Header.Get("Authorization"))
		require.NoError(t, common.DecodeJson(r.Body, &upstreamBody))
		respBody := `{"resp_data":{"acc_resp_fields":{"code":"alipay://pay/test-code"}}}`
		signResult, err := lakala.Sign("OP00000003", "00dfba8194c41b84cf", keyPEM, respBody)
		require.NoError(t, err)
		w.Header().Set("Lklapi-Appid", signResult.AppID)
		w.Header().Set("Lklapi-Serial", signResult.SerialNo)
		w.Header().Set("Lklapi-Timestamp", signResult.TimeStamp)
		w.Header().Set("Lklapi-Nonce", signResult.NonceStr)
		w.Header().Set("Lklapi-Signature", signResult.Signature)
		_, _ = w.Write([]byte(respBody))
	}))
	defer server.Close()
	oldURL := lakalaPreorderURL
	lakalaPreorderURL = server.URL + "/preorder"
	t.Cleanup(func() { lakalaPreorderURL = oldURL })

	ctx, recorder := postLakalaTopupContext(t, EpayRequest{Amount: 1, PaymentMethod: model.PaymentProviderLakala}, 1)
	RequestEpay(ctx)

	var resp struct {
		Message string            `json:"message"`
		URL     string            `json:"url"`
		Data    map[string]string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "success", resp.Message)
	require.Equal(t, "/payment/lakala/qrcode", resp.URL)
	require.Equal(t, "alipay://pay/test-code", resp.Data["code"])
	require.NotEmpty(t, resp.Data["trade_no"])

	reqData := upstreamBody["req_data"].(map[string]any)
	require.Equal(t, "ALIPAY", reqData["account_type"])
	require.Equal(t, "41", reqData["trans_type"])
	require.Equal(t, "D9261076", reqData["term_no"])
	require.Equal(t, float64(730), reqData["total_amount"])
	require.Equal(t, "https://43110f97.r34.cpolar.top/api/user/lakala/notify", reqData["notify_url"])
	require.Equal(t, "充值", reqData["subject"])
	require.Equal(t, "1", reqData["remark"])

	var topUp model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", resp.Data["trade_no"]).First(&topUp).Error)
	require.Equal(t, model.PaymentProviderLakala, topUp.PaymentMethod)
	require.Equal(t, model.PaymentProviderLakala, topUp.PaymentProvider)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)
	require.InDelta(t, 7.3, topUp.OriginalMoney, 0.001)
}

func TestRequestEpayReturnsLakalaBusinessError(t *testing.T) {
	setupLakalaTopupTestDB(t)
	keyPEM, _ := setupLakalaSettingsForTest(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "u1", Group: "default"}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respBody := `{"code":"BBS11109","msg":"交易限额超过单笔最大限额","resp_time":"20260602140044"}`
		signResult, err := lakala.Sign("OP00000003", "00dfba8194c41b84cf", keyPEM, respBody)
		require.NoError(t, err)
		w.Header().Set("Lklapi-Appid", signResult.AppID)
		w.Header().Set("Lklapi-Serial", signResult.SerialNo)
		w.Header().Set("Lklapi-Timestamp", signResult.TimeStamp)
		w.Header().Set("Lklapi-Nonce", signResult.NonceStr)
		w.Header().Set("Lklapi-Signature", signResult.Signature)
		_, _ = w.Write([]byte(respBody))
	}))
	defer server.Close()
	oldURL := lakalaPreorderURL
	lakalaPreorderURL = server.URL + "/preorder"
	t.Cleanup(func() { lakalaPreorderURL = oldURL })

	ctx, recorder := postLakalaTopupContext(t, EpayRequest{Amount: 1, PaymentMethod: model.PaymentProviderLakala}, 1)
	RequestEpay(ctx)

	var resp struct {
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "error", resp.Message)
	require.Contains(t, resp.Data, "交易限额超过单笔最大限额")
	require.Contains(t, resp.Data, "BBS11109")
}

func TestGetLakalaTopUpStatusReturnsOwnOrder(t *testing.T) {
	setupLakalaTopupTestDB(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, (&model.TopUp{
		UserId:          7,
		TradeNo:         "LKL-status-ok",
		PaymentMethod:   model.PaymentProviderLakala,
		PaymentProvider: model.PaymentProviderLakala,
		BizType:         model.TopUpBizTypePayment,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}).Insert())

	ctx, recorder := getLakalaStatusContext("LKL-status-ok", 7)
	GetLakalaTopUpStatus(ctx)

	var resp struct {
		Message string `json:"message"`
		Data    struct {
			TradeNo string `json:"trade_no"`
			Status  string `json:"status"`
			Paid    bool   `json:"paid"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "success", resp.Message)
	require.Equal(t, "LKL-status-ok", resp.Data.TradeNo)
	require.Equal(t, common.TopUpStatusSuccess, resp.Data.Status)
	require.True(t, resp.Data.Paid)
}

func TestGetLakalaTopUpStatusRejectsOtherUserOrder(t *testing.T) {
	setupLakalaTopupTestDB(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, (&model.TopUp{
		UserId:          8,
		TradeNo:         "LKL-status-other",
		PaymentMethod:   model.PaymentProviderLakala,
		PaymentProvider: model.PaymentProviderLakala,
		BizType:         model.TopUpBizTypePayment,
		Status:          common.TopUpStatusSuccess,
		CreateTime:      time.Now().Unix(),
	}).Insert())

	ctx, recorder := getLakalaStatusContext("LKL-status-other", 9)
	GetLakalaTopUpStatus(ctx)

	var resp struct {
		Message string `json:"message"`
		Data    string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "error", resp.Message)
	require.Equal(t, "订单不存在", resp.Data)
}

func TestLakalaNotifyCompletesMatchingOrder(t *testing.T) {
	setupLakalaTopupTestDB(t)
	keyPEM, _ := setupLakalaSettingsForTest(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.Create(&model.User{Id: 2, Username: "u2", Group: "default"}).Error)
	topUp := &model.TopUp{
		UserId:          2,
		Amount:          1,
		Money:           1,
		OriginalMoney:   7.3,
		TradeNo:         "LKL-notify-ok",
		PaymentMethod:   model.PaymentProviderLakala,
		PaymentProvider: model.PaymentProviderLakala,
		BizType:         model.TopUpBizTypePayment,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())

	body := `{"merchantOrderNo":"LKL-notify-ok","amount":730,"payStatus":"S"}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/lakala/notify", bytes.NewReader([]byte(body)))
	ctx.Request.Header.Set("Authorization", signLakalaNotifyForTest(t, keyPEM, body))

	LakalaNotify(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"code":"0000","message":"success"}`, recorder.Body.String())
	var user model.User
	require.NoError(t, model.DB.First(&user, 2).Error)
	require.Equal(t, int(common.QuotaPerUnit), user.Quota)
}

func TestLakalaNotifyAcceptsStringAmount(t *testing.T) {
	setupLakalaTopupTestDB(t)
	keyPEM, _ := setupLakalaSettingsForTest(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.Create(&model.User{Id: 4, Username: "u4", Group: "default"}).Error)
	require.NoError(t, (&model.TopUp{
		UserId:          4,
		Amount:          1,
		Money:           1,
		OriginalMoney:   7.3,
		TradeNo:         "LKL-notify-string-amount",
		PaymentMethod:   model.PaymentProviderLakala,
		PaymentProvider: model.PaymentProviderLakala,
		BizType:         model.TopUpBizTypePayment,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}).Insert())

	body := `{"out_trade_no":"LKL-notify-string-amount","total_amount":"730","trade_status":"SUCCESS","trade_state":"SUCCESS"}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/lakala/notify", bytes.NewReader([]byte(body)))
	ctx.Request.Header.Set("Authorization", signLakalaNotifyForTest(t, keyPEM, body))

	LakalaNotify(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForLakalaControllerTest(t, "LKL-notify-string-amount"))
}

func TestLakalaNotifyRejectsMismatchedAmount(t *testing.T) {
	setupLakalaTopupTestDB(t)
	keyPEM, _ := setupLakalaSettingsForTest(t)
	gin.SetMode(gin.TestMode)
	require.NoError(t, model.DB.Create(&model.User{Id: 3, Username: "u3", Group: "default"}).Error)
	require.NoError(t, (&model.TopUp{
		UserId:          3,
		Amount:          1,
		Money:           1,
		OriginalMoney:   7.3,
		TradeNo:         "LKL-notify-bad-amount",
		PaymentMethod:   model.PaymentProviderLakala,
		PaymentProvider: model.PaymentProviderLakala,
		BizType:         model.TopUpBizTypePayment,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}).Insert())

	body := `{"req_data":{"out_trade_no":"LKL-notify-bad-amount","total_amount":1,"trade_status":"SUCCESS"}}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/lakala/notify", bytes.NewReader([]byte(body)))
	ctx.Request.Header.Set("Authorization", signLakalaNotifyForTest(t, keyPEM, body))

	LakalaNotify(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, common.TopUpStatusPending, getTopUpStatusForLakalaControllerTest(t, "LKL-notify-bad-amount"))
}

func getTopUpStatusForLakalaControllerTest(t *testing.T, tradeNo string) string {
	t.Helper()
	var topUp model.TopUp
	require.NoError(t, model.DB.Where("trade_no = ?", tradeNo).First(&topUp).Error)
	return topUp.Status
}
