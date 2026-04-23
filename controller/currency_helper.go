package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

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
// 内部额度 ÷ QuotaPerUnit → 美元 → 四舍五入 → × 汇率 → 四舍五入
func convertQuotaToDisplay(quota int, info model.DisplayCurrencyInfo) float64 {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	usd := float64(quota) / common.QuotaPerUnit
	usd = model.RoundDisplayCurrencyAmount(usd)
	return convertUsdToDisplay(usd, info)
}

// convertUsdToDisplay 将美元金额转换为展示币种金额
// USD → 四舍五入 → × 汇率（非 CNY 时直接返回）
func convertUsdToDisplay(usdAmount float64, info model.DisplayCurrencyInfo) float64 {
	amount := model.RoundDisplayCurrencyAmount(usdAmount)
	if info.Currency != "CNY" {
		return amount
	}
	return model.RoundDisplayCurrencyAmount(amount * info.Rate)
}
