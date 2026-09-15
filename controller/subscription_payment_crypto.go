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

	// 校验套餐是否启用（同时校验是否对当前 provider_id 可见，防跨站点订阅）
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
	if plan.PriceAmount <= 0 {
		common.ApiErrorMsg(c, "套餐金额必须大于 0")
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
	// Token decimals are used as an int32 scale by decimal.Round and as a
	// base-unit multiplier during chain verification. Reject malformed config
	// rather than allowing an overflow or an unusable payment order.
	if chainCfg.TokenDecimals < 0 || chainCfg.TokenDecimals > 36 || chainCfg.MinConfirmations < 0 {
		common.ApiErrorMsg(c, "该网络代币精度配置错误")
		return
	}
	if !isValidCryptoAddress(chainCfg.TokenContract) || !isValidCryptoAddress(chainCfg.ReceiverAddress) {
		common.ApiErrorMsg(c, "该网络加密货币地址配置错误")
		return
	}
	rate, err := parseCryptoUSDtoTokenRate()
	if err != nil {
		common.ApiErrorMsg(c, "加密货币汇率未配置或无效")
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

	now := time.Now().Unix()
	var order *model.SubscriptionOrder
	var payAmount string
	var accountingPrice decimal.Decimal
	// 事务中同时创建 subscription_orders 和 crypto_transactions 记录
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		// 创建订阅订单
		order = &model.SubscriptionOrder{
			UserId: userId,  // 用户 ID
			PlanId: plan.Id, // 订阅套餐 ID
			// 订单归属服务商（0=主站），完成订单时据此给服务商 owner 结算订阅收入。
			ProviderId: c.GetInt("provider_id"),
			// Money is the normalized USD accounting amount. The converted token
			// amount is persisted separately in CryptoTransaction.UsdtAmount.
			Money:           plan.PriceAmount,
			Currency:        currencySymbol,      // 用户币种符号（$ / ￥）
			OriginalMoney:   plan.PriceAmount,    // 套餐原价
			TradeNo:         referenceId,         // 订单号（唯一）
			PaymentMethod:   PaymentMethodCrypto, // 支付方式：crypto
			PaymentProvider: model.PaymentProviderCrypto,
			CreateTime:      now,                       // 创建时间
			Status:          common.TopUpStatusPending, // 订单状态：待支付
		}
		if err := model.CreateSubscriptionOrderTx(tx, order); err != nil {
			return err
		}
		// CreateSubscriptionOrderTx locks the authoritative plan and normalizes
		// Money for crypto orders. Recompute the token amount from that locked
		// value so a concurrent catalog edit cannot change the quoted amount.
		accountingPrice = decimal.NewFromFloat(order.Money)
		if !accountingPrice.GreaterThan(decimal.Zero) {
			return fmt.Errorf("invalid subscription price")
		}
		// Keep the monetary snapshot coherent with the authoritative plan row.
		// The token quantity remains in CryptoTransaction.UsdtAmount.
		order.OriginalMoney = accountingPrice.InexactFloat64()
		if err := tx.Model(&model.SubscriptionOrder{}).
			Where("id = ?", order.Id).
			Update("original_money", order.OriginalMoney).Error; err != nil {
			return err
		}
		tokenAmount := accountingPrice.Mul(rate)
		payAmount = tokenAmount.Round(int32(chainCfg.TokenDecimals)).StringFixed(int32(chainCfg.TokenDecimals))
		parsedPayAmount, parseErr := decimal.NewFromString(payAmount)
		if parseErr != nil || !parsedPayAmount.GreaterThan(decimal.Zero) {
			return fmt.Errorf("crypto payment amount is too small")
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
			TokenDecimals:       chainCfg.TokenDecimals,                           // 精度快照，确认时不可被配置变更影响
			ReceiverAddress:     normalizeCryptoAddress(chainCfg.ReceiverAddress), // 收款地址
			UsdtAmount:          payAmount,                                        // 应支付的代币金额
			Status:              model.CryptoTransactionStatusPending,             // 交易状态：待确认
			CreateTime:          now,                                              // 记录创建时间
		}
		return tx.Create(&cryptoTx).Error
	})
	if err != nil {
		respondSubscriptionCreateError(c, err, "创建订单失败")
		return
	}

	// 返回该链的支付信息，供前端构造钱包交易
	common.ApiSuccess(c, gin.H{
		"trade_no":         referenceId,                      // 订单号，后续确认时回传
		"payment_method":   PaymentMethodCrypto,              // 支付方式标识
		"network":          chainCfg.Network,                 // 链网络名称
		"chain_id":         chainCfg.ChainID,                 // 链 ID（EIP-155），钱包切换网络时需要
		"token":            chainCfg.TokenSymbol,             // 代币符号（USDT）
		"token_contract":   chainCfg.TokenContract,           // 代币合约地址，transfer 的目标地址
		"to_address":       chainCfg.ReceiverAddress,         // 收款地址
		"pay_amount":       payAmount,                        // 需支付的代币金额（按链精度四舍五入）
		"decimals":         chainCfg.TokenDecimals,           // 代币精度，构造 transfer 时用于换算
		"confirmations":    chainCfg.MinConfirmations,        // 最小确认数要求
		"amount":           accountingPrice.InexactFloat64(), // 套餐美元价格
		"display_currency": plan.Currency,                    // 展示币种代码（USD / CNY）
		"display_symbol":   currencySymbol,                   // 展示币种符号（$ / ￥）
	})
}

// SubscriptionCryptoConfirmRequest 加密货币订阅充值确认请求
type SubscriptionCryptoConfirmRequest struct {
	TradeNo string `json:"trade_no"` // 订单号
	TxHash  string `json:"tx_hash"`  // 链上交易哈希
}

// applySubscriptionCryptoTokenDecimalsSnapshot applies the token precision
// captured on the checkout transaction.  Zero is a valid ERC-20 precision for
// some tokens, so it must not be treated as "missing" and replaced with the
// administrator's current chain configuration.  A pending order can outlive a
// chain-config edit; confirmation must therefore always use the persisted
// snapshot (including zero) to interpret UsdtAmount and transfer logs.
func applySubscriptionCryptoTokenDecimalsSnapshot(chain *cryptoChainConfig, snapshot int) error {
	if chain == nil {
		return fmt.Errorf("订单代币精度错误")
	}
	if snapshot < 0 || snapshot > 36 {
		return fmt.Errorf("订单代币精度错误")
	}
	chain.TokenDecimals = snapshot
	return nil
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
	if !isValidCryptoTxHash(txHash) {
		common.ApiErrorMsg(c, "交易哈希格式错误")
		return
	}

	// 查询加密货币交易记录，拿到 chain_id + token_symbol 反查链配置
	cryptoTx, err := model.GetCryptoTransactionByTradeNo(tradeNo)
	if err != nil || cryptoTx == nil {
		common.ApiErrorMsg(c, "加密货币交易记录不存在")
		return
	}
	// A subscription confirmation is a user-scoped operation.  Without this
	// check, anyone who learns another user's pending trade number could submit
	// a valid transfer and cause that user's order to be fulfilled (or consume
	// their pending inventory), while the caller receives no entitlement.
	if cryptoTx.UserId != c.GetInt("id") || cryptoTx.SubscriptionOrderId <= 0 || cryptoTx.TopUpId != 0 {
		common.ApiErrorMsg(c, "无权确认该订阅订单")
		return
	}
	// Validate the linked order before touching the chain.  The crypto row is
	// intentionally not sufficient authorization: a malformed/legacy row must
	// never be used to complete a non-crypto order or an order owned by another
	// user.
	order := model.GetSubscriptionOrderByTradeNo(tradeNo)
	if order == nil || order.Id != cryptoTx.SubscriptionOrderId || order.UserId != cryptoTx.UserId {
		common.ApiErrorMsg(c, "订阅订单不存在")
		return
	}
	if order.PaymentMethod != model.PaymentMethodCrypto ||
		(strings.TrimSpace(order.PaymentProvider) != "" && order.PaymentProvider != model.PaymentProviderCrypto) {
		common.ApiErrorMsg(c, "订单支付方式不匹配")
		return
	}
	if order.Status != common.TopUpStatusPending && order.Status != common.TopUpStatusSuccess {
		common.ApiErrorMsg(c, "订单状态错误")
		return
	}
	boundHash := ""
	if cryptoTx.TxHash != nil {
		boundHash = strings.ToLower(strings.TrimSpace(*cryptoTx.TxHash))
	}
	if boundHash != "" {
		// A completed row is a safe idempotent retry.  Do not call the RPC again:
		// chain settings may have been rotated since checkout, and the persisted
		// transfer metadata is already the verified evidence for this order.
		if boundHash != txHash || cryptoTx.Status != model.CryptoTransactionStatusSuccess {
			common.ApiErrorMsg(c, "交易哈希与订单已绑定记录不一致")
			return
		}
		LockOrder(tradeNo)
		defer UnlockOrder(tradeNo)
		if err := model.CompleteSubscriptionCryptoOrder(tradeNo, txHash, txHash,
			cryptoTx.PayerAddress, cryptoTx.BlockNumber, cryptoTx.Confirmations); err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{
			"trade_no":      tradeNo,
			"tx_hash":       txHash,
			"from_address":  cryptoTx.PayerAddress,
			"to_address":    cryptoTx.ReceiverAddress,
			"block_number":  cryptoTx.BlockNumber,
			"confirmations": cryptoTx.Confirmations,
		})
		return
	}
	if cryptoTx.Status != model.CryptoTransactionStatusPending {
		common.ApiErrorMsg(c, "加密货币交易状态错误")
		return
	}
	if order.Status != common.TopUpStatusPending {
		// An unbound crypto row must correspond to the pending checkout.  A
		// success order with no hash indicates an inconsistent/legacy record and
		// must not be used as a new authorization to attach a transfer.
		common.ApiErrorMsg(c, "订单状态错误")
		return
	}
	// 防止同一笔链上交易被其他订单重复使用。当前订单重复确认时，
	// 自身 hash 已在上面的幂等分支处理，不能被全局预检误判为重复支付。
	if model.CryptoTxHashExistsForOtherTradeNo(txHash, tradeNo) {
		common.ApiErrorMsg(c, "交易哈希已被使用")
		return
	}

	chain, err := getCryptoChainByID(cryptoTx.ChainId, cryptoTx.TokenSymbol)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	// The payment row is a checkout-time snapshot.  Administrators may rotate
	// the active contract later, but an in-flight order must still be verified
	// against the contract the user was shown when paying.  RPC/confirmation
	// policy remains configurable on the current chain row.
	if !isValidCryptoAddress(cryptoTx.TokenContract) {
		common.ApiErrorMsg(c, "订单代币合约地址错误")
		return
	}
	chain.TokenContract = normalizeCryptoAddress(cryptoTx.TokenContract)
	if err := applySubscriptionCryptoTokenDecimalsSnapshot(chain, cryptoTx.TokenDecimals); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !isValidCryptoAddress(cryptoTx.ReceiverAddress) {
		common.ApiErrorMsg(c, "订单收款地址错误")
		return
	}

	requiredAmount, err := decimal.NewFromString(cryptoTx.UsdtAmount)
	if err != nil || requiredAmount.LessThanOrEqual(decimal.Zero) {
		common.ApiErrorMsg(c, "订单金额错误")
		return
	}
	// 链上验证代币转账
	// The transfer must have been mined after this specific order was created.
	// Passing zero here used to disable the timestamp guard for subscription
	// payments, allowing an old (otherwise valid) transfer to be replayed against
	// a newly-created order whenever the hash was not already present locally.
	transfer, err := verifyCryptoTransfer(chain, txHash, normalizeCryptoAddress(cryptoTx.ReceiverAddress), requiredAmount, cryptoTx.CreateTime)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	// 加锁防止并发确认
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)
	// 完成订阅订单、绑定链上交易哈希必须在同一数据库事务内提交。否则
	// 订单已激活而 crypto_transactions 尚未标记成功时，进程崩溃/重试可能
	// 将另一笔交易绑定到同一订阅订单。
	if err := model.CompleteSubscriptionCryptoOrder(
		tradeNo, txHash, txHash, transfer.From, transfer.BlockNumber, transfer.Confirmations,
	); err != nil {
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
