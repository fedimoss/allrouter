package model

import (
	"math"
	"strings"
	"time"
)

// CurrencyStripeConfig 币种与 Stripe 商品价格的映射配置表
// 每个币种对应一个 Stripe Price ID，用于多币种支付场景
type CurrencyStripeConfig struct {
	Currency      string    `json:"currency" gorm:"type:varchar(3);primaryKey"`                                          // 币种代码，如 USD、CNY，作为主键
	StripePriceID string    `json:"stripe_price_id" gorm:"column:stripe_price_id;type:varchar(255);not null;default:''"` // Stripe 上的商品价格 ID（price_xxx）
	UnitPrice     float64   `json:"unit_price" gorm:"type:decimal(18,6);not null;default:0"`                             // 相对于 1 美元的汇率，如 CNY 约为 7.25
	Symbol        string    `json:"symbol" gorm:"type:varchar(10);not null"`                                             // 币种符号，如 $、¥
	UpdatedAt     time.Time `json:"updated_at" gorm:"autoUpdateTime"`                                                    // 最后更新时间，由 GORM 自动维护
}

// DisplayCurrencyInfo 前端展示用的币种信息
// 用于将后端统一存储的美元金额转换为用户本地币种显示
type DisplayCurrencyInfo struct {
	Currency string  `json:"currency"` // 币种代码，如 USD、CNY
	Symbol   string  `json:"symbol"`   // 币种显示符号，如 $、¥
	Rate     float64 `json:"rate"`     // 兑换汇率（相对于 1 美元）
}

// TableName 指定 GORM 表名
func (CurrencyStripeConfig) TableName() string {
	return "currency_stripe_config"
}

// GetCurrencyConfig 根据币种代码查询对应的 Stripe 配置
func GetCurrencyConfig(currency string) (*CurrencyStripeConfig, error) {
	var config CurrencyStripeConfig
	if err := DB.Where("currency = ?", currency).First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// defaultDisplayCurrencyInfo 返回默认的展示币种信息（美元）
func defaultDisplayCurrencyInfo() DisplayCurrencyInfo {
	return DisplayCurrencyInfo{
		Currency: "USD",
		Symbol:   "$",
		Rate:     1,
	}
}

// RoundDisplayCurrencyAmount 将金额四舍五入到两位小数
func RoundDisplayCurrencyAmount(amount float64) float64 {
	return math.Round(amount*100) / 100
}

// GetDisplayCurrencyInfoByTimezone 根据用户时区获取对应的展示币种信息
// 如果时区对应人民币（CNY），返回 CNY 配置；否则返回默认的 USD
func GetDisplayCurrencyInfoByTimezone(timezone string) DisplayCurrencyInfo {
	// 先取默认的美元展示信息
	info := defaultDisplayCurrencyInfo()
	// 根据时区判断是否为人民币用户
	if !strings.EqualFold(GetCurrencyByTimezoneWithFallback(timezone, "USD"), "CNY") {
		return info
	}

	// 查询数据库中的人民币配置
	config, err := GetCurrencyConfig("CNY")
	if err != nil || config == nil || config.UnitPrice <= 0 {
		return info
	}

	// 使用数据库中配置的符号，为空时回退到 ¥
	symbol := strings.TrimSpace(config.Symbol)
	if symbol == "" {
		symbol = "¥"
	}

	// 组装人民币展示信息
	info.Currency = "CNY"
	info.Symbol = symbol
	info.Rate = config.UnitPrice
	return info
}

// ConvertUSDToDisplayCurrencyByTimezone 将美元金额转换为用户时区对应的本地币种金额
// 返回转换后的金额和展示币种信息
func ConvertUSDToDisplayCurrencyByTimezone(amount float64, timezone string) (float64, DisplayCurrencyInfo) {
	info := GetDisplayCurrencyInfoByTimezone(timezone)
	// 非人民币用户直接返回原始金额
	if info.Currency != "CNY" {
		return RoundDisplayCurrencyAmount(amount), info
	}
	// 人民币用户按汇率换算
	return RoundDisplayCurrencyAmount(amount * info.Rate), info
}

// GetStripeConfigByTimezone 根据时区查找对应的币种和 Stripe 价格配置
// 回退链：精确匹配 → 前缀模糊匹配 → defaultCurrency → 返回 nil
func GetStripeConfigByTimezone(timezone string, defaultCurrency string) (*CurrencyStripeConfig, error) {
	currency := GetCurrencyByTimezoneWithFallback(timezone, defaultCurrency)
	if currency == "" {
		return nil, nil
	}
	return GetCurrencyConfig(currency)
}

// GetAllCurrencyConfigs 查询所有币种配置，按币种代码升序排列
func GetAllCurrencyConfigs() ([]CurrencyStripeConfig, error) {
	var configs []CurrencyStripeConfig
	err := DB.Order("currency ASC").Find(&configs).Error
	return configs, err
}

// GetEnabledCurrencyConfigs 只返回已配置 Stripe Price ID 的币种
// 未配置价格 ID 的币种不参与 Stripe 支付
func GetEnabledCurrencyConfigs() ([]CurrencyStripeConfig, error) {
	var configs []CurrencyStripeConfig
	err := DB.Where("stripe_price_id != '' AND stripe_price_id IS NOT NULL").Order("currency ASC").Find(&configs).Error
	return configs, err
}

// UpdateCurrencyConfig 创建或更新一条币种配置（GORM Save 语义）
func UpdateCurrencyConfig(config *CurrencyStripeConfig) error {
	return DB.Save(config).Error
}

// DeleteCurrencyConfig 删除指定币种的配置（仅管理后台用）
func DeleteCurrencyConfig(currency string) error {
	return DB.Where("currency = ?", currency).Delete(&CurrencyStripeConfig{}).Error
}

// SearchCurrencyConfig 按币种代码或符号模糊搜索，返回分页结果
// keyword 为搜索关键词，offset/limit 为分页参数
func SearchCurrencyConfig(keyword string, offset int, limit int) ([]CurrencyStripeConfig, int64, error) {
	var configs []CurrencyStripeConfig
	var total int64
	query := DB.Model(&CurrencyStripeConfig{})
	// 有关键词时，按币种代码或符号进行模糊匹配
	if keyword != "" {
		query = query.Where("currency LIKE ? OR symbol LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	// 先查总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	// 再查分页数据
	if err := query.Offset(offset).Limit(limit).Order("currency ASC").Find(&configs).Error; err != nil {
		return nil, 0, err
	}
	return configs, total, nil
}
