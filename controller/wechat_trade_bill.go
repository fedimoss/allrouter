package controller

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type WechatTradeBillRunRequest struct {
	BillDate string `json:"bill_date"`
}

// RunWechatTradeBill 手动触发指定日期的微信账单下载、入库与对账流程。
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

	result, err := service.RunWechatTradeBillWorkflowWithDBConfig(billDate)
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
	result, err := service.GetWechatTradeBillDashboard(filter)
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
