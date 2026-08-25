package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResponsesChatCompatViaChatCompletionsIgnoresPassThroughBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	var upstreamPath string
	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		var err error
		upstreamBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		http.Error(w, "captured", http.StatusBadRequest)
	}))
	defer upstream.Close()

	originalPassThrough := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = true
	defer func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = originalPassThrough
	}()

	rawBody := []byte(`{
		"model":"GLM5.1",
		"input":[
			{"type":"message","role":"developer","content":[{"type":"input_text","text":"be concise"}]},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}
		],
		"tools":[
			{"type":"function","name":"lookup","description":"lookup docs","parameters":{"type":"object","properties":{}}},
			{"type":"namespace","name":"multi_agent_v1"},
			{"type":"web_search"}
		],
		"tool_choice":"auto"
	}`)

	var responsesReq dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(rawBody, &responsesReq))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(rawBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeResponsesChat)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 41)
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "GLM5.1")

	info := &relaycommon.RelayInfo{
		Request:         &responsesReq,
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RequestURLPath:  "/v1/responses",
		OriginModelName: "GLM5.1",
	}

	err := ResponsesHelper(ctx, info)
	require.NotNil(t, err)
	require.Same(t, &responsesReq, info.Request)
	require.Equal(t, "/v1/chat/completions", upstreamPath)

	var sent map[string]any
	require.NoError(t, common.Unmarshal(upstreamBody, &sent))
	require.Contains(t, sent, "messages")
	require.NotContains(t, sent, "input")

	tools, ok := sent["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	firstTool, ok := tools[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "function", firstTool["type"])
	require.Contains(t, firstTool, "function")
	require.NotContains(t, firstTool, "name")
}

func TestResponsesChatCompatViaChatCompletionsPreservesToolHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	var upstreamPath string
	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		var err error
		upstreamBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		http.Error(w, "captured", http.StatusBadRequest)
	}))
	defer upstream.Close()

	rawBody := []byte(`{
		"model":"GLM5.1",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"create a java hello world file"}]},
			{"type":"function_call","call_id":"call_test","name":"shell_command","arguments":"{\"command\":\"Set-Content -Path HelloWorld.java -Value 'public class HelloWorld {}' -Encoding UTF8\"}"},
			{"type":"function_call_output","call_id":"call_test","output":"ok"}
		],
		"tools":[
			{"type":"function","name":"shell_command","description":"run a shell command","parameters":{"type":"object","properties":{"command":{"type":"string"}}}}
		],
		"tool_choice":"auto"
	}`)

	var responsesReq dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(rawBody, &responsesReq))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(rawBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeResponsesChat)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 42)
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "GLM5.1")

	info := &relaycommon.RelayInfo{
		Request:         &responsesReq,
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RequestURLPath:  "/v1/responses",
		OriginModelName: "GLM5.1",
	}

	err := ResponsesHelper(ctx, info)
	require.NotNil(t, err)
	require.Equal(t, "/v1/chat/completions", upstreamPath)

	var sent dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal(upstreamBody, &sent))
	require.Len(t, sent.Tools, 1)
	require.NotNil(t, sent.ToolChoice)
	require.Len(t, sent.Messages, 3)
	require.Equal(t, "user", sent.Messages[0].Role)
	require.Contains(t, sent.Messages[0].StringContent(), "create a java hello world file")
	require.Equal(t, "assistant", sent.Messages[1].Role)
	require.Contains(t, string(sent.Messages[1].ToolCalls), "call_test")
	require.Contains(t, string(sent.Messages[1].ToolCalls), "shell_command")
	require.Equal(t, "tool", sent.Messages[2].Role)
	require.Equal(t, "call_test", sent.Messages[2].ToolCallId)
	require.NotNil(t, sent.Messages[2].Name)
	require.Equal(t, "shell_command", *sent.Messages[2].Name)
	require.Equal(t, "ok", sent.Messages[2].StringContent())
}

func TestResponsesChatCompatViaChatCompletionsStreamsResponsesEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	originalStreamingTimeout := constant.StreamingTimeout
	originalRedisEnabled := common.RedisEnabled
	constant.StreamingTimeout = 30
	common.RedisEnabled = false
	t.Cleanup(func() {
		constant.StreamingTimeout = originalStreamingTimeout
		common.RedisEnabled = originalRedisEnabled
	})
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
	})
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "test"}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 42, Type: constant.ChannelTypeResponsesChat, Key: "test-key", Status: common.ChannelStatusEnabled, Name: "GLM5.1"}).Error)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"GLM5.1","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"GLM5.1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	rawBody := []byte(`{
		"model":"GLM5.1",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],
		"stream":true
	}`)

	var responsesReq dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(rawBody, &responsesReq))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(rawBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeResponsesChat)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 42)
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "GLM5.1")

	info := &relaycommon.RelayInfo{
		Request:         &responsesReq,
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RequestURLPath:  "/v1/responses",
		OriginModelName: "GLM5.1",
		UserId:          1,
	}

	relayErr := ResponsesHelper(ctx, info)
	require.Nil(t, relayErr)

	body := recorder.Body.String()
	require.Contains(t, body, "event: response.output_text.delta")
	require.Contains(t, body, `"delta":"hello"`)
	require.Contains(t, body, "event: response.completed")
	require.NotContains(t, body, "chat.completion.chunk")
}

// TestResponsesChatCompatViaChatCompletionsExposesReasoningContent 验证 Responses→Chat 渠道
// 会把上游 chat 的 reasoning_content 作为 Responses 的 reasoning 条目暴露给客户端
// （对齐 cc-switch streaming_codex_chat 与项目其它渠道的默认行为：暴露推理，而非剥离）。
func TestResponsesChatCompatViaChatCompletionsExposesReasoningContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	originalStreamingTimeout := constant.StreamingTimeout
	originalRedisEnabled := common.RedisEnabled
	constant.StreamingTimeout = 30
	common.RedisEnabled = false
	t.Cleanup(func() {
		constant.StreamingTimeout = originalStreamingTimeout
		common.RedisEnabled = originalRedisEnabled
	})
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
	})
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "test"}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 42, Type: constant.ChannelTypeResponsesChat, Key: "test-key", Status: common.ChannelStatusEnabled, Name: "GLM5.1"}).Error)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"GLM5.1","choices":[{"index":0,"delta":{"reasoning_content":"internal plan"},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"GLM5.1","choices":[{"index":0,"delta":{"content":"visible answer"},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"GLM5.1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	rawBody := []byte(`{
		"model":"GLM5.1",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],
		"stream":true
	}`)

	var responsesReq dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(rawBody, &responsesReq))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(rawBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeResponsesChat)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 42)
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "GLM5.1")

	info := &relaycommon.RelayInfo{
		Request:         &responsesReq,
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RequestURLPath:  "/v1/responses",
		OriginModelName: "GLM5.1",
		UserId:          1,
	}

	relayErr := ResponsesHelper(ctx, info)
	require.Nil(t, relayErr)

	body := recorder.Body.String()
	require.Contains(t, body, `"delta":"visible answer"`)
	require.Contains(t, body, `"text":"visible answer"`)
	// reasoning_content 被作为独立 reasoning 条目暴露（对齐 cc-switch）：
	// 出现在 reasoning summary 增量/完成事件与最终 reasoning 输出条目中。
	require.Contains(t, body, "internal plan")
	require.Contains(t, body, `"type":"reasoning"`)
	require.Contains(t, body, `"type":"summary_text"`)
}

func TestResponsesChatCompatViaChatCompletionsStreamsToolCallForCodex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	originalStreamingTimeout := constant.StreamingTimeout
	originalRedisEnabled := common.RedisEnabled
	constant.StreamingTimeout = 30
	common.RedisEnabled = false
	t.Cleanup(func() {
		constant.StreamingTimeout = originalStreamingTimeout
		common.RedisEnabled = originalRedisEnabled
	})
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Log{}))
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
	})
	require.NoError(t, model.DB.Create(&model.User{Id: 1, Username: "test"}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 42, Type: constant.ChannelTypeResponsesChat, Key: "test-key", Status: common.ChannelStatusEnabled, Name: "GLM5.1"}).Error)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"GLM5.1","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_test","index":0,"type":"function","function":{"name":"shell_command","arguments":""}}]},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"GLM5.1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"type":"function","function":{"arguments":"{\"command\":\"cat /dev/null\"}"}}]},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":123,"model":"GLM5.1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	rawBody := []byte(`{
		"model":"GLM5.1",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],
		"stream":true
	}`)

	var responsesReq dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(rawBody, &responsesReq))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(rawBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeResponsesChat)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 42)
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "GLM5.1")

	info := &relaycommon.RelayInfo{
		Request:         &responsesReq,
		RelayMode:       relayconstant.RelayModeResponses,
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RequestURLPath:  "/v1/responses",
		OriginModelName: "GLM5.1",
		UserId:          1,
	}

	relayErr := ResponsesHelper(ctx, info)
	require.Nil(t, relayErr)

	body := recorder.Body.String()
	require.Equal(t, 1, strings.Count(body, `"type":"response.output_item.added","item":{"type":"function_call"`))
	require.Contains(t, body, `"type":"response.function_call_arguments.delta"`)
	require.Contains(t, body, `"type":"response.function_call_arguments.done"`)
	require.Contains(t, body, `"type":"response.output_item.done","item":{"type":"function_call"`)
	require.Contains(t, body, `"output_index":0`)
	require.Contains(t, body, `"call_id":"call_test"`)
	require.Contains(t, body, `"name":"shell_command"`)
	require.Contains(t, body, `"arguments":"{\"command\":\"cat /dev/null\"}"`)
	require.Contains(t, body, `"output":[{"type":"function_call"`)
}

// TestResponsesChatChannelChatRequestHandledAsOpenAI 验证 Responses→Chat 渠道在收到
// /v1/chat/completions 请求时，按普通 OpenAI 渠道直接处理（路由到 openai 适配器，不反向转换为 responses）。
// 即便全局 ChatCompletionsToResponsesPolicy 命中该模型，也应直接放行 chat：该渠道上游仅支持
// /v1/chat/completions，反向转换必定失败。与 responses 请求走 responses→chat 转换形成对称镜像。
func TestResponsesChatChannelChatRequestHandledAsOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	// 启用全局 ChatCompletionsToResponsesPolicy 并命中 GLM5.1，验证即便策略命中也不反向转换。
	globalSettings := model_setting.GetGlobalSettings()
	originalPolicy := globalSettings.ChatCompletionsToResponsesPolicy
	globalSettings.ChatCompletionsToResponsesPolicy = model_setting.ChatCompletionsToResponsesPolicy{
		Enabled:       true,
		AllChannels:   true,
		ModelPatterns: []string{"GLM.*"},
	}
	t.Cleanup(func() {
		globalSettings.ChatCompletionsToResponsesPolicy = originalPolicy
	})

	var upstreamPath string
	var upstreamBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		var err error
		upstreamBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		http.Error(w, "captured", http.StatusBadRequest)
	}))
	defer upstream.Close()

	rawBody := []byte(`{
		"model":"GLM5.1",
		"messages":[{"role":"user","content":"hello"}]
	}`)

	var chatReq dto.GeneralOpenAIRequest
	require.NoError(t, common.Unmarshal(rawBody, &chatReq))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(rawBody))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeResponsesChat)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, 43)
	common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(ctx, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "GLM5.1")

	info := &relaycommon.RelayInfo{
		Request:         &chatReq,
		RelayMode:       relayconstant.RelayModeChatCompletions,
		RelayFormat:     types.RelayFormatOpenAI,
		RequestURLPath:  "/v1/chat/completions",
		OriginModelName: "GLM5.1",
	}

	relayErr := TextHelper(ctx, info)
	require.NotNil(t, relayErr) // 上游返回 400，预期错误返回

	// 关键断言：上游收到 chat completions 路径与 messages 体，未被反向转换为 responses
	require.Equal(t, "/v1/chat/completions", upstreamPath)
	var sent map[string]any
	require.NoError(t, common.Unmarshal(upstreamBody, &sent))
	require.Contains(t, sent, "messages")
	require.NotContains(t, sent, "input")
}
