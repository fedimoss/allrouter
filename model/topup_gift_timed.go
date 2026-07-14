package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ProviderTopUpGiftTimedOptionKey 是服务商倒计时在 provider_options 中的键名。
const ProviderTopUpGiftTimedOptionKey = "topup_gift.timed"

// TopUpGiftTimedConfig 是可公开返回的充值赠送倒计时配置。
type TopUpGiftTimedConfig struct {
	Enabled bool  `json:"enabled"`
	Day     int   `json:"day"`
	EndTime int64 `json:"end_time"`
}

// NormalizeTopUpGiftTimedConfig 校验管理端输入，并由服务端将相对天数锚定为绝对结束时间。
func NormalizeTopUpGiftTimedConfig(raw string, now time.Time) (string, error) {
	var request TopUpGiftTimedConfig
	if err := common.UnmarshalJsonStr(raw, &request); err != nil {
		return "", fmt.Errorf("充值赠送倒计时配置格式无效: %w", err)
	}
	if request.Day < 0 {
		return "", fmt.Errorf("充值赠送倒计时天数不能小于 0")
	}
	if request.Enabled && request.Day < 1 {
		return "", fmt.Errorf("启用充值赠送倒计时时，天数必须至少为 1")
	}

	request.EndTime = 0
	if request.Enabled {
		request.EndTime = now.AddDate(0, 0, request.Day).Unix()
	}
	data, err := common.Marshal(request)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ParseTopUpGiftTimedConfig 解析持久化配置；空值表示该站点尚未配置倒计时。
func ParseTopUpGiftTimedConfig(raw string) (TopUpGiftTimedConfig, error) {
	var config TopUpGiftTimedConfig
	if strings.TrimSpace(raw) == "" {
		return config, nil
	}
	if err := common.UnmarshalJsonStr(raw, &config); err != nil {
		return config, fmt.Errorf("解析充值赠送倒计时配置失败: %w", err)
	}
	return config, nil
}

// LoadTopUpGiftTimedConfig 按 provider 维度读取当前站点倒计时。
// 服务商必须显式配置，不继承主站活动，避免不同站点展示同一截止时间。
func LoadTopUpGiftTimedConfig(providerId int) (TopUpGiftTimedConfig, error) {
	var raw string
	if providerId > 0 {
		value, err := GetProviderOptionValue(providerId, ProviderTopUpGiftTimedOptionKey)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TopUpGiftTimedConfig{}, nil
		}
		if err != nil {
			return TopUpGiftTimedConfig{}, err
		}
		raw = value
	} else {
		common.OptionMapRWMutex.RLock()
		raw = common.TopUpGiftTimed
		common.OptionMapRWMutex.RUnlock()
	}

	config, err := ParseTopUpGiftTimedConfig(raw)
	if err != nil || !config.Enabled || config.EndTime > 0 {
		return config, err
	}

	// 旧配置只有 enabled/day：首次读取时补齐并持久化 end_time，避免升级后隐藏或每次刷新重新计时。
	normalized, err := NormalizeTopUpGiftTimedConfig(raw, time.Now())
	if err != nil {
		return TopUpGiftTimedConfig{}, err
	}
	if providerId > 0 {
		err = UpdateProviderOption(providerId, ProviderTopUpGiftTimedOptionKey, normalized)
	} else {
		err = UpdateOption("TopUpGiftTimed", normalized)
	}
	if err != nil {
		return TopUpGiftTimedConfig{}, err
	}
	return ParseTopUpGiftTimedConfig(normalized)
}
