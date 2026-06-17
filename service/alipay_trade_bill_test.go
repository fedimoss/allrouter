package service

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestAlipayTradeBillServiceDownloadBillZipRejectsInvalidURL(t *testing.T) {
	svc := &AlipayTradeBillService{httpClient: &http.Client{}}

	_, err := svc.DownloadBillZip(context.Background(), "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "download URL is empty")

	_, err = svc.DownloadBillZip(context.Background(), "ftp://example.com/bill.zip")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid alipay bill download URL scheme")

	_, err = svc.DownloadBillZip(context.Background(), strings.Repeat(":", 2))
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid alipay bill download URL")
}

// buildTestZip 构造一个内存 zip，包含 files 映射的 <条目名->内容> 文件。
func buildTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestExtractBillZip(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "alipay_trade_bill_2026-06-05_1781597943")
	zipBytes := buildTestZip(t, map[string]string{
		"业务账单明细.csv": "col1,col2\nv1,v2\n",
		"业务账单汇总.csv": "summary\n",
	})

	count, size, err := ExtractBillZip(zipBytes, destDir)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, int64(len("col1,col2\nv1,v2\n")+len("summary\n")), size)

	// 文件确实落盘且内容正确
	detail, err := os.ReadFile(filepath.Join(destDir, "业务账单明细.csv"))
	require.NoError(t, err)
	require.Equal(t, "col1,col2\nv1,v2\n", string(detail))
}

// TestExtractBillZipRejectsZipSlip 验证 zip-slip 防护：拒绝解压逃逸出目标目录的条目。
func TestExtractBillZipRejectsZipSlip(t *testing.T) {
	destDir := filepath.Join(t.TempDir(), "safe")
	zipBytes := buildTestZip(t, map[string]string{
		"../../../evil.txt": "pwned",
	})

	_, _, err := ExtractBillZip(zipBytes, destDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "escapes destination dir")

	// 防护应在写入前拦截：目标目录内不会出现逃逸文件
	_, statErr := os.Stat(filepath.Join(destDir, "evil.txt"))
	require.Error(t, statErr)
}

// TestDecodeZipEntryName 验证 GBK 文件名解码（支付宝账单 zip 的中文文件名乱码修复）。
func TestDecodeZipEntryName(t *testing.T) {
	want := "业务账单明细(汇总).csv"
	gbkBytes, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(want))
	require.NoError(t, err)

	// 未置 UTF-8 标志位 → 按 GBK 解码为 UTF-8
	got := decodeZipEntryName(string(gbkBytes), 0)
	require.Equal(t, want, got)

	// 置 UTF-8 标志位 → 原样返回（本身已是 UTF-8）
	require.Equal(t, want, decodeZipEntryName(want, 0x800))
}

// TestExtractBillZipDecodesGBKNames 端到端验证：GBK 编码文件名的 zip 解压后得到正确的 UTF-8 文件名。
// 模拟支付宝账单 zip：中文文件名用 GBK 编码且未置 UTF-8 标志位（flags bit 0x800）。
func TestExtractBillZipDecodesGBKNames(t *testing.T) {
	want := "业务账单明细(汇总).csv"
	gbkBytes, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(want))
	require.NoError(t, err)

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	fh := &zip.FileHeader{
		Name:    string(gbkBytes),
		Method:  zip.Store,
		NonUTF8: true, // 强制清零 UTF-8 标志位，模拟支付宝 GBK 中文文件名
	}
	fw, err := w.CreateHeader(fh)
	require.NoError(t, err)
	_, err = fw.Write([]byte("col1,col2\n"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	// 读回确认确实未置 UTF-8 标志位（否则测试构造无效）
	check, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.Len(t, check.File, 1)
	require.Equal(t, uint16(0), check.File[0].Flags&0x800, "测试 zip 必须未置 UTF-8 标志位")

	destDir := filepath.Join(t.TempDir(), "gbk_bill")
	count, _, err := ExtractBillZip(buf.Bytes(), destDir)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// 解压后文件名应为正确的 UTF-8 中文
	data, err := os.ReadFile(filepath.Join(destDir, want))
	require.NoError(t, err, "解压后文件名应为正确的 UTF-8 中文")
	require.Equal(t, "col1,col2\n", string(data))
}

// alipayDetailCSVSample 是一份精简的支付宝业务明细 CSV（UTF-8），结构与真实账单一致：
// 第4行为业务明细列表起始标记，第5行为表头，之后为数据行，遇结束标记停止。
func alipayDetailCSVSample() string {
	return "#支付宝业务明细查询\n" +
		"#账号：[20887226827483540156]\n" +
		"#起始日期：[2026年06月05日 00:00:00]   终止日期：[2026年06月06日 00:00:00]\n" +
		"#-----------------------------------------业务明细列表----------------------------------------\n" +
		"支付宝交易号,商户订单号,业务类型,商品名称,创建时间,完成时间,门店编号,门店名称,操作员,终端号,对方账户,订单金额（元）,商家实收（元）,支付宝红包（元）,集分宝（元）,支付宝优惠（元）,商家优惠（元）,券核销金额（元）,券名称,商家红包消费金额（元）,卡消费金额（元）,退款批次号/请求号,服务费（元）,分润（元）,备注\n" +
		"2026060522001419931441931972\t,USR1NOZM644N1780646479\t,交易\t,TUC1,2026-06-05 16:01:34,2026-06-05 16:01:38,\t,\t,\t,\t,**博(156******89)\t,0.01,0.01,0.01,0.00,0.00,0.00,0.00,CY24下沉城市百次立减卡模板-11月新,0.00\t,0.00,\t,0.00,0.00,\n" +
		"2026060522001419931441888001\t,SUBUSR1NOVLgV0w1780646324\t,交易\t,TUC1,2026-06-05 15:00:00,2026-06-05 15:00:05,\t,\t,\t,\t,**博(156******89)\t,9.90,9.90,0.00,0.00,0.00,0.00,0.00,CY24下沉城市百次立减卡模板-11月新,0.00\t,0.00,\t,0.00,0.00,\n" +
		"2026060522001419931442000000\t,USR1NOABC123456\t,退款\t,TUC1,2026-06-05 17:00:00,2026-06-05 17:00:05,\t,\t,\t,\t,**博(156******89)\t,0.02,0.02,0.00,0.00,0.00,0.00,0.00,,0.00\t,0.00,REF20260605001\t,0.00,0.00,退款备注\n" +
		"#-----------------------------------------业务明细列表结束------------------------------------\n" +
		"#交易合计：3笔，商家实收共9.93元，商家优惠共0.00元\n"
}

// TestParseAlipayTradeBillCSV 验证支付宝业务明细 CSV 的标记定位、表头映射与状态推导。
func TestParseAlipayTradeBillCSV(t *testing.T) {
	rows, err := ParseAlipayTradeBillCSV("2026-06-05", "/tmp/detail.csv", []byte(alipayDetailCSVSample()))
	require.NoError(t, err)
	require.Len(t, rows, 3)

	// 第一行：USR 充值交易（payment/success）
	r0 := rows[0]
	require.Equal(t, model.PaymentChannelTypeAlipay, r0.ChannelType)
	require.Equal(t, "2026-06-05", r0.BillDate)
	require.Equal(t, "2026060522001419931441931972", r0.ChannelTradeNo)
	require.Equal(t, "USR1NOZM644N1780646479", r0.MerchantTradeNo)
	require.Equal(t, "payment", r0.TradeType)   // USR 前缀 → payment
	require.Equal(t, "success", r0.TradeStatus) // 业务类型"交易"→success
	require.Equal(t, "TUC1", r0.GoodsName)
	require.Equal(t, "2026-06-05 16:01:38", r0.TradeTime) // 完成时间
	require.Equal(t, "**博(156******89)", r0.PayerID)
	require.Equal(t, "0.01", r0.OrderAmount)
	require.Equal(t, "0.01", r0.TotalAmount) // 商家实收
	require.Equal(t, "0.01", r0.EnterpriseRedPacket)
	require.Equal(t, "0.00", r0.ServiceFee)
	require.NotEmpty(t, r0.RowHash)
	require.NotEmpty(t, r0.RawDataJSON)

	// 第二行：SUB 套餐订阅交易（subscription/success）
	r1 := rows[1]
	require.Equal(t, "SUBUSR1NOVLgV0w1780646324", r1.MerchantTradeNo)
	require.Equal(t, "subscription", r1.TradeType) // SUB 前缀 → subscription
	require.Equal(t, "success", r1.TradeStatus)    // 业务类型"交易"→success
	require.Equal(t, "9.90", r1.OrderAmount)

	// 第三行：USR 充值退款（payment/refund，订单类别仍按订单号前缀归类）
	r2 := rows[2]
	require.Equal(t, "USR1NOABC123456", r2.MerchantTradeNo)
	require.Equal(t, "payment", r2.TradeType)  // USR 前缀 → payment（退款行同样归类）
	require.Equal(t, "refund", r2.TradeStatus) // 业务类型"退款"→refund
	require.Equal(t, "REF20260605001", r2.ChannelRefundNo)
	require.Equal(t, "0.02", r2.OrderAmount)
	require.NotEqual(t, r0.RowHash, r2.RowHash) // 两行哈希不同
}

// TestDeriveAlipayTradeTypeFromMerchantOrderNo 验证依据商户订单号前缀推导订单类别。
func TestDeriveAlipayTradeTypeFromMerchantOrderNo(t *testing.T) {
	require.Equal(t, "subscription", deriveAlipayTradeTypeFromMerchantOrderNo("SUBUSR1NOVLgV0w1780646324"))
	require.Equal(t, "subscription", deriveAlipayTradeTypeFromMerchantOrderNo("subusr 小写也算订阅")) // 大小写不敏感
	require.Equal(t, "payment", deriveAlipayTradeTypeFromMerchantOrderNo("USR1NOZM644N1780646479"))
	require.Equal(t, "payment", deriveAlipayTradeTypeFromMerchantOrderNo(""))               // 空 → 默认 payment
	require.Equal(t, "payment", deriveAlipayTradeTypeFromMerchantOrderNo("UNKNOWN_PREFIX")) // 未知前缀 → 默认 payment
}

// TestParseAlipayTradeBillCSVEmpty 当天无交易时（只有标记行）应返回 0 行且不报错。
func TestParseAlipayTradeBillCSVEmpty(t *testing.T) {
	empty := "#支付宝业务明细查询\n" +
		"#-----------------------------------------业务明细列表----------------------------------------\n" +
		"支付宝交易号,商户订单号,业务类型,商品名称,创建时间,完成时间\n" +
		"#-----------------------------------------业务明细列表结束------------------------------------\n"
	rows, err := ParseAlipayTradeBillCSV("2026-06-05", "/tmp/detail.csv", []byte(empty))
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestDeriveAlipayTradeStatus(t *testing.T) {
	require.Equal(t, "success", deriveAlipayTradeStatus("交易"))
	require.Equal(t, "refund", deriveAlipayTradeStatus("退款"))
	require.Equal(t, "success", deriveAlipayTradeStatus(" 交易 ")) // 含空格
	require.Equal(t, "充值", deriveAlipayTradeStatus("充值"))        // 其他业务类型原样返回
}

// TestFindAlipayBillDetailCSV 验证从解压目录中选中"业务明细.csv"（排除"汇总"），并将 GBK 内容解码为 UTF-8。
func TestFindAlipayBillDetailCSV(t *testing.T) {
	folder := t.TempDir()
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(alipayDetailCSVSample()))
	require.NoError(t, err)

	// 同时写入"汇总"文件（应被跳过）与"业务明细"文件（应被选中）
	require.NoError(t, os.WriteFile(filepath.Join(folder, "2088_xxx_业务明细(汇总).csv"), gbk, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(folder, "2088_xxx_业务明细.csv"), gbk, 0o644))

	path, content, err := findAlipayBillDetailCSV(folder)
	require.NoError(t, err)
	require.Contains(t, filepath.Base(path), "业务明细")
	require.NotContains(t, filepath.Base(path), "汇总")
	// GBK 内容应被解码为 UTF-8
	require.Contains(t, string(content), "支付宝交易号")
	require.Contains(t, string(content), "业务明细列表结束")
}
