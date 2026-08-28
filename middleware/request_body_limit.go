package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func AnonymousRequestBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		limitRequestBody(c, common.GetAnonymousRequestBodyLimitBytes())
	}
}

// RequestBodyLimit 在请求体被缓冲/multipart 解析之前按固定上限拒绝超大 body。
// 与 AnonymousRequestBodyLimit 的动态配置不同，上限在路由注册时给定，
// 适用于大小天花板已知的场景（如问卷截图上传，防止先整包落盘后才发现超限）。
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		limitRequestBody(c, maxBytes)
	}
}

func limitRequestBody(c *gin.Context, maxBytes int64) {
	if maxBytes <= 0 || c.Request.Body == nil {
		c.Next()
		return
	}

	originalBody := c.Request.Body
	limitedBody, err := readAnonymousRequestBody(originalBody, maxBytes)
	_ = originalBody.Close()
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(limitedBody))
	c.Request.ContentLength = int64(len(limitedBody))
	c.Next()
}

func readAnonymousRequestBody(body io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, common.ErrRequestBodyTooLarge
	}
	return data, nil
}
