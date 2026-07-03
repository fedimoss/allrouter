package controller

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	stripeprice "github.com/stripe/stripe-go/v81/price"
	"github.com/thanhpk/randstr"
)

// SubscriptionStripePayRequest 订阅 Stripe 支付请求参数
type SubscriptionStripePayRequest struct {
	PlanId          int    `json:"plan_id"`                    // 要购买的订阅套餐 ID
	DisplayCurrency string `json:"display_currency,omitempty"` // 前端传入的展示币种（如 "USD"、"CNY"），用于选择对应的 Stripe Price ID
}

func SubscriptionRequestStripePay(c *gin.Context) {
	var req SubscriptionStripePayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	// 校验套餐至少配置了一个 Stripe Price ID（USD 或 CNY），否则无法发起支付
	if strings.TrimSpace(plan.StripePriceId) == "" && strings.TrimSpace(plan.StripePriceCnyId) == "" {
		common.ApiErrorMsg(c, "该套餐未配置 Stripe PriceId") // 提示管理员需要在套餐中配置 Stripe 价格
		return
	}
	if plan.AllowPurchase != 1 {
		common.ApiErrorMsg(c, "该套餐暂不允许订阅")
		return
	}
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		common.ApiErrorMsg(c, "Stripe 未配置或密钥无效")
		return
	}
	if setting.StripeWebhookSecret == "" {
		common.ApiErrorMsg(c, "Stripe Webhook 未配置")
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

	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorMsg(c, "已达到该套餐购买上限")
			return
		}
	}

	// 根据用户时区和前端请求参数，确定本次支付的展示币种（USD 或 CNY）
	displayCurrency := resolveSubscriptionStripeDisplayCurrency(user, req.DisplayCurrency)
	// 根据展示币种，从套餐中选择对应的 Stripe Price ID
	priceId := resolveSubscriptionStripePriceID(displayCurrency, plan)
	// 如果该币种没有配置对应的 Stripe Price ID，则无法发起支付
	if priceId == "" {
		common.ApiErrorMsg(c, "该套餐未配置当前币种的 Stripe PriceId")
		return
	}
	// 根据币种计算实际应付金额（USD 直接返回原价，CNY 则按汇率换算）
	chargeMoney := getSubscriptionChargeMoneyByCurrency(plan.PriceAmount, displayCurrency)

	// 生成唯一的订单参考号，格式：sub-stripe-ref-{用户ID}-{毫秒时间戳}-{4位随机字符串}
	reference := fmt.Sprintf("sub-stripe-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	// 对参考号做 SHA1 哈希，加上 "sub_ref_" 前缀作为最终订单号，避免重复
	referenceId := "sub_ref_" + common.Sha1([]byte(reference))

	// 调用 Stripe API 创建订阅 Checkout Session，传入币种对应的 priceId 和计算出的应付金额
	trustedDomains := getStripeTrustedDomains(c)
	payLink, actualCharge, err := genStripeSubscriptionLink(c, referenceId, user.StripeCustomer, user.Email, priceId, chargeMoney, trustedDomains)
	if err != nil {
		log.Println("获取Stripe Checkout支付链接失败", err)
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}

	// 根据展示币种确定币种符号
	currencySymbol := "$"
	if strings.EqualFold(displayCurrency, "CNY") {
		currencySymbol = "￥"
	}

	order := &model.SubscriptionOrder{
		UserId:        userId,
		PlanId:        plan.Id,
		Money:         plan.PriceAmount,
		Currency:      currencySymbol, // 币种符号
		OriginalMoney: actualCharge,   // 实际支付金额（用户币种）
		TradeNo:       referenceId,
		PaymentMethod: PaymentMethodStripe,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": payLink,
		},
	})
}

// genStripeSubscriptionLink 生成 Stripe 订阅 Checkout Session 的支付链接
// chargeMoney 为根据币种换算后的应付金额，需要通过查询 Stripe Price 的单价来计算 quantity
func genStripeSubscriptionLink(c *gin.Context, referenceId string, customerId string, email string, priceId string, chargeMoney float64, trustedDomains []string) (string, float64, error) {
	// 设置 Stripe API 密钥
	stripe.Key = setting.StripeApiSecret

	// 通过 Stripe Price API 查询该 priceId 的单价，然后用 chargeMoney / 单价 计算出 quantity
	quantity, actualCharge, err := getStripeSubscriptionQuantity(priceId, chargeMoney)
	if err != nil {
		return "", 0, err
	}

	// 构建 Checkout Session 参数并创建会话
	params := buildStripeSubscriptionCheckoutParams(c, referenceId, customerId, email, priceId, quantity, trustedDomains)
	// 调用 Stripe SDK 创建 Checkout Session
	result, err := session.New(params)
	if err != nil {
		return "", 0, err
	}
	// 返回支付链接 URL
	return result.URL, actualCharge, nil
}

// buildStripeSubscriptionCheckoutParams 构建 Stripe 订阅 Checkout Session 的请求参数
func buildStripeSubscriptionCheckoutParams(c *gin.Context, referenceId string, customerId string, email string, priceId string, quantity int64, trustedDomains []string) *stripe.CheckoutSessionParams {
	baseURL := common.GetTrustedRequestBaseURLWithDomains(c, system_setting.ServerAddress, trustedDomains)
	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),                // 客户端引用 ID，用于 Webhook 回调时匹配订单
		SuccessURL:        stripe.String(baseURL + "/console/topup"), // 支付成功后跳回发起请求的域名
		CancelURL:         stripe.String(baseURL + "/console/topup"), // 支付取消后跳回发起请求的域名
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceId), // 使用币种对应的 Stripe Price ID
				Quantity: stripe.Int64(quantity), // quantity = chargeMoney / Price 单价
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)), // 订阅模式
	}

	// 订阅模式不传 Customer，避免同一客户下多币种冲突（Stripe 不允许一个 Customer 混用 USD/CNY 订阅）
	// 通过邮箱关联，Stripe 会按邮箱自动匹配已有客户
	if email != "" {
		params.CustomerEmail = stripe.String(email) // 传入邮箱，Stripe 会自动创建客户
	}

	return params
}

// resolveSubscriptionStripeDisplayCurrency 确定本次订阅支付使用的展示币种
// 优先使用前端请求中指定的币种，否则根据用户时区自动推断，最终回退到 USD
func resolveSubscriptionStripeDisplayCurrency(user *model.User, requestedCurrency string) string {
	// 将前端传入的币种转为大写并去除空格
	currency := strings.ToUpper(strings.TrimSpace(requestedCurrency))
	// 如果前端明确指定了 USD 或 CNY，直接使用
	if currency == "USD" || currency == "CNY" {
		return currency
	}
	// 前端未指定时，如果用户信息为空则默认 USD
	if user == nil {
		return "USD"
	}
	// 根据用户时区查询对应的展示币种（例如 Asia/Shanghai -> CNY）
	return model.GetDisplayCurrencyInfoByTimezone(user.Timezone).Currency
}

// resolveSubscriptionStripePriceID 根据展示币种从套餐中选择对应的 Stripe Price ID
// CNY -> StripePriceCnyId，其他 -> StripePriceId
func resolveSubscriptionStripePriceID(displayCurrency string, plan *model.SubscriptionPlan) string {
	if plan == nil {
		return ""
	}
	// 如果展示币种是人民币，返回人民币专用的 Stripe Price ID
	if strings.EqualFold(displayCurrency, "CNY") {
		return strings.TrimSpace(plan.StripePriceCnyId)
	}
	// 否则返回美元的 Stripe Price ID
	return strings.TrimSpace(plan.StripePriceId)
}

// getSubscriptionStripeExpectedPayMoney 获取订阅订单在指定币种下的预期支付金额
// 用于 Webhook 回调时的金额校验，确保回调金额与本地预期一致
func getSubscriptionStripeExpectedPayMoney(order *model.SubscriptionOrder, chargeCurrency string) float64 {
	if order == nil {
		return 0
	}
	// 根据订单的 PlanId 查询套餐信息
	plan, err := model.GetSubscriptionPlanById(order.PlanId)
	if err != nil || plan == nil {
		return order.Money // 查不到套餐时回退到订单原始金额
	}
	// 按币种换算应付金额
	return getSubscriptionChargeMoneyByCurrency(plan.PriceAmount, chargeCurrency)
}

// getSubscriptionChargeMoneyByCurrency 根据币种计算订阅应付金额
// USD 直接返回原价；CNY 则按 CNY 币种配置中的 UnitPrice（汇率）换算
func getSubscriptionChargeMoneyByCurrency(priceAmount float64, chargeCurrency string) float64 {
	// 如果是人民币，查找 CNY 币种配置中的汇率进行换算
	if strings.EqualFold(strings.TrimSpace(chargeCurrency), "CNY") {
		if cnyConfig, err := model.GetCurrencyConfig("CNY"); err == nil && cnyConfig != nil && cnyConfig.UnitPrice > 0 {
			// 原价 × 汇率 = 人民币金额，四舍五入到合适精度
			return model.RoundDisplayCurrencyAmount(priceAmount * cnyConfig.UnitPrice)
		}
	}
	// 非人民币或找不到汇率配置时，直接返回原价（USD）
	return model.RoundDisplayCurrencyAmount(priceAmount)
}

// getStripeSubscriptionQuantity 通过 Stripe Price API 查询单价，计算订阅的购买数量
// Stripe Checkout 的金额 = Price 单价 × quantity，因此 quantity = 应付金额(分) / 单价(分)
// 如果能整除则精确匹配，不能整除则向下取整（实际收费略低于预期），同时记录日志。
func getStripeSubscriptionQuantity(priceId string, chargeMoney float64) (int64, float64, error) {
	// 校验 priceId 非空
	if strings.TrimSpace(priceId) == "" {
		return 0, 0, errors.New("empty stripe price id")
	}

	// 调用 Stripe API 查询 Price 详情（包含单价和币种）
	priceInfo, err := stripeprice.Get(priceId, nil)
	if err != nil {
		return 0, 0, err
	}
	// 校验返回的价格信息有效且单价大于 0
	if priceInfo == nil || priceInfo.UnitAmount <= 0 {
		return 0, 0, errors.New("invalid stripe price amount")
	}

	// 将应付金额转换为最小货币单位（如 USD: 美元 -> 美分）
	expectedMinor := convertMoneyToMinorUnits(chargeMoney, string(priceInfo.Currency))
	if expectedMinor <= 0 {
		return 0, 0, errors.New("invalid subscription charge amount")
	}

	// 计算购买数量 = 应付总额(分) / 单价(分)
	quantity := expectedMinor / priceInfo.UnitAmount
	if quantity <= 0 {
		return 0, 0, errors.New("invalid stripe subscription quantity")
	}

	// 计算实际收费金额（分 -> 元）
	actualMinor := quantity * priceInfo.UnitAmount
	actualCharge := float64(actualMinor)
	if !zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(string(priceInfo.Currency)))] {
		actualCharge = actualCharge / 100.0
	}

	// 不能整除时记录日志，但不阻断支付
	if expectedMinor%priceInfo.UnitAmount != 0 {
		log.Printf("Stripe subscription amount not exact: expected=%.2f(%d minor), unit=%d, quantity=%d, actual=%.2f(%d minor)",
			chargeMoney, expectedMinor, priceInfo.UnitAmount, quantity, actualCharge, actualMinor)
	}

	return quantity, actualCharge, nil
}

// convertMoneyToMinorUnits 将金额转换为最小货币单位（如美元 -> 美分）
// 某些零小数币种（如 JPY）不需要乘以 100
func convertMoneyToMinorUnits(amount float64, currency string) int64 {
	// 使用 decimal 库规范化金额精度，避免浮点误差
	dAmount := normalizeMoneyPrecisionDecimal(amount)
	// 判断是否为零小数币种（如日元 JPY、韩元 KRW 等，最小单位就是元，不需要 ×100）
	if zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(currency))] {
		return dAmount.Round(0).IntPart() // 零小数币种直接取整
	}
	// 非零小数币种，金额 ×100 转换为最小单位（如 USD 美元 -> 美分）
	return dAmount.Mul(decimal.NewFromInt(100)).Round(0).IntPart()
}
