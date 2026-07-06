package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	BillingSourceWallet       = "wallet"
	BillingSourceSubscription = "subscription"
)

// PreConsumeBilling 根据用户计费偏好创建 BillingSession 并执行预扣费。
// 会话存储在 relayInfo.Billing 上，供后续 Settle / Refund 使用。
func PreConsumeBilling(c *gin.Context, preConsumedQuota int, relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	session, apiErr := NewBillingSession(c, relayInfo, preConsumedQuota)
	if apiErr != nil {
		return apiErr
	}
	relayInfo.Billing = session
	return nil
}

// ---------------------------------------------------------------------------
// SettleBilling — 后结算辅助函数
// ---------------------------------------------------------------------------

// SettleBilling 执行计费结算。如果 RelayInfo 上有 BillingSession 则通过 session 结算，
// 否则回退到旧的 PostConsumeQuota 路径（兼容按次计费等场景）。
func SettleBilling(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, actualQuota int) error {
	if relayInfo.Billing != nil {
		preConsumed := relayInfo.Billing.GetPreConsumedQuota()
		// 订阅计费保护：当 relay 成功但上游渠道返回的 actualQuota 为 0 时，
		// 保留预扣额度不退还（防止因上游计费统计不准确导致订阅配额被错误返还）。
		//
		// 典型场景：某些渠道在流式响应中未正确返回 usage/token 统计，
		// 导致 SettleBilling 收到 actualQuota=0。此时如果退还预扣配额，
		// 用户可无限免费使用。此保护确保至少按预扣值扣费。
		if relayInfo.BillingSource == BillingSourceSubscription && preConsumed > 0 && actualQuota == 0 {
			msg := fmt.Sprintf("subscription actual quota is 0 after successful relay, keep pre-consumed quota: %s", logger.FormatQuota(preConsumed))
			if ctx != nil {
				logger.LogWarn(ctx, msg)
			} else {
				common.SysLog(msg)
			}
			actualQuota = preConsumed
		}
		delta := actualQuota - preConsumed

		if delta > 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后补扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else if delta < 0 {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费后返还扣费：%s（实际消耗：%s，预扣费：%s）",
				logger.FormatQuota(-delta),
				logger.FormatQuota(actualQuota),
				logger.FormatQuota(preConsumed),
			))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("预扣费与实际消耗一致，无需调整：%s（按次计费）",
				logger.FormatQuota(actualQuota),
			))
		}

		if err := relayInfo.Billing.Settle(actualQuota); err != nil {
			return err
		}
		// 普通站点按充值额度消费返佣；服务商站点在利润结算中按利润返佣。
		// 订阅计费不会触发返佣。
		paidQuota := relayInfo.Billing.ClaimPaidConsumedForRebate()
		providerId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderId)
		if paidQuota > 0 && providerId <= 0 {
			if _, _, err := model.ApplyInviteConsumeRebate(relayInfo.UserId, relayInfo.RequestId, paidQuota, consumeRebateContextFromRelay(ctx, relayInfo)); err != nil {
				logger.LogError(ctx, "error applying consume rebate: "+err.Error())
			}
		}
		applyProviderProfitForRelay(ctx, relayInfo, paidQuota)

		// 发送额度通知（订阅计费使用订阅剩余额度）
		if actualQuota != 0 {
			if relayInfo.BillingSource == BillingSourceSubscription {
				checkAndSendSubscriptionQuotaNotify(relayInfo)
			} else {
				checkAndSendQuotaNotify(relayInfo, actualQuota-preConsumed, preConsumed)
			}
		}
		return nil
	}

	// 回退：无 BillingSession 时使用旧路径
	quotaDelta := actualQuota - relayInfo.FinalPreConsumedQuota
	if quotaDelta != 0 {
		return PostConsumeQuota(relayInfo, quotaDelta, relayInfo.FinalPreConsumedQuota, true)
	}
	return nil
}

func consumeRebateContextFromRelay(ctx *gin.Context, relayInfo *relaycommon.RelayInfo) *model.ConsumeRebateContext {
	providerId := 0
	providerPricingId := 0
	publicModel := ""
	baseModel := ""
	if ctx == nil {
		if relayInfo != nil {
			providerId = relayInfo.ProviderId
			providerPricingId = relayInfo.ProviderPricingId
			publicModel = relayInfo.ProviderPublicModel
			baseModel = relayInfo.ProviderBaseModel
		}
	} else {
		providerId = common.GetContextKeyInt(ctx, constant.ContextKeyProviderId)
		providerPricingId = common.GetContextKeyInt(ctx, constant.ContextKeyProviderPricingId)
		publicModel = common.GetContextKeyString(ctx, constant.ContextKeyProviderPublicModel)
		baseModel = common.GetContextKeyString(ctx, constant.ContextKeyProviderBaseModel)
	}
	if providerId <= 0 || publicModel == "" {
		return nil
	}
	if baseModel == "" && relayInfo != nil {
		baseModel = relayInfo.OriginModelName
	}
	return &model.ConsumeRebateContext{
		ProviderId:        providerId,
		ProviderPricingId: providerPricingId,
		PublicModelName:   publicModel,
		BaseModelName:     baseModel,
	}
}

func applyProviderProfitForRelay(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, paidQuota int) {
	if ctx == nil || relayInfo == nil {
		return
	}
	providerId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderId)
	ownerUserId := common.GetContextKeyInt(ctx, constant.ContextKeyProviderOwnerUserId)
	if providerId <= 0 || ownerUserId <= 0 || relayInfo.UserId <= 0 {
		return
	}
	baseCost := common.GetContextKeyInt(ctx, constant.ContextKeyProviderBaseQuota)
	providerCharge := common.GetContextKeyInt(ctx, constant.ContextKeyProviderUserQuota)
	if baseCost <= 0 {
		return
	}
	if providerCharge < 0 {
		providerCharge = 0
	}
	if paidQuota < 0 {
		paidQuota = 0
	}
	if providerCharge > 0 && paidQuota > providerCharge {
		paidQuota = providerCharge
	}

	coveredCost := 0
	profitQuota := 0
	if providerCharge > 0 && paidQuota > 0 {
		paidRatio := decimal.NewFromInt(int64(paidQuota)).Div(decimal.NewFromInt(int64(providerCharge)))
		coverableCost := baseCost
		if providerCharge < coverableCost {
			coverableCost = providerCharge
		}
		coveredCost = int(decimal.NewFromInt(int64(coverableCost)).Mul(paidRatio).IntPart())
		if grossProfit := providerCharge - baseCost; grossProfit > 0 {
			profitQuota = int(decimal.NewFromInt(int64(grossProfit)).Mul(paidRatio).IntPart())
		}
	}
	if coveredCost > baseCost {
		coveredCost = baseCost
	}
	ownerCost := baseCost - coveredCost
	if ownerCost < 0 {
		ownerCost = 0
	}

	record := &model.ProviderProfit{
		ProviderId:        providerId,
		OwnerUserId:       ownerUserId,
		ProviderUserId:    relayInfo.UserId,
		RequestId:         relayInfo.RequestId,
		PublicModelName:   common.GetContextKeyString(ctx, constant.ContextKeyProviderPublicModel),
		BaseModelName:     common.GetContextKeyString(ctx, constant.ContextKeyProviderBaseModel),
		ProviderUserQuota: providerCharge,
		BaseCostQuota:     baseCost,
		PaidQuota:         paidQuota,
		CoveredCostQuota:  coveredCost,
		OwnerCostQuota:    ownerCost,
		ProfitQuota:       profitQuota,
	}
	if record.BaseModelName == "" {
		record.BaseModelName = relayInfo.OriginModelName
	}
	result, err := model.ApplyProviderProfit(record)
	if err != nil {
		logger.LogError(ctx, "error applying provider profit: "+err.Error())
		return
	}
	profitQuota = record.ProfitQuota
	common.SetContextKey(ctx, constant.ContextKeyProviderPaidQuota, paidQuota)
	common.SetContextKey(ctx, constant.ContextKeyProviderCoveredCost, coveredCost)
	common.SetContextKey(ctx, constant.ContextKeyProviderOwnerCost, ownerCost)
	common.SetContextKey(ctx, constant.ContextKeyProviderProfitQuota, profitQuota)
	if result != nil && result.Applied {
		model.LogProviderProfit(record)
	}
}
