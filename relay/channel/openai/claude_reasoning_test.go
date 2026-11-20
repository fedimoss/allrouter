package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestConvertClaudeRequestDeepSeekThinking(t *testing.T) {
	// 对照显式开关和缺省行为，同时覆盖非目标模型与 OpenRouter 的隔离边界。
	tests := []struct {
		name, model, thinking, wantThinking, wantEffort string
		channel                                         int
	}{
		{"enabled", "deepseek-v4.1-flash-arm-210", "enabled", `{"type":"enabled"}`, "high", constant.ChannelTypeOpenAI},
		{"disabled", "deepseek-v4-flash", "disabled", `{"type":"disabled"}`, "", constant.ChannelTypeOpenAI},
		{"omitted", "deepseek-v4.1-flash-arm-210", "", "", "", constant.ChannelTypeOpenAI},
		{"different upstream family", "llama-test", "enabled", "", "", constant.ChannelTypeOpenAI},
		{"OpenRouter retains its mapping", "deepseek-v4-flash", "enabled", "", "", constant.ChannelTypeOpenRouter},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			apiType, _ := common.ChannelType2APIType(test.channel)
			info := &relaycommon.RelayInfo{
				OriginModelName: "public-alias", // 公开别名不含 DeepSeek 前缀，匹配必须依据上游模型名。
				RelayFormat:     types.RelayFormatClaude,
				ChannelMeta:     &relaycommon.ChannelMeta{ChannelType: test.channel, ApiType: apiType, UpstreamModelName: test.model},
			}
			req := &dto.ClaudeRequest{Model: test.model, MaxTokens: common.GetPointer(uint(3072))}
			if test.thinking != "" {
				req.Thinking = &dto.Thinking{Type: test.thinking, BudgetTokens: common.GetPointer(2048)}
			}
			converted, err := (&Adaptor{}).ConvertClaudeRequest(ctx, info, req)
			require.NoError(t, err)
			chat := converted.(*dto.GeneralOpenAIRequest)
			if test.wantThinking == "" {
				require.Empty(t, chat.THINKING)
			} else {
				require.JSONEq(t, test.wantThinking, string(chat.THINKING))
			}
			require.Equal(t, test.wantEffort, chat.ReasoningEffort)
			require.Equal(t, uint(3072), *chat.MaxTokens) // 思考档位映射不能覆盖客户端的总输出上限。
			if test.channel == constant.ChannelTypeOpenRouter {
				// OpenRouter 的原生 reasoning 参数仍按原有规则映射。
				require.JSONEq(t, `{"enabled":true,"max_tokens":2048}`, string(chat.Reasoning))
			}
		})
	}
}

func TestMessagesReasoningUsage(t *testing.T) {
	// 两类渠道都跑同一组计数与流式回归，防止兼容渠道再次漏判。
	t.Run("OpenAI", func(t *testing.T) {
		testMessagesReasoningUsage(t, constant.ChannelTypeOpenAI)
	})
	t.Run("ResponsesChat", func(t *testing.T) {
		testMessagesReasoningUsage(t, constant.ChannelTypeResponsesChat)
	})
}

func testMessagesReasoningUsage(t *testing.T, channelType int) {
	t.Helper()
	apiType, ok := common.ChannelType2APIType(channelType)
	require.True(t, ok)
	const deepSeek = "deepseek-v4.1-flash-arm-210"
	// 用原始 JSON 验证来源优先级，尤其区分显式零值与缺失字段。
	tests := []struct {
		name, model, extra string
		want               int
	}{
		{"standard", deepSeek, `,"completion_tokens_details":{"reasoning_tokens":9}`, 9},
		{"top level", deepSeek, `,"completion_tokens_details":{},"reasoning_tokens":8`, 8},
		{"both", deepSeek, `,"completion_tokens_details":{"reasoning_tokens":9},"reasoning_tokens":8`, 9},
		{"explicit zero", deepSeek, `,"completion_tokens_details":{"reasoning_tokens":0},"reasoning_tokens":8`, 0},
		{"missing", deepSeek, "", 0},
		{"invalid top level", deepSeek, `,"reasoning_tokens":1.5`, 0},
		{"negative detail", deepSeek, `,"completion_tokens_details":{"reasoning_tokens":-1},"reasoning_tokens":8`, 0},
		{"unmapped field", "other-model", `,"reasoning_tokens":8`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstreamUsage := `{"prompt_tokens":20,"completion_tokens":11,"total_tokens":31` + test.extra + `}` // 固定总量，只替换思考明细。
			for _, name := range []string{"json", "sse", "sse early usage", "sse updated usage"} {
				if (name == "sse early usage" || name == "sse updated usage") && test.model != deepSeek {
					continue
				}
				streaming := name != "json"
				t.Run(name, func(t *testing.T) {
					recorder := httptest.NewRecorder()
					ctx, _ := gin.CreateTestContext(recorder)
					ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
					info := &relaycommon.RelayInfo{
						RelayFormat:       types.RelayFormatClaude,
						IsStream:          streaming,
						DisablePing:       true, // 排除心跳帧，便于只检查业务事件。
						ChannelMeta:       &relaycommon.ChannelMeta{ChannelType: channelType, ApiType: apiType, UpstreamModelName: test.model},
						ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone},
					}
					body := `{"choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"summary","content":"58"},"finish_reason":"stop"}],"usage":` + upstreamUsage + `}`
					handler := OpenaiHandler
					if streaming {
						body = "data: " + `{"choices":[{"index":0,"delta":{"reasoning_content":"summary"}}]}` + "\n\n"
						answer := `{"choices":[{"index":0,"delta":{"content":"58"},"finish_reason":"stop"}]`
						switch name {
						case "sse early usage":
							// 用量和结束原因同帧，后续空帧不能清掉已保存的用量。
							body += "data: " + answer + `,"usage":` + upstreamUsage + "}\n\ndata: {\"choices\":[]}\n\n"
						case "sse updated usage":
							// 先发旧值，验证最后独立用量帧覆盖它。
							body += "data: " + answer + `,"usage":{"prompt_tokens":20,"completion_tokens":11,"total_tokens":31,"reasoning_tokens":7}}` + "\n\n"
						default:
							body += "data: " + answer + "}\n\n"
						}
						if name != "sse early usage" {
							// 模拟 Chat 以无 choices 的独立帧报告最终用量。
							body += "data: " + `{"choices":[],"usage":` + upstreamUsage + "}\n\n"
						}
						body += "data: [DONE]\n\n"
						handler = OaiStreamHandler
					}
					upstream := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
					usage, relayErr := handler(ctx, info, upstream)
					require.Nil(t, relayErr)
					require.Equal(t, http.StatusOK, recorder.Code)
					require.Equal(t, 20, usage.PromptTokens)
					require.Equal(t, 11, usage.CompletionTokens)
					require.Equal(t, 31, usage.TotalTokens)
					var result struct {
						Usage struct {
							OutputTokens    int  `json:"output_tokens"`
							ReasoningTokens *int `json:"reasoning_tokens"` // 用指针区分省略字段与返回数值 0。
						} `json:"usage"`
					}
					response := recorder.Body.String()
					if streaming {
						require.Contains(t, response, `"type":"thinking_delta"`)
						require.Contains(t, response, `"type":"text_delta"`)
						require.Equal(t, 1, strings.Count(response, "event: message_delta")) // 不为多帧用量重复发送结束用量。
						for _, line := range strings.Split(response, "\n") {
							frame, ok := strings.CutPrefix(line, "data: ")
							if ok && strings.Contains(frame, `"type":"message_delta"`) {
								require.NoError(t, common.UnmarshalJsonStr(frame, &result))
							}
						}
					} else {
						require.Contains(t, response, `"type":"thinking"`)
						require.Contains(t, response, `"type":"text"`)
						require.NoError(t, common.UnmarshalJsonStr(response, &result))
					}
					require.Equal(t, 11, result.Usage.OutputTokens) // 思考明细不能再加到输出总量。
					if test.want > 0 {
						require.Equal(t, test.want, usage.CompletionTokenDetails.ReasoningTokens)
						require.Equal(t, common.GetPointer(test.want), result.Usage.ReasoningTokens)
					} else {
						require.Nil(t, result.Usage.ReasoningTokens)
					}
				})
			}
		})
	}
}
