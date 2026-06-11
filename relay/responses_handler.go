package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	// Responses→Chat channel: convert /v1/responses to /v1/chat/completions.
	if info.ApiType == appconstant.APITypeResponsesChat {
		return responsesViaChatCompletions(c, info)
	}

	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		default:
			return types.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:              req.Model,
			Input:              req.Input,
			Instructions:       req.Instructions,
			PreviousResponseID: req.PreviousResponseID,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(responsesReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	auditRequestMessages := service.ModelContentAuditJSONString(request.Input)
	defer func() {
		service.EnqueueModelContentAuditFromRelay(c, info, auditRequestMessages, newAPIError)
	}()

	// 模型映射（model 名称已确定）
	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	// 获取上游适配器
	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	var requestBody io.Reader
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		logger.LogDebug(c, "responses route using original body pass-through: channel_id=%d channel_type=%d api_type=%d base_url=%q", info.ChannelId, info.ChannelType, info.ApiType, info.ChannelBaseUrl)
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		requestBody = common.ReaderOnly(storage)
	} else {
		// 请求格式转换的入口
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI Responses API
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}

		logger.LogDebug(c, "requestBody: %s", jsonData)
		requestBody = bytes.NewBuffer(jsonData)
	}

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)

		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usageDto := usage.(*dto.Usage)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData

		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
		}
		service.PostTextConsumeQuota(c, info, usageDto, nil)

		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
		service.PostAudioConsumeQuota(c, info, usageDto, "")
	} else {
		service.PostTextConsumeQuota(c, info, usageDto, nil)
	}
	return nil
}

// ResponsesChatCompletionsPath 根据渠道 base URL 选择对应的 Chat Completions 端点路径
func ResponsesChatCompletionsPath(baseURL string) string {
	if strings.Contains(baseURL, "ark.cn-beijing.volces.com") {
		return "/api/coding/v3/chat/completions"
	}
	return "/v1/chat/completions"
}

// responsesViaChatCompletions 将 Responses API 请求转换为 Chat Completions 请求，
// 然后委托给 TextHelper 处理。用于不支持 /v1/responses 端点的通道。
func responsesViaChatCompletions(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	// 根据请求的实际类型提取 Responses 请求对象
	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		// 标准的 Responses 请求，直接使用
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		// 压缩请求，转换为标准 Responses 请求格式
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:              req.Model,              // 模型名称
			Input:              req.Input,              // 输入内容
			Instructions:       req.Instructions,       // 系统指令
			PreviousResponseID: req.PreviousResponseID, // 上一次响应 ID（用于会话延续）
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type for Responses→Chat conversion: %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	// 对于 Responses→Chat 类型通道，记录原始请求体用于诊断调试
	// if info.ApiType == appconstant.APITypeResponsesChat || info.ChannelType == appconstant.ChannelTypeResponsesChat {
	// 	if storage, err := common.GetBodyStorage(c); err == nil {
	// 		if rawBody, bErr := storage.Bytes(); bErr == nil {
	// 			logResponsesCompatFullBody(c, info, "original responses request body before conversion", info.RequestURLPath, rawBody)
	// 		} else {
	// 			logger.LogWarn(c, fmt.Sprintf("failed to read original Responses→Chat request body for diagnostic logging: %s", bErr.Error()))
	// 		}
	// 	} else {
	// 		logger.LogWarn(c, fmt.Sprintf("failed to get original Responses→Chat request body for diagnostic logging: %s", err.Error()))
	// 	}
	// }

	// 将 Responses 请求转换为 Chat Completions 兼容请求
	chatReq, err := service.ResponsesRequestToChatCompletionsCompatRequest(responsesReq)
	if err != nil {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("failed to convert responses request to chat request: %w", err),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	// 保存原始的请求信息，以便在 defer 中恢复，避免影响后续处理
	savedRequest := info.Request                                       // 原始请求对象
	savedRelayMode := info.RelayMode                                   // 原始中继模式
	savedRequestURLPath := info.RequestURLPath                         // 原始请求路径
	savedForceRequestBodyConversion := info.ForceRequestBodyConversion // 原始请求体转换标记
	defer func() {
		// 恢复原始信息，防止 TextHelper 内部修改泄漏
		info.Request = savedRequest
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
		info.ForceRequestBodyConversion = savedForceRequestBodyConversion
	}()

	// 将中继信息切换为 Chat Completions 模式
	info.Request = chatReq                                                  // 替换为转换后的 Chat 请求
	info.RelayMode = relayconstant.RelayModeChatCompletions                 // 切换为 Chat Completions 中继模式
	info.RequestURLPath = ResponsesChatCompletionsPath(info.ChannelBaseUrl) // 修改请求路径为 Chat Completions 端点
	info.ForceRequestBodyConversion = true                                  // 强制使用转换后的请求体
	logger.LogDebug(c, "responses route converted to chat completions: channel_id=%d original_path=%q new_path=%q", info.ChannelId, savedRequestURLPath, info.RequestURLPath)

	// 委托给 TextHelper 以 Chat Completions 模式处理请求
	return TextHelper(c, info)
}
