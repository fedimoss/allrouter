package controller

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type WechatTradeBillRunRequest struct {
	BillDate      string `json:"bill_date"`
	PaymentMethod string `json:"payment_method"`
}

// RunWechatTradeBill 手动触发指定日期的账单下载、入库与对账流程。
// payment_method: "wxpay"（默认，微信支付）或 "stripe"（Stripe 支付）。
func RunWechatTradeBill(c *gin.Context) {
	var req WechatTradeBillRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	billDate := strings.TrimSpace(req.BillDate)
	if billDate == "" {
		billDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	}

	var result any
	var err error

	// 执行对应支付方式的账单拉取与对账流程
	switch strings.ToLower(strings.TrimSpace(req.PaymentMethod)) {
	// Stripe 分支：先清空当天数据，再拉取对账，防止重复执行产生重复记录
	case "stripe":
		// 清空 Stripe 账单记录
		if err = model.DeletePaymentBillRecordsByChannelAndBillDate(model.PaymentChannelTypeStripe, billDate); err != nil {
			common.ApiError(c, err)
			return
		}
		// 再清空 Stripe 账单对账记录
		if err = model.DeletePaymentBillReconcilesByChannelAndBillDate(model.PaymentChannelTypeStripe, billDate); err != nil {
			common.ApiError(c, err)
			return
		}
		// 执行 Stripe 账单拉取与对账流程
		result, err = runStripeBillWorkflow(billDate)
	// wxpay 分支
	default:
		result, err = service.RunWechatTradeBillWorkflowWithDBConfig(billDate)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// GetWechatTradeBillStat 查询支付对账页顶部报表数据。
func GetWechatTradeBillStat(c *gin.Context) {
	filter := &service.WechatTradeBillListFilter{
		BillDate:        c.Query("bill_date"),
		ReconcileStatus: c.Query("reconcile_status"),
		Keyword:         c.Query("keyword"),
		PaymentMethod:   c.Query("payment_method"),
		LocalType:       c.Query("local_type"),
	}
	result, err := service.GetWechatTradeBillDashboard(filter, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// GetWechatTradeBillList 查询支付对账结果分页列表。
func GetWechatTradeBillList(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	filter := &service.WechatTradeBillListFilter{
		BillDate:        c.Query("bill_date"),
		ReconcileStatus: c.Query("reconcile_status"),
		Keyword:         c.Query("keyword"),
		PaymentMethod:   c.Query("payment_method"),
		LocalType:       c.Query("local_type"),
	}
	if p := strings.TrimSpace(c.Query("p")); p != "" {
		if page, err := strconv.Atoi(p); err == nil && page > 0 {
			pageInfo.Page = page
		}
	}
	result, err := service.GetWechatTradeBillList(pageInfo, filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// GetWechatTradeBillDetail 查询单条支付对账记录详情。
func GetWechatTradeBillDetail(c *gin.Context) {
	id, err := strconv.Atoi(strings.TrimSpace(c.Param("id")))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	result, err := service.GetWechatTradeBillDetail(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}
