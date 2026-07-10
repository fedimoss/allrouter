package doubao

import "strings"

var ModelList = []string{
	"doubao-seedance-1-0-pro-250528",
	"doubao-seedance-1-0-lite-t2v",
	"doubao-seedance-1-0-lite-i2v",
	"doubao-seedance-1-5-pro-251215",
	"doubao-seedance-2-0-260128",
	"doubao-seedance-2-0-fast-260128",
}

var ChannelName = "doubao-video"

type seedanceBillingPrice struct {
	Text2Video720p  float64
	Image2Video720p float64
	Text2Video4K    float64
	Image2Video4K   float64
}

var seedanceBillingPrices = map[string]seedanceBillingPrice{
	"doubao-seedance-2-0-260128": {
		Text2Video720p:  46,
		Image2Video720p: 28,
		Text2Video4K:    196,
		Image2Video4K:   119,
	},
	"doubao-seedance-2-0-fast-260128": {
		Text2Video720p:  37,
		Image2Video720p: 22,
		Text2Video4K:    158,
		Image2Video4K:   95,
	},
}

func GetVideoInputRatio(modelName string, resolution string, hasVideoInput bool) (float64, bool) {
	price, ok := seedanceBillingPrices[modelName]
	if !ok {
		return 0, false
	}

	base := price.Text2Video720p
	target := price.Text2Video720p
	if is4KResolution(resolution) {
		target = price.Text2Video4K
	}
	if hasVideoInput {
		target = price.Image2Video720p
		if is4KResolution(resolution) {
			target = price.Image2Video4K
		}
	}
	if base <= 0 || target <= 0 {
		return 0, false
	}
	return target / base, true
}

func is4KResolution(resolution string) bool {
	normalized := strings.ToLower(strings.TrimSpace(resolution))
	return normalized == "4k" || normalized == "2160p" || normalized == "3840x2160"
}
