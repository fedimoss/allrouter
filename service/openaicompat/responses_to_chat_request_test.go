package openaicompat

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesChatCompatPreservesStandardToolResultContinuation(t *testing.T) {
	longInstructions := strings.Repeat("system guidance ", 2000)
	longDeveloper := strings.Repeat("developer guidance ", 1000)

	raw := []byte(`{
		"model":"GLM5.1",
		"instructions":` + mustJSONString(t, longInstructions) + `,
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":` + mustJSONString(t, longDeveloper) + `}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"write a Java hello world file using Windows PowerShell"}]},
			{"type":"function_call","call_id":"call_test","name":"shell_command","arguments":"{\"command\":\"Set-Content -Path HelloWorld.java -Value 'public class HelloWorld {}' -Encoding UTF8\"}"},
			{"type":"function_call_output","call_id":"call_test","output":"Exit code: 0\nWall time: 1.5 seconds\nOutput:\n\n    Directory: C:\\Users\\15638\\Desktop\\1\n\nMode                 LastWriteTime         Length Name\n----                 -------------         ------ ----\n-a----          2026/6/9     18:27            123 HelloWorld.java"}
		],
		"tools":[
			{"type":"function","name":"shell_command","description":"run shell","parameters":{"type":"object","properties":{"command":{"type":"string"}}}}
		],
		"tool_choice":"auto",
		"parallel_tool_calls":true,
		"stream":true
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	require.Len(t, chatReq.Tools, 1)
	require.NotNil(t, chatReq.ToolChoice)
	require.NotNil(t, chatReq.ParallelTooCalls)
	require.True(t, *chatReq.ParallelTooCalls)
	// instructions 与 developer 两条 system 消息被合并到首位（cc-switch collapse 行为）
	require.Len(t, chatReq.Messages, 4)

	require.Equal(t, "system", chatReq.Messages[0].Role)
	require.Contains(t, chatReq.Messages[0].StringContent(), longInstructions[:80])
	require.Contains(t, chatReq.Messages[0].StringContent(), longDeveloper[:80])
	require.Equal(t, "user", chatReq.Messages[1].Role)
	require.Contains(t, chatReq.Messages[1].StringContent(), "Windows PowerShell")

	assistantMessage := chatReq.Messages[2]
	require.Equal(t, "assistant", assistantMessage.Role)
	require.NotEmpty(t, assistantMessage.ToolCalls)
	require.Contains(t, string(assistantMessage.ToolCalls), "call_test")
	require.Contains(t, string(assistantMessage.ToolCalls), "shell_command")

	toolMessage := chatReq.Messages[3]
	require.Equal(t, "tool", toolMessage.Role)
	require.Equal(t, "call_test", toolMessage.ToolCallId)
	require.Contains(t, toolMessage.StringContent(), "Command status: 0")
	require.NotContains(t, toolMessage.StringContent(), "Exit code:")
	require.Contains(t, toolMessage.StringContent(), "Wall time:")
	require.Contains(t, toolMessage.StringContent(), "HelloWorld.java")
}

func TestResponsesStandardConversionKeepsToolExitCode(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"run command"}]},
			{"type":"function_call","call_id":"call_test","name":"shell_command","arguments":"{\"command\":\"echo ok\"}"},
			{"type":"function_call_output","call_id":"call_test","output":"Exit code: 0\nWall time: 0.1 seconds\nOutput:\nok"}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, err := ResponsesRequestToChatCompletionsRequest(&req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 3)

	toolMessage := chatReq.Messages[2]
	require.Equal(t, "tool", toolMessage.Role)
	require.Contains(t, toolMessage.StringContent(), "Exit code: 0")
	require.NotContains(t, toolMessage.StringContent(), "Command status:")
}

func TestResponsesChatCompatPreservesPlanUpdateToolContinuation(t *testing.T) {
	raw := []byte(`{
		"model":"GLM5.1",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"create a weather.html file"}]},
			{"type":"function_call","call_id":"call_plan","name":"update_plan","arguments":"{\"plan\":[{\"step\":\"Create weather page\",\"status\":\"in_progress\"}]}"},
			{"type":"function_call_output","call_id":"call_plan","output":"Plan updated"}
		],
		"tools":[
			{"type":"function","name":"shell_command","description":"run shell","parameters":{"type":"object","properties":{"command":{"type":"string"}}}},
			{"type":"function","name":"update_plan","description":"update plan","parameters":{"type":"object","properties":{"plan":{"type":"array"}}}}
		],
		"tool_choice":"auto",
		"parallel_tool_calls":true,
		"stream":true
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)

	require.Len(t, chatReq.Tools, 2)
	require.NotNil(t, chatReq.ToolChoice)
	require.NotNil(t, chatReq.ParallelTooCalls)
	require.True(t, *chatReq.ParallelTooCalls)
	require.Len(t, chatReq.Messages, 3)

	require.Equal(t, "user", chatReq.Messages[0].Role)
	require.Contains(t, chatReq.Messages[0].StringContent(), "weather.html")
	require.Equal(t, "assistant", chatReq.Messages[1].Role)
	require.Contains(t, string(chatReq.Messages[1].ToolCalls), "call_plan")
	require.Contains(t, string(chatReq.Messages[1].ToolCalls), "update_plan")
	require.Equal(t, "tool", chatReq.Messages[2].Role)
	require.Equal(t, "call_plan", chatReq.Messages[2].ToolCallId)
	require.Equal(t, "Plan updated", chatReq.Messages[2].StringContent())
}

func TestResponsesChatCompatMapsInputImageToObjectURL(t *testing.T) {
	// 回归测试：Responses 的 input_image.image_url 是字符串，
	// 必须转换成 chat/completions 要求的 {"url": "..."} 对象，否则严格上游返回 400。
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[
				{"type":"input_text","text":"what is this?"},
				{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KG="}
			]}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 1)

	body, err := common.Marshal(chatReq.Messages[0].Content)
	require.NoError(t, err)
	bodyStr := string(body)
	require.Contains(t, bodyStr, `"image_url":{"url":"data:image/png;base64,iVBORw0KG="}`)
	require.NotContains(t, bodyStr, `"image_url":"data:`)
}

func TestResponsesChatCompatCollapsesTextOnlyContentToString(t *testing.T) {
	// 纯文本多段 content 应折叠成字符串，避免严格上游对 user 角色 string-only content 的校验失败。
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[
				{"type":"input_text","text":"line1"},
				{"type":"input_text","text":"line2"}
			]}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 1)
	require.Equal(t, "line1\nline2", chatReq.Messages[0].StringContent())
}

func TestResponsesChatCompatHandlesTopLevelContentParts(t *testing.T) {
	// 顶层（非 message 包裹）content-part 项不应被丢弃，按 cc-switch 各自包成一条 user 消息。
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"input_text","text":"describe this"},
			{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KG="}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 2)

	body0, err := common.Marshal(chatReq.Messages[0].Content)
	require.NoError(t, err)
	require.Contains(t, string(body0), "describe this")

	body1, err := common.Marshal(chatReq.Messages[1].Content)
	require.NoError(t, err)
	require.Contains(t, string(body1), `"image_url":{"url":"data:image/png;base64,iVBORw0KG="}`)
}

func TestResponsesChatCompatNormalizesToolParameters(t *testing.T) {
	// function 工具的 parameters 为 null 时必须规范化成 {"type":"object",...}，否则严格上游 400。
	raw := []byte(`{
		"model":"gpt-test",
		"tools":[{"type":"function","name":"lookup","parameters":null}],
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Len(t, chatReq.Tools, 1)

	body, err := common.Marshal(chatReq.Tools[0])
	require.NoError(t, err)
	require.Contains(t, string(body), `"type":"object"`)
}

func TestResponsesChatCompatAttachesReasoningToAssistant(t *testing.T) {
	// 回归测试：顶层 reasoning 条目应前向附挂到其后生成的 assistant 消息的 reasoning_content，
	// 对齐 cc-switch pending_reasoning 语义（思考型模型多轮历史需保留推理上下文）。
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
			{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking..."}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
	require.NoError(t, err)
	require.Len(t, chatReq.Messages, 2)

	assistant := chatReq.Messages[1]
	require.Equal(t, "assistant", assistant.Role)
	require.NotNil(t, assistant.ReasoningContent)
	require.Equal(t, "thinking...", *assistant.ReasoningContent)

	// 序列化后 reasoning_content 字段必须存在
	body, err := common.Marshal(assistant)
	require.NoError(t, err)
	require.Contains(t, string(body), `"reasoning_content":"thinking..."`)
}

func mustJSONString(t *testing.T, value string) string {
	t.Helper()

	data, err := common.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

// TestResponsesChatCompatCanonicalizesFunctionCallArguments 验证历史 function_call 的 arguments
// 不会被双重编码。Responses 的 arguments 是 JSON 字符串包 JSON（如 "\"{\"command\":\"ls\"}\""），
// 必须解包成干净的 JSON 对象文本，否则严格上游（Kimi-K3/Minimax）以 "arguments must be a JSON
// object" 400 拒绝。空 arguments 应规整为 "{}"。
func TestResponsesChatCompatCanonicalizesFunctionCallArguments(t *testing.T) {
	t.Run("string_encoded_object_args_unwrapped", func(t *testing.T) {
		raw := []byte(`{
			"model":"gpt-test",
			"tools":[{"type":"function","name":"shell","parameters":{"type":"object"}}],
			"input":[
				{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
				{"type":"function_call","call_id":"call_a","name":"shell","arguments":"{\"command\":\"ls -la\"}"}
			]
		}`)
		var req dto.OpenAIResponsesRequest
		require.NoError(t, common.Unmarshal(raw, &req))

		chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
		require.NoError(t, err)

		assistant := chatReq.Messages[1]
		require.Equal(t, "assistant", assistant.Role)
		body := string(assistant.ToolCalls)
		// arguments 必须是干净的 JSON 对象文本，且整体 tool_calls JSON 能解析
		require.Contains(t, body, `"arguments":"{\"command\":\"ls -la\"}"`)
		require.NotContains(t, body, `\"{\\`) // 不应出现双重编码的转义
		// 解析 tool_calls，确认 arguments 字段是合法 JSON 对象字符串
		var calls []map[string]any
		require.NoError(t, common.Unmarshal(assistant.ToolCalls, &calls))
		args, ok := calls[0]["function"].(map[string]any)["arguments"].(string)
		require.True(t, ok)
		var obj map[string]any
		require.NoError(t, common.Unmarshal([]byte(args), &obj))
		require.Equal(t, "ls -la", obj["command"])
	})

	t.Run("empty_arguments_becomes_object", func(t *testing.T) {
		raw := []byte(`{
			"model":"gpt-test",
			"tools":[{"type":"function","name":"noop","parameters":{"type":"object"}}],
			"input":[
				{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
				{"type":"function_call","call_id":"call_b","name":"noop","arguments":""}
			]
		}`)
		var req dto.OpenAIResponsesRequest
		require.NoError(t, common.Unmarshal(raw, &req))

		chatReq, err := ResponsesRequestToChatCompletionsCompatRequest(&req)
		require.NoError(t, err)

		assistant := chatReq.Messages[1]
		var calls []map[string]any
		require.NoError(t, common.Unmarshal(assistant.ToolCalls, &calls))
		args := calls[0]["function"].(map[string]any)["arguments"].(string)
		require.Equal(t, "{}", args) // 空 arguments 规整为 "{}"，避免严格上游 400
	})
}

// TestResponsesChatCompatConvertsCustomToolAndHistory 验证 custom 工具映射成带 input 参数的
// chat function 工具，且历史 custom_tool_call 还原成 {input:...} 参数的 tool_call。
func TestResponsesChatCompatConvertsCustomToolAndHistory(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"tools":[{"type":"custom","name":"apply_patch","instructions":"apply a patch"}],
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"patch it"}]},
			{"type":"custom_tool_call","call_id":"call_c1","name":"apply_patch","input":"*** patch ***"}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, ctx, err := ResponsesRequestToChatCompletionsCompatRequestWithContext(&req)
	require.NoError(t, err)

	// custom 工具注册为 chat function，名为原始名
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "apply_patch", chatReq.Tools[0].Function.Name)
	toolBody, _ := common.Marshal(chatReq.Tools[0])
	require.Contains(t, string(toolBody), `"input"`)
	require.Contains(t, string(toolBody), `"required":["input"]`)

	// 上下文记录 custom 映射
	spec := ctx.Lookup("apply_patch")
	require.NotNil(t, spec)
	require.Equal(t, "custom", string(spec.Kind))

	// 历史 custom_tool_call → assistant tool_call，arguments 为 {"input":"..."}
	assistant := chatReq.Messages[1]
	require.Equal(t, "assistant", assistant.Role)
	require.Contains(t, string(assistant.ToolCalls), `"name":"apply_patch"`)
	require.Contains(t, string(assistant.ToolCalls), `\"input\":\"*** patch ***\"`)
}

// TestResponsesChatCompatConvertsToolSearchAndNamespace 验证 tool_search 映射成固定 function 名，
// namespace 工具按 namespace__name 展开；历史 tool_search_call / namespace function_call 还原正确。
func TestResponsesChatCompatConvertsToolSearchAndNamespace(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"tools":[
			{"type":"tool_search"},
			{"type":"namespace","name":"mcp","tools":[
				{"type":"function","name":"search","parameters":{"type":"object"}}
			]}
		],
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
			{"type":"tool_search_call","call_id":"call_ts","arguments":{"query":"find"}},
			{"type":"function_call","call_id":"call_ns","name":"search","namespace":"mcp","arguments":"{}"}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, ctx, err := ResponsesRequestToChatCompletionsCompatRequestWithContext(&req)
	require.NoError(t, err)

	// tool_search → "tool_search"；namespace function → "mcp__search"
	require.Len(t, chatReq.Tools, 2)
	names := []string{chatReq.Tools[0].Function.Name, chatReq.Tools[1].Function.Name}
	require.Contains(t, names, "tool_search")
	require.Contains(t, names, "mcp__search")

	// 上下文映射
	tsSpec := ctx.Lookup("tool_search")
	require.NotNil(t, tsSpec)
	require.Equal(t, "tool_search", string(tsSpec.Kind))
	nsSpec := ctx.Lookup("mcp__search")
	require.NotNil(t, nsSpec)
	require.Equal(t, "namespace", string(nsSpec.Kind))
	require.Equal(t, "mcp", nsSpec.Namespace)
	require.Equal(t, "search", nsSpec.Name)

	// 历史 tool_search_call → function name "tool_search"
	tsMsg := chatReq.Messages[1]
	require.Equal(t, "assistant", tsMsg.Role)
	require.Contains(t, string(tsMsg.ToolCalls), `"name":"tool_search"`)

	// 历史 namespace function_call → function name "mcp__search"
	nsMsg := chatReq.Messages[2]
	require.Equal(t, "assistant", nsMsg.Role)
	require.Contains(t, string(nsMsg.ToolCalls), `"name":"mcp__search"`)
}

// TestResponsesChatCompatLoadsToolsFromToolSearchOutput 验证 input 中 tool_search_output
// 动态加载的 namespace 工具会被注册到上下文。
func TestResponsesChatCompatLoadsToolsFromToolSearchOutput(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
			{"type":"tool_search_output","tools":[
				{"type":"function","name":"dyn_tool","parameters":{"type":"object"}}
			]}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	_, ctx, err := ResponsesRequestToChatCompletionsCompatRequestWithContext(&req)
	require.NoError(t, err)

	spec := ctx.Lookup("dyn_tool")
	require.NotNil(t, spec)
	require.Equal(t, "function", string(spec.Kind))
}

// TestResponsesChatCompatCustomAndRawToolCallOutput 验证 custom_tool_call_output / tool_search_output
// 历史项被规范化为 tool 消息（content 为整条 JSON），关联 call_id。
func TestResponsesChatCompatCustomAndRawToolCallOutput(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},
			{"type":"custom_tool_call_output","call_id":"call_c1","output":"done"},
			{"type":"tool_search_output","call_id":"call_ts","tools":[]}
		]
	}`)

	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	chatReq, _, err := ResponsesRequestToChatCompletionsCompatRequestWithContext(&req)
	require.NoError(t, err)

	// 两条 tool 消息（注意 tool_search_output 也会被 collectToolSearchOutputTools 扫描，但无 tools 子项）
	require.GreaterOrEqual(t, len(chatReq.Messages), 3)
	customOut := chatReq.Messages[1]
	require.Equal(t, "tool", customOut.Role)
	require.Equal(t, "call_c1", customOut.ToolCallId)
	require.Contains(t, customOut.StringContent(), `"call_id":"call_c1"`)
	require.Contains(t, customOut.StringContent(), `"output":"done"`)
}
