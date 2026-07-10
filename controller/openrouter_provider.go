package controller

import "github.com/gin-gonic/gin"

// 本文件提供专门面向 OpenRouter "for-providers" 的模型列表接口。
//
// 背景：OpenRouter 接入第三方模型供应商时，要求供应商提供一个返回其平台所有可服务模型的接口
// (文档：https://openrouter.ai/docs/guides/community/for-providers 的 "List Models Endpoint")。
// 该接口的响应格式与站内 /v1/models(OpenAI 格式)不同，包含上下文长度、定价、模态、特性等扩展字段。
//
// 与 /v1/models 的关系：两者共用 TokenAuth，但
//   - GET /v1/models            返回该 token 可用的全部模型(OpenAI 格式)
//   - GET /v1/provider/models   只返回固定的 GLM5.2(OpenRouter provider 格式)
//
// 注意：下方 providerModels 中的数值均为占位值，待后续按 GLM5.2 的真实规格与定价替换。

// ---- OpenRouter for-providers 响应结构 ----

// openrouterPricing 描述单个计费档位的定价。
// OpenRouter 要求价格以"字符串"形式返回以避免浮点精度问题，单位为 USD。
type openrouterPricing struct {
	Prompt         string `json:"prompt"`           // 每 1 个 prompt(输入) token 的价格(USD 字符串)
	Completion     string `json:"completion"`       // 每 1 个 completion(输出) token 的价格(USD 字符串)
	Image          string `json:"image"`            // 每 1 张输入图片的价格(USD 字符串)，不单独计费填 "0"
	Request        string `json:"request"`          // 每次请求的固定价格(USD 字符串)，按次计费时填，否则填 "0"
	InputCacheRead string `json:"input_cache_read"` // 每 1 个"缓存命中的输入 token"的价格(USD 字符串)，无 prompt 缓存填 "0"
}

// openrouterSlug 为 OpenRouter 提供路由用的 slug 信息。
type openrouterSlug struct {
	Slug string `json:"slug"` // OpenRouter 上该端点的 slug，通常等于 "组织/模型名"，例如 "zhipu/glm-5.2"
}

// openrouterDatacenter 声明一个数据中心所在的国家/地区。
type openrouterDatacenter struct {
	CountryCode string `json:"country_code"` // ISO 3166-1 alpha-2 国家码，例如 "US"、"CN"
}

// openrouterModel 对应 OpenRouter for-providers 模型列表中的单个模型对象。
// 字段定义严格对应官方文档示例，每个字段均标注是否必填及取值说明。
type openrouterModel struct {
	// ---- 必填字段 ----
	Id                          string            `json:"id"`                            // 模型唯一标识；OpenRouter 调用你 API 时使用的精确 model 名(必须与站内实际可调用的 model 一致)
	HuggingFaceId               string            `json:"hugging_face_id"`               // 若模型托管在 Hugging Face 则必填其 HF id，否则留空字符串
	Name                        string            `json:"name"`                          // 展示名，通常格式为 "组织: 模型名"，例如 "Zhipu: GLM 5.2"
	Created                     int64             `json:"created"`                       // 模型发布/创建时间(Unix 秒)
	InputModalities             []string          `json:"input_modalities"`              // 支持的输入模态；可选值: text, image, file, audio, video
	OutputModalities            []string          `json:"output_modalities"`             // 支持的输出模态；可选值: text, image, embeddings, audio, video, rerank, speech, transcription
	Quantization                string            `json:"quantization"`                  // 量化精度；可选值: int4, int8, fp4, fp6, fp8, fp16, bf16, fp32
	ContextLength               int               `json:"context_length"`                // 最大上下文长度(输入+输出 token 上限)
	MaxOutputLength             int               `json:"max_output_length"`             // 单次最大输出 token 数
	Pricing                     openrouterPricing `json:"pricing"`                       // 定价信息(USD 字符串)，详见 openrouterPricing 各字段注释
	SupportedSamplingParameters []string          `json:"supported_sampling_parameters"` // 支持的采样参数；可选值: temperature, top_p, top_k, min_p, top_a, frequency_penalty, presence_penalty, repetition_penalty, stop, seed, max_tokens, logit_bias
	SupportedFeatures           []string          `json:"supported_features"`            // 支持的特性；可选值: tools, json_mode, structured_outputs, logprobs, web_search, reasoning

	// ---- 可选字段 ----
	Description     string                 `json:"description,omitempty"`      // 模型描述(展示用，可留空)
	DeprecationDate string                 `json:"deprecation_date,omitempty"` // 弃用时间；ISO 8601 格式(YYYY-MM-DD 或 YYYY-MM-DDTHH:00:00Z)，OpenRouter 到期会自动隐藏并提示
	IsReady         bool                   `json:"is_ready"`                   // false=在 OpenRouter 上保持"已暂存但隐藏"，用于提前上传或临时下线；true/缺省=正常自动上架
	IsFree          bool                   `json:"is_free"`                    // true=标记为免费端点(自动加 :free 后缀，任何 pricing 都被忽略)；false/缺省=标准付费
	DiscountToUser  float64                `json:"discount_to_user,omitempty"` // 用户侧定价折扣(小数)；0 或缺省=无折扣，0.2 表示用户看到的价格打 8 折，负值为加价
	CapacityTPM     int                    `json:"capacity_tpm,omitempty"`     // 该模型每分钟可处理的输入 token 容量；用于 OpenRouter 路由与容量规划，0 或缺省=未知
	OpenRouter      *openrouterSlug        `json:"openrouter,omitempty"`       // OpenRouter 路由元信息(slug)
	Datacenters     []openrouterDatacenter `json:"datacenters,omitempty"`      // 数据中心所在国家列表
}

// providerModels 为当前对外暴露给 OpenRouter 的模型集合(占位数据，待替换为 GLM5.2 真实规格)。
//
// TODO: 以下数值均为占位值，请按 GLM5.2 的实际上下文长度、定价、模态、特性等修正。
var providerModels = []openrouterModel{
	{
		Id:            "GLM5.2",   // ← OpenRouter 调用你 API 时使用的精确 model 名(需与站内可调用模型一致)
		HuggingFaceId: "",         // 未托管在 Hugging Face，留空
		Name:          "GLM 5.2",  // 展示名
		Created:       1690502400, // Unix 秒，待替换为真实发布时间

		InputModalities:  []string{"text"}, // 当前仅支持文本输入
		OutputModalities: []string{"text"}, // 当前仅支持文本输出

		Quantization:    "fp4",   // 占位量化精度
		ContextLength:   1000000, // 上下文长度
		MaxOutputLength: 131072,  // 最大输出长度

		Pricing: openrouterPricing{
			Prompt:         "0.0000014", // 每输入 token 价格(USD)
			Completion:     "0.0000044", // 每输出 token 价格(USD)
			Image:          "0",         // 不单独对图片计费
			Request:        "0",         // 不按次计费
			InputCacheRead: "0",         // 暂不支持 prompt 缓存
		},

		SupportedSamplingParameters: []string{"temperature", "top_p", "stop", "max_tokens", "reasoning", "tools", "tool_choice", "top_k", "response_format"}, // 支持的采样参数
		SupportedFeatures:           []string{"tools", "json_mode", "structured_outputs", "reasoning"},                                                       // 支持的特性

		// ---- 可选字段 ----
		IsReady:        true,  // 已就绪，允许 OpenRouter 自动上架
		IsFree:         false, // 标准付费端点
		DiscountToUser: 0,     // 无折扣(omitempty 会省略输出)
		CapacityTPM:    0,     // 容量未知(omitempty 会省略输出)
		OpenRouter: &openrouterSlug{
			Slug: "fedimoss/nvfp4", // slug
		},
		Datacenters: []openrouterDatacenter{
			{CountryCode: "CN"}, // 数据中心
		},
	},
}

// ListProviderModels 返回面向 OpenRouter for-providers 的模型列表。
//
// 路由: GET /v1/provider/models (与 /v1/models 共用 TokenAuth，但只返回固定的 GLM5.2)。
// 响应格式: { "data": [ ... ] }，每个元素结构详见 openrouterModel。
func ListProviderModels(c *gin.Context) {
	c.JSON(200, gin.H{
		"data": providerModels,
	})
}
