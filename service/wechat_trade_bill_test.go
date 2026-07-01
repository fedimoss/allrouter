package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/pkg/wxpay_utility"
)

const (
	// Fill these values locally when you want to call the real WeChat trade bill API.
	wechatMchID          = ""
	wechatMchSerialNo    = ""
	wechatPrivateKeyPath = ""

	// Public-key mode.
	wechatPublicKeyID   = ""
	wechatPublicKeyPath = ""

	// Certificate mode.
	wechatPlatformCertSerialNo = ""
	wechatPlatformCertPath     = ""

	// Leave empty to use yesterday.
	wechatBillDate = "2026-06-23"
)

func testBillDate() string {
	if strings.TrimSpace(wechatBillDate) != "" {
		return strings.TrimSpace(wechatBillDate)
	}
	return time.Now().AddDate(0, 0, -1).Format("2006-01-02")
}

func requireHardcodedWechatConfig(t *testing.T, pairs map[string]string) {
	t.Helper()
	for name, value := range pairs {
		if strings.TrimSpace(value) == "" {
			t.Skipf("%s is empty; fill the constants at the top of service/wechat_trade_bill_test.go to call the real WeChat API", name)
		}
	}
}

func requestWechatTradeBillForTest(t *testing.T, config *wxpay_utility.MchConfig) *QueryBillEntity {
	t.Helper()
	request := &GetTradeBillRequest{
		BillDate: wxpay_utility.String(testBillDate()),
		BillType: BILLTYPE_ALL.Ptr(),
		TarType:  TARTYPE_GZIP.Ptr(),
	}

	response, err := GetTradeBill(config, request)
	if err != nil {
		t.Fatalf("GetTradeBill failed: %+v", err)
	}
	if response == nil {
		t.Fatal("GetTradeBill returned nil response")
	}
	if response.DownloadUrl == nil || strings.TrimSpace(*response.DownloadUrl) == "" {
		t.Fatalf("download_url is empty: %+v", response)
	}
	if response.HashValue == nil || strings.TrimSpace(*response.HashValue) == "" {
		t.Fatalf("hash_value is empty: %+v", response)
	}
	t.Logf("trade bill ok: bill_date=%s hash_type=%v download_url=%s", *request.BillDate, response.HashType, *response.DownloadUrl)
	return response
}

func TestWechatTradeBillPublicKeyIntegration(t *testing.T) {
	requireHardcodedWechatConfig(t, map[string]string{
		"wechatMchID":          wechatMchID,
		"wechatMchSerialNo":    wechatMchSerialNo,
		"wechatPrivateKeyPath": wechatPrivateKeyPath,
		"wechatPublicKeyID":    wechatPublicKeyID,
		"wechatPublicKeyPath":  wechatPublicKeyPath,
	})

	config, err := wxpay_utility.CreateMchConfigWithWechatPayPublicKey(
		wechatMchID,
		wechatMchSerialNo,
		wechatPrivateKeyPath,
		wechatPublicKeyID,
		wechatPublicKeyPath,
	)
	if err != nil {
		t.Fatalf("create public-key config failed: %v", err)
	}

	requestWechatTradeBillForTest(t, config)
}

func TestWechatTradeBillCertificateIntegration(t *testing.T) {
	requireHardcodedWechatConfig(t, map[string]string{
		"wechatMchID":                wechatMchID,
		"wechatMchSerialNo":          wechatMchSerialNo,
		"wechatPrivateKeyPath":       wechatPrivateKeyPath,
		"wechatPlatformCertSerialNo": wechatPlatformCertSerialNo,
		"wechatPlatformCertPath":     wechatPlatformCertPath,
	})

	config, err := wxpay_utility.CreateMchConfig(
		wechatMchID,
		wechatMchSerialNo,
		wechatPrivateKeyPath,
		wechatPlatformCertSerialNo,
		wechatPlatformCertPath,
	)
	if err != nil {
		t.Fatalf("create certificate config failed: %v", err)
	}

	requestWechatTradeBillForTest(t, config)
}
