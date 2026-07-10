package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo) {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	// 支持任务仅按次计费
	if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
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
	return task.PrivateData.BillingSource == BillingSourceSubscription && task.PrivateData.SubscriptionId > 0
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

// FinalizeTaskConsumeRebate credits the main-site inviter exactly once after
// an async wallet task reaches SUCCESS and all quota adjustments are complete.
func FinalizeTaskConsumeRebate(ctx context.Context, task *model.Task) {
	if task == nil || task.PrivateData.BillingSource != BillingSourceWallet ||
		!task.PrivateData.WalletQuotaBreakdownRecorded || task.PrivateData.ConsumeRebateSettled {
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

func persistMidjourneyBillingState(task *model.Midjourney) error {
	if task == nil || task.Id <= 0 {
		return nil
	}
	return model.DB.Model(&model.Midjourney{}).Where("id = ?", task.Id).Updates(map[string]interface{}{
		"wallet_reward_used":              task.WalletRewardUsed,
		"wallet_paid_used":                task.WalletPaidUsed,
		"wallet_quota_breakdown_recorded": task.WalletQuotaBreakdownRecorded,
		"consume_rebate_settled":          task.ConsumeRebateSettled,
	}).Error
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

// RefundMidjourneyQuota 按原路返回 Midjourney 任务消费的额度。
//
//   - 新任务（有 WalletQuotaBreakdownRecorded）：按 WalletRewardUsed/WalletPaidUsed 原路退回，
//     奖励部分退回 reward_quota，充值部分退回 quota，防止用户通过退款将奖励额度洗成充值额度。
//     同时验证 consumedTotal == Quota 确保快照一致性，不一致则报错拒绝退款。
//   - 旧任务（无快照）：兼容旧行为，全部退回 quota。
//
// 退款成功后清零快照字段并标记 ConsumeRebateSettled=1，防止重复处理。
func RefundMidjourneyQuota(task *model.Midjourney) error {
	if task == nil || task.Quota <= 0 {
		return nil
	}
	if task.WalletQuotaBreakdownRecorded != 1 {
		return model.IncreaseUserQuota(task.UserId, task.Quota, false)
	}
	consumedTotal := task.WalletRewardUsed + task.WalletPaidUsed
	if consumedTotal != task.Quota {
		return fmt.Errorf("Midjourney wallet snapshot mismatch: quota=%d consumed=%d", task.Quota, consumedTotal)
	}
	if err := model.IncreaseUserQuotaByBreakdown(task.UserId, consumedTotal, task.WalletRewardUsed); err != nil {
		return err
	}
	task.WalletRewardUsed = 0
	task.WalletPaidUsed = 0
	task.ConsumeRebateSettled = 1
	return persistMidjourneyBillingState(task)
}

// taskAdjustTokenQuota 调整任务的令牌额度，delta > 0 表示扣费，delta < 0 表示退还。
// 需要通过 resolveTokenKey 运行时获取 key（不从 PrivateData 中读取）。
func taskAdjustTokenQuota(ctx context.Context, task *model.Task, delta int) {
	if task.PrivateData.TokenId <= 0 || delta == 0 {
		return
	}
	tokenKey := resolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		return
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
	quota := task.Quota
	if quota == 0 {
		return
	}

	// 1. 退还资金来源（钱包或订阅）
	if err := taskAdjustFunding(task, -quota); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("退还资金来源失败 task %s: %s", task.TaskID, err.Error()))
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
	taskAdjustTokenQuota(ctx, task, -quota)

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
	if actualQuota <= 0 {
		return
	}
	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota

	if quotaDelta == 0 {
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

	// 调整资金来源
	if err := taskAdjustFunding(task, quotaDelta); err != nil {
		logger.LogError(ctx, fmt.Sprintf("差额结算资金调整失败 task %s: %s", task.TaskID, err.Error()))
		return
	}

	// 调整令牌额度
	taskAdjustTokenQuota(ctx, task, quotaDelta)

	task.Quota = actualQuota

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
