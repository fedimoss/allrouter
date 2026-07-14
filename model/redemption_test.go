package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupRedemptionValueTest 固定 1 USD 对应 500000 内部额度，便于覆盖小数面值场景。
func setupRedemptionValueTest(t *testing.T) {
	t.Helper()
	truncateTables(t)

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})
}

func insertRedemptionForValueTest(t *testing.T, userID, quota int, key string) *Redemption {
	t.Helper()
	insertUserForPaymentGuardTest(t, userID, 0)

	redemption := &Redemption{
		Key:         key,
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "value-test",
		Quota:       quota,
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, redemption.Insert())
	return redemption
}

func getRedemptionTopUpForValueTest(t *testing.T, redemptionID int) *TopUp {
	t.Helper()
	var topUp TopUp
	require.NoError(t, DB.Where("biz_type = ? AND source_id = ?", TopUpBizTypeRedemption, redemptionID).First(&topUp).Error)
	return &topUp
}

// TestRedeemStoresExactQuotaAndUSDOriginalValue 验证手工兑换不会丢失 0.1 USD 的精度。
func TestRedeemStoresExactQuotaAndUSDOriginalValue(t *testing.T) {
	setupRedemptionValueTest(t)
	redemption := insertRedemptionForValueTest(t, 301, 50_000, "manual-usd-value")

	quota, err := Redeem(redemption.Key, 301)
	require.NoError(t, err)
	assert.Equal(t, 50_000, quota)

	topUp := getRedemptionTopUpForValueTest(t, redemption.Id)
	assert.Equal(t, int64(50_000), topUp.Amount)
	assert.InDelta(t, 0.1, topUp.Money, 0.000001)
	assert.InDelta(t, 0.1, topUp.OriginalMoney, 0.000001)
	assert.Equal(t, "USD", topUp.Currency)

	topUps, total, err := GetUserTopUps(301, &common.PageInfo{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, topUps, 1)
	assert.Equal(t, int64(50_000), topUps[0].Amount)
	assert.InDelta(t, 0.1, topUps[0].OriginalMoney, 0.000001)
	assert.Equal(t, "USD", topUps[0].Currency)
}

// TestRedeemStoresGiftOriginalValueAndCurrency 验证充值赠送同时保留归一化美元价值和人民币原始面值。
func TestRedeemStoresGiftOriginalValueAndCurrency(t *testing.T) {
	setupRedemptionValueTest(t)
	redemption := insertRedemptionForValueTest(t, 302, 41_667, "gift-cny-value")

	quota, err := redeemWithOriginalValue(redemption.Key, 302, redemptionOriginalValue{
		Amount:   0.6,
		Currency: "CNY",
	})
	require.NoError(t, err)
	assert.Equal(t, 41_667, quota)

	topUp := getRedemptionTopUpForValueTest(t, redemption.Id)
	assert.Equal(t, int64(41_667), topUp.Amount)
	assert.InDelta(t, 0.083334, topUp.Money, 0.000001)
	assert.InDelta(t, 0.6, topUp.OriginalMoney, 0.000001)
	assert.Equal(t, "CNY", topUp.Currency)
}
