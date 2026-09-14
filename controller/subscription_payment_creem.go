package controller

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
)

type SubscriptionCreemPayRequest struct {
	PlanId int `json:"plan_id"`
}

// resolveSubscriptionCreemProductSnapshot reads the optional local Creem
// product catalog and returns the currency/price that the remote product is
// configured with.  SubscriptionPlan.PriceAmount is the USD accounting value,
// while Creem charges the fixed amount attached to ProductId; using the site
// wide quota-display currency here (for example CNY) would create an order
// whose webhook can never match a USD Creem product.  If the product is not in
// the top-up catalog, retain the historical USD behaviour and let the locked
// plan price remain authoritative.
func resolveSubscriptionCreemProductSnapshot(productID string) (currency string, price float64, found bool, err error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return "", 0, false, fmt.Errorf("empty Creem product id")
	}
	raw := strings.TrimSpace(setting.CreemProducts)
	if raw == "" || raw == "[]" {
		return "USD", 0, false, nil
	}
	var products []CreemProduct
	if err := common.Unmarshal([]byte(raw), &products); err != nil {
		return "", 0, false, fmt.Errorf("invalid Creem product catalog: %w", err)
	}
	for i := range products {
		product := &products[i]
		if !strings.EqualFold(strings.TrimSpace(product.ProductId), productID) {
			continue
		}
		if math.IsNaN(product.Price) || math.IsInf(product.Price, 0) || product.Price <= 0 {
			return "", 0, false, fmt.Errorf("invalid Creem product price")
		}
		currency = strings.ToUpper(strings.TrimSpace(product.Currency))
		if currency == "" {
			// Older catalog entries omitted currency and Creem defaults to USD.
			currency = "USD"
		}
		// The checkout/webhook path below has explicit symbol/code mappings;
		// reject an unsupported catalog currency now rather than creating an
		// order that can never pass callback reconciliation.
		switch currency {
		case "USD", "EUR", "CNY":
		default:
			return "", 0, false, fmt.Errorf("unsupported Creem product currency: %s", currency)
		}
		return currency, product.Price, true, nil
	}
	return "USD", 0, false, nil
}

func creemCurrencySymbol(currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "CNY", "RMB", "￥", "¥":
		return "￥"
	case "EUR", "€":
		return "€"
	default:
		return "$"
	}
}

func SubscriptionRequestCreemPay(c *gin.Context) {
	var req SubscriptionCreemPayRequest

	// Keep body for debugging consistency (like RequestCreemPay)
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("read subscription creem pay req body err: %v", err)
		c.JSON(200, gin.H{"message": "error", "data": "read query error"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		c.JSON(200, gin.H{"message": "error", "data": "参数错误"})
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 统一购买前校验：套餐存在/启用/允许购买 + 是否对当前 provider_id 可见（防止跨站点订阅）。
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
	if plan.CreemProductId == "" {
		common.ApiErrorMsg(c, "该套餐未配置 CreemProductId")
		return
	}
	if setting.CreemWebhookSecret == "" && !setting.CreemTestMode {
		common.ApiErrorMsg(c, "Creem Webhook 未配置")
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

	reference := "sub-creem-ref-" + randstr.String(6)
	referenceId := "sub_ref_" + common.Sha1([]byte(reference+time.Now().String()+user.Username))

	// Creem product prices are fixed on the provider side.  Do not derive the
	// charged currency from the site's quota-display setting: a CNY display
	// setting does not convert a USD Creem ProductId, and would make every
	// legitimate USD webhook fail currency validation.  When the administrator
	// has also listed this product in CreemProducts, use that immutable catalog
	// price/currency (EUR and CNY products are supported as well); otherwise
	// preserve the historical USD/plan-price path.
	currency := "USD"
	currencySymbol := "$"
	originalMoney := plan.PriceAmount
	if productCurrency, productPrice, found, err := resolveSubscriptionCreemProductSnapshot(plan.CreemProductId); err != nil {
		common.ApiErrorMsg(c, "Creem 产品配置错误")
		return
	} else if found {
		currency = productCurrency
		currencySymbol = creemCurrencySymbol(productCurrency)
		originalMoney = productPrice
	}

	// create pending order first
	order := &model.SubscriptionOrder{
		UserId: userId,
		PlanId: plan.Id,
		// 记录订单归属服务商：来自请求上下文 provider_id（0=主站），后续完成订单时据此给服务商 owner 结算订阅收入。
		ProviderId:       c.GetInt("provider_id"),
		Money:            plan.PriceAmount,
		Currency:         currencySymbol, // 币种符号
		OriginalMoney:    originalMoney,  // 实际支付金额（用户币种）
		TradeNo:          referenceId,
		PaymentMethod:    PaymentMethodCreem,
		PaymentProvider:  model.PaymentProviderCreem,
		PaymentProductId: plan.CreemProductId,
		CreateTime:       time.Now().Unix(),
		Status:           common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		respondSubscriptionCreateError(c, err, "创建订单失败")
		return
	}

	// Reuse Creem checkout generator by building a lightweight product reference.
	// The order fields were checked against the locked plan row.  Do not read
	// the mutable/cached plan again after reserving inventory.
	product := &CreemProduct{
		ProductId: order.PaymentProductId,
		Name:      plan.Title,
		Price:     order.Money,
		Currency:  currency,
		Quota:     0,
	}

	checkoutUrl, err := genCreemLink(referenceId, product, user.Email, user.Username)
	if err != nil {
		_ = model.ExpireSubscriptionOrder(referenceId, PaymentMethodCreem)
		log.Printf("获取Creem支付链接失败: %v", err)
		c.JSON(200, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"data": gin.H{
			"checkout_url": checkoutUrl,
			"order_id":     referenceId,
		},
	})
}
