package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func pricingRow(id int, baseModel, publicModel string, enabled, syncDisabled bool) model.ProviderModelPricing {
	return model.ProviderModelPricing{
		Id:              id,
		PublicModelName: publicModel,
		BaseModelName:   baseModel,
		Enabled:         enabled,
		SyncDisabled:    syncDisabled,
		PricingType:     model.ProviderPricingTypeRatio,
		Ratio:           1.5,
	}
}

func createdBaseNames(plan providerModelPricingSyncPlan) []string {
	names := make([]string, 0, len(plan.Creates))
	for _, row := range plan.Creates {
		names = append(names, row.BaseModelName)
	}
	return names
}

func updatedBaseNames(plan providerModelPricingSyncPlan, reenable bool) []string {
	ops := plan.Disable
	if reenable {
		ops = plan.Reenable
	}
	names := make([]string, 0, len(ops))
	for _, op := range ops {
		names = append(names, op.BaseModel)
	}
	return names
}

func TestMarketplaceModelVisible(t *testing.T) {
	visibleModels := []string{"gpt-4o", " claude-3-5-sonnet ", ""}

	require.True(t, isMarketplaceModelVisible("gpt-4o", visibleModels))
	require.True(t, isMarketplaceModelVisible("claude-3-5-sonnet", visibleModels))
	require.False(t, isMarketplaceModelVisible("missing", visibleModels))
	require.False(t, isMarketplaceModelVisible("", visibleModels))
}

// 主站有 10 个、服务商有 9 个 => 补 1 个缺失模型，默认 ratio=1.5。
func TestPlanProviderModelPricingSyncAddsMissingModels(t *testing.T) {
	mainModels := []string{"m1", "m2", "m3"}
	rows := []model.ProviderModelPricing{
		pricingRow(1, "m1", "m1", true, false),
		pricingRow(2, "m2", "m2", true, false),
	}

	plan := planProviderModelPricingSync(mainModels, rows)

	require.Equal(t, []string{"m3"}, createdBaseNames(plan))
	require.Len(t, plan.Creates, 1)
	created := plan.Creates[0]
	require.Equal(t, "m3", created.PublicModelName)
	require.Equal(t, "m3", created.BaseModelName)
	require.True(t, created.Enabled)
	require.False(t, created.SyncDisabled)
	require.Equal(t, model.ProviderPricingTypeRatio, created.PricingType)
	require.InDelta(t, defaultProviderModelPricingRatio, created.Ratio, 1e-9)
	require.Empty(t, plan.Reenable)
	require.Empty(t, plan.Disable)
	require.Empty(t, plan.Skipped)
}

// 已有模型配置不被覆盖：服务商改了展示名/价格/启用状态，主站模型仍在 =>
// 既不进 reenable 也不进 disable，也不重新创建。
func TestPlanProviderModelPricingSyncDoesNotOverwriteExisting(t *testing.T) {
	mainModels := []string{"m1", "m2"}
	rows := []model.ProviderModelPricing{
		{
			Id:              1,
			PublicModelName: "renamed-by-provider",
			BaseModelName:   "m1",
			Enabled:         false,
			SyncDisabled:    false,
			PricingType:     model.ProviderPricingTypeDelta,
			Ratio:           9.9,
		},
		pricingRow(2, "m2", "m2", true, false),
	}

	plan := planProviderModelPricingSync(mainModels, rows)

	// m1 visible but not sync-disabled (provider's own disabled state) => no op.
	// m2 visible and enabled => no op.
	require.Empty(t, plan.Reenable)
	require.Empty(t, plan.Disable)
	require.Empty(t, plan.Creates)
	require.Empty(t, plan.Skipped)
}

// 主站下架模型，服务商该模型当前启用 => 软禁用 (enabled=false, sync_disabled=true)。
func TestPlanProviderModelPricingSyncSoftDisablesRemovedModels(t *testing.T) {
	mainModels := []string{"m1"}
	rows := []model.ProviderModelPricing{
		pricingRow(1, "m1", "m1", true, false),
		pricingRow(2, "gone", "gone", true, false),
	}

	plan := planProviderModelPricingSync(mainModels, rows)

	require.Empty(t, plan.Reenable)
	require.Empty(t, plan.Creates)
	require.Equal(t, []string{"gone"}, updatedBaseNames(plan, false))
	require.Len(t, plan.Disable, 1)
	op := plan.Disable[0]
	require.Equal(t, 2, op.Id)
	require.False(t, op.Enabled)
	require.True(t, op.SyncDisabled)
}

// 主站恢复模型，之前被同步禁用 (sync_disabled=true) => 只恢复启用，价格不变。
func TestPlanProviderModelPricingSyncReenablesSyncDisabledRows(t *testing.T) {
	mainModels := []string{"m1", "back"}
	rows := []model.ProviderModelPricing{
		pricingRow(1, "m1", "m1", true, false),
		{
			Id:              2,
			PublicModelName: "back",
			BaseModelName:   "back",
			Enabled:         false,
			SyncDisabled:    true, // previously auto-disabled by sync
			PricingType:     model.ProviderPricingTypeDelta,
			Ratio:           3.3,
		},
	}

	plan := planProviderModelPricingSync(mainModels, rows)

	require.Empty(t, plan.Disable)
	require.Empty(t, plan.Creates)
	require.Equal(t, []string{"back"}, updatedBaseNames(plan, true))
	require.Len(t, plan.Reenable, 1)
	op := plan.Reenable[0]
	require.Equal(t, 2, op.Id)
	require.True(t, op.Enabled)
	require.False(t, op.SyncDisabled)
}

// 服务商手动禁用模型 (sync_disabled=false, enabled=false)，主站模型仍在 =>
// 同步不改它的状态（不会重新启用）。
func TestPlanProviderModelPricingSyncIgnoresManuallyDisabledRows(t *testing.T) {
	mainModels := []string{"m1", "manual-off"}
	rows := []model.ProviderModelPricing{
		pricingRow(1, "m1", "m1", true, false),
		{
			Id:              2,
			PublicModelName: "manual-off",
			BaseModelName:   "manual-off",
			Enabled:         false,
			SyncDisabled:    false, // provider disabled it manually
			PricingType:     model.ProviderPricingTypeDelta,
			Ratio:           2.0,
		},
	}

	plan := planProviderModelPricingSync(mainModels, rows)

	require.Empty(t, plan.Reenable)
	require.Empty(t, plan.Disable)
	require.Empty(t, plan.Creates)
	require.Empty(t, plan.Skipped)
}

// 主站模型名与某条已有 public_model_name 冲突但没有 base_model_name 匹配 =>
// 跳过新增，避免覆盖服务商的展示名。冲突行自身的 base 仍可见，所以不会被禁用。
func TestPlanProviderModelPricingSyncSkipsPublicNameCollision(t *testing.T) {
	mainModels := []string{"m1", "m2", "shared"}
	rows := []model.ProviderModelPricing{
		pricingRow(1, "m1", "m1", true, false),
		// base "m2" is still visible (so this row is left alone), but its public
		// display name "shared" occupies the main-site model "shared".
		pricingRow(2, "m2", "shared", true, false),
	}

	plan := planProviderModelPricingSync(mainModels, rows)

	require.Empty(t, plan.Reenable)
	require.Empty(t, plan.Disable)
	require.Empty(t, plan.Creates)
	require.Equal(t, []string{"shared"}, plan.Skipped)
}

// 组合场景：同时存在新增、软禁用、恢复、跳过、不动。
func TestPlanProviderModelPricingSyncMixed(t *testing.T) {
	mainModels := []string{"keep", "back", "fresh", "collision"}
	rows := []model.ProviderModelPricing{
		pricingRow(1, "keep", "keep", true, false),      // visible, enabled => no op
		pricingRow(2, "back", "back", false, true),      // visible, sync-disabled => reenable
		pricingRow(3, "gone", "gone", true, false),      // not visible, enabled => disable
		pricingRow(4, "manual", "manual", false, false), // not visible, manually disabled => no op
		// base "keep" still visible => row untouched; public "collision" collides
		// with main-site "collision" => that main model is skipped on insert.
		pricingRow(5, "keep", "collision", true, false),
	}

	plan := planProviderModelPricingSync(mainModels, rows)

	require.Equal(t, []string{"back"}, updatedBaseNames(plan, true))
	require.Equal(t, []string{"gone"}, updatedBaseNames(plan, false))
	require.Equal(t, []string{"fresh"}, createdBaseNames(plan))
	require.Equal(t, []string{"collision"}, plan.Skipped)
}
