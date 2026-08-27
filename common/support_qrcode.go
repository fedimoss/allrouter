package common

import (
	"strings"
)

// SupportQRCode 客服二维码条目：图片地址 + 独立描述。
// 三渠道（微信/Telegram/QQ）共用同一结构，每个渠道最多 SupportQRCodeMaxCount 张。
type SupportQRCode struct {
	URL  string `json:"url"`
	Desc string `json:"desc"`
}

// SupportQRCodeMaxCount 每个客服渠道支持的最大二维码数量
const SupportQRCodeMaxCount = 4

// ParseSupportQRCodes 解析客服二维码存储值。
// 存储格式演进：旧格式为单个 URL 纯文本（或空串），新格式为 [{"url":"...","desc":"..."}] JSON 数组。
// 兼容策略（按产品决策仅兼容图片，旧描述字段已废弃）：
//   - 以 '[' 开头 → 按新格式解析数组；解析失败回退旧格式处理
//   - 其他（含空串）→ 旧格式：非空时包装为单项 {url: 值}（desc 丢失）
func ParseSupportQRCodes(value string) []SupportQRCode {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []SupportQRCode
		if err := UnmarshalJsonStr(trimmed, &items); err == nil {
			return NormalizeSupportQRCodes(items)
		}
		// JSON 解析失败：视为损坏数据，按旧格式兜底
	}
	return []SupportQRCode{{URL: trimmed}}
}

// NormalizeSupportQRCodes 清洗二维码列表：去掉 url/desc 全空的项、去首尾空白、截断到最大数量。
func NormalizeSupportQRCodes(items []SupportQRCode) []SupportQRCode {
	result := make([]SupportQRCode, 0, len(items))
	for _, item := range items {
		url := strings.TrimSpace(item.URL)
		desc := strings.TrimSpace(item.Desc)
		if url == "" && desc == "" {
			continue
		}
		if len(result) >= SupportQRCodeMaxCount {
			break
		}
		result = append(result, SupportQRCode{URL: url, Desc: desc})
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// EncodeSupportQRCodes 将二维码列表序列化为存储值（JSON 数组）；空列表返回空串。
func EncodeSupportQRCodes(items []SupportQRCode) string {
	normalized := NormalizeSupportQRCodes(items)
	if len(normalized) == 0 {
		return ""
	}
	data, err := Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(data)
}
