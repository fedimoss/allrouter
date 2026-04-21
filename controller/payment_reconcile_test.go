package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func TestAmountStringMatchesMoney(t *testing.T) {
	if !amountStringMatchesMoney("12.30", 12.3) {
		t.Fatal("expected amount string to match local money")
	}
	if amountStringMatchesMoney("12.31", 12.3) {
		t.Fatal("expected mismatched amount string to be rejected")
	}
}

func TestMinorUnitAmountMatchesMoney(t *testing.T) {
	if !minorUnitAmountMatchesMoney(1230, "USD", 12.3) {
		t.Fatal("expected USD cent amount to match local money")
	}
	if !minorUnitAmountMatchesMoney(1200, "JPY", 1200) {
		t.Fatal("expected zero-decimal currency amount to match local money")
	}
}

func TestStripeAmountTotalMatchesMoney(t *testing.T) {
	if !stripeAmountTotalMatchesMoney("999", 9.99) {
		t.Fatal("expected Stripe amount_total to match local money")
	}
	if stripeAmountTotalMatchesMoney("1000", 9.99) {
		t.Fatal("expected mismatched Stripe amount_total to be rejected")
	}
}

func TestGetStripeExpectedPayMoneyFromTopUp(t *testing.T) {
	oldStripeUnitPrice := setting.StripeUnitPrice
	oldDiscount := operation_setting.GetPaymentSetting().AmountDiscount
	t.Cleanup(func() {
		setting.StripeUnitPrice = oldStripeUnitPrice
		operation_setting.GetPaymentSetting().AmountDiscount = oldDiscount
	})

	setting.StripeUnitPrice = 8
	operation_setting.GetPaymentSetting().AmountDiscount = map[int]float64{100: 0.9}

	topUp := &model.TopUp{
		Amount: 100,
		Money:  15,
	}
	if got := getStripeExpectedPayMoneyFromTopUp(topUp); got != 108 {
		t.Fatalf("unexpected expected pay money: got %.2f want 108.00", got)
	}
}

func TestGetChargedAmount_TokenDisplay(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	oldDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	t.Cleanup(func() {
		common.QuotaPerUnit = oldQuotaPerUnit
		operation_setting.GetGeneralSetting().QuotaDisplayType = oldDisplayType
	})

	common.QuotaPerUnit = 100
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens

	user := model.User{Group: "default"}
	if got := GetChargedAmount(500, user); got != 5 {
		t.Fatalf("unexpected charged amount under token display: got %.2f want 5.00", got)
	}
}
