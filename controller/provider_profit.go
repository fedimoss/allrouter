package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func listProviderProfits(c *gin.Context, providerId int) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	providerUserId, _ := strconv.Atoi(c.Query("provider_user_id"))
	modelName := c.Query("model_name")
	requestId := c.Query("request_id")

	records, total, summary, err := model.GetProviderProfits(providerId, startTimestamp, endTimestamp, providerUserId, modelName, requestId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, gin.H{
		"page":    pageInfo,
		"summary": summary,
	})
}

func GetProviderProfits(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	listProviderProfits(c, provider.Id)
}

func AdminGetProviderProfits(c *gin.Context) {
	id, ok := parseProviderAdminId(c)
	if !ok {
		return
	}
	listProviderProfits(c, id)
}

// AdminGetProviderProfitOverview 管理员查看所有服务商的利润汇总，
// 支持分页与时间范围过滤，无记录的服务商也会返回。
func AdminGetProviderProfitOverview(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	rows, summary, total, err := model.GetProviderProfitOverview(startTimestamp, endTimestamp, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(rows)
	common.ApiSuccess(c, gin.H{
		"page":    pageInfo,
		"summary": summary,
	})
}
