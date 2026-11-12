package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func setRetryAfter(c *gin.Context, seconds int) {
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
}

func setRetryAfterForStatus(c *gin.Context, statusCode int) {
	switch statusCode {
	case http.StatusTooManyRequests:
		setRetryAfter(c, 1)
	case http.StatusServiceUnavailable:
		setRetryAfter(c, 5)
	}
}
