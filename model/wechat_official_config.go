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
	// ServerAddress and ServerToken are the optional compatibility bridge used
	// by the legacy verification-code login service. They are scoped to the
	// same site as the official account credentials; the token is never exposed
	// in API responses.
	ServerAddress string `gorm:"type:varchar(2048)" json:"server_address"`
	ServerToken   string `gorm:"type:varchar(512)" json:"-"`
	QRCodeURL     string `gorm:"type:text" json:"qrcode_url"`
	// Defaults are applied by the controller so GORM preserves an explicit false on creation.
	Enabled   bool  `gorm:"type:boolean;not null" json:"enabled"`
	CreatedAt int64 `gorm:"bigint;not null" json:"created_at"`
	UpdatedAt int64 `gorm:"bigint;not null" json:"updated_at"`
}

func (WechatOfficialConfig) TableName() string {
	return "wechat_official_configs"
}

type wechatOfficialConfigSchema struct {
	Id            int    `gorm:"primaryKey"`
	ProviderId    int    `gorm:"type:int;not null;default:0;uniqueIndex"`
	AppId         string `gorm:"type:varchar(255);not null"`
	AppSecret     string `gorm:"type:varchar(512);not null"`
	ServerAddress string `gorm:"type:varchar(2048)"`
	ServerToken   string `gorm:"type:varchar(512)"`
	QRCodeURL     string `gorm:"type:text"`
	Enabled       bool   `gorm:"type:boolean;not null"`
	CreatedAt     int64  `gorm:"type:bigint;not null"`
	UpdatedAt     int64  `gorm:"type:bigint;not null"`
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

const (
	WechatLoginSessionPending    = "pending"
	WechatLoginSessionScanned    = "scanned"
	WechatLoginSessionAuthorized = "authorized"
	WechatLoginSessionConsumed   = "consumed"
	WechatLoginSessionExpired    = "expired"
)

// WechatLoginSession is the short-lived bridge between the desktop browser
// that displays a QR code and the mobile browser that completes official
// account web authorization. It contains no secret and is single-use.
type WechatLoginSession struct {
	Id         int    `gorm:"primaryKey"`
	State      string `gorm:"type:varchar(128);not null;uniqueIndex"`
	ProviderId int    `gorm:"type:int;not null;index"`
	OpenID     string `gorm:"type:varchar(191)"`
	Status     string `gorm:"type:varchar(24);not null;index"`
	ExpiresAt  int64  `gorm:"bigint;not null;index"`
	CreatedAt  int64  `gorm:"bigint;not null"`
	UpdatedAt  int64  `gorm:"bigint;not null"`
}

func (WechatLoginSession) TableName() string {
	return "wechat_login_sessions"
}

type wechatLoginSessionSchema struct {
	Id         int    `gorm:"primaryKey"`
	State      string `gorm:"type:varchar(128);not null;uniqueIndex"`
	ProviderId int    `gorm:"type:int;not null;index"`
	OpenID     string `gorm:"type:varchar(191)"`
	Status     string `gorm:"type:varchar(24);not null;index"`
	ExpiresAt  int64  `gorm:"bigint;not null;index"`
	CreatedAt  int64  `gorm:"bigint;not null"`
	UpdatedAt  int64  `gorm:"bigint;not null"`
}

func (wechatLoginSessionSchema) TableName() string {
	return "wechat_login_sessions"
}

func migrateWechatOfficialConfigSchema() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}
	if err := DB.AutoMigrate(&wechatOfficialConfigSchema{}, &wechatOfficialIdentitySchema{}, &wechatLoginSessionSchema{}); err != nil {
		return fmt.Errorf("migrate wechat official config schema: %w", err)
	}
	return nil
}

func CreateWechatLoginSession(providerId int, state string, expiresAt int64) error {
	if providerId < 0 || state == "" || expiresAt <= 0 {
		return errors.New("invalid WeChat login session")
	}
	now := common.GetTimestamp()
	_ = DB.Where("expires_at < ?", now-86400).Delete(&WechatLoginSession{}).Error
	return DB.Create(&WechatLoginSession{
		ProviderId: providerId,
		State:      state,
		Status:     WechatLoginSessionPending,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}).Error
}

func GetWechatLoginSessionStatus(providerId int, state string, now int64) (string, error) {
	if providerId < 0 || state == "" {
		return WechatLoginSessionExpired, errors.New("invalid WeChat login session")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	session, err := GetWechatLoginSession(providerId, state)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WechatLoginSessionExpired, nil
	}
	if err != nil {
		return WechatLoginSessionExpired, err
	}
	if session.ExpiresAt <= now && session.Status != WechatLoginSessionConsumed {
		_ = DB.Model(&WechatLoginSession{}).Where("id = ?", session.Id).Updates(map[string]interface{}{
			"status":     WechatLoginSessionExpired,
			"updated_at": now,
		}).Error
		return WechatLoginSessionExpired, nil
	}
	return session.Status, nil
}

func GetWechatLoginSession(providerId int, state string) (*WechatLoginSession, error) {
	if providerId < 0 || state == "" {
		return nil, errors.New("invalid WeChat login session")
	}
	var session WechatLoginSession
	if err := DB.Where("provider_id = ? AND state = ?", providerId, state).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// ScanWechatLoginSession stores the OpenID after WeChat redirects back. The
// user must still confirm on the phone before the desktop can consume it.
func ScanWechatLoginSession(providerId int, state string, openID string, now int64) (bool, error) {
	if providerId < 0 || state == "" || openID == "" {
		return false, errors.New("invalid WeChat login authorization")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	result := DB.Model(&WechatLoginSession{}).
		Where("provider_id = ? AND state = ? AND status = ? AND expires_at > ?", providerId, state, WechatLoginSessionPending, now).
		Updates(map[string]interface{}{
			"open_id":    openID,
			"status":     WechatLoginSessionScanned,
			"updated_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

func ConfirmWechatLoginSession(providerId int, state string, now int64) (bool, error) {
	if providerId < 0 || state == "" {
		return false, errors.New("invalid WeChat login confirmation")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	result := DB.Model(&WechatLoginSession{}).
		Where("provider_id = ? AND state = ? AND status = ? AND expires_at > ?", providerId, state, WechatLoginSessionScanned, now).
		Updates(map[string]interface{}{
			"status":     WechatLoginSessionAuthorized,
			"updated_at": now,
		})
	return result.RowsAffected == 1, result.Error
}

// ConsumeWechatLoginSession atomically claims an authorized state for the
// desktop poller and returns its OpenID. A missing, expired, or already used
// state is reported as found=false rather than an error.
func ConsumeWechatLoginSession(providerId int, state string, now int64) (openID string, found bool, err error) {
	if providerId < 0 || state == "" {
		return "", false, errors.New("invalid WeChat login session")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	var session WechatLoginSession
	if err = DB.Where("provider_id = ? AND state = ? AND status = ? AND expires_at > ?", providerId, state, WechatLoginSessionAuthorized, now).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	result := DB.Model(&WechatLoginSession{}).
		Where("id = ? AND status = ?", session.Id, WechatLoginSessionAuthorized).
		Updates(map[string]interface{}{
			"status":     WechatLoginSessionConsumed,
			"updated_at": now,
		})
	if result.Error != nil {
		return "", false, result.Error
	}
	if result.RowsAffected != 1 {
		return "", false, nil
	}
	return session.OpenID, true, nil
}

func ExpireWechatLoginSessions(now int64) error {
	if now <= 0 {
		now = common.GetTimestamp()
	}
	return DB.Model(&WechatLoginSession{}).
		Where("expires_at <= ? AND status IN ?", now, []string{WechatLoginSessionPending, WechatLoginSessionScanned, WechatLoginSessionAuthorized}).
		Updates(map[string]interface{}{
			"status":     WechatLoginSessionExpired,
			"updated_at": now,
		}).Error
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
			"app_id":         config.AppId,
			"app_secret":     config.AppSecret,
			"server_address": config.ServerAddress,
			"server_token":   config.ServerToken,
			"qr_code_url":    config.QRCodeURL,
			"enabled":        config.Enabled,
			"updated_at":     config.UpdatedAt,
		}).Error
}

// UpdateWechatConfigQRCode stores the image returned by WeChat's permanent
// QR-code endpoint. The URL is intentionally written only by the generation
// endpoint; administrators cannot replace it with an arbitrary image URL.
func UpdateWechatConfigQRCode(providerId int, qrcodeURL string) error {
	if providerId < 0 {
		return errors.New("invalid provider id")
	}
	return DB.Model(&WechatOfficialConfig{}).
		Where("provider_id = ?", providerId).
		Updates(map[string]interface{}{
			"qr_code_url": qrcodeURL,
			"updated_at":  common.GetTimestamp(),
		}).Error
}

// DeleteWechatConfig 删除微信配置
func DeleteWechatConfig(providerId int) error {
	return DB.Where("provider_id = ?", providerId).Delete(&WechatOfficialConfig{}).Error
}
