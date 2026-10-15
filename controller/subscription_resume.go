package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/lakala"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// ResumeSubscriptionOrder resumes the original pending checkout. It never
// creates another subscription order, so the user's purchase reservation and
// the original trade number remain the same for payment callbacks.
func ResumeSubscriptionOrder(c *gin.Context) {
	orderID := c.Param("id")
	var id int
	if _, err := fmt.Sscanf(orderID, "%d", &id); err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的订阅订单")
		return
	}

	order, err := model.GetSubscriptionOrderByID(id)
	if err != nil {
		common.ApiErrorMsg(c, "订阅订单不存在")
		return
	}
	if order.UserId != c.GetInt("id") {
		common.ApiErrorMsg(c, "无权操作该订阅订单")
		return
	}
	if providerID := c.GetInt("provider_id"); providerID != order.ProviderId {
		common.ApiErrorMsg(c, "该订阅订单不属于当前站点")
		return
	}
	if order.Status != common.TopUpStatusPending || order.StockStatus != model.SubscriptionStockStatusReserved {
		if order.Status == common.TopUpStatusExpired {
			common.ApiErrorMsg(c, "该订阅订单已过期，请重新购买")
		} else {
			common.ApiErrorMsg(c, "该订阅订单当前无法继续支付")
		}
		return
	}
	if order.StockExpiresAt > 0 && order.StockExpiresAt <= common.GetTimestamp() {
		_ = model.ExpireSubscriptionOrder(order.TradeNo, "")
		common.ApiErrorMsg(c, "该订阅订单已过期，请重新购买")
		return
	}

	LockOrder(order.TradeNo)
	defer UnlockOrder(order.TradeNo)
	// Re-read after acquiring the per-order lock so a concurrent expiry or
	// payment callback cannot be resumed using stale state.
	order, err = model.GetSubscriptionOrderByID(id)
	if err != nil || order.UserId != c.GetInt("id") || order.Status != common.TopUpStatusPending || order.StockStatus != model.SubscriptionStockStatusReserved {
		common.ApiErrorMsg(c, "该订阅订单当前无法继续支付")
		return
	}
	if order.StockExpiresAt > 0 && order.StockExpiresAt <= common.GetTimestamp() {
		_ = model.ExpireSubscriptionOrder(order.TradeNo, "")
		common.ApiErrorMsg(c, "该订阅订单已过期，请重新购买")
		return
	}

	switch strings.ToLower(strings.TrimSpace(order.PaymentProvider)) {
	case model.PaymentProviderEpay:
		resumeSubscriptionEpay(c, order)
	case model.PaymentProviderLakala:
		resumeSubscriptionLakala(c, order)
	case model.PaymentProviderStripe:
		resumeSubscriptionStripe(c, order)
	case model.PaymentProviderCreem:
		resumeSubscriptionCreem(c, order)
	case model.PaymentProviderWaffoPancake:
		resumeSubscriptionWaffoPancake(c, order)
	case model.PaymentProviderCrypto:
		resumeSubscriptionCrypto(c, order)
	default:
		common.ApiErrorMsg(c, "该支付方式暂不支持恢复，请联系管理员")
	}
}

func resumeSubscriptionEpay(c *gin.Context, order *model.SubscriptionOrder) {
	client := GetEpayClient()
	if client == nil {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}
	returnBaseURL := common.GetTrustedRequestBaseURLWithDomains(c, system_setting.ServerAddress, getPaymentTrustedDomains(c))
	returnURL, err := url.Parse(returnBaseURL + "/api/subscription/epay/return")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}
	notifyURL, err := url.Parse(service.GetCallbackAddress() + "/api/subscription/epay/notify")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}
	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           order.PaymentMethod,
		ServiceTradeNo: order.TradeNo,
		Name:           "SUBSCRIPTION",
		Money:          fmt.Sprintf("%.2f", order.OriginalMoney),
		Device:         epay.PC,
		NotifyUrl:      notifyURL,
		ReturnUrl:      returnURL,
	})
	if err != nil {
		common.ApiErrorMsg(c, "重新发起支付失败")
		return
	}
	common.ApiSuccess(c, gin.H{
		"kind":     "form",
		"trade_no": order.TradeNo,
		"url":      uri,
		"data":     params,
	})
}

func resumeSubscriptionStripe(c *gin.Context, order *model.SubscriptionOrder) {
	if setting.StripeApiSecret == "" || setting.StripeWebhookSecret == "" || strings.TrimSpace(order.PaymentProductId) == "" {
		common.ApiErrorMsg(c, "Stripe 支付配置不可用")
		return
	}
	user, err := model.GetUserById(order.UserId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	currency := "USD"
	if strings.Contains(order.Currency, "¥") || strings.Contains(order.Currency, "￥") || strings.EqualFold(order.Currency, "CNY") {
		currency = "CNY"
	}
	quantity, actualCharge, err := getStripeSubscriptionQuantity(order.PaymentProductId, order.OriginalMoney, currency)
	if err != nil || !normalizeMoneyPrecisionDecimal(actualCharge).Equal(normalizeMoneyPrecisionDecimal(order.OriginalMoney)) {
		common.ApiErrorMsg(c, "Stripe 商品金额已发生变化，请重新购买")
		return
	}
	stripeLink, err := genStripeSubscriptionLink(c, order.TradeNo, "", user.Email, order.PaymentProductId, quantity, getStripeTrustedDomains(c))
	if err != nil {
		common.ApiErrorMsg(c, "重新发起支付失败")
		return
	}
	common.ApiSuccess(c, gin.H{"kind": "redirect", "trade_no": order.TradeNo, "checkout_url": stripeLink})
}

func resumeSubscriptionCreem(c *gin.Context, order *model.SubscriptionOrder) {
	if setting.CreemApiKey == "" || strings.TrimSpace(order.PaymentProductId) == "" {
		common.ApiErrorMsg(c, "Creem 支付配置不可用")
		return
	}
	user, err := model.GetUserById(order.UserId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	plan, _ := model.GetSubscriptionPlanById(order.PlanId)
	name := "Subscription"
	if plan != nil && strings.TrimSpace(plan.Title) != "" {
		name = plan.Title
	}
	product := &CreemProduct{ProductId: order.PaymentProductId, Name: name, Price: order.Money}
	checkoutURL, err := genCreemLink(order.TradeNo, product, user.Email, user.Username)
	if err != nil {
		common.ApiErrorMsg(c, "重新发起支付失败")
		return
	}
	common.ApiSuccess(c, gin.H{"kind": "redirect", "trade_no": order.TradeNo, "checkout_url": checkoutURL})
}

func resumeSubscriptionWaffoPancake(c *gin.Context, order *model.SubscriptionOrder) {
	if strings.TrimSpace(setting.WaffoPancakeMerchantID) == "" || strings.TrimSpace(setting.WaffoPancakePrivateKey) == "" || strings.TrimSpace(order.PaymentProductId) == "" {
		common.ApiErrorMsg(c, "Waffo Pancake 支付配置不可用")
		return
	}
	user, err := model.GetUserById(order.UserId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}
	expiresInSeconds := int(model.DefaultSubscriptionCheckoutSeconds)
	session, err := service.CreateWaffoPancakeCheckoutSession(c.Request.Context(), &service.WaffoPancakeCreateSessionParams{
		ProductID:               order.PaymentProductId,
		BuyerIdentity:           service.WaffoPancakeBuyerIdentityFromUserID(user.Id),
		OrderMerchantExternalID: order.TradeNo,
		PriceSnapshot:           &service.WaffoPancakePriceSnapshot{Amount: decimal.NewFromFloat(order.OriginalMoney).StringFixed(2), TaxCategory: "saas"},
		BuyerEmail:              getWaffoPancakeBuyerEmail(user),
		ExpiresInSeconds:        &expiresInSeconds,
	})
	if err != nil {
		common.ApiErrorMsg(c, "重新发起支付失败")
		return
	}
	common.ApiSuccess(c, gin.H{"kind": "redirect", "trade_no": order.TradeNo, "checkout_url": session.CheckoutURL, "session_id": session.SessionID, "expires_at": session.ExpiresAt})
}

func resumeSubscriptionCrypto(c *gin.Context, order *model.SubscriptionOrder) {
	transaction, err := model.GetCryptoTransactionByTradeNo(order.TradeNo)
	if err != nil || transaction == nil || transaction.UserId != order.UserId || transaction.SubscriptionOrderId != order.Id || transaction.Status != model.CryptoTransactionStatusPending {
		common.ApiErrorMsg(c, "加密货币订单状态不可恢复")
		return
	}
	common.ApiSuccess(c, gin.H{
		"kind":             "crypto",
		"trade_no":         order.TradeNo,
		"plan_id":          order.PlanId,
		"network":          transaction.ChainId,
		"chain_id":         transaction.ChainId,
		"token":            transaction.TokenSymbol,
		"token_contract":   transaction.TokenContract,
		"to_address":       transaction.ReceiverAddress,
		"pay_amount":       transaction.UsdtAmount,
		"decimals":         transaction.TokenDecimals,
		"confirmations":    transaction.Confirmations,
		"amount":           order.Money,
		"display_currency": order.Currency,
	})
}

// resumeSubscriptionLakala recreates the QR-code preorder with the original
// trade number. The local subscription order is never duplicated, and the
// callback still settles the same reserved order.
func resumeSubscriptionLakala(c *gin.Context, order *model.SubscriptionOrder) {
	if order == nil || order.TradeNo == "" || order.PaymentMethod != model.PaymentProviderLakala {
		common.ApiErrorMsg(c, "订阅订单无效")
		return
	}
	config := getLakalaOptionConfig()
	if config.AppID == "" || config.SerialNo == "" || config.PrivateKey == "" || config.PublicCert == "" || config.MerchantNo == "" || config.CallbackAddress == "" {
		common.ApiErrorMsg(c, "当前管理员未配置拉卡拉支付信息")
		return
	}
	totalAmount := decimal.NewFromFloat(order.OriginalMoney).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if totalAmount <= 0 {
		common.ApiErrorMsg(c, "订阅金额过低")
		return
	}
	requestBody := map[string]any{
		"req_time": time.Now().Format("20060102150405"),
		"version":  "3.0",
		"req_data": map[string]any{
			"merchant_no":   config.MerchantNo,
			"term_no":       lakalaTermNo,
			"out_trade_no":  order.TradeNo,
			"account_type":  accountTypeForResume(),
			"trans_type":    "41",
			"total_amount":  totalAmount,
			"notify_url":    buildLakalaNotifyURL(config.CallbackAddress, lakalaSubscriptionNotifyURLPath),
			"location_info": map[string]any{"request_ip": c.ClientIP()},
			"subject":       lakalaSubscriptionSubject,
			"remark":        fmt.Sprintf("%d", order.UserId),
		},
	}
	body, err := common.Marshal(requestBody)
	if err != nil {
		common.ApiErrorMsg(c, "创建拉卡拉请求失败")
		return
	}
	signResult, err := lakala.Sign(config.AppID, config.SerialNo, config.PrivateKey, string(body))
	if err != nil {
		common.ApiErrorMsg(c, "拉卡拉请求签名失败")
		return
	}
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, lakalaPreorderURL, bytes.NewReader(body))
	if err != nil {
		common.ApiErrorMsg(c, "创建拉卡拉请求失败")
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", signResult.Authorization)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		common.ApiErrorMsg(c, "请求拉卡拉失败")
		return
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		common.ApiErrorMsg(c, "拉卡拉预下单失败")
		return
	}
	if err := verifyLakalaResponse(config.PublicCert, resp.Header, respBody); err != nil {
		common.ApiErrorMsg(c, "拉卡拉响应验签失败")
		return
	}
	code, err := extractLakalaQRCode(respBody)
	if err != nil {
		common.ApiErrorMsg(c, "拉卡拉未返回支付二维码")
		return
	}
	common.ApiSuccess(c, gin.H{
		"kind": "qrcode",
		"url":  lakalaQRCodePath,
		"data": map[string]string{
			"code": code, "trade_no": order.TradeNo,
			"amount": decimal.NewFromFloat(order.OriginalMoney).Round(2).StringFixed(2),
		},
	})
}

// accountTypeForResume mirrors the subscription Lakala request constant while
// keeping the resume endpoint independent from request DTOs.
func accountTypeForResume() string { return lakalaAccountType }
