package controller

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// buildProviderModelPricingSyncConfigResponse 组装返回前端的自动同步配置：
// 开关状态、上次同步时间、上次同步摘要（优先反序列化为结构体，失败则原样回传字符串）
func buildProviderModelPricingSyncConfigResponse(cfg *model.ProviderConfig) gin.H {
	resp := gin.H{
		"enabled":      false,
		"last_sync_at": int64(0),
	}
	if cfg == nil || cfg.Id == 0 {
		return resp
	}
	resp["enabled"] = cfg.ModelPricingSyncEnabled
	resp["last_sync_at"] = cfg.ModelPricingSyncLastAt
	if strings.TrimSpace(cfg.ModelPricingSyncLastSummary) != "" {
		var summary providerModelPricingSyncSummary
		if err := common.UnmarshalJsonStr(cfg.ModelPricingSyncLastSummary, &summary); err == nil {
			resp["last_summary"] = summary
		} else {
			resp["last_summary"] = cfg.ModelPricingSyncLastSummary
		}
	}
	return resp
}

// loadProviderModelPricingSyncConfig 读取服务商配置；不存在时返回 (nil, nil) 而非报错
func loadProviderModelPricingSyncConfig(providerId int) (*model.ProviderConfig, error) {
	var cfg model.ProviderConfig
	err := model.DB.Where("provider_id = ?", providerId).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// saveProviderModelPricingSyncEnabled 保存自动同步开关。
// 返回值第二个 changedToEnabled 表示“由关变开”，调用方据此决定是否立即同步一次。
// 不存在配置记录时会自动创建一条
func saveProviderModelPricingSyncEnabled(providerId int, enabled bool) (*model.ProviderConfig, bool, error) {
	var cfg model.ProviderConfig
	err := model.DB.Where("provider_id = ?", providerId).First(&cfg).Error
	now := common.GetTimestamp()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cfg = model.ProviderConfig{
			ProviderId:              providerId,
			ImportPriceRatio:        1,
			ModelPricingSyncEnabled: enabled,
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		if err := model.DB.Create(&cfg).Error; err != nil {
			return nil, false, err
		}
		return &cfg, enabled, nil
	}
	if err != nil {
		return nil, false, err
	}
	changedToEnabled := enabled && !cfg.ModelPricingSyncEnabled
	if err := model.DB.Model(&cfg).Updates(map[string]interface{}{
		"model_pricing_sync_enabled": enabled,
		"updated_at":                 now,
	}).Error; err != nil {
		return nil, false, err
	}
	cfg.ModelPricingSyncEnabled = enabled
	cfg.UpdatedAt = now
	return &cfg, changedToEnabled, nil
}

// updateProviderModelPricingSyncSummary 在事务内回写同步摘要到 provider_configs：
// 计算各分类计数、排序模型名、补默认时间，并把摘要序列化为 JSON 存储
func updateProviderModelPricingSyncSummary(tx *gorm.DB, providerId int, summary *providerModelPricingSyncSummary) error {
	if summary == nil {
		return nil
	}
	summary.AddedCount = len(summary.AddedModels)
	summary.DisabledCount = len(summary.DisabledModels)
	summary.ReenabledCount = len(summary.ReenabledModels)
	summary.SkippedCount = len(summary.SkippedModels)
	sort.Strings(summary.AddedModels)
	sort.Strings(summary.DisabledModels)
	sort.Strings(summary.ReenabledModels)
	sort.Strings(summary.SkippedModels)
	if summary.LastSyncAt == 0 {
		summary.LastSyncAt = common.GetTimestamp()
	}
	var cfg model.ProviderConfig
	err := tx.Where("provider_id = ?", providerId).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		cfg = model.ProviderConfig{
			ProviderId:             providerId,
			ImportPriceRatio:       1,
			ModelPricingSyncLastAt: summary.LastSyncAt,
			CreatedAt:              summary.LastSyncAt,
			UpdatedAt:              summary.LastSyncAt,
		}
		cfg.ModelPricingSyncEnabled = summary.SyncEnabled
		data, err := common.Marshal(summary)
		if err != nil {
			return err
		}
		cfg.ModelPricingSyncLastSummary = string(data)
		return tx.Create(&cfg).Error
	}
	if err != nil {
		return err
	}
	data, err := common.Marshal(summary)
	if err != nil {
		return err
	}
	return tx.Model(&cfg).Updates(map[string]interface{}{
		"model_pricing_sync_last_at":      summary.LastSyncAt,
		"model_pricing_sync_last_summary": string(data),
		"updated_at":                      summary.LastSyncAt,
	}).Error
}

type providerModelPricingSyncRowUpdate struct {
	Id           int
	BaseModel    string
	Enabled      bool
	SyncDisabled bool
}

type providerModelPricingSyncPlan struct {
	Reenable []providerModelPricingSyncRowUpdate
	Disable  []providerModelPricingSyncRowUpdate
	Creates  []model.ProviderModelPricing
	Skipped  []string
}

// planProviderModelPricingSync computes the deterministic set of operations
// needed to align a provider's model pricing rows with the main-site visible
// model set, without touching the database. It is safe to unit test directly.
//
// Rules:
//   - main model present but row is sync-disabled (was auto-disabled by a
//     previous sync) => re-enable, clear sync_disabled, keep other config.
//   - main model absent but row currently enabled => soft-disable (set
//     enabled=false, sync_disabled=true). Rows already disabled (whether
//     manually or by sync) are left untouched, so a provider's manual
//     "disabled" state is never modified by sync.
//   - main model missing entirely => create a default ratio row.
//   - main model name collides with an existing public_model_name but no
//     base_model_name match => skip to avoid clobbering the display name.
func planProviderModelPricingSync(mainModels []string, rows []model.ProviderModelPricing) providerModelPricingSyncPlan {
	mainSet := make(map[string]struct{}, len(mainModels))
	for _, modelName := range mainModels {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		mainSet[modelName] = struct{}{}
	}

	existingBaseModels := make(map[string]struct{}, len(rows))
	existingPublicModels := make(map[string]struct{}, len(rows))

	var plan providerModelPricingSyncPlan
	for _, row := range rows {
		baseModel := strings.TrimSpace(row.BaseModelName)
		publicModel := strings.TrimSpace(row.PublicModelName)
		if baseModel != "" {
			existingBaseModels[baseModel] = struct{}{}
		}
		if publicModel != "" {
			existingPublicModels[publicModel] = struct{}{}
		}

		_, visible := mainSet[baseModel]
		switch {
		case visible && row.SyncDisabled:
			plan.Reenable = append(plan.Reenable, providerModelPricingSyncRowUpdate{
				Id:           row.Id,
				BaseModel:    baseModel,
				Enabled:      true,
				SyncDisabled: false,
			})
		case !visible && row.Enabled:
			plan.Disable = append(plan.Disable, providerModelPricingSyncRowUpdate{
				Id:           row.Id,
				BaseModel:    baseModel,
				Enabled:      false,
				SyncDisabled: true,
			})
		}
	}

	for _, modelName := range mainModels {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, exists := existingBaseModels[modelName]; exists {
			continue
		}
		if _, exists := existingPublicModels[modelName]; exists {
			plan.Skipped = append(plan.Skipped, modelName)
			continue
		}
		plan.Creates = append(plan.Creates, model.ProviderModelPricing{
			PublicModelName: modelName,
			BaseModelName:   modelName,
			Enabled:         true,
			SyncDisabled:    false,
			PricingType:     model.ProviderPricingTypeRatio,
			Ratio:           defaultProviderModelPricingRatio,
		})
		existingBaseModels[modelName] = struct{}{}
		existingPublicModels[modelName] = struct{}{}
	}

	return plan
}

// syncProviderModelPricing 对单个服务商执行一次模型定价同步：
// 以服务商主账号在主站的可见模型集合为基准，补齐缺失模型、软禁用主站已下架的模型、
// 恢复主站又恢复的模型（仅限同步软禁用行），全过程在单个事务内完成并回写同步摘要。
// 注意：不感知“是否开启自动同步”，调用方决定是否触发；开关状态仅记录到摘要里
func syncProviderModelPricing(providerId int) (*providerModelPricingSyncSummary, error) {
	if providerId <= 0 {
		return nil, fmt.Errorf("invalid provider id")
	}
	var provider model.Provider
	if err := model.DB.Where("id = ?", providerId).First(&provider).Error; err != nil {
		return nil, err
	}
	ownerGroup, err := model.GetUserGroup(provider.OwnerUserId, false)
	if err != nil {
		ownerGroup = ""
	}
	mainModels := getMarketplaceVisibleModelNamesForUserGroup(ownerGroup)

	var cfg model.ProviderConfig
	cfgErr := model.DB.Where("provider_id = ?", providerId).First(&cfg).Error
	syncEnabled := cfgErr == nil && cfg.ModelPricingSyncEnabled

	summary := &providerModelPricingSyncSummary{
		ProviderId:  providerId,
		LastSyncAt:  common.GetTimestamp(),
		SyncEnabled: syncEnabled,
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		var rows []model.ProviderModelPricing
		if err := tx.Where("provider_id = ?", providerId).Order("id asc").Find(&rows).Error; err != nil {
			return err
		}

		plan := planProviderModelPricingSync(mainModels, rows)

		for _, op := range plan.Reenable {
			if err := tx.Model(&model.ProviderModelPricing{}).
				Where("id = ? AND provider_id = ?", op.Id, providerId).
				Updates(map[string]interface{}{
					"enabled":       op.Enabled,
					"sync_disabled": op.SyncDisabled,
					"updated_at":    summary.LastSyncAt,
				}).Error; err != nil {
				return err
			}
			summary.ReenabledModels = append(summary.ReenabledModels, op.BaseModel)
		}

		for _, op := range plan.Disable {
			if err := tx.Model(&model.ProviderModelPricing{}).
				Where("id = ? AND provider_id = ?", op.Id, providerId).
				Updates(map[string]interface{}{
					"enabled":       op.Enabled,
					"sync_disabled": op.SyncDisabled,
					"updated_at":    summary.LastSyncAt,
				}).Error; err != nil {
				return err
			}
			summary.DisabledModels = append(summary.DisabledModels, op.BaseModel)
		}

		for _, row := range plan.Creates {
			row.ProviderId = providerId
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			summary.AddedModels = append(summary.AddedModels, row.BaseModelName)
		}

		summary.SkippedModels = append(summary.SkippedModels, plan.Skipped...)

		return updateProviderModelPricingSyncSummary(tx, providerId, summary)
	})
	if err != nil {
		return nil, err
	}
	model.InvalidateProviderPublicConfigCache(providerId)
	return summary, nil
}

// syncAllEnabledProviderModelPricing 同步所有“已开启自动同步 且 启用中”的服务商。
// 先用 TryLock 抢占全量同步互斥锁，抢不到说明已有同步在跑则直接返回；
// 单个服务商同步失败不影响其他服务商，仅记录系统日志
func syncAllEnabledProviderModelPricing(reason string) {
	if model.DB == nil {
		return
	}
	if !providerModelPricingSyncAllMu.TryLock() {
		common.SysLog("provider model pricing sync skipped: another sync is running")
		return
	}
	defer providerModelPricingSyncAllMu.Unlock()

	// 先强制刷新主站定价缓存，确保依据的是最新的可见模型集合
	model.RefreshPricing()
	var providerIds []int
	if err := model.DB.Table("provider_configs").
		Joins("JOIN providers ON providers.id = provider_configs.provider_id").
		Where("provider_configs.model_pricing_sync_enabled = ? AND providers.status = ?", true, model.ProviderStatusEnabled).
		Order("provider_configs.provider_id asc").
		Pluck("provider_configs.provider_id", &providerIds).Error; err != nil {
		common.SysError("failed to list providers for model pricing sync: " + err.Error())
		return
	}
	for _, providerId := range providerIds {
		if _, err := syncProviderModelPricing(providerId); err != nil {
			common.SysError(fmt.Sprintf("provider model pricing sync failed: provider_id=%d, reason=%s, error=%v", providerId, reason, err))
		}
	}
}

// TriggerProviderModelPricingSyncForMainModelChange 主站可见模型集合发生变化时的统一触发入口。
// 异步执行全量同步，立即返回不阻塞请求。
// 调用点：模型元数据增删改、上游模型同步、渠道增删改/启禁/标签编辑、上游模型检测与应用等入口
func TriggerProviderModelPricingSyncForMainModelChange() {
	go syncAllEnabledProviderModelPricing("main model changed")
}

// getProviderModelPricingSyncConfig 读取自动同步开关 + 上次同步时间/摘要
func getProviderModelPricingSyncConfig(c *gin.Context, providerId int) {
	cfg, err := loadProviderModelPricingSyncConfig(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildProviderModelPricingSyncConfigResponse(cfg))
}

// updateProviderModelPricingSyncConfig 保存自动同步开关；当开关由 false 变 true 时立即同步一次
func updateProviderModelPricingSyncConfig(c *gin.Context, providerId int) {
	var req providerModelPricingSyncConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	cfg, changedToEnabled, err := saveProviderModelPricingSyncEnabled(providerId, req.Enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	resp := buildProviderModelPricingSyncConfigResponse(cfg)
	if changedToEnabled {
		// 刚开启自动同步，立即执行一次同步并把本次结果一并返回给前端展示
		model.RefreshPricing()
		summary, err := syncProviderModelPricing(providerId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		resp["summary"] = summary
		resp["last_sync_at"] = summary.LastSyncAt
		resp["last_summary"] = summary
	}
	common.ApiSuccess(c, resp)
}

// syncProviderModelPricingNow 手动“立即同步”：无论开关是否开启都强制执行一次同步
func syncProviderModelPricingNow(c *gin.Context, providerId int) {
	model.RefreshPricing()
	summary, err := syncProviderModelPricing(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func AdminGetProviderModelPricingSyncConfig(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	getProviderModelPricingSyncConfig(c, id)
}

func AdminUpdateProviderModelPricingSyncConfig(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	updateProviderModelPricingSyncConfig(c, id)
}

func AdminSyncProviderModelPricing(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	syncProviderModelPricingNow(c, id)
}

func GetProviderModelPricingSyncConfig(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	getProviderModelPricingSyncConfig(c, provider.Id)
}

func UpdateProviderModelPricingSyncConfig(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	updateProviderModelPricingSyncConfig(c, provider.Id)
}

func SyncProviderModelPricing(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	syncProviderModelPricingNow(c, provider.Id)
}
