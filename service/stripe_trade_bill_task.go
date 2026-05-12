package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
)

const (
	stripeBillTaskTickInterval = 1 * time.Minute
	stripeBillTaskEnabled      = true
	stripeBillTaskRedisLockTTL = 2 * time.Hour
	stripeBillTaskTimeEnv      = "WECHAT_TRADE_BILL_TASK_TIME_UTC" // 与微信共用同一触发时间
	stripeBillBatchSize        = 100                               // 每批入库条数
)

var (
	stripeBillTaskOnce        sync.Once
	stripeBillTaskRunning     atomic.Bool
	stripeBillTaskLastRunDate atomic.Value
)

// buildStripeBillTaskRedisLockKey 构建 Stripe 账单定时任务的 Redis 锁 key
func buildStripeBillTaskRedisLockKey(runDate string) string {
	return fmt.Sprintf("new-api:stripe_trade_bill_task:%s", runDate)
}

// Redis 分布式锁，未启用 Redis 时直接放行（退化为进程内原子锁）
func tryAcquireStripeBillTaskRedisLock(runDate string) (bool, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return true, nil
	}
	locked, err := common.RDB.SetNX(
		context.Background(),
		buildStripeBillTaskRedisLockKey(runDate),
		runDate,
		stripeBillTaskRedisLockTTL,
	).Result()
	if err != nil {
		return false, err
	}
	return locked, nil
}

// getStripeBillTaskTriggerTimeUTC 读取环境变量 WECHAT_TRADE_BILL_TASK_TIME_UTC 中的触发时间。
// 格式为 HH:MM（按 UTC 解释），如 "02:00" 表示 UTC 02:00（北京时间 10:00）执行。
// 与微信定时任务共用同一个环境变量，保持两个渠道在同一时刻拉取前一天账单。
// 返回值：(小时, 分钟, 是否解析成功)，未配置或格式错误时返回 false。
func getStripeBillTaskTriggerTimeUTC() (int, int, bool) {
	timeText := strings.TrimSpace(os.Getenv(stripeBillTaskTimeEnv))
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

// stripeSessionToBillRecord 将一条已完成的 Stripe Checkout Session 转换为 PaymentBillRecord。
// 改用 Session 而非 Charge 作为数据源，因为 Session.ClientReferenceID 就是本地 trade_no，
// 可以天然与 top_ups.trade_no / subscription_orders.trade_no 匹配，无需额外映射。
func stripeSessionToBillRecord(sess *stripe.CheckoutSession, billDate string, rowIndex int) *model.PaymentBillRecord {
	// Stripe 金额以分为单位，转为元
	amount := fmt.Sprintf("%.2f", float64(sess.AmountTotal)/100.0)
	currency := strings.ToUpper(string(sess.Currency))

	// 基于 Session ID 生成唯一哈希，用于入库去重
	rowHash := fmt.Sprintf("%x", sha256.Sum256([]byte(sess.ID)))

	// PaymentIntent 信息仅在 payment 模式下存在
	channelTradeNo := ""
	if sess.PaymentIntent != nil {
		channelTradeNo = sess.PaymentIntent.ID // pi_xxx
	}

	payerID := ""
	if sess.Customer != nil {
		payerID = sess.Customer.ID // cus_xxx
	}

	record := &model.PaymentBillRecord{
		ChannelType: stripeChannelType,
		BillDate:    billDate,
		RowIndex:    rowIndex,
		RowHash:     rowHash,
		// 本地账单存储的时间为 CST，Stripe Created 时间为 UTC，需要转换为 CST
		TradeTime:       time.Unix(sess.Created, 0).In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"),
		ChannelTradeNo:  channelTradeNo,         // pi_xxx（payment 模式下有值）
		MerchantTradeNo: sess.ClientReferenceID, // 本地 trade_no（ref_xxx / sub_ref_xxx），对账的关键匹配字段
		PayerID:         payerID,                // cus_xxx
		TradeType:       string(sess.Mode),      // "payment" 或 "subscription"
		TradeStatus:     string(sess.Status),    // "complete"
		Currency:        currency,
		TotalAmount:     amount,
		OrderAmount:     amount,
		RawDataJSON:     serializeSessionToJSON(sess),
		CreatedAt:       common.GetTimestamp(),
	}

	return record
}

// serializeSessionToJSON 将 Stripe CheckoutSession 序列化为 JSON
func serializeSessionToJSON(sess *stripe.CheckoutSession) string {
	data, err := common.Marshal(sess)
	if err != nil {
		return ""
	}
	return string(data)
}

type StripeBillRunResult struct {
	BillDate        string                           `json:"bill_date"`
	InsertedRows    int                              `json:"inserted_rows"`
	ReconcileResult *StripeTradeBillReconcileSummary `json:"reconcile_result,omitempty"`
	Message         string                           `json:"message,omitempty"`
}

// RunStripeBillWorkflow 拉取指定日期的 Stripe CheckoutSession → 入库 → 对账
func RunStripeBillWorkflow(billDate string) (*StripeBillRunResult, error) {
	// time.Parse 默认 UTC，与 Stripe CreatedRange 的时间戳语义一致
	date, err := time.Parse("2006-01-02", billDate)
	if err != nil {
		return nil, fmt.Errorf("日期格式错误: %s", billDate)
	}
	startTime := date.Unix()
	endTime := date.AddDate(0, 0, 1).Unix()

	// 设置 Stripe API 密钥
	stripe.Key = setting.StripeApiSecret
	if stripe.Key == "" {
		return nil, fmt.Errorf("Stripe API 密钥未配置")
	}

	// 只拉取已完成的 Session，避免未支付的进入对账
	params := &stripe.CheckoutSessionListParams{
		CreatedRange: &stripe.RangeQueryParams{
			GreaterThanOrEqual: startTime,
			LesserThan:         endTime,
		},
		Status: stripe.String(string(stripe.CheckoutSessionStatusComplete)),
	}
	params.Limit = stripe.Int64(100)

	// 迭代拉取，边查边存
	result := session.List(params)
	var batch []*model.PaymentBillRecord
	rowIndex := 0
	totalInserted := int64(0)

	for result.Next() {
		rowIndex++
		sess := result.CheckoutSession()
		record := stripeSessionToBillRecord(sess, billDate, rowIndex)
		batch = append(batch, record)

		// 够一批就入库
		if len(batch) >= stripeBillBatchSize {
			inserted, err := model.BatchInsertPaymentBillRecords(batch)
			if err != nil {
				return nil, fmt.Errorf("批量入库失败: %w", err)
			}
			totalInserted += inserted
			batch = batch[:0]
		}
	}

	// 处理最后不满一批的
	if len(batch) > 0 {
		inserted, err := model.BatchInsertPaymentBillRecords(batch)
		if err != nil {
			return nil, fmt.Errorf("批量入库失败(尾部): %w", err)
		}
		totalInserted += inserted
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("Stripe 查询失败: %w", err)
	}

	// 对账使用与拉取相同的 UTC 时间戳，确保本地查询范围与 Stripe 数据对齐
	reconcileSummary, err := ReconcileStripeTradeBillsByBillDateRange(billDate, billDate)
	if err != nil {
		return nil, fmt.Errorf("Stripe 对账失败: %w", err)
	}

	return &StripeBillRunResult{
		BillDate:        billDate,
		InsertedRows:    int(totalInserted),
		ReconcileResult: reconcileSummary,
	}, nil
}

// StartStripeTradeBillTask 启动 Stripe 账单定时任务，应在应用初始化时调用。
// 仅在主节点（IsMasterNode）上运行，通过 sync.Once 保证全局只启动一次。
// 协程内每分钟 tick 一次，由 runStripeBillTaskOnce 判断是否到达触发时刻。
// 实际执行逻辑：拉取前一天 Stripe CheckoutSession → 入库 payment_bill_record → 对账写入 payment_bill_reconcile
func StartStripeTradeBillTask() {
	// sync.Once 保证即使被多次调用也只启动一个定时协程
	stripeBillTaskOnce.Do(func() {
		// 仅主节点执行，从节点跳过（多实例部署时避免重复拉取）
		if !common.IsMasterNode {
			return
		}
		if !stripeBillTaskEnabled {
			return
		}

		// 使用协程池启动，不阻塞调用方
		gopool.Go(func() {
			hour, minute, ok := getStripeBillTaskTriggerTimeUTC()
			if ok {
				logger.LogInfo(
					context.Background(),
					fmt.Sprintf(
						"stripe trade bill task started: tick=%s trigger_utc=%02d:%02d env=%s",
						stripeBillTaskTickInterval,
						hour, minute,
						stripeBillTaskTimeEnv,
					),
				)
			} else {
				logger.LogWarn(
					context.Background(),
					fmt.Sprintf(
						"stripe trade bill task started without valid trigger time, please set env %s in HH:MM format",
						stripeBillTaskTimeEnv,
					),
				)
			}

			// 启动前先执行一次，防止应用重启后错过当天的触发时刻
			ticker := time.NewTicker(stripeBillTaskTickInterval)
			defer ticker.Stop()
			runStripeBillTaskOnce()
			for range ticker.C {
				runStripeBillTaskOnce()
			}
		})
	})
}

// runStripeBillTaskOnce 三层防重复：触发时刻校验 → 进程内按日期去重 → 原子锁防并发 → Redis 分布式锁防多实例
func runStripeBillTaskOnce() {
	nowUTC := time.Now().UTC()
	triggerHour, triggerMinute, ok := getStripeBillTaskTriggerTimeUTC()
	if !ok {
		return
	}

	if nowUTC.Hour() != triggerHour || nowUTC.Minute() != triggerMinute {
		return
	}

	// 同一天只执行一次
	runDate := nowUTC.Format("2006-01-02")
	if last, ok := stripeBillTaskLastRunDate.Load().(string); ok && last == runDate {
		return
	}
	if !stripeBillTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer stripeBillTaskRunning.Store(false)

	// 尝试获取 Redis 分布式锁
	locked, err := tryAcquireStripeBillTaskRedisLock(runDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("stripe trade bill task acquire redis lock failed: run_date=%s err=%v", runDate, err))
		return
	}
	if !locked {
		return
	}

	// 取 UTC 前一天的账单
	billDate := nowUTC.AddDate(0, 0, -1).Format("2006-01-02")
	result, err := RunStripeBillWorkflow(billDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("stripe trade bill task failed: bill_date=%s err=%v", billDate, err))
		return
	}
	stripeBillTaskLastRunDate.Store(runDate)
	logger.LogInfo(
		context.Background(),
		fmt.Sprintf("stripe trade bill task finished: bill_date=%s inserted=%d matched=%d abnormal=%d",
			result.BillDate, result.InsertedRows,
			result.ReconcileResult.MatchedCount, result.ReconcileResult.AbnormalCount),
	)
}
