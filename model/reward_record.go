package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// RewardRecord is the unified reward ledger for provider-scoped commercial rewards.
type RewardRecord struct {
	Id          int    `json:"id"`
	ProviderId  int    `json:"provider_id" gorm:"index;not null"`
	UserId      int    `json:"user_id" gorm:"index;not null"`
	SourceType  string `json:"source_type" gorm:"type:varchar(32);index:idx_reward_source,priority:1;not null"`
	SourceId    int    `json:"source_id" gorm:"index:idx_reward_source,priority:2;not null;default:0"`
	Quota       int    `json:"quota" gorm:"not null"`
	Description string `json:"description" gorm:"type:varchar(255);default:''"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;index"`
}

func (RewardRecord) TableName() string {
	return "reward_records"
}

func (r *RewardRecord) BeforeCreate(tx *gorm.DB) error {
	if r.ProviderId <= 0 || r.UserId <= 0 || r.SourceType == "" || r.Quota == 0 {
		return errors.New("invalid reward record")
	}
	if r.CreatedAt == 0 {
		r.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func CreateRewardRecordTx(tx *gorm.DB, record *RewardRecord) error {
	if tx == nil || record == nil {
		return nil
	}
	return tx.Create(record).Error
}

func CreateRewardRecord(record *RewardRecord) error {
	if record == nil {
		return nil
	}
	return DB.Create(record).Error
}

func SumRewardQuotaByUserAndSourceInProvider(providerId int, userId int, sourceType string) (int64, error) {
	var total int64
	if userId <= 0 || sourceType == "" {
		return 0, nil
	}
	query := DB.Model(&RewardRecord{}).
		Select("COALESCE(SUM(quota), 0)").
		Where("user_id = ? AND source_type = ?", userId, sourceType)
	if providerId > 0 {
		query = query.Where("provider_id = ?", providerId)
	}
	err := query.Scan(&total).Error
	return total, err
}

func GetUserRewardQuotaBySourceInProvider(providerId int, userId int, sourceType string) (float64, error) {
	total, err := SumRewardQuotaByUserAndSourceInProvider(providerId, userId, sourceType)
	if err != nil {
		return 0, err
	}
	return float64(total) / common.QuotaPerUnit, nil
}

func HasRewardRecordsBySourceInProvider(providerId int, userId int, sourceType string) (bool, error) {
	if providerId <= 0 || userId <= 0 || sourceType == "" {
		return false, nil
	}
	var count int64
	err := DB.Model(&RewardRecord{}).
		Where("provider_id = ? AND user_id = ? AND source_type = ?", providerId, userId, sourceType).
		Count(&count).Error
	return count > 0, err
}
