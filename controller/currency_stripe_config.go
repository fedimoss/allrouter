package controller

import (
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetAdminCurrencyStripeConfigs 获取所有币种 Stripe 配置（管理后台）
func GetAdminCurrencyStripeConfigs(c *gin.Context) {
	configs, err := model.GetAllCurrencyConfigs()
	if err != nil {
		c.JSON(200, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": configs})
}

// UpdateAdminCurrencyStripeConfigRequest 批量更新币种配置的请求体
type UpdateAdminCurrencyStripeConfigRequest struct {
	Configs []struct {
		Currency      string   `json:"currency"`             // 币种代码，如 USD、CNY
		StripePriceID string   `json:"stripe_price_id"`      // Stripe 商品价格 ID
		UnitPrice     *float64 `json:"unit_price,omitempty"` // 单价（仅 CNY 时需要传入）
	} `json:"configs"`
}

// UpdateAdminCurrencyStripeConfig 批量更新币种 Stripe 价格配置（管理后台）
func UpdateAdminCurrencyStripeConfig(c *gin.Context) {
	var req UpdateAdminCurrencyStripeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(200, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 允许更新的币种白名单，防止任意写入
	allowedCurrencies := map[string]bool{
		"USD": true,
		"CNY": true,
	}

	for _, cfg := range req.Configs {
		// 白名单校验，非允许币种直接跳过
		if !allowedCurrencies[cfg.Currency] {
			continue
		}
		// CNY 必须传入单价，且大于 0
		if cfg.Currency == "CNY" && (cfg.UnitPrice == nil || *cfg.UnitPrice <= 0) {
			c.JSON(200, gin.H{"success": false, "message": "人民币价格比例必须大于 0"})
			return
		}
		// 先查已有记录，保留原有的 unit_price 和 symbol
		existing, err := model.GetCurrencyConfig(cfg.Currency)
		if err != nil {
			// 记录不存在，使用默认值创建新记录
			symbol := "$"
			if cfg.Currency == "CNY" {
				symbol = "¥"
			}
			existing = &model.CurrencyStripeConfig{
				Currency:  cfg.Currency,
				Symbol:    symbol,
				UnitPrice: 0, // 新记录的 unit_price 由前端传入，不硬编码默认值
			}
		}
		// 更新 Stripe Price ID
		existing.StripePriceID = cfg.StripePriceID
		// USD 的单价固定为 1
		if cfg.Currency == "USD" {
			existing.UnitPrice = 1
		}
		// CNY 的单价由前端传入，确保写入数据库
		if cfg.Currency == "CNY" && cfg.UnitPrice != nil {
			existing.UnitPrice = *cfg.UnitPrice
		}
		if err := model.UpdateCurrencyConfig(existing); err != nil {
			c.JSON(200, gin.H{"success": false, "message": err.Error()})
			return
		}
	}

	c.JSON(200, gin.H{"success": true, "message": "更新成功"})
}
