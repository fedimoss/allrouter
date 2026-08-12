package operation_setting

import (
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// ResponsesImageGenerationSetting 控制 Responses API 的本地生图工具桥接。
// 对外仍显示并计费用户请求的文本模型，真正的图片生成则通过现有
// /v1/images/generations 适配器转交给 ImageModel 对应的独立图片渠道。
type ResponsesImageGenerationSetting struct {
	Enabled                  bool     `json:"enabled"`
	PlannerModelPatterns     []string `json:"planner_model_patterns"`
	ImageModel               string   `json:"image_model"`
	DefaultSize              string   `json:"default_size"`
	DefaultQuality           string   `json:"default_quality"`
	MaxCalls                 int      `json:"max_calls"`
	AutoInjectTool           bool     `json:"auto_inject_tool"`
	AutoInjectPromptPatterns []string `json:"auto_inject_prompt_patterns"`
	// ImplicitExecutionMode 控制未显式声明 image_generation 时的执行路径：
	// auto 在客户端暴露命令工具时使用零持久化 client_stream，否则走网关图片渠道；
	// gateway 始终走网关图片渠道；client 始终交回普通 Responses 工具流程；
	// client_stream 让客户端通过一次性执行票据下载并保存图片，无命令工具时回退 gateway。
	ImplicitExecutionMode string `json:"implicit_execution_mode"`
	// ClientStreamTicketTTLSeconds 是零持久化执行票据的有效期。票据只允许消费一次，
	// 非正值使用默认值，过大的值会被服务层限制在安全上限内。
	ClientStreamTicketTTLSeconds int `json:"client_stream_ticket_ttl_seconds"`
	// ArtifactDelivery 是否启用网关工件交付。启用后会在普通 Responses message
	// 中追加 Markdown 图片和下载链接。
	ArtifactDelivery bool `json:"artifact_delivery"`
	// ArtifactDeliveryMode 控制结果形态：auto 对 Codex 使用 artifact、对其他
	// 客户端使用 hybrid；hybrid 保留 Base64 并附链接；artifact 只返回工件链接；
	// base64 保留旧行为。auto 让客户端无需修改源码，同时避免 Codex 会话膨胀。
	ArtifactDeliveryMode string `json:"artifact_delivery_mode"`
	// ArtifactDirectory 是网关本地图片工件目录。目录只由签名下载接口读取，
	// 不挂到公开 static 路径，避免生成图片被无鉴权枚举。
	ArtifactDirectory string `json:"artifact_directory"`
	// ArtifactTTLMinutes 控制签名链接及本地工件的有效期；非正值使用默认值。
	ArtifactTTLMinutes int `json:"artifact_ttl_minutes"`
}

var responsesImageGenerationSetting = ResponsesImageGenerationSetting{
	Enabled:                      true,
	PlannerModelPatterns:         []string{`^gpt-.*`},
	ImageModel:                   "gpt-image-2",
	DefaultSize:                  "1024x1024",
	DefaultQuality:               "high",
	MaxCalls:                     1,
	AutoInjectTool:               true,
	ImplicitExecutionMode:        "auto",
	ClientStreamTicketTTLSeconds: 300,
	ArtifactDelivery:             false,
	ArtifactDeliveryMode:         "auto",
	ArtifactDirectory:            "data/responses-images",
	ArtifactTTLMinutes:           1440,
	AutoInjectPromptPatterns: []string{
		`(?i)^\s*(please\s+)?(generate|create|draw|render|illustrate|design|make|paint|sketch)\b[\s\S]{0,160}\b(image|picture|photo|illustration|artwork|poster|logo|icon|graphic)\b`,
		`(?i)^\s*(can|could|would|will)\s+you\b[\s\S]{0,40}\b(generate|create|draw|render|illustrate|design|make|paint|sketch)\b[\s\S]{0,160}\b(image|picture|photo|illustration|artwork|poster|logo|icon|graphic)\b`,
		`(?i)^\s*(i\s+(want|need|would\s+like)|give\s+me|make\s+me)\b[\s\S]{0,160}\b(image|picture|photo|illustration|artwork|poster|logo|icon|graphic)\b`,
		`^\s*(可以|能|能否|可否)?\s*(请帮我|麻烦帮我|请|请你|麻烦|麻烦你|帮我|给我|为我|替我)\s*(生成|画|绘制|创作|制作|设计).{0,80}(图|图片|图像|插画|海报|照片|徽标|图标)`,
		`^\s*(生成|画|绘制|创作|制作|设计)\s*(一|一个|个|一张|张|一幅|幅|一套|一些|几张|几幅|这张|那个|这幅).{0,80}(图|图片|图像|插画|海报|照片|徽标|图标)`,
		`^\s*(我要|我想要|我需要|想要|来一张|给我来).{0,80}(图|图片|图像|插画|海报|照片|徽标|图标)`,
		`^\s*(生图|文生图)\s*[:：]`,
	},
}

func init() {
	config.GlobalConfig.Register("responses_image_generation_setting", &responsesImageGenerationSetting)
}

// 获取生图模型的默认配置
func GetResponsesImageGenerationSetting() *ResponsesImageGenerationSetting {
	return &responsesImageGenerationSetting
}

// 检测模型是否是gpt开头的模型
func ShouldBridgeResponsesImageGeneration(modelName string) bool {
	setting := GetResponsesImageGenerationSetting()
	if setting == nil || !setting.Enabled || strings.TrimSpace(setting.ImageModel) == "" {
		return false
	}
	if len(setting.PlannerModelPatterns) == 0 {
		return true
	}
	for _, pattern := range setting.PlannerModelPatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		matched, err := regexp.MatchString(pattern, modelName)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// ShouldAutoInjectResponsesImageGeneration 判断一段自然语言请求是否明确要求生图。
// 即使客户端没有声明 image_generation 工具，匹配后也可由后端注入内部图片工具。
// 规则保持可配置，避免普通 Responses 请求全部进入需要缓冲的非流式规划流程。
func ShouldAutoInjectResponsesImageGeneration(prompt string) bool {
	setting := GetResponsesImageGenerationSetting()
	if setting == nil || !setting.Enabled || !setting.AutoInjectTool {
		return false
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return false
	}
	for _, pattern := range setting.AutoInjectPromptPatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		matched, err := regexp.MatchString(pattern, prompt)
		if err == nil && matched {
			return true
		}
	}
	return false
}
