package model

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Redemption struct {
	Id           int            `json:"id"`
	ProviderId   int            `json:"provider_id" gorm:"type:int;default:0;index;uniqueIndex:ux_provider_redemption_key"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex:ux_provider_redemption_key"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // expired time, 0 means never expires
}

func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	return getRedemptionsByProvider(DB, 0, startIdx, num)
}

func GetRedemptionsByProvider(providerId int, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	return getRedemptionsByProvider(DB, providerId, startIdx, num)
}

func getRedemptionsByProvider(db *gorm.DB, providerId int, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	query := db.Model(&Redemption{}).Where("provider_id = ?", providerId)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	return redemptions, total, err
}

func SearchRedemptions(keyword string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	return SearchRedemptionsByProvider(0, keyword, startIdx, num)
}

func SearchRedemptionsByProvider(providerId int, keyword string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	query := DB.Model(&Redemption{}).Where("provider_id = ?", providerId)
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
	} else {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	return redemptions, total, err
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func GetRedemptionByIdInProvider(id int, providerId int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	redemption := Redemption{Id: id, ProviderId: providerId}
	err := DB.Where("id = ? AND provider_id = ?", id, providerId).First(&redemption).Error
	return &redemption, err
}

func GetUserRedeemedRedemptions(userId int, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	query := DB.Model(&Redemption{}).Where("used_user_id = ? AND status = ?", userId, common.RedemptionCodeStatusUsed)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("redeemed_time desc").Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	return redemptions, total, err
}

func Redeem(key string, userId int) (quota int, err error) {
	if key == "" {
		return 0, errors.New("missing redemption code")
	}
	if userId == 0 {
		return 0, errors.New("invalid user id")
	}
	redemption := &Redemption{}

	keyCol := "`key`"
	if common.UsingPostgreSQL {
		keyCol = `"key"`
	}
	common.RandomSleep()
	now := common.GetTimestamp()

	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Select("id", "provider_id").Where("id = ?", userId).Take(&user).Error; err != nil {
			return err
		}
		err := tx.Set("gorm:query_option", "FOR UPDATE").Where("provider_id = ? AND "+keyCol+" = ?", user.ProviderId, key).First(redemption).Error
		if err != nil {
			return errors.New("invalid redemption code")
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return errors.New("redemption code already used")
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return errors.New("redemption code expired")
		}
		tradeNo := fmt.Sprintf("RDM-%d-%d-%s", redemption.Id, userId, common.GetRandomString(8))
		err = tx.Model(&User{}).Where("id = ?", userId).Updates(map[string]interface{}{
			"quota":        gorm.Expr("quota + ?", redemption.Quota),
			"reward_quota": gorm.Expr("reward_quota + ?", redemption.Quota),
		}).Error
		if err != nil {
			return err
		}
		if user.ProviderId > 0 {
			if err := CreateRewardRecordTx(tx, &RewardRecord{
				ProviderId:  user.ProviderId,
				UserId:      userId,
				SourceType:  "redemption",
				SourceId:    redemption.Id,
				Quota:       redemption.Quota,
				Description: "redemption reward",
			}); err != nil {
				return err
			}
		}
		moneyDecimal := decimal.NewFromInt(int64(redemption.Quota)).Div(decimal.NewFromFloat(common.QuotaPerUnit))
		part := moneyDecimal.Round(0).IntPart()
		topUp := &TopUp{
			ProviderId:    user.ProviderId,
			Amount:        part,
			UserId:        userId,
			Money:         0,
			TradeNo:       tradeNo,
			PaymentMethod: "redemptionCode",
			BizType:       TopUpBizTypeRedemption,
			SourceID:      redemption.Id,
			CreateTime:    now,
			CompleteTime:  now,
			Status:        common.TopUpStatusSuccess,
		}
		err = topUp.InsertTx(tx)
		if err != nil {
			return err
		}
		redemption.RedeemedTime = common.GetTimestamp()
		redemption.Status = common.RedemptionCodeStatusUsed
		redemption.UsedUserId = userId
		err = tx.Save(redemption).Error
		return err
	})
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		return 0, ErrRedeemFailed
	}
	asyncIncrUserQuotaCache(userId, redemption.Quota)
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("redeemed %s using code ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	return redemption.Quota, nil
}

// GetUsersRedemptionQuota 鎵归噺鏌ヨ鐢ㄦ埛閫氳繃鍏戞崲鐮佸厖鍊肩殑鎬婚
func GetUsersRedemptionQuota(userIds []int) (map[int]int64, error) {
	if len(userIds) == 0 {
		return map[int]int64{}, nil
	}
	type result struct {
		UsedUserId int   `json:"used_user_id"`
		Total      int64 `json:"total"`
	}
	var results []result
	err := DB.Model(&Redemption{}).
		Select("used_user_id, COALESCE(SUM(quota), 0) as total").
		Where("used_user_id IN ? AND status = ?", userIds, common.RedemptionCodeStatusUsed).
		Group("used_user_id").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	m := make(map[int]int64, len(results))
	for _, r := range results {
		m[r.UsedUserId] = r.Total
	}
	return m, nil
}

func (redemption *Redemption) Insert() error {
	var err error
	err = DB.Create(redemption).Error
	return err
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update Make sure your token's fields is completed, because this will update non-zero values
func (redemption *Redemption) Update() error {
	var err error
	err = DB.Model(redemption).Select("name", "status", "quota", "redeemed_time", "expired_time").Updates(redemption).Error
	return err
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id is empty")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteRedemptionByIdInProvider(id int, providerId int) (err error) {
	if id == 0 {
		return errors.New("id is empty")
	}
	redemption := Redemption{}
	err = DB.Where("id = ? AND provider_id = ?", id, providerId).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteInvalidRedemptions() (int64, error) {
	return DeleteInvalidRedemptionsByProvider(0)
}

func DeleteInvalidRedemptionsByProvider(providerId int) (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("provider_id = ? AND (status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?))", providerId, []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
