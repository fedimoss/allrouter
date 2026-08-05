package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetResponsesImageArtifact serves a previously generated Responses image through a signed,
// short-lived capability URL. This endpoint intentionally does not require TokenAuth because
// Markdown image renderers do not attach the caller's API key; the HMAC signature is the access check.
func GetResponsesImageArtifact(c *gin.Context) {
	setting := operation_setting.GetResponsesImageGenerationSetting()
	artifact, err := service.OpenResponsesImageArtifact(
		c.Param("id"),
		c.Query("expires"),
		c.Query("format"),
		c.Query("sig"),
		setting.ArtifactDirectory,
	)
	if err != nil {
		if !errors.Is(err, service.ErrResponsesImageArtifactNotFound) &&
			!errors.Is(err, service.ErrResponsesImageArtifactExpired) &&
			!errors.Is(err, service.ErrResponsesImageArtifactAccess) {
			common.SysError("open responses image artifact failed: " + err.Error())
			c.Status(http.StatusInternalServerError)
			return
		}
		// Keep invalid signatures, expired artifacts, and unknown IDs indistinguishable.
		c.Status(http.StatusNotFound)
		return
	}
	defer artifact.File.Close()

	disposition := "inline"
	if c.Query("download") == "1" {
		// Markdown 预览与下载复用同一份签名工件；该参数只改变浏览器行为，
		// 不改变工件身份，也不扩大签名 URL 的访问范围。
		disposition = "attachment"
	}
	c.DataFromReader(http.StatusOK, artifact.Size, artifact.ContentType, artifact.File, map[string]string{
		"Cache-Control":          "private, max-age=60",
		"Content-Disposition":    disposition + "; filename=\"" + artifact.Filename + "\"",
		"X-Content-Type-Options": "nosniff",
	})
}
