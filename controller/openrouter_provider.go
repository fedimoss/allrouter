package controller

import "github.com/gin-gonic/gin"

type providerPricing struct {
	Type    string `json:"type"`
	Unit    string `json:"unit"`
	CostUSD string `json:"cost_usd"`
}

type providerParameter struct {
	Type     string   `json:"type"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Unit     string   `json:"unit,omitempty"`
	MaxItems *int     `json:"max_items,omitempty"`
}

type providerLimit struct {
	Value int    `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

type providerInputModality struct {
	Type            string `json:"type"`
	SupportedInputs *struct {
		MaxContextLength providerLimit `json:"max_context_length"`
	} `json:"supported_inputs,omitempty"`
	Pricing []providerPricing `json:"pricing,omitempty"`
}

type providerOutputModality struct {
	Type                string                       `json:"type"`
	MaxLength           providerLimit                `json:"max_length,omitempty"`
	Streaming           bool                         `json:"streaming"`
	SupportedParameters map[string]providerParameter `json:"supported_parameters"`
	Pricing             []providerPricing            `json:"pricing,omitempty"`
}

type providerModel struct {
	SchemaVersion    string                   `json:"schema_version"`
	ID               string                   `json:"id"`
	HuggingFaceID    string                   `json:"hugging_face_id"`
	Name             string                   `json:"name"`
	Created          int64                    `json:"created"`
	Quantization     string                   `json:"quantization"`
	InputModalities  []providerInputModality  `json:"input_modalities"`
	OutputModalities []providerOutputModality `json:"output_modalities"`
	IsReady          bool                     `json:"is_ready"`
	IsFree           bool                     `json:"is_free"`
	OpenRouter       *struct {
		Slug string `json:"slug"`
	} `json:"openrouter,omitempty"`
	Datacenters []struct {
		CountryCode string `json:"country_code"`
	} `json:"datacenters,omitempty"`
}

var providerModels = []providerModel{{
	SchemaVersion: "2.4", ID: "GLM5.2", HuggingFaceID: "", Name: "GLM 5.2",
	Created: 1690502400, Quantization: "fp4", IsReady: true, IsFree: false,
	InputModalities: []providerInputModality{{
		Type: "text",
		SupportedInputs: &struct {
			MaxContextLength providerLimit `json:"max_context_length"`
		}{MaxContextLength: providerLimit{Value: 1000000, Unit: "token"}},
		Pricing: []providerPricing{{Type: "prompt", Unit: "token", CostUSD: "0.0000014"}},
	}},
	OutputModalities: []providerOutputModality{{
		Type: "text", MaxLength: providerLimit{Value: 131072, Unit: "token"}, Streaming: true,
		SupportedParameters: map[string]providerParameter{
			"temperature": {Type: "unknown"}, "top_p": {Type: "unknown"}, "stop": {Type: "array"},
			"max_tokens": {Type: "integer", Unit: "token"}, "reasoning": {Type: "boolean"},
			"tools": {Type: "boolean"}, "tool_choice": {Type: "unknown"}, "top_k": {Type: "unknown"},
			"response_format": {Type: "unknown"},
		},
		Pricing: []providerPricing{{Type: "completion", Unit: "token", CostUSD: "0.0000044"}},
	}},
	OpenRouter: &struct {
		Slug string `json:"slug"`
	}{Slug: "fedimoss/nvfp4"},
	Datacenters: []struct {
		CountryCode string `json:"country_code"`
	}{{CountryCode: "US"}},
}}

func ListProviderModels(c *gin.Context) {
	c.JSON(200, gin.H{"data": providerModels})
}
