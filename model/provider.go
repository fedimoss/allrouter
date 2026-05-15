package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ProviderStatusDisabled = 0
	ProviderStatusEnabled  = 1

	ProviderDomainStatusPending  = 0
	ProviderDomainStatusVerified = 1

	ProviderPricingTypeRatio = "ratio"
	ProviderPricingTypeDelta = "delta"
)

type Provider struct {
	Id          int    `json:"id"`
	OwnerUserId int    `json:"owner_user_id" gorm:"index;not null"`
	Name        string `json:"name" gorm:"type:varchar(128);not null"`
	Status      int    `json:"status" gorm:"type:int;default:1;index"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

type ProviderDomain struct {
	Id          int    `json:"id"`
	ProviderId  int    `json:"provider_id" gorm:"index;not null"`
	Domain      string `json:"domain" gorm:"type:varchar(255);uniqueIndex;not null"`
	Status      int    `json:"status" gorm:"type:int;default:0;index"`
	VerifyToken string `json:"verify_token" gorm:"type:varchar(64)"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

type ProviderConfig struct {
	Id              int    `json:"id"`
	ProviderId      int    `json:"provider_id" gorm:"uniqueIndex;not null"`
	SiteName        string `json:"site_name" gorm:"type:varchar(128)"`
	Logo            string `json:"logo" gorm:"type:text"`
	ThemeColor      string `json:"theme_color" gorm:"type:varchar(32);default:''"`
	SecondaryColor  string `json:"secondary_color" gorm:"type:varchar(32);default:''"`
	LoginBackground string `json:"login_background" gorm:"type:text"`
	HomeModules     string `json:"home_modules" gorm:"type:text"`
	NavModules      string `json:"nav_modules" gorm:"type:text"`
	PricingDisplay  string `json:"pricing_display" gorm:"type:text"`
	Announcement    string `json:"announcement" gorm:"type:text"`
	FooterText      string `json:"footer_text" gorm:"type:text"`
	SupportUrl      string `json:"support_url" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
	WechatSupport   string `json:"wechat_support" gorm:"type:text"`
	QQSupport       string `json:"qq_support" gorm:"type:text"`
}

type ProviderModelPricing struct {
	Id                       int     `json:"id"`
	ProviderId               int     `json:"provider_id" gorm:"index;not null;index:idx_provider_public_model,unique"`
	PublicModelName          string  `json:"public_model_name" gorm:"type:varchar(255);not null;index:idx_provider_public_model,unique"`
	BaseModelName            string  `json:"base_model_name" gorm:"type:varchar(255);not null;index"`
	Enabled                  bool    `json:"enabled" gorm:"default:true;index"`
	PricingType              string  `json:"pricing_type" gorm:"type:varchar(16);not null;default:'ratio'"`
	Ratio                    float64 `json:"ratio" gorm:"type:decimal(18,8);default:1"`
	DeltaModelRatio          float64 `json:"delta_model_ratio" gorm:"type:decimal(18,8);default:0"`
	DeltaModelPrice          float64 `json:"delta_model_price" gorm:"type:decimal(18,8);default:0"`
	ConsumeRebateRatioLevel1 float64 `json:"consume_rebate_ratio_level1" gorm:"type:decimal(10,6);not null;default:0"`
	ConsumeRebateRatioLevel2 float64 `json:"consume_rebate_ratio_level2" gorm:"type:decimal(10,6);not null;default:0"`
	CreatedAt                int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt                int64   `json:"updated_at" gorm:"bigint"`
}

type ProviderContext struct {
	ProviderId  int
	OwnerUserId int
	Name        string
	Domain      string
	Config      *ProviderConfig
}

func normalizeProviderDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if idx := strings.Index(domain, ":"); idx > -1 {
		domain = domain[:idx]
	}
	return domain
}

func GetProviderContextByDomain(domain string) (*ProviderContext, error) {
	domain = normalizeProviderDomain(domain)
	if domain == "" {
		return nil, nil
	}
	var providerDomain ProviderDomain
	err := DB.Where("domain = ? AND status = ?", domain, ProviderDomainStatusVerified).First(&providerDomain).Error
	if err != nil {
		return nil, err
	}
	var provider Provider
	if err = DB.Where("id = ? AND status = ?", providerDomain.ProviderId, ProviderStatusEnabled).First(&provider).Error; err != nil {
		return nil, err
	}
	var cfg ProviderConfig
	_ = DB.Where("provider_id = ?", provider.Id).First(&cfg).Error
	ctx := &ProviderContext{
		ProviderId:  provider.Id,
		OwnerUserId: provider.OwnerUserId,
		Name:        provider.Name,
		Domain:      domain,
	}
	if cfg.Id != 0 {
		ctx.Config = &cfg
	}
	return ctx, nil
}

func GetProviderVerifiedDomains(providerId int) ([]string, error) {
	if providerId <= 0 {
		return nil, nil
	}
	var domains []ProviderDomain
	if err := DB.Where("provider_id = ? AND status = ?", providerId, ProviderDomainStatusVerified).Order("id asc").Find(&domains).Error; err != nil {
		return nil, err
	}
	verifiedDomains := make([]string, 0, len(domains))
	seen := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		normalized := normalizeProviderDomain(domain.Domain)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		verifiedDomains = append(verifiedDomains, normalized)
	}
	return verifiedDomains, nil
}

func GetProviderByOwnerUserId(ownerUserId int) (*Provider, error) {
	if ownerUserId == 0 {
		return nil, errors.New("owner user id is empty")
	}
	var provider Provider
	err := DB.Where("owner_user_id = ?", ownerUserId).First(&provider).Error
	return &provider, err
}

func IsProviderOwner(userId int) bool {
	if userId == 0 {
		return false
	}
	var count int64
	if err := DB.Model(&Provider{}).Where("owner_user_id = ? AND status = ?", userId, ProviderStatusEnabled).Count(&count).Error; err != nil {
		common.SysLog("failed to check provider owner: " + err.Error())
		return false
	}
	return count > 0
}

func GetProviderModelPricing(providerId int, publicModelName string) (*ProviderModelPricing, error) {
	if providerId == 0 || strings.TrimSpace(publicModelName) == "" {
		return nil, errors.New("provider id or model name is empty")
	}
	var pricing ProviderModelPricing
	err := DB.Where("provider_id = ? AND public_model_name = ? AND enabled = ?", providerId, publicModelName, true).First(&pricing).Error
	return &pricing, err
}

func (p *ProviderModelPricing) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	p.CreatedAt = now
	p.UpdatedAt = now
	if p.PricingType == "" {
		p.PricingType = ProviderPricingTypeRatio
	}
	if p.Ratio == 0 {
		p.Ratio = 1
	}
	return nil
}

func (p *ProviderModelPricing) BeforeUpdate(tx *gorm.DB) error {
	p.UpdatedAt = time.Now().Unix()
	return nil
}
