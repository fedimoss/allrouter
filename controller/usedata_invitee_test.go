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

func setupInviteeDashboardTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.InviteRecord{}, &model.QuotaData{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
	})
}

func TestGetSelfInviteeDashboardDataRejectsUnrelatedUser(t *testing.T) {
	setupInviteeDashboardTestDB(t)
	gin.SetMode(gin.TestMode)

	require.NoError(t, model.DB.Create(&model.User{Id: 2, Username: "unrelated"}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/data/self/invitee?user_id=2&start_timestamp=0&end_timestamp=3600",
		nil,
	)
	context.Set("id", 1)

	GetSelfInviteeDashboardData(context)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestGetSelfInviteeDashboardDataReturnsInvitedUserStats(t *testing.T) {
	setupInviteeDashboardTestDB(t)
	gin.SetMode(gin.TestMode)

	invitee := model.User{
		Id:             2,
		Username:       "invitee",
		Quota:          900,
		UsedQuota:      100,
		TotalTokenUsed: 55,
	}
	require.NoError(t, model.DB.Create(&invitee).Error)
	require.NoError(t, model.DB.Create(&model.InviteRecord{
		InviterId: 1,
		InviteeId: 2,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QuotaData{
		UserID:    2,
		Username:  "invitee",
		ModelName: "test-model",
		CreatedAt: 1800,
		TokenUsed: 30,
		Count:     2,
		Quota:     40,
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/data/self/invitee?user_id=2&start_timestamp=0&end_timestamp=3600",
		nil,
	)
	context.Set("id", 1)

	GetSelfInviteeDashboardData(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"username":"invitee"`)
	require.Contains(t, recorder.Body.String(), `"model_name":"test-model"`)
}

func TestGetSelfAffRecordsFiltersAndPaginatesInvitees(t *testing.T) {
	setupInviteeDashboardTestDB(t)

	require.NoError(t, model.DB.Create(&model.User{Id: 2, Username: "alice-one", AffCode: "aff-2"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 3, Username: "alice-two", AffCode: "aff-3"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 4, Username: "bob", AffCode: "aff-4"}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 5, Username: "alice-other-inviter", AffCode: "aff-5"}).Error)
	require.NoError(t, model.DB.Create([]model.InviteRecord{
		{InviterId: 1, InviteeId: 2, RegisterTime: 100},
		{InviterId: 1, InviteeId: 3, RegisterTime: 200},
		{InviterId: 1, InviteeId: 4, RegisterTime: 300},
		{InviterId: 9, InviteeId: 5, RegisterTime: 400},
	}).Error)

	records, total, err := model.GetSelfAffRecords(
		1,
		"alice",
		&common.PageInfo{Page: 1, PageSize: 1},
	)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, records, 1)
	require.Equal(t, "alice-two", records[0].InviteeName)
}
