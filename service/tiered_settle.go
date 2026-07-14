package service

import (
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// TieredResultWrapper wraps billingexpr.TieredResult for use at the service layer.
type TieredResultWrapper = billingexpr.TieredResult

// BuildTieredTokenParams constructs billingexpr.TokenParams from a dto.Usage,
// normalizing P and C so they mean "tokens not separately priced by the
// expression". Sub-categories (cache, image, audio) are only subtracted
// when the expression references them via their own variable.
//
// GPT-format APIs report prompt_tokens / completion_tokens as totals that
// include all sub-categories (cache, image, audio). Claude-format APIs
// report them as text-only. This function normalizes to text-only when
// sub-categories are separately priced.
func BuildTieredTokenParams(usage *dto.Usage, isClaudeUsageSemantic bool, usedVars map[string]bool) billingexpr.TokenParams {
	if usage == nil {
		return billingexpr.TokenParams{}
	}

	p := float64(usage.PromptTokens)
	c := float64(usage.CompletionTokens)
	cr := float64(usage.PromptTokensDetails.CachedTokens)
	// Preserve the 5m/1h split whenever the converter supplied it, including
	// Claude -> OpenAI compatibility responses. Any aggregate remainder is
	// treated as the generic/5m bucket so no cache-creation tokens disappear.
	cc5mTokens, cc1hTokens := NormalizeCacheCreationSplit(
		usage.PromptTokensDetails.CachedCreationTokens,
		usage.ClaudeCacheCreation5mTokens,
		usage.ClaudeCacheCreation1hTokens,
	)
	cc5m := float64(cc5mTokens)
	cc1h := float64(cc1hTokens)

	img := float64(usage.PromptTokensDetails.ImageTokens)
	ai := float64(usage.PromptTokensDetails.AudioTokens)
	imgO := float64(usage.CompletionTokenDetails.ImageTokens)
	ao := float64(usage.CompletionTokenDetails.AudioTokens)

	// len = total input context length for tier condition evaluation.
	// Non-Claude: prompt_tokens already includes everything.
	// Claude: input_tokens is text-only, so add cache read + cache creation.
	inputLen := p
	if isClaudeUsageSemantic {
		inputLen = p + cr + cc5m + cc1h
	}

	if !isClaudeUsageSemantic {
		if usedVars["cr"] {
			p -= cr
		}
		if usedVars["cc"] {
			p -= cc5m
		}
		if usedVars["cc1h"] {
			p -= cc1h
		}
		if usedVars["img"] {
			p -= img
		}
		if usedVars["ai"] {
			p -= ai
		}
		if usedVars["img_o"] {
			c -= imgO
		}
		if usedVars["ao"] {
			c -= ao
		}
	}

	if p < 0 {
		p = 0
	}
	if c < 0 {
		c = 0
	}

	return billingexpr.TokenParams{
		P:    p,
		C:    c,
		Len:  inputLen,
		CR:   cr,
		CC:   cc5m,
		CC1h: cc1h,
		Img:  img,
		ImgO: imgO,
		AI:   ai,
		AO:   ao,
	}
}

// TryTieredSettle checks if the request uses tiered_expr billing and, if so,
// computes the actual quota using the frozen BillingSnapshot. Returns:
//   - ok=true, quota, result  when tiered billing applies
//   - ok=false, 0, nil        when it doesn't (caller should fall through to existing logic)
func TryTieredSettle(relayInfo *relaycommon.RelayInfo, params billingexpr.TokenParams) (ok bool, quota int, result *billingexpr.TieredResult) {
	if relayInfo == nil {
		return false, 0, nil
	}
	snap := relayInfo.TieredBillingSnapshot
	if snap == nil || snap.BillingMode != "tiered_expr" {
		return false, 0, nil
	}

	requestInput := billingexpr.RequestInput{}
	if relayInfo.BillingRequestInput != nil {
		requestInput = *relayInfo.BillingRequestInput
	}

	tr, err := billingexpr.ComputeTieredQuotaWithRequest(snap, params, requestInput)
	if err != nil {
		return true, tieredSettlementFallbackQuota(relayInfo, snap), nil
	}

	return true, tr.ActualQuotaAfterGroup, &tr
}

// tieredSettlementFallbackQuota returns a base-model quota. Provider-facing
// requests apply their import/public pricing after this function, so using the
// already marked-up FinalPreConsumedQuota there would apply the markup twice.
func tieredSettlementFallbackQuota(relayInfo *relaycommon.RelayInfo, snap *billingexpr.BillingSnapshot) int {
	if relayInfo != nil && relayInfo.ProviderId <= 0 && relayInfo.FinalPreConsumedQuota > 0 {
		return relayInfo.FinalPreConsumedQuota
	}
	if snap != nil {
		return snap.EstimatedQuotaAfterGroup
	}
	return 0
}

// BuildTieredRealtimeTokenParams normalizes the aggregate usage reported by
// the OpenAI Realtime API for expression-based billing.
func BuildTieredRealtimeTokenParams(usage *dto.RealtimeUsage, usedVars map[string]bool) billingexpr.TokenParams {
	if usage == nil {
		return billingexpr.TokenParams{}
	}

	p := float64(usage.InputTokens)
	c := float64(usage.OutputTokens)
	cr := float64(usage.InputTokenDetails.CachedTokens)
	ai := float64(usage.InputTokenDetails.AudioTokens)
	ao := float64(usage.OutputTokenDetails.AudioTokens)

	// Realtime input/output totals include separately reported subcategories.
	if usedVars["cr"] {
		p -= cr
	}
	if usedVars["ai"] {
		p -= ai
	}
	if usedVars["ao"] {
		c -= ao
	}
	if p < 0 {
		p = 0
	}
	if c < 0 {
		c = 0
	}

	return billingexpr.TokenParams{
		P:   p,
		C:   c,
		Len: float64(usage.InputTokens),
		CR:  cr,
		AI:  ai,
		AO:  ao,
	}
}
