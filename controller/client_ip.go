package controller

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

func getRegistrationIP(c *gin.Context) string {
	if c == nil {
		return ""
	}
	ip := net.ParseIP(strings.TrimSpace(c.ClientIP()))
	if ip == nil {
		return ""
	}
	return ip.String()
}
