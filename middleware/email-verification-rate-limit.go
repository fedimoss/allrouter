package middleware

import (
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/pkg/registrationcaptcha"
	"github.com/gin-gonic/gin"
)

// Call after human verification and email validation, immediately before mail
// delivery. Unverified requests must not reserve a victim's 60-second cooldown.
func EmailVerificationRateLimit(c *gin.Context, email string) (*registrationcaptcha.EmailSendReservation, bool) {
	providerID := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	reservation, wait, err := registrationcaptcha.ReserveEmailSend(c.Request.Context(), providerID, c.ClientIP(), email)
	if err != nil {
		common.SysLog("email verification rate limiter: " + err.Error())
		setRetryAfterForStatus(c, http.StatusServiceUnavailable)
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": common.TranslateMessage(c, i18n.MsgRetryLater),
		})
		return nil, false
	}
	if wait > 0 {
		seconds := int((wait + time.Second - 1) / time.Second)
		setRetryAfter(c, seconds)
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"success":     false,
			"retry_after": seconds,
			"message":     common.TranslateMessage(c, i18n.MsgEmailVerificationRateLimit, map[string]any{"Seconds": seconds}),
		})
		return nil, false
	}
	return reservation, true
}
