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
// 只映射已知上游错误；未知错误保持原样，避免掩盖真实排障信息。
// 不能把匹配范围放得太宽，否则会把普通参数错误误报成内容安全错误。
func LocalizeTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr == nil {
		return
	}
	key, publicCode := classifyTaskErrorMessage(taskErr.Message)
	if key == "" {
		// 未识别的错误保持原文，避免吞掉上游排障信息。
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
		// 结构化错误优先按 code + message 判断，准确率高于纯文本匹配。
		code := strings.ToLower(strings.TrimSpace(envelope.Error.Code))
		upstreamMessage := strings.ToLower(envelope.Error.Message)
		// 真人隐私判断必须放在通用敏感内容之前，否则会丢失更精确的提示。
		switch {
		case strings.Contains(code, "sensitivecontentdetected") &&
			strings.Contains(code, "privacyinformation"):
			// 真人隐私拦截比通用敏感内容更具体，先匹配。
			return i18n.MsgTaskContentSensitiveRealPerson, "content_sensitive"
		case strings.Contains(code, "sensitivecontentdetected"):
			// 其他敏感内容统一归为 content_sensitive。
			return i18n.MsgTaskContentSensitive, "content_sensitive"
		case strings.Contains(code, "invalidparameter") &&
			isImagePixelConstraint(upstreamMessage):
			// 图片像素不足是上游校验，不复制其阈值，只给出友好提示。
			return i18n.MsgTaskImageResolutionTooLow, "image_resolution_too_low"
		}
	}

	lowerMessage := strings.ToLower(message)
	// 部分上游返回纯文本而非 JSON，保留字符串兜底匹配。
	if strings.Contains(lowerMessage, "sensitivecontentdetected") &&
		strings.Contains(lowerMessage, "privacyinformation") {
		// 非 JSON 文本同样保留真人隐私优先判断。
		return i18n.MsgTaskContentSensitiveRealPerson, "content_sensitive"
	}
	if strings.Contains(lowerMessage, "sensitivecontentdetected") {
		// 兜底文本只匹配明确的敏感内容标记。
		return i18n.MsgTaskContentSensitive, "content_sensitive"
	}
	if isImagePixelConstraint(lowerMessage) {
		// 兜底文本同样识别图片像素不足。
		return i18n.MsgTaskImageResolutionTooLow, "image_resolution_too_low"
	}
	return "", ""
}

func isImagePixelConstraint(message string) bool {
	// 限定图片像素相关措辞，避免把所有 InvalidParameter 都误判成同类错误。
	if !strings.Contains(message, "pixel") {
		// 没有 pixel 关键词时不做任何图片分辨率推断。
		return false
	}
	return strings.Contains(message, "image pixel count") ||
		(strings.Contains(message, "image") &&
			strings.Contains(message, "greater than or equal"))
}
