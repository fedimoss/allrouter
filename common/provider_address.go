package common

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

var ErrInvalidProviderAddress = errors.New("invalid address: use a domain or an HTTP IP address with a port from 1 to 65535")

// ProviderAddress separates the site identity from its optional URL scheme.
type ProviderAddress struct {
	Host   string
	Port   string
	Scheme string
	IsIP   bool
}

// Key preserves ports for IP sites while retaining the existing domain lookup rules.
func (a ProviderAddress) Key() string {
	if !a.IsIP {
		return a.Host
	}
	port := a.Port
	if port == "" {
		port = "80"
	}
	return net.JoinHostPort(a.Host, port)
}

// ParseProviderAddress accepts an authority or an HTTP(S) origin, never credentials,
// query parameters or application paths. Missing IP ports are accepted here for
// incoming HTTP requests on port 80; configuration validation requires a port.
func ParseProviderAddress(raw string) (ProviderAddress, error) {
	raw = strings.TrimSpace(raw)
	var address ProviderAddress
	if raw == "" {
		return address, ErrInvalidProviderAddress
	}
	if ip := net.ParseIP(raw); ip != nil {
		return ProviderAddress{Host: ip.String(), IsIP: true}, nil
	}
	value := raw
	if !strings.Contains(value, "://") {
		value = "//" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil ||
		(parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https") ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" ||
		strings.HasSuffix(parsed.Host, ":") {
		return address, ErrInvalidProviderAddress
	}
	address.Scheme = parsed.Scheme
	address.Host = strings.ToLower(parsed.Hostname())
	address.Port = parsed.Port()
	if address.Port != "" {
		for _, digit := range address.Port {
			if digit < '0' || digit > '9' {
				return ProviderAddress{}, ErrInvalidProviderAddress
			}
		}
		port, err := strconv.Atoi(address.Port)
		if err != nil || port < 1 || port > 65535 {
			return ProviderAddress{}, ErrInvalidProviderAddress
		}
		address.Port = strconv.Itoa(port)
	}
	if ip := net.ParseIP(address.Host); ip != nil {
		address.Host = ip.String()
		address.IsIP = true
		return address, nil
	}
	if strings.ContainsAny(parsed.Host, "[]") {
		return ProviderAddress{}, ErrInvalidProviderAddress
	}
	address.Host, err = idna.Lookup.ToASCII(strings.TrimSuffix(address.Host, "."))
	if err != nil || address.Host == "" || len(address.Host) > 253 {
		return ProviderAddress{}, ErrInvalidProviderAddress
	}
	labels := strings.Split(address.Host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return ProviderAddress{}, ErrInvalidProviderAddress
		}
		for _, ch := range label {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
				return ProviderAddress{}, ErrInvalidProviderAddress
			}
		}
	}
	// Numeric final labels are not DNS names and must not bypass IP validation.
	last := labels[len(labels)-1]
	if strings.Trim(last, "0123456789") == "" ||
		(strings.HasPrefix(last, "0x") && len(last) > 2 && strings.Trim(last[2:], "0123456789abcdef") == "") {
		return ProviderAddress{}, ErrInvalidProviderAddress
	}
	return address, nil
}

// IPRequestAddress uses the same HTTP port-80 identity as provider lookup.
func IPRequestAddress(r *http.Request) string {
	address, err := ParseProviderAddress(ProviderRequestHost(r))
	if err != nil || !address.IsIP {
		return ""
	}
	return address.Key()
}

// ProviderRequestHost returns the original site host. Forwarded hosts are only
// accepted from a private/loopback proxy peer and must contain one valid host;
// public callers cannot choose a tenant by spoofing X-Forwarded-Host.
func ProviderRequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	rawHost := r.Host
	if isTrustedInternalProxy(r.RemoteAddr) {
		forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
		if forwardedHost != "" && !strings.ContainsAny(forwardedHost, ",/@?#") && !strings.Contains(forwardedHost, "://") {
			if address, err := ParseProviderAddress(forwardedHost); err == nil {
				return address.Key()
			}
		}
	}
	address, err := ParseProviderAddress(rawHost)
	if err != nil {
		return ""
	}
	return address.Key()
}

func isTrustedInternalProxy(remoteAddr string) bool {
	peer, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		peer = strings.Trim(strings.TrimSpace(remoteAddr), "[]")
	}
	ip := net.ParseIP(peer)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast())
}

// SessionCookieName isolates IP origins by port. Domain sessions retain their
// existing cookie name so deployment does not invalidate domain logins.
func SessionCookieName(r *http.Request) string {
	address := IPRequestAddress(r)
	if address == "" {
		return "session"
	}
	digest := sha256.Sum256([]byte(address))
	return "session_ip_" + hex.EncodeToString(digest[:12])
}
