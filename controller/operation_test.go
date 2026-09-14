package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOperationTestDB(t *testing.T) {
	t.Helper()

	// getOperationProviderID 会经 GetUserCache 走 DB/Redis,测试用内存库并关闭 Redis。
	oldDB := model.DB
	oldRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Provider{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	model.DB = db
	common.RedisEnabled = false

	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedisEnabled
	})
}

func TestGetOperationProviderID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupOperationTestDB(t)

	// 授权用户 user 50 持有主站 operational 权限;user 51 无任何权限。
	granted := model.User{Id: 50, Username: "granted", Role: common.RoleCommonUser, ProviderId: 0}
	granted.Permissions = model.ParsePermissionList(`["operational"]`)
	if err := model.DB.Create(&granted).Error; err != nil {
		t.Fatalf("seed granted user: %v", err)
	}
	// 服务商 7/8 启用,9 禁用:用于校验授权用户切换时的服务商有效性。
	// 注意 Provider.Status 带 gorm default:1 标签,Create 零值会被替换为默认值,
	// 禁用状态需在插入后显式更新。
	if err := model.DB.Create(&model.Provider{Id: 7, Name: "p7", Status: model.ProviderStatusEnabled}).Error; err != nil {
		t.Fatalf("seed provider 7: %v", err)
	}
	if err := model.DB.Create(&model.Provider{Id: 8, Name: "p8", Status: model.ProviderStatusEnabled}).Error; err != nil {
		t.Fatalf("seed provider 8: %v", err)
	}
	if err := model.DB.Create(&model.Provider{Id: 9, Name: "p9", Status: model.ProviderStatusEnabled}).Error; err != nil {
		t.Fatalf("seed provider 9: %v", err)
	}
	if err := model.DB.Model(&model.Provider{}).Where("id = ?", 9).Update("status", model.ProviderStatusDisabled).Error; err != nil {
		t.Fatalf("disable provider 9: %v", err)
	}

	// 保留并最终还原默认的归属服务商解析,避免影响其它测试。
	origLookup := lookupOwnedProviderID
	origEnabledLookup := lookupEnabledProviderID
	defer func() {
		lookupOwnedProviderID = origLookup
		lookupEnabledProviderID = origEnabledLookup
	}()

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
		{name: "provider owner on main site sees own data", role: common.RoleCommonUser, userID: 42, currentProviderID: 0, ownerUserID: 0, ownedProviderID: 7, ownedProviderOK: true, wantProviderID: 7, wantOK: true},
		{name: "granted user defaults to main site", role: common.RoleCommonUser, userID: 50, wantProviderID: 0, wantOK: true},
		{name: "granted user switches to enabled provider", role: common.RoleCommonUser, userID: 50, query: "?provider_id=7", wantProviderID: 7, wantOK: true},
		{name: "granted user rejects disabled provider", role: common.RoleCommonUser, userID: 50, query: "?provider_id=9", wantOK: false},
		{name: "granted user rejects unknown provider", role: common.RoleCommonUser, userID: 50, query: "?provider_id=999", wantOK: false},
		{name: "granted user rejects invalid provider param", role: common.RoleCommonUser, userID: 50, query: "?provider_id=invalid", wantOK: false},
		{name: "granted user main site scope can be explicit", role: common.RoleCommonUser, userID: 50, query: "?provider_id=0", wantProviderID: 0, wantOK: true},
		{name: "ungranted main site user is rejected", role: common.RoleCommonUser, userID: 51, wantOK: false},
		{name: "non-owner on main site is rejected", role: common.RoleCommonUser, userID: 44, currentProviderID: 0, ownerUserID: 0, wantOK: false},
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
			// enabled 校验走真实内存库(上面 seed 的服务商 7/8/9)。
			lookupEnabledProviderID = origEnabledLookup

			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest("GET", "/api/operation/dashboard"+test.query, nil)
			context.Set("role", test.role)
			context.Set("id", test.userID)
			common.SetContextKey(context, constant.ContextKeyProviderId, test.currentProviderID)
			common.SetContextKey(context, constant.ContextKeyProviderOwnerUserId, test.ownerUserID)

			providerID, ok := getOperationProviderID(context)
			if ok != test.wantOK {
				t.Fatalf("getOperationProviderID() ok = %v, want %v (resp=%s)", ok, test.wantOK, recorder.Body.String())
			}
			if providerID != test.wantProviderID {
				t.Fatalf("getOperationProviderID() providerID = %d, want %d", providerID, test.wantProviderID)
			}
		})
	}
}
