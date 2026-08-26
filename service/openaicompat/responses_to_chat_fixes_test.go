package openaicompat

// 针对 Responses→Chat 转换对齐 cc-switch 的修复项测试：
// - reasoning effort 钳制/关闭语义（mapResponsesReasoningEffort）
// - tools 为空时丢弃 parallel_tool_calls
// - user 回合边界的 pending reasoning 回溯附挂（不跨回合泄漏）
// - tool-call reasoning 去重
// - tool 输出媒体提取（planResponsesToolOutputMedia / flushPendingToolMedia 管线）

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestMapResponsesReasoningEffort(t *testing.T) {
	cases := map[string]string{
		"":          "",
		"high":      "high",
		"HIGH":      "high",
		" minimal ": "minimal",
		"low":       "low",
		"medium":    "medium",
		"xhigh":     "xhigh",
		"max":       "max",
		// 显式关闭语义：不透传（OpenAI 枚举不含 none，透传 400）
		"none":     "",
		"off":      "",
		"disabled": "",
		// 未知值丢弃
		"ultra": "",
	}
	for input, want := range cases {
		require.Equal(t, want, mapResponsesReasoningEffort(input), "input=%q", input)
	}
}

func TestResponsesChatCompatReasoningEffortPassthroughAndDrops(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"reasoning":{"effort":"high"},
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Equal(t, "high", chatReq.ReasoningEffort)
}

func TestResponsesChatCompatDropsExplicitReasoningDisable(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"reasoning":{"effort":"none"},
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Empty(t, chatReq.ReasoningEffort)
}

func TestResponsesChatCompatDropsUnknownReasoningEffort(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"reasoning":{"effort":"turbo-ultra"},
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Empty(t, chatReq.ReasoningEffort)
}

// tools 为空（无 tools 字段）时 parallel_tool_calls 必须被丢弃，
// 避免严格上游（vLLM 等）400/503。
func TestResponsesChatCompatDropsParallelToolCallsWithoutTools(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"parallel_tool_calls":true,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Nil(t, chatReq.ParallelTooCalls)
}

// [reasoning(1), assistant(1), user, reasoning(2), message(user 之后的下一个 assistant)]
// user 到达时 reasoning(2) 不得跨回合泄漏：应回溯附挂到 assistant(1)，
// 而不是留给 assistant(2)。
func TestResponsesChatCompatReasoningDoesNotLeakAcrossUserTurn(t *testing.T) {
	raw := []byte(`{
		"model":"kimi-k2-thinking",
		"input":[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"first thought"}]},
			{"type":"message","role":"assistant","content":"First answer."},
			{"type":"message","role":"user","content":"Continue"},
			{"type":"message","role":"assistant","content":"Second answer."}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	// user 之前的 assistant：不受影响
	require.Equal(t, "assistant", chatReq.Messages[0].Role)
	require.Equal(t, "First answer.", chatReq.Messages[0].StringContent())
	require.Equal(t, "first thought", chatReq.Messages[0].GetReasoningContent())
	require.Equal(t, "user", chatReq.Messages[1].Role)
	// 其后的 assistant：没有泄漏的 reasoning（回溯挂到了上一条或被丢弃，绝不前移）
	require.Equal(t, "assistant", chatReq.Messages[2].Role)
	require.Empty(t, chatReq.Messages[2].GetReasoningContent())
}

// [reasoning, function_call, output, reasoning, user]
// user 到达时，第二条 reasoning 应回溯附挂到 tool-call assistant 消息。
func TestResponsesChatCompatAttachesTrailingReasoningToToolCallAssistant(t *testing.T) {
	raw := []byte(`{
		"model":"kimi-k2-thinking",
		"input":[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"need to read a file"}]},
			{"type":"function_call","call_id":"call_1","name":"read_file","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"content"},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"done reading"}]},
			{"type":"message","role":"user","content":"thanks"}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	// assistant tool-call 消息同时承载前向与回溯的 reasoning
	require.Equal(t, "assistant", chatReq.Messages[0].Role)
	require.NotEmpty(t, chatReq.Messages[0].ToolCalls)
	rc := chatReq.Messages[0].GetReasoningContent()
	require.Contains(t, rc, "need to read a file")
	require.Contains(t, rc, "done reading")
	require.Equal(t, "tool", chatReq.Messages[1].Role)
	require.Equal(t, "user", chatReq.Messages[2].Role)
}

// tool-call 条目内嵌 reasoning 与相邻 summary reasoning 相同文本时去重，不重复拼接。
func TestResponsesChatCompatDedupesToolCallReasoning(t *testing.T) {
	raw := []byte(`{
		"model":"kimi-k2-thinking",
		"input":[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"inspect first"}]},
			{"type":"function_call","call_id":"call_1","name":"read","arguments":"{}","reasoning_content":"inspect first"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Equal(t, "inspect first", chatReq.Messages[0].GetReasoningContent())
}

// tool 输出内嵌媒体块：提取到合成 user 消息，tool 文本留替换标记。
func TestResponsesChatCompatExtractsToolOutputMedia(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"take a screenshot"}]},
			{"type":"function_call","call_id":"call_shot","name":"screenshot","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_shot","output":[
				{"type":"text","text":"captured"},
				{"type":"image_url","image_url":{"url":"data:image/png;base64,QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo="}}
			]},
			{"type":"message","role":"user","content":"what do you see"}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	// 消息序列：system? 无 → user, assistant(tool_calls), tool, user(媒体), user
	var toolMsg, mediaUser *dto.Message
	for i := range chatReq.Messages {
		m := &chatReq.Messages[i]
		switch {
		case m.Role == "tool" && m.ToolCallId == "call_shot":
			toolMsg = m
		case m.Role == "user" && m.Content != nil:
			if parts, ok := m.Content.([]dto.MediaContent); ok && len(parts) > 0 && parts[0].Type == dto.ContentTypeText && strings.Contains(parts[0].Text, "media output of tool call call_shot") {
				mediaUser = m
			}
		}
	}
	require.NotNil(t, toolMsg, "tool message must exist")
	require.Contains(t, toolMsg.StringContent(), "captured")
	require.Contains(t, toolMsg.StringContent(), "media moved to the following user message")
	require.NotContains(t, toolMsg.StringContent(), "QUJDREVGR0hJSktMTU5PUFFSU1RUFWVo")

	require.NotNil(t, mediaUser, "synthetic media user message must exist")
	parts := mediaUser.Content.([]dto.MediaContent)
	require.GreaterOrEqual(t, len(parts), 2)
	require.Equal(t, dto.ContentTypeText, parts[0].Type)
	foundImage := false
	for _, p := range parts {
		if p.Type == dto.ContentTypeImageURL {
			foundImage = true
			urlObj, ok := p.ImageUrl.(map[string]any)
			require.True(t, ok)
			url, _ := urlObj["url"].(string)
			require.True(t, strings.HasPrefix(url, "data:image/png;base64,"))
		}
	}
	require.True(t, foundImage, "image part must be present")
}

// 整串 data URL 输出（截图工具直接返回 data URL 字符串）也提取。
func TestResponsesChatCompatExtractsWholeStringDataURL(t *testing.T) {
	bigPayload := "data:image/png;base64," + strings.Repeat("QUJDREVGR0hJSktMTU5PUFFSU1RUFWVo", 400)
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"function_call","call_id":"call_img","name":"snap","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_img","output":` + mustJSONString(t, bigPayload) + `}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	require.Equal(t, "tool", chatReq.Messages[1].Role)
	require.Contains(t, chatReq.Messages[1].StringContent(), "media moved to the following user message")
	require.NotContains(t, chatReq.Messages[1].StringContent(), "QUJDREVGR0hJSktMTU5PUFFSU1RUFWVo")

	// 尾部收尾 flush：最后一条是合成 user 媒体消息
	last := chatReq.Messages[len(chatReq.Messages)-1]
	require.Equal(t, "user", last.Role)
	parts, ok := last.Content.([]dto.MediaContent)
	require.True(t, ok)
	require.NotEmpty(t, parts)
}

// 纯文本 tool 输出零改动：无媒体时不产生合成消息、内容原样保留。
func TestResponsesChatCompatKeepsPlainTextToolOutputUntouched(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":"run"},
			{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"plain text result with data: nothing suspicious"},
			{"type":"message","role":"user","content":"ok"}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	require.Len(t, chatReq.Messages, 4)
	require.Equal(t, "plain text result with data: nothing suspicious", chatReq.Messages[2].StringContent())
}

// Anthropic 形态图片块 {type:image, source:{data, media_type}} 提取为 data URL。
func TestResponsesChatCompatExtractsAnthropicShapedImage(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"function_call","call_id":"call_v","name":"view","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_v","output":[
				{"type":"text","text":"saw it"},
				{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"YWJjZGVm"}}
			]}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	last := chatReq.Messages[len(chatReq.Messages)-1]
	require.Equal(t, "user", last.Role)
	parts, ok := last.Content.([]dto.MediaContent)
	require.True(t, ok)
	found := false
	for _, p := range parts {
		if p.Type == dto.ContentTypeImageURL {
			if urlObj, ok := p.ImageUrl.(map[string]any); ok {
				if url, _ := urlObj["url"].(string); url == "data:image/jpeg;base64,YWJjZGVm" {
					found = true
				}
			}
		}
	}
	require.True(t, found, "anthropic image must map to data URL")
}

// 媒体提取后残留的裸 base64 长串被钳制为 omitted 占位。
func TestResponsesChatCompatClampsResidualBase64(t *testing.T) {
	bigBase64 := strings.Repeat("QUJDREVGR0hJSktMTU5PUFFSU1RUFWVo", 800) // >16KB 纯 base64
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"function_call","call_id":"call_m","name":"mixed","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_m","output":[
				{"type":"text","text":"partial view"},
				{"type":"image_url","image_url":{"url":"data:image/png;base64,QUJDREVGRw=="}},
				{"type":"text","text":` + mustJSONString(t, bigBase64) + `}
			]}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	toolContent := chatReq.Messages[1].StringContent()
	require.Contains(t, toolContent, "partial view")
	require.Contains(t, toolContent, "media moved to the following user message")
	require.Contains(t, toolContent, "omitted ")
	require.NotContains(t, toolContent, bigBase64[:100])
}

// 顶层 assistant content-part 前向消费 pending reasoning。
func TestResponsesChatCompatTopLevelAssistantContentPartConsumesReasoning(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking hard"}]},
			{"type":"output_text","role":"assistant","text":"Here is the answer"}
		]
	}`)
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	require.Equal(t, "assistant", chatReq.Messages[0].Role)
	require.Contains(t, chatReq.Messages[0].StringContent(), "Here is the answer")
	require.Equal(t, "thinking hard", chatReq.Messages[0].GetReasoningContent())
}

// mustJSONString 已在 responses_to_chat_request_test.go 定义，此处复用。
