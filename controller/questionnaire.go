package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// 问卷提交请求体。
// SurveyData 接收整个问卷表单的 JSON 对象（前端表单 values），
// 以原始 JSON 文本形式落库，字段增减不需要改表。
// Domain 可选：用户未登录时用于解析站点归属的服务商 ID。
type UserQuestionnaireRequest struct {
	SurveyData map[string]interface{} `json:"survey_data" validate:"required"`
	Domain     string                 `json:"domain"`
}

// SubmitUserQuestionnaire 用户提交问卷
// 接口：POST /api/user/questionnaire（公开，无需登录）
// 站点归属解析优先级：
//  1. 站点域名（前端传入的页面域名，或请求所在域名）匹配 provider_domains；
//  2. 域名未匹配到服务商（主站）：已登录用户取其 provider_id，未登录归主站(0)
func SubmitUserQuestionnaire(c *gin.Context) {
	var req UserQuestionnaireRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request body")
		return
	}
	if len(req.SurveyData) == 0 {
		common.ApiErrorMsg(c, "问卷数据不能为空")
		return
	}

	providerId, err := resolveQuestionnaireProviderId(c, req.Domain)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	userId := c.GetInt("id")

	jsonBytes, err := common.Marshal(req.SurveyData)
	if err != nil {
		common.ApiErrorMsg(c, "问卷数据序列化失败")
		return
	}

	record, err := model.CreateUserQuestionnaire(providerId, userId, string(jsonBytes))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "提交成功",
		"data": gin.H{
			"id": record.Id,
		},
	})
}

// resolveQuestionnaireProviderId 解析问卷提交所属的服务商 ID。
// 域名优先：在哪个站点的页面上提交，就归属哪个站点（服务商站点的登录用户可能是
// provider_id=0 的主站账号，例如服务商 owner，按用户 provider_id 归属会错误归到主站）。
// 域名未匹配到服务商（主站）时，已登录用户取其 provider_id，未登录归主站(0)。
func resolveQuestionnaireProviderId(c *gin.Context, domain string) (int, error) {
	// 第一优先级：按站点域名匹配服务商
	// 前端未传 domain 时兜底取请求所在域名（与 TenantResolver 语义一致）
	if domain == "" {
		domain = c.Request.Host
	}
	domain = model.NormalizeProviderDomain(domain)
	if domain != "" {
		ctx, err := model.GetProviderContextByDomainCached(domain)
		if err != nil {
			common.SysLog("failed to resolve questionnaire provider domain: " + err.Error())
		} else if ctx != nil {
			return ctx.ProviderId, nil
		}
	}

	// 第二优先级：域名未匹配到服务商（主站请求），已登录用户取其 provider_id
	userId := c.GetInt("id")
	if userId > 0 {
		user, err := model.GetUserById(userId, false)
		if err != nil {
			return 0, errors.New("获取用户信息失败")
		}
		return user.ProviderId, nil
	}
	return 0, nil // 主站
}

// GetMyUserQuestionnaires 用户查询自己提交的问卷记录
// 接口：GET /api/user/questionnaire
func GetMyUserQuestionnaires(c *gin.Context) {
	userId := c.GetInt("id")
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)

	records, err := model.GetUserQuestionnairesInProvider(providerId, userId)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	data := make([]gin.H, 0, len(records))
	for _, r := range records {
		data = append(data, gin.H{
			"id":          r.Id,
			"survey_data": r.SurveyData,
			"created_at":  r.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// GetUserQuestionnairesAdmin 管理端分页查询问卷提交记录
// 接口：GET /api/questionnaire?p=1&page_size=10&provider_id=0（AdminAuth）
// 仅主站管理员(providerId=0)可调用。provider_id 筛选：缺省/非法 → 0（主站，保持原行为）；
// >0 → 指定服务商站点；-1 → 全部站点。
func GetUserQuestionnairesAdmin(c *gin.Context) {
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	if providerId != 0 {
		common.ApiErrorMsg(c, "仅主站管理员可访问")
		return
	}

	filterProvider := 0
	if v := strings.TrimSpace(c.Query("provider_id")); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			filterProvider = p
		}
	}

	pageInfo := common.GetPageQuery(c)
	records, total, err := model.GetUserQuestionnairesAdmin(filterProvider, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
	return
}

// DeleteUserQuestionnaire 主站管理端删除问卷提交记录（任意站点）
// 接口：DELETE /api/questionnaire/:id（AdminAuth）
func DeleteUserQuestionnaire(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid id")
		return
	}
	providerId := common.GetContextKeyInt(c, constant.ContextKeyProviderId)
	if providerId != 0 {
		common.ApiErrorMsg(c, "仅主站管理员可访问")
		return
	}
	if err := model.DeleteUserQuestionnaireById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// GetProviderUserQuestionnaires 服务商 owner 分页查询本站问卷提交记录
// 接口：GET /api/provider/questionnaires?p=1&page_size=10（UserAuth，需为服务商 owner）
func GetProviderUserQuestionnaires(c *gin.Context) {
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}

	pageInfo := common.GetPageQuery(c)
	records, total, err := model.GetUserQuestionnairesAdmin(provider.Id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(records)
	common.ApiSuccess(c, pageInfo)
	return
}

// DeleteProviderUserQuestionnaire 服务商 owner 删除本站问卷提交记录
// 接口：DELETE /api/provider/questionnaires/:id（UserAuth，需为服务商 owner）
func DeleteProviderUserQuestionnaire(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid id")
		return
	}
	provider, ok := getOwnedProvider(c)
	if !ok {
		return
	}
	if err := model.DeleteUserQuestionnaire(id, provider.Id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
