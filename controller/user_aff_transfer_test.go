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

func setupAffTransferTestDB(t *testing.T) {
	t.Helper()

	// 测试会替换进程级数据库与 Redis 开关，结束后必须恢复，避免污染其他用例。
	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedisEnabled
	})
}

func postAffTransferContext(userID int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	// 全额划转金额由后端决定，因此请求体应为空。
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/aff_transfer", nil)
	ctx.Set("id", userID)
	return ctx, recorder
}

func TestTransferAffQuotaTransfersEntireBalance(t *testing.T) {
	setupAffTransferTestDB(t)
	gin.SetMode(gin.TestMode)

	user := model.User{
		Id:              1,
		Username:        "inviter",
		Quota:           100,
		RewardQuota:     40,
		AffQuota:        1, // 使用低于旧版最低门槛的额度，验证任意正数均可全额划转。
		AffHistoryQuota: 500,
	}
	require.NoError(t, model.DB.Create(&user).Error)

	ctx, recorder := postAffTransferContext(user.Id)
	TransferAffQuota(ctx)

	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	// 待划转额度清零，总余额与奖励余额同步增加，累计邀请收益保持不变。
	require.Zero(t, updated.AffQuota)
	require.Equal(t, 101, updated.Quota)
	require.Equal(t, 41, updated.RewardQuota)
	require.Equal(t, 500, updated.AffHistoryQuota)
}

func TestTransferAffQuotaRejectsEmptyBalance(t *testing.T) {
	setupAffTransferTestDB(t)
	gin.SetMode(gin.TestMode)

	user := model.User{Id: 2, Username: "no-reward", Quota: 100}
	require.NoError(t, model.DB.Create(&user).Error)

	ctx, recorder := postAffTransferContext(user.Id)
	TransferAffQuota(ctx)

	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)

	// 无可用邀请额度时，不得修改任何余额字段。
	var updated model.User
	require.NoError(t, model.DB.First(&updated, user.Id).Error)
	require.Zero(t, updated.AffQuota)
	require.Equal(t, 100, updated.Quota)
	require.Zero(t, updated.RewardQuota)
}
