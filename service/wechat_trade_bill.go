package service

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/wxpay_utility"
)

const (
	wechatPayAPIHost      = "https://api.mch.weixin.qq.com"
	wechatTradeBillDocDir = "wechattradebills"
)

// WechatTradeBillService 封装微信交易账单的申请、下载、解析与入库逻辑。
type WechatTradeBillService struct {
	httpClient *http.Client
	host       string
}

// DownloadedTradeBill 表示已经下载并完成哈希校验的账单文件内容。
type DownloadedTradeBill struct {
	Content       []byte
	VerifiedBytes []byte
	HashType      HashType
	HashValue     string
}

// ImportedTradeBill 表示账单下载、保存、解析、入库后的结果摘要。
type ImportedTradeBill struct {
	CSVPath       string
	CSVContent    []byte
	ParsedRows    int
	InsertedRows  int64
	DuplicateRows int
}

func NewWechatTradeBillService() *WechatTradeBillService {
	return &WechatTradeBillService{
		httpClient: &http.Client{},
		host:       wechatPayAPIHost,
	}
}

// SetHTTPClient 注入自定义 HTTP 客户端，便于测试或复用已有客户端配置。
func (s *WechatTradeBillService) SetHTTPClient(client *http.Client) {
	if client != nil {
		s.httpClient = client
	}
}

// SetHost 设置微信账单接口 Host，默认使用正式环境域名。
func (s *WechatTradeBillService) SetHost(host string) {
	if host != "" {
		s.host = host
	}
}

// ensureHTTPClient 确保 service 在实际发请求前有可用的 HTTP 客户端。
func (s *WechatTradeBillService) ensureHTTPClient() {
	if s.httpClient == nil {
		s.httpClient = &http.Client{}
	}
}

// GetTradeBill 向微信支付申请交易账单下载地址，并校验微信回包签名。
func (s *WechatTradeBillService) GetTradeBill(config *wxpay_utility.MchConfig, request *GetTradeBillRequest) (*QueryBillEntity, error) {
	if s == nil {
		return nil, errors.New("wechat trade bill service is nil")
	}
	if config == nil {
		return nil, errors.New("wechat merchant config is nil")
	}
	if request == nil {
		return nil, errors.New("trade bill request is nil")
	}
	if request.BillDate == nil || *request.BillDate == "" {
		return nil, errors.New("bill_date is required")
	}

	s.ensureHTTPClient()
	if s.host == "" {
		s.host = wechatPayAPIHost
	}

	const (
		method = http.MethodGet
		path   = "/v3/bill/tradebill"
	)

	reqURL, err := url.Parse(fmt.Sprintf("%s%s", s.host, path))
	if err != nil {
		return nil, err
	}
	query := reqURL.Query()
	query.Add("bill_date", *request.BillDate)
	if request.BillType != nil {
		query.Add("bill_type", string(*request.BillType))
	}
	if request.TarType != nil {
		query.Add("tar_type", string(*request.TarType))
	}
	reqURL.RawQuery = query.Encode()

	httpRequest, err := http.NewRequest(method, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Accept", "application/json")

	authorization, err := wxpay_utility.BuildAuthorization(
		config.MchId(),
		config.CertificateSerialNo(),
		config.PrivateKey(),
		method,
		reqURL.RequestURI(),
		nil,
	)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Authorization", authorization)

	httpResponse, err := s.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()

	respBody, err := wxpay_utility.ExtractResponseBody(httpResponse)
	if err != nil {
		return nil, err
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, wxpay_utility.NewAPIException(httpResponse.StatusCode, httpResponse.Header, respBody)
	}

	if config.UsesWechatPayPublicKey() {
		if err := wxpay_utility.ValidateResponse(config.WechatPayPublicKeyId(), config.WechatPayPublicKey(), &httpResponse.Header, respBody); err != nil {
			return nil, err
		}
	} else {
		if err := wxpay_utility.ValidateResponseWithPlatformCertificate(config, &httpResponse.Header, respBody); err != nil {
			return nil, err
		}
	}

	response := &QueryBillEntity{}
	if err := common.Unmarshal(respBody, response); err != nil {
		return nil, err
	}
	return response, nil
}

// buildDownloadCanonicalURL 从下载地址中提取签名时需要的 path + query 部分。
func (s *WechatTradeBillService) buildDownloadCanonicalURL(downloadURL string) (string, error) {
	uri, err := url.Parse(downloadURL)
	if err != nil {
		return "", err
	}
	if uri.RawPath != "" && uri.RawQuery != "" {
		return uri.RawPath + "?" + uri.RawQuery, nil
	}
	if uri.RawPath != "" {
		return uri.RawPath, nil
	}
	if uri.Path != "" && uri.RawQuery != "" {
		return uri.Path + "?" + uri.RawQuery, nil
	}
	return uri.Path, nil
}

// DownloadTradeBillFile 使用商户私钥重新签名请求，下载微信账单原始文件内容。
func (s *WechatTradeBillService) DownloadTradeBillFile(config *wxpay_utility.MchConfig, downloadURL string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("wechat trade bill service is nil")
	}
	if config == nil {
		return nil, errors.New("wechat merchant config is nil")
	}
	if downloadURL == "" {
		return nil, errors.New("download url is empty")
	}

	s.ensureHTTPClient()

	canonicalURL, err := s.buildDownloadCanonicalURL(downloadURL)
	if err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Accept", "application/json")
	if config.WechatPayPublicKeyId() != "" {
		httpRequest.Header.Set("Wechatpay-Serial", config.WechatPayPublicKeyId())
	}
	authorization, err := wxpay_utility.BuildAuthorization(
		config.MchId(),
		config.CertificateSerialNo(),
		config.PrivateKey(),
		http.MethodGet,
		canonicalURL,
		nil,
	)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Authorization", authorization)

	httpResponse, err := s.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		respBody, _ := io.ReadAll(httpResponse.Body)
		return nil, fmt.Errorf("download trade bill failed: status=%d body=%s", httpResponse.StatusCode, string(respBody))
	}

	return io.ReadAll(httpResponse.Body)
}

// VerifyTradeBillHash 对下载后的账单内容做哈希校验，确保文件未被篡改。
func (s *WechatTradeBillService) VerifyTradeBillHash(hashType HashType, expectedHash string, content []byte) error {
	if expectedHash == "" {
		return errors.New("hash value is empty")
	}

	var (
		actual string
		err    error
	)
	reader := bytes.NewReader(content)

	switch hashType {
	case HASHTYPE_SHA1:
		actual, err = wxpay_utility.GenerateSHA1FromStream(reader)
	case HASHTYPE_SHA256:
		actual, err = wxpay_utility.GenerateSHA256FromStream(reader)
	case HASHTYPE_SM3:
		actual, err = wxpay_utility.GenerateSM3FromStream(reader)
	default:
		return fmt.Errorf("unsupported hash type: %s", hashType)
	}
	if err != nil {
		return err
	}
	if actual != expectedHash {
		return fmt.Errorf("trade bill hash mismatch: expected %s, got %s", expectedHash, actual)
	}
	return nil
}

// DownloadAndVerifyTradeBill 完成账单下载、解压和哈希校验三个步骤。
func (s *WechatTradeBillService) DownloadAndVerifyTradeBill(config *wxpay_utility.MchConfig, tarType *TarType, bill *QueryBillEntity) (*DownloadedTradeBill, error) {
	if bill == nil {
		return nil, errors.New("trade bill response is nil")
	}
	if bill.DownloadUrl == nil || *bill.DownloadUrl == "" {
		return nil, errors.New("download_url is empty")
	}
	if bill.HashType == nil {
		return nil, errors.New("hash_type is empty")
	}
	if bill.HashValue == nil || *bill.HashValue == "" {
		return nil, errors.New("hash_value is empty")
	}

	content, err := s.DownloadTradeBillFile(config, *bill.DownloadUrl)
	if err != nil {
		return nil, err
	}
	verifiedBytes, err := s.ExtractTradeBillCSV(content, tarType)
	if err != nil {
		return nil, err
	}
	if err := s.VerifyTradeBillHash(*bill.HashType, *bill.HashValue, verifiedBytes); err != nil {
		return nil, err
	}

	return &DownloadedTradeBill{
		Content:       content,
		VerifiedBytes: verifiedBytes,
		HashType:      *bill.HashType,
		HashValue:     *bill.HashValue,
	}, nil
}

// ExtractTradeBillCSV 如果账单是 GZIP 压缩格式，则自动解压为 CSV 字节流。
func (s *WechatTradeBillService) ExtractTradeBillCSV(content []byte, tarType *TarType) ([]byte, error) {
	if len(content) == 0 {
		return nil, errors.New("trade bill content is empty")
	}
	if tarType == nil || *tarType != TARTYPE_GZIP {
		return content, nil
	}

	reader, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// SaveTradeBillCSV 将校验通过后的 CSV 文件落到本地 docs 目录，便于后续追溯。
func (s *WechatTradeBillService) SaveTradeBillCSV(billDate string, csvContent []byte) (string, error) {
	if len(csvContent) == 0 {
		return "", errors.New("csv content is empty")
	}
	if err := os.MkdirAll(wechatTradeBillDocDir, 0o755); err != nil {
		return "", err
	}

	datePart := strings.TrimSpace(billDate)
	if datePart == "" {
		datePart = "unknown-date"
	}
	filename := fmt.Sprintf("wechat_trade_bill_%s_%d.csv", datePart, common.GetTimestamp())
	filePath := filepath.Join(wechatTradeBillDocDir, filename)
	if err := os.WriteFile(filePath, csvContent, 0o644); err != nil {
		return "", err
	}
	return filePath, nil
}

// normalizeTradeBillText 统一处理 CSV 单元格中的空白、BOM 和反引号包装。
func normalizeTradeBillText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.Trim(s, "`")
	return strings.TrimSpace(s)
}

// canonicalTradeBillHeader 对表头做规范化，避免空格或制表符导致匹配失败。
func canonicalTradeBillHeader(s string) string {
	s = normalizeTradeBillText(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
}

// isTradeBillSummaryRow 判断当前行是否为微信账单尾部的汇总行，汇总行不入库。
func isTradeBillSummaryRow(record []string) bool {
	if len(record) == 0 {
		return true
	}
	if len(record) < 10 {
		return true
	}
	first := canonicalTradeBillHeader(record[0])
	if first == "" {
		return true
	}
	return strings.Contains(first, "总交易单数") ||
		strings.Contains(first, "总交易额") ||
		strings.Contains(first, "总退款金额") ||
		strings.Contains(first, "总手续费")
}

// hashTradeBillRow 对单行账单生成稳定哈希，作为去重依据。
func hashTradeBillRow(record []string) string {
	normalized := make([]string, 0, len(record))
	for _, field := range record {
		normalized = append(normalized, normalizeTradeBillText(field))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\x1f")))
	return fmt.Sprintf("%x", sum[:])
}

// mapTradeBillField 将微信账单中文表头映射到数据库模型字段。
func mapTradeBillField(row *model.PaymentBillRecord, header string, value string) {
	switch canonicalTradeBillHeader(header) {
	case "交易时间":
		row.TradeTime = value
	case "公众账号ID":
		row.AppID = value
	case "商户号":
		row.MchID = value
	case "子商户号":
		row.SubMchID = value
	case "设备号":
		row.DeviceID = value
	case "微信订单号":
		row.ChannelTradeNo = value
	case "商户订单号":
		row.MerchantTradeNo = value
	case "用户标识":
		row.PayerID = value
	case "交易类型":
		row.TradeType = value
	case "交易状态":
		row.TradeStatus = value
	case "付款银行":
		row.Bank = value
	case "货币种类":
		row.Currency = value
	case "总金额":
		row.TotalAmount = value
	case "企业红包金额":
		row.EnterpriseRedPacket = value
	case "微信退款单号":
		row.ChannelRefundNo = value
	case "商户退款单号":
		row.MerchantRefundNo = value
	case "退款金额":
		row.RefundAmount = value
	case "企业红包退款金额":
		row.EnterpriseRefund = value
	case "退款类型":
		row.RefundType = value
	case "退款状态":
		row.RefundStatus = value
	case "商品名称":
		row.GoodsName = value
	case "商户数据包":
		row.PackageData = value
	case "手续费":
		row.ServiceFee = value
	case "费率":
		row.Rate = value
	case "订单金额":
		row.OrderAmount = value
	case "申请退款金额":
		row.ApplyRefundAmount = value
	case "费率备注":
		row.RateRemark = value
	}
}

// ParseTradeBillCSV 解析微信交易账单 CSV，过滤空行和汇总行，并转成数据库模型。
func (s *WechatTradeBillService) ParseTradeBillCSV(billDate string, filePath string, csvContent []byte) ([]*model.PaymentBillRecord, error) {
	reader := csv.NewReader(bytes.NewReader(csvContent))
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("trade bill csv is empty")
	}

	var headers []string
	rows := make([]*model.PaymentBillRecord, 0, len(records))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
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

		if headers == nil {
			headers = make([]string, len(record))
			copy(headers, record)
			continue
		}

		if isTradeBillSummaryRow(record) {
			continue
		}

		rawFields := make(map[string]string, len(record))
		row := &model.PaymentBillRecord{
			ChannelType: model.PaymentChannelTypeWechat,
			BillDate:    billDate,
			FilePath:    filePath,
			RowIndex:    len(rows) + 1,
			RowHash:     hashTradeBillRow(record),
			RawLine:     strings.Join(record, ","),
		}
		for i, value := range record {
			header := fmt.Sprintf("column_%d", i+1)
			if i < len(headers) {
				header = normalizeTradeBillText(headers[i])
			}
			cleanValue := normalizeTradeBillText(value)
			rawFields[header] = cleanValue
			mapTradeBillField(row, header, cleanValue)
		}

		fieldsJSON, err := common.Marshal(rawFields)
		if err != nil {
			return nil, err
		}
		row.RawDataJSON = string(fieldsJSON)
		rows = append(rows, row)
	}

	return rows, nil
}

// ImportTradeBillRows 将解析后的账单行批量写入数据库，重复行按 row_hash 去重。
func (s *WechatTradeBillService) ImportTradeBillRows(rows []*model.PaymentBillRecord) (int64, error) {
	return model.BatchInsertPaymentBillRecords(rows)
}

// DownloadExtractSaveAndImportTradeBill 执行账单导入全流程：
// 下载账单 -> 解压校验 -> 保存本地 CSV -> 解析 -> 批量入库。
func (s *WechatTradeBillService) DownloadExtractSaveAndImportTradeBill(config *wxpay_utility.MchConfig, billDate string, tarType *TarType, bill *QueryBillEntity) (*ImportedTradeBill, error) {
	downloaded, err := s.DownloadAndVerifyTradeBill(config, tarType, bill)
	if err != nil {
		return nil, err
	}
	csvContent := downloaded.VerifiedBytes
	csvPath, err := s.SaveTradeBillCSV(billDate, csvContent)
	if err != nil {
		return nil, err
	}
	rows, err := s.ParseTradeBillCSV(billDate, csvPath, csvContent)
	if err != nil {
		return nil, err
	}
	inserted, err := s.ImportTradeBillRows(rows)
	if err != nil {
		return nil, err
	}
	return &ImportedTradeBill{
		CSVPath:       csvPath,
		CSVContent:    csvContent,
		ParsedRows:    len(rows),
		InsertedRows:  inserted,
		DuplicateRows: len(rows) - int(inserted),
	}, nil
}

// GetTradeBill 提供无状态的账单申请入口。
func GetTradeBill(config *wxpay_utility.MchConfig, request *GetTradeBillRequest) (*QueryBillEntity, error) {
	return NewWechatTradeBillService().GetTradeBill(config, request)
}

// DownloadTradeBillFile 提供无状态的账单文件下载入口。
func DownloadTradeBillFile(config *wxpay_utility.MchConfig, downloadURL string) ([]byte, error) {
	return NewWechatTradeBillService().DownloadTradeBillFile(config, downloadURL)
}

// VerifyTradeBillHash 提供无状态的账单哈希校验入口。
func VerifyTradeBillHash(hashType HashType, expectedHash string, content []byte) error {
	return NewWechatTradeBillService().VerifyTradeBillHash(hashType, expectedHash, content)
}

// DownloadAndVerifyTradeBill 提供无状态的账单下载与校验入口。
func DownloadAndVerifyTradeBill(config *wxpay_utility.MchConfig, tarType *TarType, bill *QueryBillEntity) (*DownloadedTradeBill, error) {
	return NewWechatTradeBillService().DownloadAndVerifyTradeBill(config, tarType, bill)
}

// DownloadExtractSaveAndImportTradeBill 提供无状态的账单导入入口。
func DownloadExtractSaveAndImportTradeBill(config *wxpay_utility.MchConfig, billDate string, tarType *TarType, bill *QueryBillEntity) (*ImportedTradeBill, error) {
	return NewWechatTradeBillService().DownloadExtractSaveAndImportTradeBill(config, billDate, tarType, bill)
}

// GetTradeBillRequest 表示向微信申请交易账单时的查询参数。
type GetTradeBillRequest struct {
	BillDate *string   `json:"bill_date,omitempty"`
	BillType *BillType `json:"bill_type,omitempty"`
	TarType  *TarType  `json:"tar_type,omitempty"`
}

func (o *GetTradeBillRequest) MarshalJSON() ([]byte, error) {
	type Alias GetTradeBillRequest
	a := &struct {
		BillDate *string   `json:"bill_date,omitempty"`
		BillType *BillType `json:"bill_type,omitempty"`
		TarType  *TarType  `json:"tar_type,omitempty"`
		*Alias
	}{
		BillDate: nil,
		BillType: nil,
		TarType:  nil,
		Alias:    (*Alias)(o),
	}
	return common.Marshal(a)
}

// QueryBillEntity 是微信“申请交易账单”接口的响应结构。
type QueryBillEntity struct {
	HashType    *HashType `json:"hash_type,omitempty"`
	HashValue   *string   `json:"hash_value,omitempty"`
	DownloadUrl *string   `json:"download_url,omitempty"`
}

type BillType string

func (e BillType) Ptr() *BillType {
	return &e
}

const (
	BILLTYPE_ALL     BillType = "ALL"
	BILLTYPE_SUCCESS BillType = "SUCCESS"
	BILLTYPE_REFUND  BillType = "REFUND"
)

type TarType string

func (e TarType) Ptr() *TarType {
	return &e
}

const (
	TARTYPE_GZIP TarType = "GZIP"
)

type HashType string

func (e HashType) Ptr() *HashType {
	return &e
}

const (
	HASHTYPE_SHA1   HashType = "SHA1"
	HASHTYPE_SHA256 HashType = "SHA256"
	HASHTYPE_SM3    HashType = "SM3"
)
