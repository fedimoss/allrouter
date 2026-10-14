package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type WechatOfficialConfig struct {
	Id         int    `gorm:"primaryKey" json:"id"`
	ProviderId int    `gorm:"type:int;not null;default:0;uniqueIndex" json:"provider_id"`
	AppId      string `gorm:"type:varchar(255);not null" json:"app_id"`
	AppSecret  string `gorm:"type:varchar(512);not null" json:"-"`
	QRCodeURL  string `gorm:"type:text" json:"qrcode_url"`
	// Defaults are applied by the controller so GORM preserves an explicit false on creation.
	Enabled   bool  `gorm:"type:boolean;not null" json:"enabled"`
	CreatedAt int64 `gorm:"bigint;not null" json:"created_at"`
	UpdatedAt int64 `gorm:"bigint;not null" json:"updated_at"`
}

func (WechatOfficialConfig) TableName() string {
	return "wechat_official_configs"
}

type wechatOfficialConfigSchema struct {
	Id         int    `gorm:"primaryKey"`
	ProviderId int    `gorm:"type:int;not null;default:0;uniqueIndex"`
	AppId      string `gorm:"type:varchar(255);not null"`
	AppSecret  string `gorm:"type:varchar(512);not null"`
	QRCodeURL  string `gorm:"type:text"`
	Enabled    bool   `gorm:"type:boolean;not null"`
	CreatedAt  int64  `gorm:"type:bigint;not null"`
	UpdatedAt  int64  `gorm:"type:bigint;not null"`
}

func (wechatOfficialConfigSchema) TableName() string {
	return "wechat_official_configs"
}

type WechatOfficialIdentity struct {
	Id         int    `gorm:"primaryKey"`
	ProviderId int    `gorm:"type:int;not null;uniqueIndex:idx_wechat_identity_provider_user,priority:1;uniqueIndex:idx_wechat_identity_provider_wechat,priority:1"`
	UserId     int    `gorm:"type:int;not null;uniqueIndex:idx_wechat_identity_provider_user,priority:2"`
	WechatId   string `gorm:"type:varchar(191);not null;uniqueIndex:idx_wechat_identity_provider_wechat,priority:2"`
	CreatedAt  int64  `gorm:"type:bigint;not null"`
	UpdatedAt  int64  `gorm:"type:bigint;not null"`
}

func (WechatOfficialIdentity) TableName() string {
	return "wechat_official_identities"
}

type wechatOfficialIdentitySchema struct {
	Id         int    `gorm:"primaryKey"`
	ProviderId int    `gorm:"type:int;not null;uniqueIndex:idx_wechat_identity_provider_user,priority:1;uniqueIndex:idx_wechat_identity_provider_wechat,priority:1"`
	UserId     int    `gorm:"type:int;not null;uniqueIndex:idx_wechat_identity_provider_user,priority:2"`
	WechatId   string `gorm:"type:varchar(191);not null;uniqueIndex:idx_wechat_identity_provider_wechat,priority:2"`
	CreatedAt  int64  `gorm:"type:bigint;not null"`
	UpdatedAt  int64  `gorm:"type:bigint;not null"`
}

func (wechatOfficialIdentitySchema) TableName() string {
	return "wechat_official_identities"
}

func migrateWechatOfficialConfigSchema() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	if err := DB.AutoMigrate(&wechatOfficialConfigSchema{}, &wechatOfficialIdentitySchema{}); err != nil {
		return fmt.Errorf("migrate wechat official config schema: %w", err)
	}
	return nil
}

func GetWechatOfficialIdentity(providerId int, wechatId string) (*WechatOfficialIdentity, error) {
	var identity WechatOfficialIdentity
	err := DB.Where("provider_id = ? AND wechat_id = ?", providerId, wechatId).First(&identity).Error
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

func GetWechatOfficialIdentityByUser(providerId int, userId int) (*WechatOfficialIdentity, error) {
	var identity WechatOfficialIdentity
	err := DB.Where("provider_id = ? AND user_id = ?", providerId, userId).First(&identity).Error
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

func SaveWechatOfficialIdentity(providerId int, userId int, wechatId string) error {
	if providerId <= 0 || userId <= 0 || wechatId == "" {
		return errors.New("invalid WeChat identity")
	}
	now := common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		var identity WechatOfficialIdentity
		err := tx.Where("provider_id = ? AND user_id = ?", providerId, userId).First(&identity).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&WechatOfficialIdentity{
				ProviderId: providerId,
				UserId:     userId,
				WechatId:   wechatId,
				CreatedAt:  now,
				UpdatedAt:  now,
			}).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&identity).Updates(map[string]interface{}{
			"wechat_id":  wechatId,
			"updated_at": now,
		}).Error
	})
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
			"qrcode_url": config.QRCodeURL,
			"enabled":    config.Enabled,
			"updated_at": config.UpdatedAt,
		}).Error
}

// DeleteWechatConfig 删除微信配置
func DeleteWechatConfig(providerId int) error {
	return DB.Where("provider_id = ?", providerId).Delete(&WechatOfficialConfig{}).Error
}
