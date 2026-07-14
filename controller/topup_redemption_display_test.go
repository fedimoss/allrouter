package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyTopUpDisplayCurrencyPreservesRedemptionOriginalValue(t *testing.T) {
	topUp := &model.TopUp{
		BizType:       model.TopUpBizTypeRedemption,
		Amount:        41_667,
		Money:         0.083334,
		OriginalMoney: 0.6,
		Currency:      "CNY",
	}

	applyTopUpDisplayCurrency([]*model.TopUp{topUp}, model.DisplayCurrencyInfo{
		Currency: "USD",
		Symbol:   "$",
		Rate:     1,
	})

	require.NotNil(t, topUp.DisplayAmount)
	assert.InDelta(t, 0.6, *topUp.DisplayAmount, 0.000001)
	assert.InDelta(t, 0.6, topUp.Money, 0.000001)
	assert.Equal(t, "CNY", topUp.DisplayCurrency)
	assert.Equal(t, "¥", topUp.DisplaySymbol)
}

func TestApplyTopUpDisplayCurrencyUsesUSDForManualRedemption(t *testing.T) {
	topUp := &model.TopUp{
		BizType:       model.TopUpBizTypeRedemption,
		Amount:        250_000,
		Money:         0.5,
		OriginalMoney: 0.5,
		Currency:      "USD",
	}

	applyTopUpDisplayCurrency([]*model.TopUp{topUp}, model.DisplayCurrencyInfo{
		Currency: "CNY",
		Symbol:   "¥",
		Rate:     7.2,
	})

	require.NotNil(t, topUp.DisplayAmount)
	assert.InDelta(t, 0.5, *topUp.DisplayAmount, 0.000001)
	assert.InDelta(t, 0.5, topUp.Money, 0.000001)
	assert.Equal(t, "USD", topUp.DisplayCurrency)
	assert.Equal(t, "$", topUp.DisplaySymbol)
}

func TestApplyTopUpDisplayCurrencyDoesNotReinterpretLegacyRedemption(t *testing.T) {
	topUp := &model.TopUp{
		BizType: model.TopUpBizTypeRedemption,
		Amount:  0,
		Money:   0,
	}

	applyTopUpDisplayCurrency([]*model.TopUp{topUp}, model.DisplayCurrencyInfo{
		Currency: "CNY",
		Symbol:   "¥",
		Rate:     7.2,
	})

	assert.Nil(t, topUp.DisplayAmount)
	assert.Zero(t, topUp.Money)
	assert.Equal(t, "CNY", topUp.DisplayCurrency)
	assert.Equal(t, "¥", topUp.DisplaySymbol)
}
