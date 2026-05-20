package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// VersionLog 版本日志
type VersionLog struct {
	Id        int64  `json:"id"`
	Version   string `json:"version" gorm:"size:64"`
	Log       string `json:"log" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;default:0"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint;default:0"`
}

func (v *VersionLog) TableName() string {
	return "version_log"
}

func (v *VersionLog) BeforeCreate() error {
	now := common.GetTimestamp()
	if v.CreatedAt == 0 {
		v.CreatedAt = now
	}
	if v.UpdatedAt == 0 {
		v.UpdatedAt = now
	}
	return nil
}

func (v *VersionLog) Insert() error {
	return DB.Create(v).Error
}

func (v *VersionLog) Update() error {
	v.UpdatedAt = common.GetTimestamp()
	return DB.Model(v).Update("updated_at", v.UpdatedAt).Error
}

// GetAllVersionLogs 获取所有版本日志
func GetAllVersionLogs() ([]*VersionLog, error) {
	var logs []*VersionLog
	err := DB.Order("created_at DESC").Find(&logs).Error
	return logs, err
}

// InsertVersionLog 插入版本日志
func InsertVersionLog(db *gorm.DB, version string, log string) error {
	now := common.GetTimestamp()
	v := &VersionLog{
		Version:   version,
		Log:       log,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return db.Create(v).Error
}

// GetVersionLogById 根据ID获取版本日志
func GetVersionLogById(id int64) (*VersionLog, error) {
	var log VersionLog
	err := DB.Where("id = ?", id).First(&log).Error
	return &log, err
}

// GetLatestVersionLog 获取最新一条版本日志
func GetLatestVersionLog() (*VersionLog, error) {
	var log VersionLog
	err := DB.Order("created_at DESC").First(&log).Error
	return &log, err
}

// DeleteVersionLogById 根据ID删除版本日志
func DeleteVersionLogById(db *gorm.DB, id int64) error {
	return db.Where("id = ?", id).Delete(&VersionLog{}).Error
}
