package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/pkg/wxpay_utility"
)

func TestWechatTradeBill(t *testing.T) {
	config, err := wxpay_utility.CreateMchConfig(
		"1666798226",
		"19A8C175995982710E46B1B8C0E6E8225ED5448A",
		"F:\\cert\\1666798226_20240129_cert\\apiclient_key.pem", ///data/geo/geo_sourcecode/cert/1666798226_20240129_cert/wechatpay_21F15DB4A01786411777E5861D594E2F1D218359.pem
		"21F15DB4A01786411777E5861D594E2F1D218359",
		"F:\\cert\\1666798226_20240129_cert\\wechatpay_21F15DB4A01786411777E5861D594E2F1D218359.pem",
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	request := &GetTradeBillRequest{
		BillDate: wxpay_utility.String("2026-04-07"),
		BillType: BILLTYPE_ALL.Ptr(),
		TarType:  TARTYPE_GZIP.Ptr(),
	}

	response, err := GetTradeBill(config, request)
	if err != nil {
		fmt.Printf("请求失败: %+v\n", err)
		return
	}

	fmt.Printf("请求成功: hash_type=%v, hash_value=%v, download_url=%v\n",
		*response.HashType,
		*response.HashValue,
		*response.DownloadUrl,
	)

	billFile, err := DownloadAndVerifyTradeBill(config, request.TarType, response)
	if err != nil {
		fmt.Printf("下载并校验账单失败: %+v\n", err)
		return
	}

	fmt.Printf("账单下载并校验成功: raw_bytes=%d, verified_bytes=%d, hash_type=%s, hash_value=%s\n",
		len(billFile.Content),
		len(billFile.VerifiedBytes),
		billFile.HashType,
		billFile.HashValue,
	)

	imported, err := DownloadExtractSaveAndImportTradeBill(config, *request.BillDate, request.TarType, response)
	if err != nil {
		fmt.Printf("解压、保存并入库失败: %+v\n", err)
		return
	}

	fmt.Printf("账单入库成功: csv_path=%s, parsed_rows=%d, inserted_rows=%d, duplicate_rows=%d\n",
		imported.CSVPath,
		imported.ParsedRows,
		imported.InsertedRows,
		imported.DuplicateRows,
	)
}
