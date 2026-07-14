package openai

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// responsesToolCallState 用于在流式转换过程中跟踪工具调用的累积状态。
type responsesToolCallState struct {
	ID          string // 工具调用条目的唯一标识（前缀 "fc_"）
	CallID      string // 原始调用 ID（通常来自上游）
	Name        string // 函数名称
	Arguments   string // 累积的函数参数 JSON 字符串
	OutputIndex int    // 该工具调用在 Responses output 数组中的位置索引
	Added       bool   // 是否已发送 item.added 事件
}

// responsesStatus 将状态字符串序列化为 JSON 字节切片。
func responsesStatus(status string) []byte {
	raw, _ := common.Marshal(status)
	return raw
}

// responsesUsageFromChat 将 Chat Completions 的 Usage 转换为 Responses API 兼容的 Usage。
// 当 InputTokens/OutputTokens 为零时，回退使用 PromptTokens/CompletionTokens 的值。
func responsesUsageFromChat(usage *dto.Usage) *dto.Usage {
	if usage == nil {
		return nil
	}
	out := *usage
	// 如果 InputTokens 为零，使用 PromptTokens 作为输入 token 数
	if out.InputTokens == 0 {
		out.InputTokens = out.PromptTokens
	}
	// 如果 OutputTokens 为零，使用 CompletionTokens 作为输出 token 数
	if out.OutputTokens == 0 {
		out.OutputTokens = out.CompletionTokens
	}
	// 如果 TotalTokens 为零，由输入和输出 token 数计算得出
	if out.TotalTokens == 0 {
		out.TotalTokens = out.InputTokens + out.OutputTokens
	}
	// 如果 InputTokensDetails 为空，指向 PromptTokensDetails
	if out.InputTokensDetails == nil {
		out.InputTokensDetails = &out.PromptTokensDetails
	}
	return &out
}

// responsesArgumentsRaw 将函数参数字符串序列化为 JSON 字节切片。
// 序列化失败时返回空字符串的 JSON 表示。
func responsesArgumentsRaw(arguments string) []byte {
	raw, err := common.Marshal(arguments)
	if err != nil {
		return []byte(`""`)
	}
	return raw
}

// sendResponsesEvent 向客户端发送一个 Responses API 流式事件。
// 同时记录事件日志以便调试兼容性问题。
func sendResponsesEvent(c *gin.Context, event dto.ResponsesStreamResponse) error {
	data, err := common.Marshal(event)
	if err != nil {
		return err
	}
	// logger.LogInfo(c, fmt.Sprintf("responses compatibility converted stream event: event=%s body=%s", event.Type, string(data)))
	helper.ResponseChunkData(c, event, string(data))
	return nil
}

// chatStreamToResponsesResponse 根据流式处理过程中收集的数据，
// 构建一个完整的 Responses API 非流式响应对象。
// 按索引顺序排列 message 和 function_call 输出条目。
func chatStreamToResponsesResponse(responseID string, createdAt int64, model string, text string, usage *dto.Usage, sentMessage bool, messageOutputIndex int, toolCalls map[int]*responsesToolCallState) *dto.OpenAIResponsesResponse {
	// 按索引构建输出条目映射
	outputByIndex := map[int]dto.ResponsesOutput{}
	// 如果有文本消息内容，构建 message 输出条目
	if sentMessage || text != "" {
		if messageOutputIndex < 0 {
			messageOutputIndex = 0
		}
		outputByIndex[messageOutputIndex] = dto.ResponsesOutput{
			Type:   "message",
			ID:     "msg_" + responseID,
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{
				{
					Type:        "output_text",
					Text:        text,
					Annotations: []interface{}{},
				},
			},
		}
	}
	// 记录最大输出索引，用于后续按序生成数组
	maxOutputIndex := messageOutputIndex
	// 将工具调用按其 OutputIndex 放入映射
	for _, state := range toolCalls {
		outputByIndex[state.OutputIndex] = dto.ResponsesOutput{
			Type:      "function_call",
			ID:        state.ID,
			Status:    "completed",
			CallId:    state.CallID,
			Name:      state.Name,
			Arguments: responsesArgumentsRaw(state.Arguments),
		}
		if state.OutputIndex > maxOutputIndex {
			maxOutputIndex = state.OutputIndex
		}
	}
	// 按索引顺序生成有序的输出数组
	output := make([]dto.ResponsesOutput, 0, len(outputByIndex))
	for i := 0; i <= maxOutputIndex; i++ {
		item, ok := outputByIndex[i]
		if !ok {
			continue
		}
		output = append(output, item)
	}
	return &dto.OpenAIResponsesResponse{
		ID:        responseID,                    // 响应 ID
		Object:    "response",                    // 对象类型
		CreatedAt: int(createdAt),                // 创建时间戳
		Status:    responsesStatus("completed"),  // 响应状态
		Model:     model,                         // 使用的模型名称
		Output:    output,                        // 按序排列的输出条目
		Usage:     responsesUsageFromChat(usage), // token 用量
	}
}

// OaiChatToResponsesStreamHandler 将上游 Chat Completions 流式响应
// 转换为 Responses API 流式响应格式并发送给客户端。
// 处理文本内容、推理内容和工具调用的流式转换。
func OaiChatToResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	// 校验响应和响应体是否有效
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	// 初始化流式转换所需的状态变量
	responseID := helper.GetResponseID(c) // 响应唯一标识
	createdAt := time.Now().Unix()        // 创建时间戳
	model := info.UpstreamModelName       // 上游模型名称
	messageID := ""                       // 消息条目 ID
	nextOutputIndex := 0                  // 下一个可用的输出索引
	messageOutputIndex := -1              // 文本消息在输出数组中的索引（-1 表示尚未分配）
	contentIndex := 0                     // 内容部件索引

	var (
		usage       = &dto.Usage{}     // token 用量统计
		outputText  strings.Builder    // 累积的输出文本
		reasonText  strings.Builder    // 累积的推理文本
		streamErr   *types.NewAPIError // 流式处理过程中的错误
		sentCreated bool               // 是否已发送 response.created 事件
		sentMessage bool               // 是否已发送 message 条目的 item.added 事件
		sentPart    bool               // 是否已发送内容部件的 content_part.added 事件
	)
	toolCalls := map[int]*responsesToolCallState{} // 工具调用状态映射（按工具索引）

	// sendCreated 发送 response.created 事件，仅发送一次。
	// 如果已经发送过则直接返回 true。
	sendCreated := func() bool {
		if sentCreated {
			return true
		}
		// 确保响应 ID、时间戳和模型名称已初始化
		if responseID == "" {
			responseID = helper.GetResponseID(c)
		}
		if createdAt == 0 {
			createdAt = time.Now().Unix()
		}
		if model == "" {
			model = info.UpstreamModelName
		}
		err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type: "response.created",
			Response: &dto.OpenAIResponsesResponse{
				ID:        responseID,
				Object:    "response",
				CreatedAt: int(createdAt),
				Status:    responsesStatus("in_progress"),
				Model:     model,
				Output:    []dto.ResponsesOutput{},
			},
		})
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		sentCreated = true
		return true
	}

	// sendMessageStart 发送 message 条目的 item.added 事件，标志助手消息开始。
	sendMessageStart := func() bool {
		if sentMessage {
			return true
		}
		// 确保 response.created 事件已发送
		if !sendCreated() {
			return false
		}
		// 分配消息输出索引
		if messageOutputIndex < 0 {
			messageOutputIndex = nextOutputIndex
			nextOutputIndex++
		}
		messageID = "msg_" + responseID
		err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:        dto.ResponsesOutputTypeItemAdded,
			OutputIndex: &messageOutputIndex,
			Item: &dto.ResponsesOutput{
				Type:    "message",
				ID:      messageID,
				Status:  "in_progress",
				Role:    "assistant",
				Content: []dto.ResponsesOutputContent{},
			},
		})
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		sentMessage = true
		return true
	}

	// sendTextPartStart 发送 output_text 内容部件的 content_part.added 事件。
	sendTextPartStart := func() bool {
		if sentPart {
			return true
		}
		// 确保 message 条目已开始
		if !sendMessageStart() {
			return false
		}
		err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:         "response.content_part.added",
			ItemID:       messageID,
			OutputIndex:  &messageOutputIndex,
			ContentIndex: &contentIndex,
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "output_text",
				Text: "",
			},
		})
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		sentPart = true
		return true
	}

	// sendTextDelta 发送 output_text 的增量文本事件（response.output_text.delta）。
	// 累积文本内容用于最终汇总。
	sendTextDelta := func(delta string) bool {
		if delta == "" {
			return true
		}
		// 确保内容部件已开始
		if !sendTextPartStart() {
			return false
		}
		// 累积输出文本并记录审计日志
		outputText.WriteString(delta)
		service.AppendModelContentAuditResponseText(c, delta)
		err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:         "response.output_text.delta",
			ItemID:       messageID,
			OutputIndex:  &messageOutputIndex,
			ContentIndex: &contentIndex,
			Delta:        delta,
		})
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		return true
	}

	// getToolState 获取或创建指定工具调用的跟踪状态。
	// 使用工具的 Index 字段作为映射键来追踪同一工具调用的多次增量更新。
	getToolState := func(tool dto.ToolCallResponse) *responsesToolCallState {
		index := 0
		if tool.Index != nil {
			index = *tool.Index
		}
		state := toolCalls[index]
		// 如果该索引不存在，创建新的工具调用状态
		if state == nil {
			callID := strings.TrimSpace(tool.ID)
			// 如果上游未提供调用 ID，则生成一个
			if callID == "" {
				callID = "call_" + responseID + "_" + strconv.Itoa(index)
			}
			state = &responsesToolCallState{
				ID:          "fc_" + callID,
				CallID:      callID,
				OutputIndex: nextOutputIndex,
			}
			nextOutputIndex++
			toolCalls[index] = state
		}
		// 用最新的工具调用信息更新状态
		if strings.TrimSpace(tool.ID) != "" {
			state.CallID = strings.TrimSpace(tool.ID)
			state.ID = "fc_" + state.CallID
		}
		if strings.TrimSpace(tool.Function.Name) != "" {
			state.Name = strings.TrimSpace(tool.Function.Name)
		}
		return state
	}

	// sendToolDelta 处理工具调用的增量数据。
	// 首次遇到某工具调用时发送 item.added 事件，之后发送参数增量事件。
	sendToolDelta := func(tool dto.ToolCallResponse) bool {
		// 确保 response.created 事件已发送
		if !sendCreated() {
			return false
		}
		state := getToolState(tool)
		// 首次遇到该工具调用，发送 item.added 事件
		if !state.Added {
			err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
				Type:        dto.ResponsesOutputTypeItemAdded,
				OutputIndex: &state.OutputIndex,
				Item: &dto.ResponsesOutput{
					Type:      "function_call",
					ID:        state.ID,
					Status:    "in_progress",
					CallId:    state.CallID,
					Name:      state.Name,
					Arguments: responsesArgumentsRaw(""),
				},
			})
			if err != nil {
				streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				return false
			}
			state.Added = true
		}
		// 无新增参数则跳过
		if tool.Function.Arguments == "" {
			return true
		}
		// 累积函数参数并发送增量事件
		state.Arguments += tool.Function.Arguments
		err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:   "response.function_call_arguments.delta",
			ItemID: state.ID,
			Delta:  tool.Function.Arguments,
		})
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		return true
	}

	// 使用流式扫描器逐块处理上游 Chat Completions SSE 数据
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		// 如果已有错误，停止处理
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}
		// logger.LogInfo(c, fmt.Sprintf("responses compatibility upstream chat stream body: %s", data))
		// 解析 Chat Completions 流式响应块
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			logger.LogError(c, "failed to unmarshal chat stream event for responses compatibility: "+err.Error())
			sr.Error(err)
			return
		}
		// 用上游返回的值更新响应元数据
		if chunk.Id != "" {
			responseID = chunk.Id
		}
		if chunk.Created != 0 {
			createdAt = chunk.Created
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		// 收集上游返回的 token 用量
		if service.ValidUsage(chunk.Usage) {
			usage = responsesUsageFromChat(chunk.Usage)
		}
		// 处理每个选择（通常只有一个）
		for _, choice := range chunk.Choices {
			// 处理推理内容的增量（如思考链）
			if reasoningDelta := choice.Delta.GetReasoningContent(); reasoningDelta != "" {
				reasonText.WriteString(reasoningDelta)
				service.AppendModelContentAuditReasoningText(c, reasoningDelta)
				// 如果配置了将推理内容作为正文输出，则发送推理增量文本
				if info.ChannelSetting.ThinkingToContent {
					if !sendTextDelta(reasoningDelta) {
						sr.Stop(streamErr)
						return
					}
				}
			}
			// 处理正文内容的增量
			if !sendTextDelta(choice.Delta.GetContentString()) {
				sr.Stop(streamErr)
				return
			}
			// 处理工具调用的增量
			for _, tool := range choice.Delta.ToolCalls {
				if !sendToolDelta(tool) {
					sr.Stop(streamErr)
					return
				}
			}
		}
	})

	// 流式处理完成后，检查是否有错误
	if streamErr != nil {
		return nil, streamErr
	}
	// 如果上游未返回用量数据，根据输出文本估算 token 用量
	if usage == nil || usage.TotalTokens == 0 {
		usageText := outputText.String()
		if !info.ChannelSetting.ThinkingToContent {
			usageText += reasonText.String()
		}
		usage = service.ResponseText2Usage(c, usageText, info.UpstreamModelName, info.GetEstimatePromptTokens())
	}

	// 确保至少发送了 response.created 事件（空响应场景）
	if !sendCreated() {
		return nil, streamErr
	}

	// 发送文本内容部件的完成事件
	if sentPart {
		text := outputText.String()
		// 发送 output_text.done 事件，携带完整文本
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:         "response.output_text.done",
			ItemID:       messageID,
			OutputIndex:  &messageOutputIndex,
			ContentIndex: &contentIndex,
			Delta:        text,
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
		// 发送 content_part.done 事件，标志内容部件结束
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:         "response.content_part.done",
			ItemID:       messageID,
			OutputIndex:  &messageOutputIndex,
			ContentIndex: &contentIndex,
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "output_text",
				Text: text,
			},
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}

	// 发送 message 条目的完成事件（item.done）
	if sentMessage {
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:        dto.ResponsesOutputTypeItemDone,
			OutputIndex: &messageOutputIndex,
			Item: &dto.ResponsesOutput{
				Type:   "message",
				ID:     messageID,
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{
					{
						Type:        "output_text",
						Text:        outputText.String(),
						Annotations: []interface{}{},
					},
				},
			},
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}

	// 发送所有工具调用的完成事件
	for _, state := range toolCalls {
		// 发送函数参数完成事件
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:   "response.function_call_arguments.done",
			ItemID: state.ID,
			Delta:  state.Arguments,
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
		// 发送工具调用条目完成事件（item.done）
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:        dto.ResponsesOutputTypeItemDone,
			OutputIndex: &state.OutputIndex,
			Item: &dto.ResponsesOutput{
				Type:      "function_call",
				ID:        state.ID,
				Status:    "completed",
				CallId:    state.CallID,
				Name:      state.Name,
				Arguments: responsesArgumentsRaw(state.Arguments),
			},
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}

	// 发送最终的 response.completed 事件，携带完整的响应对象
	if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
		Type:     "response.completed",
		Response: chatStreamToResponsesResponse(responseID, createdAt, model, outputText.String(), usage, sentMessage, messageOutputIndex, toolCalls),
	}); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	helper.Done(c) // 标记流式响应结束
	// 设置内容审计文本
	service.SetModelContentAuditResponseText(c, outputText.String())
	service.SetModelContentAuditReasoningText(c, reasonText.String())
	return usage, nil
}

// OaiChatToResponsesHandler 将上游 Chat Completions 非流式响应
// 转换为 Responses API 非流式响应格式并发送给客户端。
func OaiChatToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, chatResp *dto.OpenAITextResponse) (*dto.Usage, *types.NewAPIError) {
	// 校验响应是否有效
	if chatResp == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	// 提取响应 ID，为空时从上下文中获取
	responseID := chatResp.Id
	if responseID == "" {
		responseID = helper.GetResponseID(c)
	}

	// 处理创建时间，兼容不同的数值类型
	createdAt := time.Now().Unix()
	switch created := chatResp.Created.(type) {
	case int64:
		createdAt = created
	case int:
		createdAt = int64(created)
	case float64:
		createdAt = int64(created)
	}

	// 提取模型名称，为空时从上游信息中获取
	model := chatResp.Model
	if model == "" && info != nil {
		model = info.UpstreamModelName
	}

	// 提取文本内容和推理内容
	text := ""
	reasoningText := ""
	if len(chatResp.Choices) > 0 {
		text = chatResp.Choices[0].Message.StringContent()
		reasoningText = chatResp.Choices[0].Message.GetReasoningContent()
		// 如果配置了将推理内容合并到正文输出
		if info != nil && info.ChannelSetting.ThinkingToContent && reasoningText != "" {
			text = reasoningText + text
		}
	}

	// 转换用量信息并构建 Responses 响应对象
	usage := responsesUsageFromChat(&chatResp.Usage)
	responsesResp := chatStreamToResponsesResponse(responseID, createdAt, model, text, usage, text != "", 0, nil)

	// 序列化并发送给客户端
	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	// logger.LogInfo(c, fmt.Sprintf("responses compatibility converted response body: %s", string(responseBody)))
	service.IOCopyBytesGracefully(c, nil, responseBody)
	// 设置内容审计文本
	service.SetModelContentAuditResponseText(c, text)
	service.SetModelContentAuditReasoningText(c, reasoningText)
	return usage, nil
}
