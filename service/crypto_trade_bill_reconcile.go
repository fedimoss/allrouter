package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const cryptoBillChannelType = "crypto"

// CryptoBillRunResult Crypto 账单任务执行结果
type CryptoBillRunResult struct {
	BillDate     string `json:"bill_date"`
	InsertedRows int    `json:"inserted_rows"`
	Message      string `json:"message,omitempty"`
}

// getCryptoBillTaskTriggerTimeUTC 读取加密货币对账定时任务触发时间。
func getCryptoBillTaskTriggerTimeUTC() (int, int, bool) {
	timeText := strings.TrimSpace(os.Getenv(cryptoBillTaskTimeEnv))
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

// RunCryptoBillWorkflow 从区块链上拉取指定日期的交易记录 → 入库 payment_bill_record
// 流程：
//  1. 加载所有 crypto_chain_config 链配置
//  2. 对每条链，通过 eth_getLogs 查询指定日期内到收款地址的 Transfer 事件
//  3. 将链上事件转为 PaymentBillRecord 并批量入库
func RunCryptoBillWorkflow(billDate string) (*CryptoBillRunResult, error) {
	date, err := time.Parse("2006-01-02", billDate)
	if err != nil {
		return nil, fmt.Errorf("日期格式错误: %s", billDate)
	}
	startTime := date.Unix()
	endTime := date.AddDate(0, 0, 1).Unix()

	// 加载所有链配置
	var chains []model.CryptoChainConfig
	if err := model.DB.Order("network, token_symbol").Find(&chains).Error; err != nil {
		return nil, fmt.Errorf("加载链配置失败: %w", err)
	}
	if len(chains) == 0 {
		return &CryptoBillRunResult{BillDate: billDate, Message: "无链配置"}, nil
	}

	var batch []*model.PaymentBillRecord
	totalInserted := 0
	rowIndex := 0

	for i := range chains {
		chain := &chains[i]
		if strings.TrimSpace(chain.ReceiverAddress) == "" || strings.TrimSpace(chain.RPCURL) == "" {
			continue
		}

		// 通过链上日志查询该日期范围内的 Transfer 事件
		events, err := fetchChainTransferLogs(chain, startTime, endTime)
		if err != nil {
			common.SysLog(fmt.Sprintf("crypto bill: failed to fetch logs for chain=%s: %v", chain.Network, err))
			continue
		}

		common.SysLog(fmt.Sprintf("crypto bill: chain=%s date=%s found=%d events", chain.Network, billDate, len(events)))

		for _, ev := range events {
			record := buildCryptoBillRecordFromChain(billDate, rowIndex, chain, ev)
			batch = append(batch, record)
			rowIndex++

			if len(batch) >= cryptoBillBatchSize {
				n, err := model.BatchInsertPaymentBillRecords(batch)
				if err != nil {
					return nil, fmt.Errorf("批量入库失败: %w", err)
				}
				totalInserted += int(n)
				batch = batch[:0]
			}
		}
	}

	if len(batch) > 0 {
		n, err := model.BatchInsertPaymentBillRecords(batch)
		if err != nil {
			return nil, fmt.Errorf("批量入库失败(尾部): %w", err)
		}
		totalInserted += int(n)
	}

	return &CryptoBillRunResult{
		BillDate:     billDate,
		InsertedRows: totalInserted,
	}, nil
}

// ---- 链上交易查询（BscScan/Etherscan API） ----

// explorerAPITokenTxResponse BscScan/Etherscan tokentx 接口响应
type explorerAPITokenTxResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

type explorerAPITokenTxItem struct {
	BlockNumber     string `json:"blockNumber"`
	TimeStamp       string `json:"timeStamp"`
	Hash            string `json:"hash"`
	From            string `json:"from"`
	To              string `json:"to"`
	Value           string `json:"value"`
	TokenName       string `json:"tokenName"`
	TokenSymbol     string `json:"tokenSymbol"`
	TokenDecimal    string `json:"tokenDecimal"`
	ContractAddress string `json:"contractAddress"`
}

// chainTransferEvent 链上 Transfer 事件（内部统一格式）
type chainTransferEvent struct {
	TxHash        string   // 交易哈希
	BlockNumber   uint64   // 区块号
	From          string   // 转出地址（payer）
	To            string   // 收款地址
	Value         *big.Int // 转账金额（最小单位）
	BlockTime     int64    // 区块时间戳
	TokenContract string   // 代币合约地址
}

// explorerAPIURL 根据链 ID 返回对应的 Explorer API 基础 URL
func explorerAPIURL(chainID int) string {
	switch chainID {
	case 56: // BSC Mainnet
		return "https://api.bscscan.com/api"
	case 97: // BSC Testnet
		return "https://api-testnet.bscscan.com/api"
	case 11155111: // Sepolia
		return "https://api-sepolia.etherscan.io/v2/api"
	case 137: // Polygon
		return "https://api.polygonscan.com/api"
	default:
		return ""
	}
}

// fetchChainTransferLogs 通过 Explorer API 查询指定日期范围内到收款地址的代币转账记录
func fetchChainTransferLogs(chain *model.CryptoChainConfig, startTime, endTime int64) ([]chainTransferEvent, error) {
	apiURL := explorerAPIURL(chain.ChainID)
	if apiURL == "" {
		return nil, fmt.Errorf("不支持的链 ID: %d", chain.ChainID)
	}

	// 查询收款地址的代币转账记录（按合约过滤，最多 10000 条）
	reqURL := fmt.Sprintf("%s?module=account&action=tokentx&address=%s&contractaddress=%s&startblock=0&endblock=99999999&page=1&offset=10000&sort=asc&apikey=%s",
		apiURL, chain.ReceiverAddress, chain.TokenContract, explorerAPIKey())

	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Explorer API 请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var apiResp explorerAPITokenTxResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("Explorer API 响应解析失败: %w", err)
	}
	if apiResp.Status != "1" {
		// result 可能是字符串错误信息
		var errMsg string
		json.Unmarshal(apiResp.Result, &errMsg)
		return nil, fmt.Errorf("Explorer API 返回错误(%s): %s", apiResp.Message, errMsg)
	}

	var items []explorerAPITokenTxItem
	if err := json.Unmarshal(apiResp.Result, &items); err != nil {
		return nil, fmt.Errorf("Explorer API result 解析失败: %w", err)
	}

	// 过滤：时间在账单日期范围内 + 转入（to = 收款地址）
	var events []chainTransferEvent
	for _, item := range items {
		ts, _ := strconv.ParseInt(item.TimeStamp, 10, 64)
		if ts < startTime || ts >= endTime {
			continue
		}
		if !strings.EqualFold(item.To, chain.ReceiverAddress) {
			continue
		}
		value := new(big.Int)
		value.SetString(item.Value, 10)
		bn, _ := strconv.ParseUint(item.BlockNumber, 10, 64)

		events = append(events, chainTransferEvent{
			TxHash:        item.Hash,
			BlockNumber:   bn,
			From:          item.From,
			To:            item.To,
			Value:         value,
			BlockTime:     ts,
			TokenContract: item.ContractAddress,
		})
	}
	return events, nil
}

func explorerAPIKey() string {
	// TODO: 后续改为从 options 表读取 "CryptoExplorerAPIKey"
	return "xxxxxxxx"
}

// ---- PaymentBillRecord 构造 ----

func buildCryptoBillRecordFromChain(billDate string, rowIndex int, chain *model.CryptoChainConfig, ev chainTransferEvent) *model.PaymentBillRecord {
	tradeTime := time.Unix(ev.BlockTime, 0).In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
	valueStr := formatTokenAmount(ev.Value, chain.TokenDecimals)

	rowHashRaw := fmt.Sprintf("%s|%s|%s|%s|%d", cryptoBillChannelType, chain.Network, ev.TxHash, billDate, rowIndex)
	hash := sha256.Sum256([]byte(rowHashRaw))

	return &model.PaymentBillRecord{
		ChannelType:     cryptoBillChannelType,
		BillDate:        billDate,
		RowIndex:        rowIndex,
		RowHash:         hex.EncodeToString(hash[:]),
		TradeTime:       tradeTime,
		ChannelTradeNo:  ev.TxHash,     // 链上交易哈希
		MerchantTradeNo: "",            // 对账时匹配本地订单
		PayerID:         ev.From,       // 付款方地址
		TradeType:       chain.Network, // 链网络标识
		TradeStatus:     "success",
		Currency:        chain.TokenSymbol,
		TotalAmount:     valueStr,
		OrderAmount:     valueStr,
		CreatedAt:       common.GetTimestamp(),
	}
}

// formatTokenAmount 将链上最小单位的金额转为代币数量字符串
func formatTokenAmount(value *big.Int, decimals int) string {
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	quotient := new(big.Int).Div(value, divisor)
	remainder := new(big.Int).Mod(value, divisor)
	return fmt.Sprintf("%s.%0*s", quotient.String(), decimals, remainder.String())
}
