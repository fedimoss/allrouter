package controller

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetUserDashboard 用户看板
// query 参数 "period": day / week / month / year（默认 month）
func GetUserOperationDashboard(c *gin.Context) {
	period := c.DefaultQuery("period", "month")

	// 1. 总用户数
	totalUsers, err := model.CountTotalUsers()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 2. 今日新增用户数
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayNewUsers, err := model.CountNewUsersByTimeRange(todayStart.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 3. 根据 period 计算时间范围
	var currentStart, prevStart time.Time
	switch period {
	case "day":
		currentStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		prevStart = currentStart.AddDate(0, 0, -1)
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		currentStart = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		prevStart = currentStart.AddDate(0, 0, -7)
	case "year":
		currentStart = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		prevStart = time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
	default: // month
		currentStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		prevStart = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	}

	// 4. 新增注册用户：当前周期 & 上一周期
	currentNewUsers, err := model.CountNewUsersByTimeRange(currentStart.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	prevNewUsers, err := model.CountNewUsersByTimeRange(prevStart.Unix(), currentStart.Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 5. 活跃用户：当前周期 & 上一周期（token消耗>0）
	currentActiveUsers, err := model.CountActiveUsersByTimeRange(currentStart.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	prevActiveUsers, err := model.CountActiveUsersByTimeRange(prevStart.Unix(), currentStart.Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"total_users":     totalUsers,                                                       // 总用户数
			"today_new_users": todayNewUsers,                                                    // 今日新增用户
			"new_users":       currentNewUsers,                                                  // 当前周期新增用户
			"new_users_trend": calcPercentChange(int(currentNewUsers), int(prevNewUsers)),       // 新增用户环比变化百分比
			"active_users":    currentActiveUsers,                                               // 当前周期活跃用户
			"active_trend":    calcPercentChange(int(currentActiveUsers), int(prevActiveUsers)), // 活跃用户环比变化百分比
			"churned_users":   0,                                                                // 流失用户（计算方式待定）
			"churned_trend":   "+0%",                                                            // 流失用户环比变化百分比（计算方式待定）
		},
	})
}

// GetDistributorOperationDashboard 代理商看板
func GetDistributorOperationDashboard(c *gin.Context) {
	// TODO: 实现代理商看板数据查询
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{},
	})
}

// GetMerchantOperationDashboard 商家看板
func GetMerchantOperationDashboard(c *gin.Context) {
	// TODO: 实现商家看板数据查询
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{},
	})
}

// GetPlatformOperationDashboard 平台看板
func GetPlatformOperationDashboard(c *gin.Context) {
	// TODO: 实现平台看板数据查询
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{},
	})
}

// userRecordItem 用户列表记录项
type userRecordItem struct {
	UserId         int     `json:"user_id"`
	RequestCount   int64   `json:"request_count"`
	UsedQuota      float64 `json:"used_quota"` // 消耗情况
	Invited        bool    `json:"invited"`
	CreatedAt      int64   `json:"created_at"`
	LastActiveTime int64   `json:"last_active_time"`
	Retention      int     `json:"retention"`     // 留存: 1=7天内活跃, 0=7~30天, -1=超过30天
	Quota          float64 `json:"quota"`         // 余额
	TopupQuota     float64 `json:"topup_quota"`   // 充值金额
	WelfareQuota   float64 `json:"welfare_quota"` // 福利金额(兑换码+邀请奖励)
}

// GetUserOperationRecords 用户列表
func GetUserOperationRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	// 解析排序参数，所有字段均在 DB 层排序
	sortFields := make(map[string]string)
	for field, order := range map[string]string{
		"quota":         strings.ToLower(c.Query("quota")),         // 余额
		"request_count": strings.ToLower(c.Query("request_count")), // 使用次数
		"topup_quota":   strings.ToLower(c.Query("topup_quota")),   // 充值金额
		"welfare_quota": strings.ToLower(c.Query("welfare_quota")), // 福利金额
	} {
		if order == "asc" || order == "desc" {
			sortFields[field] = order
		}
	}

	// 注册时间范围筛选
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)

	// 消耗金额范围筛选
	usedQuotaMin, _ := strconv.Atoi(c.Query("used_quota_min"))
	usedQuotaMax, _ := strconv.Atoi(c.Query("used_quota_max"))

	// 余额范围筛选
	quotaMin, _ := strconv.Atoi(c.Query("quota_min"))
	quotaMax, _ := strconv.Atoi(c.Query("quota_max"))

	// 使用次数范围筛选
	requestCountMin, _ := strconv.Atoi(c.Query("request_count_min"))
	requestCountMax, _ := strconv.Atoi(c.Query("request_count_max"))

	// 条件分页查询用户列表(所有排序在 SQL 层完成)
	users, total, err := model.GetUserRecordsByCondition(pageInfo, sortFields, startTimestamp, endTimestamp, usedQuotaMin, usedQuotaMax, quotaMin, quotaMax, requestCountMin, requestCountMax, strings.TrimSpace(c.Query("keyword")))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 收集用户 ID
	userIds := make([]int, 0, len(users))
	for _, u := range users {
		userIds = append(userIds, u.Id)
	}

	// 批量查询最后活跃时间
	lastActiveMap, err := model.GetUsersLastActiveTime(userIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 批量查询充值金额
	topupMap, err := model.GetUsersTopupQuota(userIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 批量查询福利金额(兑换码 + 邀请奖励)
	welfareMap, err := model.GetUsersWelfareQuota(userIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 构建响应列表
	items := make([]userRecordItem, 0, len(users))
	for _, u := range users {
		// 查询用户全部请求次数
		countResult, _ := model.CountRequestLogs(0, 0, u.Id)

		// 计算留存状态
		retention := 0
		if lastActive, ok := lastActiveMap[u.Id]; ok && lastActive > 0 {
			daysSinceActive := (time.Now().Unix() - lastActive) / 86400
			switch {
			case daysSinceActive > 30:
				retention = -1 // 超过30天未活跃,流失
			case daysSinceActive <= 7:
				retention = 1 // 7天内活跃
			default:
				retention = 0 // 超过7天未活跃,预警
			}
		}

		items = append(items, userRecordItem{
			UserId:         u.Id,                                              // 用户ID
			RequestCount:   countResult.SuccessCount + countResult.ErrorCount, // 使用次数
			UsedQuota:      float64(u.UsedQuota) / common.QuotaPerUnit,        // 消耗情况
			Invited:        u.InviterId > 0,                                   // 注册来源(是否是邀请注册)
			CreatedAt:      u.CreatedAt,                                       // 注册时间
			LastActiveTime: lastActiveMap[u.Id],                               // 最后活跃时间
			Retention:      retention,                                         // 留存状态
			Quota:          float64(u.Quota) / common.QuotaPerUnit,            // 余额
			TopupQuota:     float64(topupMap[u.Id]) / common.QuotaPerUnit,     // 充值金额
			WelfareQuota:   float64(welfareMap[u.Id]) / common.QuotaPerUnit,   // 福利金额
		})
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// GetDistributorOperationRecords 代理商列表
func GetDistributorOperationRecords(c *gin.Context) {
	// TODO: 实现代理商列表数据查询
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{},
	})
}

// GetMerchantOperationRecords 商家列表
func GetMerchantOperationRecords(c *gin.Context) {
	// TODO: 实现商家列表数据查询
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{},
	})
}

// GetPlatformOperationRecords 平台列表
func GetPlatformOperationRecords(c *gin.Context) {
	// TODO: 实现平台列表数据查询
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{},
	})
}
