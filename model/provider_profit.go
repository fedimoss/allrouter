package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProviderProfit struct {
	Id                int    `json:"id"`
	ProviderId        int    `json:"provider_id" gorm:"index;not null"`
	OwnerUserId       int    `json:"owner_user_id" gorm:"index;not null"`
	ProviderUserId    int    `json:"provider_user_id" gorm:"index;not null"`
	RequestId         string `json:"request_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	PublicModelName   string `json:"public_model_name" gorm:"type:varchar(255)"`
	BaseModelName     string `json:"base_model_name" gorm:"type:varchar(255)"`
	ProviderUserQuota int    `json:"provider_user_quota"`
	BaseCostQuota     int    `json:"base_cost_quota"`
	PaidQuota         int    `json:"paid_quota"`
	CoveredCostQuota  int    `json:"covered_cost_quota"`
	OwnerCostQuota    int    `json:"owner_cost_quota"`
	ProfitQuota       int    `json:"profit_quota"`
	GrossProfitQuota  int    `json:"gross_profit_quota" gorm:"default:0"`
	RebateQuota       int    `json:"rebate_quota" gorm:"default:0"`
	ProfitSettled     bool   `json:"profit_settled" gorm:"default:false;index"`
	OwnerCostSettled  bool   `json:"owner_cost_settled" gorm:"default:false;index"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
}

func (ProviderProfit) TableName() string {
	return "provider_profits"
}

type ProviderProfitSummary struct {
	ProviderUserQuota int64 `json:"provider_user_quota"`
	BaseCostQuota     int64 `json:"base_cost_quota"`
	PaidQuota         int64 `json:"paid_quota"`
	CoveredCostQuota  int64 `json:"covered_cost_quota"`
	OwnerCostQuota    int64 `json:"owner_cost_quota"`
	ProfitQuota       int64 `json:"profit_quota"`
	GrossProfitQuota  int64 `json:"gross_profit_quota"`
	RebateQuota       int64 `json:"rebate_quota"`
}

type ProviderProfitApplyResult struct {
	Applied          bool
	GrossProfitQuota int
	RebateQuota      int
}

type providerProfitRebateLog struct {
	ReceiverId  int
	Level       int
	RebateQuota int
}

func buildProviderProfitQuery(providerId int, startTimestamp int64, endTimestamp int64, providerUserId int, modelName string, requestId string) *gorm.DB {
	tx := DB.Model(&ProviderProfit{}).Where("provider_id = ?", providerId)
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if providerUserId > 0 {
		tx = tx.Where("provider_user_id = ?", providerUserId)
	}
	modelName = strings.TrimSpace(modelName)
	if modelName != "" {
		tx = tx.Where("(public_model_name LIKE ? OR base_model_name LIKE ?)", "%"+modelName+"%", "%"+modelName+"%")
	}
	requestId = strings.TrimSpace(requestId)
	if requestId != "" {
		tx = tx.Where("request_id = ?", requestId)
	}
	return tx
}

func GetProviderProfits(providerId int, startTimestamp int64, endTimestamp int64, providerUserId int, modelName string, requestId string, startIdx int, num int) (records []*ProviderProfit, total int64, summary ProviderProfitSummary, err error) {
	if providerId <= 0 {
		return nil, 0, summary, nil
	}
	if err = buildProviderProfitQuery(providerId, startTimestamp, endTimestamp, providerUserId, modelName, requestId).Count(&total).Error; err != nil {
		return nil, 0, summary, err
	}
	type summaryRow struct {
		ProviderUserQuota int64
		BaseCostQuota     int64
		PaidQuota         int64
		CoveredCostQuota  int64
		OwnerCostQuota    int64
		ProfitQuota       int64
		GrossProfitQuota  int64
		RebateQuota       int64
	}
	var row summaryRow
	if err = buildProviderProfitQuery(providerId, startTimestamp, endTimestamp, providerUserId, modelName, requestId).
		Select("COALESCE(SUM(provider_user_quota), 0) AS provider_user_quota, COALESCE(SUM(base_cost_quota), 0) AS base_cost_quota, COALESCE(SUM(paid_quota), 0) AS paid_quota, COALESCE(SUM(covered_cost_quota), 0) AS covered_cost_quota, COALESCE(SUM(owner_cost_quota), 0) AS owner_cost_quota, COALESCE(SUM(profit_quota), 0) AS profit_quota, COALESCE(SUM(gross_profit_quota), 0) AS gross_profit_quota, COALESCE(SUM(rebate_quota), 0) AS rebate_quota").
		Scan(&row).Error; err != nil {
		return nil, 0, summary, err
	}
	summary = ProviderProfitSummary(row)
	err = buildProviderProfitQuery(providerId, startTimestamp, endTimestamp, providerUserId, modelName, requestId).
		Order("id desc").
		Limit(num).
		Offset(startIdx).
		Find(&records).Error
	return records, total, summary, err
}

func applyProviderProfitRebatesTx(tx *gorm.DB, record *ProviderProfit, grossProfitQuota int) (int, map[int]int, []providerProfitRebateLog, error) {
	if tx == nil || record == nil || grossProfitQuota <= 0 || record.ProviderId <= 0 || record.ProviderUserId <= 0 || record.RequestId == "" {
		return 0, nil, nil, nil
	}

	type inviteUserRef struct {
		Id         int `gorm:"column:id"`
		InviterId  int `gorm:"column:inviter_id"`
		ProviderId int `gorm:"column:provider_id"`
	}

	var invitee inviteUserRef
	if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id").Where("id = ?", record.ProviderUserId).Take(&invitee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil, nil, nil
		}
		return 0, nil, nil, err
	}
	if invitee.InviterId <= 0 {
		return 0, nil, nil, nil
	}

	var pricing ProviderModelPricing
	if err := tx.Where("provider_id = ? AND public_model_name = ? AND enabled = ?", record.ProviderId, record.PublicModelName, true).First(&pricing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil, nil, nil
		}
		return 0, nil, nil, err
	}
	level1Ratio := pricing.ConsumeRebateRatioLevel1
	level2Ratio := 0.0
	if level1Ratio <= 0 && level2Ratio <= 0 {
		return 0, nil, nil, nil
	}

	var level1Inviter inviteUserRef
	if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id").Where("id = ?", invitee.InviterId).Take(&level1Inviter).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil, nil, nil
		}
		return 0, nil, nil, err
	}

	totalRebateQuota := 0
	receiverRebates := map[int]int{}
	rebateLogs := make([]providerProfitRebateLog, 0, 2)

	applyLevel := func(level int, receiverId int, ratio float64) error {
		if receiverId <= 0 || ratio <= 0 {
			return nil
		}
		remainingProfit := grossProfitQuota - totalRebateQuota
		if remainingProfit <= 0 {
			return nil
		}
		rebateQuota := int(decimal.NewFromInt(int64(grossProfitQuota)).Mul(decimal.NewFromFloat(ratio)).Div(decimal.NewFromInt(100)).IntPart())
		if rebateQuota <= 0 {
			return nil
		}
		if rebateQuota > remainingProfit {
			rebateQuota = remainingProfit
		}
		var receiver inviteUserRef
		if err := tx.Model(&User{}).Select("id", "provider_id").Where("id = ?", receiverId).Take(&receiver).Error; err != nil {
			return err
		}
		rebate := &ConsumeRebate{
			ProviderId:        receiver.ProviderId,
			InviterId:         receiverId,
			InviteeId:         record.ProviderUserId,
			RequestId:         record.RequestId,
			Level:             level,
			SourceQuota:       grossProfitQuota,
			RebateRatio:       ratio,
			RebateQuota:       rebateQuota,
			ProviderPricingId: pricing.Id,
			PublicModelName:   pricing.PublicModelName,
			BaseModelName:     pricing.BaseModelName,
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
				Description: "provider profit rebate",
			}); err != nil {
				return err
			}
		}
		totalRebateQuota += rebateQuota
		receiverRebates[receiverId] += rebateQuota
		rebateLogs = append(rebateLogs, providerProfitRebateLog{ReceiverId: receiverId, Level: level, RebateQuota: rebateQuota})
		return nil
	}

	if err := applyLevel(1, level1Inviter.Id, level1Ratio); err != nil {
		return 0, nil, nil, err
	}
	if level1Inviter.InviterId > 0 && level2Ratio > 0 {
		var level2Inviter inviteUserRef
		if err := tx.Model(&User{}).Select("id", "inviter_id", "provider_id").Where("id = ?", level1Inviter.InviterId).Take(&level2Inviter).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return totalRebateQuota, receiverRebates, rebateLogs, nil
			}
			return 0, nil, nil, err
		}
		if err := applyLevel(2, level2Inviter.Id, level2Ratio); err != nil {
			return 0, nil, nil, err
		}
	}
	return totalRebateQuota, receiverRebates, rebateLogs, nil
}

func ApplyProviderProfit(record *ProviderProfit) (*ProviderProfitApplyResult, error) {
	result := &ProviderProfitApplyResult{}
	if record == nil || record.ProviderId <= 0 || record.OwnerUserId <= 0 || record.ProviderUserId <= 0 || record.RequestId == "" {
		return result, nil
	}
	if record.ProfitQuota <= 0 && record.OwnerCostQuota <= 0 {
		return result, nil
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = common.GetTimestamp()
	}
	grossProfitQuota := record.ProfitQuota
	receiverRebates := map[int]int{}
	var rebateLogs []providerProfitRebateLog
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing ProviderProfit
		if err := tx.Where("request_id = ?", record.RequestId).First(&existing).Error; err == nil {
			return nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		rebateQuota, rebates, logs, err := applyProviderProfitRebatesTx(tx, record, grossProfitQuota)
		if err != nil {
			return err
		}
		result.GrossProfitQuota = grossProfitQuota
		result.RebateQuota = rebateQuota
		receiverRebates = rebates
		rebateLogs = logs
		netProfitQuota := grossProfitQuota - rebateQuota
		if netProfitQuota < 0 {
			netProfitQuota = 0
		}
		record.GrossProfitQuota = grossProfitQuota
		record.RebateQuota = rebateQuota
		record.ProfitQuota = netProfitQuota
		record.ProfitSettled = record.ProfitQuota > 0
		record.OwnerCostSettled = record.OwnerCostQuota > 0
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		if record.OwnerCostQuota > 0 {
			var owner User
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Select("id", "quota", "reward_quota").
				Where("id = ?", record.OwnerUserId).
				First(&owner).Error; err != nil {
				return err
			}
			if owner.Quota < record.OwnerCostQuota {
				return fmt.Errorf("provider owner quota is not enough, user quota: %s, need quota: %s", logger.FormatQuota(owner.Quota), logger.FormatQuota(record.OwnerCostQuota))
			}
			rewardUsed := owner.RewardQuota
			if rewardUsed > owner.Quota {
				rewardUsed = owner.Quota
			}
			if rewardUsed < 0 {
				rewardUsed = 0
			}
			if rewardUsed > record.OwnerCostQuota {
				rewardUsed = record.OwnerCostQuota
			}
			if err := tx.Model(&User{}).Where("id = ?", record.OwnerUserId).Updates(map[string]interface{}{
				"quota":        gorm.Expr("quota - ?", record.OwnerCostQuota),
				"reward_quota": gorm.Expr("reward_quota - ?", rewardUsed),
			}).Error; err != nil {
				return err
			}
		}
		if record.ProfitQuota > 0 {
			if err := tx.Model(&User{}).Where("id = ?", record.OwnerUserId).Update("quota", gorm.Expr("quota + ?", record.ProfitQuota)).Error; err != nil {
				return err
			}
			money := decimal.NewFromInt(int64(record.ProfitQuota)).
				Div(decimal.NewFromFloat(common.QuotaPerUnit)).
				Round(6).
				InexactFloat64()
			topUp := &TopUp{
				ProviderId:    record.ProviderId,
				UserId:        record.OwnerUserId,
				Amount:        int64(record.ProfitQuota),
				Money:         money,
				TradeNo:       fmt.Sprintf("PROVIDER-PROFIT-%d", record.Id),
				PaymentMethod: TopUpPaymentMethodProviderProfit,
				BizType:       TopUpBizTypePayment,
				SourceID:      record.Id,
				CreateTime:    record.CreatedAt,
				CompleteTime:  record.CreatedAt,
				Status:        common.TopUpStatusSuccess,
				Currency:      "USD",
				OriginalMoney: money,
			}
			if err := tx.Create(topUp).Error; err != nil {
				return err
			}
		}
		result.Applied = true
		return nil
	})
	if err != nil {
		return result, err
	}
	for receiverId, rebateQuota := range receiverRebates {
		asyncIncrUserQuotaCache(receiverId, rebateQuota)
	}
	for _, item := range rebateLogs {
		RecordLog(item.ReceiverId, LogTypeTopup, fmt.Sprintf("provider profit rebate level %d credited %s, source user ID %d, request ID %s", item.Level, logger.LogQuota(item.RebateQuota), record.ProviderUserId, record.RequestId))
	}
	return result, nil
}

func LogProviderProfit(record *ProviderProfit) {
	if record == nil || record.OwnerUserId <= 0 {
		return
	}
	if record.OwnerCostQuota > 0 {
		if err := cacheDecrUserQuota(record.OwnerUserId, int64(record.OwnerCostQuota)); err != nil {
			common.SysLog("failed to decrease provider owner quota cache: " + err.Error())
		}
	}
	RecordLog(record.OwnerUserId, LogTypeSystem, fmt.Sprintf("provider settlement, user ID %d, request ID %s, user charged %s, base cost %s, paid quota %s, covered cost %s, owner cost %s, profit %s",
		record.ProviderUserId,
		record.RequestId,
		logger.LogQuota(record.ProviderUserQuota),
		logger.LogQuota(record.BaseCostQuota),
		logger.LogQuota(record.PaidQuota),
		logger.LogQuota(record.CoveredCostQuota),
		logger.LogQuota(record.OwnerCostQuota),
		logger.LogQuota(record.ProfitQuota),
	))
	if record.ProfitQuota > 0 {
		asyncIncrUserQuotaCache(record.OwnerUserId, record.ProfitQuota)
		RecordLog(record.OwnerUserId, LogTypeTopup, fmt.Sprintf("provider profit credited %s, provider user ID %d, request ID %s", logger.LogQuota(record.ProfitQuota), record.ProviderUserId, record.RequestId))
	}
}
