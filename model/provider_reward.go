package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// ProviderRewardConfig stores per-provider reward policy overrides.
type ProviderRewardConfig struct {
	Id                             int     `json:"id"`
	ProviderId                     int     `json:"provider_id" gorm:"uniqueIndex;not null"`
	QuotaForNewUser                int     `json:"quota_for_new_user" gorm:"default:0"`
	QuotaForInviter                int     `json:"quota_for_inviter" gorm:"default:0"`
	QuotaForInvitee                int     `json:"quota_for_invitee" gorm:"default:0"`
	CheckinEnabled                 bool    `json:"checkin_enabled" gorm:"default:false"`
	CheckinMinQuota                int     `json:"checkin_min_quota" gorm:"default:0"`
	CheckinMaxQuota                int     `json:"checkin_max_quota" gorm:"default:0"`
	InviteTopupRebateRatio         float64 `json:"invite_topup_rebate_ratio" gorm:"type:decimal(10,6);default:0"`
	InviteConsumeRebateRatioLevel2 float64 `json:"invite_consume_rebate_ratio_level2" gorm:"type:decimal(10,6);default:0"`
	CreatedAt                      int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt                      int64   `json:"updated_at" gorm:"bigint"`
}

func (ProviderRewardConfig) TableName() string {
	return "provider_reward_configs"
}

func (p *ProviderRewardConfig) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if p.CreatedAt == 0 {
		p.CreatedAt = now
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = now
	}
	return nil
}

func (p *ProviderRewardConfig) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = common.GetTimestamp()
	return nil
}

func defaultProviderRewardConfig(providerId int) *ProviderRewardConfig {
	checkin := operation_setting.GetCheckinSetting()
	return &ProviderRewardConfig{
		ProviderId:                     providerId,
		QuotaForNewUser:                common.QuotaForNewUser,
		QuotaForInviter:                common.QuotaForInviter,
		QuotaForInvitee:                common.QuotaForInvitee,
		CheckinEnabled:                 checkin.Enabled,
		CheckinMinQuota:                checkin.MinQuota,
		CheckinMaxQuota:                checkin.MaxQuota,
		InviteTopupRebateRatio:         common.InviteTopupRebateRatio,
		InviteConsumeRebateRatioLevel2: common.InviteConsumeRebateRatioLevel2,
	}
}

// GetProviderRewardConfig returns provider-specific reward config with global fallback.
func GetProviderRewardConfig(providerId int) (*ProviderRewardConfig, error) {
	if providerId <= 0 {
		return defaultProviderRewardConfig(0), nil
	}
	var cfg ProviderRewardConfig
	err := DB.Where("provider_id = ?", providerId).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultProviderRewardConfig(providerId), nil
		}
		return nil, err
	}
	if cfg.ProviderId == 0 {
		cfg.ProviderId = providerId
	}
	return &cfg, nil
}

// UpsertProviderRewardConfig saves provider reward config.
func UpsertProviderRewardConfig(cfg *ProviderRewardConfig) error {
	if cfg == nil || cfg.ProviderId <= 0 {
		return errors.New("provider id is empty")
	}
	return DB.Save(cfg).Error
}

type ProviderRewardSummary struct {
	ProviderId         int   `json:"provider_id"`
	NewUserQuota       int64 `json:"new_user_quota"`
	InviterQuota       int64 `json:"inviter_quota"`
	InviteeQuota       int64 `json:"invitee_quota"`
	CheckinQuota       int64 `json:"checkin_quota"`
	RedemptionQuota    int64 `json:"redemption_quota"`
	ConsumeRebateQuota int64 `json:"consume_rebate_quota"`
	TopUpRebateQuota   int64 `json:"topup_rebate_quota"`
	WelfareQuota       int64 `json:"welfare_quota"`
}

func GetProviderRewardSummary(providerId int) (*ProviderRewardSummary, error) {
	if providerId <= 0 {
		return &ProviderRewardSummary{}, nil
	}
	summary := &ProviderRewardSummary{ProviderId: providerId}
	type sumResult struct {
		Total int64 `gorm:"column:total"`
	}
	var row sumResult
	if err := DB.Model(&RewardRecord{}).Select("COALESCE(SUM(quota), 0) AS total").
		Where("provider_id = ? AND source_type = ?", providerId, "new_user").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	summary.NewUserQuota = row.Total

	if err := DB.Model(&RewardRecord{}).Select("COALESCE(SUM(quota), 0) AS total").
		Where("provider_id = ? AND source_type = ?", providerId, "inviter_reward").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	summary.InviterQuota = row.Total

	if err := DB.Model(&RewardRecord{}).Select("COALESCE(SUM(quota), 0) AS total").
		Where("provider_id = ? AND source_type = ?", providerId, "invitee_reward").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	summary.InviteeQuota = row.Total

	if err := DB.Model(&RewardRecord{}).Select("COALESCE(SUM(quota), 0) AS total").
		Where("provider_id = ? AND source_type = ?", providerId, "checkin").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	summary.CheckinQuota = row.Total

	if err := DB.Model(&RewardRecord{}).Select("COALESCE(SUM(quota), 0) AS total").
		Where("provider_id = ? AND source_type = ?", providerId, "redemption").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	summary.RedemptionQuota = row.Total

	if err := DB.Model(&RewardRecord{}).Select("COALESCE(SUM(quota), 0) AS total").
		Where("provider_id = ? AND source_type = ?", providerId, "consume_rebate").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	summary.ConsumeRebateQuota = row.Total

	if err := DB.Model(&RewardRecord{}).Select("COALESCE(SUM(quota), 0) AS total").
		Where("provider_id = ? AND source_type = ?", providerId, "topup_rebate").
		Scan(&row).Error; err != nil {
		return nil, err
	}
	summary.TopUpRebateQuota = row.Total

	summary.WelfareQuota = summary.NewUserQuota + summary.InviterQuota + summary.InviteeQuota + summary.CheckinQuota + summary.RedemptionQuota + summary.ConsumeRebateQuota + summary.TopUpRebateQuota
	return summary, nil
}
