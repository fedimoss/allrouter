package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetSelfInviteeDashboardData returns dashboard card data for a user invited
// by the current user. The relation check prevents arbitrary account lookup.
// user_id=all 时返回名下全部邀请用户的汇总卡片数据。
func GetSelfInviteeDashboardData(c *gin.Context) {
	inviterID := c.GetInt("id")
	inviteeIDRaw := strings.TrimSpace(c.Query("user_id"))
	if inviterID <= 0 || inviteeIDRaw == "" {
		common.ApiErrorMsg(c, "invalid user_id")
		return
	}

	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if endTimestamp-startTimestamp > 2592000 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "time range cannot exceed 30 days",
		})
		return
	}

	displayInfo := getDisplayCurrencyForUser(c)

	// 全部邀请用户模式:账户数据与聚合明细均由邀请关系限定,无越权风险。
	if strings.ToLower(inviteeIDRaw) == "all" {
		agg, err := model.GetInviteeAggregation(inviterID)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		quotaData, err := model.GetInviteeQuotaDataByInviter(inviterID, startTimestamp, endTimestamp)
		if err != nil {
			common.ApiError(c, err)
			return
		}

		// 请求/统计次数:logs 可能独立于主库,先取邀请人 ID 列表再按 IN 过滤统计
		inviteeIds, err := model.GetInviteeIdsByInviter(inviterID)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		requestResult, _ := model.CountRequestLogsByUserIds(startTimestamp, endTimestamp, inviteeIds)
		totalRequestResult, _ := model.CountRequestLogsByUserIds(0, 0, inviteeIds)

		userData := gin.H{
			"id":               "all",
			"username":         fmt.Sprintf("%s (%d)", "全部邀请用户", agg.InviteeCount),
			"quota":            agg.QuotaSum,
			"used_quota":       agg.UsedQuotaSum,
			"total_token_used": agg.TokenUsedSum,
			"display_symbol":   displayInfo.Symbol,
			"display_currency": displayInfo.Currency,
			"display_rate":     displayInfo.Rate,
			"request_count":    requestResult.SuccessCount + requestResult.ErrorCount,
			"total_count":      totalRequestResult.SuccessCount + totalRequestResult.ErrorCount,
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"user":    userData,
			"data":    quotaData,
		})
		return
	}

	inviteeID, err := strconv.Atoi(inviteeIDRaw)
	if err != nil || inviteeID <= 0 {
		common.ApiErrorMsg(c, "invalid user_id")
		return
	}

	var relationCount int64
	if err := model.DB.Model(&model.InviteRecord{}).
		Where("inviter_id = ? AND invitee_id = ?", inviterID, inviteeID).
		Count(&relationCount).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if relationCount == 0 {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "no permission"})
		return
	}

	invitee, err := model.GetUserById(inviteeID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	quotaData, err := model.GetQuotaDataByUserId(inviteeID, startTimestamp, endTimestamp)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	requestResult, _ := model.CountRequestLogs(startTimestamp, endTimestamp, inviteeID)
	totalRequestResult, _ := model.CountRequestLogs(0, 0, inviteeID)

	userData := gin.H{
		"id":               invitee.Id,
		"username":         invitee.Username,
		"display_name":     invitee.DisplayName,
		"quota":            invitee.Quota,
		"used_quota":       invitee.UsedQuota,
		"total_token_used": invitee.TotalTokenUsed,
		"display_symbol":   displayInfo.Symbol,
		"display_currency": displayInfo.Currency,
		"display_rate":     displayInfo.Rate,
		"request_count":    requestResult.SuccessCount + requestResult.ErrorCount,
		"total_count":      totalRequestResult.SuccessCount + totalRequestResult.ErrorCount,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"user":    userData,
		"data":    quotaData,
	})
}

func GetAllQuotaDates(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	dates, err := model.GetAllQuotaDates(startTimestamp, endTimestamp, username)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 获取当前用户的展示币种信息（根据时区判断 USD/CNY，汇率取 currency_stripe_config 表）
	displayInfo := getDisplayCurrencyForUser(c)
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          "",
		"data":             dates,
		"display_currency": displayInfo.Currency, // 展示币种，如 "USD"、"CNY"
		"display_rate":     displayInfo.Rate,     // 相对于 1 美元的汇率，USD 为 1，CNY 为 currency_stripe_config.unit_price
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
	// 获取当前用户的展示币种信息（根据时区判断 USD/CNY，汇率取 currency_stripe_config 表）
	displayInfo := getDisplayCurrencyForUser(c)
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          "",
		"data":             dates,
		"display_currency": displayInfo.Currency, // 展示币种，如 "USD"、"CNY"
		"display_rate":     displayInfo.Rate,     // 相对于 1 美元的汇率，USD 为 1，CNY 为 currency_stripe_config.unit_price
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
