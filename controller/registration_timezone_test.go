package controller

import "testing"

func TestNormalizeRegistrationTimezone(t *testing.T) {
	// 同时覆盖历史中国时区别名、大小写/空格容错，以及海外与非法输入的默认分支。
	tests := []struct {
		name     string
		timezone string
		want     string
	}{
		{name: "Shanghai", timezone: "Asia/Shanghai", want: registrationTimezoneChina},
		{name: "legacy China alias", timezone: "Asia/Chongqing", want: registrationTimezoneChina},
		{name: "trimmed case insensitive", timezone: " asia/urumqi ", want: registrationTimezoneChina},
		{name: "Hong Kong uses overseas profile", timezone: "Asia/Hong_Kong", want: registrationTimezoneOverseas},
		{name: "Tokyo", timezone: "Asia/Tokyo", want: registrationTimezoneOverseas},
		{name: "empty", timezone: "", want: registrationTimezoneOverseas},
		{name: "unknown", timezone: "invalid", want: registrationTimezoneOverseas},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeRegistrationTimezone(tt.timezone); got != tt.want {
				t.Fatalf("normalizeRegistrationTimezone(%q) = %q, want %q", tt.timezone, got, tt.want)
			}
		})
	}
}
