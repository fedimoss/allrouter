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
	stripeprice "github.com/stripe/stripe-go/v81/price"
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
	if !operation_setting.ContainsPayMethod(PaymentMethodStripe) {
		c.JSON(200, gin.H{"message": "error", "data": "管理员未开启Stripe充值"})
		return
	}
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

	// 根据用户时区解析币种（面值定价：1 数量 = 1 元用户币种，无需单价）
	user, _ := model.GetUserById(id, false)
	currency := resolveStripeCurrencyCode(user)

	// 与下单动态定价（price_data unit_amount × quantity）使用同一算式，
	// 保证报价展示与实际收款一致
	_, payMoney := calcStripeUnitAmountAndTotal(float64(req.Amount), group, currency)
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
	if !operation_setting.ContainsPayMethod(PaymentMethodStripe) {
		c.JSON(200, gin.H{"message": "error", "data": "管理员未开启Stripe充值"})
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

	trustedDomains := getStripeTrustedDomains(c)

	if req.SuccessURL != "" && common.ValidateRedirectURLWithDomains(req.SuccessURL, trustedDomains) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付成功重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	if req.CancelURL != "" && common.ValidateRedirectURLWithDomains(req.CancelURL, trustedDomains) != nil {
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
	// 解析币种符号
	stripeCurrency := "$"
	if config := resolveStripeCurrencyConfig(user); config != nil && config.Symbol != "" {
		stripeCurrency = config.Symbol
	}

	// Stripe 订单的 Money 字段存储"应发放的充值额度（已乘分组倍率）"，
	// 不是实际支付金额。OriginalMoney 必须保存本次 Checkout 实际报价，
	// 供异步 webhook 做不可变金额校验；不要使用通用人民币支付的 Price
	// 设置（它可能与 StripeUnitPrice 不同）。
	// genStripeLink 使用 price_data 按 unitAmount*quantity 动态定价，
	// 因此 OriginalMoney 必须等于 unitAmount×quantity 的精确值，否则回调
	// 金额校验必然失败。
	stripeCurrencyCode := resolveStripeCurrencyCode(user)
	chargedMoney := calcStripeChargedMoney(req.Amount, user)
	unitAmount, expectedPay := calcStripeUnitAmountAndTotal(float64(req.Amount), user.Group, stripeCurrencyCode)
	stripeOriginalMoney := expectedPay

	// 生成唯一的订单参考号，格式：new-api-ref-{用户ID}-{毫秒时间戳}-{4位随机字符串}
	reference := fmt.Sprintf("new-api-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "ref_" + common.Sha1([]byte(reference))

	topUp := &model.TopUp{
		ProviderId:      c.GetInt("provider_id"),
		UserId:          id,
		Amount:          req.Amount,
		Money:           chargedMoney,
		TradeNo:         referenceId,
		PaymentMethod:   PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		BizType:         model.TopUpBizTypePayment,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
		Currency:        stripeCurrency,
		OriginalMoney:   stripeOriginalMoney,
	}
	err := topUp.Insert()
	if err != nil {
		c.JSON(200, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	// Persist the pending order before creating the remote Checkout Session.
	// Stripe test-mode payments can complete immediately; creating the session
	// first races the webhook and can lose a paid top-up because no local row is
	// visible yet.
	payLink, err := genStripeLink(c, referenceId, user.StripeCustomer, user.Email, req.Amount, priceId, stripeCurrencyCode, unitAmount, req.SuccessURL, req.CancelURL, trustedDomains)
	if err != nil {
		log.Println("获取Stripe Checkout支付链接失败", err)
		// The checkout was never handed to the customer, so mark this local
		// pending order failed rather than leaving it indefinitely pending.
		if updateErr := model.UpdatePendingTopUpStatus(referenceId, model.PaymentProviderStripe, common.TopUpStatusFailed); updateErr != nil {
			log.Printf("Stripe 充值订单创建失败后更新状态失败: %v, order=%s", updateErr, referenceId)
		}
		c.JSON(200, gin.H{"message": "error", "data": "拉起支付失败"})
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

	// A 2xx response tells Stripe that the event was durably applied.  Propagate
	// fulfillment failures so Stripe retries instead of silently dropping a
	// paid entitlement when the local database is unavailable or validation
	// fails.
	return fulfillOrder(event, referenceId, customerId)
}

// sessionAsyncPaymentSucceeded handles delayed payment methods (bank transfer, SEPA, etc.)
// that confirm payment after the checkout session completes.
func sessionAsyncPaymentSucceeded(event stripe.Event) error {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	log.Printf("Stripe 异步支付成功: %s", referenceId)

	return fulfillOrder(event, referenceId, customerId)
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

	// A checkout session can represent either a wallet top-up or a subscription
	// order.  Delayed payment failure used to update only the TopUp row, leaving
	// a pending subscription order (and its reserved inventory) until the
	// periodic expiry sweep.  Release a subscription reservation immediately;
	// ExpireSubscriptionOrder is idempotent and also mirrors the checkout.expired
	// path.  Fall through to the wallet-top-up handling only when no subscription
	// order exists for this reference.
	if err := model.ExpireSubscriptionOrder(referenceId, PaymentMethodStripe); err == nil {
		log.Printf("Stripe 异步支付失败，订阅订单已释放: %s", referenceId)
		return nil
	} else if !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		// A mismatched payment method or a database failure must not be allowed to
		// mutate a similarly named wallet order.  Treat a non-pending order as an
		// idempotent no-op, but surface actual persistence errors so Stripe retries.
		if errors.Is(err, model.ErrPaymentMethodMismatch) || errors.Is(err, model.ErrSubscriptionOrderStatusInvalid) {
			log.Printf("Stripe 异步支付失败，订阅订单无需处理: ref=%s err=%v", referenceId, err)
			return nil
		}
		return err
	}

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
		return err
	}
	log.Printf("充值订单已标记为失败: %s", referenceId)
	return nil
}

// fulfillOrder is the shared logic for crediting quota after payment is
// confirmed.  It returns an error deliberately: callers use the error to
// return a non-2xx webhook response, which asks Stripe to retry transient
// database failures and prevents a paid order from being acknowledged before
// its entitlement is actually issued.
func fulfillOrder(event stripe.Event, referenceId string, customerId string) error {
	if len(referenceId) == 0 {
		log.Println("未提供支付单号")
		return errors.New("stripe payment reference is missing")
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
		// New subscription orders carry the immutable Stripe Price ID in both
		// the local snapshot and Checkout Session metadata.  Verify the signed
		// webhook metadata before amount validation/fulfillment so a session for
		// another Price (even one with the same amount) cannot complete this
		// order.  Legacy rows without a product snapshot remain compatible.
		if !stripeSubscriptionProductMatches(subscriptionOrder, &event) {
			return fmt.Errorf("stripe subscription product mismatch for %s", referenceId)
		}
		if !stripeOrderCurrencyMatches(subscriptionOrder.Currency, event.GetObjectValue("currency")) {
			return fmt.Errorf("stripe subscription order currency mismatch for %s", referenceId)
		}
		// 根据回调中的币种，重新计算该订阅订单的预期支付金额
		// 因为订阅可能使用 USD 或 CNY 的 Stripe Price，金额不同
		expectedPayMoney := getSubscriptionStripeExpectedPayMoney(
			subscriptionOrder,
			event.GetObjectValue("currency"), // 回调中的币种，用于确定按哪种币种校验
		)
		// 校验 Stripe 回调金额与预期金额是否一致
		if !stripeAmountTotalMatchesMoney(event.GetObjectValue("amount_total"), expectedPayMoney, event.GetObjectValue("currency")) {
			log.Printf("Stripe 订阅金额校验失败: ref=%s, callback_amount_total=%s, expected_pay_money=%.2f", referenceId, event.GetObjectValue("amount_total"), expectedPayMoney)
			return fmt.Errorf("stripe subscription amount mismatch for %s", referenceId)
		}
		if err := model.CompleteSubscriptionOrder(referenceId, common.GetJsonString(payload), PaymentMethodStripe, model.PaymentProviderStripe); err == nil {
			return nil
		} else if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
			log.Println("complete subscription order failed:", err.Error(), referenceId)
			return err
		}
	}

	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		log.Println("充值订单不存在", referenceId)
		return fmt.Errorf("stripe order %s not found", referenceId)
	}
	if topUp.PaymentMethod != PaymentMethodStripe {
		log.Printf("Stripe 充值订单支付方式不匹配: %s, ref: %s", topUp.PaymentMethod, referenceId)
		return model.ErrPaymentMethodMismatch
	}
	// PaymentProvider was added after the first Stripe integration.  Empty is
	// accepted for legacy rows, but an explicit provider from another gateway
	// must never be credited by a Stripe callback.
	if strings.TrimSpace(topUp.PaymentProvider) != "" && topUp.PaymentProvider != model.PaymentProviderStripe {
		log.Printf("Stripe 充值订单支付服务商不匹配: %s, ref: %s", topUp.PaymentProvider, referenceId)
		return model.ErrPaymentMethodMismatch
	}
	if !stripeOrderCurrencyMatches(topUp.Currency, event.GetObjectValue("currency")) {
		return fmt.Errorf("stripe order currency mismatch for %s", referenceId)
	}

	// Stripe 充值支持可选促销码。未开启促销码时，严格校验回调金额必须与本地订单一致；
	// 开启促销码后，实际支付金额可能低于标价，此处不做强校验，避免误伤合法优惠订单。
	if !setting.StripePromotionCodesEnabled {
		expectedPayMoney := getStripeExpectedPayMoneyFromTopUp(topUp)
		if !stripeAmountTotalMatchesMoney(event.GetObjectValue("amount_total"), expectedPayMoney, event.GetObjectValue("currency")) {
			log.Printf("Stripe 充值金额校验失败: ref=%s, callback_amount_total=%s, expected_pay_money=%.2f", referenceId, event.GetObjectValue("amount_total"), expectedPayMoney)
			return fmt.Errorf("stripe top-up amount mismatch for %s", referenceId)
		}
	}

	err := model.Recharge(referenceId, customerId)
	if err != nil {
		log.Println(err.Error(), referenceId)
		return err
	}

	total, _ := strconv.ParseFloat(event.GetObjectValue("amount_total"), 64)
	currency := strings.ToUpper(event.GetObjectValue("currency"))
	log.Printf("收到款项：%s, %.2f(%s)", referenceId, total/100, currency)
	return nil
}

// stripeSubscriptionProductMatches verifies the immutable Price ID captured
// when a subscription Checkout Session was created.  Orders from before the
// snapshot field was introduced have an empty PaymentProductId; those rows are
// intentionally accepted for backwards compatibility and still receive the
// currency/amount checks below.  For all newer rows, metadata must be present
// and must exactly match the local snapshot.
func stripeSubscriptionProductMatches(order *model.SubscriptionOrder, event *stripe.Event) bool {
	if order == nil {
		return false
	}
	expected := strings.TrimSpace(order.PaymentProductId)
	if expected == "" {
		return true
	}
	actual := stripeCheckoutMetadataValue(event, "new_api_product_id")
	return actual != "" && actual == expected
}

// stripeCheckoutMetadataValue reads Checkout Session metadata without using
// Event.GetObjectValue, whose nested traversal panics for malformed/non-map
// metadata.  Webhook payloads are untrusted input even after signature
// verification, so malformed metadata must fail closed rather than crash the
// handler process.
func stripeCheckoutMetadataValue(event *stripe.Event, key string) string {
	if event == nil || event.Data == nil || event.Data.Object == nil {
		return ""
	}
	metadata, ok := event.Data.Object["metadata"].(map[string]interface{})
	if !ok {
		// Tests and programmatic callers may construct the map with string
		// values directly; support that shape as well.
		if typed, ok := event.Data.Object["metadata"].(map[string]string); ok {
			return strings.TrimSpace(typed[key])
		}
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

// stripeOrderCurrencyMatches compares the currency snapshot stored on a local
// order with Stripe's ISO currency code.  Older rows stored a display symbol
// ("$"/"￥") and some legacy rows left it empty; those values are mapped or
// intentionally accepted for backwards compatibility.  An explicit,
// recognized mismatch is rejected so a valid payment cannot be credited to an
// order created for a different currency.
func stripeOrderCurrencyMatches(storedCurrency, callbackCurrency string) bool {
	stored := strings.ToUpper(strings.TrimSpace(storedCurrency))
	callback := strings.ToUpper(strings.TrimSpace(callbackCurrency))
	// A successful Checkout event must always include Stripe's currency.  An
	// empty callback value is not a safe legacy case because it would make a
	// malformed event indistinguishable from a correctly priced one.
	if callback == "" {
		return false
	}
	if stored == "" {
		return true
	}
	switch stored {
	case "$":
		stored = "USD"
	case "￥", "¥", "元":
		stored = "CNY"
	case "USD", "CNY":
		// already canonical
	default:
		// Older deployments persisted a custom display symbol rather than an
		// ISO code. Preserve compatibility for those rows; the amount check
		// below still protects the known USD/CNY cases.
		return true
	}
	if callback != "USD" && callback != "CNY" {
		return true
	}
	return stored == callback
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
		if errors.Is(err, model.ErrPaymentMethodMismatch) || errors.Is(err, model.ErrSubscriptionOrderStatusInvalid) {
			// The event can be a duplicate after another handler already moved the
			// order out of pending, or can reference a different payment method.
			// Neither case needs a retry.
			return nil
		}
		return err
	}

	topUp := model.GetTopUpByTradeNo(referenceId)
	if topUp == nil {
		log.Println("充值订单不存在", referenceId)
		return nil
	}
	if topUp.PaymentMethod != PaymentMethodStripe {
		log.Printf("Stripe 过期事件订单支付方式不匹配: %s, ref: %s", topUp.PaymentMethod, referenceId)
		return nil
	}
	if strings.TrimSpace(topUp.PaymentProvider) != "" && topUp.PaymentProvider != model.PaymentProviderStripe {
		log.Printf("Stripe 过期事件订单支付服务商不匹配: %s, ref: %s", topUp.PaymentProvider, referenceId)
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
		return err
	}

	log.Println("充值订单已过期", referenceId)
	return nil
}

func resolveStripeRedirectURLs(c *gin.Context, successURL string, cancelURL string, trustedDomains []string) (string, string) {
	baseURL := common.GetTrustedRequestBaseURLWithDomains(c, system_setting.ServerAddress, trustedDomains)
	if successURL == "" {
		successURL = baseURL + "/console/topup"
	}
	if cancelURL == "" {
		cancelURL = baseURL + "/console/topup"
	}
	return successURL, cancelURL
}

// genStripeLink generates a Stripe Checkout session URL for payment.
// It creates a checkout session using inline price_data — the unit amount is
// computed locally (group ratio + amount discount included), so the amount
// Stripe actually charges always matches the local quote captured in
// topUp.OriginalMoney.  quantity stays the recharge amount; the configured
// Stripe Price ID is only used to resolve the product for the line item.
//
// Parameters:
//   - referenceId: unique reference identifier for the transaction
//   - customerId: existing Stripe customer ID (empty string if new customer)
//   - email: customer email address for new customer creation
//   - amount: quantity of units to purchase
//   - priceId: Stripe Price ID whose product anchors the line item
//   - currency: lowercase ISO currency code (e.g. "usd", "cny") for price_data
//   - unitAmount: per-unit price in the currency's smallest unit, computed locally
//   - successURL: custom URL to redirect after successful payment (empty for default)
//   - cancelURL: custom URL to redirect when payment is canceled (empty for default)
//
// Returns the checkout session URL or an error if the session creation fails.
func genStripeLink(c *gin.Context, referenceId string, customerId string, email string, amount int64, priceId string, currency string, unitAmount int64, successURL string, cancelURL string, trustedDomains []string) (string, error) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return "", fmt.Errorf("无效的Stripe API密钥")
	}

	stripe.Key = setting.StripeApiSecret

	// Use custom URLs if provided, otherwise derive from the incoming request.
	successURL, cancelURL = resolveStripeRedirectURLs(c, successURL, cancelURL, trustedDomains)

	// price_data 必须指定 product 或 product_data；复用配置 Price 挂载的
	// 商品，使 Stripe 后台报表仍能归到原商品下。管理员可能改挂 Price，
	// 实时查询而非缓存。
	priceInfo, err := stripeprice.Get(priceId, nil)
	if err != nil {
		return "", fmt.Errorf("查询Stripe Price失败: %w", err)
	}
	if priceInfo == nil || priceInfo.Product == nil || priceInfo.Product.ID == "" {
		return "", fmt.Errorf("Stripe Price 未关联商品: %s", priceId)
	}
	if unitAmount <= 0 {
		return "", fmt.Errorf("Stripe 单价金额无效: %d", unitAmount)
	}

	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(currency),
					UnitAmount: stripe.Int64(unitAmount),
					Product:    stripe.String(priceInfo.Product.ID),
				},
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

// getStripePayMoney 计算用户实际应付金额（未做最小单位取整的连续值）。
// 仅作为参考保留：实际下单/报价统一走 calcStripeUnitAmountAndTotal
// （price_data unit_amount × quantity），保证与 Stripe 实收精确一致。
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

// resolveStripeCurrencyCode 根据用户时区解析 Stripe 币种的小写 ISO 代码
// （Stripe API 要求 price_data.currency 为小写，如 "usd"、"cny"）。
func resolveStripeCurrencyCode(user *model.User) string {
	if config := resolveStripeCurrencyConfig(user); config != nil && strings.TrimSpace(config.Currency) != "" {
		return strings.ToLower(strings.TrimSpace(config.Currency))
	}
	return "usd"
}

// calcStripeUnitAmountAndTotal 计算动态定价所需的"每单位单价（最小货币单位）"
// 与"应收总额（主单位）"。
// 支付环节按面值定价：1 数量 = 1 元（用户币种），不做汇率换算——
// CNY 充 4 付 ¥4，USD 充 4 付 $4。汇率仅在支付完成后入账时用于换算
// 美元额度（见 calcStripeChargedMoney / GetQuotaToAdd），不影响收款金额。
// 分组倍率与档位折扣仍作用于支付价（用户应付 = 数量 × 倍率 × 折扣）。
// 拆分为 unit_amount × quantity 的形式交给 Stripe price_data，以保证
// webhook 回调的 amount_total = unitAmount × quantity 与本地 OriginalMoney
// 快照精确一致。
// unitAmount = floor(总额/数量)（最小单位），不能整除时 +1；
// expectedPay = unitAmount × 数量 还原为主单位——总价因此可能比连续数学值
// 高出至多 (数量-1) 个最小单位（如 ¥0.01×49），对 CNY/USD 量级可忽略，
// 但保证回调校验必然通过。
func calcStripeUnitAmountAndTotal(amount float64, group string, currency string) (int64, float64) {
	// Token 展示模式下，前端传入的是 token 数量，需折算回基础充值额度
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = amount / common.QuotaPerUnit
	}
	topupGroupRatio := common.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	// 查找该充值数量对应的档位折扣，无配置则默认不打折
	discount := 1.0
	if ds, ok := operation_setting.GetPaymentSetting().AmountDiscount[int(amount)]; ok {
		if ds > 0 {
			discount = ds
		}
	}
	// 面值定价：数量 × 1（本币）× 分组倍率 × 档位折扣，不乘汇率
	total := decimal.NewFromFloat(amount).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		Mul(decimal.NewFromFloat(discount))
	unitAmount, rem := convertMoneyToMinorUnitsDecimal(total, currency).QuoRem(decimal.NewFromInt(int64(amount)), 0)
	if !rem.IsZero() {
		// 无法整除时单价进 1 个最小单位，保证总额不低于本地报价
		// （QuoRem 精度 0 返回截断后的整数商，Ceil 无效，必须显式 +1）
		unitAmount = unitAmount.Add(decimal.NewFromInt(1))
	}
	unitAmountInt := unitAmount.IntPart()
	if unitAmountInt <= 0 {
		return 0, 0
	}
	// 还原主单位总额，与 Stripe 回调 amount_total/100 精确对应
	pay := decimal.NewFromInt(unitAmountInt).Mul(decimal.NewFromInt(int64(amount)))
	if !zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(currency))] {
		pay = pay.Div(decimal.NewFromInt(100))
	}
	return unitAmountInt, pay.Round(2).InexactFloat64()
}

// convertMoneyToMinorUnitsDecimal 将金额转换为其币种最小单位的 decimal 值
// （零小数币种如 JPY 直接取整；其余 ×100）。与 convertMoneyToMinorUnits
// 行为一致但保留小数，供整除判断使用。
func convertMoneyToMinorUnitsDecimal(amount decimal.Decimal, currency string) decimal.Decimal {
	if zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(currency))] {
		return amount.Round(0)
	}
	return amount.Mul(decimal.NewFromInt(100)).Round(0)
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
