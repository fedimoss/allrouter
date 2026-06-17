// Package service 提供支付宝账单与本地订单的对账逻辑。
// 对账策略与微信一致（详见 wechat_trade_bill_reconcile.go）：
//  1. 先把当天所有 alipay 成功本地订单（充值 topup / 订阅 subscription）写入 payment_bill_reconcile，初始状态为"渠道未找到"；
//  2. 再用支付宝业务明细逐条匹配本地记录并更新结果；
//  3. 支付宝账单里找不到本地订单的，补充一条渠道单边异常记录。
//
// 匹配键：支付宝业务明细的"商户订单号" == 本地订单 TradeNo（易支付下单时作为 out_trade_no 透传）。
package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// AlipayTradeBillReconcileSummary 表示一次支付宝对账执行后的汇总结果。
type AlipayTradeBillReconcileSummary struct {
	TotalCount    int64 `json:"total_count"`
	MatchedCount  int64 `json:"matched_count"`
	AbnormalCount int64 `json:"abnormal_count"`
}

// AlipayTradeBillReconcileService 负责支付宝账单与本地订单的核对逻辑。
type AlipayTradeBillReconcileService struct{}

func NewAlipayTradeBillReconcileService() *AlipayTradeBillReconcileService {
	return &AlipayTradeBillReconcileService{}
}

// parseAlipayAmount 从账单记录中解析金额，依次尝试订单金额、商家实收。
func parseAlipayAmount(row *model.PaymentBillRecord) (string, float64) {
	candidates := []string{
		strings.TrimSpace(row.OrderAmount),
		strings.TrimSpace(row.TotalAmount),
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		val, err := strconv.ParseFloat(candidate, 64)
		if err == nil {
			return candidate, val
		}
	}
	return "", 0
}

// normalizeAlipayTradeStatus 规范化支付宝账单交易状态。
// 支付宝业务明细无显式状态列，已在解析阶段按业务类型推导（交易→success，退款→refund）。
func normalizeAlipayTradeStatus(row *model.PaymentBillRecord) string {
	status := strings.ToLower(strings.TrimSpace(row.TradeStatus))
	if status != "" {
		return status
	}
	// 兜底：依据业务类型推导（解析阶段已写入 TradeStatus，正常不会走到这里）
	return deriveAlipayTradeStatus(row.TradeType)
}

// loadLocalSuccessRecords 加载当天所有成功的本地 alipay 订单（充值 + 订阅），
// 每条生成一条初始状态为"渠道未找到"的对账记录。
func (s *AlipayTradeBillReconcileService) loadLocalSuccessRecords(billDate string) ([]*model.PaymentBillReconcile, error) {
	startTS, endTS, err := getDayTimeRange(billDate)
	if err != nil {
		return nil, err
	}

	records := make([]*model.PaymentBillReconcile, 0)

	// 充值订单：payment_method=alipay 且 biz_type=payment 且成功
	var topups []model.TopUp
	if err := model.DB.
		Where("payment_method = ? AND status = ? AND biz_type = ? AND complete_time >= ? AND complete_time < ?",
			model.PaymentChannelTypeAlipay, common.TopUpStatusSuccess, model.TopUpBizTypePayment, startTS, endTS).
		Find(&topups).Error; err != nil {
		return nil, err
	}
	for _, topup := range topups {
		records = append(records, &model.PaymentBillReconcile{
			ChannelType:        model.PaymentChannelTypeAlipay,
			ReconcileKey:       reconcileKeyForLocalRecord(billDate, "topup", topup.Id),
			RecordSource:       "local",
			BillDate:           billDate,
			MerchantTradeNo:    topup.TradeNo,
			LocalType:          "topup",
			LocalId:            topup.Id,
			LocalTradeNo:       topup.TradeNo,
			LocalPaymentMethod: topup.PaymentMethod,
			LocalStatus:        topup.Status,
			LocalAmount:        topup.OriginalMoney,
			LocalCurrency:      "¥", // 支付宝固定人民币
			LocalCreateTime:    topup.CreateTime,
			LocalCompleteTime:  topup.CompleteTime,
			ReconcileStatus:    model.PaymentReconcileStatusAbnormal,
			ReconcileReason:    model.PaymentReconcileReasonChannelNotFound,
			Remark:             "local successful alipay topup not matched in alipay bill",
		})
	}

	// 订阅订单：payment_method=alipay 且成功
	var orders []model.SubscriptionOrder
	if err := model.DB.
		Where("payment_method = ? AND status = ? AND complete_time >= ? AND complete_time < ?",
			model.PaymentChannelTypeAlipay, common.TopUpStatusSuccess, startTS, endTS).
		Find(&orders).Error; err != nil {
		return nil, err
	}
	for _, order := range orders {
		records = append(records, &model.PaymentBillReconcile{
			ChannelType:        model.PaymentChannelTypeAlipay,
			ReconcileKey:       reconcileKeyForLocalRecord(billDate, "subscription", order.Id),
			RecordSource:       "local",
			BillDate:           billDate,
			MerchantTradeNo:    order.TradeNo,
			LocalType:          "subscription",
			LocalId:            order.Id,
			LocalTradeNo:       order.TradeNo,
			LocalPaymentMethod: order.PaymentMethod,
			LocalStatus:        order.Status,
			LocalAmount:        order.OriginalMoney,
			LocalCurrency:      "¥", // 支付宝固定人民币
			LocalCreateTime:    order.CreateTime,
			LocalCompleteTime:  order.CompleteTime,
			ReconcileStatus:    model.PaymentReconcileStatusAbnormal,
			ReconcileReason:    model.PaymentReconcileReasonChannelNotFound,
			Remark:             "local successful alipay subscription not matched in alipay bill",
		})
	}

	return records, nil
}

// applyChannelMatch 将支付宝账单记录的信息填充到本地对账记录中，并判断对账结果。
// 判断顺序：空状态→不支持；金额不一致→金额异常；状态不一致→状态异常；否则一致。
func (s *AlipayTradeBillReconcileService) applyChannelMatch(record *model.PaymentBillReconcile, row *model.PaymentBillRecord) {
	record.RecordSource = "local"
	record.BillRecordId = row.Id
	record.BillDate = row.BillDate
	record.TradeTime = row.TradeTime
	record.ChannelTradeNo = row.ChannelTradeNo
	record.MerchantTradeNo = row.MerchantTradeNo
	// 订单类别按商户订单号前缀推导(payment/subscription)，不直接复制账单的 TradeType，
	// 确保对账记录不再出现原始"交易"/"退款"。
	record.TradeType = deriveAlipayTradeTypeFromMerchantOrderNo(row.MerchantTradeNo)
	record.ChannelStatus = row.TradeStatus
	record.ChannelCurrency = "CNY" // 支付宝固定人民币

	amountText, alipayAmount := parseAlipayAmount(row)
	record.ChannelAmount = amountText

	if strings.TrimSpace(row.TradeStatus) == "" {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonUnsupportedBillRow
		record.Remark = "alipay bill row has empty trade status"
		return
	}

	if !amountsEqual(alipayAmount, record.LocalAmount) {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonAmountMismatch
		record.Remark = fmt.Sprintf("alipay_amount=%s local_amount=%.6f", amountText, record.LocalAmount)
		return
	}

	alipayStatus := normalizeAlipayTradeStatus(row)
	localStatus := normalizeLocalStatus(record.LocalStatus)
	if alipayStatus != localStatus {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonStatusMismatch
		record.Remark = fmt.Sprintf("alipay_status=%s local_status=%s", alipayStatus, localStatus)
		return
	}

	record.ReconcileStatus = model.PaymentReconcileStatusMatched
	record.ReconcileReason = model.PaymentReconcileReasonMatched
	record.Remark = "matched by trade number and amount"
}

// buildChannelOnlyRecord 为支付宝账单中存在、但本地找不到对应订单的记录生成一条渠道单边异常记录。
func (s *AlipayTradeBillReconcileService) buildChannelOnlyRecord(row *model.PaymentBillRecord) *model.PaymentBillReconcile {
	record := &model.PaymentBillReconcile{
		ChannelType:        model.PaymentChannelTypeAlipay,
		ReconcileKey:       reconcileKeyForChannelRecord(row.BillDate, row.Id),
		RecordSource:       "channel",
		BillRecordId:       row.Id,
		BillDate:           row.BillDate,
		TradeTime:          row.TradeTime,
		ChannelTradeNo:     row.ChannelTradeNo,
		MerchantTradeNo:    row.MerchantTradeNo,
		TradeType:          deriveAlipayTradeTypeFromMerchantOrderNo(row.MerchantTradeNo),
		ChannelStatus:      row.TradeStatus,
		ChannelCurrency:    "CNY", // 支付宝固定人民币
		LocalPaymentMethod: model.PaymentChannelTypeAlipay,
		ReconcileStatus:    model.PaymentReconcileStatusAbnormal,
		ReconcileReason:    model.PaymentReconcileReasonLocalNotFound,
		Remark:             "alipay bill exists but local alipay order not found",
	}
	amountText, _ := parseAlipayAmount(row)
	record.ChannelAmount = amountText
	if strings.TrimSpace(row.TradeStatus) == "" {
		record.ReconcileReason = model.PaymentReconcileReasonUnsupportedBillRow
		record.Remark = "alipay bill row has empty trade status"
	}
	return record
}

// ReconcileByBillDateRange 对指定日期范围执行支付宝对账。
func (s *AlipayTradeBillReconcileService) ReconcileByBillDateRange(billDateFrom string, billDateTo string) (*AlipayTradeBillReconcileSummary, error) {
	fromTime, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(billDateFrom), time.Local)
	if err != nil {
		return nil, err
	}
	toTime, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(billDateTo), time.Local)
	if err != nil {
		return nil, err
	}
	if toTime.Before(fromTime) {
		return nil, fmt.Errorf("bill date range is invalid")
	}

	summary := &AlipayTradeBillReconcileSummary{}
	for current := fromTime; !current.After(toTime); current = current.AddDate(0, 0, 1) {
		billDate := current.Format("2006-01-02")

		// 1. 加载当天所有成功的本地 alipay 订单（初始状态为"渠道未找到"）
		localRecords, err := s.loadLocalSuccessRecords(billDate)
		if err != nil {
			return nil, err
		}

		// 2. 查询当天已入库的支付宝业务明细
		rows, err := model.GetPaymentBillRecordsByChannelAndBillDateRange(model.PaymentChannelTypeAlipay, billDate, billDate)
		if err != nil {
			return nil, err
		}

		localByTradeNo := indexLocalRecordsByTradeNo(localRecords)
		channelOnlyRecords := make([]*model.PaymentBillReconcile, 0)

		// 3. 逐条用支付宝业务明细匹配本地订单
		for _, row := range rows {
			if row == nil {
				continue
			}

			// 按"商户订单号"(== 本地 TradeNo) 为主、"支付宝交易号"为辅查找候选本地记录
			candidates := make([]*model.PaymentBillReconcile, 0)
			if tradeNo := strings.TrimSpace(row.MerchantTradeNo); tradeNo != "" {
				candidates = append(candidates, localByTradeNo[tradeNo]...)
			}
			if tradeNo := strings.TrimSpace(row.ChannelTradeNo); tradeNo != "" && tradeNo != strings.TrimSpace(row.MerchantTradeNo) {
				candidates = append(candidates, localByTradeNo[tradeNo]...)
			}

			// 去重候选（同一 ReconcileKey 只保留一次）
			uniqueCandidates := make([]*model.PaymentBillReconcile, 0, len(candidates))
			seen := make(map[string]struct{})
			for _, candidate := range candidates {
				if candidate == nil {
					continue
				}
				if _, ok := seen[candidate.ReconcileKey]; ok {
					continue
				}
				seen[candidate.ReconcileKey] = struct{}{}
				uniqueCandidates = append(uniqueCandidates, candidate)
			}

			switch len(uniqueCandidates) {
			case 0:
				// 渠道有、本地无 → 渠道单边异常
				channelOnlyRecords = append(channelOnlyRecords, s.buildChannelOnlyRecord(row))
			case 1:
				// 唯一匹配 → 校验金额/状态
				s.applyChannelMatch(uniqueCandidates[0], row)
			default:
				// 匹配到多条本地订单 → 重复匹配异常
				for _, candidate := range uniqueCandidates {
					candidate.BillRecordId = row.Id
					candidate.TradeTime = row.TradeTime
					candidate.ChannelTradeNo = row.ChannelTradeNo
					candidate.MerchantTradeNo = row.MerchantTradeNo
					candidate.TradeType = deriveAlipayTradeTypeFromMerchantOrderNo(row.MerchantTradeNo)
					candidate.ChannelStatus = row.TradeStatus
					candidate.ChannelCurrency = "CNY" // 支付宝固定人民币
					amountText, _ := parseAlipayAmount(row)
					candidate.ChannelAmount = amountText
					candidate.ReconcileStatus = model.PaymentReconcileStatusAbnormal
					candidate.ReconcileReason = model.PaymentReconcileReasonDuplicateLocal
					candidate.Remark = fmt.Sprintf("matched %d local records by trade number", len(uniqueCandidates))
				}
			}
		}

		// 4. 批量写入对账结果（按 channel_type + reconcile_key 幂等覆盖）
		results := make([]*model.PaymentBillReconcile, 0, len(localRecords)+len(channelOnlyRecords))
		results = append(results, localRecords...)
		results = append(results, channelOnlyRecords...)
		if _, err := model.UpsertPaymentBillReconciles(results); err != nil {
			return nil, err
		}

		// 5. 汇总
		for _, record := range results {
			if record == nil {
				continue
			}
			summary.TotalCount++
			if record.ReconcileStatus == model.PaymentReconcileStatusMatched {
				summary.MatchedCount++
			} else {
				summary.AbnormalCount++
			}
		}
	}

	return summary, nil
}

// ReconcileAlipayTradeBillsByBillDateRange 提供无状态的支付宝账单对账入口。
func ReconcileAlipayTradeBillsByBillDateRange(billDateFrom string, billDateTo string) (*AlipayTradeBillReconcileSummary, error) {
	return NewAlipayTradeBillReconcileService().ReconcileByBillDateRange(billDateFrom, billDateTo)
}
