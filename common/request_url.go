package common

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetTrustedRequestBaseURL returns the current request origin when it is safe for redirects.
// If the request origin is not trusted, it falls back to fallbackURL.
func GetTrustedRequestBaseURL(c *gin.Context, fallbackURL string) string {
	return GetTrustedRequestBaseURLWithDomains(c, fallbackURL, nil)
}

// GetTrustedRequestBaseURLWithDomains returns the current request origin when it is safe
// for redirects against the provided trusted domains.
func GetTrustedRequestBaseURLWithDomains(c *gin.Context, fallbackURL string, trustedDomains []string) string {
	for _, baseURL := range GetRequestBaseURLCandidates(c) {
		if ValidateRedirectURLWithDomains(baseURL+"/console/topup", trustedDomains) == nil {
			return baseURL
		}
	}
	return strings.TrimRight(fallbackURL, "/")
}

// ValidateRedirectURLWithDomains validates that a redirect URL is safe to use
// against the provided trusted domains. If trustedDomains is nil, it falls back
// to the global redirect-domain whitelist.
func ValidateRedirectURLWithDomains(rawURL string, trustedDomains []string) error {
	if trustedDomains == nil {
		return ValidateRedirectURL(rawURL)
	}
	parsedTrustedDomains := normalizeTrustedDomains(trustedDomains)

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %s", err.Error())
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: only http and https are allowed")
	}

	domain := strings.ToLower(parsedURL.Hostname())
	for _, trustedDomain := range parsedTrustedDomains {
		if domain == trustedDomain || strings.HasSuffix(domain, "."+trustedDomain) {
			return nil
		}
	}
	return fmt.Errorf("domain %s is not in the trusted domains list", domain)
}

// GetRequestBaseURLCandidates returns deduplicated base URL candidates (scheme://host)
// derived from the incoming request headers.
func GetRequestBaseURLCandidates(c *gin.Context) []string {
	if c == nil || c.Request == nil {
		return nil
	}

	scheme := getRequestScheme(c)
	hosts := getRequestHosts(c)
	candidates := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		baseURL := scheme + "://" + host
		if _, ok := seen[baseURL]; ok {
			continue
		}
		seen[baseURL] = struct{}{}
		candidates = append(candidates, baseURL)
	}
	return candidates
}

func normalizeTrustedDomains(domains []string) []string {
	normalized := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		normalized = append(normalized, domain)
	}
	return normalized
}

func getRequestScheme(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := c.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
		proto = strings.ToLower(strings.TrimSpace(strings.Split(proto, ",")[0]))
		if proto == "http" || proto == "https" {
			scheme = proto
		}
	}
	return scheme
}

func getRequestHosts(c *gin.Context) []string {
	hosts := make([]string, 0, 2)
	if c.Request.Host != "" {
		hosts = append(hosts, c.Request.Host)
	}
	if forwardedHost := firstForwardedHeaderValue(c.Request.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		hosts = append(hosts, forwardedHost)
	}
	return hosts
}

func firstForwardedHeaderValue(value string) string {
	value = strings.TrimSpace(strings.Split(value, ",")[0])
	if value == "" {
		return ""
	}
	if host, port, err := net.SplitHostPort(value); err == nil {
		return net.JoinHostPort(strings.TrimSpace(host), strings.TrimSpace(port))
	}
	return value
}
