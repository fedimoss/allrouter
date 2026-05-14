package model

import (
	"strings"
	"time"
)

// CryptoTransaction 状态常量
const (
	CryptoTransactionStatusPending = "pending" // 待确认（等待用户提交交易哈希）
	CryptoTransactionStatusSuccess = "success" // 已完成
	CryptoTransactionStatusFailed  = "failed"  // 失败
)

// CryptoTransaction 加密货币交易记录
// 存储 Web3 链上支付的详细信息，包括链 ID、代币合约、收款地址、交易哈希等
type CryptoTransaction struct {
	Id                  int       `json:"id"`
	TopUpId             int       `json:"topup_id" gorm:"index;not null"`                         // 关联的充值订单 ID（订阅支付时为 0）
	SubscriptionOrderId int       `json:"subscription_order_id" gorm:"index;default:0"`           // 关联的订阅订单 ID（充值支付时为 0）
	UserId              int       `json:"user_id" gorm:"index;not null"`                          // 用户 ID
	TradeNo             string    `json:"trade_no" gorm:"type:varchar(255);uniqueIndex;not null"` // 订单号（唯一索引）
	TxHash              *string   `json:"tx_hash,omitempty" gorm:"type:varchar(128);uniqueIndex"` // 链上交易哈希（确认后填入，唯一索引）
	ChainId             int       `json:"chain_id" gorm:"index;not null"`                         // 链 ID（如 BSC 主网为 56）
	TokenSymbol         string    `json:"token_symbol" gorm:"type:varchar(20);not null"`          // 代币符号（如 USDT）
	TokenContract       string    `json:"token_contract" gorm:"type:varchar(128);not null"`       // 代币合约地址
	ReceiverAddress     string    `json:"receiver_address" gorm:"type:varchar(128);not null"`     // 收款地址
	PayerAddress        string    `json:"payer_address" gorm:"type:varchar(128)"`                 // 付款地址（链上确认后填入）
	UsdtAmount          string    `json:"usdt_amount" gorm:"type:varchar(64);not null"`           // USDT 金额（字符串存储，保证精度）
	BlockNumber         uint64    `json:"block_number" gorm:"default:0"`                          // 区块号（确认后填入）
	Confirmations       int       `json:"confirmations" gorm:"default:0"`                         // 确认数（确认后填入）
	Status              string    `json:"status" gorm:"type:varchar(20);index;not null"`          // 状态（pending/success/failed）
	CreateTime          int64     `json:"create_time"`                                            // 创建时间
	CompleteTime        int64     `json:"complete_time"`                                          // 完成时间
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`                       // 更新时间
}

func (CryptoTransaction) TableName() string {
	return "crypto_transactions"
}

// normalizeTxHash 标准化交易哈希（转小写并去除首尾空格）
func normalizeTxHash(txHash string) string {
	return strings.ToLower(strings.TrimSpace(txHash))
}

// GetCryptoTransactionByTradeNo 根据订单号查询加密货币交易记录
func GetCryptoTransactionByTradeNo(tradeNo string) (*CryptoTransaction, error) {
	var tx CryptoTransaction
	if err := DB.Where("trade_no = ?", tradeNo).First(&tx).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

// CryptoTxHashExists 检查交易哈希是否已被使用（防止同一笔链上交易重复充值）
func CryptoTxHashExists(txHash string) bool {
	txHash = normalizeTxHash(txHash)
	if txHash == "" {
		return false
	}
	var count int64
	DB.Model(&CryptoTransaction{}).Where("tx_hash = ?", txHash).Count(&count)
	return count > 0
}

// GetCryptoTokenSymbolsByTradeNos 根据订单号批量查询代币符号
// 返回 map[tradeNo]tokenSymbol，用于 top-up 列表展示时填充 DisplaySymbol
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
