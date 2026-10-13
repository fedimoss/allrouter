package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseWechatQRCodeRequestJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/wechat/qrcode", strings.NewReader(`{"scene_str":"test123"}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	parsed, ok := parseWechatQRCodeRequest(c)
	require.True(t, ok)
	require.Equal(t, "test123", parsed.SceneStr)
}

func TestParseWechatQRCodeRequestSceneID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/wechat/qrcode", strings.NewReader(`{"scene_id":123}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	parsed, ok := parseWechatQRCodeRequest(c)
	require.True(t, ok)
	require.NotNil(t, parsed.SceneID)
	require.Equal(t, 123, *parsed.SceneID)
}

func TestParseWechatQRCodeRequestQuerySceneStr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/wechat/qrcode?scene_str=from_query", nil)
	c.Request = req

	parsed, ok := parseWechatQRCodeRequest(c)
	require.True(t, ok)
	require.Equal(t, "from_query", parsed.SceneStr)
}

func TestParseWechatQRCodeRequestRejectsFormBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/wechat/qrcode", strings.NewReader("scene_str=test123"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req

	_, ok := parseWechatQRCodeRequest(c)
	require.False(t, ok)
}

// TestParseWechatQRCodeRequestDefaultPlaceholder 验证不传任何场景值时
// 回退到占位 scene_str，从而无需参数也能直接调用联调。
func TestParseWechatQRCodeRequestDefaultPlaceholder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/api/wechat/qrcode", nil)
	c.Request = req

	parsed, ok := parseWechatQRCodeRequest(c)
	require.True(t, ok)
	require.Equal(t, wechatQRCodePlaceholderSceneStr, parsed.SceneStr)
	require.Nil(t, parsed.SceneID)
}
