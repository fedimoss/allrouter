package controller

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllQuotaDates(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	dates, err := model.GetAllQuotaDates(startTimestamp, endTimestamp, username)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
	return
}

func GetQuotaDatesByUser(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	dates, err := model.GetQuotaDataGroupByUser(startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
}

func GetUserQuotaDates(c *gin.Context) {
	userId := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	// 判断时间跨度是否超过 1 个月
	if endTimestamp-startTimestamp > 2592000 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "时间跨度不能超过 1 个月",
		})
		return
	}
	dates, err := model.GetQuotaDataByUserId(userId, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    dates,
	})
	return
}

// -------------------------- 模型热度排行 --------------------------

type ModelCountRank struct {
	ModelName string `json:"model_name"`
	Count     int    `json:"count"`
}

// 获取所有模型热度排行
func GetAllModelPopularRank(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	dates, err := model.GetAllQuotaDates(startTimestamp, endTimestamp, username)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 统计每个模型的请求次数
	modelCounts := make(map[string]int)
	for _, record := range dates {
		modelCounts[record.ModelName] += record.Count
	}

	// 按照 count 排序
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    SortByCount(modelCounts),
	})
}

// 获取用户模型热度排行
func GetUserModelPopularRank(c *gin.Context) {
	userId := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	// 判断时间跨度是否超过 1 个月
	if endTimestamp-startTimestamp > 2592000 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "时间跨度不能超过 1 个月",
		})
		return
	}
	dates, err := model.GetQuotaDataByUserId(userId, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 统计每个模型的请求次数
	modelCounts := make(map[string]int)
	for _, record := range dates {
		modelCounts[record.ModelName] += record.Count
	}

	// 按照 count 排序
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    SortByCount(modelCounts),
	})
}

// 按照 count 从高到低排序，count 相同时按模型名升序，保证结果稳定
func SortByCount(m map[string]int) []ModelCountRank {
	items := make([]ModelCountRank, 0, len(m))
	for modelName, count := range m {
		items = append(items, ModelCountRank{
			ModelName: modelName,
			Count:     count,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].ModelName < items[j].ModelName
		}
		return items[i].Count > items[j].Count
	})

	return items
}

// -------------------------- 模型额度占比 --------------------------

type ModellQuotaRank struct {
	ModelName string  `json:"model_name"`
	Quota     float64 `json:"quota"`
}

// 获取所有模型额度占比
func GetAllModelQuotaRadio(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	dates, err := model.GetAllQuotaDates(startTimestamp, endTimestamp, username)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 统计每个模型的额度总和
	modelQuotas := make(map[string]float64)
	for _, record := range dates {
		modelQuotas[record.ModelName] += float64(record.Quota)
	}

	// 计算所有模型的额度总和
	totalQuota := float64(0)
	for _, quota := range modelQuotas {
		totalQuota += quota
	}

	// 计算每个模型的额度占比
	for modelName, quota := range modelQuotas {
		modelQuotas[modelName] = float64(quota) / float64(totalQuota)
	}

	// 按照 quota 排序
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    SortByQuota(modelQuotas),
	})

}

// 获取用户模型额度占比
func GetUserModelQuotaRadio(c *gin.Context) {
	userId := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	// 判断时间跨度是否超过 1 个月
	if endTimestamp-startTimestamp > 2592000 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "时间跨度不能超过 1 个月",
		})
		return
	}
	dates, err := model.GetQuotaDataByUserId(userId, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 统计每个模型的额度总和
	modelQuotas := make(map[string]float64)
	for _, record := range dates {
		modelQuotas[record.ModelName] += float64(record.Quota)
	}

	// 计算所有模型的额度总和
	totalQuota := float64(0)
	for _, quota := range modelQuotas {
		totalQuota += quota
	}

	// 计算每个模型的额度占比
	for modelName, quota := range modelQuotas {
		modelQuotas[modelName] = float64(quota) / float64(totalQuota)
	}

	// 按照 quota 排序
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    SortByQuota(modelQuotas),
	})
}

// 按照 quota 从高到低排序，quota 相同时按模型名升序，保证结果稳定
func SortByQuota(m map[string]float64) []ModellQuotaRank {
	items := make([]ModellQuotaRank, 0, len(m))
	for modelName, quota := range m {
		items = append(items, ModellQuotaRank{
			ModelName: modelName,
			Quota:     quota,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Quota == items[j].Quota {
			return items[i].ModelName < items[j].ModelName
		}
		return items[i].Quota > items[j].Quota
	})

	return items
}
