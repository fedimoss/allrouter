package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// latestVersionLogCacheKey 最新版本日志缓存键
const latestVersionLogCacheKey = "versionLog:latest"

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
func InsertVersionLog(db *gorm.DB, version string, log string) (*VersionLog, error) {
	now := common.GetTimestamp()
	v := &VersionLog{
		Version:   version,
		Log:       log,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return v, db.Create(v).Error
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
	err := DB.Order("created_at DESC, id DESC").First(&log).Error
	return &log, err
}

// DeleteVersionLogById 根据ID删除版本日志
func DeleteVersionLogById(db *gorm.DB, id int64) error {
	return db.Where("id = ?", id).Delete(&VersionLog{}).Error
}

// latestVersionLogCacheTTL 获取版本日志缓存过期时间，默认 300 秒
func latestVersionLogCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("VERSION_LOG_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		common.SysLog(fmt.Sprintf("版本日志缓存 TTL 配置值 %d 无效，使用默认值 300 秒", ttlSeconds))
		ttlSeconds = 300
	}
	common.SysLog(fmt.Sprintf("版本日志缓存 TTL: %d 秒", ttlSeconds))
	return time.Duration(ttlSeconds) * time.Second
}

func versionLogRedisEnabled() bool {
	return common.RedisEnabled && common.RDB != nil
}

// GetLatestVersionLogCached 从缓存获取最新版本日志，Redis 不可用时回退到数据库
func GetLatestVersionLogCached() (*VersionLog, error) {
	if versionLogRedisEnabled() {
		raw, err := common.RedisGet(latestVersionLogCacheKey)
		if err != nil {
			common.SysError(fmt.Sprintf("从 Redis 获取版本日志缓存失败: %v", err))
		} else if raw != "" {
			var log VersionLog
			if err := common.UnmarshalJsonStr(raw, &log); err == nil {
				common.SysLog("从 Redis 缓存命中最新版本日志")
				return &log, nil
			}
			common.SysError(fmt.Sprintf("反序列化版本日志缓存数据失败，已删除无效缓存: %v", err))
			_ = common.RedisDelKey(latestVersionLogCacheKey)
		} else {
			common.SysLog("Redis 缓存未命中，将从数据库获取最新版本日志")
		}
	}

	latest, err := GetLatestVersionLog()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.SysLog("数据库中无版本日志记录")
		return nil, nil
	}
	if err != nil {
		common.SysError(fmt.Sprintf("从数据库获取最新版本日志失败: %v", err))
		return nil, err
	}
	common.SysLog(fmt.Sprintf("从数据库获取最新版本日志成功，版本: %s", latest.Version))
	_ = SetLatestVersionLogCache(latest)
	return latest, nil
}

// SetLatestVersionLogCache 将最新版本日志写入 Redis 缓存，Redis 未启用时不执行任何操作
func SetLatestVersionLogCache(log *VersionLog) error {
	if !versionLogRedisEnabled() {
		return nil
	}
	if log == nil {
		common.SysLog("版本日志为空，清除 Redis 缓存")
		return common.RedisDelKey(latestVersionLogCacheKey)
	}
	data, err := common.Marshal(log)
	if err != nil {
		common.SysError(fmt.Sprintf("序列化版本日志失败: %v", err))
		return err
	}
	err = common.RedisSet(latestVersionLogCacheKey, string(data), latestVersionLogCacheTTL())
	if err != nil {
		common.SysError(fmt.Sprintf("写入版本日志缓存失败: %v", err))
		return err
	}
	common.SysLog(fmt.Sprintf("版本日志缓存写入成功，版本: %s", log.Version))
	return nil
}

// RefreshLatestVersionLogCacheFromDB 从数据库重新同步最新版本日志到缓存
func RefreshLatestVersionLogCacheFromDB() error {
	common.SysLog("开始刷新版本日志缓存")
	latest, err := GetLatestVersionLog()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.SysLog("数据库中无版本日志记录，清除缓存")
		return SetLatestVersionLogCache(nil)
	}
	if err != nil {
		common.SysError(fmt.Sprintf("刷新版本日志缓存时从数据库查询失败: %v", err))
		return err
	}
	common.SysLog(fmt.Sprintf("刷新版本日志缓存成功，最新版本: %s", latest.Version))
	return SetLatestVersionLogCache(latest)
}
