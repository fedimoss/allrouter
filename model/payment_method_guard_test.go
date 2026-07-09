package model

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertUserForPaymentGuardTest(t *testing.T, id int, quota int) {
	t.Helper()
	user := &User{
		Id:       id,
		Username: "payment_guard_user_" + strconv.Itoa(id),
		Status:   common.UserStatusEnabled,
		Quota:    quota,
		AffCode:  "payment_guard_aff_" + strconv.Itoa(id),
	}
	require.NoError(t, DB.Create(user).Error)
}

func insertSubscriptionPlanForPaymentGuardTest(t *testing.T, id int) *SubscriptionPlan {
	t.Helper()
	plan := &SubscriptionPlan{
		Id:            id,
		Title:         "Guard Plan",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   1000,
	}
	require.NoError(t, DB.Create(plan).Error)
	return plan
}

func insertSubscriptionOrderForPaymentGuardTest(t *testing.T, tradeNo string, userID int, planID int, paymentProvider string) {
	t.Helper()
	order := &SubscriptionOrder{
		UserId:          userID,
		PlanId:          planID,
		Money:           9.99,
		TradeNo:         tradeNo,
		PaymentMethod:   paymentProvider,
		PaymentProvider: paymentProvider,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, order.Insert())
}

func insertTopUpForPaymentGuardTest(t *testing.T, tradeNo string, userID int, paymentProvider string) {
	t.Helper()
	topUp := &TopUp{
		UserId:          userID,
		Amount:          2,
		Money:           9.99,
		TradeNo:         tradeNo,
		PaymentMethod:   paymentProvider,
		PaymentProvider: paymentProvider,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())
}

func getTopUpStatusForPaymentGuardTest(t *testing.T, tradeNo string) string {
	t.Helper()
	topUp := GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, topUp)
	return topUp.Status
}

func countUserSubscriptionsForPaymentGuardTest(t *testing.T, userID int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	return count
}

func getUserQuotaForPaymentGuardTest(t *testing.T, userID int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("quota").Where("id = ?", userID).First(&user).Error)
	return user.Quota
}

func countTopUpsByTradeNoForPaymentGuardTest(t *testing.T, tradeNo string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, DB.Model(&TopUp{}).Where("trade_no = ?", tradeNo).Count(&count).Error)
	return count
}

func TestCreateSubscriptionOrderWithTopUp_CreatesPendingSubscriptionRecord(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 120, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 220)
	order := &SubscriptionOrder{
		UserId:          120,
		PlanId:          plan.Id,
		Money:           9.99,
		Currency:        "CNY",
		OriginalMoney:   73.00,
		TradeNo:         "sub-pending-record",
		PaymentMethod:   PaymentProviderLakala,
		PaymentProvider: PaymentProviderLakala,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}

	require.NoError(t, CreateSubscriptionOrderWithTopUp(order))

	topUp := GetTopUpByTradeNo(order.TradeNo)
	require.NotNil(t, topUp)
	assert.Equal(t, TopUpBizTypeSubscription, topUp.BizType)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
	assert.Equal(t, PaymentProviderLakala, topUp.PaymentProvider)
	assert.Equal(t, order.Id, topUp.SourceID)
	assert.InDelta(t, order.OriginalMoney, topUp.OriginalMoney, 0.001)
	assert.Zero(t, topUp.CompleteTime)
}

func TestCompleteSubscriptionOrder_UpdatesPendingSubscriptionRecord(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 121, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 221)
	order := &SubscriptionOrder{
		UserId:          121,
		PlanId:          plan.Id,
		Money:           9.99,
		Currency:        "CNY",
		OriginalMoney:   73.00,
		TradeNo:         "sub-complete-record",
		PaymentMethod:   PaymentProviderLakala,
		PaymentProvider: PaymentProviderLakala,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, CreateSubscriptionOrderWithTopUp(order))
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, order.TradeNo))

	require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, `{"provider":"lakala"}`, PaymentProviderLakala, PaymentProviderLakala))

	topUp := GetTopUpByTradeNo(order.TradeNo)
	require.NotNil(t, topUp)
	assert.Equal(t, TopUpBizTypeSubscription, topUp.BizType)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.NotZero(t, topUp.CompleteTime)
	assert.Equal(t, int64(1), countUserSubscriptionsForPaymentGuardTest(t, 121))
}

func TestCompleteSubscriptionOrder_CreditsProviderSubscriptionIncomeOnce(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 130, 0)
	insertUserForPaymentGuardTest(t, 131, 0)
	provider := &Provider{
		Id:          530,
		OwnerUserId: 130,
		Name:        "Provider Income Guard",
		Status:      ProviderStatusEnabled,
	}
	require.NoError(t, DB.Create(provider).Error)
	plan := &SubscriptionPlan{
		Id:            230,
		ProviderId:    provider.Id,
		Title:         "Provider Plan",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   1000,
	}
	require.NoError(t, DB.Create(plan).Error)
	order := &SubscriptionOrder{
		UserId:          131,
		PlanId:          plan.Id,
		ProviderId:      provider.Id,
		Money:           9.99,
		Currency:        "USD",
		OriginalMoney:   9.99,
		TradeNo:         "sub-provider-income",
		PaymentMethod:   PaymentProviderLakala,
		PaymentProvider: PaymentProviderLakala,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, CreateSubscriptionOrderWithTopUp(order))

	require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, `{"provider":"lakala"}`, PaymentProviderLakala, PaymentProviderLakala))
	require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, `{"provider":"lakala"}`, PaymentProviderLakala, PaymentProviderLakala))

	expectedQuota := int(decimal.NewFromFloat(order.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, provider.OwnerUserId))
	assert.Equal(t, int64(1), countUserSubscriptionsForPaymentGuardTest(t, order.UserId))
	assert.Equal(t, int64(1), countTopUpsByTradeNoForPaymentGuardTest(t, "PROVIDER-SUBSCRIPTION-"+strconv.Itoa(order.Id)))
}

func insertWechatTopUpForPaymentGuardTest(t *testing.T, tradeNo string, userID int, paymentProvider string) {
	t.Helper()
	insertPaymentTopUpForPaymentGuardTest(t, tradeNo, userID, "wxpay", paymentProvider)
}

func insertPaymentTopUpForPaymentGuardTest(t *testing.T, tradeNo string, userID int, paymentMethod string, paymentProvider string) {
	t.Helper()
	insertPaymentTopUpWithMoneyForPaymentGuardTest(t, tradeNo, userID, paymentMethod, paymentProvider, 2, 2)
}

func insertPaymentTopUpWithMoneyForPaymentGuardTest(t *testing.T, tradeNo string, userID int, paymentMethod string, paymentProvider string, amount int64, money float64) {
	t.Helper()
	topUp := &TopUp{
		UserId:          userID,
		Amount:          amount,
		Money:           money,
		TradeNo:         tradeNo,
		PaymentMethod:   paymentMethod,
		PaymentProvider: paymentProvider,
		BizType:         TopUpBizTypePayment,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, topUp.Insert())
}

func TestRechargeWaffoPancake_RejectsMismatchedPaymentMethod(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 101, 0)
	insertTopUpForPaymentGuardTest(t, "waffo-pancake-guard", 101, PaymentProviderStripe)

	err := RechargeWaffoPancake("waffo-pancake-guard")
	require.Error(t, err)

	topUp := GetTopUpByTradeNo("waffo-pancake-guard")
	require.NotNil(t, topUp)
	assert.Equal(t, common.TopUpStatusPending, topUp.Status)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 101))
}

func TestRechargeWechatTopUp_RejectsMismatchedPaymentProvider(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 110, 0)
	insertWechatTopUpForPaymentGuardTest(t, "wechat-provider-guard", 110, PaymentProviderEpay)

	err := RechargeWechatTopUp("wechat-provider-guard", "wxpay", PaymentProviderLakala)
	require.ErrorIs(t, err, ErrPaymentMethodMismatch)
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "wechat-provider-guard"))
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 110))
}

func TestRechargeWechatTopUp_CompletesLakalaOrderOnce(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 111, 0)
	insertPaymentTopUpForPaymentGuardTest(t, "lakala-once", 111, PaymentProviderLakala, PaymentProviderLakala)

	require.NoError(t, RechargeWechatTopUp("lakala-once", PaymentProviderLakala, PaymentProviderLakala))
	require.NoError(t, RechargeWechatTopUp("lakala-once", PaymentProviderLakala, PaymentProviderLakala))

	assert.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, "lakala-once"))
	assert.Equal(t, int(2*common.QuotaPerUnit), getUserQuotaForPaymentGuardTest(t, 111))
}

func TestRechargeWechatTopUp_AllowsLegacyEpayOrderWithoutProvider(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 112, 0)
	insertWechatTopUpForPaymentGuardTest(t, "legacy-epay-provider", 112, "")

	require.NoError(t, RechargeWechatTopUp("legacy-epay-provider", "wxpay", PaymentProviderEpay))
	assert.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, "legacy-epay-provider"))
	assert.Equal(t, int(2*common.QuotaPerUnit), getUserQuotaForPaymentGuardTest(t, 112))
}

func TestRechargeWechatTopUp_UsesUsdEquivalentMoneyForCNYGateways(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 113, 0)
	money := decimal.NewFromInt(1).Div(decimal.NewFromFloat(7.3)).InexactFloat64()
	insertPaymentTopUpWithMoneyForPaymentGuardTest(t, "cny-epay-money", 113, "wxpay", PaymentProviderEpay, 1, money)

	require.NoError(t, RechargeWechatTopUp("cny-epay-money", "wxpay", PaymentProviderEpay))

	expectedQuota := decimal.NewFromFloat(money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
	assert.Equal(t, int(expectedQuota), getUserQuotaForPaymentGuardTest(t, 113))
}

func TestUpdatePendingTopUpStatus_RejectsMismatchedPaymentProvider(t *testing.T) {
	testCases := []struct {
		name                    string
		tradeNo                 string
		storedPaymentProvider   string
		expectedPaymentProvider string
		targetStatus            string
	}{
		{
			name:                    "stripe expire",
			tradeNo:                 "stripe-expire-guard",
			storedPaymentProvider:   PaymentProviderCreem,
			expectedPaymentProvider: PaymentProviderStripe,
			targetStatus:            common.TopUpStatusExpired,
		},
		{
			name:                    "waffo failed",
			tradeNo:                 "waffo-failed-guard",
			storedPaymentProvider:   PaymentProviderStripe,
			expectedPaymentProvider: PaymentProviderWaffo,
			targetStatus:            common.TopUpStatusFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)
			insertUserForPaymentGuardTest(t, 150, 0)
			insertTopUpForPaymentGuardTest(t, tc.tradeNo, 150, tc.storedPaymentProvider)

			err := UpdatePendingTopUpStatus(tc.tradeNo, tc.expectedPaymentProvider, tc.targetStatus)
			require.ErrorIs(t, err, ErrPaymentMethodMismatch)
			assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, tc.tradeNo))
		})
	}
}

func TestCompleteSubscriptionOrder_RejectsMismatchedPaymentProvider(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 202, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 301)
	insertSubscriptionOrderForPaymentGuardTest(t, "sub-guard-order", 202, plan.Id, PaymentProviderStripe)

	err := CompleteSubscriptionOrder("sub-guard-order", `{"provider":"epay"}`, PaymentProviderEpay, "alipay")
	require.ErrorIs(t, err, ErrPaymentMethodMismatch)

	order := GetSubscriptionOrderByTradeNo("sub-guard-order")
	require.NotNil(t, order)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
	assert.Zero(t, countUserSubscriptionsForPaymentGuardTest(t, 202))

	topUp := GetTopUpByTradeNo("sub-guard-order")
	assert.Nil(t, topUp)
}

func TestExpireSubscriptionOrder_RejectsMismatchedPaymentProvider(t *testing.T) {
	truncateTables(t)

	insertUserForPaymentGuardTest(t, 303, 0)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 401)
	insertSubscriptionOrderForPaymentGuardTest(t, "sub-expire-guard", 303, plan.Id, PaymentProviderStripe)

	err := ExpireSubscriptionOrder("sub-expire-guard", PaymentProviderCreem)
	require.ErrorIs(t, err, ErrPaymentMethodMismatch)

	order := GetSubscriptionOrderByTradeNo("sub-expire-guard")
	require.NotNil(t, order)
	assert.Equal(t, common.TopUpStatusPending, order.Status)
}
