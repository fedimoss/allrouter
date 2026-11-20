package service

import "strings"

type SeedanceModelFamily string

const (
	SeedanceFamilyV1  SeedanceModelFamily = "1.0"
	SeedanceFamilyV2  SeedanceModelFamily = "2.0"
	SeedanceFamilyV25 SeedanceModelFamily = "2.5"
)

// SeedanceCapabilities is the single source of truth for the public panel and
// server-side request validation.
// 前端面板限制与后端校验共用这份能力矩阵，避免两套规则漂移。
// 修改任何数值前必须同步核对上游文档和前端提示文案。
type SeedanceCapabilities struct {
	Panel           string   `json:"panel"`
	Family          string   `json:"family"`
	MaxImages       int      `json:"max_images"`
	MaxVideos       int      `json:"max_videos"`
	MaxAudios       int      `json:"max_audios"`
	MaxVideoSeconds float64  `json:"max_video_seconds"`
	MaxAudioSeconds float64  `json:"max_audio_seconds"`
	AllowAudioOnly  bool     `json:"allow_audio_only"`
	MinDuration     int      `json:"min_duration"`
	MaxDuration     int      `json:"max_duration"`
	Resolutions     []string `json:"resolutions"`
	OutputFormats   []string `json:"output_formats,omitempty"`
}

func SeedanceFamilyForModel(modelName string) SeedanceModelFamily {
	// 只做版本归类；未知模型按 2.0 保守处理。
	// 宁可少放参数，也不要把 1.x/2.5 专属字段发给错误版本。
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	switch {
	case strings.Contains(normalized, "2-5") || strings.Contains(normalized, "2.5"):
		return SeedanceFamilyV25
	case strings.Contains(normalized, "1-0") || strings.Contains(normalized, "1.0") ||
		strings.Contains(normalized, "1-5") || strings.Contains(normalized, "1.5"):
		return SeedanceFamilyV1
	default:
		return SeedanceFamilyV2
	}
}

func IsSeedanceModelName(modelName string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "seedance")
}

func SeedanceCapabilitiesForModel(modelName string) SeedanceCapabilities {
	family := SeedanceFamilyForModel(modelName)
	switch family {
	case SeedanceFamilyV25:
		// 2.5：支持更多参考素材、更长素材时长和仅音频。
		return SeedanceCapabilities{
			Panel:           "seedance",
			Family:          string(family),
			MaxImages:       30,
			MaxVideos:       10,
			MaxAudios:       10,
			MaxVideoSeconds: 30,
			MaxAudioSeconds: 30,
			AllowAudioOnly:  true,
			MinDuration:     4,
			MaxDuration:     30,
			Resolutions:     []string{"480p", "720p", "1080p"},
			OutputFormats:   []string{"mp4", "mov"},
		}
	case SeedanceFamilyV1:
		// 1.x：参数和素材限制与 2.0 不同，不能复用 2.0 默认值。
		return SeedanceCapabilities{
			Panel:           "seedance",
			Family:          string(family),
			MaxImages:       9,
			MaxVideos:       3,
			MaxAudios:       3,
			MaxVideoSeconds: 15,
			MaxAudioSeconds: 15,
			AllowAudioOnly:  false,
			MinDuration:     2,
			MaxDuration:     12,
			Resolutions:     []string{"480p", "720p", "1080p"},
		}
	default:
		// 2.0：fast/mini 仅降低分辨率选项，其余限制与标准 2.0 相同。
		resolutions := []string{"480p", "720p", "1080p", "4k"}
		normalized := strings.ToLower(strings.TrimSpace(modelName))
		if strings.Contains(normalized, "fast") || strings.Contains(normalized, "mini") {
			resolutions = []string{"480p", "720p"}
		}
		return SeedanceCapabilities{
			Panel:           "seedance",
			Family:          string(SeedanceFamilyV2),
			MaxImages:       9,
			MaxVideos:       3,
			MaxAudios:       3,
			MaxVideoSeconds: 15,
			MaxAudioSeconds: 15,
			AllowAudioOnly:  false,
			MinDuration:     4,
			MaxDuration:     15,
			Resolutions:     resolutions,
		}
	}
}
