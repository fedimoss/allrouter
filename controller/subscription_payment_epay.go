package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type SubscriptionEpayPayRequest struct {
	PlanId        int    `json:"plan_id"`
	PaymentMethod string `json:"payment_method"`
}

// getSubscriptionEpayChargeMoney 计算订阅易支付的实际扣款金额
// 易支付面向人民币支付页面，始终将存储的 USD 套餐价格转换为 CNY
func getSubscriptionEpayChargeMoney(_ *gin.Context, usdPrice float64) (float64, error) {
	// 套餐原价必须大于 0
	if usdPrice <= 0 {
		return 0, fmt.Errorf("invalid subscription price")
	}

	// 从数据库获取 CNY 币种配置（含汇率）
	cnyConfig, err := model.GetCurrencyConfig("CNY")
	if err != nil || cnyConfig == nil || cnyConfig.UnitPrice <= 0 {
		return 0, fmt.Errorf("cny currency config not found")
	}

	// USD 价格 × CNY 汇率，保留两位小数
	chargeMoney := normalizeDisplayMoneyDecimal(usdPrice).
		Mul(normalizeDisplayMoneyDecimal(cnyConfig.UnitPrice)).
		Round(2).
		InexactFloat64()
	// 转换后金额必须大于 0
	if chargeMoney <= 0 {
		return 0, fmt.Errorf("invalid converted cny amount")
	}

	return chargeMoney, nil
}

// subscriptionEpayOrderMoneyMatches 校验易支付回调金额是否与订单金额匹配
// 统一使用 epayCallbackMoneyMatches（支持浮点容差匹配），兼容 CNY 转换后的金额
func subscriptionEpayOrderMoneyMatches(order *model.SubscriptionOrder, callbackMoney string) bool {
	// 订单为空直接不匹配
	if order == nil {
		return false
	}
	// 优先用 OriginalMoney 直接比较人民币金额，兼容历史订单回退到 USD 换算比较
	if order.OriginalMoney > 0 {
		return amountStringMatchesMoney(callbackMoney, order.OriginalMoney)
	}

	// 使用容差匹配，处理浮点精度问题
	return epayCallbackMoneyMatches(callbackMoney, order.Money)
}

func SubscriptionRequestEpay(c *gin.Context) {
	var req SubscriptionEpayPayRequest
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
	if plan.PriceAmount < 0.01 {
		common.ApiErrorMsg(c, "套餐金额过低")
		return
	}
	if !operation_setting.ContainsPayMethod(req.PaymentMethod) {
		common.ApiErrorMsg(c, "支付方式不存在")
		return
	}

	// 拉卡拉作为独立支付方式配置在"充值方式设置"中，命中后走拉卡拉预下单；其他方式继续走易支付。
	// 与充值流程（topup.go）的分支逻辑一致：易支付和拉卡拉不会同时存在。
	if req.PaymentMethod == model.PaymentProviderLakala {
		chargeMoney, err := getSubscriptionEpayChargeMoney(c, plan.PriceAmount)
		if err != nil {
			common.ApiErrorMsg(c, "计算支付金额失败")
			return
		}
		if chargeMoney < 0.01 {
			common.ApiErrorMsg(c, "套餐金额过低")
			return
		}
		userId := c.GetInt("id")
		requestSubscriptionLakalaPay(c, plan, req, userId, chargeMoney, plan.PriceAmount)
		return
	}

	userId := c.GetInt("id")
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

	callBackAddress := service.GetCallbackAddress()
	returnBaseURL := common.GetTrustedRequestBaseURLWithDomains(c, system_setting.ServerAddress, getPaymentTrustedDomains(c))
	returnUrl, err := url.Parse(returnBaseURL + "/api/subscription/epay/return")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}
	notifyUrl, err := url.Parse(callBackAddress + "/api/subscription/epay/notify")
	if err != nil {
		common.ApiErrorMsg(c, "回调地址配置错误")
		return
	}

	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("SUBUSR%dNO%s", userId, tradeNo)

	client := GetEpayClient()
	if client == nil {
		common.ApiErrorMsg(c, "当前管理员未配置支付信息")
		return
	}

	// 根据用户时区币种计算实际支付金额（USD 转 CNY）
	chargeMoney, err := getSubscriptionEpayChargeMoney(c, plan.PriceAmount)
	if err != nil {
		common.ApiErrorMsg(c, "计算支付金额失败")
		return
	}
	// 转换后金额不能低于 0.01
	if chargeMoney < 0.01 {
		common.ApiErrorMsg(c, "套餐金额过低")
		return
	}

	order := &model.SubscriptionOrder{
		UserId:        userId,
		PlanId:        plan.Id,
		Money:         plan.PriceAmount,
		Currency:      "￥",         // 易支付固定人民币
		OriginalMoney: chargeMoney, // 实际支付的人民币金额
		TradeNo:       tradeNo,
		PaymentMethod: req.PaymentMethod,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	uri, params, err := client.Purchase(&epay.PurchaseArgs{
		Type:           req.PaymentMethod,
		ServiceTradeNo: tradeNo,
		Name:           fmt.Sprintf("SUB:%s", plan.Title),
		Money:          strconv.FormatFloat(chargeMoney, 'f', 2, 64), // 使用币种转换后的实际扣款金额
		Device:         epay.PC,
		NotifyUrl:      notifyUrl,
		ReturnUrl:      returnUrl,
	})
	if err != nil {
		_ = model.ExpireSubscriptionOrder(tradeNo, req.PaymentMethod)
		common.ApiErrorMsg(c, "拉起支付失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success", "data": params, "url": uri})
}

func SubscriptionEpayNotify(c *gin.Context) {
	var params map[string]string

	if c.Request.Method == "POST" {
		// POST 请求：从 POST body 解析参数
		if err := c.Request.ParseForm(); err != nil {
			_, _ = c.Writer.Write([]byte("fail"))
			return
		}
		params = lo.Reduce(lo.Keys(c.Request.PostForm), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.PostForm.Get(t)
			return r
		}, map[string]string{})
	} else {
		// GET 请求：从 URL Query 解析参数
		params = lo.Reduce(lo.Keys(c.Request.URL.Query()), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.URL.Query().Get(t)
			return r
		}, map[string]string{})
	}

	if len(params) == 0 {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	client := GetEpayClient()
	if client == nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	if verifyInfo.TradeStatus != epay.StatusTradeSuccess {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	LockOrder(verifyInfo.ServiceTradeNo)
	defer UnlockOrder(verifyInfo.ServiceTradeNo)

	order := model.GetSubscriptionOrderByTradeNo(verifyInfo.ServiceTradeNo)
	// 校验回调金额与订单金额是否匹配（支持 CNY 容差匹配）
	if !subscriptionEpayOrderMoneyMatches(order, verifyInfo.Money) {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	if err := model.CompleteSubscriptionOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), verifyInfo.Type); err != nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	_, _ = c.Writer.Write([]byte("success"))
}

// SubscriptionEpayReturn handles browser return after payment.
// It verifies the payload and completes the order, then redirects to console.
func SubscriptionEpayReturn(c *gin.Context) {
	var params map[string]string
	returnBaseURL := common.GetTrustedRequestBaseURLWithDomains(c, system_setting.ServerAddress, getPaymentTrustedDomains(c))

	if c.Request.Method == "POST" {
		// POST 请求：从 POST body 解析参数
		if err := c.Request.ParseForm(); err != nil {
			c.Redirect(http.StatusFound, returnBaseURL+"/console/topup?pay=fail")
			return
		}
		params = lo.Reduce(lo.Keys(c.Request.PostForm), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.PostForm.Get(t)
			return r
		}, map[string]string{})
	} else {
		// GET 请求：从 URL Query 解析参数
		params = lo.Reduce(lo.Keys(c.Request.URL.Query()), func(r map[string]string, t string, i int) map[string]string {
			r[t] = c.Request.URL.Query().Get(t)
			return r
		}, map[string]string{})
	}

	if len(params) == 0 {
		c.Redirect(http.StatusFound, returnBaseURL+"/console/topup?pay=fail")
		return
	}

	client := GetEpayClient()
	if client == nil {
		c.Redirect(http.StatusFound, returnBaseURL+"/console/topup?pay=fail")
		return
	}
	verifyInfo, err := client.Verify(params)
	if err != nil || !verifyInfo.VerifyStatus {
		c.Redirect(http.StatusFound, returnBaseURL+"/console/topup?pay=fail")
		return
	}
	if verifyInfo.TradeStatus == epay.StatusTradeSuccess {
		LockOrder(verifyInfo.ServiceTradeNo)
		defer UnlockOrder(verifyInfo.ServiceTradeNo)

		order := model.GetSubscriptionOrderByTradeNo(verifyInfo.ServiceTradeNo)
		// 校验回调金额与订单金额是否匹配（支持 CNY 容差匹配）
		if !subscriptionEpayOrderMoneyMatches(order, verifyInfo.Money) {
			c.Redirect(http.StatusFound, returnBaseURL+"/console/topup?pay=fail")
			return
		}
		if err := model.CompleteSubscriptionOrder(verifyInfo.ServiceTradeNo, common.GetJsonString(verifyInfo), verifyInfo.Type); err != nil {
			c.Redirect(http.StatusFound, returnBaseURL+"/console/topup?pay=fail")
			return
		}
		c.Redirect(http.StatusFound, returnBaseURL+"/console/topup?pay=success")
		return
	}

	c.Redirect(http.StatusFound, returnBaseURL+"/console/topup?pay=pending")
}
