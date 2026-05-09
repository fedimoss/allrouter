package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// stripeChannelType Stripe 渠道标识，对应 payment_bill_record / payment_bill_reconcile 表中的 channel_type 值。
const stripeChannelType = "stripe"

// StripeTradeBillReconcileSummary Stripe 对账汇总结果，统计对账总数、匹配数和异常数。
type StripeTradeBillReconcileSummary struct {
	TotalCount    int64 `json:"total_count"`    // 对账总记录数
	MatchedCount  int64 `json:"matched_count"`  // 匹配成功数
	AbnormalCount int64 `json:"abnormal_count"` // 异常数（金额不一致、状态不匹配、单边记录等）
}

type StripeTradeBillReconcileService struct{}

// NewStripeTradeBillReconcileService 创建 Stripe 对账服务实例。
func NewStripeTradeBillReconcileService() *StripeTradeBillReconcileService {
	return &StripeTradeBillReconcileService{}
}

// parseStripeAmount 从 Stripe 账单行中提取金额，优先使用 OrderAmount，其次 TotalAmount。
func parseStripeAmount(row *model.PaymentBillRecord) (string, float64) {
	candidates := []string{
		strings.TrimSpace(row.OrderAmount), // 优先：订单金额
		strings.TrimSpace(row.TotalAmount), // 其次：总金额
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		val, err := strconv.ParseFloat(candidate, 64)
		if err == nil {
			return candidate, val // 返回字符串原始值 + 解析后的浮点数
		}
	}
	return "", 0 // 无有效金额时返回空和零
}

// normalizeStripeCurrency 将币种符号或代码归一化为标准的 3 位大写币种代码。
// 本地订单的 Currency 存储的是符号（如 $、¥），Stripe 账单存储的是代码（如 usd、CNY），
// 对账时需要归一化后再比较。
func normalizeStripeCurrency(currency string) string {
	s := strings.TrimSpace(currency)
	switch s {
	// 常见符号 → 代码
	case "$":
		return "USD"
	case "¥", "￥":
		return "CNY"
	case "€":
		return "EUR"
	case "£":
		return "GBP"
	case "₩":
		return "KRW"
	case "₹":
		return "INR"
	// 常见小写/混合代码 → 大写
	case "usd", "USD":
		return "USD"
	case "cny", "CNY", "rmb", "RMB":
		return "CNY"
	case "eur", "EUR":
		return "EUR"
	case "gbp", "GBP":
		return "GBP"
	case "jpy", "JPY":
		return "JPY"
	case "krw", "KRW":
		return "KRW"
	case "inr", "INR":
		return "INR"
	case "cad", "CAD":
		return "CAD"
	case "aud", "AUD":
		return "AUD"
	case "hkd", "HKD":
		return "HKD"
	case "sgd", "SGD":
		return "SGD"
	case "chf", "CHF":
		return "CHF"
	case "sek", "SEK":
		return "SEK"
	case "nok", "NOK":
		return "NOK"
	case "dkk", "DKK":
		return "DKK"
	case "nzd", "NZD":
		return "NZD"
	case "mxn", "MXN":
		return "MXN"
	case "brl", "BRL":
		return "BRL"
	default:
		return strings.ToUpper(s) // 未知币种直接转大写
	}
}

// Stripe 状态映射到内部统一状态
func normalizeStripeTradeStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "complete":
		return "success"
	case "expired":
		return "failed"
	case "open":
		return "pending"
	default:
		return s
	}
}

// loadLocalStripeSuccessRecords 使用与拉取 Stripe 数据时相同的 UTC 时间戳查本地库，
// 避免 time.Local 导致的时区偏移（与微信对账的 getDayTimeRange 不同，这里不依赖服务器时区）
func (s *StripeTradeBillReconcileService) loadLocalStripeSuccessRecords(billDate string, startTS, endTS int64) ([]*model.PaymentBillReconcile, error) {
	records := make([]*model.PaymentBillReconcile, 0) // 初始化对账记录切片

	// 查 top_ups 表：stripe 单笔充值，biz_type=payment 过滤掉订阅和兑换码产生的记录
	var topups []model.TopUp // 存储查询到的 topup 记录
	if err := model.DB.      // 使用 GORM 查询 top_ups 表
					Where("payment_method = ? AND status = ? AND biz_type = ? AND complete_time >= ? AND complete_time < ?", // 条件：stripe + 成功 + 纯充值 + 时间范围
							stripeChannelType, common.TopUpStatusSuccess, model.TopUpBizTypePayment, startTS, endTS). // 参数绑定
		Find(&topups).Error; err != nil { // 执行查询
		return nil, err // 查询失败直接返回
	}
	for _, topup := range topups { // 遍历每条 topup 记录
		records = append(records, &model.PaymentBillReconcile{ // 追加到对账记录列表
			ChannelType:        stripeChannelType,                                       // 固定为 stripe
			ReconcileKey:       reconcileKeyForLocalRecord(billDate, "topup", topup.Id), // 唯一键 "local:{billDate}:topup:{id}"
			RecordSource:       "local",                                                 // 来源为本地记录
			BillDate:           billDate,                                                // 对账归属日期
			MerchantTradeNo:    topup.TradeNo,                                           // 商户订单号，用于与渠道账单匹配
			LocalType:          "topup",                                                 // 本地订单类型：充值
			LocalId:            topup.Id,                                                // 本地 topup 记录 ID
			LocalTradeNo:       topup.TradeNo,                                           // 本地交易号
			LocalPaymentMethod: topup.PaymentMethod,                                     // 本地支付方式
			LocalStatus:        topup.Status,                                            // 本地订单状态
			LocalAmount:        topup.OriginalMoney,                                     // 本地订单金额（原始币种）
			LocalCurrency:      topup.Currency,                                          // 本地订单币种
			LocalCreateTime:    topup.CreateTime,                                        // 本地订单创建时间戳
			LocalCompleteTime:  topup.CompleteTime,                                      // 本地订单完成时间戳
			// 默认标记为异常，后续在 ReconcileByBillDateRange 中匹配到渠道账单后会覆盖为 matched
			ReconcileStatus: model.PaymentReconcileStatusAbnormal,                       // 默认异常状态
			ReconcileReason: model.PaymentReconcileReasonChannelNotFound,                // 默认原因：渠道侧未找到
			Remark:          "local successful stripe topup not matched in stripe bill", // 默认备注
		})
	}

	// 查 subscription_orders 表：stripe 订阅支付，complete_time 落在 [startTS, endTS) 范围内
	var orders []model.SubscriptionOrder // 存储查询到的订阅订单
	if err := model.DB.                  // 使用 GORM 查询 subscription_orders 表
						Where("payment_method = ? AND status = ? AND complete_time >= ? AND complete_time < ?", // 条件：stripe + 成功 + 时间范围
							stripeChannelType, common.TopUpStatusSuccess, startTS, endTS). // 参数绑定
		Find(&orders).Error; err != nil { // 执行查询
		return nil, err // 查询失败直接返回
	}
	for _, order := range orders { // 遍历每条订阅订单
		records = append(records, &model.PaymentBillReconcile{ // 追加到对账记录列表
			ChannelType:        stripeChannelType,                                                 // 固定为 stripe
			ReconcileKey:       reconcileKeyForLocalRecord(billDate, "subscription", order.Id),    // 唯一键 "local:{billDate}:subscription:{id}"
			RecordSource:       "local",                                                           // 来源为本地记录
			BillDate:           billDate,                                                          // 对账归属日期
			MerchantTradeNo:    order.TradeNo,                                                     // 商户订单号，用于与渠道账单匹配
			LocalType:          "subscription",                                                    // 本地订单类型：订阅
			LocalId:            order.Id,                                                          // 本地订阅订单 ID
			LocalTradeNo:       order.TradeNo,                                                     // 本地交易号
			LocalPaymentMethod: order.PaymentMethod,                                               // 本地支付方式
			LocalStatus:        order.Status,                                                      // 本地订单状态
			LocalAmount:        order.OriginalMoney,                                               // 本地订单金额（原始币种）
			LocalCurrency:      order.Currency,                                                    // 本地订单币种
			LocalCreateTime:    order.CreateTime,                                                  // 本地订单创建时间戳
			LocalCompleteTime:  order.CompleteTime,                                                // 本地订单完成时间戳
			ReconcileStatus:    model.PaymentReconcileStatusAbnormal,                              // 默认异常状态
			ReconcileReason:    model.PaymentReconcileReasonChannelNotFound,                       // 默认原因：渠道侧未找到
			Remark:             "local successful stripe subscription not matched in stripe bill", // 默认备注
		})
	}

	return records, nil // 返回所有本地成功记录
}

// applyChannelMatch 将渠道账单行匹配到本地订单记录，依次校验状态非空、币种一致、金额一致、状态一致，全部通过后标记为 matched。
func (s *StripeTradeBillReconcileService) applyChannelMatch(record *model.PaymentBillReconcile, row *model.PaymentBillRecord) {
	// 回填渠道账单基础信息到对账记录
	record.RecordSource = "local"                // 记录来源：以本地记录为主体
	record.BillRecordId = row.Id                 // 关联的渠道账单明细 ID
	record.BillDate = row.BillDate               // 账单日期
	record.TradeTime = row.TradeTime             // 渠道侧交易时间
	record.ChannelTradeNo = row.ChannelTradeNo   // 渠道交易号（pi_xxx）
	record.MerchantTradeNo = row.MerchantTradeNo // 商户订单号（ref_xxx / sub_ref_xxx）
	record.TradeType = row.TradeType             // 交易类型（payment / subscription）
	record.ChannelStatus = row.TradeStatus       // 渠道侧原始交易状态
	record.ChannelCurrency = row.Currency        // 渠道侧币种

	// 解析渠道账单金额（优先 OrderAmount，其次 TotalAmount）
	amountText, stripeAmount := parseStripeAmount(row)
	record.ChannelAmount = amountText // 渠道侧金额（字符串形式）

	// 校验1：渠道账单行无状态，无法对账，标记为不支持的账单行
	if strings.TrimSpace(row.TradeStatus) == "" {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonUnsupportedBillRow
		record.Remark = "stripe bill row has empty trade status"
		return
	}

	// 校验2：币种不一致，标记为币种不匹配（两边归一化为标准代码后再比较）
	channelCurrency := normalizeStripeCurrency(record.ChannelCurrency) // 渠道币种归一化（usd→USD）
	localCurrency := normalizeStripeCurrency(record.LocalCurrency)     // 本地币种归一化（$→USD）
	if channelCurrency != localCurrency {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonCurrencyMismatch
		record.Remark = fmt.Sprintf("channel_currency=%s local_currency=%s", record.ChannelCurrency, record.LocalCurrency)
		return
	}

	// 校验3：金额不一致（容差 0.000001），标记为金额不匹配
	if !amountsEqual(stripeAmount, record.LocalAmount) {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonAmountMismatch
		record.Remark = fmt.Sprintf("stripe_amount=%s local_amount=%.6f", amountText, record.LocalAmount)
		return
	}

	// 校验4：归一化后状态不一致，标记为状态不匹配
	stripeStatus := normalizeStripeTradeStatus(row.TradeStatus) // Stripe 状态归一化（complete→success）
	localStatus := normalizeLocalStatus(record.LocalStatus)     // 本地状态归一化（转小写）
	if stripeStatus != localStatus {
		record.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		record.ReconcileReason = model.PaymentReconcileReasonStatusMismatch
		record.Remark = fmt.Sprintf("stripe_status=%s local_status=%s", stripeStatus, localStatus)
		return
	}

	// 四项校验全部通过，标记为对账成功
	record.ReconcileStatus = model.PaymentReconcileStatusMatched
	record.ReconcileReason = model.PaymentReconcileReasonMatched
	record.Remark = "matched by trade number and amount"
}

// buildChannelOnlyRecord 构建仅存在于渠道侧而本地缺失的对账记录，标记为渠道单边异常。
func (s *StripeTradeBillReconcileService) buildChannelOnlyRecord(row *model.PaymentBillRecord) *model.PaymentBillReconcile {
	// 初始化对账记录，预填渠道侧信息，默认标记为本地无匹配
	record := &model.PaymentBillReconcile{
		ChannelType:        stripeChannelType,                                  // 固定为 stripe
		ReconcileKey:       reconcileKeyForChannelRecord(row.BillDate, row.Id), // 唯一键 "channel:{billDate}:{billRecordId}"
		RecordSource:       "channel",                                          // 来源为渠道侧
		BillRecordId:       row.Id,                                             // 关联渠道账单明细 ID
		BillDate:           row.BillDate,                                       // 账单日期
		TradeTime:          row.TradeTime,                                      // 交易时间（渠道侧）
		ChannelTradeNo:     row.ChannelTradeNo,                                 // 渠道交易号（pi_xxx）
		MerchantTradeNo:    row.MerchantTradeNo,                                // 商户订单号（ref_xxx / sub_ref_xxx）
		TradeType:          row.TradeType,                                      // 交易类型（payment / subscription）
		ChannelStatus:      row.TradeStatus,                                    // 渠道侧原始状态
		ChannelCurrency:    row.Currency,                                       // 渠道侧币种
		LocalPaymentMethod: stripeChannelType,                                  // 本地支付方式标记为 stripe
		ReconcileStatus:    model.PaymentReconcileStatusAbnormal,               // 默认异常：本地找不到对应订单
		ReconcileReason:    model.PaymentReconcileReasonLocalNotFound,          // 异常原因：本地无匹配记录
		Remark:             "stripe bill exists but local stripe order not found",
	}
	// 解析并回填渠道金额
	amountText, _ := parseStripeAmount(row)
	record.ChannelAmount = amountText
	// 如果渠道账单行无状态，细化异常原因为不支持的账单行
	if strings.TrimSpace(row.TradeStatus) == "" {
		record.ReconcileReason = model.PaymentReconcileReasonUnsupportedBillRow
		record.Remark = "stripe bill row has empty trade status"
	}
	return record
}

// ReconcileByBillDateRange 按天逐日对账。
// billDateFrom/billDateTo 均为 UTC 日期字符串（如 "2026-05-06"），表示对账的起止日期（含两端）。
// 使用 time.Parse（UTC 默认）解析日期，与 Stripe CreatedRange 的时间戳保持一致。
func (s *StripeTradeBillReconcileService) ReconcileByBillDateRange(billDateFrom string, billDateTo string) (*StripeTradeBillReconcileSummary, error) {
	// time.Parse 默认按 UTC 解析，与 Stripe CreatedRange 的时间戳语义一致，
	// 不使用 time.ParseInLocation 避免受服务器时区影响
	fromTime, err := time.Parse("2006-01-02", strings.TrimSpace(billDateFrom))
	if err != nil {
		return nil, err
	}
	toTime, err := time.Parse("2006-01-02", strings.TrimSpace(billDateTo))
	if err != nil {
		return nil, err
	}
	if toTime.Before(fromTime) {
		return nil, fmt.Errorf("bill date range is invalid")
	}

	// 逐日循环，每天独立对账并写入，避免跨天数据混淆
	summary := &StripeTradeBillReconcileSummary{}
	for current := fromTime; !current.After(toTime); current = current.AddDate(0, 0, 1) {
		billDate := current.Format("2006-01-02")

		// 用 UTC 时间戳查本地库，与拉取 Stripe 数据时的 CreatedRange 范围完全一致
		startTS := current.Unix()
		endTS := current.AddDate(0, 0, 1).Unix()

		// 加载本地成功的 Stripe 支付记录（topup + subscription）
		localRecords, err := s.loadLocalStripeSuccessRecords(billDate, startTS, endTS)
		if err != nil {
			return nil, err
		}

		// 从 payment_bill_record 表获取已入库的 Stripe 渠道账单明细
		rows, err := model.GetPaymentBillRecordsByChannelAndBillDateRange(stripeChannelType, billDate, billDate)
		if err != nil {
			return nil, err
		}

		// 按本地交易号建索引，用于与渠道账单做 O(1) 匹配
		localByTradeNo := indexLocalRecordsByTradeNo(localRecords)
		channelOnlyRecords := make([]*model.PaymentBillReconcile, 0)

		for _, row := range rows {
			if row == nil {
				continue
			}

			// 用渠道账单的 MerchantTradeNo（ref_xxx / sub_ref_xxx）在本地索引中查找匹配的本地记录
			candidates := make([]*model.PaymentBillReconcile, 0)
			if tradeNo := strings.TrimSpace(row.MerchantTradeNo); tradeNo != "" {
				candidates = append(candidates, localByTradeNo[tradeNo]...)
			}

			// 按 ReconcileKey 去重，同一笔本地记录可能被多条渠道账单匹配（两个号命中同一条）
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
				// 渠道有记录但本地没有 → 标记为渠道单边异常
				channelOnlyRecords = append(channelOnlyRecords, s.buildChannelOnlyRecord(row))
			case 1:
				// 一对一匹配，依次校验币种、金额、状态，全部通过标记 matched
				s.applyChannelMatch(uniqueCandidates[0], row)
			default:
				// 多条本地记录命中同一渠道账单 → 全部标记为重复数据异常
				for _, candidate := range uniqueCandidates {
					// 将渠道账单字段回填到每条本地记录上，便于定位问题
					candidate.BillRecordId = row.Id                 // 关联的渠道账单记录 ID
					candidate.TradeTime = row.TradeTime             // 渠道侧的交易时间
					candidate.ChannelTradeNo = row.ChannelTradeNo   // 渠道交易号（pi_xxx）
					candidate.MerchantTradeNo = row.MerchantTradeNo // 商户订单号（ref_xxx / sub_ref_xxx）
					candidate.TradeType = row.TradeType             // 交易类型（payment / subscription）
					candidate.ChannelStatus = row.TradeStatus       // 渠道侧交易状态
					candidate.ChannelCurrency = row.Currency        // 渠道侧币种
					amountText, _ := parseStripeAmount(row)
					candidate.ChannelAmount = amountText // 渠道侧金额
					candidate.ReconcileStatus = model.PaymentReconcileStatusAbnormal
					candidate.ReconcileReason = model.PaymentReconcileReasonDuplicateLocal
					candidate.Remark = fmt.Sprintf("matched %d local records by trade number", len(uniqueCandidates))
				}
			}
		}

		// 合并本地记录和渠道单边记录，upert 到对账结果表
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

// ReconcileStripeTradeBillsByBillDateRange 提供无状态的 Stripe 账单对账入口。
// billDateFrom/billDateTo 为 UTC 日期字符串（如 "2026-05-06"），含两端，按天逐日执行。
// 由 controller 的 runStripeBillWorkflow 在拉取 Stripe 数据后调用，使用相同的 UTC 时间戳保证范围一致。
func ReconcileStripeTradeBillsByBillDateRange(billDateFrom string, billDateTo string) (*StripeTradeBillReconcileSummary, error) {
	return NewStripeTradeBillReconcileService().ReconcileByBillDateRange(billDateFrom, billDateTo)
}
