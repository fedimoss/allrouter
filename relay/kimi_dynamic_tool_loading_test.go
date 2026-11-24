package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 覆盖顶层工具与消息级工具并存的完整往返，防止任一来源在序列化时丢失。
func TestGeneralOpenAIRequestPreservesMessageLevelTools(t *testing.T) {
	raw := []byte(`{
		"model":"kimi-k3.1-pro",
		"tool_choice":"required",
		"tools":[{"type":"function","function":{"name":"get_weather","description":"Get the weather","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}],
		"messages":[
			{"role":"system","content":"You are Kimi."},
			{"role":"user","content":"What time is it in Beijing?"},
			{"role":"system","tools":[{"type":"function","function":{"name":"get_current_time","description":"Get the current time of a city","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}}]},
			{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_current_time","arguments":"{\"city\":\"Beijing\"}"}}]},
			{"role":"system","content":"","tools":[{"type":"function","function":{"name":"lookup_order","parameters":{"type":"object"}}}]}
		]
	}`)

	var req dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	require.Len(t, req.Messages, 5)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	messages := gjson.GetBytes(encoded, "messages").Array()
	require.Len(t, messages, 5)
	assert.Equal(t, "required", gjson.GetBytes(encoded, "tool_choice").String())
	assert.JSONEq(t, gjson.GetBytes(raw, "tools").Raw, gjson.GetBytes(encoded, "tools").Raw)

	assert.Equal(t, "You are Kimi.", messages[0].Get("content").String())
	assert.False(t, messages[0].Get("tools").Exists())
	assert.Equal(t, "What time is it in Beijing?", messages[1].Get("content").String())

	assert.JSONEq(t, gjson.GetBytes(raw, "messages.2.tools").Raw, messages[2].Get("tools").Raw)
	assert.False(t, messages[2].Get("content").Exists())
	assert.Equal(t, "system", messages[2].Get("role").String())

	// assistant 工具调用回合必须继续输出显式的 "content": null。
	assistantContent := messages[3].Get("content")
	assert.True(t, assistantContent.Exists())
	assert.Equal(t, gjson.Null, assistantContent.Type)
	assert.JSONEq(t, gjson.GetBytes(raw, "messages.3.tool_calls").Raw, messages[3].Get("tool_calls").Raw)

	emptyContent := messages[4].Get("content")
	assert.True(t, emptyContent.Exists())
	assert.Equal(t, gjson.String, emptyContent.Type)
	assert.Equal(t, "", emptyContent.String())
	assert.JSONEq(t, gjson.GetBytes(raw, "messages.4.tools").Raw, messages[4].Get("tools").Raw)

	meta := req.GetTokenCountMeta()
	assert.Equal(t, 3, meta.ToolsCount)
	assert.Equal(t, 5, meta.MessagesCount)
	assert.Contains(t, meta.CombineText, "get_weather")
	assert.Contains(t, meta.CombineText, "get_current_time")
	assert.Contains(t, meta.CombineText, "Get the current time of a city")
	assert.Contains(t, meta.CombineText, "lookup_order")
}

// 对应线上失败形态：只有消息级工具、没有顶层 tools，且 tool_choice 为 required。
func TestGeneralOpenAIRequestPreservesOnlyDynamicTools(t *testing.T) {
	raw := []byte(`{
		"model":"kimi-k3.1-pro",
		"tool_choice":"required",
		"messages":[
			{"role":"system","content":"You are Kimi."},
			{"role":"user","content":"Calculate fuel consumption."},
			{"role":"system","tools":[{"type":"function","function":{"name":"Calculator","description":"calculate an expression","parameters":{"type":"object","properties":{"expr":{"type":"string"}},"required":["expr"]}}}]}
		]
	}`)

	var req dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal(raw, &req))
	require.Empty(t, req.Tools)

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	assert.Equal(t, "required", gjson.GetBytes(encoded, "tool_choice").String())
	assert.False(t, gjson.GetBytes(encoded, "tools").Exists())
	assert.Len(t, gjson.GetBytes(encoded, "messages").Array(), 3)
	assert.Equal(t, "Calculator", gjson.GetBytes(encoded, "messages.2.tools.0.function.name").String())
	assert.False(t, gjson.GetBytes(encoded, "messages.2.content").Exists())
	assert.Equal(t, 1, req.GetTokenCountMeta().ToolsCount)
}

// 系统提示只能注入真正的 system prompt，不能污染动态工具加载消息。
func TestApplySystemPromptIfNeededSkipsToolLoadingMessages(t *testing.T) {
	tools := json.RawMessage(`[{"type":"function","function":{"name":"get_current_time","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}]`)
	toolLoading := dto.Message{Role: "system", Tools: tools}
	user := dto.Message{Role: "user", Content: "What time is it in Beijing?"}

	tests := []struct {
		name         string
		messages     []dto.Message
		wantMessages []dto.Message
		wantOverride bool
	}{
		{
			name:     "tool loading message alone is not a system prompt",
			messages: []dto.Message{toolLoading, user},
			wantMessages: []dto.Message{
				{Role: "system", Content: "Answer in English."},
				toolLoading,
				user,
			},
		},
		{
			name:     "override targets the real system prompt only",
			messages: []dto.Message{toolLoading, {Role: "system", Content: "You are Kimi."}, user},
			wantMessages: []dto.Message{
				toolLoading,
				{Role: "system", Content: "Answer in English.\nYou are Kimi."},
				user,
			},
			wantOverride: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelSetting: dto.ChannelSettings{
						SystemPrompt:         "Answer in English.",
						SystemPromptOverride: true,
					},
				},
			}
			request := &dto.GeneralOpenAIRequest{
				Model:    "kimi-k3.1-pro",
				Messages: append([]dto.Message(nil), tt.messages...),
			}

			applySystemPromptIfNeeded(c, info, request)

			require.Len(t, request.Messages, len(tt.wantMessages))
			for i, want := range tt.wantMessages {
				got := request.Messages[i]
				assert.Equal(t, want.Role, got.Role, "message %d role", i)
				assert.Equal(t, want.Content, got.Content, "message %d content", i)
				if len(want.Tools) > 0 {
					assert.JSONEq(t, string(want.Tools), string(got.Tools), "message %d tools", i)
				} else {
					assert.Empty(t, got.Tools, "message %d tools", i)
				}
			}
			_, overrideSet := common.GetContextKey(c, constant.ContextKeySystemPromptOverride)
			assert.Equal(t, tt.wantOverride, overrideSet)
		})
	}
}
