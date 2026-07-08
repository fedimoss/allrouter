// Package controller 提供加密货币订阅支付相关的 HTTP 接口处理。
// 包含下单（SubscriptionRequestCryptoPay）和确认（SubscriptionRequestCryptoConfirm）两个核心接口。
package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"
)

// SubscriptionCryptoPayRequest 加密货币订阅充值下单请求
type SubscriptionCryptoPayRequest struct {
	PlanId      int    `json:"plan_id"`      // 要购买的订阅套餐 ID
	Network     string `json:"network"`      // 链网络名称，如 Sepolia / BSC / Polygon
	TokenSymbol string `json:"token_symbol"` // 代币符号，不传默认 USDT
}

// SubscriptionRequestCryptoPay 加密货币订阅充值下单
func SubscriptionRequestCryptoPay(c *gin.Context) {
	// 解析请求参数
	var req SubscriptionCryptoPayRequest

	// 校验请求参数
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	// 默认值：network 不传默认 Sepolia，token_symbol 不传默认 USDT
	if strings.TrimSpace(req.Network) == "" {
		req.Network = "Sepolia"
	}
	if strings.TrimSpace(req.TokenSymbol) == "" {
		req.TokenSymbol = "USDT"
	}

	// 获取订阅套餐详情
	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 校验套餐是否启用
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}

	if plan.AllowPurchase != 1 {
		common.ApiErrorMsg(c, "该套餐暂不允许订阅")
		return
	}

	// 根据前端传的 network + token_symbol 查找链配置
	chainCfg, err := model.GetCryptoChainByNetwork(req.Network, req.TokenSymbol)
	if err != nil {
		common.ApiErrorMsg(c, "不支持的链网络或代币")
		return
	}
	// 校验收款地址是否已配置
	if strings.TrimSpace(chainCfg.ReceiverAddress) == "" {
		common.ApiErrorMsg(c, "该网络的收款地址未配置")
		return
	}

	// 校验用户是否存在
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

	// 校验用户是否已购买该套餐
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

	// 以下为已注释的展示币种自动推断逻辑（保留供后续参考）：
	// 根据用户时区确定展示币种，未设置时区默认 "America/New_York" → USD
	// displayCurrency := model.GetDisplayCurrencyInfoByTimezone(user.Timezone).Currency
	// if displayCurrency == "" {
	// 	displayCurrency = model.GetDisplayCurrencyInfoByTimezone("America/New_York").Currency
	// }

	// 币种符号映射：USD → $，CNY → ￥
	currencySymbol := "$"
	if strings.EqualFold(plan.Currency, "CNY") {
		currencySymbol = "￥"
	}

	// 生成唯一的订单号，格式：sub-crypto-ref-{用户ID}-{毫秒时间戳}-{4位随机字符串}
	reference := fmt.Sprintf("sub-crypto-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "sub_ref_" + common.Sha1([]byte(reference))

	// 美元价格按汇率换算为代币金额（plan.PriceAmount 为美元价格，1 USDT ≈ 1 USD）
	usdtAmount := decimal.NewFromFloat(plan.PriceAmount).Mul(decimal.RequireFromString(getCryptoUSDtoTokenRate()))
	// 按链配置的精度四舍五入，字符串存储避免浮点精度问题
	payAmount := usdtAmount.Round(int32(chainCfg.TokenDecimals)).StringFixed(int32(chainCfg.TokenDecimals))

	now := time.Now().Unix()
	var order *model.SubscriptionOrder
	// 事务中同时创建 subscription_orders 和 crypto_transactions 记录
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// 创建订阅订单
		order = &model.SubscriptionOrder{
			UserId:        userId,                      // 用户 ID
			PlanId:        plan.Id,                     // 订阅套餐 ID
			Money:         usdtAmount.InexactFloat64(), // 换算后的代币金额（USDT）
			Currency:      currencySymbol,              // 用户币种符号（$ / ￥）
			OriginalMoney: plan.PriceAmount,            // 套餐原价
			TradeNo:       referenceId,                 // 订单号（唯一）
			PaymentMethod: PaymentMethodCrypto,         // 支付方式：crypto
			CreateTime:    now,                         // 创建时间
			Status:        common.TopUpStatusPending,   // 订单状态：待支付
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		// 创建加密货币交易记录（存储链上支付参数，确认时回查）
		cryptoTx := model.CryptoTransaction{
			TopUpId:             0,                                                // 非充值支付
			SubscriptionOrderId: order.Id,                                         // 关联的订阅订单 ID
			UserId:              userId,                                           // 用户 ID
			TradeNo:             referenceId,                                      // 订单号
			ChainId:             chainCfg.ChainID,                                 // 链 ID（confirm 时据此反查配置）
			TokenSymbol:         chainCfg.TokenSymbol,                             // 代币符号
			TokenContract:       normalizeCryptoAddress(chainCfg.TokenContract),   // 代币合约地址
			ReceiverAddress:     normalizeCryptoAddress(chainCfg.ReceiverAddress), // 收款地址
			UsdtAmount:          payAmount,                                        // 应支付的代币金额
			Status:              model.CryptoTransactionStatusPending,             // 交易状态：待确认
			CreateTime:          now,                                              // 记录创建时间
		}
		return tx.Create(&cryptoTx).Error
	})
	if err != nil {
		common.ApiErrorMsg(c, "创建订单失败")
		return
	}

	// 返回该链的支付信息，供前端构造钱包交易
	common.ApiSuccess(c, gin.H{
		"trade_no":         referenceId,               // 订单号，后续确认时回传
		"payment_method":   PaymentMethodCrypto,       // 支付方式标识
		"network":          chainCfg.Network,          // 链网络名称
		"chain_id":         chainCfg.ChainID,          // 链 ID（EIP-155），钱包切换网络时需要
		"token":            chainCfg.TokenSymbol,      // 代币符号（USDT）
		"token_contract":   chainCfg.TokenContract,    // 代币合约地址，transfer 的目标地址
		"to_address":       chainCfg.ReceiverAddress,  // 收款地址
		"pay_amount":       payAmount,                 // 需支付的代币金额（按链精度四舍五入）
		"decimals":         chainCfg.TokenDecimals,    // 代币精度，构造 transfer 时用于换算
		"confirmations":    chainCfg.MinConfirmations, // 最小确认数要求
		"amount":           plan.PriceAmount,          // 套餐美元价格
		"display_currency": plan.Currency,             // 展示币种代码（USD / CNY）
		"display_symbol":   currencySymbol,            // 展示币种符号（$ / ￥）
	})
}

// SubscriptionCryptoConfirmRequest 加密货币订阅充值确认请求
type SubscriptionCryptoConfirmRequest struct {
	TradeNo string `json:"trade_no"` // 订单号
	TxHash  string `json:"tx_hash"`  // 链上交易哈希
}

// SubscriptionRequestCryptoConfirm 加密货币订阅充值确认（用户提交交易哈希后调用）
// 流程：
//  1. 校验订单存在、支付方式匹配、状态为待支付
//  2. 查询 crypto_transactions 记录，通过 chain_id + token_symbol 反查链配置
//  3. 使用该链的 RPC 节点验证链上交易
//  4. 验证通过后完成订阅订单、激活订阅
func SubscriptionRequestCryptoConfirm(c *gin.Context) {
	var req SubscriptionCryptoConfirmRequest
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

	// 防止同一笔链上交易重复使用
	if model.CryptoTxHashExists(txHash) {
		common.ApiErrorMsg(c, "交易哈希已被使用")
		return
	}

	// 查询加密货币交易记录，拿到 chain_id + token_symbol 反查链配置
	cryptoTx, err := model.GetCryptoTransactionByTradeNo(tradeNo)
	if err != nil || cryptoTx == nil {
		common.ApiErrorMsg(c, "加密货币交易记录不存在")
		return
	}

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
	// 链上验证代币转账
	transfer, err := verifyCryptoTransfer(chain, txHash, normalizeCryptoAddress(cryptoTx.ReceiverAddress), requiredAmount, 0)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	// 加锁防止并发确认
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)
	// 完成订阅订单，激活用户订阅
	if err := model.CompleteSubscriptionOrder(tradeNo, txHash, PaymentMethodCrypto); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.CompleteCryptoTransaction(tradeNo, txHash, transfer.From, transfer.BlockNumber, transfer.Confirmations); err != nil {
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
