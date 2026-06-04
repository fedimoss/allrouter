package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

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
		item.ModelName = rule.PublicModelName
		item.ModelRatio *= importPriceRatio
		item.ModelPrice *= importPriceRatio
		if rule.PricingType == model.ProviderPricingTypeDelta {
			item.ModelRatio += rule.DeltaModelRatio
			item.ModelPrice += rule.DeltaModelPrice
		} else {
			item.ModelRatio *= rule.Ratio
			item.ModelPrice *= rule.Ratio
		}
		if item.ModelRatio < 0 {
			item.ModelRatio = 0
		}
		if item.ModelPrice < 0 {
			item.ModelPrice = 0
		}
		result = append(result, item)
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
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
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
