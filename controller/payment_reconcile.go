package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

func normalizeMoneyPrecisionDecimal(value float64) decimal.Decimal {
	return decimal.NewFromFloat(value).Round(6)
}

// normalizeMoneyDecimal 将金额统一收敛到两位小数，避免不同支付网关的字符串/浮点格式差异导致误判。
func normalizeMoneyDecimal(value float64) decimal.Decimal {
	return normalizeMoneyPrecisionDecimal(value).Round(2)
}

// amountStringMatchesMoney 比较支付网关返回的"元/美元"等主单位金额字符串与本地订单金额是否一致。
func amountStringMatchesMoney(amount string, expected float64) bool {
	if strings.TrimSpace(amount) == "" {
		return false
	}
	dAmount, err := decimal.NewFromString(strings.TrimSpace(amount))
	if err != nil {
		return false
	}
	return dAmount.Round(2).Equal(normalizeMoneyDecimal(expected))
}

// epayCallbackMoneyMatches epay 回调金额是人民币，topUp.Money 存储美元等值金额，
// 需要先把回调的人民币换算为美元再比对。
func epayCallbackMoneyMatches(cnyAmount string, expectedUsd float64) bool {
	if strings.TrimSpace(cnyAmount) == "" {
		return false
	}
	dCny, err := decimal.NewFromString(strings.TrimSpace(cnyAmount))
	if err != nil || !dCny.IsPositive() {
		return false
	}
	cnyConfig, err := model.GetCurrencyConfig("CNY")
	if err != nil || cnyConfig == nil || cnyConfig.UnitPrice <= 0 {
		return false
	}
	dUsd := dCny.Div(decimal.NewFromFloat(cnyConfig.UnitPrice)).Round(6)
	return dUsd.Equal(normalizeMoneyPrecisionDecimal(expectedUsd))
}

// minorUnitAmountMatchesMoney 比较支付网关返回的"分/美分"等最小货币单位金额与本地订单金额是否一致。
func minorUnitAmountMatchesMoney(amount int, currency string, expected float64) bool {
	dAmount := decimal.NewFromInt(int64(amount))
	if !zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(currency))] {
		dAmount = dAmount.Div(decimal.NewFromInt(100))
	}
	return dAmount.Round(2).Equal(normalizeMoneyDecimal(expected))
}

// stripeAmountTotalMatchesMoney compares Stripe's amount_total (expressed in
// the currency's smallest unit) with a local major-unit amount.  USD/CNY and
// other two-decimal currencies are divided by 100; zero-decimal currencies
// such as JPY are compared directly.  The optional currency argument keeps
// the historical two-argument helper source-compatible with existing callers.
func stripeAmountTotalMatchesMoney(amountTotal string, expected float64, currencies ...string) bool {
	if strings.TrimSpace(amountTotal) == "" {
		return false
	}
	dAmount, err := decimal.NewFromString(strings.TrimSpace(amountTotal))
	if err != nil {
		return false
	}
	currency := ""
	if len(currencies) > 0 {
		currency = strings.ToUpper(strings.TrimSpace(currencies[0]))
	}
	if !zeroDecimalCurrencies[currency] {
		dAmount = dAmount.Div(decimal.NewFromInt(100))
	}
	return dAmount.Round(2).Equal(normalizeMoneyDecimal(expected))
}

// getStripeExpectedPayMoneyFromTopUp 根据本地 Stripe 充值订单反推出本次应收款金额。
// 说明：
// 1. topUp.Money 在 Stripe 订单里存的是"应发放的充值额度（已乘分组倍率）"，不是实际收款；
// 2. 实际收款 = 应发放额度 × Stripe 单价 × 当前充值档位折扣。
func getStripeExpectedPayMoneyFromTopUp(topUp *model.TopUp) float64 {
	if topUp == nil {
		return 0
	}
	// OriginalMoney is captured when the Checkout order is created and is the
	// amount actually quoted to the customer in the selected currency.  Use the
	// immutable snapshot first; recalculating from current exchange rates,
	// group ratios, or discount settings could reject a valid delayed webhook
	// after an administrator changes pricing.
	if topUp.OriginalMoney > 0 {
		return topUp.OriginalMoney
	}
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(topUp.Amount)]; ok && ds > 0 {
		discount = ds
	}
	// 根据订单所属用户的时区解析实际单价
	unitPrice := setting.StripeUnitPrice
	if topUp.UserId > 0 {
		if user, err := model.GetUserById(topUp.UserId, false); err == nil {
			unitPrice = resolveStripeUnitPrice(user)
		}
	}
	// 第一步：充值额度 × 单价 → 原始应收金额（6 位小数）
	subTotal := normalizeMoneyPrecisionDecimal(topUp.Money).
		Mul(normalizeMoneyPrecisionDecimal(unitPrice)).
		Round(6)
	// 第二步：原始金额 × 档位折扣 → 最终应收金额（2 位小数）
	return subTotal.
		Mul(normalizeMoneyPrecisionDecimal(discount)).
		Round(2).
		InexactFloat64()
}
