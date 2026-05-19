package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ProviderWithdrawStatusPending  = 1 // 待审核
	ProviderWithdrawStatusApproved = 2 // 已通过
	ProviderWithdrawStatusRejected = 3 // 已拒绝
)

type ProviderWithdraw struct {
	Id           int     `json:"id"`
	ProviderId   int     `json:"provider_id" gorm:"index;not null"`
	ProviderName string  `json:"provider_name" gorm:"->"`
	Amount       float64 `json:"amount" gorm:"type:decimal(18,8);default:0"`
	Currency     string  `json:"currency" gorm:"type:varchar(20)"`
	UsdAmount    float64 `json:"usd_amount" gorm:"type:decimal(18,8);default:0"`
	CnyAmount    float64 `json:"cny_amount" gorm:"type:decimal(18,8);default:0"`
	UsdToCnyRate float64 `json:"usd_to_cny_rate" gorm:"type:decimal(18,8);default:0"`
	Status       int     `json:"status" gorm:"type:int;default:0;index"`
	CreatedAt    int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64   `json:"updated_at" gorm:"bigint"`
}

func (ProviderWithdraw) TableName() string {
	return "provider_withdraw"
}

// 创建前回调
func (p *ProviderWithdraw) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = now
	}
	return nil
}

// 更新前回调
func (p *ProviderWithdraw) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

// 创建提现申请
func CreateProviderWithdraw(record *ProviderWithdraw) error {
	if record == nil || record.ProviderId <= 0 {
		return errors.New("provider id is empty")
	}
	return DB.Create(record).Error
}

// 查询提现申请列表
func GetProviderWithdraws(providerId int, status int, startIdx int, num int) (records []*ProviderWithdraw, total int64, err error) {
	tx := DB.Model(&ProviderWithdraw{})

	// 服务商ID的值不为空时，添加ID查询
	if providerId > 0 {
		tx = tx.Where("provider_id = ?", providerId)
	}

	// 提现申请状态的值不为空时，添加状态查询
	if status > 0 {
		tx = tx.Where("status = ?", status)
	}

	// 统计总记录数
	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err = tx.Order("created_at desc").Limit(num).Offset(startIdx).Find(&records).Error
	return records, total, err
}

// 查询提现申请列表（支持按供应商ID和名称模糊搜索）
func SearchProviderWithdraws(providerId int, providerName string, status int, startIdx int, num int) (records []*ProviderWithdraw, total int64, err error) {
	tx := DB.Table("provider_withdraw").
		Select("provider_withdraw.*, providers.name as provider_name").
		Joins("JOIN providers ON providers.id = provider_withdraw.provider_id")

	// 提现申请状态的值不为空时，添加状态查询
	if providerId > 0 {
		tx = tx.Where("provider_withdraw.provider_id = ?", providerId)
	}

	// 供应商名称的值不为空时，添加名称查询
	if providerName != "" {
		tx = tx.Where("providers.name LIKE ?", "%"+providerName+"%")
	}

	// 提现申请状态的值不为空时，添加状态查询
	if status > 0 {
		tx = tx.Where("provider_withdraw.status = ?", status)
	}

	// 统计总记录数
	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err = tx.Order("provider_withdraw.created_at desc").Limit(num).Offset(startIdx).Find(&records).Error
	return records, total, err
}

// 更新提现申请状态
func UpdateProviderWithdrawStatus(id int, status int) error {
	if id <= 0 {
		return errors.New("id is empty")
	}
	return DB.Model(&ProviderWithdraw{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": common.GetTimestamp(),
	}).Error
}
