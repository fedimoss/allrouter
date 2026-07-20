package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

func TestGetOperationProviderID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		role              int
		userID            int
		query             string
		currentProviderID int
		ownerUserID       int
		wantProviderID    int
		wantOK            bool
	}{
		{name: "admin defaults to main site", role: common.RoleAdminUser, wantProviderID: 0, wantOK: true},
		{name: "admin selects provider", role: common.RoleAdminUser, query: "?provider_id=12", wantProviderID: 12, wantOK: true},
		{name: "admin rejects invalid provider", role: common.RoleAdminUser, query: "?provider_id=invalid", wantOK: false},
		{name: "provider owner is bound to current site", role: common.RoleCommonUser, userID: 42, query: "?provider_id=99", currentProviderID: 7, ownerUserID: 42, wantProviderID: 7, wantOK: true},
		{name: "provider member is rejected", role: common.RoleCommonUser, userID: 43, currentProviderID: 7, ownerUserID: 42, wantOK: false},
		{name: "provider owner on main site is rejected", role: common.RoleCommonUser, userID: 42, currentProviderID: 0, ownerUserID: 42, wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
