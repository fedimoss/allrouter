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

const (
	cryptoBillTaskTickInterval = 1 * time.Minute                   // 每分钟 tick 一次
	cryptoBillTaskEnabled      = true                              // 是否启用
	cryptoBillTaskRedisLockTTL = 2 * time.Hour                     // Redis 锁 TTL，防止重复执行
	cryptoBillTaskTimeEnv      = "WECHAT_TRADE_BILL_TASK_TIME_UTC" // 与微信共用同一触发时间
	cryptoBillBatchSize        = 100                               // 每批入库条数
)

var (
	cryptoBillTaskOnce        sync.Once    // 仅启动一次定时协程
	cryptoBillTaskRunning     atomic.Bool  // 协程是否正在运行
	cryptoBillTaskLastRunDate atomic.Value // 上次运行时间
)

// StartCryptoTradeBillTask 启动 Crypto 账单定时任务，应在应用初始化时调用。
// 仅在主节点（IsMasterNode）上运行，通过 sync.Once 保证全局只启动一次。
// 协程内每分钟 tick 一次，由 runCryptoBillTaskOnce 判断是否到达触发时刻。
// 实际执行逻辑：拉取前一天 Crypto 交易记录 → 入库 payment_bill_record → 对账写入 payment_bill_reconcile
func StartCryptoTradeBillTask() {
	// sync.Once 保证即使被多次调用也只启动一个定时协程
	cryptoBillTaskOnce.Do(func() {
		// 仅主节点执行，从节点跳过（多实例部署时避免重复拉取）
		if !common.IsMasterNode {
			return
		}

		// 检查是否启用
		if !cryptoBillTaskEnabled {
			return
		}

		// 使用协程池启动，不阻塞调用方
		gopool.Go(func() {
			// 获取触发时间
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

			// 启动前先执行一次，防止应用重启后错过当天的触发时刻
			ticker := time.NewTicker(cryptoBillTaskTickInterval)
			defer ticker.Stop()
			runCryptoBillTaskOnce()
			for range ticker.C {
				runCryptoBillTaskOnce()
			}
		})
	})
}

// runCryptoBillTaskOnce 三层防重复：触发时刻校验 → 进程内按日期去重 → 原子锁防并发 → Redis 分布式锁防多实例
func runCryptoBillTaskOnce() {
	// 获取当前UTC时间
	nowUTC := time.Now().UTC()

	// 校验是否到达触发时刻
	triggerHour, triggerMinute, ok := getCryptoBillTaskTriggerTimeUTC()
	if !ok {
		return
	}

	// 判断小时或者分钟是否匹配, 不匹配则直接返回
	if nowUTC.Hour() != triggerHour || nowUTC.Minute() != triggerMinute {
		return
	}

	// 转换UTC时间为日期为字符串格式
	runDate := nowUTC.Format("2006-01-02")

	// 检查是否已执行过
	if last, ok := cryptoBillTaskLastRunDate.Load().(string); ok && last == runDate {
		return
	}

	// 检查是否正在运行
	if !cryptoBillTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer cryptoBillTaskRunning.Store(false)

	// 尝试获取 Redis 分布式锁
	locked, err := tryAcquireCryptoBillTaskRedisLock(runDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("crypto trade bill task acquire redis lock failed: run_date=%s err=%v", runDate, err))
		return
	}

	// 检查是否成功获取锁
	if !locked {
		return
	}

	// 取 UTC 前一天的账单
	billDate := nowUTC.AddDate(0, 0, -1).Format("2006-01-02")
	result, err := RunCryptoBillWorkflow(billDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("crypto trade bill task failed: bill_date=%s err=%v", billDate, err))
		return
	}
	cryptoBillTaskLastRunDate.Store(runDate)
	logger.LogInfo(
		context.Background(),
		fmt.Sprintf("crypto trade bill task finished: bill_date=%s inserted=%d matched=%d abnormal=%d",
			result.BillDate, result.InsertedRows,
			result.InsertedRows),
	)
}

// buildCryptoBillTaskRedisLockKey 构建 Crypto 账单定时任务的 Redis 锁 key
func buildCryptoBillTaskRedisLockKey(runDate string) string {
	return fmt.Sprintf("new-api:crypto_trade_bill_task:%s", runDate)
}

// Redis 分布式锁，未启用 Redis 时直接放行（退化为进程内原子锁）
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
