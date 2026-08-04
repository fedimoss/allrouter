package service

import (
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/openaicompat"
)

func ChatCompletionsRequestToResponsesRequest(req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	return openaicompat.ChatCompletionsRequestToResponsesRequest(req)
}

// ResponsesRequestToChatCompletionsRequest 将 OpenAI Responses API 请求转换为 Chat Completions API 请求。
// 委托给 openaicompat 包执行实际转换逻辑。
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	return openaicompat.ResponsesRequestToChatCompletionsRequest(req)
}

// ResponsesRequestToChatCompletionsCompatRequest 将 Responses API 请求转换为 Chat Completions 兼容请求。
// 相比 ResponsesRequestToChatCompletionsRequest，额外对工具输出消息进行规范化处理，
// 以确保仅支持 Chat Completions 的上游供应商也能正常接收请求。
func ResponsesRequestToChatCompletionsCompatRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	return openaicompat.ResponsesRequestToChatCompletionsCompatRequest(req)
}

// ResponsesRequestToChatCompletionsCompatRequestWithContext 执行与
// ResponsesRequestToChatCompletionsCompatRequest 相同的兼容转换，并额外返回请求阶段构造的
// 工具上下文。Responses→Chat 渠道用该上下文在响应阶段把上游返回的 chat function_call
// 恢复成 function_call / custom_tool_call / tool_search_call / namespace function_call。
func ResponsesRequestToChatCompletionsCompatRequestWithContext(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, *relaycommon.ResponsesChatToolContext, error) {
	return openaicompat.ResponsesRequestToChatCompletionsCompatRequestWithContext(req)
}

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	return openaicompat.ResponsesResponseToChatCompletionsResponse(resp, id)
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	return openaicompat.ExtractOutputTextFromResponses(resp)
}
