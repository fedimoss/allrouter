package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type textQuotaSummary struct {
	PromptTokens             int
	CompletionTokens         int
	TotalTokens              int
	CacheTokens              int
	CacheCreationTokens      int
	CacheCreationTokens5m    int
	CacheCreationTokens1h    int
	ImageTokens              int
	AudioTokens              int
	ModelName                string
	TokenName                string
	UseTimeSeconds           int64
	CompletionRatio          float64
	CacheRatio               float64
	ImageRatio               float64
	ModelRatio               float64
	GroupRatio               float64
	ModelPrice               float64
	CacheCreationRatio       float64
	CacheCreationRatio5m     float64
	CacheCreationRatio1h     float64
	Quota                    int
	IsClaudeUsageSemantic    bool
	UsageSemantic            string
	WebSearchPrice           float64
	WebSearchCallCount       int
	ClaudeWebSearchPrice     float64
	ClaudeWebSearchCallCount int
	FileSearchPrice          float64
	FileSearchCallCount      int
	AudioInputPrice          float64
	ImageGenerationCallPrice float64
}

func cacheWriteTokensTotal(summary textQuotaSummary) int {
	if summary.CacheCreationTokens5m > 0 || summary.CacheCreationTokens1h > 0 {
		splitCacheWriteTokens := summary.CacheCreationTokens5m + summary.CacheCreationTokens1h
		if summary.CacheCreationTokens > splitCacheWriteTokens {
			return summary.CacheCreationTokens
		}
		return splitCacheWriteTokens
	}
	return summary.CacheCreationTokens
}

func ApplyProviderPricingQuota(ctx *gin.Context, baseQuota int, usePrice bool, groupRatio float64, unitCount int) (providerQuota int, importCostQuota int, applied bool) {
	providerId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderId)
	providerPublicModel := common.GetContextKeyString(ctx, constant.ContextKeyProviderPublicModel)
	if providerId <= 0 || providerPublicModel == "" {
		return baseQuota, baseQuota, false
	}
	importPriceRatio := common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderImportPriceRatio)
	if importPriceRatio <= 0 {
		importPriceRatio = 1
	}
	importCostQuota = common.QuotaFromDecimal(decimal.NewFromInt(int64(baseQuota)).Mul(decimal.NewFromFloat(importPriceRatio)))
	if baseQuota > 0 && importCostQuota == 0 {
		importCostQuota = 1
	}
	providerQuota = importCostQuota
	pricingType := common.GetContextKeyString(ctx, constant.ContextKeyProviderPricingType)
	if pricingType == model.ProviderPricingTypeDelta {
		if usePrice {
			providerQuota += common.QuotaFromDecimal(decimal.NewFromFloat(common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderDeltaPrice)).
				Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
				Mul(decimal.NewFromFloat(groupRatio)))
		} else {
			providerQuota += common.QuotaFromDecimal(decimal.NewFromFloat(common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderDeltaRatio)).
				Mul(decimal.NewFromInt(int64(unitCount))).
				Mul(decimal.NewFromFloat(groupRatio)))
		}
	} else {
		ratio := common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderPricingRatio)
		if ratio == 0 {
			ratio = 1
		}
		providerQuota = common.QuotaFromDecimal(decimal.NewFromInt(int64(importCostQuota)).Mul(decimal.NewFromFloat(ratio)))
	}
	if providerQuota < 0 {
		providerQuota = 0
	}
	return providerQuota, importCostQuota, true
}

func ApplyProviderPricingDisplay(ctx *gin.Context, modelRatio float64, modelPrice float64) (displayModelRatio float64, displayModelPrice float64, applied bool) {
	providerId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderId)
	providerPublicModel := common.GetContextKeyString(ctx, constant.ContextKeyProviderPublicModel)
	if providerId <= 0 || providerPublicModel == "" {
		return modelRatio, modelPrice, false
	}
	importPriceRatio := common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderImportPriceRatio)
	if importPriceRatio <= 0 {
		importPriceRatio = 1
	}
	displayModelRatio = modelRatio * importPriceRatio
	displayModelPrice = modelPrice
	if modelPrice > 0 {
		displayModelPrice = modelPrice * importPriceRatio
	}
	pricingType := common.GetContextKeyString(ctx, constant.ContextKeyProviderPricingType)
	if pricingType == model.ProviderPricingTypeDelta {
		displayModelRatio += common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderDeltaRatio)
		if displayModelPrice >= 0 {
			displayModelPrice += common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderDeltaPrice)
		}
	} else {
		ratio := common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderPricingRatio)
		if ratio == 0 {
			ratio = 1
		}
		displayModelRatio *= ratio
		if displayModelPrice > 0 {
			displayModelPrice *= ratio
		}
	}
	if displayModelRatio < 0 {
		displayModelRatio = 0
	}
	if displayModelPrice < 0 && modelPrice >= 0 {
		displayModelPrice = 0
	}
	return displayModelRatio, displayModelPrice, true
}

func noteQuotaClamp(relayInfo *relaycommon.RelayInfo, clamp *common.QuotaClamp) {
	if clamp == nil || relayInfo == nil || relayInfo.QuotaClamp != nil {
		return
	}
	relayInfo.QuotaClamp = clamp
}

func attachQuotaSaturation(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || relayInfo.QuotaClamp == nil || other == nil {
		return
	}
	other["quota_saturation"] = relayInfo.QuotaClamp.AuditMap()
	logger.LogError(ctx, fmt.Sprintf("quota conversion saturated for userId %d, channelId %d, model %s: %+v",
		relayInfo.UserId, relayInfo.ChannelId, relayInfo.OriginModelName, *relayInfo.QuotaClamp))
}

func isLegacyClaudeDerivedOpenAIUsage(relayInfo *relaycommon.RelayInfo, usage *dto.Usage) bool {
	if relayInfo == nil || usage == nil {
		return false
	}
	if relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return false
	}
	if usage.UsageSource != "" || usage.UsageSemantic != "" {
		return false
	}
	return usage.ClaudeCacheCreation5mTokens > 0 || usage.ClaudeCacheCreation1hTokens > 0
}

func calculateTextQuotaSummary(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage) textQuotaSummary {
	summary := textQuotaSummary{
		ModelName:            relayInfo.OriginModelName,
		TokenName:            ctx.GetString("token_name"),
		UseTimeSeconds:       time.Now().Unix() - relayInfo.StartTime.Unix(),
		CompletionRatio:      relayInfo.PriceData.CompletionRatio,
		CacheRatio:           relayInfo.PriceData.CacheRatio,
		ImageRatio:           relayInfo.PriceData.ImageRatio,
		ModelRatio:           relayInfo.PriceData.ModelRatio,
		GroupRatio:           relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelPrice:           relayInfo.PriceData.ModelPrice,
		CacheCreationRatio:   relayInfo.PriceData.CacheCreationRatio,
		CacheCreationRatio5m: relayInfo.PriceData.CacheCreation5mRatio,
		CacheCreationRatio1h: relayInfo.PriceData.CacheCreation1hRatio,
		UsageSemantic:        usageSemanticFromUsage(relayInfo, usage),
	}
	summary.IsClaudeUsageSemantic = summary.UsageSemantic == "anthropic"

	if usage == nil {
		usage = &dto.Usage{
			PromptTokens:     relayInfo.GetEstimatePromptTokens(),
			CompletionTokens: 0,
			TotalTokens:      relayInfo.GetEstimatePromptTokens(),
		}
	}

	summary.PromptTokens = usage.PromptTokens
	summary.CompletionTokens = usage.CompletionTokens
	summary.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	summary.CacheTokens = usage.PromptTokensDetails.CachedTokens
	summary.CacheCreationTokens = usage.PromptTokensDetails.CachedCreationTokens
	summary.CacheCreationTokens5m = usage.ClaudeCacheCreation5mTokens
	summary.CacheCreationTokens1h = usage.ClaudeCacheCreation1hTokens
	summary.ImageTokens = usage.PromptTokensDetails.ImageTokens
	summary.AudioTokens = usage.PromptTokensDetails.AudioTokens
	legacyClaudeDerived := isLegacyClaudeDerivedOpenAIUsage(relayInfo, usage)
	isOpenRouterClaudeBilling := relayInfo.ChannelMeta != nil &&
		relayInfo.ChannelType == constant.ChannelTypeOpenRouter &&
		summary.IsClaudeUsageSemantic

	if isOpenRouterClaudeBilling {
		summary.PromptTokens -= summary.CacheTokens
		isUsingCustomSettings := relayInfo.PriceData.UsePrice || hasCustomModelRatio(summary.ModelName, relayInfo.PriceData.ModelRatio)
		if summary.CacheCreationTokens == 0 && relayInfo.PriceData.CacheCreationRatio != 1 && usage.Cost != 0 && !isUsingCustomSettings {
			maybeCacheCreationTokens := CalcOpenRouterCacheCreateTokens(*usage, relayInfo.PriceData)
			if maybeCacheCreationTokens >= 0 && summary.PromptTokens >= maybeCacheCreationTokens {
				summary.CacheCreationTokens = maybeCacheCreationTokens
			}
		}
		summary.PromptTokens -= summary.CacheCreationTokens
	}

	dPromptTokens := decimal.NewFromInt(int64(summary.PromptTokens))
	dCacheTokens := decimal.NewFromInt(int64(summary.CacheTokens))
	dImageTokens := decimal.NewFromInt(int64(summary.ImageTokens))
	dAudioTokens := decimal.NewFromInt(int64(summary.AudioTokens))
	dCompletionTokens := decimal.NewFromInt(int64(summary.CompletionTokens))
	dCachedCreationTokens := decimal.NewFromInt(int64(summary.CacheCreationTokens))
	dCompletionRatio := decimal.NewFromFloat(summary.CompletionRatio)
	dCacheRatio := decimal.NewFromFloat(summary.CacheRatio)
	dImageRatio := decimal.NewFromFloat(summary.ImageRatio)
	dModelRatio := decimal.NewFromFloat(summary.ModelRatio)
	dGroupRatio := decimal.NewFromFloat(summary.GroupRatio)
	dModelPrice := decimal.NewFromFloat(summary.ModelPrice)
	dCacheCreationRatio := decimal.NewFromFloat(summary.CacheCreationRatio)
	dCacheCreationRatio5m := decimal.NewFromFloat(summary.CacheCreationRatio5m)
	dCacheCreationRatio1h := decimal.NewFromFloat(summary.CacheCreationRatio1h)
	dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)

	ratio := dModelRatio.Mul(dGroupRatio)

	var dWebSearchQuota decimal.Decimal
	if relayInfo.ResponsesUsageInfo != nil {
		if webSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool.CallCount > 0 {
			summary.WebSearchCallCount = webSearchTool.CallCount
			summary.WebSearchPrice = operation_setting.GetWebSearchPricePerThousand(summary.ModelName, webSearchTool.SearchContextSize)
			dWebSearchQuota = decimal.NewFromFloat(summary.WebSearchPrice).
				Mul(decimal.NewFromInt(int64(webSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
		}
	} else if strings.HasSuffix(summary.ModelName, "search-preview") {
		searchContextSize := ctx.GetString("chat_completion_web_search_context_size")
		if searchContextSize == "" {
			searchContextSize = "medium"
		}
		summary.WebSearchCallCount = 1
		summary.WebSearchPrice = operation_setting.GetWebSearchPricePerThousand(summary.ModelName, searchContextSize)
		dWebSearchQuota = decimal.NewFromFloat(summary.WebSearchPrice).
			Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
	}

	var dClaudeWebSearchQuota decimal.Decimal
	summary.ClaudeWebSearchCallCount = ctx.GetInt("claude_web_search_requests")
	if summary.ClaudeWebSearchCallCount > 0 {
		summary.ClaudeWebSearchPrice = operation_setting.GetClaudeWebSearchPricePerThousand()
		dClaudeWebSearchQuota = decimal.NewFromFloat(summary.ClaudeWebSearchPrice).
			Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit).
			Mul(decimal.NewFromInt(int64(summary.ClaudeWebSearchCallCount)))
	}

	var dFileSearchQuota decimal.Decimal
	if relayInfo.ResponsesUsageInfo != nil {
		if fileSearchTool, exists := relayInfo.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolFileSearch]; exists && fileSearchTool.CallCount > 0 {
			summary.FileSearchCallCount = fileSearchTool.CallCount
			summary.FileSearchPrice = operation_setting.GetFileSearchPricePerThousand()
			dFileSearchQuota = decimal.NewFromFloat(summary.FileSearchPrice).
				Mul(decimal.NewFromInt(int64(fileSearchTool.CallCount))).
				Div(decimal.NewFromInt(1000)).Mul(dGroupRatio).Mul(dQuotaPerUnit)
		}
	}

	var dImageGenerationCallQuota decimal.Decimal
	if ctx.GetBool("image_generation_call") {
		summary.ImageGenerationCallPrice = operation_setting.GetGPTImage1PriceOnceCall(ctx.GetString("image_generation_call_quality"), ctx.GetString("image_generation_call_size"))
		dImageGenerationCallQuota = decimal.NewFromFloat(summary.ImageGenerationCallPrice).Mul(dGroupRatio).Mul(dQuotaPerUnit)
	}

	var audioInputQuota decimal.Decimal
	if !relayInfo.PriceData.UsePrice {
		baseTokens := dPromptTokens

		var cachedTokensWithRatio decimal.Decimal
		if !dCacheTokens.IsZero() {
			if !summary.IsClaudeUsageSemantic && !legacyClaudeDerived {
				baseTokens = baseTokens.Sub(dCacheTokens)
			}
			cachedTokensWithRatio = dCacheTokens.Mul(dCacheRatio)
		}

		var cachedCreationTokensWithRatio decimal.Decimal
		hasSplitCacheCreationTokens := summary.CacheCreationTokens5m > 0 || summary.CacheCreationTokens1h > 0
		if !dCachedCreationTokens.IsZero() || hasSplitCacheCreationTokens {
			if !summary.IsClaudeUsageSemantic && !legacyClaudeDerived {
				baseTokens = baseTokens.Sub(dCachedCreationTokens)
				cachedCreationTokensWithRatio = dCachedCreationTokens.Mul(dCacheCreationRatio)
			} else {
				remaining := summary.CacheCreationTokens - summary.CacheCreationTokens5m - summary.CacheCreationTokens1h
				if remaining < 0 {
					remaining = 0
				}
				cachedCreationTokensWithRatio = decimal.NewFromInt(int64(remaining)).Mul(dCacheCreationRatio)
				cachedCreationTokensWithRatio = cachedCreationTokensWithRatio.Add(decimal.NewFromInt(int64(summary.CacheCreationTokens5m)).Mul(dCacheCreationRatio5m))
				cachedCreationTokensWithRatio = cachedCreationTokensWithRatio.Add(decimal.NewFromInt(int64(summary.CacheCreationTokens1h)).Mul(dCacheCreationRatio1h))
			}
		}

		var imageTokensWithRatio decimal.Decimal
		if !dImageTokens.IsZero() {
			baseTokens = baseTokens.Sub(dImageTokens)
			imageTokensWithRatio = dImageTokens.Mul(dImageRatio)
		}

		if !dAudioTokens.IsZero() {
			summary.AudioInputPrice = operation_setting.GetGeminiInputAudioPricePerMillionTokens(summary.ModelName)
			if summary.AudioInputPrice > 0 {
				baseTokens = baseTokens.Sub(dAudioTokens)
				audioInputQuota = decimal.NewFromFloat(summary.AudioInputPrice).
					Div(decimal.NewFromInt(1000000)).Mul(dAudioTokens).Mul(dGroupRatio).Mul(dQuotaPerUnit)
			}
		}

		promptQuota := baseTokens.Add(cachedTokensWithRatio).Add(imageTokensWithRatio).Add(cachedCreationTokensWithRatio)
		completionQuota := dCompletionTokens.Mul(dCompletionRatio)
		quotaCalculateDecimal := promptQuota.Add(completionQuota).Mul(ratio)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dWebSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dClaudeWebSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dFileSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(audioInputQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dImageGenerationCallQuota)

		quotaCalculateDecimal = relayInfo.PriceData.ApplyOtherRatiosToDecimal(quotaCalculateDecimal)

		if !ratio.IsZero() && quotaCalculateDecimal.LessThanOrEqual(decimal.Zero) {
			quotaCalculateDecimal = decimal.NewFromInt(1)
		}
		quota, clamp := common.QuotaFromDecimalChecked(quotaCalculateDecimal)
		summary.Quota = quota
		noteQuotaClamp(relayInfo, clamp)
	} else {
		quotaCalculateDecimal := dModelPrice.Mul(dQuotaPerUnit).Mul(dGroupRatio)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dWebSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dClaudeWebSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dFileSearchQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(audioInputQuota)
		quotaCalculateDecimal = quotaCalculateDecimal.Add(dImageGenerationCallQuota)
		quotaCalculateDecimal = relayInfo.PriceData.ApplyOtherRatiosToDecimal(quotaCalculateDecimal)
		quota, clamp := common.QuotaFromDecimalChecked(quotaCalculateDecimal)
		summary.Quota = quota
		noteQuotaClamp(relayInfo, clamp)
	}

	if summary.TotalTokens == 0 {
		summary.Quota = 0
	} else if !ratio.IsZero() && summary.Quota == 0 {
		summary.Quota = 1
	}

	return summary
}

func usageSemanticFromUsage(relayInfo *relaycommon.RelayInfo, usage *dto.Usage) string {
	if usage != nil && usage.UsageSemantic != "" {
		return usage.UsageSemantic
	}
	if relayInfo != nil && relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		return "anthropic"
	}
	return "openai"
}

// 文本模型扣除费用
func PostTextConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent []string) {
	originUsage := usage
	if usage == nil {
		extraContent = append(extraContent, "上游无计费信息")
	}
	if originUsage != nil {
		ObserveChannelAffinityUsageCacheByRelayFormat(ctx, usage, relayInfo.GetFinalRequestRelayFormat())
	}

	adminRejectReason := common.GetContextKeyString(ctx, constant.ContextKeyAdminRejectReason)
	summary := calculateTextQuotaSummary(ctx, relayInfo, usage)
	baseQuota := summary.Quota
	providerId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderId)
	providerOwnerUserId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderOwnerUserId)
	providerPublicModel := common.GetContextKeyString(ctx, constant.ContextKeyProviderPublicModel)
	if providerId > 0 && providerPublicModel != "" {
		providerQuota, importCostQuota, _ := ApplyProviderPricingQuota(ctx, baseQuota, relayInfo.PriceData.UsePrice, summary.GroupRatio, summary.TotalTokens)
		summary.Quota = providerQuota
		if summary.Quota < 0 {
			summary.Quota = 0
		}
		common.SetContextKey(ctx, constant.ContextKeyProviderBaseQuota, importCostQuota)
		common.SetContextKey(ctx, constant.ContextKeyProviderUserQuota, summary.Quota)
	}

	if summary.WebSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Web Search 调用 %d 次，调用花费 %s", summary.WebSearchCallCount, decimal.NewFromFloat(summary.WebSearchPrice).Mul(decimal.NewFromInt(int64(summary.WebSearchCallCount))).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.ClaudeWebSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Claude Web Search 调用 %d 次，调用花费 %s", summary.ClaudeWebSearchCallCount, decimal.NewFromFloat(summary.ClaudeWebSearchPrice).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Mul(decimal.NewFromInt(int64(summary.ClaudeWebSearchCallCount))).String()))
	}
	if summary.FileSearchCallCount > 0 {
		extraContent = append(extraContent, fmt.Sprintf("File Search 调用 %d 次，调用花费 %s", summary.FileSearchCallCount, decimal.NewFromFloat(summary.FileSearchPrice).Mul(decimal.NewFromInt(int64(summary.FileSearchCallCount))).Div(decimal.NewFromInt(1000)).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.AudioInputPrice > 0 && summary.AudioTokens > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Audio Input 花费 %s", decimal.NewFromFloat(summary.AudioInputPrice).Div(decimal.NewFromInt(1000000)).Mul(decimal.NewFromInt(int64(summary.AudioTokens))).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}
	if summary.ImageGenerationCallPrice > 0 {
		extraContent = append(extraContent, fmt.Sprintf("Image Generation Call 花费 %s", decimal.NewFromFloat(summary.ImageGenerationCallPrice).Mul(decimal.NewFromFloat(summary.GroupRatio)).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).String()))
	}

	if summary.TotalTokens == 0 {
		extraContent = append(extraContent, "上游没有返回计费信息，无法扣费（可能是上游超时）")
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, summary.ModelName, relayInfo.FinalPreConsumedQuota))
	} else {
		if relayInfo.BillingSource == BillingSourceSubscription {
			model.UpdateUserRequestCount(relayInfo.UserId, 1)
		} else {
			model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, summary.Quota)
		}
		RecordRelayTotalTokenUsage(relayInfo, summary.TotalTokens)
		channelQuota := summary.Quota
		if providerId > 0 {
			channelQuota = baseQuota
		}
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, channelQuota)
	}

	if err := SettleBilling(ctx, relayInfo, summary.Quota); err != nil {
		logger.LogError(ctx, "error settling billing: "+err.Error())
	}
	providerOwnerCostQuota := common.GetContextKeyInt(ctx, constant.ContextKeyProviderOwnerCost)
	if providerId > 0 && providerOwnerUserId > 0 && providerOwnerCostQuota > 0 && summary.TotalTokens > 0 {
		model.UpdateUserUsedQuotaAndRequestCount(providerOwnerUserId, providerOwnerCostQuota)
	}

	logModel := summary.ModelName
	if strings.HasPrefix(logModel, "gpt-4-gizmo") {
		logModel = "gpt-4-gizmo-*"
		extraContent = append(extraContent, fmt.Sprintf("模型 %s", summary.ModelName))
	}
	if strings.HasPrefix(logModel, "gpt-4o-gizmo") {
		logModel = "gpt-4o-gizmo-*"
		extraContent = append(extraContent, fmt.Sprintf("模型 %s", summary.ModelName))
	}

	logContent := strings.Join(extraContent, ", ")
	displayModelRatio := summary.ModelRatio
	displayModelPrice := summary.ModelPrice
	if providerId > 0 && providerPublicModel != "" {
		displayModelRatio, displayModelPrice, _ = ApplyProviderPricingDisplay(ctx, summary.ModelRatio, summary.ModelPrice)
	}
	var other map[string]interface{}
	if summary.IsClaudeUsageSemantic {
		other = GenerateClaudeOtherInfo(ctx, relayInfo,
			displayModelRatio, summary.GroupRatio, summary.CompletionRatio,
			summary.CacheTokens, summary.CacheRatio,
			summary.CacheCreationTokens, summary.CacheCreationRatio,
			summary.CacheCreationTokens5m, summary.CacheCreationRatio5m,
			summary.CacheCreationTokens1h, summary.CacheCreationRatio1h,
			displayModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
		other["usage_semantic"] = "anthropic"
	} else {
		other = GenerateTextOtherInfo(ctx, relayInfo, displayModelRatio, summary.GroupRatio, summary.CompletionRatio, summary.CacheTokens, summary.CacheRatio, displayModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	}
	if adminRejectReason != "" {
		other["reject_reason"] = adminRejectReason
	}
	if providerId > 0 {
		providerBaseQuota := common.GetContextKeyInt(ctx, constant.ContextKeyProviderBaseQuota)
		other["provider_id"] = providerId
		other["provider_public_model"] = providerPublicModel
		other["provider_base_model"] = relayInfo.OriginModelName
		other["provider_base_quota"] = providerBaseQuota
		other["provider_user_quota"] = summary.Quota
		other["provider_import_price_ratio"] = common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderImportPriceRatio)
		other["provider_pricing_type"] = common.GetContextKeyString(ctx, constant.ContextKeyProviderPricingType)
		other["provider_base_model_ratio"] = summary.ModelRatio
		other["provider_base_model_price"] = summary.ModelPrice
		other["provider_paid_quota"] = common.GetContextKeyInt(ctx, constant.ContextKeyProviderPaidQuota)
		other["provider_covered_cost_quota"] = common.GetContextKeyInt(ctx, constant.ContextKeyProviderCoveredCost)
		other["provider_owner_cost_quota"] = providerOwnerCostQuota
		other["provider_profit_quota"] = common.GetContextKeyInt(ctx, constant.ContextKeyProviderProfitQuota)
		other["billing_side"] = "provider_user"
	}
	if summary.ImageTokens != 0 {
		other["image"] = true
		other["image_ratio"] = summary.ImageRatio
		other["image_output"] = summary.ImageTokens
	}
	if summary.WebSearchCallCount > 0 {
		other["web_search"] = true
		other["web_search_call_count"] = summary.WebSearchCallCount
		other["web_search_price"] = summary.WebSearchPrice
	} else if summary.ClaudeWebSearchCallCount > 0 {
		other["web_search"] = true
		other["web_search_call_count"] = summary.ClaudeWebSearchCallCount
		other["web_search_price"] = summary.ClaudeWebSearchPrice
	}
	if summary.FileSearchCallCount > 0 {
		other["file_search"] = true
		other["file_search_call_count"] = summary.FileSearchCallCount
		other["file_search_price"] = summary.FileSearchPrice
	}
	if summary.AudioInputPrice > 0 && summary.AudioTokens > 0 {
		other["audio_input_seperate_price"] = true
		other["audio_input_token_count"] = summary.AudioTokens
		other["audio_input_price"] = summary.AudioInputPrice
	}
	if summary.ImageGenerationCallPrice > 0 {
		other["image_generation_call"] = true
		other["image_generation_call_price"] = summary.ImageGenerationCallPrice
	}
	if summary.CacheCreationTokens > 0 {
		other["cache_creation_tokens"] = summary.CacheCreationTokens
		other["cache_creation_ratio"] = summary.CacheCreationRatio
	}
	if summary.CacheCreationTokens5m > 0 {
		other["cache_creation_tokens_5m"] = summary.CacheCreationTokens5m
		other["cache_creation_ratio_5m"] = summary.CacheCreationRatio5m
	}
	if summary.CacheCreationTokens1h > 0 {
		other["cache_creation_tokens_1h"] = summary.CacheCreationTokens1h
		other["cache_creation_ratio_1h"] = summary.CacheCreationRatio1h
	}
	cacheWriteTokens := cacheWriteTokensTotal(summary)
	if cacheWriteTokens > 0 {
		// cache_write_tokens: normalized cache creation total for UI display.
		// If split 5m/1h values are present, this is their sum; otherwise it falls back
		// to cache_creation_tokens.
		other["cache_write_tokens"] = cacheWriteTokens
	}
	if relayInfo.GetFinalRequestRelayFormat() != types.RelayFormatClaude && usage != nil && usage.UsageSource != "" && usage.InputTokens > 0 {
		// input_tokens_total: explicit normalized total input used by the usage log UI.
		// Only write this field when upstream/current conversion has already provided a
		// reliable total input value and tagged the usage source. Do not infer it from
		// prompt/cache fields here, otherwise old upstream payloads may be double-counted.
		other["input_tokens_total"] = usage.InputTokens
	}
	attachQuotaSaturation(ctx, relayInfo, other)

	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     summary.PromptTokens,
		CompletionTokens: summary.CompletionTokens,
		ModelName:        logModel,
		TokenName:        summary.TokenName,
		Quota:            summary.Quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(summary.UseTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
	if providerId > 0 && providerOwnerUserId > 0 && providerOwnerCostQuota > 0 {
		costOther := map[string]interface{}{
			"provider_id":                 providerId,
			"billing_side":                "provider_cost",
			"provider_public_model":       providerPublicModel,
			"provider_user_id":            relayInfo.UserId,
			"provider_user_quota":         summary.Quota,
			"provider_base_quota":         common.GetContextKeyInt(ctx, constant.ContextKeyProviderBaseQuota),
			"provider_paid_quota":         common.GetContextKeyInt(ctx, constant.ContextKeyProviderPaidQuota),
			"provider_covered_cost_quota": common.GetContextKeyInt(ctx, constant.ContextKeyProviderCoveredCost),
			"provider_owner_cost_quota":   providerOwnerCostQuota,
			"provider_profit_quota":       common.GetContextKeyInt(ctx, constant.ContextKeyProviderProfitQuota),
		}
		model.RecordConsumeLog(ctx, providerOwnerUserId, model.RecordConsumeLogParams{
			ChannelId:        relayInfo.ChannelId,
			PromptTokens:     summary.PromptTokens,
			CompletionTokens: summary.CompletionTokens,
			ModelName:        relayInfo.OriginModelName,
			TokenName:        "provider-owner",
			Quota:            providerOwnerCostQuota,
			Content:          "provider upstream cost",
			TokenId:          0,
			UseTimeSeconds:   int(summary.UseTimeSeconds),
			IsStream:         relayInfo.IsStream,
			Group:            relayInfo.UsingGroup,
			Other:            costOther,
		})
	}
}
