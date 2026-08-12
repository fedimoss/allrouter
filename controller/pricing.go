package controller

import (
	"math"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func providerPricingRatio(rule model.ProviderModelPricing) float64 {
	if rule.Ratio == 0 {
		return 1
	}
	return rule.Ratio
}

// scaleBillingExprForDisplay wraps a billing expression in a numeric
// multiplier while preserving an optional version prefix. The provider-site
// pricing endpoint uses this display-only expression so the model marketplace
// shows the same dynamic prices that provider users are actually charged.
func scaleBillingExprForDisplay(expr string, multiplier float64) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) {
		multiplier = 1
	}
	if multiplier < 0 {
		multiplier = 0
	}
	if multiplier == 1 {
		return expr
	}

	prefix := ""
	body := expr
	if strings.HasPrefix(expr, "v") {
		if colon := strings.IndexByte(expr, ':'); colon > 1 {
			if _, err := strconv.Atoi(expr[1:colon]); err == nil {
				prefix = expr[:colon+1]
				body = strings.TrimSpace(expr[colon+1:])
			}
		}
	}

	// Provider ratios are persisted with finite decimal precision. Round away
	// binary floating-point artifacts before embedding the factor in a public
	// expression (for example 0.4*1.5 should be rendered as 0.6).
	multiplier = math.Round(multiplier*1e14) / 1e14
	factor := strconv.FormatFloat(multiplier, 'f', -1, 64)
	return prefix + "(" + body + ") * " + factor
}

func applyProviderPricingRule(item model.Pricing, rule model.ProviderModelPricing, importPriceRatio float64) model.Pricing {
	if importPriceRatio <= 0 {
		importPriceRatio = 1
	}
	item.ModelName = rule.PublicModelName
	item.ModelRatio *= importPriceRatio
	item.ModelPrice *= importPriceRatio
	if len(item.ResolutionPrices) > 0 {
		prices := make(map[string]float64, len(item.ResolutionPrices))
		for resolution, price := range item.ResolutionPrices {
			prices[resolution] = price * importPriceRatio
		}
		item.ResolutionPrices = prices
	}

	if rule.PricingType == model.ProviderPricingTypeDelta {
		item.ModelRatio += rule.DeltaModelRatio
		item.ModelPrice += rule.DeltaModelPrice
		for resolution, price := range item.ResolutionPrices {
			item.ResolutionPrices[resolution] = price + rule.DeltaModelPrice
		}
		if item.BillingMode == "tiered_expr" && strings.TrimSpace(item.BillingExpr) != "" {
			item.BillingExpr = scaleBillingExprForDisplay(item.BillingExpr, importPriceRatio)
			item.ProviderPricingType = model.ProviderPricingTypeDelta
			item.ProviderDeltaModelRatio = rule.DeltaModelRatio
		}
	} else {
		ratio := providerPricingRatio(rule)
		item.ModelRatio *= ratio
		item.ModelPrice *= ratio
		for resolution, price := range item.ResolutionPrices {
			item.ResolutionPrices[resolution] = price * ratio
		}
		if item.BillingMode == "tiered_expr" && strings.TrimSpace(item.BillingExpr) != "" {
			item.BillingExpr = scaleBillingExprForDisplay(item.BillingExpr, importPriceRatio*ratio)
			item.ProviderPricingType = model.ProviderPricingTypeRatio
		}
	}
	if item.ModelRatio < 0 {
		item.ModelRatio = 0
	}
	if item.ModelPrice < 0 {
		item.ModelPrice = 0
	}
	return item
}

func applyProviderPricingView(providerId int, pricing []model.Pricing) []model.Pricing {
	if providerId == 0 || len(pricing) == 0 {
		return pricing
	}
	base := make(map[string]model.Pricing, len(pricing))
	for _, item := range pricing {
		base[item.ModelName] = item
	}
	var rules []model.ProviderModelPricing
	if err := model.DB.Where("provider_id = ? AND enabled = ?", providerId, true).Find(&rules).Error; err != nil {
		common.SysLog("failed to get provider pricing: " + err.Error())
		return []model.Pricing{}
	}
	importPriceRatio := 1.0
	var cfg model.ProviderConfig
	if err := model.DB.Select("import_price_ratio").Where("provider_id = ?", providerId).First(&cfg).Error; err == nil && cfg.ImportPriceRatio > 0 {
		importPriceRatio = cfg.ImportPriceRatio
	}
	result := make([]model.Pricing, 0, len(rules))
	for _, rule := range rules {
		item, ok := base[rule.BaseModelName]
		if !ok {
			continue
		}
		result = append(result, applyProviderPricingRule(item, rule, importPriceRatio))
	}
	return result
}

func filterPricingByUsableGroups(pricing []model.Pricing, usableGroup map[string]string) []model.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []model.Pricing{}
	}

	filtered := make([]model.Pricing, 0, len(pricing))
	for _, item := range pricing {
		if common.StringsContains(item.EnableGroup, "all") {
			filtered = append(filtered, item)
			continue
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func getPricingVisibilityContext(c *gin.Context) (map[string]string, map[string]float64, []string) {
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if userId, exists := c.Get("id"); exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	usableGroup = service.GetUserUsableGroups(group)
	for group := range ratio_setting.GetGroupRatioCopy() {
		if _, ok := usableGroup[group]; !ok {
			delete(groupRatio, group)
		}
	}
	return usableGroup, groupRatio, service.GetUserAutoGroup(group)
}

// getMarketplaceVisiblePricingForUserGroup 按指定用户分组计算主站市场可见的定价列表。
// 不依赖 *gin.Context，供后台自动同步（goroutine）使用
func getMarketplaceVisiblePricingForUserGroup(userGroup string) []model.Pricing {
	pricing := model.GetPricing()
	usableGroup := service.GetUserUsableGroups(userGroup)
	return filterPricingByUsableGroups(pricing, usableGroup)
}

func getMarketplaceVisiblePricing(c *gin.Context) []model.Pricing {
	pricing := model.GetPricing()
	usableGroup, _, _ := getPricingVisibilityContext(c)
	return filterPricingByUsableGroups(pricing, usableGroup)
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	pricing = applyProviderPricingView(providerId, pricing)
	usableGroup, groupRatio, autoGroups := getPricingVisibilityContext(c)
	pricing = filterPricingByUsableGroups(pricing, usableGroup)

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        autoGroups,
		"pricing_version":    "b2c5fbb11278d477f8141196fd56208a",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
