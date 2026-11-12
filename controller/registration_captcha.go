package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/registrationcaptcha"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const registrationCaptchaSessionKey = "registration_captcha_session"

type registrationCaptchaRequest struct {
	Email string `json:"email"`
}

type registrationCaptchaVerifyRequest struct {
	Email       string                      `json:"email"`
	ChallengeID string                      `json:"challenge_id"`
	Clicks      []registrationcaptcha.Click `json:"clicks"`
}

type registrationEmailVerificationRequest struct {
	Email        string `json:"email"`
	CaptchaToken string `json:"captcha_token"`
}

func registrationCaptchaNoCache(c *gin.Context) {
	c.Header("Cache-Control", "no-store, private")
	c.Header("Pragma", "no-cache")
}

func registrationCaptchaRequired(c *gin.Context) {
	registrationCaptchaNoCache(c)
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"code":    "registration_captcha_required",
		"message": common.TranslateMessage(c, i18n.MsgRegistrationCaptchaRequired),
	})
}

func registrationCaptchaUnavailable(c *gin.Context, err error) {
	logger.LogError(c.Request.Context(), "registration captcha: "+err.Error())
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"success": false,
		"message": common.TranslateMessage(c, i18n.MsgRetryLater),
	})
}

func registrationCaptchaAllowed(c *gin.Context) bool {
	if !common.RegisterEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserRegisterDisabled)
		return false
	}
	if !common.PasswordRegisterEnabled {
		common.ApiErrorI18n(c, i18n.MsgUserPasswordRegisterDisabled)
		return false
	}
	return true
}

func validRegistrationCaptchaEmail(email string) bool {
	if len(email) > 50 || (common.EmailVerificationEnabled && email == "") {
		return false
	}
	return email == "" || common.Validate.Var(email, "email") == nil
}

func registrationCaptchaScope(c *gin.Context, email string, create bool) (registrationcaptcha.Scope, bool) {
	var scope registrationcaptcha.Scope
	session := sessions.Default(c)
	identifier, _ := session.Get(registrationCaptchaSessionKey).(string)
	if identifier == "" {
		if !create {
			registrationCaptchaRequired(c)
			return scope, false
		}
		var err error
		identifier, err = registrationcaptcha.NewSessionID()
		if err != nil {
			registrationCaptchaUnavailable(c, err)
			return scope, false
		}
		session.Set(registrationCaptchaSessionKey, identifier)
		if err = session.Save(); err != nil {
			registrationCaptchaUnavailable(c, err)
			return scope, false
		}
	}
	return registrationcaptcha.Scope{
		ProviderID: common.GetContextKeyInt(c, constant.ContextKeyProviderId),
		Session:    identifier,
		IP:         c.ClientIP(),
		Email:      email,
	}, true
}

func CreateRegistrationCaptcha(c *gin.Context) {
	registrationCaptchaNoCache(c)
	if !registrationCaptchaAllowed(c) {
		return
	}
	var request registrationCaptchaRequest
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 4096), &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	request.Email = strings.TrimSpace(request.Email)
	if !validRegistrationCaptchaEmail(request.Email) {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	scope, ok := registrationCaptchaScope(c, request.Email, true)
	if !ok {
		return
	}
	challenge, err := registrationcaptcha.Create(c.Request.Context(), scope)
	var limited *registrationcaptcha.RateLimitError
	if errors.As(err, &limited) {
		seconds := int((limited.RetryAfter + time.Second - 1) / time.Second)
		c.Header("Retry-After", strconv.Itoa(seconds))
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success":     false,
			"retry_after": seconds,
			"message":     common.TranslateMessage(c, i18n.MsgRegistrationCaptchaRateLimit, map[string]any{"Seconds": seconds}),
		})
		return
	}
	if err != nil {
		registrationCaptchaUnavailable(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": challenge})
}

func VerifyRegistrationCaptcha(c *gin.Context) {
	registrationCaptchaNoCache(c)
	if !registrationCaptchaAllowed(c) {
		return
	}
	var request registrationCaptchaVerifyRequest
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 4096), &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	request.Email = strings.TrimSpace(request.Email)
	if !validRegistrationCaptchaEmail(request.Email) {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	scope, ok := registrationCaptchaScope(c, request.Email, false)
	if !ok {
		return
	}
	grants, err := registrationcaptcha.Verify(c.Request.Context(), request.ChallengeID, scope, request.Clicks)
	if errors.Is(err, registrationcaptcha.ErrInvalidChallenge) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "registration_captcha_invalid",
			"message": common.TranslateMessage(c, i18n.MsgRegistrationCaptchaInvalid),
		})
		return
	}
	if err != nil {
		registrationCaptchaUnavailable(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": grants})
}

func SendRegistrationEmailVerification(c *gin.Context) {
	registrationCaptchaNoCache(c)
	if !registrationCaptchaAllowed(c) {
		return
	}
	var request registrationEmailVerificationRequest
	if err := common.DecodeJson(http.MaxBytesReader(c.Writer, c.Request.Body, 4096), &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	request.Email = strings.TrimSpace(request.Email)
	if request.Email == "" || !validRegistrationCaptchaEmail(request.Email) {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	scope, ok := registrationCaptchaScope(c, request.Email, false)
	if !ok {
		return
	}
	verified, err := registrationcaptcha.ConsumeGrant(c.Request.Context(), request.CaptchaToken, scope, registrationcaptcha.EmailPurpose)
	if err != nil {
		registrationCaptchaUnavailable(c, err)
		return
	}
	if !verified {
		registrationCaptchaRequired(c)
		return
	}
	sendEmailVerification(c, request.Email)
}
