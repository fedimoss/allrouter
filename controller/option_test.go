package controller

import (
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

func TestGetOptionsReturnsDatabaseValueOverOptionMapCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldDB := model.DB
	oldOptionMap := common.OptionMap
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}))

	model.DB = db
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"TopUpGiftRules": `[{"id":"old","threshold":1,"bonus":1}]`,
		"ApiSecret":      "hidden",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		model.DB = oldDB
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	require.NoError(t, model.DB.Create(&model.Option{
		Key:   "TopUpGiftRules",
		Value: `[{"id":"new-1","threshold":10,"bonus":1},{"id":"new-2","threshold":20,"bonus":3}]`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Option{
		Key:   "TelegramWebhookSecret",
		Value: "telegram-webhook-secret",
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)

	GetOptions(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool            `json:"success"`
		Data    []*model.Option `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)

	values := make(map[string]string, len(response.Data))
	for _, option := range response.Data {
		values[option.Key] = option.Value
	}
	require.Equal(t, `[{"id":"new-1","threshold":10,"bonus":1},{"id":"new-2","threshold":20,"bonus":3}]`, values["TopUpGiftRules"])
	require.NotContains(t, values, "ApiSecret")
	require.NotContains(t, values, "TelegramWebhookSecret")
	require.Equal(t, "true", values["TelegramWebhookSecretConfigured"])
}
