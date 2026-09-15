package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// taskTokenQuotaAdjuster is nil in production and exists solely as a narrow
// fault-injection seam for the async billing state-machine tests.  The real
// path delegates to model.DecreaseTokenQuota/IncreaseTokenQuota.
var taskTokenQuotaAdjuster func(context.Context, *model.Task, int) error

// taskBillingRecoveryMu serializes recovery passes within one process.  The
// durable checkpoint still provides restart safety; this mutex additionally
// prevents two goroutines (for example, a manually triggered recovery and the
// regular polling loop) from both reading the same unfinished side before
// either has persisted its completion flag.  Cross-process deployments should
// use the same database/task ownership mechanism as the polling loop; all
// underlying subscription request operations remain idempotent.
var taskBillingRecoveryMu sync.Mutex

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo) {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	// 支持任务仅按次计费
	if info.PriceData.BillingMode == "per_second" {
		logContent = fmt.Sprintf("%s, %s %.4f/s × %.2f", logContent, info.PriceData.Resolution, info.PriceData.UnitPrice, info.PriceData.UnitCount)
	} else if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		logContent = fmt.Sprintf("%s，按次计费", logContent)
	} else {
		if len(info.PriceData.OtherRatios) > 0 {
			var contents []string
			for key, ra := range info.PriceData.OtherRatios {
				if 1.0 != ra {
					contents = append(contents, fmt.Sprintf("%s: %.2f", key, ra))
				}
			}
			if len(contents) > 0 {
				logContent = fmt.Sprintf("%s, 计算参数：%s", logContent, strings.Join(contents, ", "))
			}
		}
	}
	other := make(map[string]interface{})
	other["is_task"] = true
	other["request_path"] = c.Request.URL.Path
	other["model_price"] = info.PriceData.ModelPrice
	if info.PriceData.BillingMode != "" {
		other["billing_mode"] = info.PriceData.BillingMode
		other["billing_unit"] = info.PriceData.BillingUnit
		other["resolution"] = info.PriceData.Resolution
		other["unit_price"] = info.PriceData.UnitPrice
		other["unit_count"] = info.PriceData.UnitCount
		other["output_count"] = info.PriceData.OutputCount
	}
	if info.PriceData.ModelRatio > 0 {
		other["model_ratio"] = info.PriceData.ModelRatio
	}
	other["group_ratio"] = info.PriceData.GroupRatioInfo.GroupRatio
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	if info.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = info.UpstreamModelName
	}
	model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
		ChannelId: info.ChannelId,
		ModelName: info.OriginModelName,
		TokenName: tokenName,
		Quota:     info.PriceData.Quota,
		Content:   logContent,
		TokenId:   info.TokenId,
		Group:     info.UsingGroup,
		Other:     other,
	})
	if info.BillingSource == BillingSourceSubscription {
		model.UpdateUserRequestCount(info.UserId, 1)
	} else {
		model.UpdateUserUsedQuotaAndRequestCount(info.UserId, info.PriceData.Quota)
	}
	model.UpdateChannelUsedQuota(info.ChannelId, info.PriceData.Quota)
}

// ---------------------------------------------------------------------------
// 异步任务计费辅助函数
// ---------------------------------------------------------------------------

// resolveTokenKey 通过 TokenId 运行时获取令牌 Key（用于 Redis 缓存操作）。
// 如果令牌已被删除或查询失败，返回空字符串。
func resolveTokenKey(ctx context.Context, tokenId int, taskID string) string {
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("获取令牌 key 失败 (tokenId=%d, task=%s): %s", tokenId, taskID, err.Error()))
		return ""
	}
	return token.Key
}

// taskIsSubscription 判断任务是否通过订阅计费。
func taskIsSubscription(task *model.Task) bool {
	return task != nil && task.PrivateData.BillingSource == BillingSourceSubscription && task.PrivateData.SubscriptionId > 0
}

// taskTokenQuotaRequired mirrors the synchronous BillingSession rule for
// asynchronous tasks. Subscription-funded requests reserve and settle their
// allowance in subscription_pre_consume_records; the API token's numeric
// quota is only an authentication credential and must remain untouched.
func taskTokenQuotaRequired(task *model.Task) bool {
	return task != nil && task.PrivateData.BillingSource != BillingSourceSubscription &&
		task.PrivateData.TokenId > 0
}

// taskRefundQuota returns the amount that was actually reserved for an
// asynchronous task failure refund.  Subscription requests are pre-consumed
// with at least one quota unit so that a zero-priced/rounded request still
// consumes a subscription allowance.  SettleBilling intentionally keeps that
// reservation when the submitted task reports actualQuota == 0, while the
// task row retains the raw result.Quota (zero).  In that case the persisted
// subscription snapshot is the only available refund amount.
func taskRefundQuota(task *model.Task) int {
	if task == nil {
		return 0
	}
	if task.Quota > 0 {
		return task.Quota
	}
	if !taskIsSubscription(task) || task.PrivateData.SubscriptionPreConsumed <= 0 {
		return 0
	}
	// Quota is an int for historical task storage.  Clamp an out-of-range
	// snapshot rather than allowing an implementation-dependent conversion.
	maxInt := int64(^uint(0) >> 1)
	if task.PrivateData.SubscriptionPreConsumed > maxInt {
		return int(maxInt)
	}
	return int(task.PrivateData.SubscriptionPreConsumed)
}

func RecordTaskTotalTokenUsage(ctx context.Context, task *model.Task, totalTokens int) {
	if task == nil || totalTokens <= 0 || task.PrivateData.TokenUsageSettled {
		return
	}
	model.UpdateUserAndTokenTotalTokenUsed(task.UserId, task.PrivateData.TokenId, totalTokens)
	task.PrivateData.TokenUsageSettled = true
	if err := task.Update(); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("failed to mark task token usage settled (task=%s): %s", task.TaskID, err.Error()))
	}
}

// taskAdjustFunding 调整任务的资金来源（钱包或订阅），delta > 0 表示补扣，delta < 0 表示退还。
//
// 补扣 (delta > 0)：
//   - 奖励优先扣费，并将本次扣费明细累加到 task 的资金快照中（供后续退款原路返回）。
//   - 异步任务补扣时不触发消费返利，统一延迟到任务 SUCCESS 后由 FinalizeTaskConsumeRebate 处理。
//
// 退还 (delta < 0)：
//   - 新任务（有 WalletQuotaBreakdownRecorded）：按快照中的奖励/充值比例原路退回。
//   - 旧任务（无快照）：回退到 IncreaseUserQuota 全部加到 quota（兼容旧行为）。
func taskAdjustFunding(task *model.Task, delta int) error {
	if taskIsSubscription(task) {
		if requestID := strings.TrimSpace(task.PrivateData.SubscriptionRequestId); requestID != "" {
			return model.AdjustSubscriptionPreConsume(requestID, int64(delta))
		}
		return model.PostConsumeUserSubscriptionDelta(task.PrivateData.SubscriptionId, int64(delta))
	}
	if delta > 0 {
		// Async task adjustments keep using reward balance first. Rebates are
		// deferred until the task reaches SUCCESS, so only update the persisted
		// funding snapshot here.
		breakdown, err := model.DecreaseUserQuotaPreferReward(task.UserId, delta)
		if err != nil {
			return err
		}
		if task.PrivateData.WalletQuotaBreakdownRecorded {
			task.PrivateData.WalletRewardUsed += breakdown.RewardUsed
			task.PrivateData.WalletPaidUsed += breakdown.PaidUsed
		}
		return nil
	}
	if delta == 0 {
		return nil
	}

	refundTotal := -delta
	if !task.PrivateData.WalletQuotaBreakdownRecorded {
		// 旧任务兼容：无资金来源快照时无法区分奖励/充值，全部退回到 quota。
		return model.IncreaseUserQuota(task.UserId, refundTotal, false)
	}
	// 新任务原路退回：优先退充值部分（先退 paid 再退 reward），确保退款后
	// 剩余的 WalletPaidUsed 可用于后续消费返利结算。
	consumedTotal := task.PrivateData.WalletRewardUsed + task.PrivateData.WalletPaidUsed
	if refundTotal > consumedTotal {
		return fmt.Errorf("task wallet refund exceeds consumed quota: refund=%d consumed=%d", refundTotal, consumedTotal)
	}
	// 充值优先退回：先尽量用 paid 额度退，不够再从 reward 额度补。
	paidRefund := refundTotal
	if paidRefund > task.PrivateData.WalletPaidUsed {
		paidRefund = task.PrivateData.WalletPaidUsed
	}
	rewardRefund := refundTotal - paidRefund
	if rewardRefund > task.PrivateData.WalletRewardUsed {
		return fmt.Errorf("task reward refund exceeds consumed reward quota: refund=%d consumed=%d", rewardRefund, task.PrivateData.WalletRewardUsed)
	}
	// 使用 IncreaseUserQuotaByBreakdown 分别恢复 quota 和 reward_quota，
	// 实现严格的原路返回。
	if err := model.IncreaseUserQuotaByBreakdown(task.UserId, refundTotal, rewardRefund); err != nil {
		return err
	}
	// 更新内存快照，使后续操作（如差额结算后的 FinalizeTaskConsumeRebate）
	// 基于剩余的实际充值消耗计算返利。
	task.PrivateData.WalletPaidUsed -= paidRefund
	task.PrivateData.WalletRewardUsed -= rewardRefund
	return nil
}

// consumeRebateContextFromTask 从异步任务的上下文中构建消费返利所需的模型和站点信息。
// 与 consumeRebateContextFromRelay（同步请求版本）对应，专门用于异步任务的终态返利结算。
//
// 主站（providerId<=0）使用 OriginModelName 兜底，即使 BillingContext 为 nil 也能参与返利；
// 服务商站点必须配置了 ProviderPublicModel 才能参与。
func consumeRebateContextFromTask(task *model.Task) *model.ConsumeRebateContext {
	if task == nil {
		return nil
	}
	// BillingContext 为 nil 时（如旧数据），主站仍可用 Properties.OriginModelName 兜底参与返利。
	if task.PrivateData.BillingContext == nil {
		return &model.ConsumeRebateContext{
			ProviderId:      0,
			PublicModelName: task.Properties.OriginModelName,
			BaseModelName:   task.Properties.OriginModelName,
		}
	}
	bc := task.PrivateData.BillingContext
	if bc.ProviderId <= 0 {
		modelName := bc.OriginModelName
		if modelName == "" {
			modelName = task.Properties.OriginModelName
		}
		return &model.ConsumeRebateContext{
			ProviderId:      0,
			PublicModelName: modelName,
			BaseModelName:   modelName,
		}
	}
	if bc.ProviderPublicModel == "" {
		return nil
	}
	return &model.ConsumeRebateContext{
		ProviderId:        bc.ProviderId,
		ProviderPricingId: bc.ProviderPricingId,
		PublicModelName:   bc.ProviderPublicModel,
		BaseModelName:     bc.ProviderBaseModel,
	}
}

// stableConsumeRebateRequestId 生成确定性的返利请求 ID。
// 使用 SHA256(scope:id) 确保同一任务无论被多少轮询实例处理，
// 生成的 request_id 都一致，配合 ConsumeRebates 表的 INSERT ON CONFLICT DO NOTHING
// 实现终态返利的全局幂等。
func stableConsumeRebateRequestId(scope string, id string) string {
	return fmt.Sprintf("%x", common.Sha256Raw([]byte(scope+":"+id)))
}

// persistTaskBillingState 持久化任务的 quota 和 private_data 列。
// 用于在终态返利结算后标记 ConsumeRebateSettled=true，防止后续重复处理。
func persistTaskBillingState(task *model.Task) error {
	if task == nil || task.ID <= 0 {
		return nil
	}
	return model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"quota":        task.Quota,
		"private_data": task.PrivateData,
	}).Error
}

// persistTaskBillingProgress writes only the task billing checkpoint.  It is
// deliberately called after each independently committed side of an async
// settlement.  Funding and token quota live in separate persistence systems,
// so this durable checkpoint is what makes a retry after a process crash
// continue from the unfinished side instead of replaying the entire delta.
func persistTaskBillingProgress(task *model.Task) error {
	return persistTaskBillingState(task)
}

// FinalizeTaskConsumeRebate credits the main-site inviter exactly once after
// an async wallet task reaches SUCCESS and all quota adjustments are complete.
func FinalizeTaskConsumeRebate(ctx context.Context, task *model.Task) {
	if task == nil || task.PrivateData.BillingSource != BillingSourceWallet ||
		!task.PrivateData.WalletQuotaBreakdownRecorded || task.PrivateData.ConsumeRebateSettled {
		return
	}
	// A terminal task can still have an unfinished two-phase settlement (for
	// example, funding committed but token-quota persistence failed).  The
	// completion path invokes this helper via defer, so do not settle the
	// rebate while either billing side is pending; doing so would make the
	// inviter balance reflect a charge that may later be rolled back/retried.
	if task.PrivateData.BillingSettlementPending {
		return
	}
	if bc := task.PrivateData.BillingContext; bc != nil && bc.ProviderId > 0 {
		// Provider-site rebates are settled by the existing provider-profit path.
		return
	}
	if task.PrivateData.WalletPaidUsed > 0 {
		requestId := stableConsumeRebateRequestId("task-final", task.TaskID)
		if _, _, err := model.ApplyInviteConsumeRebate(task.UserId, requestId, task.PrivateData.WalletPaidUsed, consumeRebateContextFromTask(task)); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("failed to apply final task consume rebate (task=%s): %s", task.TaskID, err.Error()))
			return
		}
	}
	task.PrivateData.ConsumeRebateSettled = true
	if err := persistTaskBillingState(task); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("failed to persist final task rebate state (task=%s): %s", task.TaskID, err.Error()))
	}
}

// FinalizeTaskProviderProfit applies provider cost/profit only after an async
// task succeeds. ProviderProfit.RequestId is unique, so retries and concurrent
// pollers remain idempotent.
func FinalizeTaskProviderProfit(ctx context.Context, task *model.Task) {
	if task == nil || task.PrivateData.ProviderProfitSettled {
		return
	}
	// See FinalizeTaskConsumeRebate: settlement is called from a defer in the
	// terminal polling path.  Provider cost/profit must wait until both the
	// funding and token sides have committed, otherwise a retry could record
	// provider profit for an uncommitted/partially rolled-back charge.
	if task.PrivateData.BillingSettlementPending {
		return
	}
	bc := task.PrivateData.BillingContext
	if bc == nil || bc.ProviderId <= 0 || bc.ProviderOwnerUserId <= 0 || bc.ProviderBaseQuota <= 0 {
		return
	}
	providerCharge := bc.ProviderUserQuota
	if providerCharge < 0 {
		providerCharge = 0
	}
	paidQuota := task.PrivateData.WalletPaidUsed
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
		coverableCost := bc.ProviderBaseQuota
		if providerCharge < coverableCost {
			coverableCost = providerCharge
		}
		coveredCost = int(decimal.NewFromInt(int64(coverableCost)).Mul(paidRatio).IntPart())
		if grossProfit := providerCharge - bc.ProviderBaseQuota; grossProfit > 0 {
			profitQuota = int(decimal.NewFromInt(int64(grossProfit)).Mul(paidRatio).IntPart())
		}
	}
	ownerCost := bc.ProviderBaseQuota - coveredCost
	if ownerCost < 0 {
		ownerCost = 0
	}
	record := &model.ProviderProfit{
		ProviderId:        bc.ProviderId,
		OwnerUserId:       bc.ProviderOwnerUserId,
		ProviderUserId:    task.UserId,
		RequestId:         stableConsumeRebateRequestId("task-provider-profit", task.TaskID),
		PublicModelName:   bc.ProviderPublicModel,
		BaseModelName:     bc.ProviderBaseModel,
		ProviderUserQuota: providerCharge,
		BaseCostQuota:     bc.ProviderBaseQuota,
		PaidQuota:         paidQuota,
		CoveredCostQuota:  coveredCost,
		OwnerCostQuota:    ownerCost,
		ProfitQuota:       profitQuota,
	}
	result, err := model.ApplyProviderProfit(record)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("failed to finalize task provider profit (task=%s): %s", task.TaskID, err.Error()))
		return
	}
	if result != nil && result.Applied {
		model.LogProviderProfit(record)
	}
	task.PrivateData.ProviderProfitSettled = true
	if err := persistTaskBillingState(task); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("failed to persist task provider profit state (task=%s): %s", task.TaskID, err.Error()))
	}
}

func persistMidjourneyBillingState(task *model.Midjourney) error {
	if task == nil || task.Id <= 0 {
		return nil
	}
	return model.DB.Model(&model.Midjourney{}).Where("id = ?", task.Id).Updates(map[string]interface{}{
		"billing_source":                  task.BillingSource,
		"billing_pre_consumed":            task.BillingPreConsumed,
		"subscription_id":                 task.SubscriptionId,
		"subscription_request_id":         task.SubscriptionRequestId,
		"subscription_pre_consumed":       task.SubscriptionPreConsumed,
		"token_id":                        task.TokenId,
		"billing_refunded":                task.BillingRefunded,
		"billing_refund_funding_done":     task.BillingRefundFundingDone,
		"billing_refund_token_done":       task.BillingRefundTokenDone,
		"wallet_reward_used":              task.WalletRewardUsed,
		"wallet_paid_used":                task.WalletPaidUsed,
		"wallet_quota_breakdown_recorded": task.WalletQuotaBreakdownRecorded,
		"consume_rebate_settled":          task.ConsumeRebateSettled,
	}).Error
}

// RecordMidjourneyBilling persists the billing-session snapshot on a
// Midjourney task.  Midjourney tasks are stored in their own table (rather
// than the generic tasks table), so this explicit copy is required for later
// polling, settlement and failure refunds.  The helper is intentionally
// idempotent and uses an UPDATE by primary key to avoid accidental inserts.
func RecordMidjourneyBilling(task *model.Midjourney, info *relaycommon.RelayInfo) error {
	if task == nil || info == nil {
		return nil
	}
	task.BillingSource = info.BillingSource
	task.BillingPreConsumed = int64(info.FinalPreConsumedQuota)
	task.SubscriptionId = info.SubscriptionId
	task.SubscriptionRequestId = ""
	task.SubscriptionPreConsumed = 0
	if info.BillingSource == BillingSourceSubscription {
		task.SubscriptionRequestId = strings.TrimSpace(info.RequestId)
		task.SubscriptionPreConsumed = info.SubscriptionPreConsumed
	}
	task.TokenId = info.TokenId
	return persistMidjourneyBillingState(task)
}

// RecordMidjourneyWalletFunding 在 Midjourney 任务提交成功后持久化钱包消费的奖励/充值拆分。
// 这是 Midjourney 退款原路返回和消费返利终态结算的数据基础。
// 仅在 PostConsumeQuota 成功后调用，此时 relayInfo.WalletRewardConsumed/WalletPaidConsumed 已经填充。
func RecordMidjourneyWalletFunding(task *model.Midjourney, rewardUsed int, paidUsed int) error {
	if task == nil {
		return nil
	}
	task.WalletRewardUsed = rewardUsed
	task.WalletPaidUsed = paidUsed
	task.WalletQuotaBreakdownRecorded = 1
	return persistMidjourneyBillingState(task)
}

// FinalizeMidjourneyConsumeRebate applies the same terminal-success policy to
// legacy Midjourney tasks, which are stored outside the generic tasks table.
func FinalizeMidjourneyConsumeRebate(ctx context.Context, task *model.Midjourney) {
	if task == nil || task.WalletQuotaBreakdownRecorded != 1 || task.ConsumeRebateSettled == 1 {
		return
	}
	if task.WalletPaidUsed > 0 {
		requestId := stableConsumeRebateRequestId("midjourney-final", task.MjId)
		modelName := CovertMjpActionToModelName(task.Action)
		rebateCtx := &model.ConsumeRebateContext{
			ProviderId:      0,
			PublicModelName: modelName,
			BaseModelName:   modelName,
		}
		if _, _, err := model.ApplyInviteConsumeRebate(task.UserId, requestId, task.WalletPaidUsed, rebateCtx); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("failed to apply final Midjourney consume rebate (task=%s): %s", task.MjId, err.Error()))
			return
		}
	}
	task.ConsumeRebateSettled = 1
	if err := persistMidjourneyBillingState(task); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("failed to persist Midjourney rebate state (task=%s): %s", task.MjId, err.Error()))
	}
}

// Midjourney refunds are performed by the polling worker, which may run in
// several goroutines/processes at once.  Wallet and token increments are
// relative (non-idempotent) operations, so a single boolean BillingRefunded
// marker is insufficient: a token failure after a successful wallet refund
// would cause a retry to refund the wallet twice.  The two durable progress
// markers below are intentionally small integers (0=pending, 1=done,
// 2=claimed).  A process-local lock handles duplicate calls in one process;
// the conditional database claim handles workers in different processes.
var midjourneyRefundLocks sync.Map // map[string]*sync.Mutex

func midjourneyRefundLockKey(task *model.Midjourney) string {
	if task == nil {
		return ""
	}
	if task.Id > 0 {
		return fmt.Sprintf("id:%d", task.Id)
	}
	if id := strings.TrimSpace(task.MjId); id != "" {
		return "mj:" + id
	}
	return fmt.Sprintf("ptr:%p", task)
}

func withMidjourneyRefundLock(task *model.Midjourney, fn func() error) error {
	key := midjourneyRefundLockKey(task)
	if key == "" {
		return fn()
	}
	entry, _ := midjourneyRefundLocks.LoadOrStore(key, &sync.Mutex{})
	mu := entry.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// claimMidjourneyRefundSide atomically claims one refund side.  It returns
// false when another worker already completed/claimed the side.  For an
// unsaved task the in-memory marker is sufficient (there is no row on which a
// database CAS could operate).
func claimMidjourneyRefundSide(task *model.Midjourney, column string, state *int) (bool, error) {
	if task == nil || state == nil {
		return false, nil
	}
	if *state == 1 || task.BillingRefunded == 1 {
		return false, nil
	}
	if task.Id <= 0 || model.DB == nil {
		if *state != 0 {
			return false, nil
		}
		*state = 2
		return true, nil
	}
	// The column names are fixed internal constants, never user input.
	result := model.DB.Model(&model.Midjourney{}).
		Where("id = ? AND billing_refunded = 0 AND "+column+" = 0", task.Id).
		Update(column, 2)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 1 {
		*state = 2
		return true, nil
	}
	// Refresh only the state columns.  This also makes a stale task object
	// observe a completion made by a different polling worker.
	var current struct {
		BillingRefunded          int `gorm:"column:billing_refunded"`
		BillingRefundFundingDone int `gorm:"column:billing_refund_funding_done"`
		BillingRefundTokenDone   int `gorm:"column:billing_refund_token_done"`
	}
	if err := model.DB.Model(&model.Midjourney{}).
		Select("billing_refunded", "billing_refund_funding_done", "billing_refund_token_done").
		Where("id = ?", task.Id).Take(&current).Error; err != nil {
		return false, err
	}
	task.BillingRefunded = current.BillingRefunded
	if column == "billing_refund_funding_done" {
		*state = current.BillingRefundFundingDone
	} else {
		*state = current.BillingRefundTokenDone
	}
	return false, nil
}

func finishMidjourneyRefundSide(task *model.Midjourney, column string, state *int, success bool) error {
	if task == nil || state == nil {
		return nil
	}
	next := 0
	if success {
		next = 1
	}
	if task.Id > 0 && model.DB != nil {
		result := model.DB.Model(&model.Midjourney{}).Where("id = ? AND "+column+" = 2", task.Id).Update(column, next)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 && success {
			// Another worker may have completed the side while this process was
			// performing the external operation.  Read-back distinguishes that
			// benign race from a missing row.
			var value int
			if err := model.DB.Model(&model.Midjourney{}).Select(column).Where("id = ?", task.Id).Scan(&value).Error; err != nil {
				return err
			}
			if value != 1 {
				return fmt.Errorf("midjourney refund checkpoint lost for %s", column)
			}
		}
	}
	*state = next
	return nil
}

func finalizeMidjourneyRefund(task *model.Midjourney) error {
	if task == nil {
		return nil
	}
	if task.BillingRefundFundingDone != 1 || task.BillingRefundTokenDone != 1 {
		return nil
	}
	task.BillingRefunded = 1
	task.WalletRewardUsed = 0
	task.WalletPaidUsed = 0
	task.WalletQuotaBreakdownRecorded = 0
	task.ConsumeRebateSettled = 1
	if task.Id > 0 && model.DB != nil {
		// Keep the final flag conditional so a stale worker cannot overwrite a
		// row that has already been finalized by another worker.
		if err := model.DB.Model(&model.Midjourney{}).
			Where("id = ? AND billing_refund_funding_done = 1 AND billing_refund_token_done = 1", task.Id).
			Updates(map[string]interface{}{
				"billing_refunded":                1,
				"wallet_reward_used":              0,
				"wallet_paid_used":                0,
				"wallet_quota_breakdown_recorded": 0,
				"consume_rebate_settled":          1,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

// RefundMidjourneyQuota 按原路返回 Midjourney 任务消费的额度。
// Each funding/token side is claimed and checkpointed independently, making
// retries safe when one persistence system is temporarily unavailable.
func RefundMidjourneyQuota(task *model.Midjourney) error {
	if task == nil {
		return nil
	}
	return withMidjourneyRefundLock(task, func() error {
		// Persisted tasks use the model-layer atomic implementation.  It locks
		// the task row and updates the wallet/subscription, token, and refund
		// marker in one transaction, so concurrent pollers and process crashes
		// cannot mint quota by replaying a relative increment.
		if task.Id > 0 && model.DB != nil {
			return model.RefundMidjourneyBilling(task)
		}
		if task.BillingRefunded == 1 {
			return nil
		}
		// A persisted caller may hold a stale copy.  Refresh billing state before
		// claiming; if the row is gone (e.g. a unit test's unsaved task), retain
		// the supplied snapshot and continue in memory.
		if task.Id > 0 && model.DB != nil {
			var current model.Midjourney
			if err := model.DB.Where("id = ?", task.Id).First(&current).Error; err == nil {
				*task = current
			}
		}

		fundingClaimed, err := claimMidjourneyRefundSide(task, "billing_refund_funding_done", &task.BillingRefundFundingDone)
		if err != nil {
			return err
		}
		if fundingClaimed {
			if task.BillingSource == BillingSourceSubscription {
				requestID := strings.TrimSpace(task.SubscriptionRequestId)
				if requestID != "" {
					err = model.RefundSubscriptionPreConsume(requestID)
				} else if task.SubscriptionId > 0 && task.SubscriptionPreConsumed > 0 {
					err = fmt.Errorf("Midjourney subscription refund request id is missing")
				}
			} else {
				refundTotal := task.Quota
				if task.BillingPreConsumed > 0 && refundTotal <= 0 {
					refundTotal = int(task.BillingPreConsumed)
				}
				if refundTotal > 0 {
					if task.WalletQuotaBreakdownRecorded == 1 {
						consumedTotal := task.WalletRewardUsed + task.WalletPaidUsed
						if consumedTotal != refundTotal {
							err = fmt.Errorf("Midjourney wallet snapshot mismatch: quota=%d consumed=%d", refundTotal, consumedTotal)
						} else {
							err = model.IncreaseUserQuotaByBreakdown(task.UserId, consumedTotal, task.WalletRewardUsed)
						}
					} else {
						err = model.IncreaseUserQuota(task.UserId, refundTotal, false)
					}
				}
			}
			if err != nil {
				_ = finishMidjourneyRefundSide(task, "billing_refund_funding_done", &task.BillingRefundFundingDone, false)
				return err
			}
			if err = finishMidjourneyRefundSide(task, "billing_refund_funding_done", &task.BillingRefundFundingDone, true); err != nil {
				return err
			}
		}

		// Token quota is pre-consumed for wallet-funded billing sessions.  A
		// subscription-funded request bypasses the token's numeric allowance, so
		// its subscription snapshot must never be credited back to a token.
		tokenClaimed, err := claimMidjourneyRefundSide(task, "billing_refund_token_done", &task.BillingRefundTokenDone)
		if err != nil {
			return err
		}
		if tokenClaimed {
			if task.BillingSource == BillingSourceSubscription {
				// Subscription funding never reserves the token's numeric quota;
				// mark this side complete without touching the token row.
				if err = finishMidjourneyRefundSide(task, "billing_refund_token_done", &task.BillingRefundTokenDone, true); err != nil {
					return err
				}
			} else {
				refundQuota := task.BillingPreConsumed
				if refundQuota <= 0 {
					refundQuota = int64(task.Quota)
				}
				if refundQuota > 0 && task.TokenId > 0 {
					token, lookupErr := model.GetTokenById(task.TokenId)
					if lookupErr != nil {
						err = lookupErr
					} else {
						err = model.IncreaseTokenQuota(task.TokenId, token.Key, int(refundQuota))
					}
				}
				if err != nil {
					_ = finishMidjourneyRefundSide(task, "billing_refund_token_done", &task.BillingRefundTokenDone, false)
					return err
				}
				if err = finishMidjourneyRefundSide(task, "billing_refund_token_done", &task.BillingRefundTokenDone, true); err != nil {
					return err
				}
			}
		}

		return finalizeMidjourneyRefund(task)
	})
}

// taskAdjustTokenQuota 调整任务的令牌额度，delta > 0 表示扣费，delta < 0 表示退还。
// 需要通过 resolveTokenKey 运行时获取 key（不从 PrivateData 中读取）。
func taskAdjustTokenQuota(ctx context.Context, task *model.Task, delta int) error {
	if !taskTokenQuotaRequired(task) || delta == 0 {
		return nil
	}
	if taskTokenQuotaAdjuster != nil {
		if err := taskTokenQuotaAdjuster(ctx, task, delta); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("调整令牌额度失败 (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
			return err
		}
		return nil
	}
	tokenKey := resolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		// A token can be deleted after an asynchronous task was submitted.  Keep
		// the historical best-effort behavior in that case: the user/subscription
		// funding side remains authoritative, while the missing token row cannot
		// be adjusted anyway.  Actual database failures below are still surfaced
		// so the checkpoint can be retried.
		return nil
	}
	var err error
	if delta > 0 {
		err = model.DecreaseTokenQuota(task.PrivateData.TokenId, tokenKey, delta)
	} else {
		err = model.IncreaseTokenQuota(task.PrivateData.TokenId, tokenKey, -delta)
	}
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("调整令牌额度失败 (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
	}
	return err
}

// taskBillingOther 从 task 的 BillingContext 构建日志 Other 字段。
func taskBillingOther(task *model.Task) map[string]interface{} {
	other := make(map[string]interface{})
	if bc := task.PrivateData.BillingContext; bc != nil {
		other["model_price"] = bc.ModelPrice
		if bc.ModelRatio > 0 {
			other["model_ratio"] = bc.ModelRatio
		}
		other["group_ratio"] = bc.GroupRatio
		if bc.BillingMode != "" {
			other["billing_mode"] = bc.BillingMode
			other["billing_unit"] = bc.BillingUnit
			other["resolution"] = bc.Resolution
			other["unit_price"] = bc.UnitPrice
			other["unit_count"] = bc.UnitCount
			other["output_count"] = bc.OutputCount
		}
		if len(bc.OtherRatios) > 0 {
			for k, v := range bc.OtherRatios {
				other[k] = v
			}
		}
		if bc.ProviderId > 0 {
			other["provider_id"] = bc.ProviderId
			other["provider_pricing_id"] = bc.ProviderPricingId
			other["provider_public_model"] = bc.ProviderPublicModel
			other["provider_base_model"] = bc.ProviderBaseModel
		}
	}
	props := task.Properties
	if props.UpstreamModelName != "" && props.UpstreamModelName != props.OriginModelName {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = props.UpstreamModelName
	}
	return other
}

// taskModelName 从 BillingContext 或 Properties 中获取模型名称。
func taskModelName(task *model.Task) string {
	if bc := task.PrivateData.BillingContext; bc != nil && bc.OriginModelName != "" {
		return bc.OriginModelName
	}
	return task.Properties.OriginModelName
}

func initializeTaskBillingSettlementAt(task *model.Task, current, target int) error {
	if task == nil {
		return errors.New("task is nil")
	}
	state := &task.PrivateData
	if state.BillingSettlementPending {
		// Keep an existing durable checkpoint untouched.  The caller that owns
		// retargeting (settleTaskBillingTarget) updates the target and done flags
		// atomically before attempting another side.
		return nil
	}
	state.BillingSettlementPending = true
	state.BillingSettlementTarget = target
	state.BillingSettlementFundingAmount = current
	state.BillingSettlementTokenAmount = current
	state.BillingSettlementFundingDone = state.BillingSettlementFundingAmount == target
	state.BillingSettlementTokenDone = state.BillingSettlementTokenAmount == target || !taskTokenQuotaRequired(task)
	return persistTaskBillingProgress(task)
}

func initializeTaskBillingSettlement(task *model.Task, target int) error {
	if task == nil {
		return errors.New("task is nil")
	}
	return initializeTaskBillingSettlementAt(task, task.Quota, target)
}

func clearTaskBillingSettlement(task *model.Task) {
	state := &task.PrivateData
	state.BillingSettlementPending = false
	state.BillingSettlementTarget = 0
	state.BillingSettlementFundingAmount = 0
	state.BillingSettlementTokenAmount = 0
	state.BillingSettlementFundingDone = false
	state.BillingSettlementTokenDone = false
}

// settleTaskBillingTarget advances an async task to an absolute quota target.
// The funding and token sides are checkpointed separately.  This mirrors the
// synchronous BillingSession state machine and prevents duplicate funding
// charges when token persistence (or the task-row write after it) fails.
func settleTaskBillingTarget(ctx context.Context, task *model.Task, target int) error {
	if target < 0 {
		return errors.New("actual quota must be >= 0")
	}
	if task == nil {
		return errors.New("task is nil")
	}
	state := &task.PrivateData
	if state.BillingSettlementPending {
		// A later poll can provide a better final amount while a previous
		// checkpoint is still pending. Keep the independently committed side
		// amounts and retarget only the unfinished difference.
		if state.BillingSettlementTarget != target {
			state.BillingSettlementTarget = target
			state.BillingSettlementFundingDone = state.BillingSettlementFundingAmount == target
			state.BillingSettlementTokenDone = state.BillingSettlementTokenAmount == target || !taskTokenQuotaRequired(task)
			if err := persistTaskBillingProgress(task); err != nil {
				return fmt.Errorf("persist task settlement retarget checkpoint: %w", err)
			}
		}
	} else if err := initializeTaskBillingSettlement(task, target); err != nil {
		return err
	}

	if !state.BillingSettlementFundingDone {
		delta := target - state.BillingSettlementFundingAmount
		if err := taskAdjustFunding(task, delta); err != nil {
			return err
		}
		state.BillingSettlementFundingAmount = target
		state.BillingSettlementFundingDone = true
		if err := persistTaskBillingProgress(task); err != nil {
			return fmt.Errorf("persist task funding settlement checkpoint: %w", err)
		}
	}

	if !state.BillingSettlementTokenDone {
		delta := target - state.BillingSettlementTokenAmount
		if err := taskAdjustTokenQuota(ctx, task, delta); err != nil {
			return err
		}
		state.BillingSettlementTokenAmount = target
		state.BillingSettlementTokenDone = true
		if err := persistTaskBillingProgress(task); err != nil {
			return fmt.Errorf("persist task token settlement checkpoint: %w", err)
		}
	}
	return nil
}

// RefundTaskQuota 统一的任务失败退款逻辑。
// 当异步任务失败时，将预扣的 quota 退还给用户（支持钱包和订阅），并退还令牌额度。
//
// 退款策略：
//   - 新任务（有 WalletQuotaBreakdownRecorded）：按消费快照原路退回
//     （奖励部分退回 reward_quota，充值部分退回 quota），防止用户通过
//     任务失败将奖励额度洗成充值额度。退款后标记 ConsumeRebateSettled
//     阻止后续误触发返利。
//   - 旧任务（无快照）：兼容旧行为，全部退到 quota。
func RefundTaskQuota(ctx context.Context, task *model.Task, reason string) {
	if task == nil {
		return
	}
	if task.PrivateData.BillingRefunded {
		return
	}
	quota := taskRefundQuota(task)
	// A settlement checkpoint can outlive the task's raw Quota value.  In
	// particular, a task submitted with Quota=0 may have already committed a
	// completion target on one side before the other side failed.  Derive the
	// refundable amount from both durable side snapshots before applying the
	// zero-quota fast path; otherwise the pending funding/token operation would
	// be stranded forever.
	if task.PrivateData.BillingSettlementPending {
		if task.PrivateData.BillingSettlementFundingAmount > quota {
			quota = task.PrivateData.BillingSettlementFundingAmount
		}
		if task.PrivateData.BillingSettlementTokenAmount > quota {
			quota = task.PrivateData.BillingSettlementTokenAmount
		}
	}
	if quota <= 0 {
		return
	}

	// 1. 退还资金来源（钱包或订阅）。  The two sides are checkpointed so
	// a token-store failure does not make a later retry refund funding twice.
	if task.PrivateData.BillingSettlementPending {
		// A failed completion settlement may have left one side at a different
		// absolute amount. Refund from each side's checkpoint independently.
		state := &task.PrivateData
		if state.BillingSettlementFundingAmount > quota {
			quota = state.BillingSettlementFundingAmount
		}
		if state.BillingSettlementTokenAmount > quota {
			quota = state.BillingSettlementTokenAmount
		}
		state.BillingSettlementTarget = 0
		state.BillingSettlementFundingDone = state.BillingSettlementFundingAmount == 0
		state.BillingSettlementTokenDone = state.BillingSettlementTokenAmount == 0 || !taskTokenQuotaRequired(task)
		if err := persistTaskBillingProgress(task); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("持久化任务退款目标失败 task %s: %s", task.TaskID, err.Error()))
			return
		}
	} else {
		current := quota
		if task.Quota > 0 {
			current = task.Quota
		}
		if err := initializeTaskBillingSettlementAt(task, current, 0); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("初始化任务退款状态失败 task %s: %s", task.TaskID, err.Error()))
			return
		}
	}
	if err := settleTaskBillingTarget(ctx, task, 0); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("退还任务资金/令牌失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
	clearTaskBillingSettlement(task)
	task.PrivateData.BillingRefunded = true
	if err := persistTaskBillingState(task); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("failed to persist task refund completion (task=%s): %s", task.TaskID, err.Error()))
		return
	}
	// 退款后标记 ConsumeRebateSettled=true，防止后续轮询误触发消费返利。
	// 任务失败不产生返利，仅成功任务在 FinalizeTaskConsumeRebate 中触发。
	if task.PrivateData.WalletQuotaBreakdownRecorded {
		task.PrivateData.ConsumeRebateSettled = true
		if err := persistTaskBillingState(task); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("failed to persist task refund funding state (task=%s): %s", task.TaskID, err.Error()))
		}
	}

	// 2. 退还令牌额度
	// 3. 记录日志
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		Content:   "",
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     quota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RecalculateTaskQuota 通用的异步差额结算。
// actualQuota 是任务完成后的实际应扣额度，与预扣额度 (task.Quota) 做差额结算。
// reason 用于日志记录（例如 "token重算" 或 "adaptor调整"）。
func RecalculateTaskQuota(ctx context.Context, task *model.Task, actualQuota int, reason string) {
	if task == nil {
		return
	}
	if actualQuota <= 0 {
		return
	}
	preConsumedQuota := task.Quota
	// A subscription-backed async submission may intentionally preserve a
	// one-unit pre-consume when the submit-time result rounds to zero.  The task
	// row keeps that raw result (Quota=0), but the subscription record already
	// contains the reserved amount.  Use the snapshot as the settlement baseline
	// so the completion adjustment charges only actual-preConsumed.
	if preConsumedQuota <= 0 && taskIsSubscription(task) {
		preConsumedQuota = taskRefundQuota(task)
	}
	quotaDelta := actualQuota - preConsumedQuota

	if quotaDelta == 0 {
		if task.Quota != actualQuota {
			task.Quota = actualQuota
			if err := persistTaskBillingState(task); err != nil {
				logger.LogWarn(ctx, fmt.Sprintf("failed to persist normalized task quota (task=%s): %s", task.TaskID, err.Error()))
			}
		}
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 预扣费准确（%s，%s）",
			task.TaskID, logger.LogQuota(actualQuota), reason))
		return
	}

	logger.LogInfo(ctx, fmt.Sprintf("任务 %s 差额结算：delta=%s（实际：%s，预扣：%s，%s）",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason,
	))

	// For a zero-valued subscription task, Quota is intentionally kept at zero
	// in the task row while the request record holds the one-unit reservation.
	// Pass that effective baseline to the checkpoint state machine.
	settlementCurrent := preConsumedQuota
	if task.PrivateData.BillingSettlementPending {
		settlementCurrent = task.PrivateData.BillingSettlementFundingAmount
	}
	if err := initializeTaskBillingSettlementAt(task, settlementCurrent, actualQuota); err != nil {
		logger.LogError(ctx, fmt.Sprintf("初始化任务差额结算状态失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
	if err := settleTaskBillingTarget(ctx, task, actualQuota); err != nil {
		logger.LogError(ctx, fmt.Sprintf("任务差额结算资金/令牌调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}
	clearTaskBillingSettlement(task)
	task.Quota = actualQuota
	if err := persistTaskBillingState(task); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("failed to persist task quota settlement (task=%s): %s", task.TaskID, err.Error()))
	}

	var logType int
	var logQuota int
	if quotaDelta > 0 {
		logType = model.LogTypeConsume
		logQuota = quotaDelta
		if taskIsSubscription(task) {
			model.UpdateUserRequestCount(task.UserId, 1)
		} else {
			model.UpdateUserUsedQuotaAndRequestCount(task.UserId, quotaDelta)
		}
		model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["pre_consumed_quota"] = preConsumedQuota
	other["actual_quota"] = actualQuota
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		UserId:    task.UserId,
		LogType:   logType,
		Content:   reason,
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     logQuota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})
}

// RetryPendingTaskBilling resumes terminal-task billing checkpoints left by
// an earlier process after one side of the two-phase settlement failed.  A
// terminal task is no longer returned by GetAllUnFinishSyncTasks, therefore a
// separate bounded recovery pass is required to make retries self-healing.
//
// SUCCESS tasks resume their absolute settlement target.  FAILURE tasks are
// refunded to zero.  Both operations are idempotent at the subscription
// request-record layer and are checkpointed independently for wallet/token
// stores.  The returned count is the number of candidates examined.
func RetryPendingTaskBilling(ctx context.Context, limit int) int {
	if limit <= 0 {
		limit = 100
	}
	taskBillingRecoveryMu.Lock()
	defer taskBillingRecoveryMu.Unlock()
	tasks, err := model.GetTerminalTasksForBillingRecovery(limit)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("load pending terminal task billing checkpoints: %v", err))
		return 0
	}
	processed := 0
	for _, task := range tasks {
		if task == nil || !task.PrivateData.BillingSettlementPending {
			continue
		}
		processed++
		switch task.Status {
		case model.TaskStatusFailure:
			RefundTaskQuota(ctx, task, "retry pending task billing refund")
		case model.TaskStatusSuccess:
			target := task.PrivateData.BillingSettlementTarget
			if target < 0 {
				logger.LogWarn(ctx, fmt.Sprintf("skip invalid pending billing target task=%s target=%d", task.TaskID, target))
				continue
			}
			// RecalculateTaskQuota intentionally ignores non-positive targets.  A
			// pending target of zero is nevertheless meaningful for a successful
			// task that must release its reservation, so drive the internal state
			// machine directly in that case.
			if target == 0 {
				if err := settleTaskBillingTarget(ctx, task, 0); err != nil {
					logger.LogWarn(ctx, fmt.Sprintf("retry pending zero settlement task=%s: %v", task.TaskID, err))
					continue
				}
				clearTaskBillingSettlement(task)
				task.Quota = 0
				if err := persistTaskBillingState(task); err != nil {
					logger.LogWarn(ctx, fmt.Sprintf("persist pending zero settlement task=%s: %v", task.TaskID, err))
					continue
				}
			} else {
				RecalculateTaskQuota(ctx, task, target, "retry pending task billing settlement")
			}
			// Rebate/profit finalizers are normally deferred by the completion
			// path.  They were intentionally skipped while the checkpoint was
			// pending, so invoke them after a successful recovery pass.
			if !task.PrivateData.BillingSettlementPending {
				FinalizeTaskConsumeRebate(ctx, task)
				FinalizeTaskProviderProfit(ctx, task)
			}
		default:
			logger.LogWarn(ctx, fmt.Sprintf("skip pending billing task %s with non-terminal status %s", task.TaskID, task.Status))
		}
	}
	return processed
}

// RecalculateTaskQuotaByTokens 根据实际 token 消耗重新计费（异步差额结算）。
// 当任务成功且返回了 totalTokens 时，根据模型倍率和分组倍率重新计算实际扣费额度，
// 与预扣费的差额进行补扣或退还。支持钱包和订阅计费来源。
func RecalculateTaskQuotaByTokens(ctx context.Context, task *model.Task, totalTokens int) {
	if totalTokens <= 0 {
		return
	}

	modelName := taskModelName(task)

	// 获取模型价格和倍率
	modelRatio, hasRatioSetting, _ := ratio_setting.GetModelRatio(modelName)
	// 只有配置了倍率(非固定价格)时才按 token 重新计费
	if !hasRatioSetting || modelRatio <= 0 {
		return
	}

	// 获取用户和组的倍率信息
	group := task.Group
	if group == "" {
		user, err := model.GetUserById(task.UserId, false)
		if err == nil {
			group = user.Group
		}
	}
	if group == "" {
		return
	}

	groupRatio := ratio_setting.GetGroupRatio(group)
	userGroupRatio, hasUserGroupRatio := ratio_setting.GetGroupGroupRatio(group, group)

	var finalGroupRatio float64
	if hasUserGroupRatio {
		finalGroupRatio = userGroupRatio
	} else {
		finalGroupRatio = groupRatio
	}

	// 计算 OtherRatios 乘积（视频折扣、时长等）
	otherMultiplier := 1.0
	if bc := task.PrivateData.BillingContext; bc != nil {
		for _, r := range bc.OtherRatios {
			if r != 1.0 && r > 0 {
				otherMultiplier *= r
			}
		}
	}

	// 计算实际应扣费额度: totalTokens * modelRatio * groupRatio * otherMultiplier
	actualQuota := common.QuotaFromFloat(float64(totalTokens) * modelRatio * finalGroupRatio * otherMultiplier)

	reason := fmt.Sprintf("token重算：tokens=%d, modelRatio=%.2f, groupRatio=%.2f, otherMultiplier=%.4f", totalTokens, modelRatio, finalGroupRatio, otherMultiplier)
	RecalculateTaskQuota(ctx, task, actualQuota, reason)
}
