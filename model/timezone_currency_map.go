package model

import (
	"time"
)

// TimezoneCurrencyMap 时区与币种的映射关系表
// 用于根据用户所在时区自动推断应使用的支付币种
type TimezoneCurrencyMap struct {
	Timezone  string    `json:"timezone" gorm:"type:varchar(64);primaryKey"` // 时区标识，如 Asia/Shanghai
	Currency  string    `json:"currency" gorm:"type:varchar(3);not null"`    // 对应的币种代码，如 CNY
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`            // 最后更新时间
}

// TableName 指定 GORM 表名
func (TimezoneCurrencyMap) TableName() string {
	return "timezone_currency_map"
}

// GetCurrencyByTimezone 精确匹配时区对应的币种代码
// 未找到或时区为空时返回空字符串
func GetCurrencyByTimezone(timezone string) string {
	if timezone == "" {
		return ""
	}
	var m TimezoneCurrencyMap
	if err := DB.Where("timezone = ?", timezone).First(&m).Error; err != nil {
		return ""
	}
	return m.Currency
}

// GetCurrencyByTimezoneWithFallback 先精确匹配，再按前缀（如 Asia/）模糊匹配，都无则返回默认币种
func GetCurrencyByTimezoneWithFallback(timezone string, defaultCurrency string) string {
	if timezone == "" {
		return defaultCurrency
	}
	// 第一步：精确匹配完整时区字符串
	var m TimezoneCurrencyMap
	if err := DB.Where("timezone = ?", timezone).First(&m).Error; err == nil {
		return m.Currency
	}
	// 第二步：提取时区前缀（如 Asia），模糊匹配同一大洲的时区
	var region string
	for i, c := range timezone {
		if c == '/' {
			region = timezone[:i]
			break
		}
	}
	if region != "" {
		prefix := region + "/"
		// 按时区升序取第一条匹配记录作为同区域回退
		if err := DB.Where("timezone LIKE ?", prefix+"%").Order("timezone ASC").First(&m).Error; err == nil {
			return m.Currency
		}
	}
	// 第三步：均未命中，返回调用方提供的默认币种
	return defaultCurrency
}

// GetAllTimezoneMaps 查询所有时区映射记录
func GetAllTimezoneMaps() ([]TimezoneCurrencyMap, error) {
	var maps []TimezoneCurrencyMap
	err := DB.Find(&maps).Error
	return maps, err
}

// BatchGetTimezoneCurrency 批量查询时区对应的币种，返回 map[timezone]currency
// 传入空切片时直接返回 nil
func BatchGetTimezoneCurrency(timezones []string) (map[string]string, error) {
	if len(timezones) == 0 {
		return nil, nil
	}
	var maps []TimezoneCurrencyMap
	if err := DB.Where("timezone IN ?", timezones).Find(&maps).Error; err != nil {
		return nil, err
	}
	// 将查询结果转为 map 方便按 O(1) 查找
	result := make(map[string]string, len(maps))
	for _, m := range maps {
		result[m.Timezone] = m.Currency
	}
	return result, nil
}

// DeleteTimezoneMap 删除指定时区的映射记录
func DeleteTimezoneMap(timezone string) error {
	return DB.Where("timezone = ?", timezone).Delete(&TimezoneCurrencyMap{}).Error
}

// UpdateTimezoneCurrency 更新或创建一条时区映射（GORM Save 语义，存在则更新，不存在则插入）
func UpdateTimezoneCurrency(timezone string, currency string) error {
	return DB.Save(&TimezoneCurrencyMap{Timezone: timezone, Currency: currency}).Error
}

// SearchTimezoneMap 按时区前缀或币种代码模糊搜索，返回分页结果
// keyword 为搜索关键词，offset/limit 为分页参数
func SearchTimezoneMap(keyword string, offset int, limit int) ([]TimezoneCurrencyMap, int64, error) {
	var maps []TimezoneCurrencyMap
	var total int64
	query := DB.Model(&TimezoneCurrencyMap{})
	// 有关键词时，按时区或币种进行模糊匹配
	if keyword != "" {
		query = query.Where("timezone LIKE ? OR currency LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	// 先查总数用于分页
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 再查分页数据，按时区升序排列
	if err := query.Offset(offset).Limit(limit).Order("timezone ASC").Find(&maps).Error; err != nil {
		return nil, 0, err
	}
	return maps, total, nil
}
