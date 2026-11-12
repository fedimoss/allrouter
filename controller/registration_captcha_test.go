package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/registrationcaptcha"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type captchaTestClient struct {
	router  *gin.Engine
	cookies []*http.Cookie
}

func (client *captchaTestClient) request(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = common.Marshal(body)
		require.NoError(t, err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(data))
	request.RemoteAddr = "192.0.2.44:1234"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept-Language", "en")
	request.Header.Set("New-Api-User", "31")
	for _, value := range client.cookies {
		request.AddCookie(value)
	}
	response := httptest.NewRecorder()
	client.router.ServeHTTP(response, request)
	if values := response.Result().Cookies(); len(values) > 0 {
		client.cookies = values
	}
	return response
}

func captchaTestResponse(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	return body
}

func newCaptchaTestEnvironment(t *testing.T) (*captchaTestClient, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	oldEnabled, oldClient, oldDB := common.RedisEnabled, common.RDB, model.DB
	oldRegister, oldPassword, oldEmail := common.RegisterEnabled, common.PasswordRegisterEnabled, common.EmailVerificationEnabled
	oldDomain, oldAlias := common.EmailDomainRestrictionEnabled, common.EmailAliasRestrictionEnabled
	oldDefaultToken := constant.GenerateDefaultToken
	oldNewQuota, oldGift := common.QuotaForNewUser, common.RegisterGiftSubscriptionPlanId
	common.RegisterEnabled, common.PasswordRegisterEnabled, common.EmailVerificationEnabled = true, true, true
	common.EmailDomainRestrictionEnabled, common.EmailAliasRestrictionEnabled = false, false
	constant.GenerateDefaultToken = false
	common.QuotaForNewUser, common.RegisterGiftSubscriptionPlanId = 0, 0
	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	common.RedisEnabled, common.RDB = true, redisClient
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.CliUser{}))
	model.DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		common.RedisEnabled, common.RDB, model.DB = oldEnabled, oldClient, oldDB
		common.RegisterEnabled, common.PasswordRegisterEnabled, common.EmailVerificationEnabled = oldRegister, oldPassword, oldEmail
		common.EmailDomainRestrictionEnabled, common.EmailAliasRestrictionEnabled = oldDomain, oldAlias
		constant.GenerateDefaultToken = oldDefaultToken
		common.QuotaForNewUser, common.RegisterGiftSubscriptionPlanId = oldNewQuota, oldGift
		_ = redisClient.Close()
		_ = sqlDB.Close()
	})
	router := gin.New()
	store := cookie.NewStore([]byte("registration-captcha-integration-test-secret"))
	store.Options(sessions.Options{Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	router.Use(sessions.Sessions("session", store))
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyProviderId, 0)
		c.Next()
	})
	router.POST("/api/user/register/captcha", CreateRegistrationCaptcha)
	router.POST("/api/user/register/captcha/verify", VerifyRegistrationCaptcha)
	router.POST("/api/user/register/verification", SendRegistrationEmailVerification)
	router.POST("/api/user/register", Register)
	router.GET("/api/verification", middleware.UserAuth(), SendEmailVerification)
	router.GET("/test/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "binding-user")
		session.Set("id", 31)
		session.Set("role", common.RoleCommonUser)
		session.Set("status", common.UserStatusEnabled)
		session.Set("provider_id", 0)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	return &captchaTestClient{router: router}, server
}

func solveCaptchaThroughHTTP(t *testing.T, client *captchaTestClient, server *miniredis.Miniredis, email string) registrationcaptcha.Grants {
	t.Helper()
	response := client.request(t, http.MethodPost, "/api/user/register/captcha", map[string]any{"email": email})
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "no-store, private", response.Header().Get("Cache-Control"))
	var result struct {
		Success bool                          `json:"success"`
		Data    registrationcaptcha.Challenge `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &result))
	require.True(t, result.Success)
	hash := sha256.Sum256([]byte(result.Data.ID))
	key := fmt.Sprintf("new-api:registration_captcha:v1:challenge:%x", hash)
	stored, err := server.Get(key)
	require.NoError(t, err)
	// Test-only access to the backend store. The HTTP response never contains
	// these answer coordinates; adjusting the timestamp avoids sleeps in tests.
	var answer struct {
		Targets  []struct{ X, Y, Radius float64 }
		IssuedAt int64
	}
	require.NoError(t, common.UnmarshalJsonStr(stored[65:], &answer))
	answer.IssuedAt = time.Now().Add(-2 * time.Second).UnixMilli()
	encoded, err := common.Marshal(answer)
	require.NoError(t, err)
	require.NoError(t, server.Set(key, stored[:65]+string(encoded)))
	server.SetTTL(key, registrationcaptcha.ChallengeTTL)
	clicks := make([]registrationcaptcha.Click, len(answer.Targets))
	for i, point := range answer.Targets {
		clicks[i] = registrationcaptcha.Click{X: point.X, Y: point.Y, TimeMS: int64(i+1) * 200}
	}
	response = client.request(t, http.MethodPost, "/api/user/register/captcha/verify", map[string]any{
		"email": email, "challenge_id": result.Data.ID, "clicks": clicks,
	})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var verified struct {
		Success bool                       `json:"success"`
		Data    registrationcaptcha.Grants `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &verified))
	require.True(t, verified.Success)
	return verified.Data
}

func startCaptchaTestSMTP(t *testing.T) (*atomic.Int64, <-chan string) {
	t.Helper()
	return startCaptchaTestSMTPWithAuthFailure(t, nil)
}

func startCaptchaTestSMTPWithAuthFailure(t *testing.T, rejectAuth *atomic.Bool) (*atomic.Int64, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	oldHost, oldPort, oldAccount, oldFrom := common.SMTPServer, common.SMTPPort, common.SMTPAccount, common.SMTPFrom
	oldToken, oldSSL, oldForce := common.SMTPToken, common.SMTPSSLEnabled, common.SMTPForceAuthLogin
	common.SMTPServer, common.SMTPPort, common.SMTPAccount, common.SMTPFrom = "127.0.0.1", port, "", "no-reply@example.com"
	common.SMTPToken, common.SMTPSSLEnabled, common.SMTPForceAuthLogin = "", false, false
	t.Cleanup(func() {
		_ = listener.Close()
		common.SMTPServer, common.SMTPPort, common.SMTPAccount, common.SMTPFrom = oldHost, oldPort, oldAccount, oldFrom
		common.SMTPToken, common.SMTPSSLEnabled, common.SMTPForceAuthLogin = oldToken, oldSSL, oldForce
	})
	count := &atomic.Int64{}
	messages := make(chan string, 8)
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer connection.Close()
				_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
				protocol := textproto.NewConn(connection)
				_ = protocol.PrintfLine("220 localhost captcha-test")
				for {
					line, err := protocol.ReadLine()
					if err != nil {
						return
					}
					command := strings.ToUpper(strings.Fields(line)[0])
					switch command {
					case "EHLO", "HELO":
						_ = protocol.PrintfLine("250-localhost\r\n250 AUTH PLAIN")
					case "AUTH":
						if rejectAuth != nil && rejectAuth.Load() {
							_ = protocol.PrintfLine("535 Login fail")
						} else {
							_ = protocol.PrintfLine("235 Authentication successful")
						}
					case "DATA":
						_ = protocol.PrintfLine("354 Send message")
						body, err := io.ReadAll(protocol.DotReader())
						if err != nil {
							return
						}
						count.Add(1)
						messages <- string(body)
						_ = protocol.PrintfLine("250 Message accepted")
					case "QUIT":
						_ = protocol.PrintfLine("221 Goodbye")
						return
					default:
						_ = protocol.PrintfLine("250 OK")
					}
				}
			}()
		}
	}()
	return count, messages
}

func TestRegistrationCaptchaGatesMailAndPreservesRegistrationFlow(t *testing.T) {
	client, server := newCaptchaTestEnvironment(t)
	mailCount, messages := startCaptchaTestSMTP(t)
	email := "human@example.com"
	response := client.request(t, http.MethodPost, "/api/user/register/verification", map[string]any{"email": email})
	require.Equal(t, http.StatusForbidden, response.Code)
	require.Zero(t, mailCount.Load())
	response = client.request(t, http.MethodGet, "/api/verification?email="+email, nil)
	require.Equal(t, http.StatusUnauthorized, response.Code, "anonymous legacy GET must not bypass captcha")

	grants := solveCaptchaThroughHTTP(t, client, server, email)
	otherBrowser := &captchaTestClient{router: client.router}
	response = otherBrowser.request(t, http.MethodPost, "/api/user/register/verification", map[string]any{
		"email": email, "captcha_token": grants.EmailToken,
	})
	require.Equal(t, http.StatusForbidden, response.Code)
	response = client.request(t, http.MethodPost, "/api/user/register/verification", map[string]any{
		"email": email, "captcha_token": grants.EmailToken,
	})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, true, captchaTestResponse(t, response)["success"])
	require.EqualValues(t, 60, captchaTestResponse(t, response)["retry_after"])
	require.EqualValues(t, 1, mailCount.Load())
	hash := sha256.Sum256([]byte(email))
	code, err := server.Get(fmt.Sprintf("new-api:verification_code:v1:0:v:%x", hash))
	require.NoError(t, err)
	require.Len(t, code, 6)
	select {
	case body := <-messages:
		require.Contains(t, body, code)
	case <-time.After(time.Second):
		t.Fatal("the verification email was not delivered to the local SMTP fixture")
	}
	response = client.request(t, http.MethodPost, "/api/user/register/verification", map[string]any{
		"email": email, "captcha_token": grants.EmailToken,
	})
	require.Equal(t, http.StatusForbidden, response.Code, "email token replay must be rejected")

	resendGrants := solveCaptchaThroughHTTP(t, client, server, email)
	response = client.request(t, http.MethodPost, "/api/user/register/verification", map[string]any{
		"email": email, "captcha_token": resendGrants.EmailToken,
	})
	require.Equal(t, http.StatusTooManyRequests, response.Code)
	retryAfter, err := strconv.Atoi(response.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.InDelta(t, 60, retryAfter, 1)
	require.EqualValues(t, retryAfter, captchaTestResponse(t, response)["retry_after"])
	require.EqualValues(t, 1, mailCount.Load())

	payload := map[string]any{
		"username": "human-test", "email": email, "password": "HumanTest123!", "verification_code": "wrong!",
		"captcha_token": grants.RegistrationToken, "role": common.RoleRootUser, "quota": 999999,
	}
	response = client.request(t, http.MethodPost, "/api/user/register", payload)
	require.Equal(t, false, captchaTestResponse(t, response)["success"])
	require.Equal(t, http.StatusOK, response.Code, "invalid email code must keep the captcha grant for a normal retry")
	payload["verification_code"] = code
	response = client.request(t, http.MethodPost, "/api/user/register", payload)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, true, captchaTestResponse(t, response)["success"], response.Body.String())
	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "human-test").First(&user).Error)
	require.Equal(t, common.RoleCommonUser, user.Role)
	require.Equal(t, 0, user.Quota)
	require.Equal(t, email, user.Email)
	require.Equal(t, "192.0.2.44", user.RegisterIp)
	require.NotEqual(t, payload["password"], user.Password)

	response = client.request(t, http.MethodPost, "/api/user/register", payload)
	require.Equal(t, http.StatusForbidden, response.Code, "registration grant replay must be rejected")
	common.EmailVerificationEnabled = false
	response = otherBrowser.request(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "no-email-captcha", "password": "HumanTest123!",
	})
	require.Equal(t, http.StatusForbidden, response.Code, "disabling email verification must not disable human verification")
	noEmailGrants := solveCaptchaThroughHTTP(t, otherBrowser, server, "")
	response = otherBrowser.request(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "no-email-captcha", "password": "HumanTest123!", "captcha_token": noEmailGrants.RegistrationToken,
	})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, true, captchaTestResponse(t, response)["success"], "existing no-email registration must still work after captcha")
	server.FastForward(registrationcaptcha.EmailSendInterval)
	reservation, wait, err := registrationcaptcha.ReserveEmailSend(context.Background(), 0, "192.0.2.44", email)
	require.NoError(t, err)
	require.Zero(t, wait)
	require.NotNil(t, reservation)
}

func TestLegacyVerificationStillSupportsAuthenticatedEmailBinding(t *testing.T) {
	client, _ := newCaptchaTestEnvironment(t)
	mailCount, _ := startCaptchaTestSMTP(t)
	response := client.request(t, http.MethodGet, "/test/login", nil)
	require.Equal(t, http.StatusNoContent, response.Code)
	response = client.request(t, http.MethodGet, "/api/verification?email=binding@example.com", nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, true, captchaTestResponse(t, response)["success"])
	require.EqualValues(t, 1, mailCount.Load())
	response = client.request(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "binding-bypass", "email": "binding@example.com", "password": "HumanTest123!", "verification_code": "123456",
	})
	require.Equal(t, http.StatusForbidden, response.Code, "an authenticated session cannot bypass registration captcha")
}

func TestRegistrationCaptchaMailFailureReleasesCooldown(t *testing.T) {
	client, server := newCaptchaTestEnvironment(t)
	var rejectAuth atomic.Bool
	rejectAuth.Store(true)
	mailCount, _ := startCaptchaTestSMTPWithAuthFailure(t, &rejectAuth)
	email := "retry@example.com"
	codeHash := sha256.Sum256([]byte(email))
	codeKey := fmt.Sprintf("new-api:verification_code:v1:0:v:%x", codeHash)

	grants := solveCaptchaThroughHTTP(t, client, server, email)
	response := client.request(t, http.MethodPost, "/api/user/register/verification", map[string]any{
		"email": email, "captcha_token": grants.EmailToken,
	})
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, false, captchaTestResponse(t, response)["success"])
	require.NotContains(t, response.Body.String(), "535", "SMTP details must stay server-side")
	require.False(t, server.Exists(codeKey), "an unsent code must not remain valid")
	require.Zero(t, mailCount.Load())

	// Recover SMTP and retry immediately, without advancing the 60-second clock.
	rejectAuth.Store(false)
	grants = solveCaptchaThroughHTTP(t, client, server, email)
	response = client.request(t, http.MethodPost, "/api/user/register/verification", map[string]any{
		"email": email, "captcha_token": grants.EmailToken,
	})
	require.Equal(t, http.StatusOK, response.Code, "failed SMTP must release the send reservation: %s", response.Body.String())
	require.Equal(t, true, captchaTestResponse(t, response)["success"])
	require.EqualValues(t, 60, captchaTestResponse(t, response)["retry_after"])
	require.True(t, server.Exists(codeKey))
	require.EqualValues(t, 1, mailCount.Load())

	// A successful send must still enforce the original interval.
	grants = solveCaptchaThroughHTTP(t, client, server, email)
	response = client.request(t, http.MethodPost, "/api/user/register/verification", map[string]any{
		"email": email, "captcha_token": grants.EmailToken,
	})
	require.Equal(t, http.StatusTooManyRequests, response.Code)
	require.Equal(t, false, captchaTestResponse(t, response)["success"])
	require.EqualValues(t, 60, captchaTestResponse(t, response)["retry_after"])
	require.EqualValues(t, 1, mailCount.Load(), "rate-limited requests must not send mail")
}
