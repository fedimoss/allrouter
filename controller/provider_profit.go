package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetProviderProfits(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	providerUserId, _ := strconv.Atoi(c.Query("provider_user_id"))
	modelName := c.Query("model_name")
	requestId := c.Query("request_id")

	records, total, summary, err := model.GetProviderProfits(provider.Id, startTimestamp, endTimestamp, providerUserId, modelName, requestId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
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
