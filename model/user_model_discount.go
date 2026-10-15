package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserModelDiscount 指定用户的指定模型专属折扣（仅输入输出 token 部分，缓存不参与；
// 仅余额支付时生效，订阅计费不参与；与分组倍率叠加）。
type UserModelDiscount struct {
	Id        int     `json:"id"`
	UserId    int     `json:"user_id" gorm:"uniqueIndex:idx_user_model_discount_unique,priority:1;not null"`
	ModelName string  `json:"model_name" gorm:"uniqueIndex:idx_user_model_discount_unique,priority:2;type:varchar(255);not null"`
	Discount  float64 `json:"discount" gorm:"type:decimal(10,6);not null;default:1"`
	CreatedAt int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt int64   `json:"updated_at" gorm:"bigint"`
}

func (UserModelDiscount) TableName() string {
	return "user_model_discounts"
}

func (d *UserModelDiscount) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if d.CreatedAt == 0 {
		d.CreatedAt = now
	}
	if d.UpdatedAt == 0 {
		d.UpdatedAt = now
	}
	return nil
}

func (d *UserModelDiscount) BeforeUpdate(tx *gorm.DB) error {
	d.UpdatedAt = common.GetTimestamp()
	return nil
}

const (
	userModelDiscountCacheNamespace = "new-api:user_model_discount:v1"
	userModelDiscountCacheTTL       = 5 * time.Minute
	// UserModelDiscountNone 表示无专属折扣（不打折）
	UserModelDiscountNone = 1.0
)

var (
	userModelDiscountCacheOnce sync.Once
	userModelDiscountCache     *cachex.HybridCache[UserModelDiscount]
)

func getUserModelDiscountCache() *cachex.HybridCache[UserModelDiscount] {
	userModelDiscountCacheOnce.Do(func() {
		userModelDiscountCache = cachex.NewHybridCache[UserModelDiscount](cachex.HybridCacheConfig[UserModelDiscount]{
			Namespace:  cachex.Namespace(userModelDiscountCacheNamespace),
			Redis:      common.RDB,
			RedisCodec: cachex.JSONCodec[UserModelDiscount]{},
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			Memory: func() *hot.HotCache[string, UserModelDiscount] {
				return hot.NewHotCache[string, UserModelDiscount](hot.LRU, 10000).
					WithTTL(userModelDiscountCacheTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return userModelDiscountCache
}

func userModelDiscountCacheKey(userId int, modelName string) string {
	return strconv.Itoa(userId) + ":" + modelName
}

// clampUserModelDiscount 将折扣钳制到 (0,1]，非法值视为无折扣
func clampUserModelDiscount(discount float64) float64 {
	if discount <= 0 || discount > 1 {
		return UserModelDiscountNone
	}
	return discount
}

// GetUserModelDiscount 返回用户对指定模型的专属折扣，无配置或配置非法时返回 1（不打折）。
func GetUserModelDiscount(userId int, modelName string) (float64, error) {
	if userId <= 0 || modelName == "" {
		return UserModelDiscountNone, nil
	}
	key := userModelDiscountCacheKey(userId, modelName)
	cache := getUserModelDiscountCache()
	cached, found, err := cache.Get(key)
	if err != nil {
		common.SysLog("failed to get user model discount cache: " + err.Error())
	} else if found {
		return clampUserModelDiscount(cached.Discount), nil
	}

	var record UserModelDiscount
	err = DB.Where("user_id = ? AND model_name = ?", userId, modelName).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 缓存"无折扣"哨兵，避免高频请求反复查库
			none := UserModelDiscount{UserId: userId, ModelName: modelName, Discount: UserModelDiscountNone}
			if err := cache.SetWithTTL(key, none, userModelDiscountCacheTTL); err != nil {
				common.SysLog("failed to set user model discount cache: " + err.Error())
			}
			return UserModelDiscountNone, nil
		}
		common.SysLog("failed to query user model discount: " + err.Error())
		return UserModelDiscountNone, err
	}
	if err := cache.SetWithTTL(key, record, userModelDiscountCacheTTL); err != nil {
		common.SysLog("failed to set user model discount cache: " + err.Error())
	}
	return clampUserModelDiscount(record.Discount), nil
}

// InvalidateUserModelDiscountCache 失效指定 (user, model) 的折扣缓存。
// modelName 为空时失效该用户全部模型的折扣缓存。
func InvalidateUserModelDiscountCache(userId int, modelName string) {
	if userId <= 0 {
		return
	}
	cache := getUserModelDiscountCache()
	if modelName == "" {
		if _, err := cache.DeleteByPrefix(strconv.Itoa(userId) + ":"); err != nil {
			common.SysLog("failed to invalidate user model discount cache: " + err.Error())
		}
		return
	}
	if _, err := cache.DeleteMany([]string{userModelDiscountCacheKey(userId, modelName)}); err != nil {
		common.SysLog("failed to invalidate user model discount cache: " + err.Error())
	}
}

// UpsertUserModelDiscount 新增或更新一条专属折扣，折扣范围 (0,1]。
func UpsertUserModelDiscount(userId int, modelName string, discount float64) error {
	if userId <= 0 {
		return errors.New("无效的用户ID invalid user id")
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return errors.New("无效的模型名 invalid model name")
	}
	if discount <= 0 || discount > 1 {
		return fmt.Errorf("折扣范围必须在 (0, 1] 之间 discount %v out of range (0, 1]", discount)
	}
	record := UserModelDiscount{
		UserId:    userId,
		ModelName: modelName,
		Discount:  discount,
	}
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "model_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"discount":   discount,
			"updated_at": common.GetTimestamp(),
		}),
	}).Create(&record).Error
	if err != nil {
		return err
	}
	InvalidateUserModelDiscountCache(userId, modelName)
	return nil
}

// DeleteUserModelDiscount 删除一条专属折扣（恢复原价）。
func DeleteUserModelDiscount(userId int, modelName string) error {
	if userId <= 0 || modelName == "" {
		return errors.New("invalid user id or model name")
	}
	result := DB.Where("user_id = ? AND model_name = ?", userId, modelName).Delete(&UserModelDiscount{})
	if result.Error != nil {
		return result.Error
	}
	InvalidateUserModelDiscountCache(userId, modelName)
	return nil
}

// GetUserModelDiscountsByUser 返回某用户全部专属折扣。
func GetUserModelDiscountsByUser(userId int) ([]UserModelDiscount, error) {
	var records []UserModelDiscount
	err := DB.Where("user_id = ?", userId).Order("model_name ASC").Find(&records).Error
	return records, err
}
