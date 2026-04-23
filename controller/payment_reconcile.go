package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
)

// normalizeMoneyDecimal 将金额统一收敛到两位小数，避免不同支付网关的字符串/浮点格式差异导致误判。
func normalizeMoneyDecimal(value float64) decimal.Decimal {
	return decimal.NewFromFloat(value).Round(2)
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

// minorUnitAmountMatchesMoney 比较支付网关返回的"分/美分"等最小货币单位金额与本地订单金额是否一致。
func minorUnitAmountMatchesMoney(amount int, currency string, expected float64) bool {
	dAmount := decimal.NewFromInt(int64(amount))
	if !zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(currency))] {
		dAmount = dAmount.Div(decimal.NewFromInt(100))
	}
	return dAmount.Round(2).Equal(normalizeMoneyDecimal(expected))
}

// stripeAmountTotalMatchesMoney Stripe webhook 的 amount_total 使用最小货币单位，需要先折算再对账。
func stripeAmountTotalMatchesMoney(amountTotal string, expected float64) bool {
	if strings.TrimSpace(amountTotal) == "" {
		return false
	}
	dAmount, err := decimal.NewFromString(strings.TrimSpace(amountTotal))
	if err != nil {
		return false
	}
	return dAmount.Div(decimal.NewFromInt(100)).Round(2).Equal(normalizeMoneyDecimal(expected))
}

// getStripeExpectedPayMoneyFromTopUp 根据本地 Stripe 充值订单反推出本次应收款金额。
// 说明：
// 1. topUp.Money 在 Stripe 订单里存的是"应发放的充值额度（已乘分组倍率）"，不是实际收款；
// 2. 实际收款 = 应发放额度 × Stripe 单价 × 当前充值档位折扣。
func getStripeExpectedPayMoneyFromTopUp(topUp *model.TopUp) float64 {
	if topUp == nil {
		return 0
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
	return normalizeMoneyDecimal(topUp.Money).Mul(decimal.NewFromFloat(unitPrice)).Mul(decimal.NewFromFloat(discount)).Round(2).InexactFloat64()
}
