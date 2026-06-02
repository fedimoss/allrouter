package model

import (
	"errors"
	"net"
	"net/url"
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
	Id               int     `json:"id"`
	ProviderId       int     `json:"provider_id" gorm:"uniqueIndex;not null"`
	SiteName         string  `json:"site_name" gorm:"type:varchar(128)"`
	Logo             string  `json:"logo" gorm:"type:text"`
	ThemeColor       string  `json:"theme_color" gorm:"type:varchar(32);default:''"`
	SecondaryColor   string  `json:"secondary_color" gorm:"type:varchar(32);default:''"`
	LoginBackground  string  `json:"login_background" gorm:"type:text"`
	HomeModules      string  `json:"home_modules" gorm:"type:text"`
	NavModules       string  `json:"nav_modules" gorm:"type:text"`
	PricingDisplay   string  `json:"pricing_display" gorm:"type:text"`
	Announcement     string  `json:"announcement" gorm:"type:text"`
	FooterText       string  `json:"footer_text" gorm:"type:text"`
	SupportUrl       string  `json:"support_url" gorm:"type:text"`
	ImportPriceRatio float64 `json:"import_price_ratio" gorm:"type:decimal(10,6);not null;default:1"`
	CreatedAt        int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt        int64   `json:"updated_at" gorm:"bigint"`
	WechatSupport    string  `json:"wechat_support" gorm:"type:text"`
	QQSupport        string  `json:"qq_support" gorm:"type:text"`
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

type TreeProvider struct {
	*User

	Children []*TreeProvider `json:"children"`

	// 分页信息（关键）
	HasMoreChildren bool `json:"has_more_children"`
}

type childCountRow struct {
	InviterId int
	Count     int64
}

func NormalizeProviderDomain(domain string) string {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if parsedURL, err := url.Parse(domain); err == nil && parsedURL.Host != "" {
		domain = parsedURL.Host
	}
	if host, _, err := net.SplitHostPort(domain); err == nil {
		domain = host
	}
	if idx := strings.Index(domain, "/"); idx > -1 {
		domain = domain[:idx]
	}
	return strings.Trim(strings.TrimSuffix(domain, "."), "[]")
}

func ProviderDomainLookupCandidates(domain string) []string {
	domain = NormalizeProviderDomain(domain)
	if domain == "" {
		return nil
	}
	candidates := []string{domain}
	if strings.HasPrefix(domain, "www.") {
		candidates = append(candidates, strings.TrimPrefix(domain, "www."))
	} else {
		candidates = append(candidates, "www."+domain)
	}
	return candidates
}

func GetProviderContextByDomain(domain string) (*ProviderContext, error) {
	domain = NormalizeProviderDomain(domain)
	if domain == "" {
		return nil, nil
	}
	var providerDomain ProviderDomain
	result := DB.Where("domain = ? AND status = ?", domain, ProviderDomainStatusVerified).Find(&providerDomain)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		for _, candidate := range ProviderDomainLookupCandidates(domain) {
			if candidate == domain {
				continue
			}
			result = DB.Where("domain = ? AND status = ?", candidate, ProviderDomainStatusVerified).Find(&providerDomain)
			if result.Error != nil {
				return nil, result.Error
			}
			if result.RowsAffected > 0 {
				break
			}
		}
		if result.RowsAffected == 0 {
			return nil, nil
		}
	}
	var provider Provider
	if err := DB.Where("id = ? AND status = ?", providerDomain.ProviderId, ProviderStatusEnabled).First(&provider).Error; err != nil {
		return nil, err
	}
	var cfg ProviderConfig
	_ = DB.Where("provider_id = ?", provider.Id).Find(&cfg).Error
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
		for _, normalized := range ProviderDomainLookupCandidates(domain.Domain) {
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			verifiedDomains = append(verifiedDomains, normalized)
		}
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

func GetProviderById(id int) (*Provider, error) {
	if id == 0 {
		return nil, errors.New("provider id is empty")
	}
	var provider Provider
	err := DB.Where("id = ?", id).First(&provider).Error
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

func GetTreeChilendUsers(userId int, parentId int, pageInfo *common.PageInfo) ([]*TreeProvider, error) {
	if parentId == 0 && userId != 1 { // userId为1是根超级管理员（目前仅限定根超级管理员能查看各个服务商）
		return nil, errors.New("Insufficient permissions")
	}
	provider, err := GetProviderByOwnerUserId(userId)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("current user is not provider")
	}
	//查看parentId的归属在哪个服务商下
	root, err := GetProviderRoot(parentId, userId)
	if err != nil {
		return nil, err
	}
	if root.Status != 1 {
		return nil, errors.New("provider is not enabled")
	}
	if root.OwnerUserId != userId {
		return nil, errors.New("current user is not provider root") //非法请求
	}
	// TODO: 查询当前节点的子节点
	ups, total, err := GetChilendsProvide(parentId, root, pageInfo)
	if pageInfo != nil {
		pageInfo.SetTotal(total)
	}

	if err != nil {
		return nil, err
	}
	return ups, nil
}

// 查询当前用户归属哪个服务商（为了各个服务商数据隔离）
func GetProviderRoot(userId int, providerId int) (*Provider, error) {

	var currnode *User
	err := DB.Model(&User{}).Where(" id = ?", userId).First(&currnode).Error
	if err != nil {
		return nil, err
	}
	if userId == providerId { //当前用户是服务商本人
		return GetProviderByOwnerUserId(currnode.Id)
	}
	p, err := GetProviderById(currnode.ProviderId)

	return p, nil
}

func GetChilendsProvide(parentId int, provider *Provider, pageInfo *common.PageInfo) ([]*TreeProvider, int, error) {
	var us []*User
	var tree []*TreeProvider
	var err error
	var total int64
	if pageInfo == nil {
		pageInfo = &common.PageInfo{Page: 1, PageSize: common.ItemsPerPage}
	}
	query := DB.Model(&User{})
	if parentId == provider.OwnerUserId {
		query = query.Where("provider_id = ? and inviter_id IN (0,?)  ", provider.Id, parentId)
	} else {
		query = query.Where("provider_id = ? AND inviter_id = ?", provider.Id, parentId)
	}

	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err = query.
		Order("id asc").
		Limit(pageInfo.GetPageSize()).
		Offset(pageInfo.GetStartIdx()).
		Find(&us).Error; err != nil {
		return nil, 0, err
	}

	//当前层userids
	userids := make([]int, 0, len(us))
	for _, u := range us {
		userids = append(userids, u.Id)
	}
	var childCounts []childCountRow
	if len(userids) > 0 {
		//查询是否有下一层
		err = DB.Model(&User{}).Select("inviter_id, COUNT(*) as count").Where("provider_id = ? AND inviter_id IN ?", provider.Id, userids).Group("inviter_id").Scan(&childCounts).Error
		if err != nil {
			return nil, 0, err
		}
	}
	childRows := make(map[int]int64, len(childCounts))
	for _, row := range childCounts {
		childRows[row.InviterId] = row.Count
	}
	for _, u := range us {
		tree = append(tree, &TreeProvider{
			User:            u,
			HasMoreChildren: childRows[u.Id] > 0,
		})
	}
	return tree, int(total), nil
}
