package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// InviteRecord stores the fixed invitation registration reward at the time the invitee registers.
type InviteRecord struct {
	Id          int    `json:"id"`
	ProviderId  int    `json:"provider_id" gorm:"index;not null;default:0"`
	InviterId   int    `json:"inviter_id" gorm:"index"`
	InviteeId   int    `json:"invitee_id" gorm:"column:invitee_id;uniqueIndex"`
	InviteeName string `json:"invitee_name" gorm:"->;-:migration;column:invitee_name"`
	// 被邀请人余额/消耗, 来自 JOIN users 表的只读字段, 不落表
	InviteeQuota     int   `json:"invitee_quota" gorm:"->;-:migration;column:invitee_quota"`
	InviteeUsedQuota int   `json:"invitee_used_quota" gorm:"->;-:migration;column:invitee_used_quota"`
	RegisterTime     int64 `json:"register_time" gorm:"bigint;index"`
	RewardQuota      int   `json:"reward_quota"`
	CreatedAt        int64 `json:"created_at" gorm:"bigint;index"`
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
		Select("invite_records.*, COALESCE(users.username, '') AS invitee_name, COALESCE(users.quota, 0) AS invitee_quota, COALESCE(users.used_quota, 0) AS invitee_used_quota").
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
func GetSelfAffRecords(userId int, keyword string, pageInfo *common.PageInfo) (records []*InviteRecord, total int64, err error) {
	buildQuery := func() *gorm.DB {
		query := getInviteRecordBaseQuery().Where("invite_records.inviter_id = ?", userId)
		if keyword != "" {
			query = query.Where("users.username LIKE ?", "%"+keyword+"%")
		}
		return query
	}

	if err = buildQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err = buildQuery().
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
