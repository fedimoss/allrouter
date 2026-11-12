package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegistrationCaptchaRoutesHaveNoAnonymousMailBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	oldRedis := common.RedisEnabled
	oldAPI, oldCritical := common.GlobalApiRateLimitEnable, common.CriticalRateLimitEnable
	oldRegister, oldPassword, oldEmail := common.RegisterEnabled, common.PasswordRegisterEnabled, common.EmailVerificationEnabled
	common.RedisEnabled, common.GlobalApiRateLimitEnable, common.CriticalRateLimitEnable = false, false, false
	common.RegisterEnabled, common.PasswordRegisterEnabled, common.EmailVerificationEnabled = true, true, true
	t.Cleanup(func() {
		common.RedisEnabled = oldRedis
		common.GlobalApiRateLimitEnable, common.CriticalRateLimitEnable = oldAPI, oldCritical
		common.RegisterEnabled, common.PasswordRegisterEnabled, common.EmailVerificationEnabled = oldRegister, oldPassword, oldEmail
	})
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("registration-captcha-route-test-key"))))
	SetApiRouter(engine)
	for _, request := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodGet, "/api/verification?email=route@example.com", "", http.StatusUnauthorized},
		{http.MethodPost, "/api/user/register/verification", `{"email":"route@example.com"}`, http.StatusForbidden},
		{http.MethodPost, "/api/user/register", `{"username":"route-test","password":"HumanTest123!","email":"route@example.com"}`, http.StatusForbidden},
		{http.MethodPost, "/api/user/register/captcha", `{"email":"route@example.com"}`, http.StatusOK},
	} {
		response := httptest.NewRecorder()
		req := httptest.NewRequest(request.method, request.path, bytes.NewBufferString(request.body))
		req.RemoteAddr = "192.0.2.55:1234"
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(response, req)
		require.Equal(t, request.status, response.Code, "%s %s: %s", request.method, request.path, response.Body.String())
	}
}
