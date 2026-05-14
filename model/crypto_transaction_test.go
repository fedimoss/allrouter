package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCryptoTransactionTestDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldLogDB := LOG_DB
	oldQuotaPerUnit := common.QuotaPerUnit
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &CryptoTransaction{}, &TopUpRebate{}, &Log{}))
	DB = db
	LOG_DB = db
	common.QuotaPerUnit = 100000
	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		common.QuotaPerUnit = oldQuotaPerUnit
	})
}

func TestRechargeCryptoCompletesOrderOnce(t *testing.T) {
	setupCryptoTransactionTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 1, Username: "u1", Quota: 0}).Error)
	topUp := TopUp{
		UserId:        1,
		Amount:        5,
		Money:         5,
		TradeNo:       "CRYPTO-test",
		PaymentMethod: "crypto",
		BizType:       TopUpBizTypePayment,
		CreateTime:    100,
		Status:        common.TopUpStatusPending,
		Currency:      "$",
		OriginalMoney: 5,
	}
	require.NoError(t, DB.Create(&topUp).Error)
	require.NoError(t, DB.Create(&CryptoTransaction{
		TopUpId:         topUp.Id,
		UserId:          1,
		TradeNo:         topUp.TradeNo,
		ChainId:         56,
		TokenSymbol:     "USDT",
		TokenContract:   "0x55d398326f99059ff775485246999027b3197955",
		ReceiverAddress: "0x1111111111111111111111111111111111111111",
		UsdtAmount:      "5.000000",
		Status:          CryptoTransactionStatusPending,
		CreateTime:      100,
	}).Error)

	err := RechargeCrypto(topUp.TradeNo, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "0x2222222222222222222222222222222222222222", 123, 3)
	require.NoError(t, err)

	var user User
	require.NoError(t, DB.First(&user, 1).Error)
	require.Equal(t, 500000, user.Quota)

	var updatedTopUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", topUp.TradeNo).First(&updatedTopUp).Error)
	require.Equal(t, common.TopUpStatusSuccess, updatedTopUp.Status)

	var cryptoTx CryptoTransaction
	require.NoError(t, DB.Where("trade_no = ?", topUp.TradeNo).First(&cryptoTx).Error)
	require.Equal(t, CryptoTransactionStatusSuccess, cryptoTx.Status)
	require.NotNil(t, cryptoTx.TxHash)

	err = RechargeCrypto(topUp.TradeNo, *cryptoTx.TxHash, cryptoTx.PayerAddress, cryptoTx.BlockNumber, cryptoTx.Confirmations)
	require.NoError(t, err)
	require.NoError(t, DB.First(&user, 1).Error)
	require.Equal(t, 500000, user.Quota)
}
