package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

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
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"`              // expired time, 0 means never expires
	SentTime     int64          `json:"sent_time" gorm:"bigint;default:0;index"` // 发放标记时间，0 表示未发放
}

// applySentFilter 在列表查询上追加发放状态筛选条件（nil 表示不过滤）。
func applySentFilter(query *gorm.DB, sentFilter *bool) *gorm.DB {
	if sentFilter == nil {
		return query
	}
	if *sentFilter {
		return query.Where("sent_time > 0")
	}
	return query.Where("sent_time = 0")
}

// GetSentQueryFilter 将请求中的 sent 参数解析为发放状态筛选条件。
// "sent" → true（已发放）、"unsent" → false（未发放）、其他值 → nil（不过滤）。
func GetSentQueryFilter(sent string) *bool {
	switch sent {
	case "sent":
		v := true
		return &v
	case "unsent":
		v := false
		return &v
	default:
		return nil
	}
}

// redemptionAmountScale 统一约束兑换码金额的计算精度，避免不同入口产生不同的小数结果。
const redemptionAmountScale int32 = 6

// redemptionOriginalValue 保存兑换码发放时的原始金额和币种，仅用于生成可追溯的充值流水。
type redemptionOriginalValue struct {
	Amount   float64
	Currency string
}

// normalizeRedemptionCurrency 将币种代码和常见符号归一化为稳定的数据库值。
func normalizeRedemptionCurrency(currency string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USD", "$":
		return "USD", nil
	case "CNY", "¥", "￥":
		return "CNY", nil
	default:
		return "", fmt.Errorf("unsupported redemption currency: %s", currency)
	}
}

// redemptionUSDValue 将内部整数额度换算为统一的美元价值，供统计和结算字段使用。
func redemptionUSDValue(quota int) float64 {
	return decimal.NewFromInt(int64(quota)).
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Round(redemptionAmountScale).
		InexactFloat64()
}

// GetAllRedemptions 分页获取全部兑换码（管理端），sentFilter 为发放状态筛选条件（nil 不过滤）。
func GetAllRedemptions(startIdx int, num int, sentFilter *bool) (redemptions []*Redemption, total int64, err error) {
	return getRedemptionsByProvider(DB, 0, startIdx, num, sentFilter)
}

// GetRedemptionsByProvider 分页获取指定服务商的兑换码（服务商端），sentFilter 为发放状态筛选条件（nil 不过滤）。
func GetRedemptionsByProvider(providerId int, startIdx int, num int, sentFilter *bool) (redemptions []*Redemption, total int64, err error) {
	return getRedemptionsByProvider(DB, providerId, startIdx, num, sentFilter)
}

func getRedemptionsByProvider(db *gorm.DB, providerId int, startIdx int, num int, sentFilter *bool) (redemptions []*Redemption, total int64, err error) {
	query := db.Model(&Redemption{}).Where("provider_id = ?", providerId)
	// 追加发放状态筛选（已发放 sent_time > 0 / 未发放 sent_time = 0）
	query = applySentFilter(query, sentFilter)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	return redemptions, total, err
}

// SearchRedemptions 按关键字搜索全部兑换码（管理端），sentFilter 为发放状态筛选条件（nil 不过滤）。
func SearchRedemptions(keyword string, startIdx int, num int, sentFilter *bool) (redemptions []*Redemption, total int64, err error) {
	return SearchRedemptionsByProvider(0, keyword, startIdx, num, sentFilter)
}

// SearchRedemptionsByProvider 按关键字搜索指定服务商的兑换码（服务商端），sentFilter 为发放状态筛选条件（nil 不过滤）。
func SearchRedemptionsByProvider(providerId int, keyword string, startIdx int, num int, sentFilter *bool) (redemptions []*Redemption, total int64, err error) {
	query := DB.Model(&Redemption{}).Where("provider_id = ?", providerId)
	if id, err := strconv.Atoi(keyword); err == nil {
		query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
	} else {
		query = query.Where("name LIKE ?", keyword+"%")
	}
	// 发放状态筛选与关键字搜索条件叠加
	query = applySentFilter(query, sentFilter)
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
	// 手工创建的兑换码固定按 USD 解释，原始金额可由内部额度准确换算。
	return redeem(key, userId, nil)
}

// redeemWithOriginalValue 用于充值赠送，保留赠送发生时的原始金额和币种。
func redeemWithOriginalValue(key string, userId int, value redemptionOriginalValue) (quota int, err error) {
	return redeem(key, userId, &value)
}

// redeem 统一处理额度入账、兑换状态更新和充值流水写入，避免两种来源出现重复实现。
func redeem(key string, userId int, originalValue *redemptionOriginalValue) (quota int, err error) {
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
		// Money 保存归一化美元价值供统计使用；OriginalMoney/Currency 保存用户看到的原始面值。
		moneyUSD := redemptionUSDValue(redemption.Quota)
		originalMoney := moneyUSD
		currency := "USD"
		if originalValue != nil {
			if originalValue.Amount <= 0 {
				return errors.New("invalid redemption original amount")
			}
			normalizedCurrency, currencyErr := normalizeRedemptionCurrency(originalValue.Currency)
			if currencyErr != nil {
				return currencyErr
			}
			currency = normalizedCurrency
			originalMoney = decimal.NewFromFloat(originalValue.Amount).
				Round(redemptionAmountScale).
				InexactFloat64()
		}
		topUp := &TopUp{
			ProviderId:    user.ProviderId,
			Amount:        int64(redemption.Quota),
			UserId:        userId,
			Money:         moneyUSD,
			TradeNo:       tradeNo,
			PaymentMethod: "redemptionCode",
			BizType:       TopUpBizTypeRedemption,
			SourceID:      redemption.Id,
			CreateTime:    now,
			CompleteTime:  now,
			Status:        common.TopUpStatusSuccess,
			Currency:      currency,
			OriginalMoney: originalMoney,
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

// BatchUpdateRedemptionSent 批量更新兑换码的发放标记，sent 为 true 时记录当前时间，false 时清零。
// providerId 大于 0 时限定服务商范围，0 表示管理端全量。
func BatchUpdateRedemptionSent(providerId int, ids []int, sent bool) (int64, error) {
	// 标记时写入当前时间戳，取消时清零；单列 Update 可正常写入零值
	sentTime := int64(0)
	if sent {
		sentTime = common.GetTimestamp()
	}
	query := DB.Model(&Redemption{}).Where("id IN ?", ids)
	// 服务商端调用时限定归属范围，防止越权操作他人兑换码
	if providerId > 0 {
		query = query.Where("provider_id = ?", providerId)
	}
	result := query.Update("sent_time", sentTime)
	return result.RowsAffected, result.Error
}

// GetRedemptionsByIds 按 ID 列表查询兑换码，providerId 大于 0 时限定服务商范围，0 表示管理端全量。
func GetRedemptionsByIds(providerId int, ids []int) ([]*Redemption, error) {
	var redemptions []*Redemption
	query := DB.Where("id IN ?", ids)
	if providerId > 0 {
		query = query.Where("provider_id = ?", providerId)
	}
	err := query.Find(&redemptions).Error
	return redemptions, err
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
