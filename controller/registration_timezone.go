package controller

import "strings"

const (
	registrationTimezoneChina    = "Asia/Shanghai"
	registrationTimezoneOverseas = "America/New_York"
)

var mainlandChinaTimezones = map[string]struct{}{
	"asia/chongqing": {},
	"asia/harbin":    {},
	"asia/kashgar":   {},
	"asia/shanghai":  {},
	"asia/urumqi":    {},
	"prc":            {},
}

// normalizeRegistrationTimezone restricts self-registration to the two
// timezone profiles selected by the frontend's browser-language rule.
func normalizeRegistrationTimezone(timezone string) string {
	normalized := strings.ToLower(strings.TrimSpace(timezone))
	if _, ok := mainlandChinaTimezones[normalized]; ok {
		return registrationTimezoneChina
	}
	return registrationTimezoneOverseas
}
