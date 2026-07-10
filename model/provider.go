package model

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
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
	HomePageTheme    string  `json:"home_page_theme" gorm:"type:varchar(64);default:''"`
	HomeModules      string  `json:"home_modules" gorm:"type:text"`
	NavModules       string  `json:"nav_modules" gorm:"type:text"`
	PricingDisplay   string  `json:"pricing_display" gorm:"type:text"`
	Announcement     string  `json:"announcement" gorm:"type:text"`
	FooterText       string  `json:"footer_text" gorm:"type:text"`
	SupportUrl       string  `json:"support_url" gorm:"type:text"`
	ImportPriceRatio float64 `json:"import_price_ratio" gorm:"type:decimal(10,6);not null;default:1"`
	// 模型定价自动同步开关：开启后，主站模型新增/下架/恢复时会自动同步该服务商
	ModelPricingSyncEnabled bool `json:"model_pricing_sync_enabled" gorm:"default:false;index"`
	// 上次自动同步的 Unix 时间戳
	ModelPricingSyncLastAt int64 `json:"model_pricing_sync_last_at" gorm:"bigint;default:0"`
	// 上次自动同步结果摘要（JSON），包含新增/软禁用/恢复/跳过的模型名及计数
	ModelPricingSyncLastSummary string `json:"model_pricing_sync_last_summary" gorm:"type:text"`
	CreatedAt                   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt                   int64  `json:"updated_at" gorm:"bigint"`
	WechatSupport               string `json:"wechat_support" gorm:"type:text"`
	QQSupport                   string `json:"qq_support" gorm:"type:text"`
	WechatSupportDesc           string `json:"wechat_support_desc" gorm:"type:text"`
	QQSupportQrcode             string `json:"qq_support_qrcode" gorm:"type:text"`
	TelegramSupport             string `json:"telegram_support" gorm:"type:text"`
	TelegramSupportDesc         string `json:"telegram_support_desc" gorm:"type:text"`
}

type ProviderModelPricing struct {
	Id              int    `json:"id"`
	ProviderId      int    `json:"provider_id" gorm:"index;not null;index:idx_provider_public_model,unique"`
	PublicModelName string `json:"public_model_name" gorm:"type:varchar(255);not null;index:idx_provider_public_model,unique"`
	BaseModelName   string `json:"base_model_name" gorm:"type:varchar(255);not null;index"`
	Enabled         bool   `json:"enabled" gorm:"default:true;index"`
	// 同步软禁用标记：true 表示该行是被自动同步逻辑（主站模型下架）禁用的；
	// 服务商/管理员手动保存时会清为 false，避免后续同步误改服务商的手动状态
	SyncDisabled             bool    `json:"sync_disabled" gorm:"default:false;index"`
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

type providerContextCacheEntry struct {
	Found       bool            `json:"found"`
	ProviderId  int             `json:"provider_id"`
	OwnerUserId int             `json:"owner_user_id"`
	Name        string          `json:"name"`
	Domain      string          `json:"domain"`
	Config      *ProviderConfig `json:"config,omitempty"`
}

type providerPublicConfigCacheEntry struct {
	Found  bool           `json:"found"`
	Config ProviderConfig `json:"config"`
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

const (
	providerDomainContextCacheNamespace = "new-api:provider_domain_context:v1"
	providerPublicConfigCacheNamespace  = "new-api:provider_public_config:v1"
	providerDomainContextCacheTTL       = 5 * time.Minute
	providerDomainContextMissCacheTTL   = 1 * time.Minute
	providerPublicConfigCacheTTL        = 5 * time.Minute
)

var (
	providerDomainContextCacheOnce sync.Once
	providerDomainContextCache     *cachex.HybridCache[providerContextCacheEntry]

	providerPublicConfigCacheOnce sync.Once
	providerPublicConfigCache     *cachex.HybridCache[providerPublicConfigCacheEntry]
)

func getProviderDomainContextCache() *cachex.HybridCache[providerContextCacheEntry] {
	providerDomainContextCacheOnce.Do(func() {
		providerDomainContextCache = cachex.NewHybridCache[providerContextCacheEntry](cachex.HybridCacheConfig[providerContextCacheEntry]{
			Namespace:  cachex.Namespace(providerDomainContextCacheNamespace),
			Redis:      common.RDB,
			RedisCodec: cachex.JSONCodec[providerContextCacheEntry]{},
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			Memory: func() *hot.HotCache[string, providerContextCacheEntry] {
				return hot.NewHotCache[string, providerContextCacheEntry](hot.LRU, 10000).
					WithTTL(providerDomainContextCacheTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return providerDomainContextCache
}

func getProviderPublicConfigCache() *cachex.HybridCache[providerPublicConfigCacheEntry] {
	providerPublicConfigCacheOnce.Do(func() {
		providerPublicConfigCache = cachex.NewHybridCache[providerPublicConfigCacheEntry](cachex.HybridCacheConfig[providerPublicConfigCacheEntry]{
			Namespace:  cachex.Namespace(providerPublicConfigCacheNamespace),
			Redis:      common.RDB,
			RedisCodec: cachex.JSONCodec[providerPublicConfigCacheEntry]{},
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			Memory: func() *hot.HotCache[string, providerPublicConfigCacheEntry] {
				return hot.NewHotCache[string, providerPublicConfigCacheEntry](hot.LRU, 10000).
					WithTTL(providerPublicConfigCacheTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return providerPublicConfigCache
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

func GetProviderContextByDomainCached(domain string) (*ProviderContext, error) {
	domain = NormalizeProviderDomain(domain)
	if domain == "" {
		return nil, nil
	}

	cache := getProviderDomainContextCache()
	entry, found, err := cache.Get(domain)
	if err != nil {
		common.SysLog("failed to get provider domain context cache: " + err.Error())
	} else if found {
		if !entry.Found {
			return nil, nil
		}
		ctx := &ProviderContext{
			ProviderId:  entry.ProviderId,
			OwnerUserId: entry.OwnerUserId,
			Name:        entry.Name,
			Domain:      entry.Domain,
		}
		if cfg, err := GetProviderPublicConfigCached(entry.ProviderId); err == nil && cfg != nil {
			ctx.Config = cfg
		}
		return ctx, nil
	}

	ctx, err := GetProviderContextByDomain(domain)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		if err := cache.SetWithTTL(domain, providerContextCacheEntry{Found: false}, providerDomainContextMissCacheTTL); err != nil {
			common.SysLog("failed to set provider domain context miss cache: " + err.Error())
		}
		return nil, nil
	}

	entry = providerContextCacheEntry{
		Found:       true,
		ProviderId:  ctx.ProviderId,
		OwnerUserId: ctx.OwnerUserId,
		Name:        ctx.Name,
		Domain:      ctx.Domain,
	}
	if err := cache.SetWithTTL(domain, entry, providerDomainContextCacheTTL); err != nil {
		common.SysLog("failed to set provider domain context cache: " + err.Error())
	}
	if ctx.Config != nil {
		if err := getProviderPublicConfigCache().SetWithTTL(strconv.Itoa(ctx.ProviderId), providerPublicConfigCacheEntry{Found: true, Config: *ctx.Config}, providerPublicConfigCacheTTL); err != nil {
			common.SysLog("failed to set provider public config cache: " + err.Error())
		}
	} else {
		if cfg, err := GetProviderPublicConfigCached(ctx.ProviderId); err == nil && cfg != nil {
			ctx.Config = cfg
		}
	}
	return ctx, nil
}

func GetProviderPublicConfigCached(providerId int) (*ProviderConfig, error) {
	if providerId <= 0 {
		return nil, nil
	}
	key := strconv.Itoa(providerId)
	cache := getProviderPublicConfigCache()
	entry, found, err := cache.Get(key)
	if err != nil {
		common.SysLog("failed to get provider public config cache: " + err.Error())
	} else if found {
		if !entry.Found {
			return nil, nil
		}
		cfg := entry.Config
		return &cfg, nil
	}

	var cfg ProviderConfig
	err = DB.Where("provider_id = ?", providerId).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if cacheErr := cache.SetWithTTL(key, providerPublicConfigCacheEntry{Found: false}, providerPublicConfigCacheTTL); cacheErr != nil {
			common.SysLog("failed to set provider public config miss cache: " + cacheErr.Error())
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := cache.SetWithTTL(key, providerPublicConfigCacheEntry{Found: true, Config: cfg}, providerPublicConfigCacheTTL); err != nil {
		common.SysLog("failed to set provider public config cache: " + err.Error())
	}
	return &cfg, nil
}

func InvalidateProviderDomainCache(providerId int) {
	if providerId <= 0 {
		return
	}
	domains, err := GetProviderVerifiedDomains(providerId)
	if err != nil {
		common.SysLog("failed to list provider domains for cache invalidation: " + err.Error())
		return
	}
	for _, domain := range domains {
		InvalidateProviderDomainCacheByDomain(domain)
	}
}

func InvalidateProviderDomainCacheByDomain(domain string) {
	cache := getProviderDomainContextCache()
	for _, candidate := range ProviderDomainLookupCandidates(domain) {
		if candidate == "" {
			continue
		}
		if _, err := cache.DeleteMany([]string{candidate}); err != nil {
			common.SysLog("failed to invalidate provider domain cache: " + err.Error())
		}
	}
}

func InvalidateProviderPublicConfigCache(providerId int) {
	if providerId <= 0 {
		return
	}
	if _, err := getProviderPublicConfigCache().DeleteMany([]string{strconv.Itoa(providerId)}); err != nil {
		common.SysLog("failed to invalidate provider public config cache: " + err.Error())
	}
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
