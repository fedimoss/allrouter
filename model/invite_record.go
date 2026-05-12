package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// InviteRecord stores the fixed invitation registration reward at the time the invitee registers.
type InviteRecord struct {
	Id           int    `json:"id"`
	ProviderId   int    `json:"provider_id" gorm:"index;not null"`
	InviterId    int    `json:"inviter_id" gorm:"index"`
	InviteeId    int    `json:"invitee_id" gorm:"column:invitee_id;uniqueIndex"`
	InviteeName  string `json:"invitee_name" gorm:"->;-:migration;column:invitee_name"`
	RegisterTime int64  `json:"register_time" gorm:"bigint;index"`
	RewardQuota  int    `json:"reward_quota"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index"`
}

func (InviteRecord) TableName() string {
	return "invite_records"
}

func createInviteRecordTx(tx *gorm.DB, inviterId int, invitee *User) error {
	if tx == nil || invitee == nil || inviterId <= 0 || invitee.Id <= 0 {
		return nil
	}
	registerTime := invitee.CreatedAt
	if registerTime <= 0 {
		registerTime = common.GetTimestamp()
	}
	record := &InviteRecord{
		ProviderId:   invitee.ProviderId,
		InviterId:    inviterId,
		InviteeId:    invitee.Id,
		RegisterTime: registerTime,
		RewardQuota:  0,
		CreatedAt:    common.GetTimestamp(),
	}
	if inviterId > 0 {
		if inviter, err := GetUserById(inviterId, true); err == nil {
			if cfg, err := GetProviderRewardConfig(inviter.ProviderId); err == nil {
				record.RewardQuota = cfg.QuotaForInviter
			}
		}
	}
	return tx.Create(record).Error
}

// getInviteRecordBaseQuery 获取邀请记录基础查询
func getInviteRecordBaseQuery() *gorm.DB {
	return DB.Model(&InviteRecord{}).
		Select("invite_records.*, COALESCE(users.username, '') AS invitee_name").
		Joins("LEFT JOIN users ON users.id = invite_records.invitee_id")
}

// GetUserAffRecords 获取用户邀请记录
func GetUserAffRecords(pageInfo *common.PageInfo) (records []*InviteRecord, total int64, err error) {
	query := DB.Model(&InviteRecord{})

	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err = getInviteRecordBaseQuery().
		Order("register_time desc").
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error
	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// GetSelfAffRecords 获取用户自己的邀请记录
func GetSelfAffRecords(userId int, pageInfo *common.PageInfo) (records []*InviteRecord, total int64, err error) {
	query := DB.Model(&InviteRecord{}).Where("inviter_id = ?", userId)

	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err = getInviteRecordBaseQuery().
		Where("invite_records.inviter_id = ?", userId).
		Order("register_time desc").
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error
	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}
