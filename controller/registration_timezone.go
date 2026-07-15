package controller

import "strings"

const (
	registrationTimezoneChina    = "Asia/Shanghai"
	registrationTimezoneOverseas = "America/New_York"
)

// 兼容历史客户端可能直接提交的中国大陆 IANA 时区别名，最终统一收敛为上海时区。
var mainlandChinaTimezones = map[string]struct{}{
	"asia/chongqing": {},
	"asia/harbin":    {},
	"asia/kashgar":   {},
	"asia/shanghai":  {},
	"asia/urumqi":    {},
	"prc":            {},
}

// normalizeRegistrationTimezone 将自助注册请求限制为系统支持的两个时区档位。
// 前端按浏览器语言提交推荐值；空值、未知值及非中国大陆时区统一回退到纽约时区。
func normalizeRegistrationTimezone(timezone string) string {
	normalized := strings.ToLower(strings.TrimSpace(timezone))
	if _, ok := mainlandChinaTimezones[normalized]; ok {
		return registrationTimezoneChina
	}
	return registrationTimezoneOverseas
}
