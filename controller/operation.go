package controller

import (
	"math"
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

// GetDashboardByPeriod 看板数据
// query 参数
// "period": day / week / month / year（默认 month）
// "provider_id": 服务商ID（0 表示主站，>0 表示对应服务商）
func GetDashboardByPeriod(c *gin.Context) {
	period := c.DefaultQuery("period", "month")
	providerId, _ := strconv.Atoi(c.Query("provider_id")) // 0 = 主站；>0 = 对应服务商

	// ============ 累计注册用户：按 provider_id 维度的用户总数（不依赖 period） ============
	totalUsers, err := model.CountTotalUsersByProvider(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 根据 period 计算当前周期 & 上一周期起始时间
	now := time.Now()
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

	// ============ 新增注册用户：当前周期内、按 provider_id 维度的注册用户数 ============
	newUsers, err := model.CountNewUsersByProvider(providerId, currentStart.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// ============ 入金金额：当前周期内、按 provider_id 维度的用户实付金额 ============
	// 计入在线支付(payment) + 订阅(subscription)；排除内部结算流水(分润/订阅收入)和未支付成功的订单
	moneySum, err := model.SumTopUpMoneyByProvider(providerId, currentStart.Unix(), 0,
		[]string{model.TopUpBizTypePayment, model.TopUpBizTypeSubscription})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	displayInfo := getDisplayCurrencyForUser(c)
	// 加密货币 money 是 USDT，按系统"美元到 USDT 汇率"换算成美元后与非加密部分合并，
	// 再按展示币种换算（CNY 用户乘以汇率）
	totalUsd := moneySum.FiatMoney + cryptoUsdtToUsd(moneySum.CryptoMoney)
	depositAmount := convertUsdToDisplay(totalUsd, displayInfo)

	// ============ 上一周期数据，用于计算环比 ============
	prevNewUsers, err := model.CountNewUsersByProvider(providerId, prevStart.Unix(), currentStart.Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	prevMoneySum, err := model.SumTopUpMoneyByProvider(providerId, prevStart.Unix(), currentStart.Unix(),
		[]string{model.TopUpBizTypePayment, model.TopUpBizTypeSubscription})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	prevTotalUsd := prevMoneySum.FiatMoney + cryptoUsdtToUsd(prevMoneySum.CryptoMoney)

	// ============ 活跃用户指标（不依赖 period，仅按 provider_id） ============
	// 活跃定义：调用过模型（logs 表 type=LogTypeConsume 且 quota>0）
	// 周活/月活用滚动窗口（近7天/近30天），而非本周/本月
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	sevenDaysAgo := now.AddDate(0, 0, -7)
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	// ============ 日活：今日调用过模型 ============
	dailyActiveUsers, err := model.CountActiveUsersByProvider(providerId, todayStart.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// ============ 周活：近7天调用过模型 ============
	weeklyActiveUsers, err := model.CountActiveUsersByProvider(providerId, sevenDaysAgo.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// ============ 月活：近30天调用过模型 ============
	monthlyActiveUsers, err := model.CountActiveUsersByProvider(providerId, thirtyDaysAgo.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// ============ 活跃粘性 = 今日活跃 / 近7天活跃（即周活）；今日 ⊂ 近7天，故比值 ∈ [0,1] ============
	var activeStickiness float64
	if weeklyActiveUsers > 0 {
		activeStickiness = math.Round(float64(dailyActiveUsers)/float64(weeklyActiveUsers)*10000) / 10000
	}

	// ============ 活跃环比：日活较昨日、周活(近7天)较上个近7天 ============
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	fourteenDaysAgo := sevenDaysAgo.AddDate(0, 0, -7) // 上个近7天的起点
	yesterdayActive, err := model.CountActiveUsersByProvider(providerId, yesterdayStart.Unix(), todayStart.Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	prevWeeklyActive, err := model.CountActiveUsersByProvider(providerId, fourteenDaysAgo.Unix(), sevenDaysAgo.Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"total_users":               totalUsers,                                                       // 累计注册用户
			"new_users":                 newUsers,                                                         // 周期内新增注册用户
			"new_users_trend":           calcPercentChange(int(newUsers), int(prevNewUsers)),              // 新增用户环比
			"deposit_amount":            depositAmount,                                                    // 周期内入金金额（按展示币种换算）
			"deposit_amount_trend":      calcPercentChangeFloat(totalUsd, prevTotalUsd),                   // 入金金额环比（按美元口径）
			"daily_active_users":        dailyActiveUsers,                                                 // 日活：今日调用过模型
			"daily_active_users_trend":  calcPercentChange(int(dailyActiveUsers), int(yesterdayActive)),   // 日活环比（较昨日）
			"weekly_active_users":       weeklyActiveUsers,                                                // 周活：近7天调用过模型
			"weekly_active_users_trend": calcPercentChange(int(weeklyActiveUsers), int(prevWeeklyActive)), // 周活环比（较上个近7天）
			"monthly_active_users":      monthlyActiveUsers,                                               // 月活：近30天调用过模型
			"active_stickiness":         activeStickiness,                                                 // 活跃粘性 = 今日活跃 / 近7天活跃
			"display_symbol":            displayInfo.Symbol,                                               // 入金金额展示币种符号
		},
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

// GetRecords 列表数据
// "provider_id": 服务商ID（0 表示主站，>0 表示对应服务商）
func GetRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	providerId, _ := strconv.Atoi(c.Query("provider_id")) // 0 = 主站；>0 = 对应服务商

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
	users, total, err := model.GetUserRecordsByCondition(pageInfo, sortFields, startTimestamp, endTimestamp, usedQuotaMin, usedQuotaMax, quotaMin, quotaMax, requestCountMin, requestCountMax, strings.TrimSpace(c.Query("keyword")), providerId)
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

	// 获取展示币种
	displayInfo := getDisplayCurrencyForUser(c)

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
			UserId:         u.Id,                                                      // 用户ID
			RequestCount:   countResult.SuccessCount + countResult.ErrorCount,         // 使用次数
			UsedQuota:      convertQuotaToDisplay(u.UsedQuota, displayInfo),           // 消耗情况
			Invited:        u.InviterId > 0,                                           // 注册来源(是否是邀请注册)
			CreatedAt:      u.CreatedAt,                                               // 注册时间
			LastActiveTime: lastActiveMap[u.Id],                                       // 最后活跃时间
			Retention:      retention,                                                 // 留存状态
			Quota:          convertQuotaToDisplay(u.Quota, displayInfo),               // 余额
			TopupQuota:     convertQuotaToDisplay(int(topupMap[u.Id]), displayInfo),   // 充值金额
			WelfareQuota:   convertQuotaToDisplay(int(welfareMap[u.Id]), displayInfo), // 福利金额
		})
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"page":             pageInfo.Page,
			"page_size":        pageInfo.PageSize,
			"total":            pageInfo.Total,
			"items":            items,
			"display_symbol":   displayInfo.Symbol,
			"display_currency": displayInfo.Currency,
		},
	})
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

// GetProviders 获取服务商列表
func GetProviders(c *gin.Context) {
	// 获取分页参数
	pageInfo := common.GetPageQuery(c)

	// 分页查询服务商列表
	items, total, err := model.GetProviderConfigList(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))  // 设置总记录数
	pageInfo.SetItems(items)       // 设置列表项
	common.ApiSuccess(c, pageInfo) // 返回成功响应
}
