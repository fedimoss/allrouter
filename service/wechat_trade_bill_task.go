package service

import (
	"context"
	"errors"
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
	"github.com/QuantumNous/new-api/pkg/wxpay_utility"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	wechatTradeBillTaskTickInterval = 1 * time.Minute
	wechatTradeBillTaskEnabled      = true
	wechatTradeBillTaskRedisLockTTL = 2 * time.Hour
	wechatTradeBillTaskTimeEnv      = "WECHAT_TRADE_BILL_TASK_TIME_UTC"
)

var (
	wechatTradeBillTaskOnce        sync.Once
	wechatTradeBillTaskRunning     atomic.Bool
	wechatTradeBillTaskLastRunDate atomic.Value
)

// buildWechatTradeBillTaskRedisLockKey 构建微信账单定时任务的 Redis 锁 key。
// 按执行日期加锁，避免多实例在同一天重复拉取同一批账单。
func buildWechatTradeBillTaskRedisLockKey(runDate string) string {
	return fmt.Sprintf("new-api:wechat_trade_bill_task:%s", runDate)
}

// tryAcquireWechatTradeBillTaskRedisLock 尝试获取 Redis 分布式锁。
// 如果当前未启用 Redis，则直接放行，继续使用进程内原子锁兜底。
func tryAcquireWechatTradeBillTaskRedisLock(runDate string) (bool, error) {
	if !common.RedisEnabled || common.RDB == nil {
		return true, nil
	}

	locked, err := common.RDB.SetNX(
		context.Background(),
		buildWechatTradeBillTaskRedisLockKey(runDate),
		runDate,
		wechatTradeBillTaskRedisLockTTL,
	).Result()
	if err != nil {
		return false, err
	}
	return locked, nil
}

// getWechatTradeBillTaskTriggerTimeUTC 读取定时任务触发时间。
// 环境变量格式固定为 HH:MM，按 UTC 时间解释；未配置或格式错误时返回 false。
func getWechatTradeBillTaskTriggerTimeUTC() (int, int, bool) {
	timeText := strings.TrimSpace(os.Getenv(wechatTradeBillTaskTimeEnv))
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

type WechatTradeBillRunResult struct {
	BillDate        string                           `json:"bill_date"`
	TradeBill       *QueryBillEntity                 `json:"trade_bill,omitempty"`
	ImportResult    *ImportedTradeBill               `json:"import_result,omitempty"`
	ReconcileResult *WechatTradeBillReconcileSummary `json:"reconcile_result,omitempty"`
	Message         string                           `json:"message,omitempty"`
}

func isWechatNoStatementError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *wxpay_utility.APIException
	if errors.As(err, &apiErr) {
		return strings.EqualFold(strings.TrimSpace(apiErr.ErrorCode()), "NO_STATEMENT_EXIST")
	}
	return false
}

// LoadWechatTradeBillConfig 从 service_configs 最新一条记录中加载微信账单所需的商户配置。
func LoadWechatTradeBillConfig() (*wxpay_utility.MchConfig, error) {
	serviceConfig, err := model.GetLatestServiceConfig()
	if err != nil {
		return nil, fmt.Errorf("load service_configs failed: %w", err)
	}

	mchID := strings.TrimSpace(serviceConfig.WechatMchID)
	certSerialNo := strings.TrimSpace(serviceConfig.WechatMchSerialNo)
	privateKeyPath := strings.TrimSpace(serviceConfig.WechatPrivateKeyPath)
	platformCertSerialNo, err := model.GetLatestServiceConfigColumn("wechat_serial_no")
	if err != nil {
		return nil, fmt.Errorf("load service_configs.wechat_serial_no failed: %w", err)
	}
	platformCertPath, err := model.GetLatestServiceConfigColumn("wechat_cert_path")
	if err != nil {
		return nil, fmt.Errorf("load service_configs.wechat_cert_path failed: %w", err)
	}

	if mchID == "" {
		return nil, fmt.Errorf("service_configs.wechat_mch_id is empty")
	}
	if certSerialNo == "" {
		return nil, fmt.Errorf("service_configs.wechat_mch_serial_no is empty")
	}
	if privateKeyPath == "" {
		return nil, fmt.Errorf("service_configs.wechat_private_key_path is empty")
	}
	if platformCertSerialNo == "" {
		return nil, fmt.Errorf("service_configs.wechat_serial_no is empty")
	}
	if platformCertPath == "" {
		return nil, fmt.Errorf("service_configs.wechat_cert_path is empty")
	}

	return wxpay_utility.CreateMchConfig(
		mchID,
		certSerialNo,
		privateKeyPath,
		platformCertSerialNo,
		platformCertPath,
	)
}

// RunWechatTradeBillWorkflow 执行单日微信账单的完整流程：申请账单、下载导入、再做对账。
func RunWechatTradeBillWorkflow(config *wxpay_utility.MchConfig, billDate string) (*WechatTradeBillRunResult, error) {
	billDate = strings.TrimSpace(billDate)
	if billDate == "" {
		return nil, fmt.Errorf("bill date is empty")
	}
	if config == nil {
		return nil, fmt.Errorf("wechat merchant config is nil")
	}

	request := &GetTradeBillRequest{
		BillDate: wxpay_utility.String(billDate),
		BillType: BILLTYPE_ALL.Ptr(),
		TarType:  TARTYPE_GZIP.Ptr(),
	}

	tradeBill, err := GetTradeBill(config, request)
	if err != nil {
		if isWechatNoStatementError(err) {
			return &WechatTradeBillRunResult{
				BillDate: billDate,
				ImportResult: &ImportedTradeBill{
					ParsedRows:    0,
					InsertedRows:  0,
					DuplicateRows: 0,
				},
				ReconcileResult: &WechatTradeBillReconcileSummary{
					TotalCount:    0,
					MatchedCount:  0,
					AbnormalCount: 0,
				},
				Message: "当天无账单",
			}, nil
		}
		return nil, err
	}
	//下载,保存,入库
	imported, err := DownloadExtractSaveAndImportTradeBill(config, billDate, request.TarType, tradeBill)
	if err != nil {
		return nil, err
	}
	//按账单日期范围对微信交易账单进行核对
	reconcileSummary, err := ReconcileWechatTradeBillsByBillDateRange(billDate, billDate)
	if err != nil {
		return nil, err
	}

	//return &WechatTradeBillRunResult{
	//	BillDate:        billDate,
	//	TradeBill:       tradeBill,
	//	ImportResult:    imported,
	//	ReconcileResult: reconcileSummary,
	//}, nil
	return &WechatTradeBillRunResult{
		BillDate:        billDate,
		TradeBill:       tradeBill,
		ImportResult:    imported,
		ReconcileResult: reconcileSummary,
	}, nil
}

// RunWechatTradeBillWorkflowWithDBConfig 使用数据库中的微信配置执行指定日期账单流程。
func RunWechatTradeBillWorkflowWithDBConfig(billDate string) (*WechatTradeBillRunResult, error) {
	config, err := LoadWechatTradeBillConfig()
	if err != nil {
		return nil, err
	}
	return RunWechatTradeBillWorkflow(config, billDate)
}

// StartWechatTradeBillTask 启动微信支付账单定时任务。
// 当前策略是每分钟检查一次，在每天上午 10:00 拉取前一天账单并执行对账。
func StartWechatTradeBillTask() {
	wechatTradeBillTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !wechatTradeBillTaskEnabled {
			return
		}

		gopool.Go(func() {
			hour, minute, ok := getWechatTradeBillTaskTriggerTimeUTC()
			if ok {
				logger.LogInfo(
					context.Background(),
					fmt.Sprintf(
						"wechat trade bill task started: tick=%s trigger_utc=%02d:%02d env=%s",
						wechatTradeBillTaskTickInterval,
						hour,
						minute,
						wechatTradeBillTaskTimeEnv,
					),
				)
			} else {
				logger.LogWarn(
					context.Background(),
					fmt.Sprintf(
						"wechat trade bill task started without valid trigger time, please set env %s in HH:MM format",
						wechatTradeBillTaskTimeEnv,
					),
				)
			}
			ticker := time.NewTicker(wechatTradeBillTaskTickInterval)
			defer ticker.Stop()
			runWechatTradeBillTaskOnce()
			for range ticker.C {
				runWechatTradeBillTaskOnce()
			}
		})
	})
}

// runWechatTradeBillTaskOnce 执行单次定时任务检查。
// 这里做了三个保护：
// 1. 只在 10:00 触发；
// 2. 同一天只执行一次；
// 3. 使用原子锁避免并发重复执行。
func runWechatTradeBillTaskOnce() {
	nowUTC := time.Now().UTC()
	triggerHour, triggerMinute, ok := getWechatTradeBillTaskTriggerTimeUTC()
	if !ok {
		return
	}
	if nowUTC.Hour() != triggerHour || nowUTC.Minute() != triggerMinute {
		return
	}
	runDate := nowUTC.Format("2006-01-02")
	if last, ok := wechatTradeBillTaskLastRunDate.Load().(string); ok && last == runDate {
		return
	}
	if !wechatTradeBillTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer wechatTradeBillTaskRunning.Store(false)

	// 进程内锁通过后，再尝试获取 Redis 分布式锁，避免多实例重复执行。
	//locked, err := tryAcquireWechatTradeBillTaskRedisLock(runDate)
	//if err != nil {
	//	logger.LogWarn(context.Background(), fmt.Sprintf("wechat trade bill task acquire redis lock failed: run_date=%s err=%v", runDate, err))
	//	return
	//}
	//if !locked {
	//	return
	//}

	billDate := nowUTC.AddDate(0, 0, -1).Format("2006-01-02")
	result, err := RunWechatTradeBillWorkflowWithDBConfig(billDate)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("wechat trade bill task failed: bill_date=%s err=%v", billDate, err))
		return
	}
	wechatTradeBillTaskLastRunDate.Store(runDate)
	logger.LogInfo(
		context.Background(),
		fmt.Sprintf(
			"wechat trade bill task finished: bill_date=%s parsed_rows=%d inserted_rows=%d matched=%d abnormal=%d",
			result.BillDate,
			result.ImportResult.ParsedRows,
			result.ImportResult.InsertedRows,
			result.ReconcileResult.MatchedCount,
			result.ReconcileResult.AbnormalCount,
		),
	)
}
