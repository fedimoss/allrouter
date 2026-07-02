package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

var responsesCompatDiagFileMu sync.Mutex

// TextHelper 是 OpenAI 兼容接口的请求入口。
//
// 模型内容审计链路（第 1 段）：
//  1. 将请求消息整理成审计字符串（优先取最后一条 user 消息）
//  2. 注册 defer，确保请求处理结束时执行入队（第 3 段）
//  3. 请求处理过程中，上游适配器会调用 Set/AppendModelContentAuditResponseText
//     将响应内容存入 gin.Context（第 2 段）
//  4. defer 触发时从 gin.Context 取出完整内容，组装记录并入队
func TextHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	textReq, ok := info.Request.(*dto.GeneralOpenAIRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.GeneralOpenAIRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(textReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	// 【审计链路 第 1 段】从请求中提取审计用的请求消息（优先取最后一条 user 消息）
	auditRequestMessages := service.ModelContentAuditOpenAIRequestMessages(request)

	// 【审计链路 第 3 段】defer 注册：请求处理结束后，从 gin.Context 取出完整响应内容，组装记录并入队
	defer func() {
		service.EnqueueModelContentAuditFromRelay(c, info, auditRequestMessages, newAPIError)
	}()

	if request.WebSearchOptions != nil {
		c.Set("chat_completion_web_search_context_size", request.WebSearchOptions.SearchContextSize)
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	includeUsage := true
	// 判断用户是否需要返回使用情况
	if request.StreamOptions != nil {
		includeUsage = request.StreamOptions.IncludeUsage
	}

	// 如果不支持StreamOptions，将StreamOptions设置为nil
	if !info.SupportStreamOptions || !lo.FromPtrOr(request.Stream, false) {
		request.StreamOptions = nil
	} else {
		// 如果支持StreamOptions，且请求中没有设置StreamOptions，根据配置文件设置StreamOptions
		if constant.ForceStreamOption {
			request.StreamOptions = &dto.StreamOptions{
				IncludeUsage: true,
			}
		}
	}

	info.ShouldIncludeUsage = includeUsage

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	// 全局透传开关：是否将客户端原始请求体直接转发给上游，不做任何转换
	passThroughGlobal := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	// 最终透传开关：综合判断是否启用请求体透传。
	// 当 ForceRequestBodyConversion=true（Responses→Chat 兼容路径）时，强制禁用透传，
	// 确保请求体必须经过格式转换；否则取决于全局或通道级配置。
	passThroughBodyEnabled := !info.ForceRequestBodyConversion && (passThroughGlobal || info.ChannelSetting.PassThroughBodyEnabled)

	// Chat→Responses 反向兼容路径：
	// 当满足以下条件时，将 Chat Completions 请求转换为 Responses API 请求发送给上游：
	//   1. 当前中继模式为 Chat Completions
	//   2. 未启用请求体透传（需要经过转换）
	//   3. 全局配置允许该模型/通道使用 Responses API
	if info.RelayMode == relayconstant.RelayModeChatCompletions &&
		!passThroughBodyEnabled &&
		service.ShouldChatCompletionsUseResponsesGlobal(info.ChannelId, info.ChannelType, info.OriginModelName) {
		applySystemPromptIfNeeded(c, info, request)
		usage, newApiErr := chatCompletionsViaResponses(c, info, adaptor, request)
		if newApiErr != nil {
			return newApiErr
		}

		// 根据是否包含音频 token 和音频计费比例，选择音频或文本计费方式
		var containAudioTokens = usage.CompletionTokenDetails.AudioTokens > 0 || usage.PromptTokensDetails.AudioTokens > 0
		var containsAudioRatios = ratio_setting.ContainsAudioRatio(info.OriginModelName) || ratio_setting.ContainsAudioCompletionRatio(info.OriginModelName)

		if containAudioTokens && containsAudioRatios {
			service.PostAudioConsumeQuota(c, info, usage, "")
		} else {
			service.PostTextConsumeQuota(c, info, usage, nil)
		}
		return nil
	}

	var requestBody io.Reader

	if passThroughBodyEnabled {
		// 透传模式：直接读取客户端原始请求体，不做任何格式转换
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		// 调试模式下打印原始请求体
		if common.DebugEnabled {
			if debugBytes, bErr := storage.Bytes(); bErr == nil {
				logger.LogDebug(c, "requestBody: %s", debugBytes)
			}
		}
		requestBody = common.ReaderOnly(storage)
	} else {
		// 转换模式：通过适配器将请求转换为上游供应商所需的格式
		convertedRequest, err := adaptor.ConvertOpenAIRequest(c, info, request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

		// 处理通道级系统提示词的注入/覆盖
		if info.ChannelSetting.SystemPrompt != "" {
			// 如果有系统提示，则将其添加到请求中
			request, ok := convertedRequest.(*dto.GeneralOpenAIRequest)
			if ok {
				containSystemPrompt := false
				// 检查消息列表中是否已存在系统提示
				for _, message := range request.Messages {
					if message.Role == request.GetSystemRoleName() {
						containSystemPrompt = true
						break
					}
				}
				if !containSystemPrompt {
					// 如果没有系统提示，则在消息列表头部插入系统提示
					systemMessage := dto.Message{
						Role:    request.GetSystemRoleName(),
						Content: info.ChannelSetting.SystemPrompt,
					}
					request.Messages = append([]dto.Message{systemMessage}, request.Messages...)
				} else if info.ChannelSetting.SystemPromptOverride {
					// 如果已有系统提示且允许覆盖，则将通道系统提示拼接到原有内容前面
					common.SetContextKey(c, constant.ContextKeySystemPromptOverride, true)
					for i, message := range request.Messages {
						if message.Role == request.GetSystemRoleName() {
							if message.IsStringContent() {
								// 字符串内容：直接拼接
								request.Messages[i].SetStringContent(info.ChannelSetting.SystemPrompt + "\n" + message.StringContent())
							} else {
								// 多媒体内容：在内容数组头部插入文本部件
								contents := message.ParseContent()
								contents = append([]dto.MediaContent{
									{
										Type: dto.ContentTypeText,
										Text: info.ChannelSetting.SystemPrompt,
									},
								}, contents...)
								request.Messages[i].Content = contents
							}
							break
						}
					}
				}
			}
		}

		// 将转换后的请求序列化为 JSON
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
		}

		// 移除上游供应商不支持的字段
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// 应用参数覆盖（如模型参数的动态调整）
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}

		// 对于Responses→Chat，打印转换后的请求体, 用于调试
		// if info.ApiType == constant.APITypeResponsesChat || info.ChannelType == constant.ChannelTypeResponsesChat || info.ForceRequestBodyConversion {
		// 	logResponsesCompatChatRequestSummary(c, info, convertedRequest, len(jsonData))
		// 	logResponsesCompatFullBody(c, info, "converted chat completions request body after conversion", info.RequestURLPath, jsonData)
		// }

		logger.LogDebug(c, "text request body: %s", jsonData)

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
		info.IsStream = info.IsStream || strings.HasPrefix(httpResp.Header.Get("Content-Type"), "text/event-stream")
		if httpResp.StatusCode != http.StatusOK {
			newApiErr := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newApiErr, statusCodeMappingStr)
			return newApiErr
		}
	}

	usage, newApiErr := adaptor.DoResponse(c, httpResp, info)
	if newApiErr != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newApiErr, statusCodeMappingStr)
		return newApiErr
	}

	var containAudioTokens = usage.(*dto.Usage).CompletionTokenDetails.AudioTokens > 0 || usage.(*dto.Usage).PromptTokensDetails.AudioTokens > 0
	var containsAudioRatios = ratio_setting.ContainsAudioRatio(info.OriginModelName) || ratio_setting.ContainsAudioCompletionRatio(info.OriginModelName)

	if containAudioTokens && containsAudioRatios {
		service.PostAudioConsumeQuota(c, info, usage.(*dto.Usage), "")
	} else {
		service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
	}
	return nil
}

// logResponsesCompatChatRequestSummary 记录 Responses→Chat 兼容转换后的请求摘要信息。
// 包括通道信息、模型、消息数量、工具定义、消息内容预览等，用于诊断和调试兼容性问题。
func logResponsesCompatChatRequestSummary(c *gin.Context, info *relaycommon.RelayInfo, convertedRequest any, bodyBytes int) {
	// 尝试将转换后的请求断言为 GeneralOpenAIRequest 类型
	request, ok := convertedRequest.(*dto.GeneralOpenAIRequest)
	if !ok {
		// 非标准请求类型，仅记录基本信息
		logger.LogInfo(c, fmt.Sprintf(
			"responses compatibility upstream chat request summary: channel_id=%d channel_type=%d api_type=%d converted_type=%T body_bytes=%d",
			info.ChannelId,
			info.ChannelType,
			info.ApiType,
			convertedRequest,
			bodyBytes,
		))
		return
	}

	// 构建详细的请求摘要信息
	var b strings.Builder
	// 获取请求 ID，用于日志追踪
	requestID := "SYSTEM"
	if v := c.Value(common.RequestIdKey); v != nil {
		requestID = fmt.Sprintf("%v", v)
	}
	// 写入请求概览：通道信息、模型、路径、消息/工具数量等
	fmt.Fprintf(
		&b,
		"responses compatibility upstream chat request summary: request_id=%s channel_id=%d channel_type=%d api_type=%d model=%q path=%q body_bytes=%d messages=%d tools=%d has_tool_choice=%t stream_set=%t",
		requestID,
		info.ChannelId,
		info.ChannelType,
		info.ApiType,
		request.Model,
		info.RequestURLPath,
		bodyBytes,
		len(request.Messages),
		len(request.Tools),
		request.ToolChoice != nil,
		request.Stream != nil,
	)
	// 遍历每条消息，记录角色、内容长度、工具调用等详情
	for i, message := range request.Messages {
		toolCalls := message.ParseToolCalls() // 解析消息中的工具调用列表
		fmt.Fprintf(
			&b,
			"\n  message[%d]: role=%q content_len=%d tool_call_id=%q raw_tool_calls_bytes=%d parsed_tool_calls=%d",
			i,
			message.Role,
			messageContentLength(message), // 内容字节长度
			message.ToolCallId,
			len(message.ToolCalls),
			len(toolCalls),
		)
		// 如果消息内容不为空，生成内容预览（最多 1200 字符）
		if preview := messageContentPreview(message, 1200); preview != "" {
			fmt.Fprintf(&b, "\n    content_preview=%q", preview)
		}
		// 记录该消息中每个工具调用的详情
		for j, toolCall := range toolCalls {
			fmt.Fprintf(
				&b,
				"\n    tool_call[%d]: id=%q type=%q name=%q arguments_len=%d",
				j,
				toolCall.ID,
				toolCall.Type,
				toolCall.Function.Name,
				len(toolCall.Function.Arguments),
			)
		}
	}
	// 遍历每个工具定义，记录类型、名称和参数大小
	for i, tool := range request.Tools {
		parametersBytes := 0 // 工具参数的字节大小
		if tool.Function.Parameters != nil {
			if data, err := common.Marshal(tool.Function.Parameters); err == nil {
				parametersBytes = len(data)
			}
		}
		fmt.Fprintf(
			&b,
			"\n  tool[%d]: type=%q name=%q parameters_bytes=%d",
			i,
			tool.Type,
			tool.Function.Name,
			parametersBytes,
		)
	}

	// 将摘要写入日志和诊断文件
	summary := b.String()
	logger.LogInfo(c, summary)
	appendResponsesCompatDiagFile(c, summary)
}

// messageContentLength 计算消息内容的字节长度。
// 支持字符串内容和多媒体内容数组两种格式。
func messageContentLength(message dto.Message) int {
	// 内容为空，长度为 0
	if message.Content == nil {
		return 0
	}
	// 字符串内容直接返回长度
	if content, ok := message.Content.(string); ok {
		return len(content)
	}
	// 非字符串内容（如多媒体数组），序列化后计算长度
	data, err := common.Marshal(message.Content)
	if err != nil {
		return 0
	}
	return len(data)
}

// messageContentPreview 生成消息内容的预览文本。
// limit 参数指定最大预览长度，超出时截断并追加 "...(truncated)" 标记。
func messageContentPreview(message dto.Message, limit int) string {
	// 内容为空或限制无效，返回空字符串
	if message.Content == nil || limit <= 0 {
		return ""
	}

	var content string
	// 字符串内容直接使用
	if text, ok := message.Content.(string); ok {
		content = text
	} else if data, err := common.Marshal(message.Content); err == nil {
		// 非字符串内容，序列化为 JSON 字符串
		content = string(data)
	}

	// 去除首尾空白字符
	content = strings.TrimSpace(content)
	// 未超出限制，返回完整内容
	if len(content) <= limit {
		return content
	}
	// 超出限制，截断并追加省略标记
	return content[:limit] + "...(truncated)"
}

// appendResponsesCompatDiagFile 将摘要信息追加写入 Responses→Chat 兼容性诊断日志文件。
// 日志文件路径为 {LogDir}/responses-chat-compat.log，使用互斥锁保证并发安全。
func appendResponsesCompatDiagFile(c *gin.Context, summary string) {
	// 如果未配置日志目录，跳过写入
	if common.LogDir == nil || *common.LogDir == "" {
		return
	}
	// 加互斥锁，防止多个请求并发写入同一个文件
	responsesCompatDiagFileMu.Lock()
	defer responsesCompatDiagFileMu.Unlock()

	// 确保日志目录存在
	if err := os.MkdirAll(*common.LogDir, 0777); err != nil {
		logger.LogWarn(c, fmt.Sprintf("failed to create Responses→Chat compatibility diagnostic log directory %q: %s", *common.LogDir, err.Error()))
		return
	}
	// 打开或创建诊断日志文件（追加写入模式）
	path := filepath.Join(*common.LogDir, "responses-chat-compat.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logger.LogWarn(c, fmt.Sprintf("failed to open Responses→Chat compatibility diagnostic log file %q: %s", path, err.Error()))
		return
	}
	defer file.Close()

	// 带时间戳写入摘要内容
	if _, err := fmt.Fprintf(file, "[%s]\n%s\n\n", time.Now().Format("2006/01/02 - 15:04:05"), summary); err != nil {
		logger.LogWarn(c, fmt.Sprintf("failed to write Responses→Chat compatibility diagnostic log file %q: %s", path, err.Error()))
	}
}

// logResponsesCompatFullBody 将完整的请求/响应体转储到诊断日志文件中。
// stage 参数标识当前阶段（如 "original responses request body before conversion"），
// 用于区分转换前后的不同日志记录。
func logResponsesCompatFullBody(c *gin.Context, info *relaycommon.RelayInfo, stage string, requestPath string, body []byte) {
	// 获取请求 ID，用于日志追踪
	requestID := "SYSTEM"
	if v := c.Value(common.RequestIdKey); v != nil {
		requestID = fmt.Sprintf("%v", v)
	}

	// 构建包含完整请求体的摘要信息
	summary := fmt.Sprintf(
		"responses compatibility full body dump: stage=%q request_id=%s channel_id=%d channel_type=%d api_type=%d model=%q path=%q body_bytes=%d\n%s",
		stage,
		requestID,
		info.ChannelId,
		info.ChannelType,
		info.ApiType,
		info.OriginModelName,
		requestPath,
		len(body),
		string(body),
	)
	// 写入诊断日志文件
	appendResponsesCompatDiagFile(c, summary)

	// 如果配置了日志目录，记录转储成功的日志
	if common.LogDir != nil && *common.LogDir != "" {
		logger.LogInfo(c, fmt.Sprintf(
			"responses compatibility full body dumped: stage=%q request_id=%s path=%q body_bytes=%d file=%q",
			stage,
			requestID,
			requestPath,
			len(body),
			filepath.Join(*common.LogDir, "responses-chat-compat.log"),
		))
		return
	}
	// 未配置日志目录时，记录跳过转储的日志
	logger.LogInfo(c, fmt.Sprintf(
		"responses compatibility full body dump skipped because log dir is empty: stage=%q request_id=%s path=%q body_bytes=%d",
		stage,
		requestID,
		requestPath,
		len(body),
	))
}
