package controller

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const telegramMiniAppTestBotToken = "123456:test-token"

func setupTelegramMiniAppBindTestDB(t *testing.T) {
	t.Helper()

	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldAirdropPlanId := common.AirdropSubscriptionPlanId
	oldTelegramBotToken := common.TelegramBotToken
	oldUsingSQLite := common.UsingSQLite
	oldRedisEnabled := common.RedisEnabled

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TelegramUserBinding{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.Log{},
	))

	model.DB = db
	model.LOG_DB = db
	common.TelegramBotToken = telegramMiniAppTestBotToken
	common.UsingSQLite = true
	common.RedisEnabled = false

	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.AirdropSubscriptionPlanId = oldAirdropPlanId
		common.TelegramBotToken = oldTelegramBotToken
		common.UsingSQLite = oldUsingSQLite
		common.RedisEnabled = oldRedisEnabled
	})
}

func createAirdropPlan(t *testing.T) int {
	t.Helper()

	plan := &model.SubscriptionPlan{
		Id:            901,
		Title:         "Telegram Airdrop",
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationDay,
		DurationValue: 7,
		Enabled:       true,
		TotalAmount:   1000,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	common.AirdropSubscriptionPlanId = plan.Id
	model.InvalidateSubscriptionPlanCache(plan.Id)
	return plan.Id
}

func telegramMiniAppBindContext(t *testing.T, username string, telegramID int64) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	body, err := common.Marshal(gin.H{
		"username": username,
		"initData": signedTelegramMiniAppInitData(t, telegramID),
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/telegram/miniapp/bind", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func signedTelegramMiniAppInitData(t *testing.T, telegramID int64) string {
	t.Helper()

	userJSON, err := common.Marshal(gin.H{
		"id":         telegramID,
		"first_name": "Test",
		"username":   fmt.Sprintf("tg%d", telegramID),
	})
	require.NoError(t, err)

	values := url.Values{}
	values.Set("auth_date", "1700000000")
	values.Set("query_id", "test-query")
	values.Set("user", string(userJSON))
	values.Set("hash", telegramWebAppHash(values, telegramMiniAppTestBotToken))
	return values.Encode()
}

func telegramWebAppHash(values url.Values, token string) string {
	pairs := make([]string, 0, len(values))
	for key, value := range values {
		if key == "hash" {
			continue
		}
		unescaped, _ := url.QueryUnescape(value[0])
		pairs = append(pairs, key+"="+unescaped)
	}
	sort.Strings(pairs)

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))

	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))
	return fmt.Sprintf("%x", mac.Sum(nil))
}

func requireTelegramMiniAppBindSuccess(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	var resp struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, recorder.Body.String())
}

func countAirdropSubscriptions(t *testing.T, userID int) int64 {
	t.Helper()

	var count int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).
		Where("user_id = ? AND source = ?", userID, "airdrop").
		Count(&count).Error)
	return count
}

func TestTelegramMiniAppBindGrantsAirdropOnlyForFirstBinding(t *testing.T) {
	setupTelegramMiniAppBindTestDB(t)
	gin.SetMode(gin.TestMode)

	planID := createAirdropPlan(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "alice", Group: "default"}).Error)

	ctx, recorder := telegramMiniAppBindContext(t, "alice", 10001)
	TelegramMiniAppBind(ctx)

	requireTelegramMiniAppBindSuccess(t, recorder)
	require.Equal(t, int64(1), countAirdropSubscriptions(t, 1))

	var sub model.UserSubscription
	require.NoError(t, model.DB.Where("user_id = ? AND source = ?", 1, "airdrop").First(&sub).Error)
	require.Equal(t, planID, sub.PlanId)

	ctx, recorder = telegramMiniAppBindContext(t, "alice", 10001)
	TelegramMiniAppBind(ctx)

	requireTelegramMiniAppBindSuccess(t, recorder)
	require.Equal(t, int64(1), countAirdropSubscriptions(t, 1))
}

func TestTelegramMiniAppBindDoesNotGrantAirdropWhenUserDoesNotExist(t *testing.T) {
	setupTelegramMiniAppBindTestDB(t)
	gin.SetMode(gin.TestMode)

	createAirdropPlan(t)

	ctx, recorder := telegramMiniAppBindContext(t, "missing-user", 10002)
	TelegramMiniAppBind(ctx)

	var resp struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &resp))
	require.False(t, resp.Success)
	require.Equal(t, int64(0), countAirdropSubscriptions(t, 1))
}
