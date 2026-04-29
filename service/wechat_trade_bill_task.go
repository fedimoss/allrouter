package service

import (
	"context"
	"errors"
	"fmt"
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
)

var (
	wechatTradeBillTaskOnce        sync.Once
	wechatTradeBillTaskRunning     atomic.Bool
	wechatTradeBillTaskLastRunDate atomic.Value
)

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
			logger.LogInfo(context.Background(), fmt.Sprintf("wechat trade bill task started: tick=%s", wechatTradeBillTaskTickInterval))
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
	now := time.Now()
	if now.Hour() != 10 || now.Minute() != 0 {
		return
	}
	runDate := now.Format("2006-01-02")
	if last, ok := wechatTradeBillTaskLastRunDate.Load().(string); ok && last == runDate {
		return
	}
	if !wechatTradeBillTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer wechatTradeBillTaskRunning.Store(false)

	billDate := now.AddDate(0, 0, -1).Format("2006-01-02")
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
