package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// TelegramUserBinding 记录 Telegram 用户与网站用户的绑定关系。
// 与 users.telegram_id 不同：该表仅用于身份映射（发福利等），不授予 Telegram 登录权限。
type TelegramUserBinding struct {
	ID             int       `gorm:"primaryKey;autoIncrement"                              json:"id"`
	TelegramUserID string    `gorm:"column:telegram_user_id;uniqueIndex;not null;size:64"  json:"telegram_user_id"`
	UserID         int       `gorm:"column:user_id;uniqueIndex;not null"                   json:"user_id"`
	UserName       string    `gorm:"column:user_name;not null;size:50"                     json:"user_name"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"                      json:"created_at"`
}

func (TelegramUserBinding) TableName() string {
	return "telegram_user_bindings"
}

// GetTelegramBindingByTGID 查该 Telegram 用户已有的绑定；未绑定返回 (nil, nil)。
func GetTelegramBindingByTGID(telegramUserId string) (*TelegramUserBinding, error) {
	var binding TelegramUserBinding
	err := DB.Where("telegram_user_id = ?", telegramUserId).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetTelegramBindingByUserID 查该网站用户已有的绑定；未绑定返回 (nil, nil)。
func GetTelegramBindingByUserID(userId int) (*TelegramUserBinding, error) {
	var binding TelegramUserBinding
	err := DB.Where("user_id = ?", userId).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// CreateTelegramBinding 插入一条新的 Telegram 绑定记录。
func CreateTelegramBinding(binding *TelegramUserBinding) error {
	return DB.Create(binding).Error
}
