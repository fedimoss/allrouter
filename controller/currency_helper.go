package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// normalizeDisplayMoneyDecimal 将金额统一收敛到 6 位小数，用于展示币种的精度处理
func normalizeDisplayMoneyDecimal(value float64) decimal.Decimal {
	return decimal.NewFromFloat(value).Round(6)
}

// getDisplayCurrencyForUser 从 gin.Context 解析当前登录用户的展示币种信息
// 未登录或获取失败时返回默认 USD
func getDisplayCurrencyForUser(c *gin.Context) model.DisplayCurrencyInfo {
	userId := c.GetInt("id")
	if userId <= 0 {
		return model.GetDisplayCurrencyInfoByTimezone("")
	}
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		return model.GetDisplayCurrencyInfoByTimezone("")
	}
	return model.GetDisplayCurrencyInfoByTimezone(user.Timezone)
}

// convertQuotaToDisplay 将内部额度转换为展示币种金额
// 内部额度 ÷ QuotaPerUnit → 美元（6 位小数）→ × 汇率（如 CNY）→ 最终金额
func convertQuotaToDisplay(quota int, info model.DisplayCurrencyInfo) float64 {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	usd := decimal.NewFromInt(int64(quota)).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(6).
		InexactFloat64()
	return convertUsdToDisplay(usd, info)
}

// convertUsdToDisplay 将美元金额转换为展示币种金额
// USD 用户：保留 2 位小数直接返回
// CNY 用户：× 汇率后保留 2 位小数返回
func convertUsdToDisplay(usdAmount float64, info model.DisplayCurrencyInfo) float64 {
	amount := normalizeDisplayMoneyDecimal(usdAmount)
	if info.Currency != "CNY" {
		return amount.Round(2).InexactFloat64()
	}
	return amount.
		Mul(normalizeDisplayMoneyDecimal(info.Rate)).
		Round(6).
		Round(2).
		InexactFloat64()
}
