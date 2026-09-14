package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
)

type SubscriptionWaffoPancakePayRequest struct {
	PlanId int `json:"plan_id"`
}

// normalizeWaffoPancakeCurrency converts the symbols persisted in local
// orders to the ISO code returned by Pancake webhooks.
func normalizeWaffoPancakeCurrency(currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	switch currency {
	case "$", "USD":
		return "USD"
	case "￥", "¥", "CNY":
		return "CNY"
	default:
		return currency
	}
}

// subscriptionWaffoPancakePaymentMatches validates the immutable payment
// snapshot on a subscription order against a verified Pancake completion
// event before the entitlement is activated.
func subscriptionWaffoPancakePaymentMatches(order *model.SubscriptionOrder, event *service.WaffoPancakeWebhookEvent) bool {
	if order == nil || event == nil {
		return false
	}
	// Pancake's current webhook schema omits productId.  Checkout metadata is
	// echoed as orderMetadata, so bind a new order to the exact product that was
	// selected when its checkout session was created.  Legacy orders may not
	// contain this snapshot and retain the amount/currency-only compatibility
	// check below.
	expectedProductID := strings.TrimSpace(order.PaymentProductId)
	if expectedProductID != "" {
		actualProductID := ""
		if event.Data.OrderMetadata != nil {
			for _, key := range []string{"new_api_product_id", "product_id", "productId"} {
				if value := strings.TrimSpace(event.Data.OrderMetadata[key]); value != "" {
					actualProductID = value
					break
				}
			}
		}
		if actualProductID == "" || actualProductID != expectedProductID {
			return false
		}
	}
	expectedAmount := order.OriginalMoney
	if expectedAmount <= 0 {
		expectedAmount = order.Money // legacy orders predating OriginalMoney
	}
	if expectedAmount <= 0 || !amountStringMatchesMoney(event.Data.Amount, expectedAmount) {
		return false
	}
	expectedCurrency := normalizeWaffoPancakeCurrency(order.Currency)
	if expectedCurrency == "" {
		expectedCurrency = "USD" // legacy Waffo subscription orders
	}
	actualCurrency := normalizeWaffoPancakeCurrency(event.Data.Currency)
	return actualCurrency != "" && actualCurrency == expectedCurrency
}

func SubscriptionRequestWaffoPancakePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req SubscriptionWaffoPancakePayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 统一购买前校验：套餐存在/启用/允许购买 + 是否对当前 provider_id 可见（防跨站点订阅）。
	if !ensureSubscriptionPlanPurchasable(c, plan) {
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	if plan.AllowPurchase != 1 {
		common.ApiErrorMsg(c, "该套餐暂不允许订阅")
		return
	}
	// Pancake checkout accepts a positive USD amount.  Keep the same lower
	// bound used by the other one-time payment paths so an accidentally zero
	// priced plan cannot create a pending order that can never be settled.
	if plan.PriceAmount < 0.01 {
		common.ApiErrorMsg(c, "套餐金额过低")
		return
	}
	if strings.TrimSpace(plan.WaffoPancakeProductId) == "" {
		common.ApiErrorMsg(c, "该套餐未配置 WaffoPancakeProductId")
		return
	}
	// Plan targets its own Pancake product, so we only require credentials
	// here — not the gateway-level WaffoPancakeProductID.
	if strings.TrimSpace(setting.WaffoPancakeMerchantID) == "" ||
		strings.TrimSpace(setting.WaffoPancakePrivateKey) == "" {
		common.ApiErrorMsg(c, "Waffo Pancake 未配置或密钥无效")
		return
	}

	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	// WAFFO_PANCAKE_SUB- prefix (vs. wallet's WAFFO_PANCAKE-) drives webhook
	// dispatch in WaffoPancakeWebhook.
	tradeNo := fmt.Sprintf("WAFFO_PANCAKE_SUB-%d-%d-%s", userId, time.Now().UnixMilli(), randstr.String(6))

	order := &model.SubscriptionOrder{
		UserId: userId,
		PlanId: plan.Id,
		// 订单归属服务商（0=主站），完成订单时据此给服务商 owner 结算订阅收入。
		ProviderId:       c.GetInt("provider_id"),
		Money:            plan.PriceAmount,
		Currency:         "$",
		OriginalMoney:    model.RoundDisplayCurrencyAmount(plan.PriceAmount),
		TradeNo:          tradeNo,
		PaymentMethod:    model.PaymentMethodWaffoPancake,
		PaymentProvider:  model.PaymentProviderWaffoPancake,
		PaymentProductId: plan.WaffoPancakeProductId,
		CreateTime:       time.Now().Unix(),
		Status:           common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Waffo Pancake 订阅订单创建失败 user_id=%d plan_id=%d trade_no=%s error=%q", userId, plan.Id, tradeNo, err.Error()))
		respondSubscriptionCreateError(c, err, "创建订单失败")
		return
	}

	expiresInSeconds := int(model.DefaultSubscriptionCheckoutSeconds)
	session, err := service.CreateWaffoPancakeCheckoutSession(c.Request.Context(), &service.WaffoPancakeCreateSessionParams{
		// Use the immutable values accepted by CreateSubscriptionOrderTx rather
		// than the cached plan object.  This keeps the remote checkout, callback
		// verifier, and entitlement snapshot on the same catalog version.
		ProductID:               order.PaymentProductId,
		BuyerIdentity:           service.WaffoPancakeBuyerIdentityFromUserID(user.Id),
		OrderMerchantExternalID: tradeNo,
		PriceSnapshot: &service.WaffoPancakePriceSnapshot{
			Amount:      decimal.NewFromFloat(order.OriginalMoney).StringFixed(2),
			TaxCategory: "saas",
		},
		BuyerEmail:       getWaffoPancakeBuyerEmail(user),
		ExpiresInSeconds: &expiresInSeconds,
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Waffo Pancake 订阅结账会话创建失败 user_id=%d plan_id=%d trade_no=%s error=%q", userId, plan.Id, tradeNo, err.Error()))
		_ = model.ExpireSubscriptionOrder(tradeNo, model.PaymentMethodWaffoPancake)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Waffo Pancake 订阅订单创建成功 user_id=%d plan_id=%d trade_no=%s session_id=%s money=%.2f", userId, plan.Id, tradeNo, session.SessionID, order.OriginalMoney))

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"checkout_url":     session.CheckoutURL,
			"session_id":       session.SessionID,
			"expires_at":       session.ExpiresAt,
			"order_id":         tradeNo,
			"token":            session.Token,
			"token_expires_at": session.TokenExpiresAt,
		},
	})
}
