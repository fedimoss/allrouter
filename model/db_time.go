package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// GetDBTimestamp 从数据库当前时间获取 UNIX 时间戳。
// 查询失败时回退到应用服务器时间。
func GetDBTimestamp() int64 {
	return getDBTimestampTx(DB)
}

// getDBTimestampTx 使用传入事务查询数据库当前时间，保证事务内时间读取不额外占用连接。
// tx 为空时使用全局 DB。
func getDBTimestampTx(tx *gorm.DB) int64 {
	if tx == nil {
		tx = DB
	}
	var ts int64
	var err error
	switch {
	case common.UsingPostgreSQL:
		err = tx.Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
	case common.UsingSQLite:
		err = tx.Raw("SELECT strftime('%s','now')").Scan(&ts).Error
	default:
		err = tx.Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
	}
	if err != nil || ts <= 0 {
		return common.GetTimestamp()
	}
	return ts
}
