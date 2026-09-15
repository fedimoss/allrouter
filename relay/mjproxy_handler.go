package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

func RelayMidjourneyImage(c *gin.Context) {
	taskId := c.Param("id")
	midjourneyTask := model.GetByOnlyMJId(taskId)
	if midjourneyTask == nil {
		c.JSON(400, gin.H{
			"error": "midjourney_task_not_found",
		})
		return
	}
	var httpClient *http.Client
	if channel, err := model.CacheGetChannel(midjourneyTask.ChannelId); err == nil {
		proxy := channel.GetSetting().Proxy
		if proxy != "" {
			if httpClient, err = service.NewProxyHttpClient(proxy); err != nil {
				c.JSON(400, gin.H{
					"error": "proxy_url_invalid",
				})
				return
			}
		}
	}
	if httpClient == nil {
		httpClient = service.GetHttpClient()
	}
	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(midjourneyTask.ImageUrl, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": fmt.Sprintf("request blocked: %v", err),
		})
		return
	}
	resp, err := httpClient.Get(midjourneyTask.ImageUrl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "http_get_image_failed",
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{
			"error": string(responseBody),
		})
		return
	}
	// 从Content-Type头获取MIME类型
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		// 如果无法确定内容类型，则默认为jpeg
		contentType = "image/jpeg"
	}
	// 设置响应的内容类型
	c.Writer.Header().Set("Content-Type", contentType)
	// 将图片流式传输到响应体
	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Println("Failed to stream image:", err)
	}
	return
}

func RelayMidjourneyNotify(c *gin.Context) *dto.MidjourneyResponse {
	var midjRequest dto.MidjourneyDto
	err := common.UnmarshalBodyReusable(c, &midjRequest)
	if err != nil {
		return &dto.MidjourneyResponse{
			Code:        4,
			Description: "bind_request_body_failed",
			Properties:  nil,
			Result:      "",
		}
	}
	midjourneyTask := model.GetByOnlyMJId(midjRequest.MjId)
	if midjourneyTask == nil {
		return &dto.MidjourneyResponse{
			Code:        4,
			Description: "midjourney_task_not_found",
			Properties:  nil,
			Result:      "",
		}
	}
	midjourneyTask.Progress = midjRequest.Progress
	midjourneyTask.PromptEn = midjRequest.PromptEn
	midjourneyTask.State = midjRequest.State
	midjourneyTask.SubmitTime = midjRequest.SubmitTime
	midjourneyTask.StartTime = midjRequest.StartTime
	midjourneyTask.FinishTime = midjRequest.FinishTime
	midjourneyTask.ImageUrl = midjRequest.ImageUrl
	midjourneyTask.VideoUrl = midjRequest.VideoUrl
	videoUrlsStr, _ := json.Marshal(midjRequest.VideoUrls)
	midjourneyTask.VideoUrls = string(videoUrlsStr)
	midjourneyTask.Status = midjRequest.Status
	midjourneyTask.FailReason = midjRequest.FailReason
	err = midjourneyTask.Update()
	if err != nil {
		return &dto.MidjourneyResponse{
			Code:        4,
			Description: "update_midjourney_task_failed",
		}
	}

	return nil
}

func coverMidjourneyTaskDto(c *gin.Context, originTask *model.Midjourney) (midjourneyTask dto.MidjourneyDto) {
	midjourneyTask.MjId = originTask.MjId
	midjourneyTask.Progress = originTask.Progress
	midjourneyTask.PromptEn = originTask.PromptEn
	midjourneyTask.State = originTask.State
	midjourneyTask.SubmitTime = originTask.SubmitTime
	midjourneyTask.StartTime = originTask.StartTime
	midjourneyTask.FinishTime = originTask.FinishTime
	midjourneyTask.ImageUrl = ""
	if originTask.ImageUrl != "" && setting.MjForwardUrlEnabled {
		midjourneyTask.ImageUrl = system_setting.ServerAddress + "/mj/image/" + originTask.MjId
		if originTask.Status != "SUCCESS" {
			midjourneyTask.ImageUrl += "?rand=" + strconv.FormatInt(time.Now().UnixNano(), 10)
		}
	} else {
		midjourneyTask.ImageUrl = originTask.ImageUrl
	}
	if originTask.VideoUrl != "" {
		midjourneyTask.VideoUrl = originTask.VideoUrl
	}
	midjourneyTask.Status = originTask.Status
	midjourneyTask.FailReason = originTask.FailReason
	midjourneyTask.Action = originTask.Action
	midjourneyTask.Description = originTask.Description
	midjourneyTask.Prompt = originTask.Prompt
	if originTask.Buttons != "" {
		var buttons []dto.ActionButton
		err := json.Unmarshal([]byte(originTask.Buttons), &buttons)
		if err == nil {
			midjourneyTask.Buttons = buttons
		}
	}
	if originTask.VideoUrls != "" {
		var videoUrls []dto.ImgUrls
		err := json.Unmarshal([]byte(originTask.VideoUrls), &videoUrls)
		if err == nil {
			midjourneyTask.VideoUrls = videoUrls
		}
	}
	if originTask.Properties != "" {
		var properties dto.Properties
		err := json.Unmarshal([]byte(originTask.Properties), &properties)
		if err == nil {
			midjourneyTask.Properties = &properties
		}
	}
	return
}

func RelaySwapFace(c *gin.Context, info *relaycommon.RelayInfo) *dto.MidjourneyResponse {
	var swapFaceRequest dto.SwapFaceRequest
	if err := common.UnmarshalBodyReusable(c, &swapFaceRequest); err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "bind_request_body_failed")
	}

	info.InitChannelMeta(c)
	if swapFaceRequest.SourceBase64 == "" || swapFaceRequest.TargetBase64 == "" {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "sour_base64_and_target_base64_is_required")
	}
	modelName := service.CovertMjpActionToModelName(constant.MjActionSwapFace)

	priceData, err := helper.ModelPriceHelperPerCall(c, info)
	if err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, err.Error())
	}
	info.PriceData = priceData

	// Midjourney tasks outlive the submit request, therefore the full amount
	// must be reserved before contacting the upstream. This also lets a
	// subscription fund the request when the wallet balance is zero.
	if !priceData.FreeModel {
		info.ForcePreConsume = true
		if apiErr := service.PreConsumeBilling(c, priceData.Quota, info); apiErr != nil {
			return service.MidjourneyErrorWrapper(constant.MjRequestError, apiErr.Error())
		}
	}

	requestURL := getMjRequestPath(c.Request.URL.String())
	baseURL := c.GetString("base_url")
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)
	mjResp, _, err := service.DoMidjourneyHttpRequest(c, time.Second*60, fullRequestURL)
	if err != nil {
		if info.Billing != nil {
			info.Billing.Refund(c)
		}
		return &mjResp.Response
	}

	midjResponse := &mjResp.Response
	// Swap-face is synchronous: only HTTP-success/code=1 is accepted. Free
	// models still produce a normal task, but do not deduct quota.
	accepted := mjResp.StatusCode == http.StatusOK && midjResponse.Code == 1
	charged := accepted && !priceData.FreeModel
	if !accepted && info.Billing != nil {
		info.Billing.Refund(c)
	}

	midjourneyTask := &model.Midjourney{
		UserId:      info.UserId,
		Code:        midjResponse.Code,
		Action:      constant.MjActionSwapFace,
		MjId:        midjResponse.Result,
		Prompt:      "InsightFace",
		Description: midjResponse.Description,
		SubmitTime:  info.StartTime.UnixNano() / int64(time.Millisecond),
		StartTime:   time.Now().UnixNano() / int64(time.Millisecond),
		ChannelId:   c.GetInt("channel_id"),
		Progress:    "0%",
		Quota:       0,
	}
	if accepted {
		midjourneyTask.Quota = priceData.Quota
	}
	if midjResponse.Code != 1 {
		midjourneyTask.FailReason = midjResponse.Description
	}
	if err := midjourneyTask.Insert(); err != nil {
		if info.Billing != nil && info.Billing.NeedsRefund() {
			info.Billing.Refund(c)
		}
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "insert_midjourney_task_failed")
	}

	if charged {
		if err := service.SettleBilling(c, info, priceData.Quota); err != nil {
			if info.Billing != nil && info.Billing.NeedsRefund() {
				info.Billing.Refund(c)
			}
			return service.MidjourneyErrorWrapper(constant.MjRequestError, "settle_midjourney_billing_failed")
		}
		if err := service.RecordMidjourneyBilling(midjourneyTask, info); err != nil {
			common.SysLog("error persisting Midjourney billing: " + err.Error())
			return service.MidjourneyErrorWrapper(constant.MjRequestError, "persist_midjourney_billing_failed")
		}
		other := service.GenerateMjOtherInfo(info, priceData)
		logContent := fmt.Sprintf("Midjourney fixed price %.2f, group ratio %.2f, action %s", priceData.ModelPrice, priceData.GroupRatioInfo.GroupRatio, constant.MjActionSwapFace)
		model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
			ChannelId: info.ChannelId,
			ModelName: modelName,
			TokenName: c.GetString("token_name"),
			Quota:     priceData.Quota,
			Content:   logContent,
			TokenId:   info.TokenId,
			Group:     info.UsingGroup,
			Other:     other,
		})
		if info.BillingSource == service.BillingSourceSubscription {
			model.UpdateUserRequestCount(info.UserId, 1)
		} else {
			model.UpdateUserUsedQuotaAndRequestCount(info.UserId, priceData.Quota)
		}
		model.UpdateChannelUsedQuota(info.ChannelId, priceData.Quota)
	}

	c.Writer.WriteHeader(mjResp.StatusCode)
	respBody, err := common.Marshal(midjResponse)
	if err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "marshal_response_body_failed")
	}
	if _, err = io.Copy(c.Writer, bytes.NewBuffer(respBody)); err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "copy_response_body_failed")
	}
	return nil
}

func RelayMidjourneyTaskImageSeed(c *gin.Context) *dto.MidjourneyResponse {
	taskId := c.Param("id")
	userId := c.GetInt("id")
	originTask := model.GetByMJId(userId, taskId)
	if originTask == nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "task_no_found")
	}
	channel, err := model.GetChannelById(originTask.ChannelId, true)
	if err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "get_channel_info_failed")
	}
	if channel.Status != common.ChannelStatusEnabled {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "该任务所属渠道已被禁用")
	}
	c.Set("channel_id", originTask.ChannelId)
	c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", channel.Key))

	requestURL := getMjRequestPath(c.Request.URL.String())
	fullRequestURL := fmt.Sprintf("%s%s", channel.GetBaseURL(), requestURL)
	midjResponseWithStatus, _, err := service.DoMidjourneyHttpRequest(c, time.Second*30, fullRequestURL)
	if err != nil {
		return &midjResponseWithStatus.Response
	}
	midjResponse := &midjResponseWithStatus.Response
	c.Writer.WriteHeader(midjResponseWithStatus.StatusCode)
	respBody, err := json.Marshal(midjResponse)
	if err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "unmarshal_response_body_failed")
	}
	service.IOCopyBytesGracefully(c, nil, respBody)
	return nil
}

func RelayMidjourneyTask(c *gin.Context, relayMode int) *dto.MidjourneyResponse {
	userId := c.GetInt("id")
	var err error
	var respBody []byte
	switch relayMode {
	case relayconstant.RelayModeMidjourneyTaskFetch:
		taskId := c.Param("id")
		originTask := model.GetByMJId(userId, taskId)
		if originTask == nil {
			return &dto.MidjourneyResponse{
				Code:        4,
				Description: "task_no_found",
			}
		}
		midjourneyTask := coverMidjourneyTaskDto(c, originTask)
		respBody, err = json.Marshal(midjourneyTask)
		if err != nil {
			return &dto.MidjourneyResponse{
				Code:        4,
				Description: "unmarshal_response_body_failed",
			}
		}
	case relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		var condition = struct {
			IDs []string `json:"ids"`
		}{}
		err = c.BindJSON(&condition)
		if err != nil {
			return &dto.MidjourneyResponse{
				Code:        4,
				Description: "do_request_failed",
			}
		}
		var tasks []dto.MidjourneyDto
		if len(condition.IDs) != 0 {
			originTasks := model.GetByMJIds(userId, condition.IDs)
			for _, originTask := range originTasks {
				midjourneyTask := coverMidjourneyTaskDto(c, originTask)
				tasks = append(tasks, midjourneyTask)
			}
		}
		if tasks == nil {
			tasks = make([]dto.MidjourneyDto, 0)
		}
		respBody, err = json.Marshal(tasks)
		if err != nil {
			return &dto.MidjourneyResponse{
				Code:        4,
				Description: "unmarshal_response_body_failed",
			}
		}
	}

	c.Writer.Header().Set("Content-Type", "application/json")

	_, err = io.Copy(c.Writer, bytes.NewBuffer(respBody))
	if err != nil {
		return &dto.MidjourneyResponse{
			Code:        4,
			Description: "copy_response_body_failed",
		}
	}
	return nil
}

func RelayMidjourneySubmit(c *gin.Context, relayInfo *relaycommon.RelayInfo) *dto.MidjourneyResponse {
	consumeQuota := true
	var midjRequest dto.MidjourneyRequest
	if err := common.UnmarshalBodyReusable(c, &midjRequest); err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "bind_request_body_failed")
	}

	relayInfo.InitChannelMeta(c)
	if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyAction { // Midjourney Plus action parsed from customId.
		if mjErr := service.CoverPlusActionToNormalAction(&midjRequest); mjErr != nil {
			return mjErr
		}
		relayInfo.RelayMode = relayconstant.RelayModeMidjourneyChange
	}
	if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyVideo {
		midjRequest.Action = constant.MjActionVideo
	}

	if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyImagine {
		if midjRequest.Prompt == "" {
			return service.MidjourneyErrorWrapper(constant.MjRequestError, "prompt_is_required")
		}
		midjRequest.Action = constant.MjActionImagine
	} else if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyDescribe {
		midjRequest.Action = constant.MjActionDescribe
	} else if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyEdits {
		midjRequest.Action = constant.MjActionEdits
	} else if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyShorten {
		midjRequest.Action = constant.MjActionShorten
	} else if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyBlend {
		midjRequest.Action = constant.MjActionBlend
	} else if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyUpload {
		midjRequest.Action = constant.MjActionUpload
	} else if midjRequest.TaskId != "" {
		mjId := ""
		if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyChange {
			if midjRequest.TaskId == "" {
				return service.MidjourneyErrorWrapper(constant.MjRequestError, "task_id_is_required")
			} else if midjRequest.Action == "" {
				return service.MidjourneyErrorWrapper(constant.MjRequestError, "action_is_required")
			} else if midjRequest.Index == 0 {
				return service.MidjourneyErrorWrapper(constant.MjRequestError, "index_is_required")
			}
			mjId = midjRequest.TaskId
		} else if relayInfo.RelayMode == relayconstant.RelayModeMidjourneySimpleChange {
			if midjRequest.Content == "" {
				return service.MidjourneyErrorWrapper(constant.MjRequestError, "content_is_required")
			}
			params := service.ConvertSimpleChangeParams(midjRequest.Content)
			if params == nil {
				return service.MidjourneyErrorWrapper(constant.MjRequestError, "content_parse_failed")
			}
			mjId = params.TaskId
			midjRequest.Action = params.Action
		} else if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyModal {
			mjId = midjRequest.TaskId
			midjRequest.Action = constant.MjActionModal
		} else if relayInfo.RelayMode == relayconstant.RelayModeMidjourneyVideo {
			midjRequest.Action = constant.MjActionVideo
			if midjRequest.TaskId == "" {
				return service.MidjourneyErrorWrapper(constant.MjRequestError, "task_id_is_required")
			} else if midjRequest.Action == "" {
				return service.MidjourneyErrorWrapper(constant.MjRequestError, "action_is_required")
			}
			mjId = midjRequest.TaskId
		}

		originTask := model.GetByMJId(relayInfo.UserId, mjId)
		if originTask == nil {
			return service.MidjourneyErrorWrapper(constant.MjRequestError, "task_not_found")
		}
		if setting.MjActionCheckSuccessEnabled {
			if originTask.Status != "SUCCESS" && relayInfo.RelayMode != relayconstant.RelayModeMidjourneyModal {
				return service.MidjourneyErrorWrapper(constant.MjRequestError, "task_status_not_success")
			}
		}
		channel, err := model.GetChannelById(originTask.ChannelId, true)
		if err != nil {
			return service.MidjourneyErrorWrapper(constant.MjRequestError, "get_channel_info_failed")
		}
		if channel.Status != common.ChannelStatusEnabled {
			return service.MidjourneyErrorWrapper(constant.MjRequestError, "origin task channel is disabled")
		}
		c.Set("base_url", channel.GetBaseURL())
		c.Set("channel_id", originTask.ChannelId)
		c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", channel.Key))
		logger.LogDebug(c, "Midjourney action uses origin channel: id=%s, base_url=%s", strconv.Itoa(originTask.ChannelId), channel.GetBaseURL())
		midjRequest.Prompt = originTask.Prompt
	}

	if midjRequest.Action == constant.MjActionInPaint || midjRequest.Action == constant.MjActionCustomZoom {
		consumeQuota = false
	}

	requestURL := getMjRequestPath(c.Request.URL.String())
	baseURL := c.GetString("base_url")
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)
	modelName := service.CovertMjpActionToModelName(midjRequest.Action)

	priceData, err := helper.ModelPriceHelperPerCall(c, relayInfo)
	if err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, err.Error())
	}
	relayInfo.PriceData = priceData

	// Reserve quota before submitting an asynchronous Midjourney job.  The
	// unified billing session selects a subscription when configured, so a
	// zero wallet balance no longer prevents a valid subscription request.
	if consumeQuota && !priceData.FreeModel {
		relayInfo.ForcePreConsume = true
		if apiErr := service.PreConsumeBilling(c, priceData.Quota, relayInfo); apiErr != nil {
			return service.MidjourneyErrorWrapper(constant.MjRequestError, apiErr.Error())
		}
	}

	midjResponseWithStatus, responseBody, err := service.DoMidjourneyHttpRequest(c, time.Second*60, fullRequestURL)
	if err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		return &midjResponseWithStatus.Response
	}
	midjResponse := &midjResponseWithStatus.Response
	responseCode := midjResponse.Code // preserve the upstream code for task state

	// 21 = existing task, 22 = queued; both are accepted and billable.  All
	// other non-success codes are failures and must release the reservation.
	if responseCode != 1 && responseCode != 21 && responseCode != 22 {
		consumeQuota = false
	}

	if responseCode == 21 {
		if midjRequest.Action != constant.MjActionInPaint && midjRequest.Action != constant.MjActionCustomZoom {
			responseBody = []byte(strings.Replace(string(responseBody), `"code":21`, `"code":1`, -1))
		}
	}

	// A billable response must be an HTTP success and one of the accepted task
	// codes.  For code 21/22 the original code is retained in the task record.
	billable := consumeQuota && !priceData.FreeModel && midjResponseWithStatus.StatusCode == http.StatusOK &&
		(responseCode == 1 || responseCode == 21 || responseCode == 22)
	if !billable && relayInfo.Billing != nil {
		relayInfo.Billing.Refund(c)
	}

	midjourneyTask := &model.Midjourney{
		UserId:      relayInfo.UserId,
		Code:        responseCode,
		Action:      midjRequest.Action,
		MjId:        midjResponse.Result,
		Prompt:      midjRequest.Prompt,
		Description: midjResponse.Description,
		SubmitTime:  time.Now().UnixNano() / int64(time.Millisecond),
		ChannelId:   c.GetInt("channel_id"),
		Progress:    "0%",
		Quota:       0,
	}
	if !billable && responseCode != 1 {
		midjourneyTask.FailReason = midjResponse.Description
	}
	if responseCode == 21 {
		if properties, ok := midjResponse.Properties.(map[string]interface{}); ok {
			if imageURL, ok1 := properties["imageUrl"].(string); ok1 {
				midjourneyTask.ImageUrl = imageURL
			}
			if status, ok2 := properties["status"].(string); ok2 {
				midjourneyTask.Status = status
				if status == "SUCCESS" {
					midjourneyTask.Progress = "100%"
					midjourneyTask.StartTime = time.Now().UnixNano() / int64(time.Millisecond)
					midjourneyTask.FinishTime = midjourneyTask.StartTime
				}
			}
		}
	}
	if responseCode == 1 && midjRequest.Action == constant.MjActionUpload {
		midjourneyTask.Progress = "100%"
		midjourneyTask.Status = "SUCCESS"
	}
	if billable {
		midjourneyTask.Quota = priceData.Quota
	}
	if err := midjourneyTask.Insert(); err != nil {
		if relayInfo.Billing != nil && relayInfo.Billing.NeedsRefund() {
			relayInfo.Billing.Refund(c)
		}
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "insert_midjourney_task_failed")
	}

	if billable {
		// Persist the reservation snapshot before settlement so a later polling
		// worker can recover it even if the submit process exits unexpectedly.
		if err := service.RecordMidjourneyBilling(midjourneyTask, relayInfo); err != nil {
			if relayInfo.Billing != nil && relayInfo.Billing.NeedsRefund() {
				relayInfo.Billing.Refund(c)
			}
			return service.MidjourneyErrorWrapper(constant.MjRequestError, "persist_midjourney_billing_failed")
		}
		if err := service.SettleBilling(c, relayInfo, priceData.Quota); err != nil {
			if relayInfo.Billing != nil && relayInfo.Billing.NeedsRefund() {
				relayInfo.Billing.Refund(c)
			}
			return service.MidjourneyErrorWrapper(constant.MjRequestError, "settle_midjourney_billing_failed")
		}
		if relayInfo.BillingSource == service.BillingSourceWallet && relayInfo.Billing != nil {
			if err := service.RecordMidjourneyWalletFunding(midjourneyTask, relayInfo.WalletRewardConsumed, relayInfo.WalletPaidConsumed); err != nil {
				common.SysLog("error persisting Midjourney wallet funding: " + err.Error())
			}
		}
		// Re-save the billing fields after settlement (the first write captured
		// the pre-consume state and this one captures wallet funding details).
		if err := service.RecordMidjourneyBilling(midjourneyTask, relayInfo); err != nil {
			common.SysLog("error persisting Midjourney billing: " + err.Error())
		}
		if midjourneyTask.Status == string(model.TaskStatusSuccess) {
			service.FinalizeMidjourneyConsumeRebate(c, midjourneyTask)
		}

		tokenName := c.GetString("token_name")
		logContent := fmt.Sprintf("Midjourney fixed price %.2f, group ratio %.2f, action %s, ID %s", priceData.ModelPrice, priceData.GroupRatioInfo.GroupRatio, midjRequest.Action, midjResponse.Result)
		other := service.GenerateMjOtherInfo(relayInfo, priceData)
		model.RecordConsumeLog(c, relayInfo.UserId, model.RecordConsumeLogParams{
			ChannelId: relayInfo.ChannelId,
			ModelName: modelName,
			TokenName: tokenName,
			Quota:     priceData.Quota,
			Content:   logContent,
			TokenId:   relayInfo.TokenId,
			Group:     relayInfo.UsingGroup,
			Other:     other,
		})
		if relayInfo.BillingSource == service.BillingSourceSubscription {
			model.UpdateUserRequestCount(relayInfo.UserId, 1)
		} else {
			model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, priceData.Quota)
		}
		model.UpdateChannelUsedQuota(relayInfo.ChannelId, priceData.Quota)
	}

	if responseCode == 22 {
		responseBody = []byte(strings.Replace(string(responseBody), `"code":22`, `"code":1`, -1))
	}
	c.Writer.WriteHeader(midjResponseWithStatus.StatusCode)
	bodyReader := io.NopCloser(bytes.NewBuffer(responseBody))
	_, err = io.Copy(c.Writer, bodyReader)
	_ = bodyReader.Close()
	if err != nil {
		return service.MidjourneyErrorWrapper(constant.MjRequestError, "copy_response_body_failed")
	}
	return nil
}

type taskChangeParams struct {
	ID     string
	Action string
	Index  int
}

func getMjRequestPath(path string) string {
	requestURL := path
	if strings.Contains(requestURL, "/mj-") {
		urls := strings.Split(requestURL, "/mj/")
		if len(urls) < 2 {
			return requestURL
		}
		requestURL = "/mj/" + urls[1]
	}
	return requestURL
}
