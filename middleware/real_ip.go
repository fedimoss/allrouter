package middleware

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// DefaultRealIPHeader is the single-value header that Nginx should set
	// after applying its real_ip_header/set_real_ip_from rules.
	DefaultRealIPHeader = "X-Real-IP"
	// ResolvedClientIPHeader is internal-only. Incoming values are always
	// removed before a validated address is written by ResolveRealIPHeader.
	ResolvedClientIPHeader = "X-AllRouter-Resolved-Client-IP"
)

// ResolveRealIPHeader makes Nginx's resolved client-IP header available to
// Gin's ClientIP() without hard-coding a Docker bridge address. To reduce
// direct-header spoofing, the source header is accepted only when the direct
// TCP peer is a private, loopback, or link-local address.
func ResolveRealIPHeader(sourceHeader string) gin.HandlerFunc {
	sourceHeader = strings.TrimSpace(sourceHeader)
	if sourceHeader == "" {
		sourceHeader = DefaultRealIPHeader
	}

	return func(c *gin.Context) {
		// A caller must never be able to provide our internal trusted header.
		c.Request.Header.Del(ResolvedClientIPHeader)

		if isInternalProxyPeer(c.RemoteIP()) {
			if ip := normalizeSingleIP(c.GetHeader(sourceHeader)); ip != "" {
				c.Request.Header.Set(ResolvedClientIPHeader, ip)
			}
		}

		c.Next()
	}
}

func isInternalProxyPeer(value string) bool {
	ip := net.ParseIP(strings.TrimSpace(value))
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast())
}

func normalizeSingleIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, ",") {
		return ""
	}

	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	} else {
		value = strings.TrimPrefix(value, "[")
		value = strings.TrimSuffix(value, "]")
	}

	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}
