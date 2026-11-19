package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildClaudeUsageFromOpenAIUsage(t *testing.T) {
	tests := []struct {
		name                            string
		semantic                        string
		prompt, read, write, five, hour int
		wantInput, wantFive, wantHour   int
	}{
		{name: "cold", prompt: 6146, wantInput: 6146},
		{name: "cache read", prompt: 6146, read: 5888, wantInput: 258},
		{name: "cache write", prompt: 6146, write: 6000, wantInput: 146, wantFive: 6000},
		{name: "read and write", prompt: 6146, read: 5888, write: 200, five: 80, hour: 100, wantInput: 58, wantFive: 100, wantHour: 100},
		{name: "split only", prompt: 6146, five: 80, hour: 100, wantInput: 5966, wantFive: 80, wantHour: 100},
		{name: "split exceeds aggregate", prompt: 6146, write: 120, five: 80, hour: 100, wantInput: 5966, wantFive: 80, wantHour: 100},
		{name: "full hit", prompt: 6146, read: 6146},
		{name: "already split", semantic: "anthropic", prompt: 258, read: 5888, five: 20, wantInput: 258, wantFive: 20},
		{name: "inconsistent upstream total", prompt: 6146, read: 6200},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := dto.Usage{
				PromptTokens:                test.prompt,
				CompletionTokens:            2,
				UsageSemantic:               test.semantic,
				PromptTokensDetails:         dto.InputTokenDetails{CachedTokens: test.read, CachedCreationTokens: test.write},
				ClaudeCacheCreation5mTokens: test.five,
				ClaudeCacheCreation1hTokens: test.hour,
			}
			original := upstream
			usage := buildClaudeUsageFromOpenAIUsage(&upstream)
			require.Equal(t, test.wantInput, usage.InputTokens)
			require.Equal(t, 2, usage.OutputTokens)
			require.Equal(t, test.read, usage.CacheReadInputTokens)
			require.Equal(t, test.wantFive+test.wantHour, usage.CacheCreationInputTokens)
			require.Equal(t, test.wantFive, usage.GetCacheCreation5mTokens())
			require.Equal(t, test.wantHour, usage.GetCacheCreation1hTokens())
			require.Equal(t, original, upstream)
		})
	}
}

func TestResponseOpenAI2ClaudePreservesSettlementUsage(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatClaude,
		FinalRequestRelayFormat: types.RelayFormatOpenAI,
		PriceData: types.PriceData{
			ModelRatio:      0.5,
			CompletionRatio: 1,
			CacheRatio:      1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
		},
	}
	for _, cached := range []int{0, 5888} {
		upstream := dto.OpenAITextResponse{Usage: dto.Usage{
			PromptTokens:        6146,
			CompletionTokens:    2,
			PromptTokensDetails: dto.InputTokenDetails{CachedTokens: cached},
		}}
		before := calculateTextQuotaSummary(ctx, info, &upstream.Usage)
		converted := ResponseOpenAI2Claude(&upstream, info)
		require.Equal(t, 6146-cached, converted.Usage.InputTokens)
		require.Equal(t, 3074, before.Quota)
		require.Equal(t, before, calculateTextQuotaSummary(ctx, info, &upstream.Usage))
	}
}
