package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type WechatTradeBillListFilter struct {
	BillDate        string
	ReconcileStatus string
	Keyword         string
	PaymentMethod   string
	LocalType       string
}

// WechatTradeBillDashboardResponse 对应支付对账页顶部统计卡片。
type WechatTradeBillDashboardResponse struct {
	LatestSyncAt        int64   `json:"latest_sync_at"`
	LatestSyncAtText    string  `json:"latest_sync_at_text"`
	TotalOrderCount     int64   `json:"total_order_count"`
	PaymentSuccessCount int64   `json:"payment_success_count"`
	MatchedCount        int64   `json:"matched_count"`
	AbnormalCount       int64   `json:"abnormal_count"`
	TotalAmount         float64 `json:"total_amount"`
	AbnormalAmount      float64 `json:"abnormal_amount"`
	MatchedRate         float64 `json:"matched_rate"`
	AbnormalRate        float64 `json:"abnormal_rate"`
}

// WechatTradeBillListItem 对应对账结果列表中的单行数据。
type WechatTradeBillListItem struct {
	Id                 int     `json:"id"`
	Status             string  `json:"status"`
	StatusText         string  `json:"status_text"`
	UserId             int     `json:"user_id"`
	Username           string  `json:"username"`
	DisplayName        string  `json:"display_name"`
	MerchantTradeNo    string  `json:"merchant_trade_no"`
	WechatTradeNo      string  `json:"wechat_trade_no"`
	LocalTradeNo       string  `json:"local_trade_no"`
	LocalId            int     `json:"local_id"`
	Amount             float64 `json:"amount"`
	AmountText         string  `json:"amount_text"`
	PaymentMethod      string  `json:"payment_method"`
	PaymentMethodText  string  `json:"payment_method_text"`
	TradeTime          string  `json:"trade_time"`
	LocalType          string  `json:"local_type"`
	LocalTypeText      string  `json:"local_type_text"`
	WechatTradeStatus  string  `json:"wechat_trade_status"`
	LocalStatus        string  `json:"local_status"`
	AbnormalReason     string  `json:"abnormal_reason"`
	AbnormalReasonText string  `json:"abnormal_reason_text"`
	Remark             string  `json:"remark"`
}

type WechatTradeBillDetailParty struct {
	Title string `json:"title"`
	Tag   string `json:"tag"`
}

type WechatTradeBillLocalRecord struct {
	LocalType           string  `json:"local_type"`
	LocalTypeText       string  `json:"local_type_text"`
	LocalId             int     `json:"local_id"`
	SystemTradeNo       string  `json:"system_trade_no"`
	Status              string  `json:"status"`
	StatusText          string  `json:"status_text"`
	UserId              int     `json:"user_id"`
	Username            string  `json:"username"`
	DisplayName         string  `json:"display_name"`
	RequestedAmount     float64 `json:"requested_amount"`
	RequestedAmountText string  `json:"requested_amount_text"`
	CreateTime          int64   `json:"create_time"`
	CreateTimeText      string  `json:"create_time_text"`
	CompleteTime        int64   `json:"complete_time"`
	CompleteTimeText    string  `json:"complete_time_text"`
	Remark              string  `json:"remark"`
}

type WechatTradeBillChannelRecord struct {
	PaymentMethod     string  `json:"payment_method"`
	PaymentMethodText string  `json:"payment_method_text"`
	WechatTradeNo     string  `json:"wechat_trade_no"`
	TradeStatus       string  `json:"trade_status"`
	TradeStatusText   string  `json:"trade_status_text"`
	UserIdentifier    string  `json:"user_identifier"`
	ActualAmount      float64 `json:"actual_amount"`
	ActualAmountText  string  `json:"actual_amount_text"`
	TradeTime         string  `json:"trade_time"`
	TradeCompleteTime string  `json:"trade_complete_time"`
	Remark            string  `json:"remark"`
	GoodsName         string  `json:"goods_name"`
	Bank              string  `json:"bank"`
	Currency          string  `json:"currency"`
	AppID             string  `json:"app_id"`
	MchID             string  `json:"mch_id"`
}

type WechatTradeBillDetailResponse struct {
	Id                  int                           `json:"id"`
	BillRowId           int                           `json:"bill_row_id"`
	Header              *WechatTradeBillDetailParty   `json:"header"`
	LocalRecord         *WechatTradeBillLocalRecord   `json:"local_record"`
	ChannelRecord       *WechatTradeBillChannelRecord `json:"channel_record"`
	ReconcileStatus     string                        `json:"reconcile_status"`
	ReconcileStatusText string                        `json:"reconcile_status_text"`
	AbnormalReason      string                        `json:"abnormal_reason"`
	AbnormalReasonText  string                        `json:"abnormal_reason_text"`
	Remark              string                        `json:"remark"`
	RawBill             map[string]string             `json:"raw_bill,omitempty"`
}

type WechatTradeBillQueryService struct{}

func NewWechatTradeBillQueryService() *WechatTradeBillQueryService {
	return &WechatTradeBillQueryService{}
}

func normalizeWechatTradeBillListFilter(filter *WechatTradeBillListFilter) *model.PaymentBillReconcileFilter {
	if filter == nil {
		return &model.PaymentBillReconcileFilter{}
	}
	return &model.PaymentBillReconcileFilter{
		BillDate:        strings.TrimSpace(filter.BillDate),
		ReconcileStatus: strings.TrimSpace(filter.ReconcileStatus),
		Keyword:         strings.TrimSpace(filter.Keyword),
		PaymentMethod:   strings.TrimSpace(filter.PaymentMethod),
		LocalType:       strings.TrimSpace(filter.LocalType),
	}
}

func formatTimestampText(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func formatMoneyText(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

func getLocalTradeStatusText(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "success":
		return "支付成功"
	case "pending":
		return "待支付"
	case "failed":
		return "支付失败"
	case "expired":
		return "已过期"
	case "refund":
		return "已退款"
	default:
		return strings.TrimSpace(status)
	}
}

func getWechatTradeStatusText(status string) string {
	switch strings.TrimSpace(strings.ToUpper(status)) {
	case "SUCCESS":
		return "支付成功"
	case "REFUND":
		return "已退款"
	case "NOTPAY":
		return "未支付"
	case "USERPAYING":
		return "支付中"
	case "CLOSED":
		return "已关闭"
	case "REVOKED":
		return "已撤销"
	case "PAYERROR":
		return "支付失败"
	default:
		return strings.TrimSpace(status)
	}
}

func getWechatTradeBillStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case model.PaymentReconcileStatusMatched:
		return "一致"
	case model.PaymentReconcileStatusAbnormal:
		return "异常"
	default:
		return strings.TrimSpace(status)
	}
}

func getWechatTradeBillReasonText(reason string) string {
	switch strings.TrimSpace(reason) {
	case model.PaymentReconcileReasonMatched:
		return "对账一致"
	case model.PaymentReconcileReasonChannelNotFound:
		return "本地成功单未在微信账单中找到"
	case model.PaymentReconcileReasonLocalNotFound:
		return "本地订单不存在"
	case model.PaymentReconcileReasonDuplicateLocal:
		return "匹配到多条本地订单"
	case model.PaymentReconcileReasonAmountMismatch:
		return "金额不一致"
	case model.PaymentReconcileReasonStatusMismatch:
		return "状态不一致"
	case model.PaymentReconcileReasonUnsupportedBillRow:
		return "账单行不支持对账"
	default:
		return strings.TrimSpace(reason)
	}
}

func getWechatTradeBillLocalTypeText(localType string) string {
	switch strings.TrimSpace(localType) {
	case "topup":
		return "充值订单"
	case "subscription":
		return "订阅订单"
	default:
		return strings.TrimSpace(localType)
	}
}

func getWechatTradeBillPaymentMethodText(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "wxpay", "wechat", "wechatpay":
		return "微信支付"
	case "stripe":
		return "Stripe"
	case "epay":
		return "易支付"
	case "creem":
		return "Creem"
	case "waffo":
		return "Waffo"
	default:
		return strings.TrimSpace(method)
	}
}

func collectWechatTradeBillLocalIDs(rows []*model.PaymentBillReconcile) ([]int, []int) {
	topupIDs := make([]int, 0)
	subscriptionIDs := make([]int, 0)
	topupSet := make(map[int]struct{})
	subscriptionSet := make(map[int]struct{})
	for _, row := range rows {
		if row == nil || row.LocalId <= 0 {
			continue
		}
		switch strings.TrimSpace(row.LocalType) {
		case "topup":
			if _, ok := topupSet[row.LocalId]; !ok {
				topupSet[row.LocalId] = struct{}{}
				topupIDs = append(topupIDs, row.LocalId)
			}
		case "subscription":
			if _, ok := subscriptionSet[row.LocalId]; !ok {
				subscriptionSet[row.LocalId] = struct{}{}
				subscriptionIDs = append(subscriptionIDs, row.LocalId)
			}
		}
	}
	return topupIDs, subscriptionIDs
}

func loadWechatTradeBillUserMap(rows []*model.PaymentBillReconcile) (map[int]*model.User, map[int]*model.TopUp, map[int]*model.SubscriptionOrder, error) {
	topupIDs, subscriptionIDs := collectWechatTradeBillLocalIDs(rows)
	topupMap := make(map[int]*model.TopUp)
	subscriptionMap := make(map[int]*model.SubscriptionOrder)
	userIDs := make([]int, 0)
	userSet := make(map[int]struct{})

	if len(topupIDs) > 0 {
		var topups []model.TopUp
		if err := model.DB.Where("id IN ?", topupIDs).Find(&topups).Error; err != nil {
			return nil, nil, nil, err
		}
		for i := range topups {
			topup := topups[i]
			topupMap[topup.Id] = &topup
			if _, ok := userSet[topup.UserId]; !ok && topup.UserId > 0 {
				userSet[topup.UserId] = struct{}{}
				userIDs = append(userIDs, topup.UserId)
			}
		}
	}

	if len(subscriptionIDs) > 0 {
		var orders []model.SubscriptionOrder
		if err := model.DB.Where("id IN ?", subscriptionIDs).Find(&orders).Error; err != nil {
			return nil, nil, nil, err
		}
		for i := range orders {
			order := orders[i]
			subscriptionMap[order.Id] = &order
			if _, ok := userSet[order.UserId]; !ok && order.UserId > 0 {
				userSet[order.UserId] = struct{}{}
				userIDs = append(userIDs, order.UserId)
			}
		}
	}

	userMap := make(map[int]*model.User)
	if len(userIDs) > 0 {
		var users []model.User
		if err := model.DB.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, nil, nil, err
		}
		for i := range users {
			user := users[i]
			userMap[user.Id] = &user
		}
	}

	return userMap, topupMap, subscriptionMap, nil
}

func (s *WechatTradeBillQueryService) GetDashboard(filter *WechatTradeBillListFilter) (*WechatTradeBillDashboardResponse, error) {
	overview, err := model.GetPaymentBillReconcileOverview(model.PaymentChannelTypeWechat, normalizeWechatTradeBillListFilter(filter))
	if err != nil {
		return nil, err
	}

	resp := &WechatTradeBillDashboardResponse{
		LatestSyncAt:        overview.LatestSyncAt,
		LatestSyncAtText:    formatTimestampText(overview.LatestSyncAt),
		TotalOrderCount:     overview.TotalCount,
		PaymentSuccessCount: overview.SuccessCount,
		MatchedCount:        overview.MatchedCount,
		AbnormalCount:       overview.AbnormalCount,
		TotalAmount:         overview.TotalAmount,
		AbnormalAmount:      overview.AbnormalAmount,
	}
	if overview.TotalCount > 0 {
		resp.MatchedRate = float64(overview.MatchedCount) / float64(overview.TotalCount)
		resp.AbnormalRate = float64(overview.AbnormalCount) / float64(overview.TotalCount)
	}
	return resp, nil
}

func (s *WechatTradeBillQueryService) GetList(pageInfo *common.PageInfo, filter *WechatTradeBillListFilter) (*common.PageInfo, error) {
	rows, total, err := model.GetPaymentBillReconciles(model.PaymentChannelTypeWechat, pageInfo, normalizeWechatTradeBillListFilter(filter))
	if err != nil {
		return nil, err
	}
	userMap, topupMap, subscriptionMap, err := loadWechatTradeBillUserMap(rows)
	if err != nil {
		return nil, err
	}

	items := make([]*WechatTradeBillListItem, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		item := &WechatTradeBillListItem{
			Id:                 row.Id,
			Status:             row.ReconcileStatus,
			StatusText:         getWechatTradeBillStatusText(row.ReconcileStatus),
			MerchantTradeNo:    row.MerchantTradeNo,
			WechatTradeNo:      row.ChannelTradeNo,
			LocalTradeNo:       row.LocalTradeNo,
			LocalId:            row.LocalId,
			Amount:             row.LocalAmount,
			AmountText:         formatMoneyText(row.LocalAmount),
			PaymentMethod:      row.LocalPaymentMethod,
			PaymentMethodText:  getWechatTradeBillPaymentMethodText(row.LocalPaymentMethod),
			TradeTime:          row.TradeTime,
			LocalType:          row.LocalType,
			LocalTypeText:      getWechatTradeBillLocalTypeText(row.LocalType),
			WechatTradeStatus:  row.ChannelStatus,
			LocalStatus:        row.LocalStatus,
			AbnormalReason:     row.ReconcileReason,
			AbnormalReasonText: getWechatTradeBillReasonText(row.ReconcileReason),
			Remark:             row.Remark,
		}
		if strings.TrimSpace(item.PaymentMethod) == "" {
			item.PaymentMethod = "wxpay"
			item.PaymentMethodText = getWechatTradeBillPaymentMethodText(item.PaymentMethod)
		}
		if item.Amount <= 0 {
			item.Amount = modelAmountOrFallback(row)
			item.AmountText = formatMoneyText(item.Amount)
		}

		switch strings.TrimSpace(row.LocalType) {
		case "topup":
			if topup, ok := topupMap[row.LocalId]; ok && topup != nil {
				item.UserId = topup.UserId
			}
		case "subscription":
			if order, ok := subscriptionMap[row.LocalId]; ok && order != nil {
				item.UserId = order.UserId
			}
		}
		if user, ok := userMap[item.UserId]; ok && user != nil {
			item.Username = user.Username
			item.DisplayName = user.DisplayName
		}

		items = append(items, item)
	}

	result := pageInfo
	if result == nil {
		result = &common.PageInfo{Page: 1, PageSize: 20}
	}
	result.SetTotal(int(total))
	result.SetItems(items)
	return result, nil
}

func modelAmountOrFallback(row *model.PaymentBillReconcile) float64 {
	if row == nil {
		return 0
	}
	if row.LocalAmount > 0 {
		return row.LocalAmount
	}
	return parseAmountText(row.ChannelAmount)
}

func parseAmountText(text string) float64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	var amount float64
	fmt.Sscanf(text, "%f", &amount)
	return amount
}

func parseRawBillFields(text string) map[string]string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	fields := make(map[string]string)
	if err := common.Unmarshal([]byte(text), &fields); err != nil {
		return nil
	}
	return fields
}

func getRawBillField(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fields[key]); value != "" {
			return value
		}
	}
	return ""
}

func (s *WechatTradeBillQueryService) GetDetail(id int) (*WechatTradeBillDetailResponse, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid reconcile id")
	}

	var reconcile model.PaymentBillReconcile
	if err := model.DB.Where("channel_type = ? AND id = ?", model.PaymentChannelTypeWechat, id).First(&reconcile).Error; err != nil {
		return nil, err
	}

	var billRow model.PaymentBillRecord
	if reconcile.BillRecordId > 0 {
		if err := model.DB.Where("channel_type = ? AND id = ?", model.PaymentChannelTypeWechat, reconcile.BillRecordId).First(&billRow).Error; err != nil {
			return nil, err
		}
	}

	userMap, topupMap, subscriptionMap, err := loadWechatTradeBillUserMap([]*model.PaymentBillReconcile{&reconcile})
	if err != nil {
		return nil, err
	}

	resp := &WechatTradeBillDetailResponse{
		Id:                  reconcile.Id,
		BillRowId:           reconcile.BillRecordId,
		ReconcileStatus:     reconcile.ReconcileStatus,
		ReconcileStatusText: getWechatTradeBillStatusText(reconcile.ReconcileStatus),
		AbnormalReason:      reconcile.ReconcileReason,
		AbnormalReasonText:  getWechatTradeBillReasonText(reconcile.ReconcileReason),
		Remark:              reconcile.Remark,
		RawBill:             parseRawBillFields(billRow.RawDataJSON),
	}

	localRecord := &WechatTradeBillLocalRecord{
		LocalType:           reconcile.LocalType,
		LocalTypeText:       getWechatTradeBillLocalTypeText(reconcile.LocalType),
		LocalId:             reconcile.LocalId,
		SystemTradeNo:       reconcile.LocalTradeNo,
		Status:              reconcile.LocalStatus,
		StatusText:          getLocalTradeStatusText(reconcile.LocalStatus),
		RequestedAmount:     reconcile.LocalAmount,
		RequestedAmountText: formatMoneyText(reconcile.LocalAmount),
		CreateTime:          reconcile.LocalCreateTime,
		CreateTimeText:      formatTimestampText(reconcile.LocalCreateTime),
		CompleteTime:        reconcile.LocalCompleteTime,
		CompleteTimeText:    formatTimestampText(reconcile.LocalCompleteTime),
	}

	switch strings.TrimSpace(reconcile.LocalType) {
	case "topup":
		if topup, ok := topupMap[reconcile.LocalId]; ok && topup != nil {
			localRecord.UserId = topup.UserId
			localRecord.SystemTradeNo = topup.TradeNo
			localRecord.Status = topup.Status
			localRecord.StatusText = getLocalTradeStatusText(topup.Status)
			localRecord.RequestedAmount = topup.OriginalMoney
			localRecord.RequestedAmountText = formatMoneyText(topup.OriginalMoney)
			localRecord.CreateTime = topup.CreateTime
			localRecord.CreateTimeText = formatTimestampText(topup.CreateTime)
			localRecord.CompleteTime = topup.CompleteTime
			localRecord.CompleteTimeText = formatTimestampText(topup.CompleteTime)
		}
	case "subscription":
		if order, ok := subscriptionMap[reconcile.LocalId]; ok && order != nil {
			localRecord.UserId = order.UserId
			localRecord.SystemTradeNo = order.TradeNo
			localRecord.Status = order.Status
			localRecord.StatusText = getLocalTradeStatusText(order.Status)
			localRecord.RequestedAmount = order.OriginalMoney
			localRecord.RequestedAmountText = formatMoneyText(order.OriginalMoney)
			localRecord.CreateTime = order.CreateTime
			localRecord.CreateTimeText = formatTimestampText(order.CreateTime)
			localRecord.CompleteTime = order.CompleteTime
			localRecord.CompleteTimeText = formatTimestampText(order.CompleteTime)
		}
	}
	if user, ok := userMap[localRecord.UserId]; ok && user != nil {
		localRecord.Username = user.Username
		localRecord.DisplayName = user.DisplayName
	}

	headerTitle := getWechatTradeBillPaymentMethodText(reconcile.LocalPaymentMethod)
	if headerTitle == "" {
		headerTitle = "支付详情"
	}
	headerTag := ""
	if localRecord.UserId > 0 {
		headerTag = fmt.Sprintf("%d", localRecord.UserId)
	}
	resp.Header = &WechatTradeBillDetailParty{
		Title: headerTitle,
		Tag:   headerTag,
	}
	resp.LocalRecord = localRecord

	actualAmount := parseAmountText(billRow.OrderAmount)
	if actualAmount <= 0 {
		actualAmount = parseAmountText(billRow.TotalAmount)
	}
	if actualAmount <= 0 {
		actualAmount = reconcile.LocalAmount
	}
	channelRemark := strings.TrimSpace(billRow.PackageData)
	if channelRemark == "" {
		channelRemark = strings.TrimSpace(billRow.RateRemark)
	}
	if channelRemark == "" {
		channelRemark = strings.TrimSpace(reconcile.Remark)
	}

	resp.ChannelRecord = &WechatTradeBillChannelRecord{
		PaymentMethod:     reconcile.LocalPaymentMethod,
		PaymentMethodText: getWechatTradeBillPaymentMethodText(reconcile.LocalPaymentMethod),
		WechatTradeNo:     billRow.ChannelTradeNo,
		TradeStatus:       billRow.TradeStatus,
		TradeStatusText:   getWechatTradeStatusText(billRow.TradeStatus),
		UserIdentifier:    billRow.PayerID,
		ActualAmount:      actualAmount,
		ActualAmountText:  formatMoneyText(actualAmount),
		TradeTime:         billRow.TradeTime,
		TradeCompleteTime: billRow.TradeTime,
		Remark:            channelRemark,
		GoodsName:         billRow.GoodsName,
		Bank:              billRow.Bank,
		Currency:          billRow.Currency,
		AppID:             billRow.AppID,
		MchID:             billRow.MchID,
	}
	if strings.TrimSpace(resp.ChannelRecord.PaymentMethod) == "" {
		resp.ChannelRecord.PaymentMethod = "wxpay"
		resp.ChannelRecord.PaymentMethodText = getWechatTradeBillPaymentMethodText(resp.ChannelRecord.PaymentMethod)
	}
	if resp.ChannelRecord.WechatTradeNo == "" {
		resp.ChannelRecord.WechatTradeNo = reconcile.ChannelTradeNo
	}
	if strings.TrimSpace(resp.ChannelRecord.TradeStatus) == "" {
		resp.ChannelRecord.TradeStatus = reconcile.ChannelStatus
		resp.ChannelRecord.TradeStatusText = getWechatTradeStatusText(reconcile.ChannelStatus)
	}
	if resp.ChannelRecord.UserIdentifier == "" {
		resp.ChannelRecord.UserIdentifier = getRawBillField(resp.RawBill, "用户标识")
	}
	return resp, nil
}

func GetWechatTradeBillDashboard(filter *WechatTradeBillListFilter) (*WechatTradeBillDashboardResponse, error) {
	return NewWechatTradeBillQueryService().GetDashboard(filter)
}

func GetWechatTradeBillList(pageInfo *common.PageInfo, filter *WechatTradeBillListFilter) (*common.PageInfo, error) {
	return NewWechatTradeBillQueryService().GetList(pageInfo, filter)
}

func GetWechatTradeBillDetail(id int) (*WechatTradeBillDetailResponse, error) {
	return NewWechatTradeBillQueryService().GetDetail(id)
}
