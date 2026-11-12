package middleware

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSiteSessionsIsolateIPPorts(t *testing.T) {
	oldSecret, oldSecure := common.SessionSecret, common.SessionCookieSecure
	common.SessionSecret = "provider-address-session-test-key"
	common.SessionCookieSecure = true
	t.Cleanup(func() {
		common.SessionSecret, common.SessionCookieSecure = oldSecret, oldSecure
	})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SiteSessions())
	router.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		if value, exists := c.GetQuery("value"); exists {
			if value == "" {
				session.Clear()
			} else {
				session.Set("value", value)
			}
			require.NoError(t, session.Save())
		}
		value, _ := session.Get("value").(string)
		c.String(http.StatusOK, value)
	})
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	request := func(rawURL string) *httptest.ResponseRecorder {
		t.Helper()
		address, err := url.Parse(rawURL)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodGet, rawURL, nil)
		for _, cookie := range jar.Cookies(address) {
			req.AddCookie(cookie)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		jar.SetCookies(address, recorder.Result().Cookies())
		return recorder
	}
	first := request("http://203.0.113.10:8001/session?value=provider-one")
	require.False(t, first.Result().Cookies()[0].Secure)
	request("http://203.0.113.10:8002/session?value=provider-two")
	require.Equal(t, "provider-one", request("http://203.0.113.10:8001/session").Body.String())
	require.Equal(t, "provider-two", request("http://203.0.113.10:8002/session").Body.String())
	request("http://203.0.113.10:8002/session?value=")
	require.Equal(t, "provider-one", request("http://203.0.113.10:8001/session").Body.String())
	require.Empty(t, request("http://203.0.113.10:8002/session").Body.String())
	domain := request("https://ai.example.com/session?value=domain-user")
	require.Equal(t, "session", domain.Result().Cookies()[0].Name)
	require.True(t, domain.Result().Cookies()[0].Secure)
}
