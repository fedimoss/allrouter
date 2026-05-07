package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ConsumeRebate records rebates generated from invitees consuming paid quota.
type ConsumeRebate struct {
	Id          int     `json:"id"`
	InviterId   int     `json:"inviter_id" gorm:"index"`
	InviteeId   int     `json:"invitee_id" gorm:"index"`
	RequestId   string  `json:"request_id" gorm:"type:varchar(64);uniqueIndex"`
	SourceQuota int     `json:"source_quota"`
	RebateRatio float64 `json:"rebate_ratio"`
	RebateQuota int     `json:"rebate_quota"`
	CreatedAt   int64   `json:"created_at" gorm:"bigint;index"`
}

func (ConsumeRebate) TableName() string {
	return "consume_rebates"
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

// 计算邀请消费返利总和
func SumConsumeRebateQuotaByInviteeId(userId int) (int64, error) {
	var totalQuota int64
	err := DB.Model(&ConsumeRebate{}).
		Select("COALESCE(SUM(rebate_quota), 0)").
		Where("invitee_id = ?", userId).
		Scan(&totalQuota).Error
	return totalQuota, err
}

// 当被邀请人实际使用充值额度消费时，按比例给邀请人返利。返利入账也算奖励，所以邀请人会同时增加 quota 和 reward_quota
func ApplyInviteConsumeRebate(inviteeId int, requestId string, paidQuota int) (int, int, error) {
	if inviteeId <= 0 || paidQuota <= 0 || common.InviteTopupRebateRatio <= 0 {
		return 0, 0, nil
	}
	if requestId == "" {
		requestId = fmt.Sprintf("consume-rebate-%d-%d-%s", inviteeId, common.GetTimestamp(), common.GetRandomString(8))
	}

	var inviterId int
	var rebateQuota int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var invitee struct {
			InviterId int `gorm:"column:inviter_id"`
		}
		if err := tx.Model(&User{}).Select("inviter_id").Where("id = ?", inviteeId).Take(&invitee).Error; err != nil {
			return err
		}
		if invitee.InviterId <= 0 {
			return nil
		}

		rebateQuota = int(decimal.NewFromInt(int64(paidQuota)).
			Mul(decimal.NewFromFloat(common.InviteTopupRebateRatio)).
			Div(decimal.NewFromInt(100)).
			IntPart())
		if rebateQuota <= 0 {
			return nil
		}

		rebate := &ConsumeRebate{
			InviterId:   invitee.InviterId,
			InviteeId:   inviteeId,
			RequestId:   requestId,
			SourceQuota: paidQuota,
			RebateRatio: common.InviteTopupRebateRatio,
			RebateQuota: rebateQuota,
			CreatedAt:   common.GetTimestamp(),
		}
		if err := tx.Create(rebate).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", invitee.InviterId).Updates(map[string]interface{}{
			"quota":        gorm.Expr("quota + ?", rebateQuota),
			"reward_quota": gorm.Expr("reward_quota + ?", rebateQuota),
		}).Error; err != nil {
			return err
		}
		inviterId = invitee.InviterId
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	if inviterId > 0 && rebateQuota > 0 {
		asyncIncrUserQuotaCache(inviterId, rebateQuota)
		RecordLog(inviterId, LogTypeTopup, fmt.Sprintf("invite consume rebate credited %s, source user ID %d, request ID %s", logger.LogQuota(rebateQuota), inviteeId, requestId))
	}
	return inviterId, rebateQuota, nil
}
