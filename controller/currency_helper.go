package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal" // 高精度十进制运算库，避免浮点数精度丢失
)

// normalizeDisplayMoneyDecimal 将金额统一收敛到 6 位小数
// 使用 decimal 库确保精度，避免浮点数运算的精度丢失问题
// 6 位小数足以覆盖 USD/CNY 等主流法币的精度需求
func normalizeDisplayMoneyDecimal(value float64) decimal.Decimal {
	return decimal.NewFromFloat(value).Round(6)
}

// getDisplayCurrencyForUser 从 gin.Context 中获取当前登录用户的展示币种信息
// 根据用户的时区（timezone）来决定展示 USD 还是 CNY
// 如果用户未登录（userId <= 0）或查询用户失败，返回默认的 USD 币种信息
func getDisplayCurrencyForUser(c *gin.Context) model.DisplayCurrencyInfo {
	// 从 gin.Context 中取出中间件注入的用户 ID
	userId := c.GetInt("id")
	// 未登录用户，返回默认币种（USD）
	if userId <= 0 {
		return model.GetDisplayCurrencyInfoByTimezone("")
	}
	// 根据 userId 查询用户信息（不从缓存获取，false 表示不强制命中缓存）
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		// 查询失败，返回默认币种（USD）
		return model.GetDisplayCurrencyInfoByTimezone("")
	}
	// 根据用户的时区返回对应的币种信息
	// 例如 timezone 为 "Asia/Shanghai" 会返回 CNY，其他返回 USD
	return model.GetDisplayCurrencyInfoByTimezone(user.Timezone)
}

// convertQuotaToDisplay 将系统内部额度（int64 整数）转换为展示币种的金额（float64）
// 转换链路：内部额度 → ÷ QuotaPerUnit → 美元金额（6位小数）→ × 汇率 → 展示币种金额
// QuotaPerUnit 是系统定义的额度单位换算比，例如 500000 表示 1 美元 = 500000 内部额度
func convertQuotaToDisplay(quota int, info model.DisplayCurrencyInfo) float64 {
	// 防止除零错误
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	// 第一步：内部额度 ÷ QuotaPerUnit = 美元金额
	usd := decimal.NewFromInt(int64(quota)).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(6). // 保留 6 位小数精度
		InexactFloat64()
	// 第二步：美元金额 → 展示币种金额（CNY 需要乘以汇率）
	return convertUsdToDisplay(usd, info)
}

// convertUsdToDisplay 将美元金额转换为用户展示币种的金额
// USD 用户：直接保留 2 位小数返回
// CNY 用户：× 汇率后保留 6 位中间精度，最终保留 2 位小数返回
// 注意：先 Round(6) 再 Round(2) 是为了避免浮点运算在 Round(2) 时产生舍入误差
func convertUsdToDisplay(usdAmount float64, info model.DisplayCurrencyInfo) float64 {
	// 将输入的美元金额标准化为 6 位小数精度
	amount := normalizeDisplayMoneyDecimal(usdAmount)
	if info.Currency != "CNY" {
		// 非 CNY 币种（如 USD），直接保留 2 位小数
		return amount.Round(2).InexactFloat64()
	}
	// CNY 币种：美元金额 × 汇率 → 人民币金额
	return amount.
		Mul(normalizeDisplayMoneyDecimal(info.Rate)). // 乘以汇率（同样 6 位精度）
		Round(6).                                     // 先保留 6 位中间精度
		Round(2).                                     // 最终收敛到 2 位小数
		InexactFloat64()
}

// applyDisplayCurrencyInfo 将展示币种信息注入到 API 响应数据中
// 在返回给前端的数据中附带 display_currency（币种代码）、display_symbol（货币符号）、display_rate（汇率）
// 前端根据这些信息进行金额的展示格式化
func applyDisplayCurrencyInfo(data gin.H, info model.DisplayCurrencyInfo) gin.H {
	if data == nil {
		data = gin.H{}
	}
	// 注入展示币种代码，如 "USD" 或 "CNY"
	data["display_currency"] = info.Currency
	// 注入货币符号，如 "$" 或 "¥"
	data["display_symbol"] = info.Symbol
	// 注入汇率（相对 USD），如 USD=1, CNY=7.25
	data["display_rate"] = info.Rate
	return data
}
