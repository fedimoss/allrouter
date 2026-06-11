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
	require.Len(t, chatReq.Messages, 5)

	require.Equal(t, "system", chatReq.Messages[0].Role)
	require.Contains(t, chatReq.Messages[0].StringContent(), longInstructions[:80])
	require.Equal(t, "system", chatReq.Messages[1].Role)
	require.Contains(t, chatReq.Messages[1].StringContent(), longDeveloper[:80])
	require.Equal(t, "user", chatReq.Messages[2].Role)
	require.Contains(t, chatReq.Messages[2].StringContent(), "Windows PowerShell")

	assistantMessage := chatReq.Messages[3]
	require.Equal(t, "assistant", assistantMessage.Role)
	require.NotEmpty(t, assistantMessage.ToolCalls)
	require.Contains(t, string(assistantMessage.ToolCalls), "call_test")
	require.Contains(t, string(assistantMessage.ToolCalls), "shell_command")

	toolMessage := chatReq.Messages[4]
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

func mustJSONString(t *testing.T, value string) string {
	t.Helper()

	data, err := common.Marshal(value)
	require.NoError(t, err)
	return string(data)
}
