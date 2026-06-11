package responseschat

import (
	"fmt"

	"github.com/QuantumNous/new-api/dto"
	openaichannel "github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"

	"github.com/gin-gonic/gin"
)

// Adaptor 继承 openai.Adaptor，使所有标准端点
// （chat/completions、embeddings、audio 等）开箱即用。
// 仅重写 ConvertOpenAIResponsesRequest，将 Responses 请求转换为 Chat Completions 请求。
type Adaptor struct {
	openaichannel.Adaptor // 嵌入 OpenAI 适配器，复用其全部标准功能
}

// Init 初始化适配器，委托给嵌入的 OpenAI 适配器完成初始化。
func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.Adaptor.Init(info)
}

// ConvertOpenAIResponsesRequest 将 OpenAI Responses API 请求转换为 Chat Completions 请求，
// 然后委托给 OpenAI 适配器的 ConvertOpenAIRequest 方法处理。
// 这使得仅支持 /v1/chat/completions 的上游供应商也能接收源自 /v1/responses 的请求。
func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// 将 Responses 请求转换为 Chat Completions 兼容请求
	chatReq, err := openaicompat.ResponsesRequestToChatCompletionsCompatRequest(&request)
	if err != nil {
		return nil, fmt.Errorf("failed to convert responses request to chat completions: %w", err)
	}

	// 委托给 OpenAI 适配器处理转换后的 Chat Completions 请求
	return a.Adaptor.ConvertOpenAIRequest(c, info, chatReq)
}

// GetChannelName 返回通道名称标识。
func (a *Adaptor) GetChannelName() string {
	return "Responses→Chat"
}
