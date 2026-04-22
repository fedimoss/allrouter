package model

// TopUpRebate 充值返利记录数据模型
// 记录每次充值返利的详细信息，用于返利统计和审计
type TopUpRebate struct {
	Id            int     `json:"id"`                                          // 返利记录ID（主键）
	InviterId     int     `json:"inviter_id" gorm:"index"`                     // 邀请人ID（索引）
	InviteeId     int     `json:"invitee_id" gorm:"index"`                     // 被邀请人ID（索引）
	TopUpId       int     `json:"topup_id" gorm:"column:topup_id;uniqueIndex"` // 对应的充值记录ID（唯一索引）
	TradeNo       string  `json:"trade_no" gorm:"type:varchar(255);index"`     // 充值订单号（索引）
	PaymentMethod string  `json:"payment_method" gorm:"type:varchar(50)"`      // 支付方式
	SourceMoney   float64 `json:"source_money"`                                // 原始支付金额（美元）
	SourceQuota   int     `json:"source_quota"`                                // 原始充值额度
	RebateRatio   float64 `json:"rebate_ratio"`                                // 返利比例（百分比）
	RebateQuota   int     `json:"rebate_quota"`                                // 返利额度
	CreatedAt     int64   `json:"created_at" gorm:"bigint;index"`              // 创建时间（Unix时间戳）
}

// TableName 返回返利记录表的名称
func (TopUpRebate) TableName() string {
	return "topup_rebates"
}
