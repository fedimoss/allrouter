package controller

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func itoa64(v int64) string { return strconv.FormatInt(v, 10) }

// TestCalcStripeUnitAmountAndTotal 验证面值定价（1 数量 = 1 元用户币种，
// 支付环节不做汇率换算）下"单价×数量=回调总额"不变式：
// stripeAmountTotalMatchesMoney(amount_total, expectedPay) 必须恒为 true，
// 否则 webhook 金额校验会拒绝合法支付（线上 bug 的直接成因）。
func TestCalcStripeUnitAmountAndTotal(t *testing.T) {
	cases := []struct {
		name     string
		amount   float64
		group    string
		currency string
		wantUnit int64
		wantPay  float64
	}{
		{
			// CNY 面值：充 4 付 ¥4.00（数量×1，不乘汇率 6.82）
			name: "cny 4 units face value", amount: 4, group: "default",
			currency: "cny",
			wantUnit: 100, wantPay: 4.00,
		},
		{
			// USD 面值：充 10 付 $10.00
			name: "usd 10 units face value", amount: 10, group: "default",
			currency: "usd",
			wantUnit: 100, wantPay: 10.00,
		},
		{
			// 不能整除时向上取整：50×0.9折=45→4500分/50=90 整除无进位；
			// 改用 3×1.5倍率=4.5→450分/3=150 整除；再改 7×1.2=8.4→840/7=120 整除；
			// 用 3×0.35折=1.05→105分/3=35 整除——改用不可整除例：3 数量无折扣
			// 倍率 0.7 → 2.1 → 210 分 / 3 = 70 整除。真正的不可整除：数量 3，倍率 1 → 3 元 → 300/3=100 整除。
			// 构造：数量 3，档位折扣不存在但分组倍率 1.7 → 5.1 → 510/3=170 整除。
			// 最简不可整除：数量 3 × 倍率 0.333 → 0.999 → 100 分（四舍五入）/3 = 33 余 1 → 34 分/个 → 102 分
			name: "indivisible rounds up", amount: 3, group: "odd",
			currency: "usd",
			wantUnit: 34, wantPay: 1.02,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldRatio := common.TopupGroupRatio2JSONString()
			t.Cleanup(func() { _ = common.UpdateTopupGroupRatioByJSONString(oldRatio) })
			if tc.group == "odd" {
				if err := common.UpdateTopupGroupRatioByJSONString(`{"odd":0.333}`); err != nil {
					t.Fatalf("set group ratio: %v", err)
				}
			}
			unitAmount, pay := calcStripeUnitAmountAndTotal(tc.amount, tc.group, tc.currency)
			if unitAmount != tc.wantUnit {
				t.Fatalf("unitAmount = %d, want %d", unitAmount, tc.wantUnit)
			}
			if pay != tc.wantPay {
				t.Fatalf("pay = %.2f, want %.2f", pay, tc.wantPay)
			}
			// 核心不变式：Stripe 回调 amount_total（unit_amount×quantity）必须与本地快照一致
			amountTotal := unitAmount * int64(tc.amount)
			if !stripeAmountTotalMatchesMoney(itoa64(amountTotal), pay, tc.currency) {
				t.Fatalf("invariant broken: amount_total=%d vs pay=%.2f", amountTotal, pay)
			}
		})
	}
}

// TestCalcStripeUnitAmountAndTotalWithDiscount 验证档位折扣参与面值定价。
func TestCalcStripeUnitAmountAndTotalWithDiscount(t *testing.T) {
	oldDiscount := operation_setting.GetPaymentSetting().AmountDiscount
	oldRatio := common.TopupGroupRatio2JSONString()
	t.Cleanup(func() {
		operation_setting.GetPaymentSetting().AmountDiscount = oldDiscount
		_ = common.UpdateTopupGroupRatioByJSONString(oldRatio)
	})

	// 充 100 打 9 折，分组倍率 1.2：100×1×1.2×0.9=108 → 单价 108 分
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{100: 0.9}
	if err := common.UpdateTopupGroupRatioByJSONString(`{"vip":1.2}`); err != nil {
		t.Fatalf("update group ratio: %v", err)
	}

	unitAmount, pay := calcStripeUnitAmountAndTotal(100, "vip", "usd")
	if unitAmount != 108 {
		t.Fatalf("unitAmount = %d, want 108", unitAmount)
	}
	if pay != 108.00 {
		t.Fatalf("pay = %.2f, want 108.00", pay)
	}
	if !stripeAmountTotalMatchesMoney("10800", pay, "usd") {
		t.Fatalf("invariant broken: amount_total=10800 vs pay=%.2f", pay)
	}
}

// TestGetStripeExpectedPayMoneyFromTopUpUsesSnapshot 回归测试：回调校验必须
// 优先使用下单快照 OriginalMoney，而不是按当前汇率重算（历史 bug：改价后
// 延迟 webhook 被拒）。
func TestGetStripeExpectedPayMoneyFromTopUpUsesSnapshot(t *testing.T) {
	topUp := &model.TopUp{Amount: 50, Money: 7.331378, OriginalMoney: 341.00}
	if got := getStripeExpectedPayMoneyFromTopUp(topUp); got != 341.00 {
		t.Fatalf("expected snapshot money 341.00, got %.2f", got)
	}
}
