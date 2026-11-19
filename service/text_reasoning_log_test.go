package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestMessagesReasoningUsageLogDoesNotChangeQuota 比较有无独立思考计数的扣额与日志。
func TestMessagesReasoningUsageLogDoesNotChangeQuota(t *testing.T) {
	const tokenID = 21002
	for _, test := range []struct {
		id    string
		count int
	}{{"tc02-reasoning-on", 345}, {"tc02-reasoning-absent", 0}} {
		t.Run(test.id, func(t *testing.T) {
			// 只清理当前请求的记录，不影响其他用例共用的日志库。
			t.Cleanup(func() {
				require.NoError(t, model.LOG_DB.Where("request_id = ?", test.id).Delete(&model.Log{}).Error)
			})
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			ctx.Set(common.RequestIdKey, test.id)
			info := &relaycommon.RelayInfo{
				RequestId:               test.id,
				TokenId:                 tokenID,
				OriginModelName:         "deepseek-v4.1-flash-arm-210",
				StartTime:               time.Now(),
				RelayFormat:             types.RelayFormatClaude,
				FinalRequestRelayFormat: types.RelayFormatOpenAI,
				ChannelMeta:             &relaycommon.ChannelMeta{},
				FinalPreConsumedQuota:   414, // 与预期扣额对齐，聚焦用量转换和日志明细。
				PriceData: types.PriceData{
					ModelRatio:      1,
					CompletionRatio: 1,
					GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
				},
			}
			upstream := dto.OpenAITextResponse{Usage: dto.Usage{
				PromptTokens:           67,
				CompletionTokens:       347, // 两组输出总量相同，独立思考计数只是其中的子项。
				TotalTokens:            414,
				CompletionTokenDetails: dto.OutputTokenDetails{ReasoningTokens: test.count},
			}}
			// 先生成客户端响应，再用同一份上游数据走正式结算。
			response := ResponseOpenAI2Claude(&upstream, info)
			PostTextConsumeQuota(ctx, info, &upstream.Usage, nil)
			logs, err := model.GetLogByTokenId(tokenID)
			require.NoError(t, err)
			require.Len(t, logs, 1)
			require.Equal(t, test.id, logs[0].RequestId)
			require.Equal(t, 414, logs[0].Quota) // 按输入和总输出结算，不重复加收思考计数。
			require.Equal(t, 347, logs[0].CompletionTokens)
			require.Equal(t, 347, response.Usage.OutputTokens)
			var raw map[string]any
			require.NoError(t, common.UnmarshalJsonStr(logs[0].Other, &raw))
			require.Equal(t, test.count, response.Usage.ReasoningTokens)
			if test.count == 0 {
				// 没有可报告的独立计数时，正式日志也省略该字段。
				require.NotContains(t, raw, "reasoning_tokens")
			} else {
				require.EqualValues(t, response.Usage.ReasoningTokens, raw["reasoning_tokens"])
			}
		})
	}
}
