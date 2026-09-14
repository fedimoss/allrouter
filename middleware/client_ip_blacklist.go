package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// ClientIPBlacklist rejects requests whose resolved source IP is present in
// the administrator-managed ClientIPBlacklist option. It is deliberately
// separate from the SSRF IP list: this check applies to an incoming client,
// while SSRF protection applies to outbound URL targets.
func ClientIPBlacklist() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !setting.IsClientIPBlacklisted(c.ClientIP()) {
			c.Next()
			return
		}

		// Preserve the OpenAI-compatible error envelope for model/relay paths;
		// dashboard and other browser/API endpoints use the normal success/message
		// envelope expected by the frontend.
		if isRelayPath(c.Request.URL.Path) {
			abortWithOpenAiMessage(c, http.StatusForbidden, "客户端 IP 已被禁止访问", types.ErrorCodeAccessDenied)
			return
		}

		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "客户端 IP 已被禁止访问",
		})
		c.Abort()
	}
}

func isRelayPath(path string) bool {
	for _, prefix := range []string{"/v1", "/v1beta", "/mj", "/suno", "/pg"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
