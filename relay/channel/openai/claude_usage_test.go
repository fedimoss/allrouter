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

func TestOpenAIHandlersConvertClaudeCacheUsage(t *testing.T) {
	const upstreamUsage = `{"prompt_tokens":6146,"completion_tokens":2,"total_tokens":6148,"prompt_cache_hit_tokens":5888}`
	for _, streaming := range []bool{false, true} {
		name := "json"
		if streaming {
			name = "sse"
		}
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			info := &relaycommon.RelayInfo{
				RelayFormat:       types.RelayFormatClaude,
				IsStream:          streaming,
				DisablePing:       true,
				ChannelMeta:       &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeDeepSeek},
				ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone},
			}
			body := `{"id":"chatcmpl_test","model":"deepseek-test","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":` + upstreamUsage + `}`
			handler := OpenaiHandler
			if streaming {
				body = "data: " + `{"id":"chatcmpl_test","model":"deepseek-test","choices":[{"index":0,"delta":{"content":"OK"}}]}` + "\n\n" +
					"data: " + `{"id":"chatcmpl_test","model":"deepseek-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n" +
					"data: " + `{"id":"chatcmpl_test","model":"deepseek-test","choices":[],"usage":` + upstreamUsage + `}` + "\n\ndata: [DONE]\n\n"
				handler = OaiStreamHandler
			}
			resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}
			settlement, relayErr := handler(ctx, info, resp)
			require.Nil(t, relayErr)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, 6146, settlement.PromptTokens)
			require.Equal(t, 5888, settlement.PromptTokensDetails.CachedTokens)
			require.Equal(t, 2, settlement.CompletionTokens)

			var message dto.ClaudeResponse
			if streaming {
				var events []string
				for _, line := range strings.Split(recorder.Body.String(), "\n") {
					payload, ok := strings.CutPrefix(line, "data: ")
					if !ok {
						continue
					}
					var event dto.ClaudeResponse
					require.NoError(t, common.Unmarshal([]byte(payload), &event))
					events = append(events, event.Type)
					if event.Type == "message_delta" {
						message = event
					}
				}
				require.Equal(t, []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}, events)
			} else {
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &message))
			}
			require.NotNil(t, message.Usage)
			require.Equal(t, 258, message.Usage.InputTokens)
			require.Equal(t, 5888, message.Usage.CacheReadInputTokens)
			require.Zero(t, message.Usage.CacheCreationInputTokens)
			require.Equal(t, 2, message.Usage.OutputTokens)
		})
	}
}
