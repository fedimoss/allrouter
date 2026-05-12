package model

import "github.com/QuantumNous/new-api/common"

// TopUpRebate 充值返利记录数据模型
// 记录每次充值返利的详细信息，用于返利统计和审计
type TopUpRebate struct {
	Id            int     `json:"id"`
	ProviderId    int     `json:"provider_id" gorm:"index;default:0"`
	InviterId     int     `json:"inviter_id" gorm:"index"`
	InviteeId     int     `json:"invitee_id" gorm:"index"`
	TopUpId       int     `json:"topup_id" gorm:"column:topup_id;uniqueIndex"`
	TradeNo       string  `json:"trade_no" gorm:"type:varchar(255);index"`
	PaymentMethod string  `json:"payment_method" gorm:"type:varchar(50)"`
	SourceMoney   float64 `json:"source_money"`
	SourceQuota   int     `json:"source_quota"`
	RebateRatio   float64 `json:"rebate_ratio"`
	RebateQuota   int     `json:"rebate_quota"`
	CreatedAt     int64   `json:"created_at" gorm:"bigint;index"`
	Money         float64 `json:"money"`
	Status        string  `json:"status"`
}

// TableName 返回返利记录表的名称
func (TopUpRebate) TableName() string {
	return "topup_rebates"
}

// GetTopUpRebateRecordsByInviteeId 获取用户充值返利记录
func GetTopUpRebateRecordsByInviteeId(userId int, pageInfo *common.PageInfo) (records []*TopUpRebate, total int64, err error) {
	query := DB.Model(&TopUpRebate{}).
		Select("rebate_quota, created_at").
		Order("created_at desc").
		Order("id desc").
		Where("invitee_id = ?", userId)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err = query.
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// SumTopUpRebateQuotaByInviteeId 获取用户充值返利额度总和
func SumTopUpRebateQuotaByInviteeId(userId int) (int64, error) {
	var totalQuota int64
	err := DB.Model(&TopUpRebate{}).
		Select("COALESCE(SUM(rebate_quota), 0)").
		Where("invitee_id = ?", userId).
		Scan(&totalQuota).Error
	return totalQuota, err
}
