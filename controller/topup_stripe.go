package controller

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/thanhpk/randstr"
)

const (
	PaymentMethodStripe = "stripe"
)

var stripeAdaptor = &StripeAdaptor{}

// StripePayRequest represents a payment request for Stripe checkout.
type StripePayRequest struct {
	// Amount is the quantity of units to purchase.
	Amount int64 `json:"amount"`
	// PaymentMethod specifies the payment method (e.g., "stripe").
	PaymentMethod string `json:"payment_method"`
	// SuccessURL is the optional custom URL to redirect after successful payment.
	// If empty, defaults to the server's console topup page.
	SuccessURL string `json:"success_url,omitempty"`
	// CancelURL is the optional custom URL to redirect when payment is canceled.
	// If empty, defaults to the server's console topup page.
	CancelURL string `json:"cancel_url,omitempty"`
}

type StripeAdaptor struct {
}

func (*StripeAdaptor) RequestAmount(c *gin.Context, req *StripePayRequest) {
	if req.Amount < getStripeMinTopup() {
		c.JSON(200, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup())})
		return
	}
	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(200, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}

	// 根据用户时区解析实际单价
	user, _ := model.GetUserById(id, false)
	unitPrice := resolveStripeUnitPrice(user)

	// 根据单价、分组倍率、折扣计算实际应付金额
	payMoney := getStripePayMoney(float64(req.Amount), group, unitPrice)
	if payMoney <= 0.01 {
		c.JSON(200, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(200, gin.H{"message": "success", "data": strconv.FormatFloat(payMoney, 'f', 2, 64)})
}

func (*StripeAdaptor) RequestPay(c *gin.Context, req *StripePayRequest) {
	if req.PaymentMethod != PaymentMethodStripe {
		c.JSON(200, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}
	if req.Amount < getStripeMinTopup() {
		c.JSON(200, gin.H{"message": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup()), "data": 10})
		return
	}
	if req.Amount > 10000 {
		c.JSON(200, gin.H{"message": "充值数量不能大于 10000", "data": 10})
		return
	}

	if req.SuccessURL != "" && common.ValidateRedirectURL(req.SuccessURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付成功重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	if req.CancelURL != "" && common.ValidateRedirectURL(req.CancelURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付取消重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	id := c.GetInt("id")
	user, _ := model.GetUserById(id, false)
	// 查询用户失败时无法继续，直接返回错误
	if user == nil {
		c.JSON(200, gin.H{"message": "error", "data": "获取用户信息失败"})
		return
	}

	// 根据用户时区查找对应的 Stripe Price ID
	priceId := resolveStripePriceId(user)
	// 如果最终没有找到有效的 Price ID，拒绝发起支付
	if priceId == "" {
		c.JSON(200, gin.H{"message": "error", "data": "未找到对应币种的支付配置"})
		return
	}

	// Stripe 订单的 Money 字段存储"应发放的充值额度（已乘分组倍率）"，
	// 不是实际支付金额；实际支付金额由 Stripe Checkout/回调金额决定。
	chargedMoney := calcStripeChargedMoney(req.Amount, user)

	// 生成唯一的订单参考号，格式：new-api-ref-{用户ID}-{毫秒时间戳}-{4位随机字符串}
	reference := fmt.Sprintf("new-api-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "ref_" + common.Sha1([]byte(reference))

	// 调用 Stripe API 创建 Checkout Session，传入时区对应的 priceId
	payLink, err := genStripeLink(referenceId, user.StripeCustomer, user.Email, req.Amount, priceId, req.SuccessURL, req.CancelURL)
	if err != nil {
		log.Println("获取Stripe Checkout支付链接失败", err)
		c.JSON(200, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	topUp := &model.TopUp{
		UserId:        id,
		Amount:        req.Amount,
		Money:         chargedMoney,
		TradeNo:       referenceId,
		PaymentMethod: PaymentMethodStripe,
		BizType:       model.TopUpBizTypePayment,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	err = topUp.Insert()
	if err != nil {
		c.JSON(200, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}
	c.JSON(200, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": payLink,
		},
	})
}

func RequestStripeAmount(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(200, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestAmount(c, &req)
}

func RequestStripePay(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(200, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestPay(c, &req)
}

func StripeWebhook(c *gin.Context) {
	if setting.StripeWebhookSecret == "" {
		log.Println("Stripe Webhook Secret 未配置，拒绝处理")
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("解析Stripe Webhook参数失败: %v\n", err)
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	event, err := webhook.ConstructEventWithOptions(payload, signature, setting.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})

	if err != nil {
		log.Printf("Stripe Webhook验签失败: %v\n", err)
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	var handlerErr error
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		handlerErr = sessionCompleted(event)
	case stripe.EventTypeCheckoutSessionExpired:
		handlerErr = sessionExpired(event)
	case stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded:
		handlerErr = sessionAsyncPaymentSucceeded(event)
	case stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
		handlerErr = sessionAsyncPaymentFailed(event)
	default:
		log.Printf("不支持的Stripe Webhook事件类型: %s\n", event.Type)
	}
	if handlerErr != nil {
		log.Printf("Stripe Webhook处理失败: %v\n", handlerErr)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

func sessionCompleted(event stripe.Event) error {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	status := event.GetObjectValue("status")
	if "complete" != status {
		log.Println("错误的Stripe Checkout完成状态:", status, ",", referenceId)
		return nil
	}

	paymentStatus := event.GetObjectValue("payment_status")
	if paymentStatus != "paid" {
		log.Printf("Stripe Checkout 支付尚未完成，payment_status: %s, ref: %s（等待异步支付结果）", paymentStatus, referenceId)
		return nil
	}

	fulfillOrder(event, referenceId, customerId)
	return nil
}

// sessionAsyncPaymentSucceeded handles delayed payment methods (bank transfer, SEPA, etc.)
// that confirm payment after the checkout session completes.
func sessionAsyncPaymentSucceeded(event stripe.Event) error {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	log.Printf("Stripe 异步支付成功: %s", referenceId)

	fulfillOrder(event, referenceId, customerId)
	return nil
}

// sessionAsyncPaymentFailed marks orders as failed when delayed payment methods
// ultimately fail (e.g. bank transfer not received, SEPA rejected).
func sessionAsyncPaymentFailed(event stripe.Event) error {
	referenceId := event.GetObjectValue("client_reference_id")
	log.Printf("Stripe 异步支付失败: %s", referenceId)

	if len(referenceId) == 0 {
		log.Println("异步支付失败事件未提供支付单号")
		return nil
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)

	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		log.Println("异步支付失败，充值订单不存在:", referenceId)
		return nil
	}

	if topUp.PaymentMethod != PaymentMethodStripe {
		log.Printf("异步支付失败，订单支付方式不匹配: %s, ref: %s", topUp.PaymentMethod, referenceId)
		return nil
	}

	if topUp.Status != common.TopUpStatusPending {
		log.Printf("异步支付失败，订单状态非pending: %s, ref: %s", topUp.Status, referenceId)
		return nil
	}

	topUp.Status = common.TopUpStatusFailed
	if err := topUp.Update(); err != nil {
		log.Printf("标记充值订单失败出错: %v, ref: %s", err, referenceId)
		return nil
	}
	log.Printf("充值订单已标记为失败: %s", referenceId)
	return nil
}

// fulfillOrder is the shared logic for crediting quota after payment is confirmed.
func fulfillOrder(event stripe.Event, referenceId string, customerId string) {
	if len(referenceId) == 0 {
		log.Println("未提供支付单号")
		return
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	payload := map[string]any{
		"customer":     customerId,
		"amount_total": event.GetObjectValue("amount_total"),
		"currency":     strings.ToUpper(event.GetObjectValue("currency")),
		"event_type":   string(event.Type),
	}

	// 先尝试按"订阅订单"处理；订阅单金额固定，允许做严格金额校验。
	if subscriptionOrder := model.GetSubscriptionOrderByTradeNo(referenceId); subscriptionOrder != nil {
		if !stripeAmountTotalMatchesMoney(event.GetObjectValue("amount_total"), subscriptionOrder.Money) {
			log.Printf("Stripe 订阅金额校验失败: ref=%s, callback_amount_total=%s, local_money=%.2f", referenceId, event.GetObjectValue("amount_total"), subscriptionOrder.Money)
			return
		}
		if err := model.CompleteSubscriptionOrder(referenceId, common.GetJsonString(payload), PaymentMethodStripe); err == nil {
			return
		} else if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
			log.Println("complete subscription order failed:", err.Error(), referenceId)
			return
		}
	}

	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		log.Println("充值订单不存在", referenceId)
		return
	}
	if topUp.PaymentMethod != PaymentMethodStripe {
		log.Printf("Stripe 充值订单支付方式不匹配: %s, ref: %s", topUp.PaymentMethod, referenceId)
		return
	}

	// Stripe 充值支持可选促销码。未开启促销码时，严格校验回调金额必须与本地订单一致；
	// 开启促销码后，实际支付金额可能低于标价，此处不做强校验，避免误伤合法优惠订单。
	if !setting.StripePromotionCodesEnabled {
		expectedPayMoney := getStripeExpectedPayMoneyFromTopUp(topUp)
		if !stripeAmountTotalMatchesMoney(event.GetObjectValue("amount_total"), expectedPayMoney) {
			log.Printf("Stripe 充值金额校验失败: ref=%s, callback_amount_total=%s, expected_pay_money=%.2f", referenceId, event.GetObjectValue("amount_total"), expectedPayMoney)
			return
		}
	}

	err := model.Recharge(referenceId, customerId)
	if err != nil {
		log.Println(err.Error(), referenceId)
		return
	}

	total, _ := strconv.ParseFloat(event.GetObjectValue("amount_total"), 64)
	currency := strings.ToUpper(event.GetObjectValue("currency"))
	log.Printf("收到款项：%s, %.2f(%s)", referenceId, total/100, currency)
}

func sessionExpired(event stripe.Event) error {
	referenceId := event.GetObjectValue("client_reference_id")
	status := event.GetObjectValue("status")
	if "expired" != status {
		log.Println("错误的Stripe Checkout过期状态:", status, ",", referenceId)
		return nil
	}

	if len(referenceId) == 0 {
		log.Println("未提供支付单号")
		return nil
	}

	// Subscription order expiration
	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	if err := model.ExpireSubscriptionOrder(referenceId, PaymentMethodStripe); err == nil {
		return nil
	} else if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		log.Println("过期订阅订单失败", referenceId, ", err:", err.Error())
		return nil
	}

	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		log.Println("充值订单不存在", referenceId)
		return nil
	}

	if topUp.Status == common.TopUpStatusExpired {
		log.Println("充值订单已是过期状态", referenceId)
		return nil
	}
	if topUp.Status != common.TopUpStatusPending {
		log.Println("充值订单状态错误", referenceId)
		return nil
	}

	topUp.Status = common.TopUpStatusExpired
	err := topUp.Update()
	if err != nil {
		log.Println("过期充值订单失败", referenceId, ", err:", err.Error())
		return nil
	}

	log.Println("充值订单已过期", referenceId)
	return nil
}

func resolveStripeRedirectURLs(successURL string, cancelURL string) (string, string) {
	if successURL == "" {
		successURL = system_setting.ServerAddress + "/console/topup"
	}
	if cancelURL == "" {
		cancelURL = system_setting.ServerAddress + "/console/topup"
	}
	return successURL, cancelURL
}

// genStripeLink generates a Stripe Checkout session URL for payment.
// It creates a new checkout session with the specified parameters and returns the payment URL.
//
// Parameters:
//   - referenceId: unique reference identifier for the transaction
//   - customerId: existing Stripe customer ID (empty string if new customer)
//   - email: customer email address for new customer creation
//   - amount: quantity of units to purchase
//   - successURL: custom URL to redirect after successful payment (empty for default)
//   - cancelURL: custom URL to redirect when payment is canceled (empty for default)
//
// Returns the checkout session URL or an error if the session creation fails.
func genStripeLink(referenceId string, customerId string, email string, amount int64, priceId string, successURL string, cancelURL string) (string, error) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return "", fmt.Errorf("无效的Stripe API密钥")
	}

	stripe.Key = setting.StripeApiSecret

	// Use custom URLs if provided, otherwise use defaults.
	successURL, cancelURL = resolveStripeRedirectURLs(successURL, cancelURL)

	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceId),
				Quantity: stripe.Int64(amount),
			},
		},
		Mode:                stripe.String(string(stripe.CheckoutSessionModePayment)),
		AllowPromotionCodes: stripe.Bool(setting.StripePromotionCodesEnabled),
	}

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}

		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := session.New(params)
	if err != nil {
		return "", err
	}

	return result.URL, nil
}

func GetChargedAmount(count float64, user model.User) float64 {
	// Token 展示模式下，前端传入的是 token 数量，需要先折算回基础充值额度，
	// 否则后续 Recharge 会再次乘 QuotaPerUnit，导致额度被重复放大。
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		count = count / common.QuotaPerUnit
	}
	topUpGroupRatio := common.GetTopupGroupRatio(user.Group)
	if topUpGroupRatio == 0 {
		topUpGroupRatio = 1
	}

	return count * topUpGroupRatio
}

// calcStripeChargedMoney 计算Stripe充值订单存储到 topUp.Money 的美元等值金额
// USD 用户：充值数量 × 分组倍率（无需换算）
// CNY 用户：充值数量 × 分组倍率 ÷ unitPrice（换算为美元），保留 6 位小数
func calcStripeChargedMoney(amount int64, user *model.User) float64 {
	if user == nil {
		return 0
	}

	chargedMoney := decimal.NewFromFloat(GetChargedAmount(float64(amount), *user))
	if model.GetDisplayCurrencyInfoByTimezone(user.Timezone).Currency == "CNY" {
		unitPrice := resolveStripeUnitPrice(user)
		if unitPrice > 0 {
			chargedMoney = chargedMoney.Div(decimal.NewFromFloat(unitPrice))
		}
	}

	return chargedMoney.Round(6).InexactFloat64()
}

// getStripePayMoney 计算用户实际应付金额
// 参数：amount 充值数量，group 用户分组，unitPrice 单价（根据时区解析）
// 计算：充值数量 × 单价 × 分组倍率 × 档位折扣
func getStripePayMoney(amount float64, group string, unitPrice float64) float64 {
	// 保留原始数量，用于匹配档位折扣
	originalAmount := amount
	// Token 展示模式下，前端传入的是 token 数量，需折算回基础充值额度
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = amount / common.QuotaPerUnit
	}
	// Using float64 for monetary calculations is acceptable here due to the small amounts involved
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	// 查找该充值数量对应的档位折扣，无配置则默认不打折
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(originalAmount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	payMoney := amount * unitPrice * topupGroupRatio * discount
	return payMoney
}

// resolveStripeCurrencyConfig 根据用户时区解析 Stripe 币种配置
// 优先使用时区映射，回退到全局 StripePriceId 对应的币种配置，找不到返回 nil
func resolveStripeCurrencyConfig(user *model.User) *model.CurrencyStripeConfig {
	// 优先根据用户时区匹配
	if user != nil && user.Timezone != "" {
		if config, _ := model.GetStripeConfigByTimezone(user.Timezone, ""); config != nil && config.StripePriceID != "" {
			return config
		}
	}
	// 回退：使用全局 StripePriceId 对应的币种（兼容未配置映射的场景）
	if setting.StripePriceId != "" {
		configs, _ := model.GetEnabledCurrencyConfigs()
		for _, cfg := range configs {
			if cfg.StripePriceID == setting.StripePriceId {
				return &cfg
			}
		}
	}
	return nil
}

// resolveStripeUnitPrice 根据用户时区解析 Stripe 单价
func resolveStripeUnitPrice(user *model.User) float64 {
	if config := resolveStripeCurrencyConfig(user); config != nil && config.UnitPrice > 0 {
		return config.UnitPrice
	}
	return setting.StripeUnitPrice
}

// resolveStripePriceId 根据用户时区解析 Stripe Price ID
func resolveStripePriceId(user *model.User) string {
	if config := resolveStripeCurrencyConfig(user); config != nil {
		return config.StripePriceID
	}
	return setting.StripePriceId
}

func getStripeMinTopup() int64 {
	minTopup := setting.StripeMinTopUp
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		minTopup = minTopup * int(common.QuotaPerUnit)
	}
	return int64(minTopup)
}
