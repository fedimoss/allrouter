package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetNoticeProviderVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldOptionMap := common.OptionMap
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	testCases := []struct {
		name            string
		providerId      int
		showToProviders string
		expectedNotice  string
	}{
		{
			name:            "main site always sees notice",
			showToProviders: "false",
			expectedNotice:  "maintenance notice",
		},
		{
			name:            "provider site sees notice when enabled",
			providerId:      42,
			showToProviders: "true",
			expectedNotice:  "maintenance notice",
		},
		{
			name:            "provider site does not see notice when disabled",
			providerId:      42,
			showToProviders: "false",
			expectedNotice:  "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			common.OptionMapRWMutex.Lock()
			common.OptionMap = map[string]string{
				"Notice":                             "maintenance notice",
				model.NoticeShowToProvidersOptionKey: testCase.showToProviders,
			}
			common.OptionMapRWMutex.Unlock()

			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/notice", nil)
			common.SetContextKey(ctx, constant.ContextKeyProviderId, testCase.providerId)

			GetNotice(ctx)

			var response struct {
				Success bool   `json:"success"`
				Data    string `json:"data"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success)
			require.Equal(t, testCase.expectedNotice, response.Data)
		})
	}
}
