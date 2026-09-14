package controller

import (
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

func TestGetOperationProviderID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	// GetUserCache should use the SQLite fallback; no Redis client is initialized
	// in this unit-test process.
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedisEnabled
	})
	for _, user := range []model.User{
		{Id: 42, ProviderId: 0, Username: "operation-owner", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "operation-owner-aff"},
		{Id: 43, ProviderId: 7, Username: "operation-member", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "operation-member-aff"},
		{Id: 44, ProviderId: 0, Username: "operation-main", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "operation-main-aff"},
		{Id: 45, ProviderId: 7, Username: "operation-member-granted", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "operation-member-granted-aff", Permissions: model.PermissionList{"providerOperational"}},
		{Id: 46, ProviderId: 0, Username: "operation-main-granted", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "operation-main-granted-aff", Permissions: model.PermissionList{"operational"}},
	} {
		require.NoError(t, db.Create(&user).Error)
	}

	// 保留并最终还原默认的归属服务商解析,避免影响其它测试。
	origLookup := lookupOwnedProviderID
	defer func() { lookupOwnedProviderID = origLookup }()

	tests := []struct {
		name              string
		role              int
		userID            int
		query             string
		currentProviderID int
		ownerUserID       int
		ownedProviderID   int // lookupOwnedProviderID 对该 userID 的模拟返回值
		ownedProviderOK   bool
		wantProviderID    int
		wantOK            bool
	}{
		{name: "admin defaults to main site", role: common.RoleAdminUser, wantProviderID: 0, wantOK: true},
		{name: "admin selects provider", role: common.RoleAdminUser, query: "?provider_id=12", wantProviderID: 12, wantOK: true},
		{name: "admin rejects invalid provider", role: common.RoleAdminUser, query: "?provider_id=invalid", wantOK: false},
		{name: "provider owner is bound to current site", role: common.RoleCommonUser, userID: 42, query: "?provider_id=99", currentProviderID: 7, ownerUserID: 42, wantProviderID: 7, wantOK: true},
		{name: "provider member is rejected", role: common.RoleCommonUser, userID: 43, currentProviderID: 7, ownerUserID: 42, wantOK: false},
		{name: "provider member with providerOperational is allowed", role: common.RoleCommonUser, userID: 45, currentProviderID: 7, ownerUserID: 42, wantProviderID: 7, wantOK: true},
		{name: "owner of another provider cannot use current provider tenant", role: common.RoleCommonUser, userID: 42, currentProviderID: 99, ownerUserID: 123, ownedProviderID: 7, ownedProviderOK: true, wantOK: false},
		{name: "provider owner on main site sees own data", role: common.RoleCommonUser, userID: 42, currentProviderID: 0, ownerUserID: 0, ownedProviderID: 7, ownedProviderOK: true, wantProviderID: 7, wantOK: true},
		{name: "non-owner on main site is rejected", role: common.RoleCommonUser, userID: 44, currentProviderID: 0, ownerUserID: 0, wantOK: false},
		{name: "main-site operational member sees main-site data", role: common.RoleCommonUser, userID: 46, currentProviderID: 0, ownerUserID: 0, wantProviderID: 0, wantOK: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 按用例模拟"用户名下服务商"的解析结果。
			lookupOwnedProviderID = func(userId int) (int, bool) {
				if userId == test.userID {
					return test.ownedProviderID, test.ownedProviderOK
				}
				return 0, false
			}

			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest("GET", "/api/operation/dashboard"+test.query, nil)
			context.Set("role", test.role)
			context.Set("id", test.userID)
			common.SetContextKey(context, constant.ContextKeyProviderId, test.currentProviderID)
			common.SetContextKey(context, constant.ContextKeyProviderOwnerUserId, test.ownerUserID)

			providerID, ok := getOperationProviderID(context)
			if ok != test.wantOK {
				t.Fatalf("getOperationProviderID() ok = %v, want %v", ok, test.wantOK)
			}
			if providerID != test.wantProviderID {
				t.Fatalf("getOperationProviderID() providerID = %d, want %d", providerID, test.wantProviderID)
			}
		})
	}
}
