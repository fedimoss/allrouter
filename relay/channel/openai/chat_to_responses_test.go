package openai

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiChatToResponsesHandlerPreservesNonStreamingToolCalls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	toolCalls, err := common.Marshal([]dto.ToolCallRequest{
		{
			ID:   "call_image",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "responses_image_generation",
				Arguments: `{"prompt":"draw a stone"}`,
			},
		},
	})
	require.NoError(t, err)

	chatResponse := &dto.OpenAITextResponse{
		Id:      "chatcmpl_test",
		Object:  "chat.completion",
		Created: 123,
		Model:   "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:      "assistant",
					Content:   "",
					ToolCalls: toolCalls,
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: dto.Usage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}

	usage, relayErr := OaiChatToResponsesHandler(ctx, info, chatResponse)
	require.Nil(t, relayErr)
	require.NotNil(t, usage)

	var response dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Output, 1)
	require.Equal(t, "function_call", response.Output[0].Type)
	require.Equal(t, "call_image", response.Output[0].CallId)
	require.Equal(t, "responses_image_generation", response.Output[0].Name)
	require.Equal(t, `{"prompt":"draw a stone"}`, response.Output[0].ArgumentsString())
	require.Equal(t, 10, response.Usage.InputTokens)
	require.Equal(t, 4, response.Usage.OutputTokens)
}

func TestOaiChatToResponsesHandlerOrdersMessageBeforeToolCalls(t *testing.T) {
	toolCalls, err := common.Marshal([]dto.ToolCallRequest{
		{
			ID:   "call_lookup",
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      "lookup",
				Arguments: `{}`,
			},
		},
	})
	require.NoError(t, err)

	chatResponse := &dto.OpenAITextResponse{
		Id:      "chatcmpl_test",
		Created: 123,
		Model:   "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:      "assistant",
					Content:   "working",
					ToolCalls: toolCalls,
				},
			},
		},
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	_, relayErr := OaiChatToResponsesHandler(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}, chatResponse)
	require.Nil(t, relayErr)

	var response dto.OpenAIResponsesResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Output, 2)
	require.Equal(t, "message", response.Output[0].Type)
	require.Equal(t, "working", response.Output[0].Content[0].Text)
	require.Equal(t, "function_call", response.Output[1].Type)
}
