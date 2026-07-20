package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"
)

const (
	// PaymentMethodCrypto 加密货币支付方式标识
	PaymentMethodCrypto = "crypto"

	cryptoDefaultTimezone   = "America/New_York"                                                   // 默认用户时区（用于确定用户币种）
	cryptoTransferTopic     = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" // ERC20 Transfer 事件签名哈希（所有 EVM 链通用）
	cryptoOrderTimeSkewSecs = 600                                                                  // 链上交易时间与订单创建时间的最大允许偏差（秒）
)

// getCryptoUSDtoTokenRate 获取美元到代币的汇率，从 options 表读取
func getCryptoUSDtoTokenRate() string {
	if v, ok := common.OptionMap["CryptoUSDtoTokenRate"]; ok {
		s := strings.TrimSpace(common.Interface2String(v))
		if s != "" {
			return s
		}
	}
	return ""
}

// getCryptoCNYtoTokenRate 获取人民币到代币的汇率，从 options 表读取
func getCryptoCNYtoTokenRate() string {
	if v, ok := common.OptionMap["CryptoCNYtoTokenRate"]; ok {
		s := strings.TrimSpace(common.Interface2String(v))
		if s != "" {
			return s
		}
	}
	return ""
}

// cryptoUsdtToUsd 将 USDT 金额按系统设置的"美元到 USDT 汇率"换算为美元
// 汇率语义：USDT = USD × rate  =>  USD = USDT / rate
// 汇率未设置或非法时按 1:1 估算（USDT ≈ USD），避免阻塞看板聚合
func cryptoUsdtToUsd(usdt float64) float64 {
	rate, err := decimal.NewFromString(getCryptoUSDtoTokenRate())
	if err != nil || !rate.GreaterThan(decimal.Zero) {
		return usdt
	}
	return decimal.NewFromFloat(usdt).Div(rate).InexactFloat64()
}

// cryptoChainConfig 单条链的配置参数
// 一个 struct 对应一组 RPC URL、合约地址、收款地址等，按网络名称索引
type cryptoChainConfig struct {
	Network          string // 网络名称，如 Sepolia / BSC / Polygon（前端传入）
	ChainID          int    // EIP-155 链 ID（Sepolia=11155111, BSC=56）
	TokenSymbol      string // 代币符号，如 USDT
	TokenDecimals    int    // 代币精度（Sepolia MockUSDT=6, BSC USDT=18）
	TokenContract    string // 代币合约地址
	ReceiverAddress  string // 收款钱包地址
	RPCURL           string // 链节点 RPC 地址（含 API Key）
	MinConfirmations int    // 最小链上确认数
}

// toCryptoChainConfig 将 model 层的配置转换为 controller 内部使用的结构体
func toCryptoChainConfig(m *model.CryptoChainConfig) *cryptoChainConfig {
	return &cryptoChainConfig{
		Network:          m.Network,
		ChainID:          m.ChainID,
		TokenSymbol:      m.TokenSymbol,
		TokenDecimals:    m.TokenDecimals,
		TokenContract:    m.TokenContract,
		ReceiverAddress:  m.ReceiverAddress,
		RPCURL:           m.RPCURL,
		MinConfirmations: m.MinConfirmations,
	}
}

// getCryptoChainConfig 从数据库查询链配置（大小写不敏感）
// token 为空时默认 "USDT"
func getCryptoChainConfig(network string, token string) (*cryptoChainConfig, error) {
	m, err := model.GetCryptoChainByNetwork(network, token)
	if err != nil {
		return nil, fmt.Errorf("不支持的网络或代币: %s/%s", network, token)
	}
	return toCryptoChainConfig(m), nil
}

// getCryptoChainByID 从数据库根据链 ID 和代币符号查询链配置（confirm 时使用）
func getCryptoChainByID(chainID int, tokenSymbol string) (*cryptoChainConfig, error) {
	m, err := model.GetCryptoChainByID(chainID, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("不支持的链 ID 或代币: %d/%s", chainID, tokenSymbol)
	}
	return toCryptoChainConfig(m), nil
}

// CryptoPayRequest 加密货币充值请求参数
type CryptoPayRequest struct {
	Amount        int64  `json:"amount"`         // 充值金额（本地币种单位）
	Network       string `json:"network"`        // 链网络名称，如 Sepolia / BSC / Polygon（大小写不敏感）
	PaymentMethod string `json:"payment_method"` // 支付方式，固定为 "crypto"
	Token         string `json:"token"`          // 代币符号，如 USDT / USDC（不传默认 USDT）
}

// CryptoConfirmRequest 加密货币充值确认请求参数
type CryptoConfirmRequest struct {
	TradeNo string `json:"trade_no"` // 订单号
	TxHash  string `json:"tx_hash"`  // 链上交易哈希
}

// cryptoReceipt 链交易回执
type cryptoReceipt struct {
	TransactionHash string      `json:"transactionHash"`
	Status          string      `json:"status"`      // 交易状态（0x1 表示成功）
	To              string      `json:"to"`          // 目标合约地址
	BlockNumber     string      `json:"blockNumber"` // 区块号（十六进制）
	Logs            []cryptoLog `json:"logs"`        // 交易日志
}

// cryptoLog 链交易日志（事件）
type cryptoLog struct {
	Address string   `json:"address"` // 合约地址
	Topics  []string `json:"topics"`  // 事件主题（索引参数）
	Data    string   `json:"data"`    // 事件数据（非索引参数）
}

// cryptoBlock 链区块信息
type cryptoBlock struct {
	Timestamp string `json:"timestamp"` // 区块时间戳（十六进制）
}

// cryptoRPCResponse JSON-RPC 响应结构
type cryptoRPCResponse struct {
	Result json.RawMessage `json:"result"` // 结果数据
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"` // 错误信息
}

// cryptoVerifiedTransfer 已验证的链上转账信息
type cryptoVerifiedTransfer struct {
	From          string   // 转出地址
	To            string   // 收款地址
	AmountBase    *big.Int // 转账金额（最小单位）
	BlockNumber   uint64   // 区块号
	Confirmations int      // 确认数
	BlockTime     int64    // 区块时间戳
}

// normalizeCryptoAddress 标准化链上地址（转小写并去除首尾空格）
func normalizeCryptoAddress(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

// isValidCryptoAddress 校验链上地址格式是否合法
func isValidCryptoAddress(address string) bool {
	address = strings.TrimSpace(address)
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}
	for _, r := range address[2:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// resolveCryptoUserCurrency 根据用户时区解析用户币种（USD/CNY）
func resolveCryptoUserCurrency(user *model.User) string {
	timezone := cryptoDefaultTimezone
	if user != nil && strings.TrimSpace(user.Timezone) != "" {
		timezone = strings.TrimSpace(user.Timezone)
	}
	currency := model.GetCurrencyByTimezone(timezone)
	if strings.TrimSpace(currency) == "" {
		return "USD"
	}
	return strings.ToUpper(strings.TrimSpace(currency))
}

// calcCryptoAmounts 计算加密货币充值金额
// 返回：美元金额、代币金额、币种符号、错误信息
func calcCryptoAmounts(localAmount int64, currency string) (decimal.Decimal, decimal.Decimal, string, error) {
	if localAmount <= 0 {
		return decimal.Zero, decimal.Zero, "", errors.New("invalid amount")
	}
	local := decimal.NewFromInt(localAmount)
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "CNY":
		// 人民币用户：金额 × 汇率 = 代币金额
		usdt := local.Mul(decimal.RequireFromString(getCryptoCNYtoTokenRate()))
		return usdt, usdt, "¥", nil
	case "USD", "":
		// 美元用户：1:1 转换
		usdt := local.Mul(decimal.RequireFromString(getCryptoUSDtoTokenRate()))
		return local, usdt, "$", nil
	default:
		// 其他币种按美元处理
		usdt := local.Mul(decimal.RequireFromString(getCryptoUSDtoTokenRate()))
		return local, usdt, "$", nil
	}
}

// RequestCryptoPay 创建加密货币充值订单
// 流程：
//  1. 解析请求参数（含网络名称）并校验
//  2. 根据网络名称查找链配置
//  3. 校验收款地址和合约地址
//  4. 根据用户时区确定展示币种，计算美元 / 代币金额
//  5. 在事务中创建 top_ups 和 crypto_transactions 记录
//  6. 返回链上支付信息供前端调起钱包
func RequestCryptoPay(c *gin.Context) {
	var req CryptoPayRequest
	// 解析 JSON 请求体
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	// 校验充值金额
	if req.Amount <= 0 {
		common.ApiErrorMsg(c, "充值金额必须大于 0")
		return
	}
	// 校验支付方式（兼容旧前端不传）
	if strings.TrimSpace(req.PaymentMethod) == "" {
		req.PaymentMethod = PaymentMethodCrypto
	}
	if req.PaymentMethod != PaymentMethodCrypto {
		common.ApiErrorMsg(c, "支付方式不支持")
		return
	}
	// 默认网络为 Sepolia（测试环境兼容旧前端不传 network）
	if strings.TrimSpace(req.Network) == "" {
		req.Network = "Sepolia"
	}
	// 根据前端传入的网络名称查找链配置
	chain, err := getCryptoChainConfig(req.Network, req.Token)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	// 校验收款地址是否已配置
	if !isValidCryptoAddress(chain.ReceiverAddress) {
		common.ApiErrorMsg(c, "该网络的加密货币收款地址未配置")
		return
	}
	// 校验代币合约地址是否配置正确
	if !isValidCryptoAddress(chain.TokenContract) {
		common.ApiErrorMsg(c, "该网络的加密货币合约地址配置错误")
		return
	}

	// 从 JWT 会话获取当前用户 ID
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, "用户不存在")
		return
	}

	// 根据用户时区确定展示币种
	currency := resolveCryptoUserCurrency(user)
	// 计算美元金额、代币金额、币种符号
	usdAmount, usdtAmount, symbol, err := calcCryptoAmounts(req.Amount, currency)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 生成唯一订单号：CRYPTO-用户ID-毫秒时间戳-6位随机串
	tradeNo := fmt.Sprintf("CRYPTO-%d-%d-%s", userId, time.Now().UnixMilli(), randstr.String(6))
	// 代币金额按链配置的精度四舍五入，字符串存储避免浮点精度问题
	usdtAmountStr := usdtAmount.Round(int32(chain.TokenDecimals)).StringFixed(int32(chain.TokenDecimals))
	now := time.Now().Unix()

	var topUp model.TopUp
	// 事务中同时创建 top_ups 和 crypto_transactions 两条记录
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// 创建充值订单记录
		topUp = model.TopUp{
			UserId:        userId,                              // 用户 ID
			Amount:        req.Amount,                          // 充值金额（本地币种单位）
			Money:         usdAmount.Round(6).InexactFloat64(), // 折算后的美元金额
			TradeNo:       tradeNo,                             // 订单号（唯一）
			PaymentMethod: PaymentMethodCrypto,                 // 支付方式标识
			BizType:       model.TopUpBizTypePayment,           // 业务类型
			CreateTime:    now,                                 // 创建时间
			Status:        common.TopUpStatusPending,           // 订单状态：待支付
			Currency:      symbol,                              // 展示币种符号
			OriginalMoney: float64(req.Amount),                 // 原始本地币种金额
		}
		if err := tx.Create(&topUp).Error; err != nil {
			return err
		}
		// 创建加密货币交易记录（存储链上支付参数，确认时回查）
		cryptoTx := model.CryptoTransaction{
			TopUpId:         topUp.Id,                                      // 关联的充值订单 ID
			UserId:          userId,                                        // 用户 ID
			TradeNo:         tradeNo,                                       // 订单号
			ChainId:         chain.ChainID,                                 // 链 ID（confirm 时据此反查配置）
			TokenSymbol:     chain.TokenSymbol,                             // 代币符号
			TokenContract:   normalizeCryptoAddress(chain.TokenContract),   // 代币合约地址
			ReceiverAddress: normalizeCryptoAddress(chain.ReceiverAddress), // 收款地址
			UsdtAmount:      usdtAmountStr,                                 // 应支付的代币金额
			Status:          model.CryptoTransactionStatusPending,          // 交易状态：待确认
			CreateTime:      now,                                           // 记录创建时间
		}
		return tx.Create(&cryptoTx).Error
	})
	if err != nil {
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	// 返回链上支付所需信息给前端
	common.ApiSuccess(c, gin.H{
		"trade_no":         tradeNo,                           // 订单号，后续确认时回传
		"payment_method":   PaymentMethodCrypto,               // 支付方式标识
		"network":          chain.Network,                     // 链网络名称
		"chain_id":         chain.ChainID,                     // 链 ID（EIP-155），钱包切换网络时需要
		"token":            chain.TokenSymbol,                 // 代币符号
		"token_contract":   chain.TokenContract,               // 代币合约地址，transfer 的目标地址
		"to_address":       chain.ReceiverAddress,             // 收款地址
		"amount":           req.Amount,                        // 用户请求的充值金额
		"display_currency": currency,                          // 展示币种代码（USD / CNY）
		"display_symbol":   symbol,                            // 展示币种符号（$ / ¥）
		"usd_amount":       usdAmount.Round(6).StringFixed(6), // 折算后的美元金额
		"pay_amount":       usdtAmountStr,                     // 用户实际需支付的代币金额
		"decimals":         chain.TokenDecimals,               // 代币精度，构造 transfer 时用于换算
		"confirmations":    chain.MinConfirmations,            // 最小确认数要求
	})
}

// RequestCryptoConfirm 确认加密货币充值（用户提交交易哈希后调用）
// 流程：
//  1. 校验订单存在、支付方式匹配、状态为待支付
//  2. 查询 crypto_transactions 记录，通过 chain_id 反查链配置
//  3. 使用该链的 RPC 节点验证链上交易
//  4. 验证通过后完成充值入账
func RequestCryptoConfirm(c *gin.Context) {
	var req CryptoConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	tradeNo := strings.TrimSpace(req.TradeNo)
	txHash := strings.ToLower(strings.TrimSpace(req.TxHash))
	if tradeNo == "" || txHash == "" {
		common.ApiErrorMsg(c, "订单号和交易哈希不能为空")
		return
	}
	if !strings.HasPrefix(txHash, "0x") || len(txHash) != 66 {
		common.ApiErrorMsg(c, "交易哈希格式错误")
		return
	}

	userId := c.GetInt("id")
	// 校验订单存在且属于当前用户
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil || topUp.UserId != userId {
		common.ApiErrorMsg(c, "充值订单不存在")
		return
	}
	if topUp.PaymentMethod != PaymentMethodCrypto {
		common.ApiErrorMsg(c, "订单支付方式不匹配")
		return
	}
	if topUp.Status != common.TopUpStatusPending {
		common.ApiErrorMsg(c, "订单状态错误")
		return
	}
	// 获取关联的加密货币交易记录
	cryptoTx, err := model.GetCryptoTransactionByTradeNo(tradeNo)
	if err != nil || cryptoTx == nil {
		common.ApiErrorMsg(c, "加密货币交易记录不存在")
		return
	}
	// 防止同一笔链上交易重复使用
	if model.CryptoTxHashExists(txHash) {
		common.ApiErrorMsg(c, "交易哈希已被使用")
		return
	}

	// 通过链 ID 反查链配置，拿到该链的 RPC URL、合约地址、确认数等
	chain, err := getCryptoChainByID(cryptoTx.ChainId, cryptoTx.TokenSymbol)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	requiredAmount, err := decimal.NewFromString(cryptoTx.UsdtAmount)
	if err != nil || requiredAmount.LessThanOrEqual(decimal.Zero) {
		common.ApiErrorMsg(c, "订单金额错误")
		return
	}
	// 使用该链的配置进行链上验证
	transfer, err := verifyCryptoTransfer(chain, txHash, normalizeCryptoAddress(cryptoTx.ReceiverAddress), requiredAmount, topUp.CreateTime)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	// 加锁防止并发确认同一订单
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)
	if err := model.RechargeCrypto(tradeNo, txHash, transfer.From, transfer.BlockNumber, transfer.Confirmations); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"trade_no":      tradeNo,
		"tx_hash":       txHash,
		"from_address":  transfer.From,
		"to_address":    transfer.To,
		"block_number":  transfer.BlockNumber,
		"confirmations": transfer.Confirmations,
	})
}

// verifyCryptoTransfer 验证链上代币转账是否有效
// 参数 chain 指定使用哪条链的配置（RPC、合约地址、确认数等）
func verifyCryptoTransfer(chain *cryptoChainConfig, txHash string, receiver string, requiredAmount decimal.Decimal, orderCreateTime int64) (*cryptoVerifiedTransfer, error) {
	receipt, err := getCryptoTransactionReceipt(chain, txHash)
	if err != nil {
		return nil, err
	}
	if receipt == nil {
		return nil, errors.New("链上交易不存在")
	}
	if !strings.EqualFold(receipt.Status, "0x1") {
		return nil, errors.New("链上交易未成功")
	}
	// 验证交易目标是该链的代币合约
	if !strings.EqualFold(receipt.To, chain.TokenContract) {
		return nil, errors.New("链上交易目标不是代币合约")
	}
	blockNumber, err := hexToUint64(receipt.BlockNumber)
	if err != nil || blockNumber == 0 {
		return nil, errors.New("链上交易区块号无效")
	}
	// 获取当前区块号，计算确认数
	currentBlock, err := getCryptoCurrentBlockNumber(chain)
	if err != nil {
		return nil, err
	}
	confirmations := 0
	if currentBlock >= blockNumber {
		confirmations = int(currentBlock - blockNumber + 1)
	}
	if confirmations < chain.MinConfirmations {
		return nil, fmt.Errorf("链上确认数不足，当前 %d，需要 %d", confirmations, chain.MinConfirmations)
	}
	// 校验区块时间晚于订单创建时间
	blockTime, err := getCryptoBlockTimestamp(chain, blockNumber)
	if err != nil {
		return nil, err
	}
	if orderCreateTime > 0 && blockTime+cryptoOrderTimeSkewSecs < orderCreateTime {
		return nil, errors.New("链上交易时间早于订单创建时间")
	}

	// 遍历交易日志，查找匹配的 ERC20 Transfer 事件
	requiredBaseUnits := requiredAmount.Mul(decimal.New(1, int32(chain.TokenDecimals))).BigInt()
	for _, logItem := range receipt.Logs {
		from, to, value, ok := parseCryptoTransferLog(chain, logItem)
		if !ok {
			continue
		}
		if !strings.EqualFold(to, receiver) {
			continue
		}
		if value.Cmp(requiredBaseUnits) < 0 {
			return nil, errors.New("链上转账金额不足")
		}
		return &cryptoVerifiedTransfer{
			From:          from,
			To:            to,
			AmountBase:    value,
			BlockNumber:   blockNumber,
			Confirmations: confirmations,
			BlockTime:     blockTime,
		}, nil
	}
	return nil, errors.New("未找到匹配的代币转账事件")
}

// parseCryptoTransferLog 解析链上 ERC20 Transfer 事件日志
func parseCryptoTransferLog(chain *cryptoChainConfig, logItem cryptoLog) (string, string, *big.Int, bool) {
	// 校验日志来源于该链的代币合约
	if !strings.EqualFold(logItem.Address, chain.TokenContract) {
		return "", "", nil, false
	}
	// 校验事件类型为 Transfer(address,address,uint256)
	if len(logItem.Topics) < 3 || !strings.EqualFold(logItem.Topics[0], cryptoTransferTopic) {
		return "", "", nil, false
	}
	// 从 topic[1] 提取转出地址（32 字节十六进制，后 20 字节为地址）
	from, ok := topicAddress(logItem.Topics[1])
	if !ok {
		return "", "", nil, false
	}
	// 从 topic[2] 提取收款地址
	to, ok := topicAddress(logItem.Topics[2])
	if !ok {
		return "", "", nil, false
	}
	// 从 data 字段提取转账金额
	value, ok := hexToBigInt(logItem.Data)
	if !ok {
		return "", "", nil, false
	}
	return from, to, value, true
}

// topicAddress 从事件 topic（32 字节）中提取以太坊地址（后 20 字节）
func topicAddress(topic string) (string, bool) {
	topic = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(topic)), "0x")
	if len(topic) != 64 {
		return "", false
	}
	return "0x" + topic[24:], true
}

// hexToBigInt 将十六进制字符串转换为大整数
func hexToBigInt(hexValue string) (*big.Int, bool) {
	hexValue = strings.TrimPrefix(strings.TrimSpace(hexValue), "0x")
	if hexValue == "" {
		return nil, false
	}
	v := new(big.Int)
	if _, ok := v.SetString(hexValue, 16); !ok {
		return nil, false
	}
	return v, true
}

// hexToUint64 将十六进制字符串转换为 uint64
func hexToUint64(hexValue string) (uint64, error) {
	v, ok := hexToBigInt(hexValue)
	if !ok {
		return 0, errors.New("invalid hex number")
	}
	return v.Uint64(), nil
}

// getCryptoTransactionReceipt 通过 RPC 获取链上交易回执
func getCryptoTransactionReceipt(chain *cryptoChainConfig, txHash string) (*cryptoReceipt, error) {
	var receipt cryptoReceipt
	if err := callCryptoRPC(chain, "eth_getTransactionReceipt", []any{txHash}, &receipt); err != nil {
		return nil, err
	}
	if receipt.TransactionHash == "" {
		return nil, nil
	}
	return &receipt, nil
}

// getCryptoCurrentBlockNumber 获取链上当前区块号
func getCryptoCurrentBlockNumber(chain *cryptoChainConfig) (uint64, error) {
	var blockHex string
	if err := callCryptoRPC(chain, "eth_blockNumber", []any{}, &blockHex); err != nil {
		return 0, err
	}
	return hexToUint64(blockHex)
}

// getCryptoBlockTimestamp 获取链上指定区块的时间戳
func getCryptoBlockTimestamp(chain *cryptoChainConfig, blockNumber uint64) (int64, error) {
	var block cryptoBlock
	blockHex := fmt.Sprintf("0x%x", blockNumber)
	if err := callCryptoRPC(chain, "eth_getBlockByNumber", []any{blockHex, false}, &block); err != nil {
		return 0, err
	}
	ts, err := hexToUint64(block.Timestamp)
	if err != nil {
		return 0, errors.New("链上区块时间无效")
	}
	return int64(ts), nil
}

// callCryptoRPC 调用指定链的 JSON-RPC 接口
// chain.RPCURL 决定了使用哪个链的节点
func callCryptoRPC(chain *cryptoChainConfig, method string, params []any, result any) error {
	payload, err := common.Marshal(gin.H{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(chain.RPCURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return errors.New("查询 RPC 节点失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("RPC 节点响应异常: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var rpcResp cryptoRPCResponse
	if err := common.Unmarshal(body, &rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("RPC 调用错误: %s", rpcResp.Error.Message)
	}
	if len(rpcResp.Result) == 0 || string(rpcResp.Result) == "null" {
		return nil
	}
	return common.Unmarshal(rpcResp.Result, result)
}
