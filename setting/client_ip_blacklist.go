package setting

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"unicode"
)

const ClientIPBlacklistOptionKey = "ClientIPBlacklist"

// ClientIPBlacklist stores the client source addresses that are not allowed
// to access the application. Entries may be individual IP addresses or CIDR
// networks. An empty list disables the check.
var ClientIPBlacklist = []string{}

var clientIPBlacklistMutex sync.RWMutex

// ParseClientIPBlacklist parses the value used by the admin option. The UI
// sends a multiline value, but accepting commas and semicolons makes the
// setting convenient to update through the API as well. Every non-empty
// entry must be an IP address or a valid CIDR network.
func ParseClientIPBlacklist(value string) ([]string, error) {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})

	entries := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		entry := strings.TrimSpace(field)
		if entry == "" {
			continue
		}

		if ip := net.ParseIP(entry); ip != nil {
			entry = ip.String()
		} else if _, network, err := net.ParseCIDR(entry); err == nil {
			entry = network.String()
		} else {
			return nil, fmt.Errorf("invalid client IP blacklist entry %q", entry)
		}

		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		entries = append(entries, entry)
	}

	return entries, nil
}

// NormalizeClientIPBlacklist validates a value and returns the canonical
// newline-delimited representation used for persistence and display.
func NormalizeClientIPBlacklist(value string) (string, error) {
	entries, err := ParseClientIPBlacklist(value)
	if err != nil {
		return "", err
	}
	return strings.Join(entries, "\n"), nil
}

// UpdateClientIPBlacklist validates and atomically replaces the current
// blacklist. The existing list is retained if validation fails.
func UpdateClientIPBlacklist(value string) error {
	entries, err := ParseClientIPBlacklist(value)
	if err != nil {
		return err
	}

	clientIPBlacklistMutex.Lock()
	ClientIPBlacklist = entries
	clientIPBlacklistMutex.Unlock()
	return nil
}

// ClientIPBlacklistToString returns the canonical multiline representation
// used by the options endpoint and the rate-limit settings page.
func ClientIPBlacklistToString() string {
	clientIPBlacklistMutex.RLock()
	defer clientIPBlacklistMutex.RUnlock()
	return strings.Join(ClientIPBlacklist, "\n")
}

// IsClientIPBlacklisted reports whether clientIP matches any configured
// address/network. Invalid request addresses never match; the middleware can
// still choose how to handle an unparsable address separately.
func IsClientIPBlacklisted(clientIP string) bool {
	clientIPBlacklistMutex.RLock()
	if len(ClientIPBlacklist) == 0 {
		clientIPBlacklistMutex.RUnlock()
		return false
	}
	defer clientIPBlacklistMutex.RUnlock()

	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false
	}
	for _, entry := range ClientIPBlacklist {
		if parsed := net.ParseIP(entry); parsed != nil {
			if ip.Equal(parsed) {
				return true
			}
			continue
		}
		if _, network, err := net.ParseCIDR(entry); err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}
