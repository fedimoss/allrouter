package model

import (
	"errors"
	"fmt"

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
	ProfitSettled     bool   `json:"profit_settled" gorm:"default:false;index"`
	OwnerCostSettled  bool   `json:"owner_cost_settled" gorm:"default:false;index"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
}

func (ProviderProfit) TableName() string {
	return "provider_profits"
}

func ApplyProviderProfit(record *ProviderProfit) (bool, error) {
	if record == nil || record.ProviderId <= 0 || record.OwnerUserId <= 0 || record.ProviderUserId <= 0 || record.RequestId == "" {
		return false, nil
	}
	if record.ProfitQuota <= 0 && record.OwnerCostQuota <= 0 {
		return false, nil
	}
	if record.CreatedAt == 0 {
		record.CreatedAt = common.GetTimestamp()
	}
	applied := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing ProviderProfit
		if err := tx.Where("request_id = ?", record.RequestId).First(&existing).Error; err == nil {
			return nil
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

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
		applied = true
		return nil
	})
	return applied, err
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
