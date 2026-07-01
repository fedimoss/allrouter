package model

import (
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

var serviceConfigColumnPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type ServiceConfig struct {
	ID int64 `json:"id"`

	MerchantPID string `json:"merchant_pid" gorm:"column:merchant_pid"`
	MerchantKey string `json:"merchant_key" gorm:"column:merchant_key"`

	WechatAppID          string `json:"wechat_app_id" gorm:"column:wechat_app_id"`
	WechatMchID          string `json:"wechat_mch_id" gorm:"column:wechat_mch_id"`
	WechatMchSerialNo    string `json:"wechat_mch_serial_no" gorm:"column:wechat_mch_serial_no"`
	WechatPrivateKeyPath string `json:"wechat_private_key_path" gorm:"column:wechat_private_key_path"`
	WechatPublicKeyPath  string `json:"wechat_public_key_path" gorm:"column:wechat_public_key_path"`
	WechatAPIV3Key       string `json:"wechat_apiv3_key" gorm:"column:wechat_apiv3_key"`
	WechatNotifyURL      string `json:"wechat_notify_url" gorm:"column:wechat_notify_url"`

	AlipayAppID      string `json:"alipay_app_id" gorm:"column:alipay_app_id"`
	AlipayPrivateKey string `json:"alipay_private_key" gorm:"column:alipay_private_key"`
	AlipayPublicKey  string `json:"alipay_public_key" gorm:"column:alipay_public_key"`
	AlipayNotifyURL  string `json:"alipay_notify_url" gorm:"column:alipay_notify_url"`
	AlipayIsProd     bool   `json:"alipay_is_prod" gorm:"column:alipay_is_prod"`

	USDTEnabled         bool    `json:"usdt_enabled" gorm:"column:usdt_enabled"`
	USDTTRC20Address    string  `json:"usdt_trc20_address" gorm:"column:usdt_trc20_address"`
	USDTTrongridAPIKey  string  `json:"usdt_trongrid_api_key" gorm:"column:usdt_trongrid_api_key"`
	USDTCNYRate         float64 `json:"usdt_cny_rate" gorm:"column:usdt_cny_rate"`
	USDTPollIntervalSec int64   `json:"usdt_poll_interval_sec" gorm:"column:usdt_poll_interval_sec"`
	USDTExpiryMinutes   int64   `json:"usdt_expiry_minutes" gorm:"column:usdt_expiry_minutes"`

	MerchantPId string `json:"merchant_p_id" gorm:"column:merchant_p_id"`
}

func (ServiceConfig) TableName() string {
	return "service_configs"
}

func GetLatestServiceConfig() (*ServiceConfig, error) {
	var config ServiceConfig
	err := DB.Order("id desc").Limit(1).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func GetLatestServiceConfigTx(tx *gorm.DB) (*ServiceConfig, error) {
	if tx == nil {
		return GetLatestServiceConfig()
	}
	var config ServiceConfig
	err := tx.Order("id desc").Limit(1).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func GetLatestServiceConfigColumn(column string) (string, error) {
	column = strings.TrimSpace(column)
	if column == "" {
		return "", fmt.Errorf("service config column is empty")
	}
	if !serviceConfigColumnPattern.MatchString(column) {
		return "", fmt.Errorf("invalid service config column: %s", column)
	}
	var value string
	query := fmt.Sprintf("SELECT %s FROM service_configs ORDER BY id DESC LIMIT 1", column)
	if err := DB.Raw(query).Scan(&value).Error; err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}
