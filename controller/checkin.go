package controller

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// GetCheckinStatus 获取用户签到状态和历史记录
// 接口：GET /api/user/checkin?month=YYYY-MM
// 返回数据包含：签到功能开关、最小/最大奖励额度、当月签到统计、展示币种信息
func GetCheckinStatus(c *gin.Context) {
	userId := c.GetInt("id")
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	setting, err := getCurrentCheckinSetting(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !setting.Enabled {
		// 签到功能未启用，直接返回错误
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}

	// 获取当前用户的展示币种信息（根据时区决定 USD/CNY）
	displayInfo := getDisplayCurrencyForUser(c)
	// 获取查询的月份参数，默认为当前月（格式 YYYY-MM）
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))

	// 从数据库查询该用户在指定月份的签到统计和历史记录
	stats, err := model.GetUserCheckinStatsInProvider(providerId, userId, month)
	if err != nil {
		// 查询失败，返回错误信息
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 查询成功，返回签到状态数据
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": applyDisplayCurrencyInfo(gin.H{
			"enabled":   setting.Enabled,  // 签到功能是否启用
			"min_quota": setting.MinQuota, // 单次签到最小奖励额度
			"max_quota": setting.MaxQuota, // 单次签到最大奖励额度
			"stats":     stats,            // 签到统计数据（当月记录、累计天数等）
		}, displayInfo), // 注入展示币种信息（currency, symbol, rate）
	})
}

// DoCheckin 执行用户签到操作
// 接口：POST /api/user/checkin
// 签到成功后系统随机分配 [MinQuota, MaxQuota] 范围内的额度奖励
// 每日只能签到一次，重复签到会返回错误
func DoCheckin(c *gin.Context) {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	setting, err := getCurrentCheckinSetting(providerId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !setting.Enabled {
		// 签到功能未启用
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}

	// 获取当前用户 ID 和展示币种信息
	userId := c.GetInt("id")
	displayInfo := getDisplayCurrencyForUser(c)

	// 调用 model 层执行签到逻辑（包括：检查今日是否已签到、计算随机奖励额度、写入签到记录）
	checkin, err := model.UserCheckin(userId)
	if err != nil {
		// 签到失败（可能今日已签到，或其他错误）
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 签到成功，记录日志（方便管理员审计和问题排查）
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("用户签到，获得额度 %s", logger.LogQuota(checkin.QuotaAwarded)))

	// 返回签到成功结果
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "签到成功",
		"data": applyDisplayCurrencyInfo(gin.H{
			"quota_awarded": checkin.QuotaAwarded, // 本次签到获得的额度
			"checkin_date":  checkin.CheckinDate,  // 签到日期（YYYY-MM-DD）
		}, displayInfo), // 注入展示币种信息
	})
}

func getCurrentCheckinSetting(providerId int) (*operation_setting.CheckinSetting, error) {
	if providerId > 0 {
		cfg, err := model.GetProviderRewardConfig(providerId)
		if err != nil {
			return nil, err
		}
		return &operation_setting.CheckinSetting{
			Enabled:  cfg.CheckinEnabled,
			MinQuota: cfg.CheckinMinQuota,
			MaxQuota: cfg.CheckinMaxQuota,
		}, nil
	}
	setting := operation_setting.GetCheckinSetting()
	return setting, nil
}
