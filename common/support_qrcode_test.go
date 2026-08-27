package common

import (
	"reflect"
	"testing"
)

func TestParseSupportQRCodes(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  []SupportQRCode
	}{
		{"空串", "", nil},
		{"旧格式单URL", "/static/wechatQrcode/1/a.jpg", []SupportQRCode{{URL: "/static/wechatQrcode/1/a.jpg"}}},
		{"新格式数组", `[{"url":"/a.jpg","desc":"官方客服"},{"url":"/b.jpg","desc":"售后群"}]`,
			[]SupportQRCode{{URL: "/a.jpg", Desc: "官方客服"}, {URL: "/b.jpg", Desc: "售后群"}}},
		{"新格式含空项", `[{"url":"/a.jpg","desc":""},{"url":"","desc":""},{"desc":"仅描述"}]`,
			[]SupportQRCode{{URL: "/a.jpg"}, {Desc: "仅描述"}}},
		{"损坏JSON回退旧格式", "[not-json", []SupportQRCode{{URL: "[not-json"}}},
		{"超量截断", `[{"url":"1"},{"url":"2"},{"url":"3"},{"url":"4"},{"url":"5"}]`,
			[]SupportQRCode{{URL: "1"}, {URL: "2"}, {URL: "3"}, {URL: "4"}}},
		// 空数组：Normalize 后返回 nil（与空串一致语义）
		{"全空数组", `[]`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseSupportQRCodes(tc.value)
			if tc.want == nil {
				// 全空数组 Normalize 后为 nil
				if len(got) != 0 && got != nil {
					t.Fatalf("ParseSupportQRCodes(%q) = %v, want empty/nil", tc.value, got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseSupportQRCodes(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestParseSupportQRCodesLegacyDescDropped(t *testing.T) {
	// 旧格式（单 URL）：仅保留图片，描述字段已废弃
	got := ParseSupportQRCodes("/a.jpg")
	want := []SupportQRCode{{URL: "/a.jpg"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseSupportQRCodes legacy = %v, want %v", got, want)
	}
	// 空值
	if got := ParseSupportQRCodes(""); got != nil {
		t.Fatalf("ParseSupportQRCodes empty = %v, want nil", got)
	}
}

func TestEncodeSupportQRCodes(t *testing.T) {
	// 空列表 → 空串
	if got := EncodeSupportQRCodes(nil); got != "" {
		t.Fatalf("EncodeSupportQRCodes(nil) = %q, want empty", got)
	}
	// 正常编码并去除空项
	got := EncodeSupportQRCodes([]SupportQRCode{{URL: " /a.jpg "}, {Desc: " "}})
	want := `[{"url":"/a.jpg","desc":""}]`
	if got != want {
		t.Fatalf("EncodeSupportQRCodes = %q, want %q", got, want)
	}
	// 编码→解析 roundtrip
	parsed := ParseSupportQRCodes(got)
	if len(parsed) != 1 || parsed[0].URL != "/a.jpg" {
		t.Fatalf("roundtrip = %v", parsed)
	}
}
