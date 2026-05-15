// Package service 提供加密货币账单的生成与对账逻辑。
// 核心流程：
//  1. 从本地 crypto_transactions 表提取已成功的链上交易，逐一通过 RPC 验证并生成账单记录与对账记录
//  2. 批量入库账单记录，并对账写入 payment_bill_reconcile
package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

// 加密货币账单相关常量
const (
	cryptoBillChannelType = "crypto"                                                             // 账单渠道类型标识
	cryptoTransferTopic   = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" // ERC20 Transfer 事件签名哈希（keccak256("Transfer(address,address,uint256)")）
)

// CryptoBillRunResult 账单执行结果，返回给前端展示
type CryptoBillRunResult struct {
	BillDate        string                           `json:"bill_date"`                  // 账单日期
	InsertedRows    int                              `json:"inserted_rows"`              // 入库的账单记录条数
	ReconcileResult *CryptoTradeBillReconcileSummary `json:"reconcile_result,omitempty"` // 对账结果汇总
	Message         string                           `json:"message,omitempty"`          // 附加信息
}

// CryptoTradeBillReconcileSummary 对账结果汇总统计
type CryptoTradeBillReconcileSummary struct {
	TotalCount    int64 `json:"total_count"`    // 对账总条数
	MatchedCount  int64 `json:"matched_count"`  // 匹配成功条数
	AbnormalCount int64 `json:"abnormal_count"` // 异常条数（金额不匹配/状态不一致/本地未找到等）
}

// 以下为 JSON-RPC 调用相关的请求/响应数据结构
type cryptoBillRPCError struct {
	Code    int    `json:"code"`    // RPC 错误码
	Message string `json:"message"` // RPC 错误信息
}

type cryptoBillRPCResponse struct {
	JSONRPC string              `json:"jsonrpc"`         // JSON-RPC 版本号
	ID      int                 `json:"id"`              // 请求 ID
	Result  json.RawMessage     `json:"result"`          // 调用结果（原始 JSON，按方法动态解析）
	Error   *cryptoBillRPCError `json:"error,omitempty"` // 错误信息
}

type cryptoBillRPCBlock struct {
	Number    string `json:"number"`    // 区块号（十六进制字符串）
	Timestamp string `json:"timestamp"` // 区块时间戳（十六进制字符串，Unix 秒）
}

type cryptoBillRPCLog struct {
	Address         string   `json:"address"`         // 日志来源合约地址
	Topics          []string `json:"topics"`          // 事件主题数组（索引参数）
	Data            string   `json:"data"`            // 事件数据（非索引参数）
	BlockNumber     string   `json:"blockNumber"`     // 所在区块号
	TransactionHash string   `json:"transactionHash"` // 交易哈希
	Removed         bool     `json:"removed"`         // 是否因链重组被移除
}

type cryptoBillRPCReceipt struct {
	TransactionHash string             `json:"transactionHash"` // 交易哈希
	Status          string             `json:"status"`          // 交易状态："0x1" 成功，"0x0" 失败
	To              string             `json:"to"`              // 交易目标地址
	BlockNumber     string             `json:"blockNumber"`     // 所在区块号
	Logs            []cryptoBillRPCLog `json:"logs"`            // 交易产生的日志列表
}

// chainTransferEvent 解析后的链上转账事件
type chainTransferEvent struct {
	TxHash        string   // 交易哈希
	BlockNumber   uint64   // 区块号
	From          string   // 发送方地址
	To            string   // 接收方地址
	Value         *big.Int // 转账金额（最小单位，未除精度）
	BlockTime     int64    // 区块时间戳
	TokenContract string   // 代币合约地址
}

// cryptoBillChainRuntime 链运行时上下文，缓存链配置和当前区块号，避免重复加载
type cryptoBillChainRuntime struct {
	Config       *model.CryptoChainConfig // 链配置
	CurrentBlock uint64                   // 当前最新区块号
}

// getCryptoBillTaskTriggerTimeUTC 从环境变量读取定时任务触发时间（UTC）
// 返回 (hour, minute, ok)，ok 为 false 表示未配置或格式错误
func getCryptoBillTaskTriggerTimeUTC() (int, int, bool) {
	timeText := strings.TrimSpace(os.Getenv(cryptoBillTaskTimeEnv))
	if timeText == "" {
		return 0, 0, false
	}
	parts := strings.Split(timeText, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	hour, errHour := strconv.Atoi(strings.TrimSpace(parts[0]))
	minute, errMinute := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}

// RunCryptoBillWorkflow 执行单日加密货币账单的完整工作流：
//  1. 从本地 crypto_transactions 表提取已成功的交易，逐一通过 RPC 验证并生成账单与对账记录
//  2. 批量入库账单记录，并对账写入 payment_bill_reconcile
func RunCryptoBillWorkflow(billDate string) (*CryptoBillRunResult, error) {
	// 解析账单日期，计算当天的 Unix 时间范围
	date, err := time.Parse("2006-01-02", strings.TrimSpace(billDate))
	if err != nil {
		return nil, fmt.Errorf("日期格式错误: %s", billDate)
	}
	startTime := date.Unix()
	endTime := date.AddDate(0, 0, 1).Unix()

	// 第一步：从本地已成功的 crypto 交易生成账单记录
	records, reconciles, err := buildCryptoBillFromLocalTransactions(billDate, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// 第二步：批量入库账单记录
	inserted, err := model.BatchInsertPaymentBillRecords(records)
	if err != nil {
		return nil, fmt.Errorf("批量入库失败: %w", err)
	}

	// 第三步：重新查询刚入库的记录，建立 txHash → record 映射，回填 BillRecordId
	rows, err := model.GetPaymentBillRecordsByChannelAndBillDateRange(model.PaymentChannelTypeCrypto, billDate, billDate)
	if err != nil {
		return nil, err
	}
	rowByTxHash := make(map[string]*model.PaymentBillRecord) // 交易哈希 → 账单记录映射
	// 建立交易哈希 → 账单记录映射，用于后续对账
	for _, row := range rows {
		if row == nil || strings.TrimSpace(row.ChannelTradeNo) == "" {
			continue
		}
		rowByTxHash[normalizeCryptoBillHash(row.ChannelTradeNo)] = row
	}
	// 回填对账记录的 BillRecordId
	for _, reconcile := range reconciles {
		if reconcile == nil || strings.TrimSpace(reconcile.ChannelTradeNo) == "" {
			continue
		}
		row := rowByTxHash[normalizeCryptoBillHash(reconcile.ChannelTradeNo)]
		if row == nil {
			continue
		}
		reconcile.BillRecordId = row.Id
	}

	// 第四步：对账记录写入（Upsert，避免重复）
	if _, err := model.UpsertPaymentBillReconciles(reconciles); err != nil {
		return nil, err
	}

	// 第五步：汇总对账结果
	summary := summarizeCryptoBillReconciles(reconciles)
	return &CryptoBillRunResult{
		BillDate:        billDate,
		InsertedRows:    int(inserted),
		ReconcileResult: summary,
	}, nil
}

// buildCryptoBillFromLocalTransactions 从本地 crypto_transactions 表中提取指定日期范围内已成功的交易，
// 逐一通过链上 RPC 验证，生成账单记录（PaymentBillRecord）和对账记录（PaymentBillReconcile）。
func buildCryptoBillFromLocalTransactions(billDate string, startTime, endTime int64) ([]*model.PaymentBillRecord, []*model.PaymentBillReconcile, error) {
	// 查询指定日期范围内状态为 success 的加密货币交易
	var cryptoTxs []model.CryptoTransaction
	if err := model.DB.
		Where("status = ? AND complete_time >= ? AND complete_time < ?", model.CryptoTransactionStatusSuccess, startTime, endTime).
		Order("complete_time asc, id asc").
		Find(&cryptoTxs).Error; err != nil {
		return nil, nil, err
	}

	records := make([]*model.PaymentBillRecord, 0, len(cryptoTxs))       // 账单记录列表
	reconciles := make([]*model.PaymentBillReconcile, 0, len(cryptoTxs)) // 对账记录列表
	runtimeCache := make(map[string]*cryptoBillChainRuntime)             // 链配置缓存，减少数据库查询
	blockTimeCache := make(map[string]int64)                             // 区块时间缓存，减少 RPC 调用

	// 遍历所有交易，生成账单记录和对账记录
	for i := range cryptoTxs {
		cryptoTx := &cryptoTxs[i]

		// 为每条本地交易创建对账记录（初始状态为"渠道未找到"）
		reconcile := buildCryptoLocalReconcile(billDate, cryptoTx)
		reconciles = append(reconciles, reconcile)

		// 跳过没有 tx_hash 的记录
		if cryptoTx.TxHash == nil || strings.TrimSpace(*cryptoTx.TxHash) == "" {
			reconcile.ReconcileReason = model.PaymentReconcileReasonChannelNotFound
			reconcile.Remark = "crypto transaction has no tx_hash"
			continue
		}

		// 加载链配置（带缓存）
		runtime, err := getCryptoBillChainRuntime(cryptoTx, runtimeCache)
		if err != nil {
			reconcile.ReconcileReason = model.PaymentReconcileReasonUnsupportedBillRow
			reconcile.Remark = err.Error()
			continue
		}

		// 通过 RPC 验证链上交易（收据 + Transfer 事件）
		event, err := verifyCryptoBillTransaction(runtime, cryptoTx, blockTimeCache)
		if err != nil {
			reconcile.ReconcileReason = cryptoBillReconcileReasonFromError(err)
			reconcile.Remark = err.Error()
			continue
		}

		// 验证通过，构建账单记录并对账匹配
		record := buildCryptoBillRecordFromChain(billDate, len(records)+1, runtime.Config, event, cryptoTx.TradeNo)
		records = append(records, record)
		applyCryptoChannelMatch(reconcile, record)
	}

	return records, reconciles, nil
}

// getCryptoBillChainRuntime 获取链运行时上下文（带缓存）。
// 根据 crypto_transaction 中的 chain_id + token_symbol 查找对应链配置，
// 并获取链上当前最新区块号。
func getCryptoBillChainRuntime(cryptoTx *model.CryptoTransaction, cache map[string]*cryptoBillChainRuntime) (*cryptoBillChainRuntime, error) {
	if cryptoTx == nil {
		return nil, fmt.Errorf("crypto transaction is nil")
	}
	key := fmt.Sprintf("%d:%s", cryptoTx.ChainId, strings.ToLower(strings.TrimSpace(cryptoTx.TokenSymbol)))
	if runtime := cache[key]; runtime != nil {
		return runtime, nil
	}
	cfg, err := model.GetCryptoChainByID(cryptoTx.ChainId, cryptoTx.TokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("load chain config failed: %w", err)
	}
	if strings.TrimSpace(cfg.RPCURL) == "" {
		return nil, fmt.Errorf("chain %s has empty RPCURL", cfg.Network)
	}
	currentBlock, err := getCryptoBillCurrentBlockNumber(cfg.RPCURL)
	if err != nil {
		return nil, err
	}
	runtime := &cryptoBillChainRuntime{Config: cfg, CurrentBlock: currentBlock}
	cache[key] = runtime
	return runtime, nil
}

// verifyCryptoBillTransaction 通过链上 RPC 验证一笔加密货币交易。
// 验证步骤：
//  1. 获取交易收据（receipt），确认交易存在
//  2. 检查交易状态为成功（0x1）
//  3. 检查目标地址为代币合约
//  4. 检查区块确认数满足最小要求
//  5. 遍历日志查找匹配的 Transfer 事件，校验收款地址和金额
func verifyCryptoBillTransaction(runtime *cryptoBillChainRuntime, cryptoTx *model.CryptoTransaction, blockTimeCache map[string]int64) (*chainTransferEvent, error) {
	if runtime == nil || runtime.Config == nil || cryptoTx == nil || cryptoTx.TxHash == nil {
		return nil, fmt.Errorf("missing crypto transaction data")
	}

	// 1. 获取交易收据
	txHash := normalizeCryptoBillHash(*cryptoTx.TxHash)
	receipt, err := getCryptoBillTransactionReceipt(runtime.Config.RPCURL, txHash)
	if err != nil {
		return nil, err
	}
	if receipt == nil {
		return nil, fmt.Errorf("链上交易不存在")
	}

	// 2. 检查交易状态
	if !strings.EqualFold(receipt.Status, "0x1") {
		return nil, fmt.Errorf("链上交易未成功")
	}

	// 3. 检查目标地址为代币合约
	if !strings.EqualFold(receipt.To, cryptoTx.TokenContract) && !strings.EqualFold(receipt.To, runtime.Config.TokenContract) {
		return nil, fmt.Errorf("链上交易目标不是代币合约")
	}

	// 4. 解析区块号并检查确认数
	blockNumber, err := parseCryptoBillHexUint64(receipt.BlockNumber)
	if err != nil || blockNumber == 0 {
		return nil, fmt.Errorf("链上交易区块号无效")
	}
	confirmations := 0
	if runtime.CurrentBlock >= blockNumber {
		confirmations = int(runtime.CurrentBlock - blockNumber + 1)
	}
	if confirmations < runtime.Config.MinConfirmations {
		return nil, fmt.Errorf("链上确认数不足，当前 %d，需要 %d", confirmations, runtime.Config.MinConfirmations)
	}

	// 5. 获取区块时间戳（带缓存）
	blockTime, err := getCryptoBillCachedBlockTimestamp(runtime.Config.RPCURL, blockNumber, blockTimeCache)
	if err != nil {
		return nil, err
	}

	// 6. 校验本地记录的支付金额
	requiredAmount, err := decimal.NewFromString(cryptoTx.UsdtAmount)
	if err != nil || requiredAmount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("本地 crypto 支付金额无效")
	}
	requiredBaseUnits := requiredAmount.Mul(decimal.New(1, int32(runtime.Config.TokenDecimals))).BigInt()
	receiver := normalizeCryptoBillAddressText(cryptoTx.ReceiverAddress)

	// 7. 遍历交易日志，查找匹配的 Transfer 事件
	for _, logItem := range receipt.Logs {
		from, to, value, ok := parseCryptoBillTransferLog(runtime.Config.TokenContract, logItem)
		if !ok {
			continue
		}
		// 校验收款地址
		if !strings.EqualFold(to, receiver) {
			continue
		}
		// 校验转账金额
		if value.Cmp(requiredBaseUnits) < 0 {
			return nil, fmt.Errorf("链上转账金额不足")
		}
		return &chainTransferEvent{
			TxHash:        txHash,
			BlockNumber:   blockNumber,
			From:          from,
			To:            to,
			Value:         value,
			BlockTime:     blockTime,
			TokenContract: logItem.Address,
		}, nil
	}

	return nil, fmt.Errorf("未找到匹配的代币转账事件")
}

// buildCryptoLocalReconcile 根据本地 crypto_transaction 记录创建对账记录（初始状态为"渠道未找到"）。
// 根据交易关联的业务类型（订阅/充值/纯加密）设置 LocalType 和 LocalId。
func buildCryptoLocalReconcile(billDate string, cryptoTx *model.CryptoTransaction) *model.PaymentBillReconcile {
	// 确定本地业务类型：优先订阅，其次充值，最后纯加密货币
	localType := "crypto"
	localID := cryptoTx.Id
	if cryptoTx.SubscriptionOrderId > 0 {
		localType = "subscription"
		localID = cryptoTx.SubscriptionOrderId
	} else if cryptoTx.TopUpId > 0 {
		localType = "topup"
		localID = cryptoTx.TopUpId
	}

	localAmount, _ := strconv.ParseFloat(strings.TrimSpace(cryptoTx.UsdtAmount), 64)
	record := &model.PaymentBillReconcile{
		ChannelType:        model.PaymentChannelTypeCrypto,
		ReconcileKey:       reconcileKeyForLocalRecord(billDate, localType, localID), // 唯一对账键
		RecordSource:       "local",                                                  // 记录来源：本地
		BillDate:           billDate,
		MerchantTradeNo:    cryptoTx.TradeNo,
		LocalType:          localType,
		LocalId:            localID,
		LocalTradeNo:       cryptoTx.TradeNo,
		LocalPaymentMethod: model.PaymentChannelTypeCrypto,
		LocalStatus:        cryptoTx.Status,
		LocalAmount:        localAmount,
		LocalCreateTime:    cryptoTx.CreateTime,
		LocalCompleteTime:  cryptoTx.CompleteTime,
		LocalCurrency:      cryptoTx.TokenSymbol,
		ReconcileStatus:    model.PaymentReconcileStatusAbnormal,
		ReconcileReason:    model.PaymentReconcileReasonChannelNotFound,
		Remark:             "local successful crypto transaction not verified on chain",
	}
	if cryptoTx.TxHash != nil {
		record.ChannelTradeNo = normalizeCryptoBillHash(*cryptoTx.TxHash)
	}
	return record
}

// applyCryptoChannelMatch 将渠道账单记录的信息填充到对账记录中，并判断对账结果。
// 判断逻辑：
//   - 金额不一致 → 异常（金额不匹配）
//   - 状态不一致 → 异常（状态不匹配）
//   - 金额和状态均一致 → 匹配成功
func applyCryptoChannelMatch(reconcile *model.PaymentBillReconcile, row *model.PaymentBillRecord) {
	if reconcile == nil || row == nil {
		return
	}

	// 填充渠道侧信息
	amountText, channelAmount := parseCryptoBillAmount(row)
	reconcile.BillDate = row.BillDate
	reconcile.TradeTime = row.TradeTime
	reconcile.ChannelTradeNo = row.ChannelTradeNo
	reconcile.MerchantTradeNo = row.MerchantTradeNo
	reconcile.TradeType = row.TradeType
	reconcile.ChannelStatus = row.TradeStatus
	reconcile.ChannelAmount = amountText
	reconcile.ChannelCurrency = row.Currency

	// 金额比较
	if !amountsEqual(channelAmount, reconcile.LocalAmount) {
		reconcile.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		reconcile.ReconcileReason = model.PaymentReconcileReasonAmountMismatch
		reconcile.Remark = fmt.Sprintf("crypto_amount=%s local_amount=%.6f", amountText, reconcile.LocalAmount)
		return
	}

	// 状态比较
	if normalizeLocalStatus(reconcile.LocalStatus) != "success" || normalizeLocalStatus(row.TradeStatus) != "success" {
		reconcile.ReconcileStatus = model.PaymentReconcileStatusAbnormal
		reconcile.ReconcileReason = model.PaymentReconcileReasonStatusMismatch
		reconcile.Remark = fmt.Sprintf("crypto_status=%s local_status=%s", row.TradeStatus, reconcile.LocalStatus)
		return
	}

	// 匹配成功
	reconcile.ReconcileStatus = model.PaymentReconcileStatusMatched
	reconcile.ReconcileReason = model.PaymentReconcileReasonMatched
	reconcile.Remark = "matched by local tx_hash and on-chain receipt"
}

// summarizeCryptoBillReconciles 汇总对账结果：统计总数、匹配数、异常数
func summarizeCryptoBillReconciles(rows []*model.PaymentBillReconcile) *CryptoTradeBillReconcileSummary {
	summary := &CryptoTradeBillReconcileSummary{}
	for _, row := range rows {
		if row == nil {
			continue
		}
		summary.TotalCount++
		if row.ReconcileStatus == model.PaymentReconcileStatusMatched {
			summary.MatchedCount++
		} else {
			summary.AbnormalCount++
		}
	}
	return summary
}

// cryptoBillReconcileReasonFromError 根据错误信息推断对账异常原因
func cryptoBillReconcileReasonFromError(err error) string {
	if err == nil {
		return model.PaymentReconcileReasonChannelNotFound
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "金额"):
		return model.PaymentReconcileReasonAmountMismatch
	case strings.Contains(message, "未成功"), strings.Contains(message, "确认数"):
		return model.PaymentReconcileReasonStatusMismatch
	default:
		return model.PaymentReconcileReasonChannelNotFound
	}
}

// parseCryptoBillAmount 从账单记录中解析金额，依次尝试 OrderAmount 和 TotalAmount 字段
func parseCryptoBillAmount(row *model.PaymentBillRecord) (string, float64) {
	candidates := []string{strings.TrimSpace(row.OrderAmount), strings.TrimSpace(row.TotalAmount)}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		value, err := strconv.ParseFloat(candidate, 64)
		if err == nil {
			return candidate, value
		}
	}
	return "", 0
}

// getCryptoBillTransactionReceipt 通过 RPC eth_getTransactionReceipt 获取交易收据
func getCryptoBillTransactionReceipt(rpcURL string, txHash string) (*cryptoBillRPCReceipt, error) {
	var receipt cryptoBillRPCReceipt
	if err := callCryptoBillRPC(rpcURL, "eth_getTransactionReceipt", []any{txHash}, &receipt); err != nil {
		return nil, err
	}
	if strings.TrimSpace(receipt.TransactionHash) == "" {
		return nil, nil
	}
	return &receipt, nil
}

// getCryptoBillCachedBlockTimestamp 获取区块时间戳（带缓存），减少重复 RPC 调用
func getCryptoBillCachedBlockTimestamp(rpcURL string, blockNumber uint64, cache map[string]int64) (int64, error) {
	key := fmt.Sprintf("%s:%d", rpcURL, blockNumber)
	if value, ok := cache[key]; ok {
		return value, nil
	}
	value, err := getCryptoBillBlockTimestamp(rpcURL, blockNumber)
	if err != nil {
		return 0, err
	}
	cache[key] = value
	return value, nil
}

// parseCryptoBillTransferLog 解析 ERC20 Transfer 事件日志。
// 返回 (from, to, value, ok)：
//   - from: 代币发送方地址
//   - to: 代币接收方地址
//   - value: 转账金额（最小单位，big.Int）
//   - ok: 是否解析成功（验证合约地址匹配、topic 匹配、地址有效）
func parseCryptoBillTransferLog(tokenContract string, logItem cryptoBillRPCLog) (string, string, *big.Int, bool) {
	if !strings.EqualFold(logItem.Address, tokenContract) {
		return "", "", nil, false
	}
	if len(logItem.Topics) < 3 || !strings.EqualFold(logItem.Topics[0], cryptoTransferTopic) {
		return "", "", nil, false
	}
	from := cryptoBillTopicToAddress(logItem.Topics[1]) // topics[1] = from（indexed）
	to := cryptoBillTopicToAddress(logItem.Topics[2])   // topics[2] = to（indexed）
	if from == "" || to == "" {
		return "", "", nil, false
	}
	value, err := parseCryptoBillHexBigInt(logItem.Data) // data = value（uint256）
	if err != nil {
		return "", "", nil, false
	}
	return from, to, value, true
}

// callCryptoBillRPC 通用的 JSON-RPC 调用封装。
// 构造 JSON-RPC 请求、发送 HTTP POST、解析响应、处理错误。
func callCryptoBillRPC(rpcURL string, method string, params []any, result any) error {
	payload, err := common.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, rpcURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "new-api-crypto-bill/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("链 RPC 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("链 RPC HTTP 状态异常: %d %s", resp.StatusCode, string(body))
	}

	var rpcResp cryptoBillRPCResponse
	if err := common.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("链 RPC 响应解析失败: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("链 RPC 返回错误(%d): %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if result == nil {
		return nil
	}
	if len(rpcResp.Result) == 0 || strings.TrimSpace(string(rpcResp.Result)) == "null" {
		return fmt.Errorf("链 RPC 响应 result 为空: method=%s", method)
	}
	return common.Unmarshal(rpcResp.Result, result)
}

// getCryptoBillCurrentBlockNumber 通过 eth_blockNumber 获取当前最新区块号
func getCryptoBillCurrentBlockNumber(rpcURL string) (uint64, error) {
	var hexBlock string
	if err := callCryptoBillRPC(rpcURL, "eth_blockNumber", []any{}, &hexBlock); err != nil {
		return 0, err
	}
	return parseCryptoBillHexUint64(hexBlock)
}

// getCryptoBillBlockTimestamp 通过 eth_getBlockByNumber 获取指定区块的时间戳
func getCryptoBillBlockTimestamp(rpcURL string, blockNumber uint64) (int64, error) {
	var block cryptoBillRPCBlock
	if err := callCryptoBillRPC(rpcURL, "eth_getBlockByNumber", []any{formatCryptoBillHexUint64(blockNumber), false}, &block); err != nil {
		return 0, err
	}
	ts, err := parseCryptoBillHexUint64(block.Timestamp)
	return int64(ts), err
}

// ============================================================================
// 地址/哈希/数值格式转换工具函数
// ============================================================================

// normalizeCryptoBillAddressText 标准化地址文本（小写+去空格，不做长度校验）
func normalizeCryptoBillAddressText(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

// normalizeCryptoBillHash 标准化哈希（小写+去空格）
func normalizeCryptoBillHash(hash string) string {
	return strings.ToLower(strings.TrimSpace(hash))
}

// cryptoBillTopicToAddress 从 EVM 事件 topic 中提取地址（取右侧 20 字节）
func cryptoBillTopicToAddress(topic string) string {
	value := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(topic)), "0x")
	if len(value) < 40 {
		return ""
	}
	return "0x" + value[len(value)-40:]
}

// formatCryptoBillHexUint64 将 uint64 格式化为 0x 开头的十六进制字符串
func formatCryptoBillHexUint64(value uint64) string {
	return fmt.Sprintf("0x%x", value)
}

// parseCryptoBillHexUint64 将 0x 开头的十六进制字符串解析为 uint64
func parseCryptoBillHexUint64(value string) (uint64, error) {
	value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "0x")
	if value == "" {
		return 0, fmt.Errorf("empty hex value")
	}
	return strconv.ParseUint(value, 16, 64)
}

// parseCryptoBillHexBigInt 将 0x 开头的十六进制字符串解析为 big.Int（用于大额代币转账金额）
func parseCryptoBillHexBigInt(value string) (*big.Int, error) {
	value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "0x")
	if value == "" {
		return big.NewInt(0), nil
	}
	n := new(big.Int)
	if _, ok := n.SetString(value, 16); !ok {
		return nil, fmt.Errorf("invalid hex bigint")
	}
	return n, nil
}

// buildCryptoBillRecordFromChain 根据链上转账事件构建一条账单记录。
// 计算行哈希（用于去重）、格式化时间（CST 时区）、换算代币金额。
func buildCryptoBillRecordFromChain(billDate string, rowIndex int, chain *model.CryptoChainConfig, ev *chainTransferEvent, merchantTradeNo string) *model.PaymentBillRecord {
	tradeTime := time.Unix(ev.BlockTime, 0).In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	valueStr := formatTokenAmount(ev.Value, chain.TokenDecimals)

	// 计算行哈希，用于后续去重判断
	rowHashRaw := fmt.Sprintf("%s|%s|%s|%s|%s|%s", cryptoBillChannelType, chain.Network, chain.TokenSymbol, ev.TxHash, ev.To, ev.Value.String())
	hash := sha256.Sum256([]byte(rowHashRaw))

	return &model.PaymentBillRecord{
		ChannelType:     model.PaymentChannelTypeCrypto,
		BillDate:        billDate,
		RowIndex:        rowIndex,
		RowHash:         hex.EncodeToString(hash[:]),
		TradeTime:       tradeTime,
		ChannelTradeNo:  normalizeCryptoBillHash(ev.TxHash),
		MerchantTradeNo: strings.TrimSpace(merchantTradeNo),
		PayerID:         ev.From,
		TradeType:       chain.Network,
		TradeStatus:     "success",
		Currency:        chain.TokenSymbol,
		TotalAmount:     valueStr,
		OrderAmount:     valueStr,
		CreatedAt:       common.GetTimestamp(),
	}
}

// formatTokenAmount 将最小单位的代币金额转换为人类可读格式（除以 10^decimals）
func formatTokenAmount(value *big.Int, decimals int) string {
	if value == nil {
		value = big.NewInt(0)
	}
	if decimals <= 0 {
		return value.String()
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	quotient := new(big.Int).Div(value, divisor)
	remainder := new(big.Int).Mod(value, divisor)
	return fmt.Sprintf("%s.%0*s", quotient.String(), decimals, remainder.String())
}
