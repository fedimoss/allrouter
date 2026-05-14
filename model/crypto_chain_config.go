package model

import (
	"strings"

	"gorm.io/gorm"
)

// CryptoChainConfig 加密货币链配置表
// 复合主键 (network, token_symbol)，每条链每种代币各一行
type CryptoChainConfig struct {
	Network          string `json:"network" gorm:"column:network;type:varchar(32);primaryKey"`                             // 网络名称，如 Sepolia / BSC / Polygon
	ChainID          int    `json:"chain_id" gorm:"column:chain_id;type:int8;not null"`                                    // EIP-155 链 ID
	TokenSymbol      string `json:"token_symbol" gorm:"column:token_symbol;type:varchar(20);primaryKey;default:USDT"`      // 代币符号
	TokenDecimals    int    `json:"token_decimals" gorm:"column:token_decimals;type:int2;not null;default:18"`             // 代币精度
	TokenContract    string `json:"token_contract" gorm:"column:token_contract;type:varchar(128);not null;default:''"`     // 代币合约地址
	ReceiverAddress  string `json:"receiver_address" gorm:"column:receiver_address;type:varchar(128);not null;default:''"` // 收款钱包地址
	RPCURL           string `json:"rpc_url" gorm:"column:rpc_url;type:varchar(512);not null;default:''"`                   // 链节点 RPC 地址
	MinConfirmations int    `json:"min_confirmations" gorm:"column:min_confirmations;type:int2;not null;default:3"`        // 最小链上确认数
}

func (CryptoChainConfig) TableName() string {
	return "crypto_chain_config"
}

// GetCryptoChainByNetwork 根据网络名称和代币符号查询链配置（大小写不敏感）
// tokenSymbol 为空时默认 "USDT"
func GetCryptoChainByNetwork(network string, tokenSymbol string) (*CryptoChainConfig, error) {
	if strings.TrimSpace(tokenSymbol) == "" {
		tokenSymbol = "USDT"
	}
	var cfg CryptoChainConfig
	err := DB.Where("LOWER(network) = ? AND LOWER(token_symbol) = ?",
		strings.ToLower(network), strings.ToLower(tokenSymbol)).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetCryptoChainConfigList 获取所有加密货币链配置列表
func GetCryptoChainConfigList() ([]CryptoChainConfig, error) {
	var cfgs []CryptoChainConfig

	// 排序：按网络名称和代币符号升序
	err := DB.Order("network, token_symbol").Find(&cfgs).Error
	if err != nil {
		return nil, err
	}

	return cfgs, nil
}

// UpdateCryptoChainConfigList 更新加密货币链配置列表
func UpdateCryptoChainConfigList(cfgs []CryptoChainConfig) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		// 先清空全表，再按照前端传入的数据重新插入
		if err := tx.Where("1 = 1").Delete(&CryptoChainConfig{}).Error; err != nil {
			return err
		}

		// 插入新配置
		for i := range cfgs {
			if err := tx.Create(&cfgs[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetCryptoChainByID 根据链 ID 和代币符号查询链配置
func GetCryptoChainByID(chainID int, tokenSymbol string) (*CryptoChainConfig, error) {
	if strings.TrimSpace(tokenSymbol) == "" {
		tokenSymbol = "USDT"
	}
	var cfg CryptoChainConfig
	err := DB.Where("chain_id = ? AND LOWER(token_symbol) = ?",
		chainID, strings.ToLower(tokenSymbol)).First(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
