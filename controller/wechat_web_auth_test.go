package controller

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestWechatWebAuthConfigReturnsAuthorizeURL 校验配置接口能返回可用的授权链接。
func TestWechatWebAuthConfigReturnsAuthorizeURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	query := url.Values{}
	query.Set("redirect_uri", "https://allrouter.ai/wechat/scan")
	c.Request = httptest.NewRequest("GET", "/api/wechat/web_auth/config?"+query.Encode(), nil)

	WechatWebAuthDebugConfig(c)

	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `"success":true`)
	require.Contains(t, body, "scope=snsapi_base")
	require.Contains(t, body, "open.weixin.qq.com")
}

// TestWechatWebAuthConfigMissingRedirect 校验缺少 redirect_uri 时返回业务错误。
func TestWechatWebAuthConfigMissingRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/api/wechat/web_auth/config", nil)

	WechatWebAuthDebugConfig(c)

	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"success":false`)
}

// TestWechatWebAuthTokenMissingCode 校验缺少 code 时返回业务错误（不触发网络/DB）。
func TestWechatWebAuthTokenMissingCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("GET", "/api/wechat/web_auth/token", nil)

	WechatWebAuthToken(c)

	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"success":false`)
	require.Contains(t, rec.Body.String(), "code")
}
