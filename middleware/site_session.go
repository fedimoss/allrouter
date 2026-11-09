package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// SiteSessions keeps legacy domain sessions and uses separate HTTP cookies for
// each IP:port origin. Stores are initialized once, never mutated per request.
func SiteSessions() gin.HandlerFunc {
	domainStore := cookie.NewStore([]byte(common.SessionSecret))
	ipStore := cookie.NewStore([]byte(common.SessionSecret))
	options := sessions.Options{
		Path:     "/",
		MaxAge:   common.SessionMaxAgeDefault,
		HttpOnly: true,
		Secure:   common.SessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
	}
	domainStore.Options(options)
	options.Secure = false
	ipStore.Options(options)
	return func(c *gin.Context) {
		store := domainStore
		if common.IPRequestAddress(c.Request) != "" {
			store = ipStore
		}
		sessions.Sessions(common.SessionCookieName(c.Request), store)(c)
	}
}
