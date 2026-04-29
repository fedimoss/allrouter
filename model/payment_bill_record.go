package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PaymentChannelTypeWechat = "wxpay"
)

// PaymentBillRecord 通用支付渠道账单明细表。
// 不同支付渠道的原始账单都落到这张表，微信当前固定写 channel_type=wxpay。
type PaymentBillRecord struct {
	Id int `json:"id"`

	ChannelType string `json:"channel_type" gorm:"type:varchar(32);uniqueIndex:idx_payment_bill_record_channel_row_hash;index"`
	BillDate    string `json:"bill_date" gorm:"type:varchar(16);index"`
	FilePath    string `json:"file_path" gorm:"type:varchar(255)"`
	RowIndex    int    `json:"row_index" gorm:"index"`
	RowHash     string `json:"row_hash" gorm:"type:varchar(64);uniqueIndex:idx_payment_bill_record_channel_row_hash"`

	TradeTime       string `json:"trade_time" gorm:"type:varchar(64);index"`
	AppID           string `json:"app_id" gorm:"type:varchar(64)"`
	MchID           string `json:"mch_id" gorm:"type:varchar(64);index"`
	SubMchID        string `json:"sub_mch_id" gorm:"type:varchar(64)"`
	DeviceID        string `json:"device_id" gorm:"type:varchar(64)"`
	ChannelTradeNo  string `json:"channel_trade_no" gorm:"type:varchar(64);index"`
	MerchantTradeNo string `json:"merchant_trade_no" gorm:"type:varchar(64);index"`
	PayerID         string `json:"payer_id" gorm:"type:varchar(128)"`

	TradeType        string `json:"trade_type" gorm:"type:varchar(64)"`
	TradeStatus      string `json:"trade_status" gorm:"type:varchar(64);index"`
	RefundStatus     string `json:"refund_status" gorm:"type:varchar(64)"`
	RefundType       string `json:"refund_type" gorm:"type:varchar(64)"`
	Currency         string `json:"currency" gorm:"type:varchar(16)"`
	Bank             string `json:"bank" gorm:"type:varchar(128)"`
	TotalAmount      string `json:"total_amount" gorm:"type:varchar(64)"`
	OrderAmount      string `json:"order_amount" gorm:"type:varchar(64)"`
	RefundAmount     string `json:"refund_amount" gorm:"type:varchar(64)"`
	ServiceFee       string `json:"service_fee" gorm:"type:varchar(64)"`
	Rate             string `json:"rate" gorm:"type:varchar(64)"`
	RateRemark       string `json:"rate_remark" gorm:"type:text"`
	GoodsName        string `json:"goods_name" gorm:"type:text"`
	PackageData      string `json:"package_data" gorm:"type:text"`
	ChannelRefundNo  string `json:"channel_refund_no" gorm:"type:varchar(64);index"`
	MerchantRefundNo string `json:"merchant_refund_no" gorm:"type:varchar(64);index"`

	EnterpriseRedPacket string `json:"enterprise_red_packet" gorm:"type:varchar(64)"`
	EnterpriseRefund    string `json:"enterprise_refund" gorm:"type:varchar(64)"`
	ApplyRefundAmount   string `json:"apply_refund_amount" gorm:"type:varchar(64)"`

	RawLine     string `json:"raw_line" gorm:"type:text"`
	RawDataJSON string `json:"raw_data_json" gorm:"type:text"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
}

func (PaymentBillRecord) TableName() string {
	return "payment_bill_record"
}

func (r *PaymentBillRecord) BeforeCreate(tx *gorm.DB) error {
	r.ChannelType = strings.TrimSpace(r.ChannelType)
	r.BillDate = strings.TrimSpace(r.BillDate)
	r.FilePath = strings.TrimSpace(r.FilePath)
	r.RowHash = strings.TrimSpace(r.RowHash)
	if r.ChannelType == "" {
		return errors.New("channel type is empty")
	}
	if r.RowHash == "" {
		return errors.New("row hash is empty")
	}
	if r.CreatedAt == 0 {
		r.CreatedAt = common.GetTimestamp()
	}
	return nil
}

// BatchInsertPaymentBillRecords 批量写入账单明细，按 channel_type + row_hash 去重。
func BatchInsertPaymentBillRecords(rows []*PaymentBillRecord) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	batch := make([]PaymentBillRecord, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		row.ChannelType = strings.TrimSpace(row.ChannelType)
		row.BillDate = strings.TrimSpace(row.BillDate)
		row.FilePath = strings.TrimSpace(row.FilePath)
		row.RowHash = strings.TrimSpace(row.RowHash)
		if row.ChannelType == "" || row.RowHash == "" {
			continue
		}
		if row.CreatedAt == 0 {
			row.CreatedAt = common.GetTimestamp()
		}
		batch = append(batch, *row)
	}
	if len(batch) == 0 {
		return 0, nil
	}

	tx := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "channel_type"},
			{Name: "row_hash"},
		},
		DoNothing: true,
	}).CreateInBatches(batch, 200)
	return tx.RowsAffected, tx.Error
}

// GetPaymentBillRecordsByChannelAndBillDateRange 按渠道和账单日期范围查询账单明细。
func GetPaymentBillRecordsByChannelAndBillDateRange(channelType string, billDateFrom string, billDateTo string) ([]*PaymentBillRecord, error) {
	var rows []*PaymentBillRecord
	query := DB.Model(&PaymentBillRecord{}).Where("channel_type = ?", strings.TrimSpace(channelType))
	if strings.TrimSpace(billDateFrom) != "" {
		query = query.Where("bill_date >= ?", strings.TrimSpace(billDateFrom))
	}
	if strings.TrimSpace(billDateTo) != "" {
		query = query.Where("bill_date <= ?", strings.TrimSpace(billDateTo))
	}
	err := query.Order("bill_date asc").Order("row_index asc").Find(&rows).Error
	return rows, err
}
