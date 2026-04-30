package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

// WechatTradeBillReconcileSummary 表示一次对账执行后的汇总结果。
type WechatTradeBillReconcileSummary struct {
	TotalCount    int64 `json:"total_count"`
	MatchedCount  int64 `json:"matched_count"`
	AbnormalCount int64 `json:"abnormal_count"`
}

// localTradeMatch 表示从本地订单表中匹配到的一条候选记录。
type localTradeMatch struct {
	LocalType          string
	LocalID            int
	LocalTradeNo       string
	LocalPaymentMethod string
	LocalStatus        string
	LocalAmount        float64
	LocalCreateTime    int64
	LocalCompleteTime  int64
}

// WechatTradeBillReconcileService 负责微信账单与本地订单的核对逻辑。
type WechatTradeBillReconcileService struct{}

func NewWechatTradeBillReconcileService() *WechatTradeBillReconcileService {
	return &WechatTradeBillReconcileService{}
}

// parseWechatAmount 从微信账单行中提取一个可用于对账比较的金额。
// 优先顺序依次为：订单金额、总金额、申请退款金额、退款金额。
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

// amountsEqual 判断两个金额是否近似相等，避免浮点精度误差导致误判。
func amountsEqual(a float64, b float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= 0.000001
}

// normalizeWechatTradeStatus 将微信账单中的交易/退款状态归一化为本地对账状态。
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
	case "CLOSED":
		return "failed"
	case "REVOKED":
		return "failed"
	case "PAYERROR":
		return "failed"
	case "USERPAYING":
		return "pending"
	case "NOTPAY":
		return "pending"
	default:
		return strings.ToLower(strings.TrimSpace(row.TradeStatus))
	}
}

// normalizeLocalStatus 将本地订单状态规整为小写，便于和微信状态比较。
func normalizeLocalStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

// findSubscriptionOrder 先按商户订单号、再按微信订单号去订阅订单表中匹配。
func (s *WechatTradeBillReconcileService) findSubscriptionOrder(row *model.PaymentBillRecord) ([]localTradeMatch, error) {
	candidates := []string{}
	if strings.TrimSpace(row.MerchantTradeNo) != "" {
		candidates = append(candidates, strings.TrimSpace(row.MerchantTradeNo))
	}
	if strings.TrimSpace(row.ChannelTradeNo) != "" && strings.TrimSpace(row.ChannelTradeNo) != strings.TrimSpace(row.MerchantTradeNo) {
		candidates = append(candidates, strings.TrimSpace(row.ChannelTradeNo))
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	var orders []model.SubscriptionOrder
	if err := model.DB.Where("trade_no IN ?", candidates).Find(&orders).Error; err != nil {
		return nil, err
	}
	results := make([]localTradeMatch, 0, len(orders))
	for _, order := range orders {
		results = append(results, localTradeMatch{
			LocalType:          "subscription",
			LocalID:            order.Id,
			LocalTradeNo:       order.TradeNo,
			LocalPaymentMethod: order.PaymentMethod,
			LocalStatus:        order.Status,
			LocalAmount:        order.Money,
			LocalCreateTime:    order.CreateTime,
			LocalCompleteTime:  order.CompleteTime,
		})
	}
	return results, nil
}

// findTopUp 再按商户订单号、微信订单号去充值订单表中匹配。
func (s *WechatTradeBillReconcileService) findTopUp(row *model.PaymentBillRecord) ([]localTradeMatch, error) {
	candidates := []string{}
	if strings.TrimSpace(row.MerchantTradeNo) != "" {
		candidates = append(candidates, strings.TrimSpace(row.MerchantTradeNo))
	}
	if strings.TrimSpace(row.ChannelTradeNo) != "" && strings.TrimSpace(row.ChannelTradeNo) != strings.TrimSpace(row.MerchantTradeNo) {
		candidates = append(candidates, strings.TrimSpace(row.ChannelTradeNo))
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	var topups []model.TopUp
	if err := model.DB.Where("trade_no IN ?", candidates).Find(&topups).Error; err != nil {
		return nil, err
	}
	results := make([]localTradeMatch, 0, len(topups))
	for _, topup := range topups {
		results = append(results, localTradeMatch{
			LocalType:          "topup",
			LocalID:            topup.Id,
			LocalTradeNo:       topup.TradeNo,
			LocalPaymentMethod: topup.PaymentMethod,
			LocalStatus:        topup.Status,
			// 充值对账按 top_ups.amount 比对，不再按 top_ups.money 比对。
			LocalAmount:       topup.OriginalMoney,
			LocalCreateTime:   topup.CreateTime,
			LocalCompleteTime: topup.CompleteTime,
		})
	}
	return results, nil
}

// findLocalTrade 先匹配订阅订单，未命中时再匹配充值订单。
func (s *WechatTradeBillReconcileService) findLocalTrade(row *model.PaymentBillRecord) ([]localTradeMatch, error) {
	subscriptionMatches, err := s.findSubscriptionOrder(row)
	if err != nil {
		return nil, err
	}
	if len(subscriptionMatches) > 0 {
		return subscriptionMatches, nil
	}
	return s.findTopUp(row)
}

// buildReconcileRecord 根据单条微信账单构造一条对账结果。
// 这里会完成本地订单匹配、金额比对、状态比对，并给出异常原因。
func (s *WechatTradeBillReconcileService) buildReconcileRecord(row *model.PaymentBillRecord) (*model.PaymentBillReconcile, error) {
	record := &model.PaymentBillReconcile{
		ChannelType:         model.PaymentChannelTypeWechat,
		BillRecordId:        row.Id,
		BillDate:            row.BillDate,
		TradeTime:           row.TradeTime,
		ChannelTradeNo:      row.ChannelTradeNo,
		MerchantTradeNo:     row.MerchantTradeNo,
		TradeType:           row.TradeType,
		ChannelStatus:       row.TradeStatus,
		ChannelRefundStatus: row.RefundStatus,
		ChannelRefundAmount: row.RefundAmount,
	}
	amountText, wechatAmount := parseWechatAmount(row)
	record.ChannelAmount = amountText

	if strings.TrimSpace(row.TradeStatus) == "" && strings.TrimSpace(row.RefundStatus) == "" {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonUnsupportedBillRow
		record.Remark = "wechat bill row has empty trade status"
		return record, nil
	}

	matches, err := s.findLocalTrade(row)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonLocalNotFound
		record.Remark = "no local topup or subscription order matched by wechat_trade_no / merchant_trade_no"
		return record, nil
	}
	if len(matches) > 1 {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonDuplicateLocal
		record.Remark = fmt.Sprintf("matched %d local records", len(matches))
		return record, nil
	}

	match := matches[0]
	record.LocalType = match.LocalType
	record.LocalId = match.LocalID
	record.LocalTradeNo = match.LocalTradeNo
	record.LocalPaymentMethod = match.LocalPaymentMethod
	record.LocalStatus = match.LocalStatus
	record.LocalAmount = match.LocalAmount
	record.LocalCreateTime = match.LocalCreateTime
	record.LocalCompleteTime = match.LocalCompleteTime

	if !amountsEqual(wechatAmount, match.LocalAmount) {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonAmountMismatch
		record.Remark = fmt.Sprintf("wechat_amount=%s local_amount=%.6f", amountText, match.LocalAmount)
		return record, nil
	}

	wechatStatus := normalizeWechatTradeStatus(row)
	localStatus := normalizeLocalStatus(match.LocalStatus)
	if wechatStatus != localStatus {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonStatusMismatch
		record.Remark = fmt.Sprintf("wechat_status=%s local_status=%s", wechatStatus, localStatus)
		return record, nil
	}

	record.ReconcileStatus = model.PaymentReconcileStatusMatched
	record.ReconcileReason = model.PaymentReconcileReasonMatched
	record.Remark = "matched by trade number and amount"
	return record, nil
}

// ReconcileByBillDateRange 对指定账单日期范围内的微信账单逐条执行对账，并批量写入结果表。
func (s *WechatTradeBillReconcileService) ReconcileByBillDateRange(billDateFrom string, billDateTo string) (*WechatTradeBillReconcileSummary, error) {
	rows, err := model.GetPaymentBillRecordsByChannelAndBillDateRange(model.PaymentChannelTypeWechat, billDateFrom, billDateTo)
	if err != nil {
		return nil, err
	}
	results := make([]*model.PaymentBillReconcile, 0, len(rows))
	summary := &WechatTradeBillReconcileSummary{}
	for _, row := range rows {
		record, err := s.buildReconcileRecord(row)
		if err != nil {
			return nil, err
		}
		results = append(results, record)
		summary.TotalCount++
		if record.ReconcileStatus == model.PaymentReconcileStatusMatched {
			summary.MatchedCount++
		} else {
			summary.AbnormalCount++
		}
	}
	if _, err := model.UpsertPaymentBillReconciles(results); err != nil {
		return nil, err
	}
	return summary, nil
}

// ReconcileWechatTradeBillsByBillDateRange 提供无状态的微信账单对账入口。
func ReconcileWechatTradeBillsByBillDateRange(billDateFrom string, billDateTo string) (*WechatTradeBillReconcileSummary, error) {
	return NewWechatTradeBillReconcileService().ReconcileByBillDateRange(billDateFrom, billDateTo)
}
