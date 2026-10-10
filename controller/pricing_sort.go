/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	pricingSortOptionKey         = "PricingSortConfig"
	providerPricingSortOptionKey = "pricing.sort"
)

type pricingSortCriterion struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type pricingSortConfig struct {
	Mode     string                 `json:"mode"`
	Criteria []pricingSortCriterion `json:"criteria"`
	Weights  map[string]int         `json:"weights"`
}

var pricingSortFields = map[string]struct{}{
	"model_name":               {},
	"vendor_name":              {},
	"model_ratio":              {},
	"completion_ratio":         {},
	"model_price":              {},
	"quota_type":               {},
	"billing_mode":             {},
	"tags":                     {},
	"supported_endpoint_types": {},
	"sort_weight":              {},
}

var pricingSortModes = map[string]struct{}{
	"hot":    {},
	"latest": {},
	"value":  {},
	"custom": {},
}

func defaultPricingSortConfig() pricingSortConfig {
	return pricingSortConfig{
		Mode:     "hot",
		Criteria: []pricingSortCriterion{},
		Weights:  map[string]int{},
	}
}

func normalizePricingSortConfig(config pricingSortConfig) (pricingSortConfig, error) {
	mode := strings.ToLower(strings.TrimSpace(config.Mode))
	if mode == "" {
		mode = "hot"
	}
	if _, ok := pricingSortModes[mode]; !ok {
		return pricingSortConfig{}, fmt.Errorf("不支持的模型排序模式")
	}

	normalized := pricingSortConfig{
		Mode:     mode,
		Criteria: make([]pricingSortCriterion, 0, len(config.Criteria)),
		Weights:  make(map[string]int, len(config.Weights)),
	}
	seen := make(map[string]struct{}, len(config.Criteria))
	for _, criterion := range config.Criteria {
		field := strings.TrimSpace(criterion.Field)
		direction := strings.ToLower(strings.TrimSpace(criterion.Direction))
		if _, ok := pricingSortFields[field]; !ok {
			return pricingSortConfig{}, fmt.Errorf("不支持的模型排序字段")
		}
		if direction != "asc" && direction != "desc" {
			return pricingSortConfig{}, fmt.Errorf("不支持的模型排序方向")
		}
		if _, exists := seen[field]; exists {
			return pricingSortConfig{}, fmt.Errorf("模型排序字段不能重复")
		}
		seen[field] = struct{}{}
		normalized.Criteria = append(normalized.Criteria, pricingSortCriterion{
			Field:     field,
			Direction: direction,
		})
	}
	if len(normalized.Criteria) > len(pricingSortFields) {
		return pricingSortConfig{}, fmt.Errorf("模型排序条件过多")
	}
	if mode == "custom" && len(normalized.Criteria) == 0 {
		return pricingSortConfig{}, fmt.Errorf("自定义排序至少需要一个排序字段")
	}
	if mode != "custom" {
		normalized.Criteria = []pricingSortCriterion{}
	}
	for modelName, weight := range config.Weights {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			return pricingSortConfig{}, fmt.Errorf("排序大小对应的模型名称不能为空")
		}
		if weight < 0 {
			return pricingSortConfig{}, fmt.Errorf("排序大小不能小于 0")
		}
		if len(modelName) > 255 {
			return pricingSortConfig{}, fmt.Errorf("排序大小对应的模型名称过长")
		}
		if _, exists := normalized.Weights[modelName]; exists {
			return pricingSortConfig{}, fmt.Errorf("排序大小对应的模型名称不能重复")
		}
		normalized.Weights[modelName] = weight
	}
	if len(normalized.Weights) > 10000 {
		return pricingSortConfig{}, fmt.Errorf("排序大小设置过多")
	}
	return normalized, nil
}

func readPricingSortConfig(providerId int) pricingSortConfig {
	globalRaw := ""
	common.OptionMapRWMutex.RLock()
	globalRaw = common.OptionMap[pricingSortOptionKey]
	common.OptionMapRWMutex.RUnlock()

	if providerId > 0 {
		value, err := model.GetProviderOptionValue(providerId, providerPricingSortOptionKey)
		if err == nil && strings.TrimSpace(value) != "" {
			var providerConfig pricingSortConfig
			if common.UnmarshalJsonStr(value, &providerConfig) == nil {
				if normalized, normalizeErr := normalizePricingSortConfig(providerConfig); normalizeErr == nil {
					return normalized
				}
			}
		}
	}
	raw := globalRaw
	if strings.TrimSpace(raw) == "" {
		return defaultPricingSortConfig()
	}

	var config pricingSortConfig
	if err := common.UnmarshalJsonStr(raw, &config); err != nil {
		return defaultPricingSortConfig()
	}
	normalized, err := normalizePricingSortConfig(config)
	if err != nil {
		return defaultPricingSortConfig()
	}
	return normalized
}

func canUpdatePricingSort(c *gin.Context, providerId int) bool {
	if c.GetInt("role") >= common.RoleAdminUser {
		return true
	}
	if providerId <= 0 {
		return model.IsProviderOwner(c.GetInt("id"))
	}
	provider, err := model.GetProviderById(providerId)
	return err == nil && provider.OwnerUserId == c.GetInt("id")
}

// UpdatePricingSort saves the default ordering for the current site. Main-site
// ordering belongs to administrators; a provider owner can update the ordering
// of the provider domain they own, or the global ordering when operating on the
// main site. The resulting config is public through the pricing endpoint so
// ordinary users receive the same ordering.
func UpdatePricingSort(c *gin.Context) {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	if !canUpdatePricingSort(c, providerId) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权修改模型广场排序",
		})
		return
	}

	var request pricingSortConfig
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的模型排序配置",
		})
		return
	}
	normalized, err := normalizePricingSortConfig(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	encoded, err := common.Marshal(normalized)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if providerId > 0 {
		err = model.UpdateProviderOption(providerId, providerPricingSortOptionKey, string(encoded))
	} else {
		err = model.UpdateOption(pricingSortOptionKey, string(encoded))
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    normalized,
	})
}
