// Package service 提供加密货币账单定时任务的启动与调度逻辑。
// 通过 StartCryptoTradeBillTask 在应用初始化时启动后台协程，
// 每分钟 tick 一次，到达配置的 UTC 触发时刻后执行前一天的账单生成与对账。
package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/bytedance/gopkg/util/gopool"
)

// 定时任务相关常量
const (
	cryptoBillTaskTickInterval = 1 * time.Minute                   // 每分钟 tick 一次，检查是否到达触发时刻
	cryptoBillTaskEnabled      = true                              // 全局开关：是否启用加密货币账单定时任务
	cryptoBillTaskRedisLockTTL = 2 * time.Hour                     // Redis 分布式锁 TTL，防止同一日期的任务被多实例重复执行
	cryptoBillTaskTimeEnv      = "WECHAT_TRADE_BILL_TASK_TIME_UTC" // 与微信账单共用同一触发时间的环境变量名，格式 HH:MM（UTC 时间）
	cryptoBillBatchSize        = 100                               // 每批入库条数（预留，当前未使用）
)

var (
	cryptoBillTaskOnce        sync.Once    // 确保定时协程全局只启动一次
	cryptoBillTaskRunning     atomic.Bool  // 进程级原子锁，防止同一分钟内重复执行
	cryptoBillTaskLastRunDate atomic.Value // 记录上次执行日期（字符串），防止同一天内重复执行
)

// StartCryptoTradeBillTask 启动加密货币账单定时任务，应在应用初始化时调用。
//
// 防护机制（四层防重复）：
//  1. sync.Once —— 保证全局只启动一个定时协程
//  2. IsMasterNode —— 仅主节点执行（多实例部署时从节点跳过）
//  3. 触发时刻校验 —— 只有当前 UTC 时间的小时和分钟与配置匹配时才执行
//  4. 进程内按日期去重 —— cryptoBillTaskLastRunDate 记录上次执行日期，同一天不重复执行
//  5. 原子锁 —— cryptoBillTaskRunning 防止并发的 tick 同时进入执行逻辑
//  6. Redis 分布式锁 —— tryAcquireCryptoBillTaskRedisLock 防止多实例重复执行同一天任务
//
// 实际执行逻辑：
//
//	拉取前一天 UTC 时间的加密货币交易记录 → 入库 payment_bill_record → 对账写入 payment_bill_reconcile
func StartCryptoTradeBillTask() {
	// sync.Once 保证即使被多次调用也只启动一个定时协程
	cryptoBillTaskOnce.Do(func() {
		// 仅主节点执行，从节点跳过（多实例部署时避免重复拉取）
		if !common.IsMasterNode {
			return
		}

		// 全局开关检查
		if !cryptoBillTaskEnabled {
			return
		}

		// 使用协程池启动，不阻塞调用方
		gopool.Go(func() {
			// 读取配置的触发时间（UTC）
			hour, minute, ok := getCryptoBillTaskTriggerTimeUTC()
			if ok {
				logger.LogInfo(
					context.Background(),
					fmt.Sprintf(
						"crypto trade bill task started: tick=%s trigger_utc=%02d:%02d env=%s",
						cryptoBillTaskTickInterval,
						hour, minute,
						cryptoBillTaskTimeEnv,
					),
				)
			} else {
				logger.LogWarn(
					context.Background(),
					fmt.Sprintf(
						"crypto trade bill task started without valid trigger time, please set env %s in HH:MM format",
						cryptoBillTaskTimeEnv,
					),
				)
			}

			// 启动后立即执行一次（防止应用重启后错过当天的触发时刻）
			ticker := time.NewTicker(cryptoBillTaskTickInterval)
			defer ticker.Stop()
			runCryptoBillTaskOnce()
			for range ticker.C {
				runCryptoBillTaskOnce()
			}
		})
	})
}

// runCryptoBillTaskOnce 单次执行入口，内置多层防护：
//  1. 校验当前 UTC 时间是否匹配配置的触发时刻
//  2. 进程内按日期去重（cryptoBillTaskLastRunDate）
//  3. 原子锁（cryptoBillTaskRunning）防止并发
//  4. Redis 分布式锁（tryAcquireCryptoBillTaskRedisLock）防止多实例重复
func runCryptoBillTaskOnce() {
	// 获取当前 UTC 时间
	nowUTC := time.Now().UTC()

	// 第一层防护：校验是否到达配置的触发时刻
	triggerHour, triggerMinute, ok := getCryptoBillTaskTriggerTimeUTC()
	if !ok {
		return
	}

	// 小时和分钟必须精确匹配
	if nowUTC.Hour() != triggerHour || nowUTC.Minute() != triggerMinute {
		return
	}

	// 第二层防护：按日期去重，今天已执行则跳过
	runDate := nowUTC.Format("2006-01-02")
	if last, ok := cryptoBillTaskLastRunDate.Load().(string); ok && last == runDate {
		return
	}

	// 第三层防护：原子锁，防止并发进入
	if !cryptoBillTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer cryptoBillTaskRunning.Store(false)

	// 第四层防护：获取 Redis 分布式锁
	locked, err := tryAcquireCryptoBillTaskRedisLock(runDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("crypto trade bill task acquire redis lock failed: run_date=%s err=%v", runDate, err))
		return
	}
	if !locked {
		return
	}

	// 取 UTC 前一天的日期作为账单日期
	billDate := nowUTC.AddDate(0, 0, -1).Format("2006-01-02")
	result, err := RunCryptoBillWorkflow(billDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("crypto trade bill task failed: bill_date=%s err=%v", billDate, err))
		return
	}

	// 记录执行日期，避免同一天重复执行
	cryptoBillTaskLastRunDate.Store(runDate)
	logger.LogInfo(
		context.Background(),
		fmt.Sprintf("crypto trade bill task finished: bill_date=%s inserted=%d",
			result.BillDate, result.InsertedRows),
	)
}

// buildCryptoBillTaskRedisLockKey 构建加密货币账单定时任务的 Redis 分布式锁 key
// 格式：new-api:crypto_trade_bill_task:{runDate}
func buildCryptoBillTaskRedisLockKey(runDate string) string {
	return fmt.Sprintf("new-api:crypto_trade_bill_task:%s", runDate)
}

// tryAcquireCryptoBillTaskRedisLock 尝试获取 Redis 分布式锁。
// 未启用 Redis 时直接放行（退化为仅依赖进程内原子锁）。
// 使用 SETNX 命令，TTL 为 2 小时，确保即使进程崩溃锁也会自动释放。
func tryAcquireCryptoBillTaskRedisLock(runDate string) (bool, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return true, nil
	}
	locked, err := common.RDB.SetNX(
		context.Background(),
		buildCryptoBillTaskRedisLockKey(runDate),
		runDate,
		cryptoBillTaskRedisLockTTL,
	).Result()
	if err != nil {
		return false, err
	}
	return locked, nil
}
