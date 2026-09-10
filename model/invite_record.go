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

// InviteeAggregation 全部邀请用户的账户汇总(来自 users 表 JOIN invite_records)
type InviteeAggregation struct {
	InviteeCount int   `json:"invitee_count"`
	QuotaSum     int64 `json:"quota_sum"`
	UsedQuotaSum int64 `json:"used_quota_sum"`
	TokenUsedSum int64 `json:"token_used_sum"`
}

// GetInviteeAggregation 汇总当前用户邀请的所有用户的账户数据:
// 人数/余额合计/历史消耗合计/Token 消耗总量合计。
func GetInviteeAggregation(inviterId int) (*InviteeAggregation, error) {
	var result InviteeAggregation
	err := DB.Model(&InviteRecord{}).
		Select("COUNT(DISTINCT invite_records.invitee_id) as invitee_count, "+
			"COALESCE(SUM(users.quota), 0) as quota_sum, "+
			"COALESCE(SUM(users.used_quota), 0) as used_quota_sum, "+
			"COALESCE(SUM(users.total_token_used), 0) as token_used_sum").
		Joins("LEFT JOIN users ON users.id = invite_records.invitee_id").
		Where("invite_records.inviter_id = ?", inviterId).
		Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetInviteeQuotaDataByInviter 查询某邀请人名下全部被邀请人在时间范围内的聚合明细
// (quota_data 表,按时间聚合),用于"全部邀请用户"的看板卡片统计。
func GetInviteeQuotaDataByInviter(inviterId int, startTime int64, endTime int64) (quotaData []*QuotaData, err error) {
	err = DB.Table("quota_data").
		Select("quota_data.created_at, SUM(quota_data.count) as count, SUM(quota_data.quota) as quota, SUM(quota_data.token_used) as token_used").
		Joins("JOIN invite_records ON invite_records.invitee_id = quota_data.user_id").
		Where("invite_records.inviter_id = ? AND quota_data.created_at >= ? AND quota_data.created_at <= ?", inviterId, startTime, endTime).
		Group("quota_data.created_at").
		Find(&quotaData).Error
	return quotaData, err
}

// GetInviteeIdsByInviter 返回某邀请人名下全部被邀请人 ID 列表。
// 用于跨库统计:logs 表可能独立于主库(LOG_SQL_DSN),不能与 invite_records 直接 JOIN,
// 需先取 ID 列表再按 user_id IN 过滤。
func GetInviteeIdsByInviter(inviterId int) ([]int, error) {
	var ids []int
	err := DB.Model(&InviteRecord{}).
		Where("inviter_id = ?", inviterId).
		Distinct().
		Pluck("invitee_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
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
