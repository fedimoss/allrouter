package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 逻辑：覆盖真人隐私、通用敏感内容和图片像素不足三类上游错误。
// 注意：未识别的错误必须保持原样，不能被误归类。
func TestClassifyTaskErrorMessage(t *testing.T) {
	key, code := classifyTaskErrorMessage(`{"error":{"code":"InputImageSensitiveContentDetected.PrivacyInformation","message":"may contain real person","param":"content[1]","type":"BadRequest"}}`)
	require.Equal(t, i18n.MsgTaskContentSensitiveRealPerson, key)
	require.Equal(t, "content_sensitive", code)

	key, code = classifyTaskErrorMessage(`{"error":{"code":"InputImageSensitiveContentDetected","message":"sensitive"}}`)
	require.Equal(t, i18n.MsgTaskContentSensitive, key)
	require.Equal(t, "content_sensitive", code)

	key, code = classifyTaskErrorMessage(`{"error":{"code":"InvalidParameter","message":"other"}}`)
	require.Empty(t, key)
	require.Empty(t, code)

	key, code = classifyTaskErrorMessage(`{"error":{"code":"InvalidParameter","message":"The parameter content[1] specified in the request is not valid: the parameter image pixel count specified in the request must be greater than or equal to 90000 for model doubao-seedance-2-5 in r2v, but received a 215x145px image (31175 pixels) instead."}}`)
	require.Equal(t, i18n.MsgTaskImageResolutionTooLow, key)
	require.Equal(t, "image_resolution_too_low", code)
}

func TestClassifyTaskErrorMessageFallbackText(t *testing.T) {
	key, code := classifyTaskErrorMessage(`upstream failed: InputImageSensitiveContentDetected.PrivacyInformation`)
	require.Equal(t, i18n.MsgTaskContentSensitiveRealPerson, key)
	require.Equal(t, "content_sensitive", code)
}

func TestLocalizeTaskErrorUsesTranslation(t *testing.T) {
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/pg/videos", nil)
	taskErr := &dto.TaskError{
		Code:    "fail_to_fetch_task",
		Message: `{"error":{"code":"InputImageSensitiveContentDetected.PrivacyInformation"}}`,
	}

	LocalizeTaskError(ctx, taskErr)

	require.Equal(t, "content_sensitive", taskErr.Code)
	require.NotContains(t, taskErr.Message, "InputImageSensitiveContentDetected")
	require.Contains(t, taskErr.Message, "real person")
	require.Contains(t, i18n.Translate(i18n.LangZhCN, i18n.MsgTaskContentSensitiveRealPerson), "真人")
}

func TestLocalizeTaskErrorImageResolution(t *testing.T) {
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/pg/videos", nil)
	taskErr := &dto.TaskError{
		Code:    "fail_to_fetch_task",
		Message: `{"error":{"code":"InvalidParameter","message":"the parameter image pixel count specified in the request must be greater than or equal to 90000"}}`,
	}

	LocalizeTaskError(ctx, taskErr)

	require.Equal(t, "image_resolution_too_low", taskErr.Code)
	require.NotContains(t, taskErr.Message, "pixel count")
	require.Contains(t, i18n.Translate(i18n.LangZhCN, i18n.MsgTaskImageResolutionTooLow), "分辨率")
}
