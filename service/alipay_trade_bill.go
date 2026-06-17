// Package service 提供支付宝账单下载的实现。
// 通过 alipay.data.dataservice.bill.downloadurl.query 接口查询对账单下载地址，
// 然后立即通过 HTTP 下载账单压缩包（下载地址仅 30 秒有效）并解压到以账单日期命名的文件夹。
// 当前阶段仅下载并解压，账单解析、入库与对账在后续阶段实现。
package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	alipay "github.com/smartwalle/alipay/v3"
	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	// alipayTradeBillDocDir 账单保存根目录（相对进程工作目录），每个账单日期在其下建独立文件夹
	alipayTradeBillDocDir = "alipaytradebills"
	// alipayTradeBillType 业务账单（trade）；账务账单为 signcustomer
	alipayTradeBillType = "trade"
)

// alipayGbkDecoder 解码支付宝账单 zip 中 GBK 编码的文件名（中文 Windows 默认编码）。
var alipayGbkDecoder = simplifiedchinese.GBK.NewDecoder()

// alipayConfig 是从 options 表（common.OptionMap）加载的支付宝商户配置。
type alipayConfig struct {
	AppID      string // 应用 APPID（option: AlipayAppId）
	PrivateKey string // 应用私钥（option: AlipayPrivateKey，开放平台密钥工具生成的 base64 串，无需 PEM 头）
	PublicKey  string // 支付宝公钥（option: AlipayPublicCert，base64 串，非应用公钥）
	IsProd     bool   // 是否生产环境（option: AlipayIsProduction，默认 true）
}

// AlipayTradeBillService 封装支付宝客户端与账单文件下载用的 HTTP 客户端。
type AlipayTradeBillService struct {
	client     *alipay.Client
	httpClient *http.Client
}

// AlipayBillRunResult 是单次账单下载与对账流程的返回结果。
type AlipayBillRunResult struct {
	BillDate      string `json:"bill_date"`
	DownloadURL   string `json:"download_url"`
	SavedPath     string `json:"saved_path"`     // 解压后的文件夹路径
	CSVPath       string `json:"csv_path"`       // 解析的业务明细 CSV 路径
	FileCount     int    `json:"file_count"`     // 解压出的文件数
	FileSize      int64  `json:"file_size"`      // 解压出的总字节数
	ParsedRows    int    `json:"parsed_rows"`    // 解析出的业务明细行数
	InsertedRows  int64  `json:"inserted_rows"`  // 入库的业务明细行数
	MatchedCount  int64  `json:"matched_count"`  // 对账一致条数
	AbnormalCount int64  `json:"abnormal_count"` // 对账异常条数
	HasBill       bool   `json:"has_bill"`
	Message       string `json:"message,omitempty"`
}

// LoadAlipayTradeBillConfig 从 options 表（common.OptionMap）读取支付宝商户配置并校验必填项。
// 选项键：AlipayAppId、AlipayPrivateKey、AlipayPublicCert（支付宝公钥，公钥模式）、AlipayIsProduction。
func LoadAlipayTradeBillConfig() (*alipayConfig, error) {
	common.OptionMapRWMutex.RLock()
	appID := strings.TrimSpace(common.OptionMap["AlipayAppId"])
	privateKey := strings.TrimSpace(common.OptionMap["AlipayPrivateKey"])
	publicKey := strings.TrimSpace(common.OptionMap["AlipayPublicCert"])
	isProdText := strings.TrimSpace(common.OptionMap["AlipayIsProduction"])
	common.OptionMapRWMutex.RUnlock()

	if appID == "" {
		return nil, fmt.Errorf("option AlipayAppId is empty")
	}
	if privateKey == "" {
		return nil, fmt.Errorf("option AlipayPrivateKey is empty")
	}
	if publicKey == "" {
		return nil, fmt.Errorf("option AlipayPublicCert is empty")
	}
	// AlipayIsProduction 未配置或解析失败时默认生产环境
	isProd := true
	if isProdText != "" {
		if v, err := strconv.ParseBool(isProdText); err == nil {
			isProd = v
		}
	}
	return &alipayConfig{
		AppID:      appID,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		IsProd:     isProd,
	}, nil
}

// NewAlipayClient 使用公钥模式构造支付宝客户端（RSA2 / UTF-8 / JSON 由 SDK 固定）。
func NewAlipayClient(cfg *alipayConfig) (*alipay.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("alipay config is nil")
	}
	client, err := alipay.New(cfg.AppID, cfg.PrivateKey, cfg.IsProd)
	if err != nil {
		return nil, fmt.Errorf("alipay.New failed: %w", err)
	}
	if err := client.LoadAliPayPublicKey(cfg.PublicKey); err != nil {
		return nil, fmt.Errorf("alipay LoadAliPayPublicKey failed: %w", err)
	}
	return client, nil
}

// NewAlipayTradeBillService 构造账单下载服务。
func NewAlipayTradeBillService(cfg *alipayConfig) (*AlipayTradeBillService, error) {
	client, err := NewAlipayClient(cfg)
	if err != nil {
		return nil, err
	}
	return &AlipayTradeBillService{
		client:     client,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// QueryBillDownloadURL 调用 alipay.data.dataservice.bill.downloadurl.query 获取账单下载地址。
// 当该日期没有账单时返回 hasBill=false 而非错误（避免空账单日产生告警噪音）。
func (s *AlipayTradeBillService) QueryBillDownloadURL(ctx context.Context, billDate string) (downloadURL string, hasBill bool, err error) {
	param := alipay.BillDownloadURLQuery{
		BillType: alipayTradeBillType,
		BillDate: billDate,
		// AppAuthToken 留空：自持应用模式。第三方代调用（ISV）模式需设置应用授权令牌。
	}
	rsp, err := s.client.BillDownloadURLQuery(ctx, param)
	if err != nil {
		// smartwalle SDK 把业务失败（含"当天无账单"）作为 err 返回（*alipay.Error），此时 rsp 为 nil。
		// 需从 err 中提取 SubCode/SubMsg 判断是否属于"无账单"，避免空账单日产生告警噪音。
		var apiErr *alipay.Error
		if errors.As(err, &apiErr) {
			if isAlipayNoBillError(apiErr.SubCode, apiErr.SubMsg) {
				logger.LogInfo(ctx, fmt.Sprintf(
					"alipay bill download url query: no bill for bill_date=%s code=%s sub_code=%s sub_msg=%s",
					billDate, apiErr.Code, apiErr.SubCode, apiErr.SubMsg))
				return "", false, nil
			}
			return "", false, fmt.Errorf("alipay bill query business error: code=%s msg=%s sub_code=%s sub_msg=%s",
				apiErr.Code, apiErr.Msg, apiErr.SubCode, apiErr.SubMsg)
		}
		return "", false, fmt.Errorf("alipay BillDownloadURLQuery call failed: %w", err)
	}
	if rsp == nil {
		return "", false, fmt.Errorf("alipay bill query returned empty response")
	}
	if rsp.IsFailure() {
		if isAlipayNoBillError(rsp.SubCode, rsp.SubMsg) {
			logger.LogInfo(ctx, fmt.Sprintf(
				"alipay bill download url query: no bill for bill_date=%s code=%s sub_code=%s sub_msg=%s",
				billDate, rsp.Code, rsp.SubCode, rsp.SubMsg))
			return "", false, nil
		}
		return "", false, fmt.Errorf("alipay bill query business error: code=%s msg=%s sub_code=%s sub_msg=%s",
			rsp.Code, rsp.Msg, rsp.SubCode, rsp.SubMsg)
	}
	downloadURL = strings.TrimSpace(rsp.BillDownloadURL)
	if downloadURL == "" {
		return "", false, fmt.Errorf("alipay bill query returned empty download URL: bill_date=%s code=%s msg=%s sub_code=%s sub_msg=%s",
			billDate, rsp.Code, rsp.Msg, rsp.SubCode, rsp.SubMsg)
	}
	return downloadURL, true, nil
}

// DownloadBillZip 立即通过 HTTP GET 下载预签名账单压缩包（地址约 30 秒有效，无需鉴权头），返回 zip 原始字节。
func (s *AlipayTradeBillService) DownloadBillZip(ctx context.Context, downloadURL string) ([]byte, error) {
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return nil, fmt.Errorf("alipay bill download URL is empty")
	}
	parsedURL, err := url.ParseRequestURI(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("invalid alipay bill download URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("invalid alipay bill download URL scheme: %s", parsedURL.Scheme)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	// 支付宝账单下载 CDN 可能拒绝默认的 Go-http-client User-Agent（返回 403），显式设置一个 UA。
	req.Header.Set("User-Agent", "new-api-alipay-bill/1.0")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download alipay bill failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("download alipay bill http %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// decodeZipEntryName 解码 zip 条目文件名。
// 支付宝账单 zip 的中文文件名用 GBK 编码且未置 UTF-8 标志位（flags bit 0x800），
// Go 的 archive/zip 此时会把 GBK 原始字节原样赋给 f.Name（不做 CP437 转码），
// 直接使用会出现中文乱码。因此当未置 UTF-8 标志位时按 GBK 解码为 UTF-8。
func decodeZipEntryName(name string, flags uint16) string {
	if flags&0x800 != 0 {
		return name // UTF-8 标志位已置位，本身即为 UTF-8
	}
	if decoded, err := alipayGbkDecoder.String(name); err == nil {
		return decoded
	}
	return name
}

// ExtractBillZip 将账单 zip 解压到 destDir 目录（业务账单 zip 内含"业务账单明细"与"业务账单汇总"两张 CSV）。
// 内置 zip-slip 防护，拒绝解压逃逸出 destDir 的条目。返回（解压文件数，解压总字节数）。
func ExtractBillZip(zipBytes []byte, destDir string) (int, int64, error) {
	r, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return 0, 0, fmt.Errorf("open alipay bill zip failed: %w", err)
	}
	destDirClean := filepath.Clean(destDir)
	if err := os.MkdirAll(destDirClean, 0o755); err != nil {
		return 0, 0, err
	}
	count := 0
	var totalSize int64
	for _, f := range r.File {
		// 支付宝账单 zip 的中文文件名用 GBK 编码，先解码为 UTF-8（详见 decodeZipEntryName）
		entryName := decodeZipEntryName(f.Name, f.Flags)
		destPath := filepath.Join(destDirClean, entryName)
		// zip-slip 防护：解压后的路径必须仍在 destDir 之内
		rel, err := filepath.Rel(destDirClean, destPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return count, totalSize, fmt.Errorf("zip entry %q escapes destination dir", entryName)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return count, totalSize, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return count, totalSize, err
		}
		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return count, totalSize, err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return count, totalSize, err
		}
		// 先 Copy 再依次 Close，并检查错误，避免吞掉截断/写入失败导致账单文件损坏却被误判为成功。
		n, copyErr := io.Copy(out, rc)
		rc.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return count, totalSize, copyErr
		}
		if closeErr != nil {
			return count, totalSize, fmt.Errorf("close bill file %q failed: %w", entryName, closeErr)
		}
		count++
		totalSize += n
	}
	return count, totalSize, nil
}

// buildAlipayBillFolderPath 生成账单解压目录：alipaytradebills/alipay_trade_bill_{billDate}_{ts}
func buildAlipayBillFolderPath(billDate string) string {
	name := fmt.Sprintf("alipay_trade_bill_%s_%d", strings.TrimSpace(billDate), common.GetTimestamp())
	return filepath.Join(alipayTradeBillDocDir, name)
}

// isAlipayNoBillError 判断业务失败是否属于"该日期无账单"。
// 支付宝账单下载接口在指定日期无交易/无账单时返回业务错误，
// 此类情况属于正常结果（非异常），不应作为错误告警。
func isAlipayNoBillError(subCode, subMsg string) bool {
	// 支付宝账单下载接口在指定日期无账单时返回业务错误（作为 *alipay.Error 由 SDK 抛出）。
	// 官方 sub_code 主要为 NO_BILL_DATA（该账单类型当天无业务）/ BILL_NOT_EXIST（账单不存在）。
	// 此类情况属于正常结果（非异常），不应作为错误告警。
	combined := strings.ToLower(subCode + " " + subMsg)
	for _, kw := range []string{
		"no_bill_data", "bill_not_exist", "no_bill", "not exist", "no data",
		"无记录", "不存在", "没有账单", "无账单",
	} {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

// mapAlipayBillField 将支付宝业务明细中文表头映射到数据库模型字段。
// 支付宝业务明细的表头（业务账单 zip 内的"业务明细.csv"）形如：
//
//	支付宝交易号,商户订单号,业务类型,商品名称,创建时间,完成时间,门店编号,门店名称,
//	操作员,终端号,对方账户,订单金额（元）,商家实收（元）,支付宝红包（元）,集分宝（元）,
//	支付宝优惠（元）,商家优惠（元）,券核销金额（元）,券名称,商家红包消费金额（元）,
//	卡消费金额（元）,退款批次号/请求号,服务费（元）,分润（元）,备注
func mapAlipayBillField(row *model.PaymentBillRecord, header string, value string) {
	switch canonicalTradeBillHeader(header) {
	case "支付宝交易号":
		row.ChannelTradeNo = value
	case "商户订单号":
		row.MerchantTradeNo = value
	case "业务类型":
		// 支付宝业务明细无显式交易状态列，按业务类型推导：交易→success，退款→refund。
		// TradeType(订单类别: payment/subscription)改由商户订单号前缀推导，见 ParseAlipayTradeBillCSV。
		row.TradeStatus = deriveAlipayTradeStatus(value)
	case "商品名称":
		row.GoodsName = value
	case "完成时间":
		row.TradeTime = value
	case "终端号":
		row.DeviceID = value
	case "对方账户":
		row.PayerID = value
	case "订单金额（元）":
		row.OrderAmount = value
	case "商家实收（元）":
		row.TotalAmount = value
	case "支付宝红包（元）":
		row.EnterpriseRedPacket = value
	case "退款批次号/请求号":
		row.ChannelRefundNo = value
	case "服务费（元）":
		row.ServiceFee = value
	case "备注":
		row.PackageData = value
	}
}

// deriveAlipayTradeStatus 依据业务类型推导交易状态（支付宝业务明细无显式状态列）。
func deriveAlipayTradeStatus(bizType string) string {
	bizType = strings.TrimSpace(bizType)
	if strings.Contains(bizType, "退款") {
		return "refund"
	}
	if strings.Contains(bizType, "交易") {
		return "success"
	}
	return bizType
}

// deriveAlipayTradeTypeFromMerchantOrderNo 依据商户订单号(== 本地 TradeNo)前缀推导订单类别。
// 支付宝业务明细的"业务类型"列(交易/退款)只反映是交易还是退款，无法区分订单类别，
// 故 TradeType 改由订单号前缀决定：
//   - SUB 开头为套餐订阅 -> "subscription"
//   - USR 开头为充值支付 -> "payment"
//
// 其它未知前缀默认按充值支付(payment)处理。
func deriveAlipayTradeTypeFromMerchantOrderNo(merchantTradeNo string) string {
	s := strings.ToUpper(strings.TrimSpace(merchantTradeNo))
	if strings.HasPrefix(s, "SUB") {
		return model.TopUpBizTypeSubscription
	}
	return model.TopUpBizTypePayment
}

// ParseAlipayTradeBillCSV 解析支付宝业务明细 CSV，转成数据库模型。
// 解析规则（与用户提供的账单格式一致）：
//  1. 跳过开头的 # 注释行，定位"#...业务明细列表..."起始标记；
//  2. 起始标记的下一行为表头，据此按列名映射字段；
//  3. 表头之后为数据行，直到遇到"#...业务明细列表结束..."结束标记（或任意 # 开头的行）。
//
// csvContent 必须是已从 GBK 解码为 UTF-8 的内容（见 findAlipayBillDetailCSV）。
func ParseAlipayTradeBillCSV(billDate string, filePath string, csvContent []byte) ([]*model.PaymentBillRecord, error) {
	reader := csv.NewReader(bytes.NewReader(csvContent))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	// 1. 定位业务明细列表起始标记，其下一行为表头
	headerIdx := -1
	for i, record := range records {
		if len(record) == 0 {
			continue
		}
		first := canonicalTradeBillHeader(record[0])
		if strings.Contains(first, "业务明细列表") && !strings.Contains(first, "结束") {
			headerIdx = i + 1
			break
		}
	}
	if headerIdx < 0 || headerIdx >= len(records) {
		return nil, fmt.Errorf("alipay bill detail header not found")
	}
	headers := make([]string, len(records[headerIdx]))
	for i, h := range records[headerIdx] {
		headers[i] = normalizeTradeBillText(h)
	}

	// 2. 表头之后为数据行，遇 # 开头（结束标记/页脚）即停止
	rows := make([]*model.PaymentBillRecord, 0)
	for i := headerIdx + 1; i < len(records); i++ {
		record := records[i]
		if len(record) == 0 {
			continue
		}
		if strings.HasPrefix(canonicalTradeBillHeader(record[0]), "#") {
			break
		}
		// 跳过整行空白
		empty := true
		for _, field := range record {
			if normalizeTradeBillText(field) != "" {
				empty = false
				break
			}
		}
		if empty {
			continue
		}

		rawFields := make(map[string]string, len(record))
		row := &model.PaymentBillRecord{
			ChannelType: model.PaymentChannelTypeAlipay,
			BillDate:    billDate,
			FilePath:    filePath,
			RowIndex:    len(rows) + 1,
			RowHash:     hashTradeBillRow(record),
			RawLine:     strings.Join(record, ","),
		}
		for j, value := range record {
			header := fmt.Sprintf("column_%d", j+1)
			if j < len(headers) {
				header = headers[j]
			}
			cleanValue := normalizeTradeBillText(value)
			rawFields[header] = cleanValue
			mapAlipayBillField(row, header, cleanValue)
		}

		fieldsJSON, err := common.Marshal(rawFields)
		if err != nil {
			return nil, err
		}
		row.RawDataJSON = string(fieldsJSON)
		// TradeType(订单类别)由商户订单号前缀推导：SUB→subscription，USR→payment
		row.TradeType = deriveAlipayTradeTypeFromMerchantOrderNo(row.MerchantTradeNo)
		rows = append(rows, row)
	}

	return rows, nil
}

// findAlipayBillDetailCSV 在解压目录中查找业务明细 CSV（排除"汇总"文件），
// 返回文件路径与 GBK 解码后的 UTF-8 内容。
// 以用户提供的账单为准：业务明细（非汇总）才是对账数据源。
func findAlipayBillDetailCSV(folderPath string) (string, []byte, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return "", nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".csv") {
			continue
		}
		// 解压阶段已把文件名解码为 UTF-8，这里直接按 UTF-8 名匹配
		if !strings.Contains(name, "业务明细") || strings.Contains(name, "汇总") {
			continue
		}
		fullPath := filepath.Join(folderPath, name)
		raw, err := os.ReadFile(fullPath)
		if err != nil {
			return "", nil, err
		}
		// 账单内容为 GBK 编码，解码为 UTF-8 后再解析。
		// 注意：GBK 解码器对无效字节只会用 U+FFFD 静默替换、永不返回错误，
		// 因此不能依赖返回的 err 判断编码是否正确。改为事后校验已知锚点是否存在：
		// 若账单其实是 UTF-8 等其他编码，会在此给出根因明确的错误，
		// 而不是在下游 ParseAlipayTradeBillCSV 报与编码无关的 "header not found"。
		utf8Content, _ := alipayGbkDecoder.Bytes(raw)
		if !bytes.Contains(utf8Content, []byte("业务明细列表")) && !bytes.Contains(utf8Content, []byte("支付宝交易号")) {
			return "", nil, fmt.Errorf("alipay bill csv may use an unexpected encoding (expected GBK): missing known anchor in %s", name)
		}
		return fullPath, utf8Content, nil
	}
	return "", nil, fmt.Errorf("alipay bill detail csv not found in %s", folderPath)
}

// RunAlipayBillWorkflow 是账单下载与对账流程入口：
// 查询下载地址 -> 立即下载 zip -> 解压到文件夹 -> 解析业务明细 -> 入库 -> 对账。
func RunAlipayBillWorkflow(billDate string) (*AlipayBillRunResult, error) {
	billDate = strings.TrimSpace(billDate)
	if billDate == "" {
		return nil, fmt.Errorf("bill date is empty")
	}
	// 校验日期格式 yyyy-MM-dd，与 Stripe/Crypto 的 RunXxxBillWorkflow 保持一致，
	// 并阻止手动触发接口传入畸形日期直达支付宝。
	if _, err := time.Parse("2006-01-02", billDate); err != nil {
		return nil, fmt.Errorf("日期格式错误: %s", billDate)
	}
	cfg, err := LoadAlipayTradeBillConfig()
	if err != nil {
		return nil, err
	}
	svc, err := NewAlipayTradeBillService(cfg)
	if err != nil {
		return nil, err
	}

	// 用一个总体超时包裹"查询地址 + 下载"，确保拿到地址后立即下载，
	// 不会让 30 秒有效的下载地址在传输途中过期。
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	downloadURL, hasBill, err := svc.QueryBillDownloadURL(ctx, billDate)
	if err != nil {
		return nil, err
	}
	if !hasBill {
		// 当天无账单：记录结果但不视为错误
		return &AlipayBillRunResult{
			BillDate: billDate,
			HasBill:  false,
			Message:  fmt.Sprintf("no bill for bill_date=%s", billDate),
		}, nil
	}
	logger.LogInfo(ctx, fmt.Sprintf("alipay bill download url obtained: bill_date=%s url_len=%d", billDate, len(downloadURL)))

	// 下载 zip 字节后立即解压到以账单日期+时间戳命名的文件夹（不再保留压缩包）
	zipBytes, err := svc.DownloadBillZip(ctx, downloadURL)
	if err != nil {
		return nil, err
	}
	destDir := buildAlipayBillFolderPath(billDate)
	fileCount, totalSize, err := ExtractBillZip(zipBytes, destDir)
	if err != nil {
		return nil, err
	}
	logger.LogInfo(ctx, fmt.Sprintf("alipay bill extracted: bill_date=%s dir=%s files=%d size=%d", billDate, destDir, fileCount, totalSize))

	// 查找业务明细 CSV（内容从 GBK 解码为 UTF-8）并解析为账单明细记录
	csvPath, csvContent, err := findAlipayBillDetailCSV(destDir)
	if err != nil {
		return nil, err
	}
	rows, err := ParseAlipayTradeBillCSV(billDate, csvPath, csvContent)
	if err != nil {
		return nil, err
	}
	// 当天确认有账单：先清空该日期已有的账单记录与对账记录，再重新入库/对账，
	// 防止重复执行产生重复记录。放在 hasBill=true 分支内，避免"当天无账单"时误删已有数据。
	if err := model.DeletePaymentBillRecordsByChannelAndBillDate(model.PaymentChannelTypeAlipay, billDate); err != nil {
		return nil, err
	}
	if err := model.DeletePaymentBillReconcilesByChannelAndBillDate(model.PaymentChannelTypeAlipay, billDate); err != nil {
		return nil, err
	}
	// 批量入库账单明细（按 channel_type + row_hash 去重）
	inserted, err := model.BatchInsertPaymentBillRecords(rows)
	if err != nil {
		return nil, err
	}
	logger.LogInfo(ctx, fmt.Sprintf("alipay bill imported: bill_date=%s parsed=%d inserted=%d", billDate, len(rows), inserted))

	// 对账：将业务明细与本地 alipay 成功订单逐条核对
	reconcileSummary, err := ReconcileAlipayTradeBillsByBillDateRange(billDate, billDate)
	if err != nil {
		return nil, err
	}
	logger.LogInfo(ctx, fmt.Sprintf("alipay bill reconciled: bill_date=%s total=%d matched=%d abnormal=%d",
		billDate, reconcileSummary.TotalCount, reconcileSummary.MatchedCount, reconcileSummary.AbnormalCount))

	return &AlipayBillRunResult{
		BillDate:      billDate,
		DownloadURL:   downloadURL,
		SavedPath:     destDir,
		CSVPath:       csvPath,
		FileCount:     fileCount,
		FileSize:      totalSize,
		ParsedRows:    len(rows),
		InsertedRows:  inserted,
		MatchedCount:  reconcileSummary.MatchedCount,
		AbnormalCount: reconcileSummary.AbnormalCount,
		HasBill:       true,
		Message:       "downloaded, extracted, imported and reconciled",
	}, nil
}
