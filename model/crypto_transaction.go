package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Errors returned by crypto payment completion helpers.  Keeping these as
// sentinels lets controllers distinguish a replayed hash or malformed state
// from transient database/RPC failures without relying on localized strings.
var (
	ErrCryptoTransactionStatusInvalid = errors.New("crypto transaction status invalid")
	ErrCryptoTransactionHashUsed      = errors.New("crypto transaction hash already used")
)

// CryptoTransaction 状态常量
const (
	CryptoTransactionStatusPending = "pending" // 待确认（等待用户提交交易哈希）
	CryptoTransactionStatusSuccess = "success" // 已完成（链上确认通过）
	CryptoTransactionStatusFailed  = "failed"  // 失败
)

// CryptoTransaction 加密货币交易记录
// 存储 Web3 链上支付的详细信息，包括链 ID、代币合约、收款地址、交易哈希等。
// 同时服务于充值（TopUp）和订阅支付（Subscription）两种场景，通过 TopUpId / SubscriptionOrderId 区分。
type CryptoTransaction struct {
	Id                  int     `json:"id"`
	TopUpId             int     `json:"topup_id" gorm:"index;not null"`                         // 关联的充值订单 ID（订阅支付时为 0）
	SubscriptionOrderId int     `json:"subscription_order_id" gorm:"index;default:0"`           // 关联的订阅订单 ID（充值支付时为 0）
	UserId              int     `json:"user_id" gorm:"index;not null"`                          // 用户 ID
	TradeNo             string  `json:"trade_no" gorm:"type:varchar(255);uniqueIndex;not null"` // 订单号（唯一索引，下单时生成）
	TxHash              *string `json:"tx_hash,omitempty" gorm:"type:varchar(128);uniqueIndex"` // 链上交易哈希（用户确认后填入，唯一索引防止重复使用）
	ChainId             int     `json:"chain_id" gorm:"index;not null"`                         // 链 ID（如 BSC 主网为 56，Sepolia 为 11155111）
	TokenSymbol         string  `json:"token_symbol" gorm:"type:varchar(20);not null"`          // 代币符号（如 USDT）
	TokenContract       string  `json:"token_contract" gorm:"type:varchar(128);not null"`       // 代币合约地址（transfer 的目标合约）
	// TokenDecimals is snapshotted at checkout.  Contract settings can be
	// edited while an order is pending; without this field a changed decimal
	// scale would reinterpret the same UsdtAmount during confirmation.
	TokenDecimals   int       `json:"token_decimals" gorm:"default:0"`
	ReceiverAddress string    `json:"receiver_address" gorm:"type:varchar(128);not null"` // 收款地址（平台的钱包地址）
	PayerAddress    string    `json:"payer_address" gorm:"type:varchar(128)"`             // 付款地址（链上确认后从 transfer 事件中提取）
	UsdtAmount      string    `json:"usdt_amount" gorm:"type:varchar(64);not null"`       // 应支付的代币金额（字符串存储，避免浮点精度丢失）
	BlockNumber     uint64    `json:"block_number" gorm:"default:0"`                      // 区块号（链上确认后填入）
	Confirmations   int       `json:"confirmations" gorm:"default:0"`                     // 确认数（链上确认后填入）
	Status          string    `json:"status" gorm:"type:varchar(20);index;not null"`      // 状态：pending（待确认）/ success（已完成）/ failed（失败）
	CreateTime      int64     `json:"create_time"`                                        // 创建时间（Unix 时间戳）
	CompleteTime    int64     `json:"complete_time"`                                      // 完成时间（链上确认通过的时间）
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime"`                   // 更新时间（GORM 自动维护）
}

// TableName 返回 CryptoTransaction 对应的数据库表名
func (CryptoTransaction) TableName() string {
	return "crypto_transactions"
}

// normalizeTxHash 标准化交易哈希（转小写并去除首尾空格），用于统一比较和存储
func normalizeTxHash(txHash string) string {
	return strings.ToLower(strings.TrimSpace(txHash))
}

// GetCryptoTransactionByTradeNo 根据订单号查询加密货币交易记录
// 返回 nil 表示未找到（调用方需判断 err == gorm.ErrRecordNotFound）
func GetCryptoTransactionByTradeNo(tradeNo string) (*CryptoTransaction, error) {
	var tx CryptoTransaction
	if err := DB.Where("trade_no = ?", tradeNo).First(&tx).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

// CompleteCryptoTransaction 完成加密货币交易：写入 tx_hash、付款地址、区块号、确认数并更新状态为 success
// 调用时机：用户提交交易哈希后，链上验证通过
func CompleteCryptoTransaction(tradeNo string, txHash string, payerAddress string, blockNumber uint64, confirmations int) error {
	tradeNo = strings.TrimSpace(tradeNo)
	txHash = normalizeTxHash(txHash)
	if tradeNo == "" {
		return errors.New("trade no is empty")
	}
	if txHash == "" {
		return errors.New("tx hash is empty")
	}
	if DB == nil {
		return errors.New("database is not initialized")
	}
	result := DB.Model(&CryptoTransaction{}).Where("trade_no = ?", tradeNo).Updates(map[string]any{
		"tx_hash":       txHash,
		"payer_address": strings.ToLower(strings.TrimSpace(payerAddress)),
		"block_number":  blockNumber,
		"confirmations": confirmations,
		"status":        CryptoTransactionStatusSuccess,
		"complete_time": common.GetTimestamp(),
	})
	if result.Error != nil {
		if IsCryptoTransactionHashUniqueViolation(result.Error) {
			return ErrCryptoTransactionHashUsed
		}
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("crypto transaction not found")
	}
	return nil
}

// CryptoTxHashExists 检查交易哈希是否已被使用，防止同一笔链上交易重复充值
// 查询前会对 txHash 做标准化处理（小写 + 去空格）
func CryptoTxHashExists(txHash string) bool {
	txHash = normalizeTxHash(txHash)
	if txHash == "" || DB == nil {
		return false
	}
	var count int64
	// Hashes are hexadecimal and semantically case-insensitive.  LOWER keeps
	// checks consistent with the normalized value even for legacy rows written
	// before normalization was introduced.
	DB.Model(&CryptoTransaction{}).Where("LOWER(tx_hash) = LOWER(?)", txHash).Count(&count)
	return count > 0
}

// CryptoTxHashExistsForOtherTradeNo reports whether txHash is already bound to
// a different payment order.  A confirmation retry for the same subscription
// order must be idempotent: its own CryptoTransaction row may already contain
// the hash and should not be rejected by a global duplicate check.  The
// database-level unique index on tx_hash remains the final race-proof guard;
// this helper is only the early, user-facing validation.
func CryptoTxHashExistsForOtherTradeNo(txHash, tradeNo string) bool {
	txHash = normalizeTxHash(txHash)
	tradeNo = strings.TrimSpace(tradeNo)
	if txHash == "" || DB == nil {
		return false
	}
	query := DB.Model(&CryptoTransaction{}).Where("LOWER(tx_hash) = LOWER(?)", txHash)
	if tradeNo != "" {
		query = query.Where("trade_no <> ?", tradeNo)
	}
	var count int64
	query.Count(&count)
	return count > 0
}

// IsCryptoTransactionHashUniqueViolation reports a database uniqueness error
// for the tx_hash column.  The exact driver error text differs across SQLite,
// MySQL and PostgreSQL, so callers should use this narrow helper when mapping
// a concurrent confirmation race to ErrCryptoTransactionHashUsed.
func IsCryptoTransactionHashUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if !strings.Contains(s, "tx_hash") && !strings.Contains(s, "crypto_transactions") {
		return false
	}
	return strings.Contains(s, "unique") ||
		strings.Contains(s, "duplicate") ||
		strings.Contains(s, "constraint failed")
}

// GetCryptoTokenSymbolsByTradeNos 根据订单号批量查询代币符号
// 返回 map[tradeNo]tokenSymbol，用于 top-up 列表展示时填充 DisplaySymbol
// 入参为空切片时直接返回 nil，避免无效查询
func GetCryptoTokenSymbolsByTradeNos(tradeNos []string) (map[string]string, error) {
	if len(tradeNos) == 0 {
		return nil, nil
	}
	var rows []struct {
		TradeNo     string `gorm:"column:trade_no"`
		TokenSymbol string `gorm:"column:token_symbol"`
	}
	if err := DB.Model(&CryptoTransaction{}).Select("trade_no, token_symbol").Where("trade_no IN ?", tradeNos).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, r := range rows {
		result[r.TradeNo] = r.TokenSymbol
	}
	return result, nil
}
