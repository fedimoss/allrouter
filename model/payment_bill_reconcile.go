package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PaymentReconcileStatusMatched  = "matched"
	PaymentReconcileStatusAbnormal = "abnormal"

	PaymentReconcileReasonMatched            = "matched"
	PaymentReconcileReasonChannelNotFound    = "channel_not_found"
	PaymentReconcileReasonLocalNotFound      = "local_not_found"
	PaymentReconcileReasonDuplicateLocal     = "duplicate_local"
	PaymentReconcileReasonAmountMismatch     = "amount_mismatch"
	PaymentReconcileReasonCurrencyMismatch   = "currency_mismatch"
	PaymentReconcileReasonStatusMismatch     = "status_mismatch"
	PaymentReconcileReasonUnsupportedBillRow = "unsupported_bill_row"
)

// PaymentBillReconcile 通用支付账单对账结果表。
type PaymentBillReconcile struct {
	Id int `json:"id"`

	ChannelType  string `json:"channel_type" gorm:"type:varchar(32);uniqueIndex:idx_payment_bill_reconcile_channel_key;index"`
	ReconcileKey string `json:"reconcile_key" gorm:"type:varchar(128);uniqueIndex:idx_payment_bill_reconcile_channel_key"`
	RecordSource string `json:"record_source" gorm:"type:varchar(32);index"`
	BillRecordId int    `json:"bill_record_id" gorm:"index"`
	BillDate     string `json:"bill_date" gorm:"type:varchar(16);index"`
	TradeTime    string `json:"trade_time" gorm:"type:varchar(64);index"`

	ChannelTradeNo      string `json:"channel_trade_no" gorm:"type:varchar(64);index"`
	MerchantTradeNo     string `json:"merchant_trade_no" gorm:"type:varchar(64);index"`
	TradeType           string `json:"trade_type" gorm:"type:varchar(64)"`
	ChannelStatus       string `json:"channel_status" gorm:"type:varchar(64);index"`
	ChannelRefundStatus string `json:"channel_refund_status" gorm:"type:varchar(64)"`
	ChannelAmount       string `json:"channel_amount" gorm:"type:varchar(64)"`
	ChannelRefundAmount string `json:"channel_refund_amount" gorm:"type:varchar(64)"`
	ChannelCurrency     string `json:"channel_currency" gorm:"type:varchar(16)"`

	LocalType          string  `json:"local_type" gorm:"type:varchar(32);index"`
	LocalId            int     `json:"local_id" gorm:"index"`
	LocalTradeNo       string  `json:"local_trade_no" gorm:"type:varchar(255);index"`
	LocalPaymentMethod string  `json:"local_payment_method" gorm:"type:varchar(50)"`
	LocalStatus        string  `json:"local_status" gorm:"type:varchar(64);index"`
	LocalAmount        float64 `json:"local_amount" gorm:"type:decimal(12,6);default:0"`
	LocalCreateTime    int64   `json:"local_create_time" gorm:"bigint;index"`
	LocalCompleteTime  int64   `json:"local_complete_time" gorm:"bigint;index"`
	LocalCurrency      string  `json:"local_currency" gorm:"type:varchar(16)"`

	ReconcileStatus string `json:"reconcile_status" gorm:"type:varchar(32);index"`
	ReconcileReason string `json:"reconcile_reason" gorm:"type:varchar(64);index"`
	Remark          string `json:"remark" gorm:"type:text"`

	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (PaymentBillReconcile) TableName() string {
	return "payment_bill_reconcile"
}

func (r *PaymentBillReconcile) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	if r.UpdatedAt == 0 {
		r.UpdatedAt = now
	}
	r.ChannelType = strings.TrimSpace(r.ChannelType)
	r.ReconcileKey = strings.TrimSpace(r.ReconcileKey)
	r.RecordSource = strings.TrimSpace(r.RecordSource)
	r.BillDate = strings.TrimSpace(r.BillDate)
	r.TradeTime = strings.TrimSpace(r.TradeTime)
	r.ChannelTradeNo = strings.TrimSpace(r.ChannelTradeNo)
	r.MerchantTradeNo = strings.TrimSpace(r.MerchantTradeNo)
	r.LocalTradeNo = strings.TrimSpace(r.LocalTradeNo)
	r.LocalType = strings.TrimSpace(r.LocalType)
	r.ReconcileStatus = strings.TrimSpace(r.ReconcileStatus)
	r.ReconcileReason = strings.TrimSpace(r.ReconcileReason)
	return nil
}

func (r *PaymentBillReconcile) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

// PaymentBillReconcileFilter 支付对账列表/报表共用筛选条件。
type PaymentBillReconcileFilter struct {
	BillDate        string
	ReconcileStatus string
	Keyword         string
	PaymentMethod   string
	LocalType       string
}

type PaymentBillReconcileStat struct {
	TotalCount    int64 `json:"total_count"`
	MatchedCount  int64 `json:"matched_count"`
	AbnormalCount int64 `json:"abnormal_count"`
}

type PaymentBillReconcileOverview struct {
	LatestSyncAt   int64   `json:"latest_sync_at"`
	TotalCount     int64   `json:"total_count"`
	SuccessCount   int64   `json:"success_count"`
	MatchedCount   int64   `json:"matched_count"`
	AbnormalCount  int64   `json:"abnormal_count"`
	TotalAmount    float64 `json:"total_amount"`
	AbnormalAmount float64 `json:"abnormal_amount"`
}

// UpsertPaymentBillReconciles 批量写入对账结果，按 channel_type + bill_record_id 幂等覆盖。
func UpsertPaymentBillReconciles(rows []*PaymentBillReconcile) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	batch := make([]PaymentBillReconcile, 0, len(rows))
	now := common.GetTimestamp()
	for _, row := range rows {
		if row == nil || strings.TrimSpace(row.ChannelType) == "" || strings.TrimSpace(row.ReconcileKey) == "" {
			continue
		}
		if row.CreatedAt == 0 {
			row.CreatedAt = now
		}
		row.UpdatedAt = now
		batch = append(batch, *row)
	}
	if len(batch) == 0 {
		return 0, nil
	}

	tx := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "channel_type"},
			{Name: "reconcile_key"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"record_source",
			"bill_record_id",
			"bill_date",
			"trade_time",
			"channel_trade_no",
			"merchant_trade_no",
			"trade_type",
			"channel_status",
			"channel_refund_status",
			"channel_amount",
			"channel_refund_amount",
			"local_type",
			"local_id",
			"local_trade_no",
			"local_payment_method",
			"local_status",
			"local_amount",
			"local_create_time",
			"local_complete_time",
			"reconcile_status",
			"reconcile_reason",
			"remark",
			"updated_at",
		}),
	}).CreateInBatches(batch, 200)
	return tx.RowsAffected, tx.Error
}

func buildPaymentBillReconcileQuery(channelType string, filter *PaymentBillReconcileFilter) *gorm.DB {
	query := DB.Model(&PaymentBillReconcile{}).Where("channel_type = ?", strings.TrimSpace(channelType))
	if filter == nil {
		return query
	}
	if strings.TrimSpace(filter.BillDate) != "" {
		query = query.Where("bill_date = ?", strings.TrimSpace(filter.BillDate))
	}
	if strings.TrimSpace(filter.ReconcileStatus) != "" {
		query = query.Where("reconcile_status = ?", strings.TrimSpace(filter.ReconcileStatus))
	}
	if strings.TrimSpace(filter.PaymentMethod) != "" {
		query = query.Where("local_payment_method = ?", strings.TrimSpace(filter.PaymentMethod))
	}
	if strings.TrimSpace(filter.LocalType) != "" {
		query = query.Where("local_type = ?", strings.TrimSpace(filter.LocalType))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		userID, err := strconv.Atoi(strings.TrimSpace(filter.Keyword))
		if err != nil || userID <= 0 {
			return query.Where("1 = 0")
		}
		query = query.
			Joins("LEFT JOIN top_ups AS pbr_topups ON pbr_topups.id = payment_bill_reconcile.local_id AND payment_bill_reconcile.local_type = ?", "topup").
			Joins("LEFT JOIN subscription_orders AS pbr_subscription_orders ON pbr_subscription_orders.id = payment_bill_reconcile.local_id AND payment_bill_reconcile.local_type = ?", "subscription").
			Joins("LEFT JOIN users AS pbr_topup_users ON pbr_topup_users.id = pbr_topups.user_id").
			Joins("LEFT JOIN users AS pbr_subscription_users ON pbr_subscription_users.id = pbr_subscription_orders.user_id")
		query = query.Where(
			"pbr_topup_users.id = ? OR pbr_subscription_users.id = ?",
			userID, userID,
		)
	}
	return query
}

func parsePaymentBillAmount(text string) float64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return value
}

// GetPaymentBillReconciles 查询某渠道的对账分页列表。
func GetPaymentBillReconciles(channelType string, pageInfo *common.PageInfo, filter *PaymentBillReconcileFilter) ([]*PaymentBillReconcile, int64, error) {
	var (
		rows  []*PaymentBillReconcile
		total int64
	)
	query := buildPaymentBillReconcileQuery(channelType, filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: 20}
	}
	err := query.Order("bill_date desc").Order("trade_time desc").Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&rows).Error
	return rows, total, err
}

// GetPaymentBillReconcileOverview 查询某渠道的对账报表汇总。
func GetPaymentBillReconcileOverview(channelType string, filter *PaymentBillReconcileFilter) (*PaymentBillReconcileOverview, error) {
	rows := make([]PaymentBillReconcile, 0)
	query := buildPaymentBillReconcileQuery(channelType, filter)
	if err := query.Select(
		"id",
		"channel_status",
		"channel_amount",
		"local_status",
		"local_amount",
		"reconcile_status",
		"updated_at",
	).Find(&rows).Error; err != nil {
		return nil, err
	}

	overview := &PaymentBillReconcileOverview{}
	for _, row := range rows {
		overview.TotalCount++
		amount := parsePaymentBillAmount(row.ChannelAmount)
		if amount <= 0 {
			amount = row.LocalAmount
		}
		overview.TotalAmount += amount
		if strings.EqualFold(strings.TrimSpace(row.ChannelStatus), "SUCCESS") || strings.EqualFold(strings.TrimSpace(row.LocalStatus), "success") {
			overview.SuccessCount++
		}
		switch strings.TrimSpace(row.ReconcileStatus) {
		case PaymentReconcileStatusMatched:
			overview.MatchedCount++
		case PaymentReconcileStatusAbnormal:
			overview.AbnormalCount++
			overview.AbnormalAmount += amount
		}
		if row.UpdatedAt > overview.LatestSyncAt {
			overview.LatestSyncAt = row.UpdatedAt
		}
	}
	return overview, nil
}

// GetAllPaymentBillReconciles 查询所有对账记录（不分页），用于 Stripe Dashboard 的币种换算聚合。
// 与 GetPaymentBillReconcileOverview 的区别是多查了 local_currency / channel_currency 字段。
func GetAllPaymentBillReconciles(channelType string, filter *PaymentBillReconcileFilter) ([]PaymentBillReconcile, error) {
	rows := make([]PaymentBillReconcile, 0)
	query := buildPaymentBillReconcileQuery(channelType, filter)
	if err := query.Select(
		"id",
		"local_status",
		"local_amount",
		"local_currency",
		"channel_currency",
		"reconcile_status",
		"updated_at",
	).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// DeletePaymentBillReconcilesByChannelAndBillDate 按渠道和账单日期删除对账结果。
func DeletePaymentBillReconcilesByChannelAndBillDate(channelType string, billDate string) error {
	return DB.Where("channel_type = ? AND bill_date = ?", channelType, billDate).Delete(&PaymentBillReconcile{}).Error
}

func (f *PaymentBillReconcileFilter) DebugString() string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf(
		"bill_date=%s reconcile_status=%s payment_method=%s local_type=%s keyword=%s",
		f.BillDate, f.ReconcileStatus, f.PaymentMethod, f.LocalType, f.Keyword,
	)
}
