package service

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type TokenDetails struct {
	TextTokens  int
	AudioTokens int
}

type QuotaInfo struct {
	InputDetails  TokenDetails
	OutputDetails TokenDetails
	ModelName     string
	UsePrice      bool
	ModelPrice    float64
	ModelRatio    float64
	GroupRatio    float64
	// UserDiscount 用户模型专属折扣 (0,1]，0/1 均视为不打折。
	// 仅余额支付时由调用方传入；音频/WSS 计费无缓存部分，
	// 折扣作用于全部输入输出 token；UsePrice（按次）不打折。
	UserDiscount float64
}

func RecordRelayTotalTokenUsage(relayInfo *relaycommon.RelayInfo, totalTokens int) {
	if relayInfo == nil || relayInfo.IsPlayground || totalTokens <= 0 {
		return
	}
	model.UpdateUserAndTokenTotalTokenUsed(relayInfo.UserId, relayInfo.TokenId, totalTokens)
}

func hasCustomModelRatio(modelName string, currentRatio float64) bool {
	defaultRatio, exists := ratio_setting.GetDefaultModelRatioMap()[modelName]
	if !exists {
		return true
	}
	return currentRatio != defaultRatio
}

// effectiveUserDiscount 返回本次请求生效的用户专属折扣：
// 1）折扣钳制到 (0,1]，未配置（<=0 或 ==1）返回 1；
// 2）订阅支付（BillingSource == "subscription"）不参与折扣，直接返回 1；
// 3）其余（含空串历史路径）按余额支付处理，折扣生效。
func effectiveUserDiscount(relayInfo *relaycommon.RelayInfo) float64 {
	if relayInfo == nil {
		return 1
	}
	if relayInfo.BillingSource == BillingSourceSubscription {
		return 1
	}
	return relayInfo.PriceData.EffectiveUserDiscount()
}

func calculateAudioQuota(info QuotaInfo) (int, *common.QuotaClamp) {
	if info.UsePrice {
		modelPrice := decimal.NewFromFloat(info.ModelPrice)
		quotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		groupRatio := decimal.NewFromFloat(info.GroupRatio)

		quota := modelPrice.Mul(quotaPerUnit).Mul(groupRatio)
		return common.QuotaFromDecimalChecked(quota)
	}

	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(info.ModelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(info.ModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(info.ModelName))

	groupRatio := decimal.NewFromFloat(info.GroupRatio)
	modelRatio := decimal.NewFromFloat(info.ModelRatio)
	ratio := groupRatio.Mul(modelRatio)

	inputTextTokens := decimal.NewFromInt(int64(info.InputDetails.TextTokens))
	outputTextTokens := decimal.NewFromInt(int64(info.OutputDetails.TextTokens))
	inputAudioTokens := decimal.NewFromInt(int64(info.InputDetails.AudioTokens))
	outputAudioTokens := decimal.NewFromInt(int64(info.OutputDetails.AudioTokens))

	quota := decimal.Zero
	quota = quota.Add(inputTextTokens)
	quota = quota.Add(outputTextTokens.Mul(completionRatio))
	quota = quota.Add(inputAudioTokens.Mul(audioRatio))
	quota = quota.Add(outputAudioTokens.Mul(audioRatio).Mul(audioCompletionRatio))

	// 用户专属折扣：只作用于输入输出 token（音频路径无缓存部分），
	// UsePrice（按次）不打折；仅余额支付时由调用方传入非 1 折扣
	if !info.UsePrice && info.UserDiscount > 0 && info.UserDiscount < 1 {
		quota = quota.Mul(decimal.NewFromFloat(info.UserDiscount))
	}

	quota = quota.Mul(ratio)

	// If ratio is not zero and quota is less than or equal to zero, set quota to 1
	if !ratio.IsZero() && quota.LessThanOrEqual(decimal.Zero) {
		quota = decimal.NewFromInt(1)
	}

	return common.QuotaFromDecimalChecked(quota)
}

func PreWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.RealtimeUsage) error {
	if relayInfo == nil || usage == nil {
		return errors.New("invalid realtime billing arguments")
	}
	if relayInfo.UsePrice {
		return nil
	}
	// A regular request has already reserved the initial estimate through its
	// BillingSession.  Realtime usage arrives in several chunks; when that
	// session supports progress settlement, apply each chunk as an incremental
	// target instead of calling the legacy PostConsumeQuota path (which would
	// charge the same request a second time).  Keep the old path for callers
	// that deliberately do not create a BillingSession.
	progressSettler, hasBillingSession := relayInfo.Billing.(interface {
		SettleProgress(int) error
	})

	var userQuota int
	var err error
	if relayInfo.BillingSource != BillingSourceSubscription {
		userQuota, err = model.GetUserQuota(relayInfo.UserId, false)
		if err != nil {
			return err
		}
	}

	token, err := model.GetTokenByKey(strings.TrimPrefix(relayInfo.TokenKey, "sk-"), false)
	if err != nil {
		return err
	}

	modelName := relayInfo.OriginModelName
	textInputTokens := usage.InputTokenDetails.TextTokens
	textOutTokens := usage.OutputTokenDetails.TextTokens
	audioInputTokens := usage.InputTokenDetails.AudioTokens
	audioOutTokens := usage.OutputTokenDetails.AudioTokens
	groupRatio := ratio_setting.GetGroupRatio(relayInfo.UsingGroup)
	modelRatio, _, _ := ratio_setting.GetModelRatio(modelName)

	autoGroup, exists := common.GetContextKey(ctx, constant.ContextKeyAutoGroup)
	if exists {
		groupRatio = ratio_setting.GetGroupRatio(autoGroup.(string))
		log.Printf("final group ratio: %f", groupRatio)
		relayInfo.UsingGroup = autoGroup.(string)
	}

	actualGroupRatio := groupRatio
	userGroupRatio, ok := ratio_setting.GetGroupGroupRatio(relayInfo.UserGroup, relayInfo.UsingGroup)
	if ok {
		actualGroupRatio = userGroupRatio
	}

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  modelName,
		UsePrice:   relayInfo.UsePrice,
		ModelRatio: modelRatio,
		GroupRatio: actualGroupRatio,

		UserDiscount: func() float64 {
			if relayInfo.ProviderId > 0 {
				return 1
			}
			return effectiveUserDiscount(relayInfo)
		}(),
	}

	quota, clamp := calculateAudioQuota(quotaInfo)
	var tieredResult *billingexpr.TieredResult
	var tieredParams billingexpr.TokenParams
	if snap := relayInfo.TieredBillingSnapshot; snap != nil {
		usedVars := billingexpr.UsedVars(snap.ExprString)
		tieredParams = BuildTieredRealtimeTokenParams(usage, usedVars)
		if ok, tieredQuota, result := TryTieredSettle(relayInfo, tieredParams); ok {
			quota = tieredQuota
			tieredResult = result
			clamp = nil
			if result != nil {
				clamp = result.Clamp
			}
		}
	}
	if relayInfo.ProviderId > 0 {
		cacheQuota := decimal.Zero
		discountableQuota := decimal.NewFromInt(int64(quota))
		discountableTokens := usage.TotalTokens
		nonCacheTokens := usage.TotalTokens
		if tieredResult != nil {
			cacheQuota = decimal.NewFromFloat(tieredResult.CacheQuotaBeforeGroup).Mul(decimal.NewFromFloat(actualGroupRatio))
			discountableQuota = decimal.NewFromFloat(tieredResult.DiscountableQuotaBeforeGroup).Mul(decimal.NewFromFloat(actualGroupRatio))
			discountableTokens = int(tieredResult.DiscountableTokens)
			nonCacheTokens = tieredNonCacheTokenCount(usage.TotalTokens, tieredParams)
		}
		if providerQuota, _, applied := ApplyProviderPricingQuota(ctx, quota, cacheQuota, relayInfo.PriceData.UsePrice, actualGroupRatio, usage.TotalTokens, nonCacheTokens); applied {
			quota = applyProviderUserDiscount(ctx, relayInfo, providerQuota, discountableQuota, relayInfo.PriceData.UsePrice, actualGroupRatio, discountableTokens)
		}
	}
	noteQuotaClamp(relayInfo, clamp)

	if relayInfo.BillingSource != BillingSourceSubscription && userQuota < quota {
		return fmt.Errorf("user quota is not enough, user quota: %s, need quota: %s", logger.FormatQuota(userQuota), logger.FormatQuota(quota))
	}

	// Subscription funding is independent of the per-token numeric allowance.
	// Keep loading the token above (the request was authenticated and the row
	// must still exist), but do not reject a valid subscription chunk merely
	// because RemainQuota has reached zero.  Wallet/legacy callers retain the
	// original token quota guard.
	if relayInfo.BillingSource != BillingSourceSubscription && !token.UnlimitedQuota && token.RemainQuota < quota {
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(token.RemainQuota), logger.FormatQuota(quota))
	}

	if hasBillingSession {
		err = progressSettler.SettleProgress(quota)
	} else {
		err = PostConsumeQuota(relayInfo, quota, 0, false)
	}
	if err != nil {
		return err
	}
	logger.LogInfo(ctx, "realtime streaming consume quota success, quota: "+fmt.Sprintf("%d", quota))
	return nil
}

// wss 大模型消耗处理逻辑
func PostWssConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelName string,
	usage *dto.RealtimeUsage, extraContent string) {
	var tieredResult *billingexpr.TieredResult
	var tieredUsedVars map[string]bool
	if snap := relayInfo.TieredBillingSnapshot; snap != nil {
		tieredUsedVars = billingexpr.UsedVars(snap.ExprString)
	}
	tieredParams := BuildTieredRealtimeTokenParams(usage, tieredUsedVars)
	tieredOk, tieredQuota, tieredRes := TryTieredSettle(relayInfo, tieredParams)
	if tieredOk {
		tieredResult = tieredRes
	}

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	textInputTokens := usage.InputTokenDetails.TextTokens
	textOutTokens := usage.OutputTokenDetails.TextTokens

	audioInputTokens := usage.InputTokenDetails.AudioTokens
	audioOutTokens := usage.OutputTokenDetails.AudioTokens

	tokenName := ctx.GetString("token_name")
	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(modelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(relayInfo.OriginModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(modelName))

	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	usePrice := relayInfo.PriceData.UsePrice

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  modelName,
		UsePrice:   usePrice,
		ModelRatio: modelRatio,
		GroupRatio: groupRatio,

		UserDiscount: func() float64 {
			if relayInfo.ProviderId > 0 {
				return 1
			}
			return effectiveUserDiscount(relayInfo)
		}(),
	}

	quota, clamp := calculateAudioQuota(quotaInfo)
	if tieredOk {
		quota = tieredQuota
		clamp = nil
		if tieredResult != nil {
			clamp = tieredResult.Clamp
		}
	}
	noteQuotaClamp(relayInfo, clamp)

	totalTokens := usage.TotalTokens
	baseQuota := quota
	providerId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderId)
	providerOwnerUserId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderOwnerUserId)
	if totalTokens != 0 {
		cacheQuota := decimal.Zero
		discountableQuota := decimal.NewFromInt(int64(baseQuota))
		discountableTokens := totalTokens
		nonCacheTokens := totalTokens
		if tieredResult != nil {
			cacheQuota = decimal.NewFromFloat(tieredResult.CacheQuotaBeforeGroup).Mul(decimal.NewFromFloat(groupRatio))
			discountableQuota = decimal.NewFromFloat(tieredResult.DiscountableQuotaBeforeGroup).Mul(decimal.NewFromFloat(groupRatio))
			discountableTokens = int(tieredResult.DiscountableTokens)
			nonCacheTokens = tieredNonCacheTokenCount(totalTokens, tieredParams)
		}
		if providerQuota, importCostQuota, applied := ApplyProviderPricingQuota(ctx, baseQuota, cacheQuota, usePrice, groupRatio, totalTokens, nonCacheTokens); applied {
			quota = applyProviderUserDiscount(ctx, relayInfo, providerQuota, discountableQuota, usePrice, groupRatio, discountableTokens)
			common.SetContextKey(ctx, constant.ContextKeyProviderBaseQuota, importCostQuota)
			common.SetContextKey(ctx, constant.ContextKeyProviderUserQuota, quota)
		}
	}
	var logContent string
	if !usePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，音频倍率 %.2f，音频补全倍率 %.2f，分组倍率 %.2f",
			modelRatio, completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), groupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", modelPrice, groupRatio)
	}

	// record all the consume log even if quota is 0
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
		logContent += fmt.Sprintf("（可能是上游超时）")
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, modelName, relayInfo.FinalPreConsumedQuota))
	} else {
		if relayInfo.BillingSource == BillingSourceSubscription {
			model.UpdateUserRequestCount(relayInfo.UserId, 1)
		} else {
			model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		}
		RecordRelayTotalTokenUsage(relayInfo, totalTokens)
		channelQuota := quota
		if providerId > 0 {
			channelQuota = baseQuota
		}
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, channelQuota)
	}
	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		logger.LogError(ctx, "error settling billing: "+err.Error())
		if relayInfo.Billing != nil && relayInfo.Billing.NeedsRefund() {
			relayInfo.Billing.Refund(ctx)
		}
	}
	providerOwnerCostQuota := common.GetContextKeyInt(ctx, constant.ContextKeyProviderOwnerCost)
	if providerId > 0 && providerOwnerUserId > 0 && providerOwnerCostQuota > 0 && totalTokens > 0 {
		model.UpdateUserUsedQuotaAndRequestCount(providerOwnerUserId, providerOwnerCostQuota)
	}

	logModel := modelName
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	displayModelRatio := modelRatio
	displayModelPrice := modelPrice
	if providerId > 0 && common.GetContextKeyString(ctx, constant.ContextKeyProviderPublicModel) != "" {
		displayModelRatio, displayModelPrice, _ = ApplyProviderPricingDisplay(ctx, modelRatio, modelPrice)
	}
	other := GenerateWssOtherInfo(ctx, relayInfo, usage, displayModelRatio, groupRatio,
		completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), displayModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	if tieredOk {
		InjectTieredBillingInfo(ctx, other, relayInfo, tieredResult)
	}
	attachQuotaSaturation(ctx, relayInfo, other)
	if providerId > 0 {
		other["provider_import_price_ratio"] = common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderImportPriceRatio)
		other["provider_pricing_type"] = common.GetContextKeyString(ctx, constant.ContextKeyProviderPricingType)
		other["provider_base_model_ratio"] = modelRatio
		other["provider_base_model_price"] = modelPrice
	}
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(useTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
}

func CalcOpenRouterCacheCreateTokens(usage dto.Usage, priceData types.PriceData) int {
	if priceData.CacheCreationRatio == 1 {
		return 0
	}
	quotaPrice := priceData.ModelRatio / common.QuotaPerUnit
	promptCacheCreatePrice := quotaPrice * priceData.CacheCreationRatio
	promptCacheReadPrice := quotaPrice * priceData.CacheRatio
	completionPrice := quotaPrice * priceData.CompletionRatio

	cost, _ := usage.Cost.(float64)
	totalPromptTokens := float64(usage.PromptTokens)
	completionTokens := float64(usage.CompletionTokens)
	promptCacheReadTokens := float64(usage.PromptTokensDetails.CachedTokens)

	return int(math.Round((cost -
		totalPromptTokens*quotaPrice +
		promptCacheReadTokens*(quotaPrice-promptCacheReadPrice) -
		completionTokens*completionPrice) /
		(promptCacheCreatePrice - quotaPrice)))
}

// 音频模型消耗逻辑
func PostAudioConsumeQuota(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, extraContent string) {

	var tieredUsedVars map[string]bool
	if snap := relayInfo.TieredBillingSnapshot; snap != nil {
		tieredUsedVars = billingexpr.UsedVars(snap.ExprString)
	}
	var tieredResult *billingexpr.TieredResult
	tieredParams := BuildTieredTokenParams(usage, false, tieredUsedVars)
	tieredOk, tieredQuota, tieredRes := TryTieredSettle(relayInfo, tieredParams)
	if tieredOk {
		tieredResult = tieredRes
	}

	useTimeSeconds := time.Now().Unix() - relayInfo.StartTime.Unix()
	textInputTokens := usage.PromptTokensDetails.TextTokens
	textOutTokens := usage.CompletionTokenDetails.TextTokens

	audioInputTokens := usage.PromptTokensDetails.AudioTokens
	audioOutTokens := usage.CompletionTokenDetails.AudioTokens

	tokenName := ctx.GetString("token_name")
	completionRatio := decimal.NewFromFloat(ratio_setting.GetCompletionRatio(relayInfo.OriginModelName))
	audioRatio := decimal.NewFromFloat(ratio_setting.GetAudioRatio(relayInfo.OriginModelName))
	audioCompletionRatio := decimal.NewFromFloat(ratio_setting.GetAudioCompletionRatio(relayInfo.OriginModelName))

	modelRatio := relayInfo.PriceData.ModelRatio
	groupRatio := relayInfo.PriceData.GroupRatioInfo.GroupRatio
	modelPrice := relayInfo.PriceData.ModelPrice
	usePrice := relayInfo.PriceData.UsePrice

	quotaInfo := QuotaInfo{
		InputDetails: TokenDetails{
			TextTokens:  textInputTokens,
			AudioTokens: audioInputTokens,
		},
		OutputDetails: TokenDetails{
			TextTokens:  textOutTokens,
			AudioTokens: audioOutTokens,
		},
		ModelName:  relayInfo.OriginModelName,
		UsePrice:   usePrice,
		ModelRatio: modelRatio,
		GroupRatio: groupRatio,

		UserDiscount: func() float64 {
			if relayInfo.ProviderId > 0 {
				return 1
			}
			return effectiveUserDiscount(relayInfo)
		}(),
	}

	quota, clamp := calculateAudioQuota(quotaInfo)
	if tieredOk {
		quota = tieredQuota
		clamp = nil
		if tieredResult != nil {
			clamp = tieredResult.Clamp
		}
	}
	noteQuotaClamp(relayInfo, clamp)

	totalTokens := usage.TotalTokens
	baseQuota := quota
	providerId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderId)
	providerOwnerUserId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderOwnerUserId)
	if totalTokens != 0 {
		cacheQuota := decimal.Zero
		discountableQuota := decimal.NewFromInt(int64(baseQuota))
		discountableTokens := totalTokens
		nonCacheTokens := totalTokens
		if tieredResult != nil {
			cacheQuota = decimal.NewFromFloat(tieredResult.CacheQuotaBeforeGroup).Mul(decimal.NewFromFloat(groupRatio))
			discountableQuota = decimal.NewFromFloat(tieredResult.DiscountableQuotaBeforeGroup).Mul(decimal.NewFromFloat(groupRatio))
			discountableTokens = int(tieredResult.DiscountableTokens)
			nonCacheTokens = tieredNonCacheTokenCount(totalTokens, tieredParams)
		}
		if providerQuota, importCostQuota, applied := ApplyProviderPricingQuota(ctx, baseQuota, cacheQuota, usePrice, groupRatio, totalTokens, nonCacheTokens); applied {
			quota = applyProviderUserDiscount(ctx, relayInfo, providerQuota, discountableQuota, usePrice, groupRatio, discountableTokens)
			common.SetContextKey(ctx, constant.ContextKeyProviderBaseQuota, importCostQuota)
			common.SetContextKey(ctx, constant.ContextKeyProviderUserQuota, quota)
		}
	}
	var logContent string
	if !usePrice {
		logContent = fmt.Sprintf("模型倍率 %.2f，补全倍率 %.2f，音频倍率 %.2f，音频补全倍率 %.2f，分组倍率 %.2f",
			modelRatio, completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), groupRatio)
	} else {
		logContent = fmt.Sprintf("模型价格 %.2f，分组倍率 %.2f", modelPrice, groupRatio)
	}

	// record all the consume log even if quota is 0
	if totalTokens == 0 {
		// in this case, must be some error happened
		// we cannot just return, because we may have to return the pre-consumed quota
		quota = 0
		logContent += fmt.Sprintf("（可能是上游超时）")
		logger.LogError(ctx, fmt.Sprintf("total tokens is 0, cannot consume quota, userId %d, channelId %d, "+
			"tokenId %d, model %s， pre-consumed quota %d", relayInfo.UserId, relayInfo.ChannelId, relayInfo.TokenId, relayInfo.OriginModelName, relayInfo.FinalPreConsumedQuota))
	} else {
		if relayInfo.BillingSource == BillingSourceSubscription {
			model.UpdateUserRequestCount(relayInfo.UserId, 1)
		} else {
			model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, quota)
		}
		RecordRelayTotalTokenUsage(relayInfo, totalTokens)
		channelQuota := quota
		if providerId > 0 {
			channelQuota = baseQuota
		}
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, channelQuota)
	}

	if err := SettleBilling(ctx, relayInfo, quota); err != nil {
		logger.LogError(ctx, "error settling billing: "+err.Error())
		if relayInfo.Billing != nil && relayInfo.Billing.NeedsRefund() {
			relayInfo.Billing.Refund(ctx)
		}
	}
	providerOwnerCostQuota := common.GetContextKeyInt(ctx, constant.ContextKeyProviderOwnerCost)
	if providerId > 0 && providerOwnerUserId > 0 && providerOwnerCostQuota > 0 && totalTokens > 0 {
		model.UpdateUserUsedQuotaAndRequestCount(providerOwnerUserId, providerOwnerCostQuota)
	}

	logModel := relayInfo.OriginModelName
	if extraContent != "" {
		logContent += ", " + extraContent
	}
	displayModelRatio := modelRatio
	displayModelPrice := modelPrice
	if providerId > 0 && common.GetContextKeyString(ctx, constant.ContextKeyProviderPublicModel) != "" {
		displayModelRatio, displayModelPrice, _ = ApplyProviderPricingDisplay(ctx, modelRatio, modelPrice)
	}
	other := GenerateAudioOtherInfo(ctx, relayInfo, usage, displayModelRatio, groupRatio,
		completionRatio.InexactFloat64(), audioRatio.InexactFloat64(), audioCompletionRatio.InexactFloat64(), displayModelPrice, relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)
	if tieredOk {
		InjectTieredBillingInfo(ctx, other, relayInfo, tieredResult)
	}
	attachQuotaSaturation(ctx, relayInfo, other)
	if providerId > 0 {
		other["provider_import_price_ratio"] = common.GetContextKeyFloat64(ctx, constant.ContextKeyProviderImportPriceRatio)
		other["provider_pricing_type"] = common.GetContextKeyString(ctx, constant.ContextKeyProviderPricingType)
		other["provider_base_model_ratio"] = modelRatio
		other["provider_base_model_price"] = modelPrice
	}
	model.RecordConsumeLog(ctx, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        relayInfo.ChannelId,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        logModel,
		TokenName:        tokenName,
		Quota:            quota,
		Content:          logContent,
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(useTimeSeconds),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
}

func PreConsumeTokenQuota(relayInfo *relaycommon.RelayInfo, quota int) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if relayInfo.IsPlayground {
		return nil
	}
	// Subscription allowance is a separate funding source.  It still requires
	// a valid token for identity/authentication, but must not reserve or consume
	// the token's numeric quota (which may legitimately be zero).
	if relayInfo.BillingSource == BillingSourceSubscription {
		return nil
	}
	//if relayInfo.TokenUnlimited {
	//	return nil
	//}
	token, err := model.GetTokenByKey(relayInfo.TokenKey, false)
	if err != nil {
		return err
	}
	if !relayInfo.TokenUnlimited && token.RemainQuota < quota {
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s", logger.FormatQuota(token.RemainQuota), logger.FormatQuota(quota))
	}
	err = model.DecreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
	if err != nil {
		return err
	}
	return nil
}

func PostConsumeQuota(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int, sendEmail bool) (err error) {

	// 1) Consume from wallet quota OR subscription item
	if relayInfo != nil && relayInfo.BillingSource == BillingSourceSubscription {
		if relayInfo.SubscriptionId == 0 {
			return errors.New("subscription id is missing")
		}
		delta := int64(quota)
		if delta != 0 {
			var settleErr error
			requestAware := strings.TrimSpace(relayInfo.RequestId) != "" && relayInfo.SubscriptionPreConsumed > 0
			var actual int64
			if requestAware {
				// PostConsumeQuota receives incremental amounts on the compatibility
				// realtime path, but SettleBilling's fallback passes a final delta
				// relative to the original reservation (preConsumedQuota != 0).
				// Build an absolute target in both cases: accumulate prior chunks for
				// the former, while treating the latter as the already-cumulative final
				// amount so a final settlement cannot double-charge the stream.
				incremental := preConsumedQuota == 0
				actual = relayInfo.SubscriptionPreConsumed + delta
				if incremental {
					actual += relayInfo.SubscriptionPostDelta
				}
				if actual < 0 {
					actual = 0
				}
				settleErr = model.SettleSubscriptionPreConsume(relayInfo.RequestId, actual)
			} else {
				settleErr = model.PostConsumeUserSubscriptionDelta(relayInfo.SubscriptionId, delta)
			}
			if settleErr != nil {
				return settleErr
			}
			if requestAware && preConsumedQuota == 0 {
				// Keep the compatibility field synchronized with the absolute
				// target (including clamping at zero) so a negative adjustment or
				// retry cannot leave stale bookkeeping behind.
				relayInfo.SubscriptionPostDelta = actual - relayInfo.SubscriptionPreConsumed
			} else if requestAware {
				// The caller supplied the final absolute target as
				// preConsumed + delta; synchronize the compatibility field rather
				// than retaining stale per-chunk bookkeeping.
				relayInfo.SubscriptionPostDelta = delta
			}
		}
	} else {
		// Wallet
		if quota > 0 {
			//兼容旧的 PostConsumeQuota 路径：没有 BillingSession 的钱包扣费也改成奖励优先，并对充值额度消费部分触发返利。
			breakdown, decreaseErr := model.DecreaseUserQuotaPreferReward(relayInfo.UserId, quota)
			err = decreaseErr
			if err == nil {
				relayInfo.WalletRewardConsumed = breakdown.RewardUsed
				relayInfo.WalletPaidConsumed = breakdown.PaidUsed
			}
			// 同步请求直接触发消费返利；异步任务（ForcePreConsume）和违规扣费（DisableConsumeRebate）跳过。
			if err == nil && breakdown.PaidUsed > 0 && relayInfo.ProviderId <= 0 && !relayInfo.ForcePreConsume && !relayInfo.DisableConsumeRebate {
				rebateRequestId := fmt.Sprintf("%s:post:%s", relayInfo.RequestId, common.GetRandomString(8))
				if _, _, rebateErr := model.ApplyInviteConsumeRebate(relayInfo.UserId, rebateRequestId, breakdown.PaidUsed, consumeRebateContextFromRelay(nil, relayInfo)); rebateErr != nil {
					common.SysLog("error applying consume rebate: " + rebateErr.Error())
				}
			}
		} else {
			err = model.IncreaseUserQuota(relayInfo.UserId, -quota, false)
		}
		if err != nil {
			return err
		}
	}

	if !relayInfo.IsPlayground && relayInfo.BillingSource != BillingSourceSubscription {
		if quota > 0 {
			err = model.DecreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, quota)
		} else {
			err = model.IncreaseTokenQuota(relayInfo.TokenId, relayInfo.TokenKey, -quota)
		}
		if err != nil {
			return err
		}
	}

	if sendEmail {
		if (quota + preConsumedQuota) != 0 {
			checkAndSendQuotaNotify(relayInfo, quota, preConsumedQuota)
		}
	}

	return nil
}

func getProviderAwareConsoleBaseURL(providerId int) string {
	fallback := strings.TrimRight(system_setting.ServerAddress, "/")
	if providerId <= 0 {
		return fallback
	}
	domains, err := model.GetProviderVerifiedDomains(providerId)
	if err != nil || len(domains) == 0 {
		return fallback
	}
	scheme := "https"
	if strings.HasPrefix(strings.ToLower(fallback), "http://") {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, domains[0])
}

func checkAndSendQuotaNotify(relayInfo *relaycommon.RelayInfo, quota int, preConsumedQuota int) {
	gopool.Go(func() {
		userSetting := relayInfo.UserSetting
		threshold := common.QuotaRemindThreshold
		if userSetting.QuotaWarningThreshold != 0 {
			threshold = int(userSetting.QuotaWarningThreshold)
		}

		//noMoreQuota := userCache.Quota-(quota+preConsumedQuota) <= 0
		quotaTooLow := false
		consumeQuota := quota + preConsumedQuota
		if relayInfo.UserQuota-consumeQuota < threshold {
			quotaTooLow = true
		}
		if quotaTooLow {
			prompt := "您的额度即将用尽 / Quota Running Low"
			topUpLink := fmt.Sprintf("%s/console/topup", getProviderAwareConsoleBaseURL(relayInfo.ProviderId))

			// 纯文本内容，用于 Bark/Gotify 等非邮件渠道
			content := "{{value}}，剩余额度：{{value}}，请及时充值"
			values := []interface{}{prompt, logger.FormatQuota(relayInfo.UserQuota)}

			notify := dto.NewNotify(dto.NotifyTypeQuotaExceed, prompt, content, values)
			notify.TemplateName = "quota_warning.html"
			notify.TemplateData = map[string]any{
				"Quota":     logger.FormatQuota(relayInfo.UserQuota),
				"TopUpLink": topUpLink,
			}

			err := NotifyUser(relayInfo.UserId, relayInfo.UserEmail, relayInfo.UserSetting, notify)
			if err != nil {
				common.SysError(fmt.Sprintf("failed to send quota notify to user %d: %s", relayInfo.UserId, err.Error()))
			}
		}
	})
}

func checkAndSendSubscriptionQuotaNotify(relayInfo *relaycommon.RelayInfo) {
	gopool.Go(func() {
		if relayInfo == nil {
			return
		}
		if relayInfo.SubscriptionId == 0 || relayInfo.SubscriptionAmountTotal <= 0 {
			return
		}

		userSetting := relayInfo.UserSetting
		threshold := common.QuotaRemindThreshold
		if userSetting.QuotaWarningThreshold != 0 {
			threshold = int(userSetting.QuotaWarningThreshold)
		}

		usedAfter := relayInfo.SubscriptionAmountUsedAfterPreConsume + relayInfo.SubscriptionPostDelta
		remaining := relayInfo.SubscriptionAmountTotal - usedAfter
		if remaining >= int64(threshold) {
			return
		}

		prompt := "您的订阅额度即将用尽 / Subscription Quota Running Low"
		topUpLink := fmt.Sprintf("%s/console/topup", getProviderAwareConsoleBaseURL(relayInfo.ProviderId))

		// 纯文本内容，用于 Bark/Gotify 等非邮件渠道
		content := "{{value}}，剩余额度：{{value}}，请及时充值"
		values := []interface{}{prompt, logger.FormatQuota(int(remaining))}

		notify := dto.NewNotify(dto.NotifyTypeQuotaExceed, prompt, content, values)
		notify.TemplateName = "subscription_quota_warning.html"
		notify.TemplateData = map[string]any{
			"Quota":     logger.FormatQuota(int(remaining)),
			"TopUpLink": topUpLink,
		}

		if err := NotifyUser(relayInfo.UserId, relayInfo.UserEmail, relayInfo.UserSetting, notify); err != nil {
			common.SysError(fmt.Sprintf("failed to send subscription quota notify to user %d: %s", relayInfo.UserId, err.Error()))
		}
	})
}
