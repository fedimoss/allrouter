package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

// TestChatStreamToResponsesResponseIncludesReasoningItem 验证非流式 Responses 响应
// 在带推理内容时，会在 message 之前输出 reasoning 条目（对齐 cc-switch）。
func TestChatStreamToResponsesResponseIncludesReasoningItem(t *testing.T) {
	resp := chatStreamToResponsesResponse("resp_123", 1700000000, "gpt-test", "hello", &dto.Usage{}, true, 1, nil, "think...", 0)

	body, err := common.Marshal(resp)
	require.NoError(t, err)
	bodyStr := string(body)

	// reasoning 条目位于 message 之前
	require.Contains(t, bodyStr, `"type":"reasoning"`)
	require.Contains(t, bodyStr, `"id":"rs_resp_123"`)
	require.Contains(t, bodyStr, `"summary":[{"type":"summary_text","text":"think..."}]`)
	require.Contains(t, bodyStr, `"type":"message"`)
}

// TestChatStreamToResponsesResponseOmitsReasoningWhenEmpty 验证无推理内容时不输出 reasoning 条目。
func TestChatStreamToResponsesResponseOmitsReasoningWhenEmpty(t *testing.T) {
	resp := chatStreamToResponsesResponse("resp_123", 1700000000, "gpt-test", "hello", &dto.Usage{}, true, 0, nil, "", -1)

	body, err := common.Marshal(resp)
	require.NoError(t, err)
	require.NotContains(t, string(body), `"type":"reasoning"`)
}

// TestResponsesToolItemIDPrefix 验证 item ID 前缀：custom → ctc_，其余 → fc_。
func TestResponsesToolItemIDPrefix(t *testing.T) {
	require.Equal(t, "ctc_call_1", responsesToolItemID("call_1", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindCustom}))
	require.Equal(t, "fc_call_1", responsesToolItemID("call_1", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindFunction}))
	require.Equal(t, "fc_call_1", responsesToolItemID("call_1", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindToolSearch}))
	require.Equal(t, "fc_call_1", responsesToolItemID("call_1", nil))
}

// TestResponsesToolItemFromChatRestoresTypes 验证按 spec 还原四种工具条目类型（对齐 cc-switch）。
func TestResponsesToolItemFromChatRestoresTypes(t *testing.T) {
	t.Run("function", func(t *testing.T) {
		item := responsesToolItemFromChat("fc_c1", "completed", "c1", "shell", `{"a":1}`, "", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindFunction, Name: "shell"})
		body, _ := common.Marshal(item)
		s := string(body)
		require.Contains(t, s, `"type":"function_call"`)
		require.Contains(t, s, `"id":"fc_c1"`)
		require.Contains(t, s, `"name":"shell"`)
		require.Contains(t, s, `"arguments":"{\"a\":1}"`)
		require.NotContains(t, s, `"namespace"`)
	})

	t.Run("namespace", func(t *testing.T) {
		item := responsesToolItemFromChat("fc_c1", "completed", "c1", "mcp__search", `{}`, "", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindNamespace, Name: "search", Namespace: "mcp"})
		body, _ := common.Marshal(item)
		s := string(body)
		require.Contains(t, s, `"type":"function_call"`)
		require.Contains(t, s, `"name":"search"`)
		require.Contains(t, s, `"namespace":"mcp"`)
	})

	t.Run("custom", func(t *testing.T) {
		// chat arguments 为 {"input":"patch text"}，解包成 input 字段
		item := responsesToolItemFromChat("ctc_c1", "completed", "c1", "apply_patch", `{"input":"patch text"}`, "", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindCustom, Name: "apply_patch"})
		body, _ := common.Marshal(item)
		s := string(body)
		require.Contains(t, s, `"type":"custom_tool_call"`)
		require.Contains(t, s, `"id":"ctc_c1"`)
		require.Contains(t, s, `"name":"apply_patch"`)
		require.Contains(t, s, `"input":"patch text"`)
		require.NotContains(t, s, `"arguments"`)
	})

	t.Run("tool_search", func(t *testing.T) {
		// arguments 解析为对象；无 id/name；execution="client"
		item := responsesToolItemFromChat("fc_c1", "completed", "c1", "tool_search", `{"query":"find"}`, "", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindToolSearch})
		body, _ := common.Marshal(item)
		s := string(body)
		require.Contains(t, s, `"type":"tool_search_call"`)
		require.Contains(t, s, `"execution":"client"`)
		require.Contains(t, s, `"arguments":{"query":"find"}`)
		require.NotContains(t, s, `"id":`)
		require.NotContains(t, s, `"name":`)
	})

	t.Run("tool_search_non_object_args_wraps_query", func(t *testing.T) {
		// 非对象 JSON 参数包装成 {query: arguments}
		item := responsesToolItemFromChat("fc_c1", "completed", "c1", "tool_search", `rawtext`, "", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindToolSearch})
		body, _ := common.Marshal(item)
		require.Contains(t, string(body), `"query":"rawtext"`)
	})

	t.Run("unknown_falls_back_to_function_call", func(t *testing.T) {
		// 无 spec：回退为普通 function_call，用 chat 名
		item := responsesToolItemFromChat("fc_c1", "completed", "c1", "mystery", `{}`, "", nil)
		body, _ := common.Marshal(item)
		s := string(body)
		require.Contains(t, s, `"type":"function_call"`)
		require.Contains(t, s, `"name":"mystery"`)
	})
}

// TestResponsesToolItemFromChatAttachesReasoning 验证 reasoning 非空时附挂 reasoning_content。
func TestResponsesToolItemFromChatAttachesReasoning(t *testing.T) {
	item := responsesToolItemFromChat("fc_c1", "completed", "c1", "shell", `{}`, "think hard", &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindFunction, Name: "shell"})
	body, _ := common.Marshal(item)
	require.Contains(t, string(body), `"reasoning_content":"think hard"`)
}

// TestChatStreamToResponsesResponseRestoresToolTypes 验证最终 response.completed 负载里
// 工具条目按 spec 还原类型（custom/tool_search/namespace）。
func TestChatStreamToResponsesResponseRestoresToolTypes(t *testing.T) {
	toolCalls := map[int]*responsesToolCallState{
		0: {
			ID: "ctc_c1", CallID: "c1", Name: "apply_patch", Arguments: `{"input":"x"}`,
			OutputIndex: 1,
			Spec:        &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindCustom, Name: "apply_patch"},
		},
		1: {
			ID: "fc_c2", CallID: "c2", Name: "tool_search", Arguments: `{"query":"q"}`,
			OutputIndex: 2,
			Spec:        &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindToolSearch},
		},
	}
	resp := chatStreamToResponsesResponse("resp_1", 1700000000, "gpt-test", "hi", &dto.Usage{}, true, 0, toolCalls, "", -1)
	body, _ := common.Marshal(resp)
	s := string(body)
	require.Contains(t, s, `"type":"custom_tool_call"`)
	require.Contains(t, s, `"type":"tool_search_call"`)
	require.Contains(t, s, `"input":"x"`)
	require.Contains(t, s, `"execution":"client"`)
}
