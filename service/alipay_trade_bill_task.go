// Package service 提供支付宝账单定时下载任务的启动与调度逻辑。
// 通过 StartAlipayTradeBillTask 在应用初始化时启动后台协程，
// 每分钟 tick 一次，到达配置的 UTC 触发时刻后下载前一天的账单文件。
package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/bytedance/gopkg/util/gopool"
)

// 定时任务相关常量
const (
	alipayBillTaskTickInterval = 1 * time.Minute                   // 每分钟 tick 一次，检查是否到达触发时刻
	alipayBillTaskEnabled      = true                              // 全局开关：是否启用支付宝账单定时任务
	alipayBillTaskRedisLockTTL = 2 * time.Hour                     // Redis 分布式锁 TTL，防止同一日期的任务被多实例重复执行
	alipayBillTaskTimeEnv      = "WECHAT_TRADE_BILL_TASK_TIME_UTC" // 与微信/Stripe/Crypto 共用同一触发时间的环境变量名，格式 HH:MM（UTC 时间）
)

var (
	alipayBillTaskOnce        sync.Once    // 确保定时协程全局只启动一次
	alipayBillTaskRunning     atomic.Bool  // 进程级原子锁，防止同一分钟内重复执行
	alipayBillTaskLastRunDate atomic.Value // 记录上次执行日期（字符串），防止同一天内重复执行
)

// StartAlipayTradeBillTask 启动支付宝账单定时下载任务，应在应用初始化时调用。
//
// 防护机制（与微信/Stripe/Crypto 一致）：
//  1. sync.Once —— 保证全局只启动一个定时协程
//  2. IsMasterNode —— 仅主节点执行（多实例部署时从节点跳过）
//  3. 触发时刻校验 —— 只有当前 UTC 时间的小时和分钟与配置匹配时才执行
//  4. 进程内按日期去重 —— alipayBillTaskLastRunDate 记录上次执行日期，同一天不重复执行
//  5. 原子锁 —— alipayBillTaskRunning 防止并发的 tick 同时进入执行逻辑
//  6. Redis 分布式锁 —— tryAcquireAlipayBillTaskRedisLock 防止多实例重复执行同一天任务
//
// 实际执行逻辑：
//
//	下载前一天 UTC 日期的支付宝业务账单文件并保存到本地（alipaytradebills/）
func StartAlipayTradeBillTask() {
	// sync.Once 保证即使被多次调用也只启动一个定时协程
	alipayBillTaskOnce.Do(func() {
		// 仅主节点执行，从节点跳过（多实例部署时避免重复下载）
		if !common.IsMasterNode {
			return
		}

		// 全局开关检查
		if !alipayBillTaskEnabled {
			return
		}

		// 使用协程池启动，不阻塞调用方
		gopool.Go(func() {
			// 读取配置的触发时间（UTC）
			hour, minute, ok := getAlipayBillTaskTriggerTimeUTC()
			if ok {
				logger.LogInfo(
					context.Background(),
					fmt.Sprintf(
						"alipay trade bill task started: tick=%s trigger_utc=%02d:%02d env=%s",
						alipayBillTaskTickInterval,
						hour, minute,
						alipayBillTaskTimeEnv,
					),
				)
			} else {
				logger.LogWarn(
					context.Background(),
					fmt.Sprintf(
						"alipay trade bill task started without valid trigger time, please set env %s in HH:MM format",
						alipayBillTaskTimeEnv,
					),
				)
			}

			// 启动后立即执行一次（防止应用重启后错过当天的触发时刻）
			ticker := time.NewTicker(alipayBillTaskTickInterval)
			defer ticker.Stop()
			runAlipayBillTaskOnce()
			for range ticker.C {
				runAlipayBillTaskOnce()
			}
		})
	})
}

// runAlipayBillTaskOnce 单次执行入口，内置多层防护：
//  1. 校验当前 UTC 时间是否匹配配置的触发时刻
//  2. 进程内按日期去重（alipayBillTaskLastRunDate）
//  3. 原子锁（alipayBillTaskRunning）防止并发
//  4. Redis 分布式锁（tryAcquireAlipayBillTaskRedisLock）防止多实例重复
func runAlipayBillTaskOnce() {
	// 获取当前 UTC 时间
	nowUTC := time.Now().UTC()

	// 第一层防护：校验是否到达配置的触发时刻
	triggerHour, triggerMinute, ok := getAlipayBillTaskTriggerTimeUTC()
	if !ok {
		return
	}

	// 小时和分钟必须精确匹配
	if nowUTC.Hour() != triggerHour || nowUTC.Minute() != triggerMinute {
		return
	}

	// 第二层防护：按日期去重，今天已执行则跳过
	runDate := nowUTC.Format("2006-01-02")
	if last, ok := alipayBillTaskLastRunDate.Load().(string); ok && last == runDate {
		return
	}

	// 第三层防护：原子锁，防止并发进入
	if !alipayBillTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer alipayBillTaskRunning.Store(false)

	// 第四层防护：获取 Redis 分布式锁
	locked, err := tryAcquireAlipayBillTaskRedisLock(runDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("alipay trade bill task acquire redis lock failed: run_date=%s err=%v", runDate, err))
		return
	}
	if !locked {
		return
	}

	// 取 UTC 前一天的日期作为账单日期
	billDate := nowUTC.AddDate(0, 0, -1).Format("2006-01-02")
	result, err := RunAlipayBillWorkflow(billDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("alipay trade bill task failed: bill_date=%s err=%v", billDate, err))
		return
	}

	// 记录执行日期，避免同一天重复执行
	alipayBillTaskLastRunDate.Store(runDate)
	if result.HasBill {
		logger.LogInfo(
			context.Background(),
			fmt.Sprintf("alipay trade bill task finished: bill_date=%s saved=%s parsed=%d inserted=%d matched=%d abnormal=%d",
				result.BillDate, result.SavedPath, result.ParsedRows, result.InsertedRows, result.MatchedCount, result.AbnormalCount),
		)
	} else {
		logger.LogInfo(
			context.Background(),
			fmt.Sprintf("alipay trade bill task finished (no bill): bill_date=%s msg=%s",
				result.BillDate, result.Message),
		)
	}
}

// getAlipayBillTaskTriggerTimeUTC 读取环境变量 WECHAT_TRADE_BILL_TASK_TIME_UTC 中的触发时间。
// 格式为 HH:MM（按 UTC 解释），如 "02:00" 表示 UTC 02:00（北京时间 10:00）执行。
// 与微信/Stripe/Crypto 共用同一个环境变量，保持各渠道在同一时刻拉取前一天账单。
// 返回值：(小时, 分钟, 是否解析成功)，未配置或格式错误时返回 false。
func getAlipayBillTaskTriggerTimeUTC() (int, int, bool) {
	timeText := strings.TrimSpace(os.Getenv(alipayBillTaskTimeEnv))
	if timeText == "" {
		return 0, 0, false
	}
	parts := strings.Split(timeText, ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	hour, errHour := strconv.Atoi(strings.TrimSpace(parts[0]))
	minute, errMinute := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errHour != nil || errMinute != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, false
	}
	return hour, minute, true
}

// buildAlipayBillTaskRedisLockKey 构建支付宝账单定时任务的 Redis 分布式锁 key
// 格式：new-api:alipay_trade_bill_task:{runDate}
func buildAlipayBillTaskRedisLockKey(runDate string) string {
	return fmt.Sprintf("new-api:alipay_trade_bill_task:%s", runDate)
}

// tryAcquireAlipayBillTaskRedisLock 尝试获取 Redis 分布式锁。
// 未启用 Redis 时直接放行（退化为仅依赖进程内原子锁）。
// 使用 SETNX 命令，TTL 为 2 小时，确保即使进程崩溃锁也会自动释放。
func tryAcquireAlipayBillTaskRedisLock(runDate string) (bool, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return true, nil
	}
	locked, err := common.RDB.SetNX(
		context.Background(),
		buildAlipayBillTaskRedisLockKey(runDate),
		runDate,
		alipayBillTaskRedisLockTTL,
	).Result()
	if err != nil {
		return false, err
	}
	return locked, nil
}
