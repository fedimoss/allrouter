package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/lakala"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	// lakalaSubscriptionNotifyURLPath 是订阅拉卡拉支付回调的 API 路径（相对）。
	// 实际完整地址 = service.GetCallbackAddress() + 此路径。
	lakalaSubscriptionNotifyURLPath = "/api/subscription/lakala/notify"
	// lakalaSubscriptionSubject 是订阅拉卡拉支付订单的商品标题。
	lakalaSubscriptionSubject = "订阅"
)

// requestSubscriptionLakalaPay 处理订阅套餐购买的拉卡拉扫码支付预下单。
//
// 参数说明：
//   - c: Gin 上下文，用于获取用户信息和客户端IP
//   - plan: 订阅套餐对象
//   - req: 前端请求参数（包含 payment_method）
//   - userId: 当前用户ID
//   - chargeMoney: 实际支付的人民币金额（元）
//   - usdPrice: 套餐原始美元价格
//
// 流程与充值拉卡拉（requestLakalaWechatPay）一致，区别仅在于：
//   - 创建 SubscriptionOrder 而非 TopUp
//   - 使用独立的回调路径 /api/subscription/lakala/notify
//   - 使用 lakalaSubscriptionSubject 作为商品标题
func requestSubscriptionLakalaPay(c *gin.Context, plan *model.SubscriptionPlan, req SubscriptionEpayPayRequest, userId int, chargeMoney float64, usdPrice float64) {
	// 获取拉卡拉配置并校验完整性
	config := getLakalaOptionConfig()
	if config.AppID == "" || config.SerialNo == "" || config.PrivateKey == "" || config.PublicCert == "" || config.MerchantNo == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "当前管理员未配置拉卡拉支付信息"})
		return
	}

	// 将人民币金额元转分，拉卡拉金额单位为分
	totalAmount := decimal.NewFromFloat(chargeMoney).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if totalAmount <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "套餐金额过低"})
		return
	}

	// lakalaPayMoney 保存人民币元金额，用于前端展示和后续订单记录
	lakalaPayMoney := decimal.NewFromFloat(chargeMoney)

	// 生成唯一交易流水号（与充值格式区分：SUB 前缀）
	tradeNo := fmt.Sprintf("%s%d", common.GetRandomString(6), time.Now().Unix())
	tradeNo = fmt.Sprintf("SUBUSR%dNO%s", userId, tradeNo)

	// 构造回调地址（订阅专用路径）
	callBackAddress := service.GetCallbackAddress()
	lakalaNotifyURL := callBackAddress + lakalaSubscriptionNotifyURLPath

	// 构造拉卡拉预下单请求体
	requestBody := map[string]any{
		"req_time": time.Now().Format("20060102150405"),
		"version":  "3.0",
		"req_data": map[string]any{
			"merchant_no":  config.MerchantNo,
			"term_no":      lakalaTermNo,
			"out_trade_no": tradeNo,
			"account_type": lakalaAccountType,
			"trans_type":   "41",
			"total_amount": totalAmount,
			"notify_url":   lakalaNotifyURL,
			"location_info": map[string]any{
				"request_ip": c.ClientIP(),
			},
			"subject": lakalaSubscriptionSubject,
			"remark":  fmt.Sprintf("%d", userId),
		},
	}

	// 将请求体序列化为JSON
	body, err := common.Marshal(requestBody)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建拉卡拉请求失败"})
		return
	}

	// 使用拉卡拉SDK对请求体签名，生成Authorization头
	signResult, err := lakala.Sign(config.AppID, config.SerialNo, config.PrivateKey, string(body))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉卡拉请求签名失败"})
		return
	}

	// 构造HTTP请求
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, lakalaPreorderURL, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建拉卡拉请求失败"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", signResult.Authorization)

	// 发送预下单请求到拉卡拉
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "请求拉卡拉失败"})
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "读取拉卡拉响应失败"})
		return
	}

	// 检查HTTP状态码
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉卡拉预下单失败"})
		return
	}

	// 使用拉卡拉平台公钥验证响应签名
	if err := verifyLakalaResponse(config.PublicCert, resp.Header, respBody); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉卡拉响应验签失败"})
		return
	}

	// 从响应中提取支付二维码字符串
	code, err := extractLakalaQRCode(respBody)
	if err != nil {
		common.SysLog(fmt.Sprintf("订阅拉卡拉预下单未返回二维码: %v, response=%s", err, string(respBody)))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}

	// 创建本地订阅订单记录
	order := &model.SubscriptionOrder{
		UserId:          userId,
		PlanId:          plan.Id,
		Money:           usdPrice,                        // 套餐原价（USD）
		Currency:        "¥",                             // 拉卡拉固定人民币
		OriginalMoney:   lakalaPayMoney.InexactFloat64(), // 实际支付的人民币金额
		TradeNo:         tradeNo,
		PaymentMethod:   req.PaymentMethod,
		PaymentProvider: model.PaymentProviderLakala,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	if err := order.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	// 返回二维码和订单信息给前端展示（url 使用 lakalaQRCodePath，与充值共用同一二维码页面）
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"url":     lakalaQRCodePath,
		"data": map[string]string{
			"code":     code,
			"trade_no": tradeNo,
			"amount":   lakalaPayMoney.Round(2).StringFixed(2),
		},
	})
}

// SubscriptionLakalaNotify 处理订阅拉卡拉支付结果回调（异步通知）。
//
// 完整流程：
//  1. 读取回调请求体
//  2. 获取拉卡拉平台公钥配置
//  3. 验证回调签名（防止伪造通知）
//  4. 解析回调数据，提取订单号和交易状态
//  5. 检查交易状态是否为支付成功
//  6. 根据订单号查询本地订阅订单，校验支付方式匹配
//  7. 校验回调金额与订单金额一致
//  8. 加锁后执行订阅激活流程
//
// 注意：拉卡拉可能对同一订单发送多次通知，通过订单锁机制保证幂等性。
func SubscriptionLakalaNotify(c *gin.Context) {
	// 读取回调请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "bad request"})
		return
	}

	// 获取配置并校验公钥证书是否存在
	config := getLakalaOptionConfig()
	if config.PublicCert == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "FAIL", "message": "invalid signature"})
		return
	}

	// 使用平台公钥验证回调签名
	if err := lakala.VerifyNotify(config.PublicCert, c.GetHeader("Authorization"), string(body)); err != nil {
		common.SysLog(fmt.Sprintf("subscription lakala notify verify failed: %v", err))
		c.JSON(http.StatusUnauthorized, gin.H{"code": "FAIL", "message": "invalid signature"})
		return
	}

	// 解析回调数据，提取订单信息（复用充值拉卡拉的解析逻辑）
	data, err := parseLakalaNotify(body)
	if err != nil {
		common.SysLog(fmt.Sprintf("subscription lakala notify parse failed: %v, body=%s", err, string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "invalid payload"})
		return
	}

	// 非成功状态的通知直接返回成功，避免拉卡拉重复推送
	if !isLakalaTradeSuccess(data) {
		common.SysLog(fmt.Sprintf("subscription lakala notify ignored non-success status, trade_no=%s, body=%s", data.tradeNo(), string(body)))
		c.JSON(http.StatusOK, gin.H{"code": "0000", "message": "success"})
		return
	}

	// 提取订单号并查询本地订阅订单
	tradeNo := data.tradeNo()
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil || order.PaymentProvider != model.PaymentProviderLakala {
		common.SysLog(fmt.Sprintf("subscription lakala notify order not found or provider mismatch, trade_no=%s, body=%s", tradeNo, string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "order not found"})
		return
	}

	// 校验回调金额与订单金额一致（防止金额篡改）
	expectedAmount := decimal.NewFromFloat(order.OriginalMoney).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if data.amountInCents() != expectedAmount {
		common.SysLog(fmt.Sprintf("subscription lakala notify amount mismatch, trade_no=%s, notify_amount=%d, expected_amount=%d, body=%s", tradeNo, data.amountInCents(), expectedAmount, string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "amount mismatch"})
		return
	}

	// 加锁防止同一订单并发处理
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	// 执行订阅激活流程：CompleteSubscriptionOrder 会创建 UserSubscription 并更新用户额度
	if err := model.CompleteSubscriptionOrder(tradeNo, common.GetJsonString(data), order.PaymentMethod, model.PaymentProviderLakala); err != nil {
		common.SysLog(fmt.Sprintf("subscription lakala notify complete order failed: %v, trade_no=%s", err, tradeNo))
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "complete failed"})
		return
	}

	// 返回成功响应，拉卡拉收到后会停止重复通知
	c.JSON(http.StatusOK, gin.H{"code": "0000", "message": "success"})
}

// GetSubscriptionLakalaStatus 供二维码页面轮询当前用户的拉卡拉订阅订单状态。
// 前端会定时调用此接口检查用户是否已完成扫码支付。
func GetSubscriptionLakalaStatus(c *gin.Context) {
	// 获取并校验trade_no参数
	tradeNo := strings.TrimSpace(c.Query("trade_no"))
	if tradeNo == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单号不能为空"})
		return
	}

	// 根据trade_no和用户ID查询订阅订单
	var order model.SubscriptionOrder
	err := model.DB.
		Where("trade_no = ? AND user_id = ? AND payment_provider = ?",
			tradeNo,
			c.GetInt("id"),
			model.PaymentProviderLakala,
		).
		First(&order).Error
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单不存在"})
		return
	}

	// 返回订单状态和是否已支付的标志
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"trade_no": order.TradeNo,
			"status":   order.Status,
			"paid":     order.Status == common.TopUpStatusSuccess,
		},
	})
}
