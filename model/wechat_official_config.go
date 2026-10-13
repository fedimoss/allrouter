package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
)

type WechatOfficialConfig struct {
	Id         int    `gorm:"primaryKey" json:"id"`
	ProviderId int    `gorm:"type:int;not null;default:0;uniqueIndex" json:"provider_id"`
	AppId      string `gorm:"type:varchar(255);not null" json:"app_id"`
	AppSecret  string `gorm:"type:varchar(512);not null" json:"-"`
	// Defaults are applied by the controller so GORM preserves an explicit false on creation.
	Enabled   bool  `gorm:"type:boolean;not null" json:"enabled"`
	CreatedAt int64 `gorm:"bigint;not null" json:"created_at"`
	UpdatedAt int64 `gorm:"bigint;not null" json:"updated_at"`
}

func (WechatOfficialConfig) TableName() string {
	return "wechat_official_configs"
}

// GetWechatConfigByProvider 根据服务商ID获取微信配置
func GetWechatConfigByProvider(providerId int) (*WechatOfficialConfig, error) {
	var config WechatOfficialConfig
	err := DB.Where("provider_id = ?", providerId).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// CreateWechatConfig 创建微信配置
func CreateWechatConfig(config *WechatOfficialConfig) error {
	if config == nil {
		return errors.New("config is nil")
	}
	now := common.GetTimestamp()
	config.CreatedAt = now
	config.UpdatedAt = now
	return DB.Create(config).Error
}

// UpdateWechatConfig 更新微信配置
func UpdateWechatConfig(config *WechatOfficialConfig) error {
	if config == nil {
		return errors.New("config is nil")
	}
	config.UpdatedAt = common.GetTimestamp()
	return DB.Model(&WechatOfficialConfig{}).
		Where("provider_id = ?", config.ProviderId).
		Updates(map[string]interface{}{
			"app_id":     config.AppId,
			"app_secret": config.AppSecret,
			"enabled":    config.Enabled,
			"updated_at": config.UpdatedAt,
		}).Error
}

// DeleteWechatConfig 删除微信配置
func DeleteWechatConfig(providerId int) error {
	return DB.Where("provider_id = ?", providerId).Delete(&WechatOfficialConfig{}).Error
}
