package service

import (
	"fmt"
	"math"
	"strconv"
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
	CurrencySymbol      string  `json:"currency_symbol"` // 币种符号
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
	ChannelCurrency    string  `json:"channel_currency"` // 渠道币种
	LocalCurrency      string  `json:"local_currency"`   // 本地币种
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
	Currency            string  `json:"currency"`        // 币种
	CurrencySymbol      string  `json:"currency_symbol"` // 币种符号
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
	Currency          string  `json:"currency"`        // 币种
	CurrencySymbol    string  `json:"currency_symbol"` // 币种符号
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

// formatTimestampText 格式化时间戳为 CST 格式
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
	case model.PaymentReconcileReasonCurrencyMismatch:
		return "币种不一致"
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

func (s *WechatTradeBillQueryService) GetDashboard(filter *WechatTradeBillListFilter, userId int) (*WechatTradeBillDashboardResponse, error) {
	switch strings.TrimSpace(filter.PaymentMethod) {
	// stripe 分支
	case "stripe":
		return s.getStripeDashboard(filter, userId)
	// wxpay 分支
	default:
		return s.getWechatDashboard(filter)
	}
}

// getWechatDashboard 微信支付 dashboard（全部人民币，无需币种换算）。
func (s *WechatTradeBillQueryService) getWechatDashboard(filter *WechatTradeBillListFilter) (*WechatTradeBillDashboardResponse, error) {
	overview, err := model.GetPaymentBillReconcileOverview(model.PaymentChannelTypeWechat, normalizeWechatTradeBillListFilter(filter))
	if err != nil {
		return nil, err
	}
	return buildDashboardResponse(overview, "¥"), nil
}

// getStripeDashboard Stripe 支付 dashboard，按当前管理员的时区确定目标币种，统一换算后汇总。
func (s *WechatTradeBillQueryService) getStripeDashboard(filter *WechatTradeBillListFilter, userId int) (*WechatTradeBillDashboardResponse, error) {
	// 1. 获取当前管理员的时区（默认 America/New_York）
	timezone := "America/New_York" // 默认纽约时区（美元）
	if userId > 0 {                // 已登录的管理员
		var user model.User                                                                            // 临时 user 对象，只读 timezone 字段
		if err := model.DB.Select("timezone").Where("id = ?", userId).First(&user).Error; err == nil { // 查询 users 表
			if tz := strings.TrimSpace(user.Timezone); tz != "" { // 用户时区非空
				timezone = tz // 使用用户的时区
			}
		}
	}

	// 2. 根据时区确定目标币种（默认 USD）
	targetCurrency := model.GetCurrencyByTimezoneWithFallback(timezone, "USD") // 通过 timezone_currency_map 查询币种

	// 3. 从 options 表获取美元人民币汇率
	usdExchangeRate := loadUSDExchangeRate() // 1 USD = X CNY 的汇率值

	// 4. 查询 Stripe 对账记录（不分页，含币种字段）
	rows, err := model.GetAllPaymentBillReconciles(model.PaymentChannelTypeStripe, normalizeWechatTradeBillListFilter(filter))
	if err != nil {
		return nil, err
	}

	// 5. 逐条换算后汇总
	overview := &model.PaymentBillReconcileOverview{} // 初始化汇总对象
	for _, row := range rows {                        // 遍历每条对账记录
		overview.TotalCount++                                                                                           // 总记录数 +1
		convertedAmount := convertToTargetCurrency(row.LocalAmount, row.LocalCurrency, targetCurrency, usdExchangeRate) // 按币种换算到目标币种
		overview.TotalAmount += convertedAmount                                                                         // 累加换算后的金额
		if strings.EqualFold(strings.TrimSpace(row.LocalStatus), "success") {                                           // 本地支付成功
			overview.SuccessCount++ // 支付成功数 +1
		}
		switch strings.TrimSpace(row.ReconcileStatus) { // 对账状态分拣
		case model.PaymentReconcileStatusMatched: // 对账一致
			overview.MatchedCount++ // 一致数 +1
		case model.PaymentReconcileStatusAbnormal: // 对账异常
			overview.AbnormalCount++                   // 异常数 +1
			overview.AbnormalAmount += convertedAmount // 累加异常金额
		}
		if row.UpdatedAt > overview.LatestSyncAt { // 取最新的同步时间
			overview.LatestSyncAt = row.UpdatedAt
		}
	}
	return buildDashboardResponse(overview, currencySymbolForCode(targetCurrency)), nil // 构造响应并返回
}

// coalesceCurrency 优先取第一个非空币种，stripe 的 billRow.Currency 是代码（usd/cny），
// wxpay 的 Currency 可能为空，此时根据 payment_method 兜底（wxpay→CNY，其他→USD）。
func coalesceCurrency(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return normalizeStripeCurrency(s)
		}
	}
	return "USD"
}

// currencySymbolForCode 将币种代码转为显示符号。
func currencySymbolForCode(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "CNY":
		return "¥"
	case "USD":
		return "$"
	default:
		return code
	}
}

// loadUSDExchangeRate 从 options 表读取美元人民币汇率，默认返回 7.25。
func loadUSDExchangeRate() float64 {
	var option model.Option
	if err := model.DB.Where("key = ?", "USDExchangeRate").First(&option).Error; err == nil {
		rate, err := strconv.ParseFloat(strings.TrimSpace(option.Value), 64)
		if err == nil && rate > 0 {
			return rate
		}
	}
	return 7.25
}

// convertToTargetCurrency 将原始金额按币种换算为目标币种金额，保留 6 位小数。
// fromCurrency 可能是符号（$、¥）或代码（USD、cny），目标币种为标准 3 位大写代码。
func convertToTargetCurrency(amount float64, fromCurrency string, toCurrency string, usdExchangeRate float64) float64 {
	if amount == 0 {
		return 0
	}
	from := normalizeStripeCurrency(fromCurrency)
	to := normalizeStripeCurrency(toCurrency)
	if from == to {
		return amount
	}
	if usdExchangeRate <= 0 {
		return amount
	}
	// USD → CNY
	if from == "USD" && to == "CNY" {
		return math.Round(amount*usdExchangeRate*1e6) / 1e6
	}
	// CNY → USD
	if from == "CNY" && to == "USD" {
		return math.Round(amount/usdExchangeRate*1e6) / 1e6
	}
	return amount
}

// buildDashboardResponse 从 overview 构造 Dashboard 响应。
func buildDashboardResponse(overview *model.PaymentBillReconcileOverview, currencySymbol string) *WechatTradeBillDashboardResponse {
	resp := &WechatTradeBillDashboardResponse{
		LatestSyncAt:        overview.LatestSyncAt,
		LatestSyncAtText:    formatTimestampText(overview.LatestSyncAt),
		TotalOrderCount:     overview.TotalCount,
		PaymentSuccessCount: overview.SuccessCount,
		MatchedCount:        overview.MatchedCount,
		AbnormalCount:       overview.AbnormalCount,
		TotalAmount:         overview.TotalAmount,
		AbnormalAmount:      overview.AbnormalAmount,
		CurrencySymbol:      currencySymbol, // 币种符号
	}
	if overview.TotalCount > 0 {
		resp.MatchedRate = float64(overview.MatchedCount) / float64(overview.TotalCount)
		resp.AbnormalRate = float64(overview.AbnormalCount) / float64(overview.TotalCount)
	}
	return resp
}

func (s *WechatTradeBillQueryService) GetList(pageInfo *common.PageInfo, filter *WechatTradeBillListFilter) (*common.PageInfo, error) {
	// 定义变量，用于存储查询结果和总记录数
	rows := []*model.PaymentBillReconcile{}
	total := int64(0)
	var err error

	// 根据支付渠道查询账单
	switch filter.PaymentMethod {
	// Stripe 分支
	case "stripe":
		rows, total, err = model.GetPaymentBillReconciles(model.PaymentChannelTypeStripe, pageInfo, normalizeWechatTradeBillListFilter(filter))
	// wxpay 分支
	default:
		rows, total, err = model.GetPaymentBillReconciles(model.PaymentChannelTypeWechat, pageInfo, normalizeWechatTradeBillListFilter(filter))
	}

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
			ChannelCurrency:    row.ChannelCurrency, // 渠道币种
			LocalCurrency:      row.LocalCurrency,   // 本地币种
			AbnormalReason:     row.ReconcileReason,
			AbnormalReasonText: getWechatTradeBillReasonText(row.ReconcileReason),
			Remark:             row.Remark,
		}
		if strings.TrimSpace(item.PaymentMethod) == "" {
			item.PaymentMethod = "wxpay"
			item.PaymentMethodText = getWechatTradeBillPaymentMethodText(item.PaymentMethod)
		}
		// wxpay 老数据兜底：币种字段为空时默认为人民币
		if strings.TrimSpace(item.ChannelCurrency) == "" && item.PaymentMethod == "wxpay" {
			item.ChannelCurrency = "CNY"
			item.LocalCurrency = "¥"
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

	// 先仅通过 id 查询对账记录，从中获取实际的 channel_type，再用于后续查询
	var reconcile model.PaymentBillReconcile
	if err := model.DB.Where("id = ?", id).First(&reconcile).Error; err != nil {
		return nil, err
	}

	// 根据用户ID获取支付类型后，再查询账单记录
	var billRow model.PaymentBillRecord
	if reconcile.BillRecordId > 0 {
		if err := model.DB.Where("channel_type = ? AND id = ?", reconcile.ChannelType, reconcile.BillRecordId).First(&billRow).Error; err != nil {
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
		Currency:            coalesceCurrency(reconcile.LocalCurrency, reconcile.ChannelCurrency, reconcile.LocalPaymentMethod),
		CurrencySymbol:      currencySymbolForCode(coalesceCurrency(reconcile.LocalCurrency, reconcile.ChannelCurrency, reconcile.LocalPaymentMethod)),
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
		Currency:          coalesceCurrency(reconcile.ChannelCurrency, billRow.Currency, reconcile.LocalPaymentMethod),
		CurrencySymbol:    currencySymbolForCode(coalesceCurrency(reconcile.ChannelCurrency, billRow.Currency, reconcile.LocalPaymentMethod)),
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

func GetWechatTradeBillDashboard(filter *WechatTradeBillListFilter, userId int) (*WechatTradeBillDashboardResponse, error) {
	return NewWechatTradeBillQueryService().GetDashboard(filter, userId)
}

func GetWechatTradeBillList(pageInfo *common.PageInfo, filter *WechatTradeBillListFilter) (*common.PageInfo, error) {
	return NewWechatTradeBillQueryService().GetList(pageInfo, filter)
}

func GetWechatTradeBillDetail(id int) (*WechatTradeBillDetailResponse, error) {
	return NewWechatTradeBillQueryService().GetDetail(id)
}
