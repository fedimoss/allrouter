package relay

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestClaudeHelperResponsesChatDeepSeekThinking 走真实的渠道选择和 HTTP 转发，避免只测普通 OpenAI 渠道而漏掉渠道 98。
func TestClaudeHelperResponsesChatDeepSeekThinking(t *testing.T) {
	const modelName = "deepseek-v4.1-flash-arm-210"
	const channelID = 98
	const expectedQuota = 31 // 模拟上游让三组总量相同，用于检查思考明细不会重复扣费。
	service.InitHttpClient()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{}) // 与真实数据库隔离，仍走正式用量日志写入流程。
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // SQLite 内存库按连接隔离，确保读写使用同一份表。
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Token{}, &model.Log{}))
	// 保存并恢复包级配置，避免模拟数据库影响其他测试。
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldLogging, oldRedis := common.LogConsumeEnabled, common.RedisEnabled
	oldBatch := common.BatchUpdateEnabled
	model.DB, model.LOG_DB = db, db
	common.LogConsumeEnabled, common.RedisEnabled, common.BatchUpdateEnabled = true, false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.LogConsumeEnabled, common.RedisEnabled, common.BatchUpdateEnabled = oldLogging, oldRedis, oldBatch
	})
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "test"}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: channelID, Type: constant.ChannelTypeResponsesChat}).Error)

	for _, mode := range []string{"enabled", "disabled", "omitted"} { // 省略与明确关闭是两种不同的请求语义。
		t.Run(mode, func(t *testing.T) {
			type capturedRequest struct {
				path string
				body dto.GeneralOpenAIRequest
				err  error
			}
			captured := make(chan capturedRequest, 1) // 上游处理器返回后再断言收到的载荷。
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				var sent dto.GeneralOpenAIRequest
				parseErr := common.DecodeJson(request.Body, &sent)
				captured <- capturedRequest{request.URL.Path, sent, parseErr}
				if parseErr != nil {
					http.Error(writer, "invalid request JSON", http.StatusBadRequest)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				if sent.ReasoningEffort == "high" { // 只有参数传对才返回思考内容，便于发现漏传。
					_, _ = io.WriteString(writer, `{"model":"`+modelName+`","choices":[{"message":{"role":"assistant","reasoning_content":"summary","content":"58"},"finish_reason":"stop"}],"usage":{"prompt_tokens":20,"completion_tokens":11,"total_tokens":31,"reasoning_tokens":9}}`)
					return
				}
				_, _ = io.WriteString(writer, `{"model":"`+modelName+`","choices":[{"message":{"role":"assistant","content":"58"},"finish_reason":"stop"}],"usage":{"prompt_tokens":29,"completion_tokens":2,"total_tokens":31}}`)
			}))
			t.Cleanup(upstream.Close)

			// 给开启分支附带预算，确认网关只映射开关和 high 档，不转发预算。
			var request dto.ClaudeRequest
			require.NoError(t, common.UnmarshalJsonStr(`{"model":"`+modelName+`","max_tokens":3072,"messages":[{"role":"user","content":"Give the final integer"}]}`, &request))
			if mode != "omitted" {
				request.Thinking = &dto.Thinking{Type: mode}
				if mode == "enabled" {
					request.Thinking.BudgetTokens = common.GetPointer(2048)
				}
			}
			body, err := common.Marshal(&request)
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			requestID := "tc02-responses-chat-" + mode // 用同一请求 ID 将回包和落库用量关联起来。
			ctx.Set(common.RequestIdKey, requestID)
			ctx.Set("id", 1)
			common.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeResponsesChat)
			common.SetContextKey(ctx, constant.ContextKeyChannelId, channelID)
			common.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, upstream.URL)
			common.SetContextKey(ctx, constant.ContextKeyChannelKey, "test-key")
			common.SetContextKey(ctx, constant.ContextKeyOriginalModel, modelName)
			info := relaycommon.GenRelayInfoClaude(ctx, &request) // 由生产入口初始化路由，不手工指定 API 类型。
			info.UserId = 1
			info.StartTime = time.Now()
			info.FinalPreConsumedQuota = expectedQuota
			info.PriceData = types.PriceData{ModelRatio: 1, CompletionRatio: 1, GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1}}
			require.Nil(t, ClaudeHelper(ctx, info))
			require.Equal(t, constant.APITypeResponsesChat, info.ApiType) // 确认没有误用普通 OpenAI 类型掩盖漏判。
			require.Len(t, captured, 1)
			sent := <-captured
			require.NoError(t, sent.err)
			require.Equal(t, "/v1/chat/completions", sent.path) // 客户端调用 Messages，上游实际接收 Chat 请求。
			require.Equal(t, modelName, sent.body.Model)
			if mode == "omitted" {
				require.Empty(t, sent.body.THINKING)
			} else {
				require.JSONEq(t, `{"type":"`+mode+`"}`, string(sent.body.THINKING))
			}

			require.Equal(t, http.StatusOK, recorder.Code)
			var response dto.ClaudeResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.NotNil(t, response.Usage)
			// 查正式用量日志，而不是依赖临时打印来核对扣额。
			var log model.Log
			require.NoError(t, db.Where("request_id = ?", requestID).Take(&log).Error)
			require.Equal(t, expectedQuota, log.Quota)
			require.Equal(t, response.Usage.OutputTokens, log.CompletionTokens)
			var details map[string]any
			require.NoError(t, common.UnmarshalJsonStr(log.Other, &details))
			if mode == "enabled" {
				require.Equal(t, "high", sent.body.ReasoningEffort)
				require.Len(t, response.Content, 2)
				require.Equal(t, "thinking", response.Content[0].Type)
				require.Equal(t, "text", response.Content[1].Type)
				require.Equal(t, 9, response.Usage.ReasoningTokens) // 独立计数来自上游，不从总输出量估算。
				require.EqualValues(t, response.Usage.ReasoningTokens, details["reasoning_tokens"])
			} else {
				require.Empty(t, sent.body.ReasoningEffort)
				require.Len(t, response.Content, 1)
				require.Equal(t, "text", response.Content[0].Type)
				require.NotContains(t, recorder.Body.String(), `"reasoning_tokens"`)
				require.NotContains(t, details, "reasoning_tokens")
			}
		})
	}
}
