package model

import (
	"errors"
	"math/rand"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Checkin 签到记录
type Checkin struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ProviderId   int    `json:"provider_id" gorm:"not null;default:0;index;uniqueIndex:idx_provider_user_checkin_date,priority:1"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_provider_user_checkin_date,priority:2"`
	CheckinDate  string `json:"checkin_date" gorm:"type:varchar(10);not null;uniqueIndex:idx_provider_user_checkin_date,priority:3"` // 格式: YYYY-MM-DD
	QuotaAwarded int    `json:"quota_awarded" gorm:"not null"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
}

// CheckinRecord 用于API返回的签到记录（不包含敏感字段）
type CheckinRecord struct {
	CheckinDate  string `json:"checkin_date"`
	QuotaAwarded int    `json:"quota_awarded"`
}

func (Checkin) TableName() string {
	return "checkins"
}

// GetUserCheckinRecords 获取用户在指定日期范围内的签到记录
func GetUserCheckinRecords(userId int, startDate, endDate string) ([]Checkin, error) {
	return GetUserCheckinRecordsInProvider(0, userId, startDate, endDate)
}

func GetUserCheckinRecordsInProvider(providerId int, userId int, startDate, endDate string) ([]Checkin, error) {
	var records []Checkin
	query := DB.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?", userId, startDate, endDate)
	if providerId > 0 {
		query = query.Where("provider_id = ?", providerId)
	}
	err := query.Order("checkin_date DESC").Find(&records).Error
	return records, err
}

// HasCheckedInToday 检查用户今天是否已签到
func HasCheckedInToday(userId int) (bool, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, today).
		Count(&count).Error
	return count > 0, err
}

func HasCheckedInTodayInProvider(providerId int, userId int) (bool, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	err := DB.Model(&Checkin{}).
		Where("provider_id = ? AND user_id = ? AND checkin_date = ?", providerId, userId, today).
		Count(&count).Error
	return count > 0, err
}

// UserCheckin 执行用户签到
// MySQL 和 PostgreSQL 使用事务保证原子性
// SQLite 不支持嵌套事务，使用顺序操作 + 手动回滚
func UserCheckin(userId int) (*Checkin, error) {
	var user User
	if err := DB.Select("id", "provider_id").Where("id = ?", userId).Take(&user).Error; err != nil {
		return nil, err
	}
	setting, err := GetProviderRewardConfig(user.ProviderId)
	if err != nil {
		return nil, err
	}
	if !setting.CheckinEnabled {
		return nil, errors.New("check-in is disabled")
	}

	hasChecked, err := HasCheckedInTodayInProvider(user.ProviderId, userId)
	if err != nil {
		return nil, err
	}
	if hasChecked {
		return nil, errors.New("already checked in today")
	}

	quotaAwarded := setting.CheckinMinQuota
	if setting.CheckinMaxQuota > setting.CheckinMinQuota {
		quotaAwarded = setting.CheckinMinQuota + rand.Intn(setting.CheckinMaxQuota-setting.CheckinMinQuota+1)
	}

	today := time.Now().Format("2006-01-02")
	checkin := &Checkin{
		ProviderId:   user.ProviderId,
		UserId:       userId,
		CheckinDate:  today,
		QuotaAwarded: quotaAwarded,
		CreatedAt:    time.Now().Unix(),
	}

	if common.UsingSQLite {
		return userCheckinWithoutTransaction(checkin, userId, quotaAwarded)
	}
	return userCheckinWithTransaction(checkin, userId, quotaAwarded)
}

// userCheckinWithTransaction 使用事务执行签到（适用于 MySQL 和 PostgreSQL）
func userCheckinWithTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(checkin).Error; err != nil {
			return errors.New("check-in failed, please try again later")
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
			"quota":        gorm.Expr("quota + ?", quotaAwarded),
			"reward_quota": gorm.Expr("reward_quota + ?", quotaAwarded),
		}).Error; err != nil {
			return errors.New("check-in failed: quota update error")
		}
		if checkin.ProviderId > 0 {
			if err := CreateRewardRecordTx(tx, &RewardRecord{
				ProviderId:  checkin.ProviderId,
				UserId:      userId,
				SourceType:  "checkin",
				SourceId:    checkin.Id,
				Quota:       quotaAwarded,
				Description: "checkin reward",
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	go func() {
		_ = cacheIncrUserQuota(userId, int64(quotaAwarded))
	}()
	return checkin, nil
}

// userCheckinWithoutTransaction 不使用事务执行签到（适用于 SQLite）
func userCheckinWithoutTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, error) {
	if err := DB.Create(checkin).Error; err != nil {
		return nil, errors.New("check-in failed, please try again later")
	}
	if err := IncreaseUserRewardQuota(userId, quotaAwarded, true); err != nil {
		DB.Delete(checkin)
		return nil, errors.New("check-in failed: quota update error")
	}
	if checkin.ProviderId > 0 {
		if err := CreateRewardRecord(&RewardRecord{
			ProviderId:  checkin.ProviderId,
			UserId:      userId,
			SourceType:  "checkin",
			SourceId:    checkin.Id,
			Quota:       quotaAwarded,
			Description: "checkin reward",
		}); err != nil {
			DB.Delete(checkin)
			return nil, err
		}
	}
	return checkin, nil
}

// GetUserCheckinStats 获取用户签到统计信息
func GetUserCheckinStats(userId int, month string) (map[string]interface{}, error) {
	return GetUserCheckinStatsInProvider(0, userId, month)
}

func GetUserCheckinStatsInProvider(providerId int, userId int, month string) (map[string]interface{}, error) {
	// 获取指定月份的所有签到记录
	startDate := month + "-01"
	endDate := month + "-31"

	records, err := GetUserCheckinRecordsInProvider(providerId, userId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 转换为不包含敏感字段的记录
	checkinRecords := make([]CheckinRecord, len(records))
	for i, r := range records {
		checkinRecords[i] = CheckinRecord{
			CheckinDate:  r.CheckinDate,
			QuotaAwarded: r.QuotaAwarded,
		}
	}

	// 检查今天是否已签到
	var hasCheckedToday bool
	if providerId > 0 {
		hasCheckedToday, _ = HasCheckedInTodayInProvider(providerId, userId)
	} else {
		hasCheckedToday, _ = HasCheckedInToday(userId)
	}

	// 获取用户所有时间的签到统计
	var totalCheckins int64
	var totalQuota int64
	totalQuery := DB.Model(&Checkin{}).Where("user_id = ?", userId)
	if providerId > 0 {
		totalQuery = totalQuery.Where("provider_id = ?", providerId)
	}
	totalQuery.Count(&totalCheckins)
	totalQuery.Select("COALESCE(SUM(quota_awarded), 0)").Scan(&totalQuota)

	return map[string]interface{}{
		"total_quota":      totalQuota,      // 所有时间累计获得的额度
		"total_checkins":   totalCheckins,   // 所有时间累计签到次数
		"checkin_count":    len(records),    // 本月签到次数
		"checked_in_today": hasCheckedToday, // 今天是否已签到
		"records":          checkinRecords,  // 本月签到记录详情（不含id和user_id）
	}, nil
}
