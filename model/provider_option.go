package model

import "github.com/QuantumNous/new-api/common"

type ProviderOption struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ProviderId int    `json:"provider_id" gorm:"index"`
	Key        string `json:"key" gorm:"type:text;not null"`
	Value      string `json:"value" gorm:"type:text"`
}

// GetProviderOptions 获取服务商配置
func GetProviderOptions(providerId int) ([]*ProviderOption, error) {
	var options []*ProviderOption
	err := DB.Where("provider_id = ?", providerId).Find(&options).Error
	return options, err
}

// UpdateProviderOption 更新服务商配置
func GetProviderOptionValue(providerId int, key string) (string, error) {
	var option ProviderOption
	err := DB.Where("provider_id = ? AND key = ?", providerId, key).First(&option).Error
	if err != nil {
		return "", err
	}
	return option.Value, nil
}

func GetProviderAnnouncements(providerId int) []map[string]interface{} {
	value, err := GetProviderOptionValue(providerId, "console_setting.announcements")
	if err != nil || value == "" {
		return nil
	}
	var list []map[string]interface{}
	if err := common.UnmarshalJsonStr(value, &list); err != nil {
		return nil
	}
	return list
}

func UpdateProviderOption(providerId int, key string, value string) error {
	option := ProviderOption{
		ProviderId: providerId,
		Key:        key,
	}
	if err := DB.Where("provider_id = ? AND key = ?", providerId, key).FirstOrCreate(&option).Error; err != nil {
		return err
	}
	option.Value = value
	return DB.Save(&option).Error
}
