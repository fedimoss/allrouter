package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// ApplyInviteConsumeRebate 当被邀请人实际使用充值额度消费时，按比例给邀请人返利。
// 返利入账也算奖励，所以邀请人会同时增加 quota 和 reward_quota。
//
// 兼容两种站点模式：
//   - 主站（provider_id=0）：使用全局 InviteTopupRebateRatio，不要求邀请人开启 invite_consume_rebate_enabled。
//   - 服务商站点（provider_id>0）：使用 ProviderModelPricing 中的 ConsumeRebateRatioLevel1，要求邀请人开启资格。
//
// 幂等性：通过 (request_id, level) 唯一索引实现 INSERT ON CONFLICT DO NOTHING，
// 同一笔结算重复调用不会重复入账。
func ApplyInviteConsumeRebate(inviteeId int, requestId string, paidQuota int, rebateCtx *ConsumeRebateContext) (int, int, error) {
	if inviteeId <= 0 || paidQuota <= 0 {
		return 0, 0, nil
	}
	if requestId == "" {
		requestId = fmt.Sprintf("consume-rebate-%d-%d-%s", inviteeId, common.GetTimestamp(), common.GetRandomString(8))
	}
	// 主站（provider_id=0）使用全局一级消费返佣比例，不需要按用户检查返佣资格。
	// 服务商站点保持原有的模型定价和资格审核策略。
	mainSite := rebateCtx == nil || rebateCtx.ProviderId <= 0
	siteProviderId := 0
	if !mainSite {
		siteProviderId = rebateCtx.ProviderId
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
			Id                         int `gorm:"column:id"`
			InviterId                  int `gorm:"column:inviter_id"`
			ProviderId                 int `gorm:"column:provider_id"`
			InviteConsumeRebateEnabled int `gorm:"column:invite_consume_rebate_enabled"`
		}

		var invitee inviteUserRef
		if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id").Where("id = ?", inviteeId).Take(&invitee).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		// 被邀请人的 provider 必须与返佣站点一致，防止跨站返佣。
		if invitee.ProviderId != siteProviderId || invitee.InviterId <= 0 {
			return nil
		}

		// 确定返佣比例和模型信息。
		// 主站：从 ProviderRewardConfig 读取全局比例。
		// 服务商站点：从 ProviderModelPricing 读取各模型配置的比例。
		level1Ratio := 0.0
		level2Ratio := 0.0
		providerPricingId := 0
		publicModelName := ""
		baseModelName := ""
		if rebateCtx != nil {
			publicModelName = rebateCtx.PublicModelName
			baseModelName = rebateCtx.BaseModelName
		}
		if mainSite {
			rewardCfg, err := GetProviderRewardConfig(0)
			if err != nil {
				return err
			}
			level1Ratio = rewardCfg.InviteTopupRebateRatio
		} else {
			var pricing ProviderModelPricing
			query := tx.Where("provider_id = ? AND enabled = ?", siteProviderId, true)
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
			level1Ratio = pricing.ConsumeRebateRatioLevel1
			providerPricingId = pricing.Id
			publicModelName = pricing.PublicModelName
			baseModelName = pricing.BaseModelName
		}
		if level1Ratio <= 0 && level2Ratio <= 0 {
			return nil
		}

		var level1Inviter inviteUserRef
		if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id", "invite_consume_rebate_enabled").Where("id = ?", invitee.InviterId).Take(&level1Inviter).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		// 一级邀请人必须与返佣站点一致，防止跨站分佣。
		if level1Inviter.ProviderId != siteProviderId {
			return nil
		}
		// 主站不需要邀请人开启分佣资格，服务商站点需要。
		if !mainSite && level1Inviter.InviteConsumeRebateEnabled != 1 {
			return nil
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
			// 返佣接收者也必须与返佣站点一致。
			if receiver.ProviderId != siteProviderId {
				return nil
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
				ProviderPricingId: providerPricingId,
				PublicModelName:   publicModelName,
				BaseModelName:     baseModelName,
				CreatedAt:         common.GetTimestamp(),
			}
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "request_id"}, {Name: "level"}},
				DoNothing: true,
			}).Create(rebate)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				// 同一结算已存在。视为幂等成功，不重复入账。
				return nil
			}
			// 同时增加 quota 和 reward_quota，因为返佣以奖励形式发放。
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
			if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id", "invite_consume_rebate_enabled").Where("id = ?", level1Inviter.InviterId).Take(&level2Inviter).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			// 二级邀请人也必须与返佣站点一致。
			if level2Inviter.ProviderId != siteProviderId {
				return nil
			}
			// 主站不需要二级邀请人开启分佣资格，服务商站点需要。
			if !mainSite && level2Inviter.InviteConsumeRebateEnabled != 1 {
				return nil
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
