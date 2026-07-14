package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const ProviderTopUpGiftTimedOptionKey = "topup_gift.timed"

// TopUpGiftTimedConfig is the public countdown configuration for a top-up gift campaign.
type TopUpGiftTimedConfig struct {
	Enabled bool  `json:"enabled"`
	Day     int   `json:"day"`
	EndTime int64 `json:"end_time"`
}

// NormalizeTopUpGiftTimedConfig validates an admin request and anchors the
// countdown to an absolute server-generated end time.
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

// LoadTopUpGiftTimedConfig returns the current site's countdown configuration.
// Providers must opt in explicitly and never inherit the main site's campaign.
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

	// Legacy values only contained enabled/day. Anchor and persist them once so
	// upgrading does not hide the countdown or restart it on every page load.
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
