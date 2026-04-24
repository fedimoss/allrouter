package controller

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func GetSubscription(c *gin.Context) {
	var remainQuota int
	var usedQuota int
	var err error
	var token *model.Token
	var expiredTime int64
	if common.DisplayTokenStatEnabled {
		tokenId := c.GetInt("token_id")
		token, err = model.GetTokenById(tokenId)
		expiredTime = token.ExpiredTime
		remainQuota = token.RemainQuota
		usedQuota = token.UsedQuota
	} else {
		userId := c.GetInt("id")
		remainQuota, err = model.GetUserQuota(userId, false)
		usedQuota, err = model.GetUserUsedQuota(userId)
	}
	if expiredTime <= 0 {
		expiredTime = 0
	}
	if err != nil {
		openAIError := types.OpenAIError{
			Message: err.Error(),
			Type:    "upstream_error",
		}
		c.JSON(200, gin.H{
			"error": openAIError,
		})
		return
	}
	quota := remainQuota + usedQuota
	amount := float64(quota)
	// OpenAI 兼容接口中的 *_USD 字段含义保持“额度单位”对应值：
	// 我们将其解释为以“站点展示类型”为准：
	// - USD: 直接除以 QuotaPerUnit
	// - CNY: 先转 USD 再乘汇率
	// - TOKENS: 直接使用 tokens 数量
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		amount = amount / common.QuotaPerUnit * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeTokens:
		// amount 保持 tokens 数值
	default:
		amount = amount / common.QuotaPerUnit
	}
	if token != nil && token.UnlimitedQuota {
		amount = 100000000
	}
	subscription := OpenAISubscriptionResponse{
		Object:             "billing_subscription",
		HasPaymentMethod:   true,
		SoftLimitUSD:       amount,
		HardLimitUSD:       amount,
		SystemHardLimitUSD: amount,
		AccessUntil:        expiredTime,
	}
	c.JSON(200, subscription)
	return
}

func GetUsage(c *gin.Context) {
	var quota int
	var err error
	var token *model.Token
	if common.DisplayTokenStatEnabled {
		tokenId := c.GetInt("token_id")
		token, err = model.GetTokenById(tokenId)
		quota = token.UsedQuota
	} else {
		userId := c.GetInt("id")
		quota, err = model.GetUserUsedQuota(userId)
	}
	if err != nil {
		openAIError := types.OpenAIError{
			Message: err.Error(),
			Type:    "new_api_error",
		}
		c.JSON(200, gin.H{
			"error": openAIError,
		})
		return
	}
	amount := float64(quota)
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		amount = amount / common.QuotaPerUnit * operation_setting.USDExchangeRate
	case operation_setting.QuotaDisplayTypeTokens:
		// tokens 保持原值
	default:
		amount = amount / common.QuotaPerUnit
	}
	usage := OpenAIUsageResponse{
		Object:     "list",
		TotalUsage: amount * 100,
	}
	c.JSON(200, usage)
	return
}

// GetAllBill 获取所有用户账单概览
// query 参数 "period": day / week / month / year（默认 month）
func GetAllBill(c *gin.Context) {
	period := c.DefaultQuery("period", "month")

	now := time.Now()
	var currentStart, prevStart time.Time

	switch period {
	case "day":
		currentStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		prevStart = currentStart.AddDate(0, 0, -1)
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		currentStart = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		prevStart = currentStart.AddDate(0, 0, -7)
	case "year":
		currentStart = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		prevStart = time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
	default: // month
		currentStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		prevStart = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	}

	// 当前周期全平台消费额度
	currentQuota, err := model.SumAllUsedQuota(currentStart.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 上周期全平台消费额度
	prevQuota, err := model.SumAllUsedQuota(prevStart.Unix(), currentStart.Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 当前周期全平台充值金额
	paymentAmount, err := model.SumAllTopUp(currentStart.Unix(), 0, model.TopUpBizTypePayment)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 当前周期全平台获赠金额(兑换码)
	redemptionAmount, err := model.SumAllTopUp(currentStart.Unix(), 0, model.TopUpBizTypeRedemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 当前周期净变动 = 充值 + 获赠 - 消费
	netChange := paymentAmount + redemptionAmount - int64(currentQuota)

	// 币种转换：内部额度 ÷ QuotaPerUnit → 美元 → 按汇率转换为本地币种
	displayInfo := getDisplayCurrencyForUser(c)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"expense":        convertUsdToDisplay(float64(currentQuota)/common.QuotaPerUnit, displayInfo),
			"expense_trend":  calcPercentChange(currentQuota, prevQuota),
			"topup":          convertUsdToDisplay(float64(paymentAmount)/common.QuotaPerUnit, displayInfo),
			"bonus":          convertUsdToDisplay(float64(redemptionAmount)/common.QuotaPerUnit, displayInfo),
			"net_change":     convertUsdToDisplay(float64(netChange)/common.QuotaPerUnit, displayInfo),
			"display_symbol": displayInfo.Symbol,
		},
	})
}

// GetSelfBill 获取用户账单概览
// query 参数 "period": day / week / month / year（默认 month）
func GetSelfBill(c *gin.Context) {
	userId := c.GetInt("id")                    // 用户ID
	period := c.DefaultQuery("period", "month") // 时间周期

	now := time.Now()
	var currentStart, prevStart time.Time // 当前周期起止时间

	switch period {
	case "day":
		currentStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		prevStart = currentStart.AddDate(0, 0, -1)
	case "week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // 周日归到本周
		}
		currentStart = time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
		prevStart = currentStart.AddDate(0, 0, -7)
	case "year":
		currentStart = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		prevStart = time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, now.Location())
	default: // month
		currentStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		prevStart = time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, now.Location())
	}

	// 当前周期消费额度
	currentQuota, err := model.SumUsedQuotaByUserId(userId, currentStart.Unix(), 0)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 上周期消费额度
	prevQuota, err := model.SumUsedQuotaByUserId(userId, prevStart.Unix(), currentStart.Unix())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 当前周期充值金额
	paymentAmount, err := model.SumTopUpByUserId(userId, currentStart.Unix(), 0, model.TopUpBizTypePayment)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 当前周期获赠金额(兑换码)
	redemptionAmount, err := model.SumTopUpByUserId(userId, currentStart.Unix(), 0, model.TopUpBizTypeRedemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 邀请好友奖励额度
	// inviteCount, err := model.CountInviteRewardsByUserId(userId, currentStart.Unix(), 0)
	// if err != nil {
	// 	common.ApiError(c, err)
	// 	return
	// }
	// inviteQuota := inviteCount * int64(common.QuotaForInviter)

	// 当前周期净变动 = 充值(money,美元) + 获赠(兑换码,money,美元) - 消费(内部额度→美元)
	currentQuotaUsd := float64(currentQuota) / common.QuotaPerUnit
	netChange := paymentAmount + redemptionAmount - currentQuotaUsd

	// paymentAmount / redemptionAmount 已是美元，直接转展示币种
	displayInfo := getDisplayCurrencyForUser(c)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"expense":        convertUsdToDisplay(currentQuotaUsd, displayInfo),
			"expense_trend":  calcPercentChange(currentQuota, prevQuota),
			"topup":          convertUsdToDisplay(paymentAmount, displayInfo),
			"bonus":          convertUsdToDisplay(redemptionAmount, displayInfo),
			"net_change":     convertUsdToDisplay(netChange, displayInfo),
			"display_symbol": displayInfo.Symbol,
		},
	})
}

// calcPercentChange 计算环比变化百分比
func calcPercentChange(current, previous int) string {
	if previous == 0 {
		if current > 0 {
			return "+100%"
		}
		return "+0%"
	}
	ratio := float64(current-previous) / float64(previous) * 100
	if ratio >= 0 {
		return fmt.Sprintf("+%.2f%%", ratio)
	}
	return fmt.Sprintf("%.2f%%", ratio)
}

// GetDistributorBill 获取分销商账单概览
func GetDistributorBill(c *gin.Context) {
	// userId := c.GetInt("id")                    // 用户ID
	// period := c.DefaultQuery("period", "month") // 时间周期

	// 待定

}
