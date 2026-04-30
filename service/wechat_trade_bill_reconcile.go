package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// WechatTradeBillReconcileSummary 表示一次对账执行后的汇总结果。
type WechatTradeBillReconcileSummary struct {
	TotalCount    int64 `json:"total_count"`
	MatchedCount  int64 `json:"matched_count"`
	AbnormalCount int64 `json:"abnormal_count"`
}

// WechatTradeBillReconcileService 负责微信账单与本地订单的核对逻辑。
// 当前策略：
// 1. 先把当天所有 wxpay 成功本地订单写入 payment_bill_reconcile；
// 2. 再用微信账单逐条匹配本地记录并更新结果；
// 3. 微信账单里找不到本地订单的，补充一条渠道单边异常记录。
type WechatTradeBillReconcileService struct{}

func NewWechatTradeBillReconcileService() *WechatTradeBillReconcileService {
	return &WechatTradeBillReconcileService{}
}

func parseWechatAmount(row *model.PaymentBillRecord) (string, float64) {
	candidates := []string{
		strings.TrimSpace(row.OrderAmount),
		strings.TrimSpace(row.TotalAmount),
		strings.TrimSpace(row.ApplyRefundAmount),
		strings.TrimSpace(row.RefundAmount),
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

func amountsEqual(a float64, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= 0.000001
}

func normalizeWechatTradeStatus(row *model.PaymentBillRecord) string {
	if strings.TrimSpace(row.RefundStatus) != "" {
		return strings.ToLower(strings.TrimSpace(row.RefundStatus))
	}
	status := strings.ToUpper(strings.TrimSpace(row.TradeStatus))
	switch status {
	case "SUCCESS":
		return "success"
	case "REFUND":
		return "refund"
	case "CLOSED", "REVOKED", "PAYERROR":
		return "failed"
	case "USERPAYING", "NOTPAY":
		return "pending"
	default:
		return strings.ToLower(strings.TrimSpace(row.TradeStatus))
	}
}

func normalizeLocalStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func reconcileKeyForLocalRecord(billDate string, localType string, localID int) string {
	return fmt.Sprintf("local:%s:%s:%d", strings.TrimSpace(billDate), strings.TrimSpace(localType), localID)
}

func reconcileKeyForChannelRecord(billDate string, billRecordID int) string {
	return fmt.Sprintf("channel:%s:%d", strings.TrimSpace(billDate), billRecordID)
}

func getDayTimeRange(billDate string) (int64, int64, error) {
	location := time.Local
	startTime, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(billDate), location)
	if err != nil {
		return 0, 0, err
	}
	endTime := startTime.AddDate(0, 0, 1)
	return startTime.Unix(), endTime.Unix(), nil
}

func (s *WechatTradeBillReconcileService) loadLocalSuccessRecords(billDate string) ([]*model.PaymentBillReconcile, error) {
	startTS, endTS, err := getDayTimeRange(billDate)
	if err != nil {
		return nil, err
	}

	records := make([]*model.PaymentBillReconcile, 0)

	var topups []model.TopUp
	if err := model.DB.
		Where("payment_method = ? AND status = ? AND biz_type = ? AND complete_time >= ? AND complete_time < ?",
			model.PaymentChannelTypeWechat, common.TopUpStatusSuccess, model.TopUpBizTypePayment, startTS, endTS).
		Find(&topups).Error; err != nil {
		return nil, err
	}
	for _, topup := range topups {
		records = append(records, &model.PaymentBillReconcile{
			ChannelType:        model.PaymentChannelTypeWechat,
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
			LocalCreateTime:    topup.CreateTime,
			LocalCompleteTime:  topup.CompleteTime,
			ReconcileStatus:    model.PaymentReconcileStatusAbnormal,
			ReconcileReason:    model.PaymentReconcileReasonChannelNotFound,
			Remark:             "local successful wxpay topup not matched in wechat bill",
		})
	}

	var orders []model.SubscriptionOrder
	if err := model.DB.
		Where("payment_method = ? AND status = ? AND complete_time >= ? AND complete_time < ?",
			model.PaymentChannelTypeWechat, common.TopUpStatusSuccess, startTS, endTS).
		Find(&orders).Error; err != nil {
		return nil, err
	}
	for _, order := range orders {
		records = append(records, &model.PaymentBillReconcile{
			ChannelType:        model.PaymentChannelTypeWechat,
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
			LocalCreateTime:    order.CreateTime,
			LocalCompleteTime:  order.CompleteTime,
			ReconcileStatus:    model.PaymentReconcileStatusAbnormal,
			ReconcileReason:    model.PaymentReconcileReasonChannelNotFound,
			Remark:             "local successful wxpay subscription not matched in wechat bill",
		})
	}

	return records, nil
}

func (s *WechatTradeBillReconcileService) applyChannelMatch(record *model.PaymentBillReconcile, row *model.PaymentBillRecord) {
	record.RecordSource = "local"
	record.BillRecordId = row.Id
	record.BillDate = row.BillDate
	record.TradeTime = row.TradeTime
	record.ChannelTradeNo = row.ChannelTradeNo
	record.MerchantTradeNo = row.MerchantTradeNo
	record.TradeType = row.TradeType
	record.ChannelStatus = row.TradeStatus
	record.ChannelRefundStatus = row.RefundStatus
	record.ChannelRefundAmount = row.RefundAmount

	amountText, wechatAmount := parseWechatAmount(row)
	record.ChannelAmount = amountText

	if strings.TrimSpace(row.TradeStatus) == "" && strings.TrimSpace(row.RefundStatus) == "" {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonUnsupportedBillRow
		record.Remark = "wechat bill row has empty trade status"
		return
	}

	if !amountsEqual(wechatAmount, record.LocalAmount) {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonAmountMismatch
		record.Remark = fmt.Sprintf("wechat_amount=%s local_amount=%.6f", amountText, record.LocalAmount)
		return
	}

	wechatStatus := normalizeWechatTradeStatus(row)
	localStatus := normalizeLocalStatus(record.LocalStatus)
	if wechatStatus != localStatus {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonStatusMismatch
		record.Remark = fmt.Sprintf("wechat_status=%s local_status=%s", wechatStatus, localStatus)
		return
	}

	record.ReconcileStatus = model.PaymentReconcileStatusMatched
	record.ReconcileReason = model.PaymentReconcileReasonMatched
	record.Remark = "matched by trade number and amount"
}

func (s *WechatTradeBillReconcileService) buildChannelOnlyRecord(row *model.PaymentBillRecord) *model.PaymentBillReconcile {
	record := &model.PaymentBillReconcile{
		ChannelType:         model.PaymentChannelTypeWechat,
		ReconcileKey:        reconcileKeyForChannelRecord(row.BillDate, row.Id),
		RecordSource:        "channel",
		BillRecordId:        row.Id,
		BillDate:            row.BillDate,
		TradeTime:           row.TradeTime,
		ChannelTradeNo:      row.ChannelTradeNo,
		MerchantTradeNo:     row.MerchantTradeNo,
		TradeType:           row.TradeType,
		ChannelStatus:       row.TradeStatus,
		ChannelRefundStatus: row.RefundStatus,
		ChannelRefundAmount: row.RefundAmount,
		LocalPaymentMethod:  model.PaymentChannelTypeWechat,
		ReconcileStatus:     model.PaymentReconcileStatusAbnormal,
		ReconcileReason:     model.PaymentReconcileReasonLocalNotFound,
		Remark:              "wechat bill exists but local wxpay order not found",
	}
	amountText, _ := parseWechatAmount(row)
	record.ChannelAmount = amountText
	if strings.TrimSpace(row.TradeStatus) == "" && strings.TrimSpace(row.RefundStatus) == "" {
		record.ReconcileReason = model.PaymentReconcileReasonUnsupportedBillRow
		record.Remark = "wechat bill row has empty trade status"
	}
	return record
}

func indexLocalRecordsByTradeNo(records []*model.PaymentBillReconcile) map[string][]*model.PaymentBillReconcile {
	result := make(map[string][]*model.PaymentBillReconcile)
	for _, record := range records {
		if record == nil || strings.TrimSpace(record.LocalTradeNo) == "" {
			continue
		}
		tradeNo := strings.TrimSpace(record.LocalTradeNo)
		result[tradeNo] = append(result[tradeNo], record)
	}
	return result
}

// ReconcileByBillDateRange 对指定日期范围执行微信对账。
// 当前按日使用，因此会逐天处理，便于和本地成功订单的日期归属保持一致。
func (s *WechatTradeBillReconcileService) ReconcileByBillDateRange(billDateFrom string, billDateTo string) (*WechatTradeBillReconcileSummary, error) {
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

	summary := &WechatTradeBillReconcileSummary{}
	for current := fromTime; !current.After(toTime); current = current.AddDate(0, 0, 1) {
		billDate := current.Format("2006-01-02")

		localRecords, err := s.loadLocalSuccessRecords(billDate)
		if err != nil {
			return nil, err
		}

		rows, err := model.GetPaymentBillRecordsByChannelAndBillDateRange(model.PaymentChannelTypeWechat, billDate, billDate)
		if err != nil {
			return nil, err
		}

		localByTradeNo := indexLocalRecordsByTradeNo(localRecords)
		channelOnlyRecords := make([]*model.PaymentBillReconcile, 0)

		for _, row := range rows {
			if row == nil {
				continue
			}

			candidates := make([]*model.PaymentBillReconcile, 0)
			if tradeNo := strings.TrimSpace(row.MerchantTradeNo); tradeNo != "" {
				candidates = append(candidates, localByTradeNo[tradeNo]...)
			}
			if tradeNo := strings.TrimSpace(row.ChannelTradeNo); tradeNo != "" && tradeNo != strings.TrimSpace(row.MerchantTradeNo) {
				candidates = append(candidates, localByTradeNo[tradeNo]...)
			}

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
				channelOnlyRecords = append(channelOnlyRecords, s.buildChannelOnlyRecord(row))
			case 1:
				s.applyChannelMatch(uniqueCandidates[0], row)
			default:
				for _, candidate := range uniqueCandidates {
					candidate.BillRecordId = row.Id
					candidate.TradeTime = row.TradeTime
					candidate.ChannelTradeNo = row.ChannelTradeNo
					candidate.MerchantTradeNo = row.MerchantTradeNo
					candidate.TradeType = row.TradeType
					candidate.ChannelStatus = row.TradeStatus
					candidate.ChannelRefundStatus = row.RefundStatus
					amountText, _ := parseWechatAmount(row)
					candidate.ChannelAmount = amountText
					candidate.ChannelRefundAmount = row.RefundAmount
					candidate.ReconcileStatus = model.PaymentReconcileStatusAbnormal
					candidate.ReconcileReason = model.PaymentReconcileReasonDuplicateLocal
					candidate.Remark = fmt.Sprintf("matched %d local records by trade number", len(uniqueCandidates))
				}
			}
		}

		results := make([]*model.PaymentBillReconcile, 0, len(localRecords)+len(channelOnlyRecords))
		results = append(results, localRecords...)
		results = append(results, channelOnlyRecords...)
		if _, err := model.UpsertPaymentBillReconciles(results); err != nil {
			return nil, err
		}

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

// ReconcileWechatTradeBillsByBillDateRange 提供无状态的微信账单对账入口。
func ReconcileWechatTradeBillsByBillDateRange(billDateFrom string, billDateTo string) (*WechatTradeBillReconcileSummary, error) {
	return NewWechatTradeBillReconcileService().ReconcileByBillDateRange(billDateFrom, billDateTo)
}
