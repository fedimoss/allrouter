package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/lakala"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// lakalaPreorderURL 是拉卡拉测试环境的预下单地址，测试中可被替换为本地 mock 地址。
var lakalaPreorderURL = "https://test.wsmsd.cn/sit/api/v3/labs/trans/preorder"

const (
	// lakalaQRCodePath 是前端展示拉卡拉支付二维码的页面路由。
	lakalaQRCodePath = "/payment/lakala/qrcode"
	// lakalaTermNo 是拉卡拉分配的终端号。
	lakalaTermNo = "D9261076"
	// lakalaTopUpNotifyURLPath 是拉卡拉充值支付结果异步回调路径。
	lakalaTopUpNotifyURLPath = "/api/user/lakala/notify"
	// lakalaTopUpSubject 是拉卡拉支付订单的商品标题。
	lakalaTopUpSubject = "充值"
	// lakalaAccountType 是拉卡拉支付账户类型，目前固定为支付宝。
	lakalaAccountType = "ALIPAY"
)

// lakalaOptionConfig 保存拉卡拉接口需要的 options 配置。
// 这些配置从系统设置（common.OptionMap）中读取，由管理员在后台配置。
type lakalaOptionConfig struct {
	AppID           string // 拉卡拉分配的接入应用ID
	SerialNo        string // 拉卡拉证书序列号
	PrivateKey      string // 商户私钥（PEM格式），用于对请求签名
	PublicCert      string // 拉卡拉平台公钥证书（PEM格式），用于验签回调
	MerchantNo      string // 拉卡拉分配的商户号
	CallbackAddress string // 拉卡拉回调地址域名，例如 https://example.com
}

// getLakalaOptionConfig 从系统全局配置中读取拉卡拉相关参数。
// 读取时加读锁保护，保证并发安全。
func getLakalaOptionConfig() lakalaOptionConfig {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return lakalaOptionConfig{
		AppID:           strings.TrimSpace(common.OptionMap["LakalaAppID"]),
		SerialNo:        strings.TrimSpace(common.OptionMap["LakalaSerialNo"]),
		PrivateKey:      strings.TrimSpace(common.OptionMap["LakalaPrivateKey"]),
		PublicCert:      strings.TrimSpace(common.OptionMap["LakalaPublicCert"]),
		MerchantNo:      strings.TrimSpace(common.OptionMap["LakalaMerchantNo"]),
		CallbackAddress: strings.TrimRight(strings.TrimSpace(common.OptionMap["LakalaCallbackAddress"]), "/"),
	}
}

func buildLakalaNotifyURL(callbackAddress string, path string) string {
	return strings.TrimRight(strings.TrimSpace(callbackAddress), "/") + path
}

// isLakalaConfigured 检查拉卡拉支付是否已完整配置。
// 只要有一项配置为空，就认为未配置，前端会据此隐藏拉卡拉支付选项。
func isLakalaConfigured() bool {
	config := getLakalaOptionConfig()
	return config.AppID != "" &&
		config.SerialNo != "" &&
		config.PrivateKey != "" &&
		config.PublicCert != "" &&
		config.MerchantNo != "" &&
		config.CallbackAddress != ""
}

// requestLakalaWechatPay 处理额度充值的拉卡拉扫码支付预下单。
//
// 参数说明：
//   - c: Gin 上下文，用于获取用户信息和客户端IP
//   - req: 充值请求参数，包含支付方式等信息
//   - amount: 充值额度（美元，整数）
//   - usdMoney: 充值金额对应的美元价值
//   - cnyChargeMoney: 实际支付的人民币金额（元）
//   - tradeNo: 预生成的唯一交易流水号
//
// 流程：
//  1. 校验拉卡拉配置是否完整
//  2. 将人民币金额转换为分（拉卡拉以分为单位）
//  3. 构造预下单请求体
//  4. 使用商户私钥对请求签名
//  5. 调用拉卡拉预下单接口
//  6. 验签响应后提取支付二维码
//  7. 创建本地充值订单记录
//  8. 将二维码和订单信息返回给前端
func requestLakalaWechatPay(c *gin.Context, req EpayRequest, amount int64, usdMoney float64, cnyChargeMoney float64, tradeNo string) {
	// 获取拉卡拉配置并校验完整性
	config := getLakalaOptionConfig()
	if config.AppID == "" || config.SerialNo == "" || config.PrivateKey == "" || config.PublicCert == "" || config.MerchantNo == "" || config.CallbackAddress == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "当前管理员未配置拉卡拉支付信息"})
		return
	}

	// 将人民币金额元转分，拉卡拉金额单位为分
	totalAmount := decimal.NewFromFloat(cnyChargeMoney).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if totalAmount <= 0 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	// lakalaPayMoney 保存人民币元金额，用于前端展示和后续订单记录
	lakalaPayMoney := decimal.NewFromFloat(cnyChargeMoney)
	lakalaNotifyURL := buildLakalaNotifyURL(config.CallbackAddress, lakalaTopUpNotifyURLPath)

	// 构造拉卡拉预下单请求体
	requestBody := map[string]any{
		"req_time": time.Now().Format("20060102150405"), // 请求时间，格式 yyyyMMddHHmmss
		"version":  "3.0",                               // 接口版本号
		"req_data": map[string]any{
			"merchant_no":  config.MerchantNo, // 商户号
			"term_no":      lakalaTermNo,      // 终端号
			"out_trade_no": tradeNo,           // 商户订单号（唯一）
			"account_type": lakalaAccountType, // 支付账户类型
			"trans_type":   "41",              // 交易类型：41=扫码支付
			"total_amount": totalAmount,       // 交易金额（单位：分）
			"notify_url":   lakalaNotifyURL,   // 支付结果异步通知地址
			"location_info": map[string]any{
				"request_ip": c.ClientIP(), // 客户端IP地址
			},
			"subject": lakalaTopUpSubject,           // 商品标题
			"remark":  strconv.Itoa(c.GetInt("id")), // 备注：用户ID
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
	httpReq.Header.Set("Authorization", signResult.Authorization) // 签名生成的Authorization头

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

	// 检查HTTP状态码，非2xx视为失败
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉卡拉预下单失败"})
		return
	}

	// 使用拉卡拉平台公钥验证响应签名，防止数据被篡改
	if err := verifyLakalaResponse(config.PublicCert, resp.Header, respBody); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉卡拉响应验签失败"})
		return
	}

	// 从响应中提取支付二维码字符串
	code, err := extractLakalaQRCode(respBody)
	if err != nil {
		common.SysLog(fmt.Sprintf("拉卡拉预下单未返回二维码: %v, response=%s", err, string(respBody)))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": err.Error()})
		return
	}

	// 创建本地充值订单记录
	topUp := &model.TopUp{
		ProviderId:      c.GetInt("provider_id"),         // 服务商ID
		UserId:          c.GetInt("id"),                  // 用户ID
		Amount:          amount,                          // 充值额度（美元）
		Money:           usdMoney,                        // 美元金额
		TradeNo:         tradeNo,                         // 交易流水号
		PaymentMethod:   req.PaymentMethod,               // 支付方式
		PaymentProvider: model.PaymentProviderLakala,     // 支付服务商：拉卡拉
		BizType:         model.TopUpBizTypePayment,       // 业务类型：充值支付
		CreateTime:      time.Now().Unix(),               // 创建时间戳
		Status:          common.TopUpStatusPending,       // 初始状态：待支付
		Currency:        "¥",                             // 币种：人民币
		OriginalMoney:   lakalaPayMoney.InexactFloat64(), // 原始支付金额（元）
	}
	if err := topUp.Insert(); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	// 返回二维码和订单信息给前端展示
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"url":     lakalaQRCodePath, // 前端二维码页面路由
		"data": map[string]string{
			"code":     code,                                   // 支付二维码字符串
			"trade_no": tradeNo,                                // 交易流水号
			"amount":   lakalaPayMoney.Round(2).StringFixed(2), // 支付金额，保留两位小数
		},
	})
}

// GetLakalaTopUpStatus 供二维码页面轮询当前用户的拉卡拉充值订单状态。
// 前端会定时调用此接口检查用户是否已完成扫码支付。
func GetLakalaTopUpStatus(c *gin.Context) {
	// 获取并校验trade_no参数
	tradeNo := strings.TrimSpace(c.Query("trade_no"))
	if tradeNo == "" {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单号不能为空"})
		return
	}

	// 根据trade_no和用户ID查询订单
	var topUp model.TopUp
	err := model.DB.
		Where("trade_no = ? AND user_id = ? AND payment_method = ? AND payment_provider = ? AND biz_type = ?",
			tradeNo,
			c.GetInt("id"),
			model.PaymentProviderLakala,
			model.PaymentProviderLakala,
			model.TopUpBizTypePayment,
		).
		First(&topUp).Error
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "订单不存在"})
		return
	}

	// 返回订单状态和是否已支付的标志
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"trade_no": topUp.TradeNo,                             // 交易流水号
			"status":   topUp.Status,                              // 订单状态码
			"paid":     topUp.Status == common.TopUpStatusSuccess, // 是否已支付成功
		},
	})
}

// verifyLakalaResponse 使用拉卡拉平台公钥验证接口响应的签名。
//
// 参数：
//   - publicCert: 拉卡拉平台公钥证书（PEM格式）
//   - header: 响应头，包含签名相关的头字段
//   - body: 响应体明文
//
// 验证方法：从响应头提取签名所需的各个字段，调用拉卡拉SDK进行验签。
func verifyLakalaResponse(publicCert string, header http.Header, body []byte) error {
	// 从响应头获取签名值
	signature := header.Get("Lklapi-Signature")
	if strings.TrimSpace(signature) == "" {
		return fmt.Errorf("missing signature")
	}

	// 组装验签所需的请求头字段
	headers := lakala.VerifySignHeaders{
		AppID:     header.Get("Lklapi-Appid"),     // 应用ID
		SerialNo:  header.Get("Lklapi-Serial"),    // 证书序列号
		TimeStamp: header.Get("Lklapi-Timestamp"), // 时间戳
		NonceStr:  header.Get("Lklapi-Nonce"),     // 随机字符串
		Signature: signature,                      // 签名值
		TraceID:   header.Get("Lklapi-Traceid"),   // 链路追踪ID
	}

	// 调用SDK验证签名
	return lakala.Verify(publicCert, headers, string(body))
}

// extractLakalaQRCode 从拉卡拉预下单响应中提取支付二维码字符串。
//
// 成功时返回二维码字符串，失败时返回包含上游错误信息的error。
func extractLakalaQRCode(body []byte) (string, error) {
	// 定义响应结构体，只声明需要的字段
	var payload struct {
		Code     string `json:"code"` // 上游业务状态码
		Msg      string `json:"msg"`  // 上游业务消息
		RespData struct {
			AccRespFields struct {
				Code string `json:"code"` // 支付二维码字符串
			} `json:"acc_resp_fields"`
		} `json:"resp_data"`
	}

	// 反序列化响应体
	if err := common.Unmarshal(body, &payload); err != nil {
		return "", err
	}

	// 提取二维码字符串
	code := strings.TrimSpace(payload.RespData.AccRespFields.Code)
	if code == "" {
		// 二维码为空时，返回上游的错误信息以便排查
		upstreamCode := strings.TrimSpace(payload.Code)
		upstreamMsg := strings.TrimSpace(payload.Msg)
		if upstreamMsg != "" {
			if upstreamCode != "" {
				return "", fmt.Errorf("拉卡拉预下单失败：%s（%s）", upstreamMsg, upstreamCode)
			}
			return "", fmt.Errorf("拉卡拉预下单失败：%s", upstreamMsg)
		}
		return "", fmt.Errorf("拉卡拉未返回支付二维码")
	}
	return code, nil
}

// lakalaNotifyPayload 是拉卡拉异步回调的完整请求体。
// 回调数据可能放在 req_data 或 resp_data 中，解析时会优先尝试 req_data。
type lakalaNotifyPayload struct {
	ReqData  lakalaNotifyData `json:"req_data"`  // 请求数据（优先使用）
	RespData lakalaNotifyData `json:"resp_data"` // 响应数据（备选）
}

// lakalaNotifyData 是拉卡拉回调中的订单数据。
// 字段使用 snake_case 和 camelCase 双写以兼容拉卡拉不同版本的回调格式。
type lakalaNotifyData struct {
	OutTradeNo         string       `json:"out_trade_no"`      // 商户订单号（snake_case）
	OutTradeNoCamel    string       `json:"outTradeNo"`        // 商户订单号（camelCase）
	MerchantOrderNo    string       `json:"merchantOrderNo"`   // 拉卡拉订单号（camelCase）
	MerchantOrderNoAlt string       `json:"merchant_order_no"` // 拉卡拉订单号（snake_case）
	TotalAmount        lakalaAmount `json:"total_amount"`      // 交易金额（snake_case，单位：分）
	TotalAmountCamel   lakalaAmount `json:"totalAmount"`       // 交易金额（camelCase，单位：分）
	Amount             lakalaAmount `json:"amount"`            // 金额（备选字段，单位：分）
	TradeStatus        string       `json:"trade_status"`      // 交易状态（snake_case）
	Status             string       `json:"status"`            // 状态（简写版）
	TradeState         string       `json:"trade_state"`       // 交易状态（备选）
	PayStatus          string       `json:"payStatus"`         // 支付状态（camelCase）
	PayStatusAlt       string       `json:"pay_status"`        // 支付状态（snake_case）
}

// lakalaAmount 是拉卡拉金额类型。
// 拉卡拉接口中金额字段可能是数字或字符串，使用自定义类型兼容两种格式。
type lakalaAmount int64

// UnmarshalJSON 实现 json.Unmarshaler 接口，兼容数字和字符串两种JSON格式。
// 拉卡拉回调中金额字段有时是JSON数字，有时是JSON字符串，此方法统一处理。
func (amount *lakalaAmount) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	// 处理null、空字符串、空引号的情况
	if raw == "" || raw == "null" || raw == `""` {
		*amount = 0
		return nil
	}
	// 去除首尾引号（字符串格式的情况）
	raw = strings.Trim(raw, `"`)
	if raw == "" {
		*amount = 0
		return nil
	}
	// 解析为int64
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid lakala amount %q: %w", raw, err)
	}
	*amount = lakalaAmount(value)
	return nil
}

// Int64 将lakalaAmount转换为int64，供外部使用。
func (amount lakalaAmount) Int64() int64 {
	return int64(amount)
}

// tradeNo 从回调数据中提取商户订单号。
// 按优先级依次尝试多个字段，返回第一个非空值。
func (data *lakalaNotifyData) tradeNo() string {
	if data == nil {
		return ""
	}
	// 按优先级依次尝试：OutTradeNo -> OutTradeNoCamel -> MerchantOrderNo -> MerchantOrderNoAlt
	for _, value := range []string{data.OutTradeNo, data.OutTradeNoCamel, data.MerchantOrderNo, data.MerchantOrderNoAlt} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// amountInCents 从回调数据中提取交易金额（单位：分）。
// 按优先级依次尝试 TotalAmount -> TotalAmountCamel -> Amount，返回第一个非零值。
func (data *lakalaNotifyData) amountInCents() int64 {
	if data == nil {
		return 0
	}
	if data.TotalAmount != 0 {
		return data.TotalAmount.Int64()
	}
	if data.TotalAmountCamel != 0 {
		return data.TotalAmountCamel.Int64()
	}
	return data.Amount.Int64()
}

// LakalaNotify 处理拉卡拉支付结果回调（异步通知）。
//
// 完整流程：
//  1. 读取回调请求体
//  2. 获取拉卡拉平台公钥配置
//  3. 验证回调签名（防止伪造通知）
//  4. 解析回调数据，提取订单号和交易状态
//  5. 检查交易状态是否为支付成功
//  6. 根据订单号查询本地订单，校验支付方式匹配
//  7. 校验回调金额与订单金额一致
//  8. 加锁后执行充值入账流程
//
// 注意：拉卡拉可能对同一订单发送多次通知，通过订单锁机制保证幂等性。
func LakalaNotify(c *gin.Context) {
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
		common.SysLog(fmt.Sprintf("lakala notify verify failed: %v", err))
		c.JSON(http.StatusUnauthorized, gin.H{"code": "FAIL", "message": "invalid signature"})
		return
	}

	// 解析回调数据，提取订单信息
	data, err := parseLakalaNotify(body)
	if err != nil {
		common.SysLog(fmt.Sprintf("lakala notify parse failed: %v, body=%s", err, string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "invalid payload"})
		return
	}

	// 非成功状态的通知直接返回成功，避免拉卡拉重复推送
	if !isLakalaTradeSuccess(data) {
		common.SysLog(fmt.Sprintf("lakala notify ignored non-success status, trade_no=%s, body=%s", data.tradeNo(), string(body)))
		c.JSON(http.StatusOK, gin.H{"code": "0000", "message": "success"})
		return
	}

	// 提取订单号并查询本地订单
	tradeNo := data.tradeNo()
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.PaymentMethod != model.PaymentProviderLakala || topUp.PaymentProvider != model.PaymentProviderLakala {
		common.SysLog(fmt.Sprintf("lakala notify order not found or method mismatch, trade_no=%s, body=%s", tradeNo, string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "order not found"})
		return
	}

	// 校验回调金额与订单金额是否一致（防止金额篡改）
	expectedAmount := decimal.NewFromFloat(topUp.OriginalMoney).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if data.amountInCents() != expectedAmount {
		common.SysLog(fmt.Sprintf("lakala notify amount mismatch, trade_no=%s, notify_amount=%d, expected_amount=%d, body=%s", tradeNo, data.amountInCents(), expectedAmount, string(body)))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "amount mismatch"})
		return
	}

	// 加锁防止同一订单并发处理
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)

	// 执行充值入账
	if err := model.RechargeWechatTopUp(tradeNo, model.PaymentProviderLakala, model.PaymentProviderLakala); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "recharge failed"})
		return
	}

	// 返回成功响应，拉卡拉收到后会停止重复通知
	c.JSON(http.StatusOK, gin.H{"code": "0000", "message": "success"})
}

// parseLakalaNotify 从回调请求体中解析订单数据。
//
// 解析策略：先按嵌套结构（lakalaNotifyPayload）解析，优先取 req_data；
// 如果 req_data 中没有订单号，则取 resp_data；
// 如果两个嵌套结构都没有，则尝试直接将body反序列化为 lakalaNotifyData（扁平结构兼容）。
func parseLakalaNotify(body []byte) (*lakalaNotifyData, error) {
	// 先按嵌套结构解析
	var payload lakalaNotifyPayload
	if err := common.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	// 优先使用 req_data
	data := payload.ReqData
	if data.tradeNo() == "" {
		// req_data为空时尝试 resp_data
		data = payload.RespData
	}
	if data.tradeNo() == "" {
		// 两个嵌套结构都没有订单号，尝试扁平结构反序列化
		if err := common.Unmarshal(body, &data); err != nil {
			return nil, err
		}
	}
	if data.tradeNo() == "" {
		return nil, fmt.Errorf("missing out_trade_no")
	}
	return &data, nil
}

// isLakalaTradeSuccess 判断拉卡拉回调中的交易状态是否为支付成功。
//
// 兼容多种状态字段和多种表示方式（全大写的英文状态码、数字状态码等）。
// 按优先级依次检查：TradeStatus -> Status -> TradeState -> PayStatus -> PayStatusAlt
func isLakalaTradeSuccess(data *lakalaNotifyData) bool {
	if data == nil {
		return false
	}

	// 按优先级依次尝试各状态字段，统一转为大写比较
	status := strings.ToUpper(strings.TrimSpace(data.TradeStatus))
	if status == "" {
		status = strings.ToUpper(strings.TrimSpace(data.Status))
	}
	if status == "" {
		status = strings.ToUpper(strings.TrimSpace(data.TradeState))
	}
	if status == "" {
		status = strings.ToUpper(strings.TrimSpace(data.PayStatus))
	}
	if status == "" {
		status = strings.ToUpper(strings.TrimSpace(data.PayStatusAlt))
	}

	// 匹配所有可能的成功状态值
	switch status {
	case "S", "SUCCESS", "TRADE_SUCCESS", "PAY_SUCCESS", "PAID":
		return true
	default:
		return false
	}
}
