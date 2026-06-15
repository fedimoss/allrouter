package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ConsumeRebate records rebates generated from invitees consuming paid quota.
type ConsumeRebate struct {
	Id                int     `json:"id"`
	ProviderId        int     `json:"provider_id" gorm:"index;default:0"`
	InviterId         int     `json:"inviter_id" gorm:"index"`
	InviteeId         int     `json:"invitee_id" gorm:"index"`
	RequestId         string  `json:"request_id" gorm:"type:varchar(64);uniqueIndex:idx_consume_rebate_request_level"`
	Level             int     `json:"level" gorm:"default:1;uniqueIndex:idx_consume_rebate_request_level"`
	SourceQuota       int     `json:"source_quota"`
	RebateRatio       float64 `json:"rebate_ratio"`
	RebateQuota       int     `json:"rebate_quota"`
	ProviderPricingId int     `json:"provider_pricing_id" gorm:"default:0;index"`
	PublicModelName   string  `json:"public_model_name" gorm:"type:varchar(255);default:'';index:idx_consume_rebate_provider_model,priority:2"`
	BaseModelName     string  `json:"base_model_name" gorm:"type:varchar(255);default:''"`
	CreatedAt         int64   `json:"created_at" gorm:"bigint;index"`
}

func (ConsumeRebate) TableName() string {
	return "consume_rebates"
}

type ConsumeRebateContext struct {
	ProviderId        int
	ProviderPricingId int
	PublicModelName   string
	BaseModelName     string
}

// 获取被邀请人消费返利记录
func GetConsumeRebateRecordsByInviteeId(userId int, pageInfo *common.PageInfo) (records []*ConsumeRebate, total int64, err error) {
	query := DB.Model(&ConsumeRebate{}).
		Select("rebate_quota, created_at").
		Where("invitee_id = ?", userId)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err = query.
		Order("created_at desc").
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// GetConsumeRebateRecordsByInviterAndInviteeId gets rebate records earned by inviter from a specific invitee at a level.
func GetConsumeRebateRecordsByInviterAndInviteeId(inviterId int, inviteeId int, level int, pageInfo *common.PageInfo) (records []*ConsumeRebate, total int64, err error) {
	query := DB.Model(&ConsumeRebate{}).
		Select("rebate_quota, created_at").
		Where("inviter_id = ? AND invitee_id = ? AND level = ?", inviterId, inviteeId, level)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err = query.
		Order("created_at desc").
		Order("id desc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// 计算邀请消费返利总和
func SumConsumeRebateQuotaByInviteeId(userId int) (int64, error) {
	var totalQuota int64
	err := DB.Model(&ConsumeRebate{}).
		Select("COALESCE(SUM(rebate_quota), 0)").
		Where("invitee_id = ?", userId).
		Scan(&totalQuota).Error
	return totalQuota, err
}

// SumConsumeRebateQuotaByInviterAndInviteeId sums rebates earned by inviter from a specific invitee at a level.
func SumConsumeRebateQuotaByInviterAndInviteeId(inviterId int, inviteeId int, level int) (int64, error) {
	var totalQuota int64
	err := DB.Model(&ConsumeRebate{}).
		Select("COALESCE(SUM(rebate_quota), 0)").
		Where("inviter_id = ? AND invitee_id = ? AND level = ?", inviterId, inviteeId, level).
		Scan(&totalQuota).Error
	return totalQuota, err
}

// SumLevel2ConsumeRebateQuotaByInviterAndParentInviteeId sums level-2 rebates earned by inviter from users invited by parentInviteeId.
func SumLevel2ConsumeRebateQuotaByInviterAndParentInviteeId(inviterId int, parentInviteeId int) (int64, error) {
	var totalQuota int64
	err := DB.Model(&ConsumeRebate{}).
		Select("COALESCE(SUM(consume_rebates.rebate_quota), 0)").
		Joins("JOIN users ON users.id = consume_rebates.invitee_id").
		Where("consume_rebates.inviter_id = ? AND consume_rebates.level = ? AND users.inviter_id = ?", inviterId, 2, parentInviteeId).
		Scan(&totalQuota).Error
	return totalQuota, err
}

// 当被邀请人实际使用充值额度消费时，按比例给邀请人返利。返利入账也算奖励，所以邀请人会同时增加 quota 和 reward_quota
func ApplyInviteConsumeRebate(inviteeId int, requestId string, paidQuota int, rebateCtx *ConsumeRebateContext) (int, int, error) {
	if inviteeId <= 0 || paidQuota <= 0 {
		return 0, 0, nil
	}
	if rebateCtx == nil || rebateCtx.ProviderId <= 0 || rebateCtx.PublicModelName == "" {
		return 0, 0, nil
	}
	if requestId == "" {
		requestId = fmt.Sprintf("consume-rebate-%d-%d-%s", inviteeId, common.GetTimestamp(), common.GetRandomString(8))
	}

	var inviterId int
	var totalRebateQuota int
	receiverRebates := map[int]int{}
	type rebateLog struct {
		ReceiverId  int
		Level       int
		RebateQuota int
	}
	var rebateLogs []rebateLog
	err := DB.Transaction(func(tx *gorm.DB) error {
		type inviteUserRef struct {
			Id         int `gorm:"column:id"`
			InviterId  int `gorm:"column:inviter_id"`
			ProviderId int `gorm:"column:provider_id"`
		}

		var invitee inviteUserRef
		if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id").Where("id = ?", inviteeId).Take(&invitee).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if invitee.InviterId <= 0 {
			return nil
		}

		providerId := rebateCtx.ProviderId
		var pricing ProviderModelPricing
		query := tx.Where("provider_id = ? AND enabled = ?", providerId, true)
		if rebateCtx.ProviderPricingId > 0 {
			query = query.Where("id = ?", rebateCtx.ProviderPricingId)
		} else {
			query = query.Where("public_model_name = ?", rebateCtx.PublicModelName)
		}
		if err := query.First(&pricing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		level1Ratio := pricing.ConsumeRebateRatioLevel1
		level2Ratio := 0.0
		if level1Ratio <= 0 && level2Ratio <= 0 {
			return nil
		}
		publicModelName := pricing.PublicModelName
		baseModelName := pricing.BaseModelName

		var level1Inviter inviteUserRef
		if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id").Where("id = ?", invitee.InviterId).Take(&level1Inviter).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		applyLevel := func(level int, receiverId int, ratio float64) error {
			if receiverId <= 0 || ratio <= 0 {
				return nil
			}
			rebateQuota := int(decimal.NewFromInt(int64(paidQuota)).Mul(decimal.NewFromFloat(ratio)).Div(decimal.NewFromInt(100)).IntPart())
			if rebateQuota <= 0 {
				return nil
			}
			var receiver inviteUserRef
			if err := tx.Model(&User{}).Select("id", "provider_id").Where("id = ?", receiverId).Take(&receiver).Error; err != nil {
				return err
			}
			rebate := &ConsumeRebate{
				ProviderId:        receiver.ProviderId,
				InviterId:         receiverId,
				InviteeId:         inviteeId,
				RequestId:         requestId,
				Level:             level,
				SourceQuota:       paidQuota,
				RebateRatio:       ratio,
				RebateQuota:       rebateQuota,
				ProviderPricingId: pricing.Id,
				PublicModelName:   publicModelName,
				BaseModelName:     baseModelName,
				CreatedAt:         common.GetTimestamp(),
			}
			if err := tx.Create(rebate).Error; err != nil {
				return err
			}
			if err := tx.Model(&User{}).Where("id = ?", receiverId).Updates(map[string]interface{}{
				"quota":        gorm.Expr("quota + ?", rebateQuota),
				"reward_quota": gorm.Expr("reward_quota + ?", rebateQuota),
			}).Error; err != nil {
				return err
			}
			if receiver.ProviderId > 0 {
				if err := CreateRewardRecordTx(tx, &RewardRecord{
					ProviderId:  receiver.ProviderId,
					UserId:      receiverId,
					SourceType:  "consume_rebate",
					SourceId:    rebate.Id,
					Quota:       rebateQuota,
					Description: "invite consume rebate",
				}); err != nil {
					return err
				}
			}
			totalRebateQuota += rebateQuota
			receiverRebates[receiverId] += rebateQuota
			if level == 1 {
				inviterId = receiverId
			}
			rebateLogs = append(rebateLogs, rebateLog{ReceiverId: receiverId, Level: level, RebateQuota: rebateQuota})
			return nil
		}

		if err := applyLevel(1, level1Inviter.Id, level1Ratio); err != nil {
			return err
		}

		if level1Inviter.InviterId > 0 && level2Ratio > 0 {
			var level2Inviter inviteUserRef
			if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id").Where("id = ?", level1Inviter.InviterId).Take(&level2Inviter).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			if err := applyLevel(2, level2Inviter.Id, level2Ratio); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	for receiverId, rebateQuota := range receiverRebates {
		asyncIncrUserQuotaCache(receiverId, rebateQuota)
	}
	for _, item := range rebateLogs {
		RecordLog(item.ReceiverId, LogTypeTopup, fmt.Sprintf("invite consume rebate level %d credited %s, source user ID %d, request ID %s", item.Level, logger.LogQuota(item.RebateQuota), inviteeId, requestId))
	}
	return inviterId, totalRebateQuota, nil
}
