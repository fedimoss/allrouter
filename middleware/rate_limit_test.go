package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUploadRateLimitReturnsMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())

	previousNum := common.UploadRateLimitNum
	previousRedisEnabled := common.RedisEnabled
	common.UploadRateLimitNum = 1
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.UploadRateLimitNum = previousNum
		common.RedisEnabled = previousRedisEnabled
	})

	tests := []struct {
		name         string
		acceptLang   string
		expectedLang string
		remoteAddr   string
		userSetting  *dto.UserSetting
	}{
		{
			name:         "Chinese",
			acceptLang:   "zh-CN",
			expectedLang: i18n.LangZhCN,
			remoteAddr:   "192.0.2.1:1234",
		},
		{
			name:         "English",
			acceptLang:   "en",
			expectedLang: i18n.LangEn,
			remoteAddr:   "192.0.2.2:1234",
		},
		{
			name:         "UserSettingWins",
			acceptLang:   "en",
			expectedLang: i18n.LangZhTW,
			remoteAddr:   "192.0.2.3:1234",
			userSetting:  &dto.UserSetting{Language: "zh-TW"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
			handlerCalls := 0
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if test.userSetting != nil {
					common.SetContextKey(c, constant.ContextKeyUserSetting, *test.userSetting)
				}
				c.Next()
			})
			router.POST("/upload", UploadRateLimit(), func(c *gin.Context) {
				handlerCalls++
				c.Status(http.StatusNoContent)
			})

			firstRequest := httptest.NewRequest(http.MethodPost, "/upload", nil)
			firstRequest.RemoteAddr = test.remoteAddr
			first := httptest.NewRecorder()
			router.ServeHTTP(first, firstRequest)
			require.Equal(t, http.StatusNoContent, first.Code)

			secondRequest := httptest.NewRequest(http.MethodPost, "/upload", nil)
			secondRequest.RemoteAddr = test.remoteAddr
			secondRequest.Header.Set("Accept-Language", test.acceptLang)
			second := httptest.NewRecorder()
			router.ServeHTTP(second, secondRequest)
			require.Equal(t, http.StatusTooManyRequests, second.Code)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(second.Body.Bytes(), &response))
			require.False(t, response.Success)
			require.Equal(t, i18n.Translate(test.expectedLang, i18n.MsgUploadRateLimitReached), response.Message)
			require.Equal(t, 1, handlerCalls)
		})
	}
}
