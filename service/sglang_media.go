package service

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/system_setting"
)

// NormalizeSGLangFilePath strips container workspace prefixes from an SGLang
// file path so it can be served below the gateway's public /sglang/ route.
// e.g. "/sgl-workspace/sglang/media-hub/x.mp4" => "media-hub/x.mp4".
func NormalizeSGLangFilePath(filePath string) string {
	filePath = strings.TrimLeft(strings.TrimSpace(filePath), "/")
	filePath = strings.TrimPrefix(filePath, "sgl-workspace/sglang/")
	return strings.TrimPrefix(filePath, "sglang/")
}

// BuildSGLangMediaURL exposes an SGLang file path through the public /sglang/
// route, e.g. "https://your-server.com/sglang/media-hub/2026/08/28/x.mp4".
// publicBaseURL 通常为提交请求时记录的公开域名（Task.PrivateData.PublicBaseURL），
// 为空时回退到系统 ServerAddress。filePath 无法归一化时返回空串。
func BuildSGLangMediaURL(publicBaseURL, filePath string) string {
	filePath = NormalizeSGLangFilePath(filePath)
	if filePath == "" {
		return ""
	}
	publicBaseURL = strings.TrimSpace(publicBaseURL)
	if publicBaseURL == "" {
		publicBaseURL = strings.TrimSpace(system_setting.ServerAddress)
	}
	if publicBaseURL == "" {
		return ""
	}
	return strings.TrimRight(publicBaseURL, "/") + "/sglang/" + filePath
}
