package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"

	"github.com/gin-gonic/gin"
)

type upstreamTaskErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Param   string `json:"param"`
		Type    string `json:"type"`
	} `json:"error"`
}

// LocalizeTaskError replaces known raw upstream errors with stable localized
// messages while preserving a stable public error code for the frontend.
// 风险：只映射已知上游错误；未知错误保持原样，避免掩盖真实排障信息。
func LocalizeTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr == nil {
		return
	}
	key, publicCode := classifyTaskErrorMessage(taskErr.Message)
	if key == "" {
		return
	}
	taskErr.Message = i18n.T(c, key)
	taskErr.Code = publicCode
}

func classifyTaskErrorMessage(message string) (string, string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", ""
	}

	var envelope upstreamTaskErrorEnvelope
	if err := common.Unmarshal([]byte(message), &envelope); err == nil {
		code := strings.ToLower(strings.TrimSpace(envelope.Error.Code))
		upstreamMessage := strings.ToLower(envelope.Error.Message)
		// 注意：真人隐私判断必须放在通用敏感内容之前，否则会丢失更精确的提示。
		switch {
		case strings.Contains(code, "sensitivecontentdetected") &&
			strings.Contains(code, "privacyinformation"):
			return i18n.MsgTaskContentSensitiveRealPerson, "content_sensitive"
		case strings.Contains(code, "sensitivecontentdetected"):
			return i18n.MsgTaskContentSensitive, "content_sensitive"
		case strings.Contains(code, "invalidparameter") &&
			isImagePixelConstraint(upstreamMessage):
			return i18n.MsgTaskImageResolutionTooLow, "image_resolution_too_low"
		}
	}

	lowerMessage := strings.ToLower(message)
	// 兼容：部分上游返回纯文本而非 JSON，保留字符串兜底匹配。
	if strings.Contains(lowerMessage, "sensitivecontentdetected") &&
		strings.Contains(lowerMessage, "privacyinformation") {
		return i18n.MsgTaskContentSensitiveRealPerson, "content_sensitive"
	}
	if strings.Contains(lowerMessage, "sensitivecontentdetected") {
		return i18n.MsgTaskContentSensitive, "content_sensitive"
	}
	if isImagePixelConstraint(lowerMessage) {
		return i18n.MsgTaskImageResolutionTooLow, "image_resolution_too_low"
	}
	return "", ""
}

func isImagePixelConstraint(message string) bool {
	// 注意：限定图片像素相关措辞，避免把所有 InvalidParameter 都误判成同类错误。
	if !strings.Contains(message, "pixel") {
		return false
	}
	return strings.Contains(message, "image pixel count") ||
		(strings.Contains(message, "image") &&
			strings.Contains(message, "greater than or equal"))
}
