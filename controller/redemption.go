package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetAllRedemptions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	// 解析发放状态筛选参数（sent=已发放 / unsent=未发放 / 其他值不过滤）
	sentFilter := model.GetSentQueryFilter(c.Query("sent"))
	redemptions, total, err := model.GetAllRedemptions(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), sentFilter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	// 获取当前用户的展示币种信息
	displayInfo := getDisplayCurrencyForUser(c)
	// 序列化原始结构体为 map，再替换 quota 字段为币种转换后的值
	var items []map[string]any
	if raw, err := common.Marshal(redemptions); err == nil {
		common.Unmarshal(raw, &items)
	}
	for i, r := range redemptions {
		if i < len(items) {
			// 内部额度 ÷ QuotaPerUnit → 美元 → × 汇率 → 展示币种金额
			items[i]["quota"] = convertQuotaToDisplay(r.Quota, displayInfo)
		}
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"page":           pageInfo.Page,
			"page_size":      pageInfo.PageSize,
			"total":          pageInfo.Total,
			"items":          items,
			"display_symbol": displayInfo.Symbol,
		},
	})
}

func SearchRedemptions(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	// 发放状态筛选（sent=已发放 / unsent=未发放 / 其他值不过滤），与关键字搜索叠加
	sentFilter := model.GetSentQueryFilter(c.Query("sent"))
	redemptions, total, err := model.SearchRedemptions(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), sentFilter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetSelfRedemptionRecords(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	redemptions, total, err := model.GetUserRedeemedRedemptions(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	displayInfo := getDisplayCurrencyForUser(c)
	var items []map[string]any
	if raw, err := common.Marshal(redemptions); err == nil {
		common.Unmarshal(raw, &items)
	}
	for i, r := range redemptions {
		if i < len(items) {
			items[i]["quota"] = convertQuotaToDisplay(r.Quota, displayInfo)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"page":           pageInfo.Page,
			"page_size":      pageInfo.PageSize,
			"total":          total,
			"items":          items,
			"display_symbol": displayInfo.Symbol,
		},
	})
}

func GetRedemption(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	redemption, err := model.GetRedemptionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    redemption,
	})
	return
}

func AddRedemption(c *gin.Context) {
	//if !operation_setting.IsPaymentComplianceConfirmed() { //新版,后台合规声明校验,前端没有开发(确认前，支付、兑换码、订阅计划和邀请返利功能将保持锁定。)
	//	common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
	//	return
	//}

	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if utf8.RuneCountInString(redemption.Name) == 0 || utf8.RuneCountInString(redemption.Name) > 20 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionNameLength)
		return
	}
	if redemption.Count <= 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountPositive)
		return
	}
	if redemption.Count > 100 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountMax)
		return
	}
	if valid, msg := validateExpiredTime(c, redemption.ExpiredTime); !valid {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
		return
	}
	var keys []string
	for i := 0; i < redemption.Count; i++ {
		key := common.GetUUID()
		cleanRedemption := model.Redemption{
			ProviderId:  c.GetInt("provider_id"),
			UserId:      c.GetInt("id"),
			Name:        redemption.Name,
			Key:         key,
			CreatedTime: common.GetTimestamp(),
			Quota:       redemption.Quota,
			ExpiredTime: redemption.ExpiredTime,
		}
		err = cleanRedemption.Insert()
		if err != nil {
			common.SysError("failed to insert redemption: " + err.Error())
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgRedemptionCreateFailed),
				"data":    keys,
			})
			return
		}
		keys = append(keys, key)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    keys,
	})
	return
}

func DeleteRedemption(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	err := model.DeleteRedemptionById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func UpdateRedemption(c *gin.Context) {
	statusOnly := c.Query("status_only")
	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cleanRedemption, err := model.GetRedemptionById(redemption.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if statusOnly == "" {
		if valid, msg := validateExpiredTime(c, redemption.ExpiredTime); !valid {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
			return
		}
		// If you add more fields, please also update redemption.Update()
		cleanRedemption.Name = redemption.Name
		cleanRedemption.Quota = redemption.Quota
		cleanRedemption.ExpiredTime = redemption.ExpiredTime
	}
	if statusOnly != "" {
		cleanRedemption.Status = redemption.Status
	}
	err = cleanRedemption.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    cleanRedemption,
	})
	return
}

func DeleteInvalidRedemption(c *gin.Context) {
	rows, err := model.DeleteInvalidRedemptions()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
	return
}

// RedemptionSentBatch 批量发放标记请求体：ids 为兑换码 ID 列表，sent 为 true 标记已发放、false 取消标记；
// email 为可选邮箱，标记发放且填写了 email 时，将兑换码以邮件发送到该邮箱
type RedemptionSentBatch struct {
	Ids   []int  `json:"ids"`
	Sent  bool   `json:"sent"`
	Email string `json:"email"`
}

// UpdateRedemptionSent 管理端：批量标记/取消兑换码的发放状态，可选发送兑换码邮件
func UpdateRedemptionSent(c *gin.Context) {
	sentBatch := RedemptionSentBatch{}
	if err := c.ShouldBindJSON(&sentBatch); err != nil || len(sentBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	// 发件配置跟随当前访问域名的服务商（与登录/注册邮件一致），主站域名则用主站全局配置；
	// 兑换码数据范围仍为管理端全量，不受发件服务商影响
	mailProviderId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	rows, err := updateRedemptionSentAndNotify(c, 0, mailProviderId, &sentBatch)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

// UpdateProviderRedemptionSent 服务商端：批量标记/取消本服务商名下兑换码的发放状态，可选发送兑换码邮件
func UpdateProviderRedemptionSent(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	sentBatch := RedemptionSentBatch{}
	if err := c.ShouldBindJSON(&sentBatch); err != nil || len(sentBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	rows, err := updateRedemptionSentAndNotify(c, provider.Id, provider.Id, &sentBatch)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

// updateRedemptionSentAndNotify 批量更新发放标记；标记发放且请求携带邮箱时，
// 渲染兑换码邮件模板并通过 mailProviderId 对应的邮件配置发送到目标邮箱。
// providerId 控制兑换码数据范围（0 为管理端全量），mailProviderId 控制发件配置（0 为主站全局）。
func updateRedemptionSentAndNotify(c *gin.Context, providerId int, mailProviderId int, sentBatch *RedemptionSentBatch) (int64, error) {
	rows, err := model.BatchUpdateRedemptionSent(providerId, sentBatch.Ids, sentBatch.Sent)
	if err != nil {
		return 0, err
	}
	email := strings.TrimSpace(sentBatch.Email)
	if sentBatch.Sent && email != "" {
		if err := common.Validate.Var(email, "email"); err != nil {
			return 0, fmt.Errorf("无效的邮箱地址")
		}
		redemptions, err := model.GetRedemptionsByIds(providerId, sentBatch.Ids)
		if err != nil {
			return rows, err
		}
		if len(redemptions) == 0 {
			return rows, nil
		}
		systemName := getRequestSystemName(c)
		subject := fmt.Sprintf("%s兑换码发放 / Redemption Code", systemName)
		content, err := common.RenderEmailTemplate("redemption_sent.html", map[string]any{
			"SystemName": systemName,
			"Quota":      logger.FormatQuota(redemptions[0].Quota),
			"Key":        redemptionsKey(redemptions),
		})
		if err != nil {
			return rows, err
		}
		if err := service.SendProviderMail(mailProviderId, subject, email, content); err != nil {
			// SMTP 错误可能包含主机、端口等配置信息，仅记录在服务端日志
			logger.LogError(c.Request.Context(), fmt.Sprintf("failed to send redemption email to %s: %s", email, err.Error()))
			return 0, err
		}
	}
	return rows, nil
}

// redemptionsKey 兑换码内容：单个直接返回 key，多个按行拼接
func redemptionsKey(redemptions []*model.Redemption) string {
	keys := make([]string, 0, len(redemptions))
	for _, r := range redemptions {
		keys = append(keys, r.Key)
	}
	return strings.Join(keys, "\n")
}

func GetProviderRedemptions(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	pageInfo := common.GetPageQuery(c)
	// 发放状态筛选（sent=已发放 / unsent=未发放 / 其他值不过滤）
	sentFilter := model.GetSentQueryFilter(c.Query("sent"))
	redemptions, total, err := model.GetRedemptionsByProvider(provider.Id, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), sentFilter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	displayInfo := getDisplayCurrencyForUser(c)
	var items []map[string]any
	if raw, err := common.Marshal(redemptions); err == nil {
		common.Unmarshal(raw, &items)
	}
	for i, r := range redemptions {
		if i < len(items) {
			items[i]["quota"] = convertQuotaToDisplay(r.Quota, displayInfo)
		}
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"page":           pageInfo.Page,
			"page_size":      pageInfo.PageSize,
			"total":          pageInfo.Total,
			"items":          items,
			"display_symbol": displayInfo.Symbol,
		},
	})
}

func SearchProviderRedemptions(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	// 发放状态筛选（sent=已发放 / unsent=未发放 / 其他值不过滤），与关键字搜索叠加
	sentFilter := model.GetSentQueryFilter(c.Query("sent"))
	redemptions, total, err := model.SearchRedemptionsByProvider(provider.Id, keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), sentFilter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(redemptions)
	common.ApiSuccess(c, pageInfo)
}

func GetProviderRedemption(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	redemption, err := model.GetRedemptionByIdInProvider(id, provider.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    redemption,
	})
}

func AddProviderRedemption(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if utf8.RuneCountInString(redemption.Name) == 0 || utf8.RuneCountInString(redemption.Name) > 20 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionNameLength)
		return
	}
	if redemption.Count <= 0 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountPositive)
		return
	}
	if redemption.Count > 100 {
		common.ApiErrorI18n(c, i18n.MsgRedemptionCountMax)
		return
	}
	if valid, msg := validateExpiredTime(c, redemption.ExpiredTime); !valid {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
		return
	}
	var keys []string
	for i := 0; i < redemption.Count; i++ {
		key := common.GetUUID()
		cleanRedemption := model.Redemption{
			ProviderId:  provider.Id,
			UserId:      c.GetInt("id"),
			Name:        redemption.Name,
			Key:         key,
			CreatedTime: common.GetTimestamp(),
			Quota:       redemption.Quota,
			ExpiredTime: redemption.ExpiredTime,
		}
		err = cleanRedemption.Insert()
		if err != nil {
			common.SysError("failed to insert redemption: " + err.Error())
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": i18n.T(c, i18n.MsgRedemptionCreateFailed),
				"data":    keys,
			})
			return
		}
		keys = append(keys, key)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    keys,
	})
}

func UpdateProviderRedemption(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	statusOnly := c.Query("status_only")
	redemption := model.Redemption{}
	err := c.ShouldBindJSON(&redemption)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	cleanRedemption, err := model.GetRedemptionByIdInProvider(redemption.Id, provider.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if statusOnly == "" {
		if valid, msg := validateExpiredTime(c, redemption.ExpiredTime); !valid {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": msg})
			return
		}
		cleanRedemption.Name = redemption.Name
		cleanRedemption.Quota = redemption.Quota
		cleanRedemption.ExpiredTime = redemption.ExpiredTime
	}
	if statusOnly != "" {
		cleanRedemption.Status = redemption.Status
	}
	err = cleanRedemption.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    cleanRedemption,
	})
}

func DeleteProviderRedemption(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	err := model.DeleteRedemptionByIdInProvider(id, provider.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteInvalidProviderRedemption(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	rows, err := model.DeleteInvalidRedemptionsByProvider(provider.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

func validateExpiredTime(c *gin.Context, expired int64) (bool, string) {
	if expired != 0 && expired < common.GetTimestamp() {
		return false, i18n.T(c, i18n.MsgRedemptionExpireTimeInvalid)
	}
	return true, ""
}
