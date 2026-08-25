package openai

import (
	"encoding/json"
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
	ID          string                             // 工具调用条目的唯一标识（前缀 "fc_" 或 "ctc_"）
	CallID      string                             // 原始调用 ID（通常来自上游）
	Name        string                             // chat function 名称（上游返回）
	Spec        *relaycommon.ResponsesChatToolSpec // 经工具上下文解析出的原始 Responses 工具规范（nil 表示未知，按普通 function_call 处理）
	Arguments   string                             // 累积的函数参数 JSON 字符串
	OutputIndex int                                // 该工具调用在 Responses output 数组中的位置索引
	Added       bool                               // 是否已发送 item.added 事件
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

// responsesToolSpec 经 Responses→Chat 工具上下文解析 chat function 名对应的原始工具规范。
// 无上下文或名称未知时返回 nil，调用方按普通 function_call 处理。
func responsesToolSpec(info *relaycommon.RelayInfo, chatName string) *relaycommon.ResponsesChatToolSpec {
	if info == nil || info.ResponsesChatToolCtx == nil {
		return nil
	}
	return info.ResponsesChatToolCtx.Lookup(chatName)
}

// responsesToolItemID 返回恢复后工具条目的 ID：custom 工具用 "ctc_" 前缀，其余用 "fc_"。
// 对齐 cc-switch response_tool_call_item_id_from_chat_name。
func responsesToolItemID(callID string, spec *relaycommon.ResponsesChatToolSpec) string {
	if spec != nil && spec.Kind == relaycommon.ResponsesChatToolKindCustom {
		return "ctc_" + callID
	}
	return "fc_" + callID
}

// responsesIsCustomToolSpec 判断 spec 是否为 custom 工具（流式阶段不发 arguments.delta，改发 input 事件）。
func responsesIsCustomToolSpec(spec *relaycommon.ResponsesChatToolSpec) bool {
	return spec != nil && spec.Kind == relaycommon.ResponsesChatToolKindCustom
}

// responsesToolItemFromChat 根据解析出的 spec 把 chat function_call 还原成 Responses 工具条目。
// spec 决定条目类型：tool_search_call / custom_tool_call / (namespace) function_call；
// spec 为 nil 时回退为普通 function_call（用 chat 名）。reasoning 非空时附挂 reasoning_content。
// 对齐 cc-switch response_tool_call_item_from_chat_name。
func responsesToolItemFromChat(itemID, status, callID, chatName, arguments, reasoning string, spec *relaycommon.ResponsesChatToolSpec) dto.ResponsesOutput {
	switch {
	case spec != nil && spec.Kind == relaycommon.ResponsesChatToolKindToolSearch:
		// tool_search_call：无 id/name，arguments 为对象，execution 固定 "client"。
		return dto.ResponsesOutput{
			Type:             "tool_search_call",
			Status:           status,
			CallId:           callID,
			Execution:        "client",
			Arguments:        responsesToolSearchArgumentsRaw(arguments),
			ReasoningContent: reasoning,
		}
	case spec != nil && spec.Kind == relaycommon.ResponsesChatToolKindCustom:
		// custom_tool_call：从 chat arguments 解包出原始 input 字符串。
		return dto.ResponsesOutput{
			Type:             "custom_tool_call",
			ID:               itemID,
			Status:           status,
			CallId:           callID,
			Name:             spec.Name,
			Input:            responsesCustomToolInputFromChat(arguments),
			ReasoningContent: reasoning,
		}
	case spec != nil && (spec.Kind == relaycommon.ResponsesChatToolKindNamespace || spec.Kind == relaycommon.ResponsesChatToolKindFunction):
		// function_call（可能带 namespace）：用原始工具名，namespace 非空时附带。
		return dto.ResponsesOutput{
			Type:             "function_call",
			ID:               itemID,
			Status:           status,
			CallId:           callID,
			Name:             spec.Name,
			Arguments:        responsesArgumentsRaw(arguments),
			Namespace:        spec.Namespace,
			ReasoningContent: reasoning,
		}
	default:
		// 未知 chat 名：回退为普通 function_call，沿用 chat 名。
		return dto.ResponsesOutput{
			Type:             "function_call",
			ID:               itemID,
			Status:           status,
			CallId:           callID,
			Name:             chatName,
			Arguments:        responsesArgumentsRaw(arguments),
			ReasoningContent: reasoning,
		}
	}
}

// responsesToolSearchArgumentsRaw 把 chat 参数字符串解析为 tool_search_call 要求的对象形式。
// 空 → {}；合法 JSON 对象 → 该对象；否则 → {query: arguments}。对齐 cc-switch parse_tool_arguments_object。
func responsesToolSearchArgumentsRaw(arguments string) json.RawMessage {
	s := strings.TrimSpace(arguments)
	if s == "" {
		return json.RawMessage(`{}`)
	}
	var obj map[string]any
	if err := common.UnmarshalJsonStr(s, &obj); err == nil {
		if b, err := common.Marshal(obj); err == nil {
			return b
		}
	}
	out := map[string]any{"query": arguments}
	if b, err := common.Marshal(out); err == nil {
		return b
	}
	return json.RawMessage(`{}`)
}

// responsesCustomToolInputFromChat 从 chat arguments 中解包 custom 工具的原始 input 字符串。
// 空 → ""；含 input 字符串字段 → 该字符串；否则 → 原始 arguments。对齐 cc-switch custom_tool_input_from_chat_arguments。
func responsesCustomToolInputFromChat(arguments string) string {
	s := strings.TrimSpace(arguments)
	if s == "" {
		return ""
	}
	var obj map[string]any
	if err := common.UnmarshalJsonStr(s, &obj); err == nil {
		if input, ok := obj[relaycommon.ResponsesChatCustomInputField()].(string); ok {
			return input
		}
	}
	return arguments
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
	// [4] chat→response 后响应体：转换后发给客户端的 Responses 流式事件
	// helper.DumpResponsesCompatSection(c, helper.ResponsesCompatDumpResponseAfter, []byte("event: "+event.Type+"\ndata: "+string(data)+"\n"))
	return nil
}

// chatStreamToResponsesResponse 根据流式处理过程中收集的数据，
// 构建一个完整的 Responses API 非流式响应对象。
// 按索引顺序排列 reasoning / message / function_call 输出条目。
func chatStreamToResponsesResponse(responseID string, createdAt int64, model string, text string, usage *dto.Usage, sentMessage bool, messageOutputIndex int, toolCalls map[int]*responsesToolCallState, reasoningText string, reasoningOutputIndex int) *dto.OpenAIResponsesResponse {
	// 按索引构建输出条目映射
	outputByIndex := map[int]dto.ResponsesOutput{}
	// 若有推理内容，构建 reasoning 输出条目（位于 message 之前）
	maxOutputIndex := -1
	if reasoningText != "" && reasoningOutputIndex >= 0 {
		outputByIndex[reasoningOutputIndex] = dto.ResponsesOutput{
			Type:   "reasoning",
			ID:     "rs_" + responseID,
			Status: "completed",
			Summary: []dto.ResponsesReasoningSummaryPart{
				{Type: "summary_text", Text: reasoningText},
			},
		}
		if reasoningOutputIndex > maxOutputIndex {
			maxOutputIndex = reasoningOutputIndex
		}
	}
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
		if messageOutputIndex > maxOutputIndex {
			maxOutputIndex = messageOutputIndex
		}
	}
	// 将工具调用按其 OutputIndex 放入映射：经工具上下文恢复成 function_call /
	// custom_tool_call / tool_search_call / namespace function_call（对齐 cc-switch）。
	for _, state := range toolCalls {
		outputByIndex[state.OutputIndex] = responsesToolItemFromChat(
			state.ID, "completed", state.CallID, state.Name, state.Arguments, reasoningText, state.Spec,
		)
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
		// reasoning 输出条目状态（仅当上游返回 reasoning_content 且未合并进正文时使用）
		reasoningStarted     bool   // 是否已发送 reasoning 条目的 item.added
		reasoningOutputIndex int    // reasoning 条目在输出数组中的索引
		reasoningItemID      string // reasoning 条目 ID（rs_<responseID>）
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

	// sendReasoningStart 发送 reasoning 条目的 item.added 与 summary_part.added（仅一次）。
	sendReasoningStart := func() bool {
		if reasoningStarted {
			return true
		}
		if !sendCreated() {
			return false
		}
		reasoningOutputIndex = nextOutputIndex
		nextOutputIndex++
		reasoningItemID = "rs_" + responseID
		reasoningStarted = true
		// 发送 reasoning 条目的 item.added 事件
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:        dto.ResponsesOutputTypeItemAdded,
			OutputIndex: &reasoningOutputIndex,
			Item: &dto.ResponsesOutput{
				Type:   "reasoning",
				ID:     reasoningItemID,
				Status: "in_progress",
			},
		}); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		// 发送 summary_part.added 事件
		summaryIndex := 0
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:         "response.reasoning_summary_part.added",
			ItemID:       reasoningItemID,
			OutputIndex:  &reasoningOutputIndex,
			SummaryIndex: &summaryIndex,
			Part:         &dto.ResponsesReasoningSummaryPart{Type: "summary_text"},
		}); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		return true
	}

	// sendReasoningDelta 发送 reasoning summary_text 增量事件。
	sendReasoningDelta := func(delta string) bool {
		if delta == "" {
			return true
		}
		if !sendReasoningStart() {
			return false
		}
		summaryIndex := 0
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:         "response.reasoning_summary_text.delta",
			ItemID:       reasoningItemID,
			OutputIndex:  &reasoningOutputIndex,
			SummaryIndex: &summaryIndex,
			Delta:        delta,
		}); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		return true
	}

	// getToolState 获取或创建指定工具调用的跟踪状态。
	// 使用工具的 Index 字段作为映射键来追踪同一工具调用的多次增量更新。
	// 注意：item ID（fc_/ctc_ 前缀）与 spec 在 sendToolDelta 发送 item.added 时才解析锁定，
	// 因为它们依赖 chat function 名（可能晚于 call_id 到达）。
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
				CallID:      callID,
				OutputIndex: nextOutputIndex,
			}
			nextOutputIndex++
			toolCalls[index] = state
		}
		// 用最新的工具调用信息更新状态（call_id / name 可能分多块到达）。
		// 不在此处设置 state.ID：前缀（fc_/ctc_）依赖 spec，由 sendToolDelta 在 item.added 时锁定。
		if id := strings.TrimSpace(tool.ID); id != "" {
			state.CallID = id
		}
		if name := strings.TrimSpace(tool.Function.Name); name != "" {
			state.Name = name
		}
		return state
	}

	// sendToolDelta 处理工具调用的增量数据。
	// 首次遇到某工具调用（且 name 已知）时发送 item.added 事件，之后发送参数增量事件。
	// custom 工具不发 function_call_arguments.delta，改在 finalize 阶段发 custom_tool_call_input 事件
	// （对齐 cc-switch streaming_codex_chat）。
	sendToolDelta := func(tool dto.ToolCallResponse) bool {
		// 确保 response.created 事件已发送
		if !sendCreated() {
			return false
		}
		state := getToolState(tool)
		// 先累积参数（即便尚未发送 item.added，name 可能晚到）
		if tool.Function.Arguments != "" {
			state.Arguments += tool.Function.Arguments
		}
		// 首次处理该工具调用：等待 name 到达后解析 spec 并发送 item.added（spec 依赖 name）
		if !state.Added {
			if strings.TrimSpace(state.Name) == "" {
				return true
			}
			spec := responsesToolSpec(info, state.Name)
			state.Spec = spec
			state.ID = responsesToolItemID(state.CallID, spec)
			addedItem := responsesToolItemFromChat(state.ID, "in_progress", state.CallID, state.Name, "", "", spec)
			if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
				Type:        dto.ResponsesOutputTypeItemAdded,
				OutputIndex: &state.OutputIndex,
				Item:        &addedItem,
			}); err != nil {
				streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				return false
			}
			state.Added = true
			// 非 custom 工具：把 name 到达前已累积的参数作为首个 delta 批量发出。
			if state.Arguments != "" && !responsesIsCustomToolSpec(spec) {
				if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
					Type:        "response.function_call_arguments.delta",
					ItemID:      state.ID,
					OutputIndex: &state.OutputIndex,
					Delta:       state.Arguments,
				}); err != nil {
					streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
					return false
				}
			}
			return true
		}
		// 已发送 item.added：custom 工具不发 arguments.delta
		if responsesIsCustomToolSpec(state.Spec) {
			return true
		}
		if tool.Function.Arguments == "" {
			return true
		}
		// 发送参数增量事件
		err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:        "response.function_call_arguments.delta",
			ItemID:      state.ID,
			OutputIndex: &state.OutputIndex,
			Delta:       tool.Function.Arguments,
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
		// [3] chat→response 前响应体：上游 Chat Completions 流式块（逐块累积）
		//helper.DumpResponsesCompatSection(c, helper.ResponsesCompatDumpResponseBefore, []byte(data+"\n"))
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
				} else {
					// 否则作为独立 reasoning 条目的 summary_text 增量输出
					if !sendReasoningDelta(reasoningDelta) {
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

	// 关闭 reasoning 条目（位于 message 之前）：summary_text.done + summary_part.done + item.done
	if reasoningStarted {
		summaryIndex := 0
		fullReasoning := reasonText.String()
		// 发送 summary_text.done 事件，携带完整推理文本
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:         "response.reasoning_summary_text.done",
			ItemID:       reasoningItemID,
			OutputIndex:  &reasoningOutputIndex,
			SummaryIndex: &summaryIndex,
			Delta:        fullReasoning,
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
		// 发送 summary_part.done 事件
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:         "response.reasoning_summary_part.done",
			ItemID:       reasoningItemID,
			OutputIndex:  &reasoningOutputIndex,
			SummaryIndex: &summaryIndex,
			Part:         &dto.ResponsesReasoningSummaryPart{Type: "summary_text", Text: fullReasoning},
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
		// 发送 reasoning 条目的 item.done 事件
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:        dto.ResponsesOutputTypeItemDone,
			OutputIndex: &reasoningOutputIndex,
			Item: &dto.ResponsesOutput{
				Type:   "reasoning",
				ID:     reasoningItemID,
				Status: "completed",
				Summary: []dto.ResponsesReasoningSummaryPart{
					{Type: "summary_text", Text: fullReasoning},
				},
			},
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
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

	// 发送所有工具调用的完成事件。
	// 经工具上下文恢复类型：custom 工具发 custom_tool_call_input.delta/.done，
	// 其余发 function_call_arguments.done，最后统一发 item.done（对齐 cc-switch finalize_tools）。
	toolReasoning := reasonText.String()
	for _, state := range toolCalls {
		// 防御：跳过缺少 name 的工具调用（部分模型可能产出无名 tool_call）
		if strings.TrimSpace(state.Name) == "" {
			continue
		}
		// 兜底：极少数情况下 item.added 尚未发送（如 name 在最后一块才到达），在此补发并锁定 spec/ID。
		if !state.Added {
			spec := responsesToolSpec(info, state.Name)
			state.Spec = spec
			state.ID = responsesToolItemID(state.CallID, spec)
			addedItem := responsesToolItemFromChat(state.ID, "in_progress", state.CallID, state.Name, "", toolReasoning, spec)
			if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
				Type:        dto.ResponsesOutputTypeItemAdded,
				OutputIndex: &state.OutputIndex,
				Item:        &addedItem,
			}); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			}
			state.Added = true
		}
		// 构建完成条目（类型按 spec 还原，附挂推理内容）
		doneItem := responsesToolItemFromChat(state.ID, "completed", state.CallID, state.Name, state.Arguments, toolReasoning, state.Spec)
		if responsesIsCustomToolSpec(state.Spec) {
			// custom 工具：从 chat arguments 解包 input，发 custom_tool_call_input.delta/.done
			input := responsesCustomToolInputFromChat(state.Arguments)
			if input != "" {
				if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
					Type:        "response.custom_tool_call_input.delta",
					ItemID:      state.ID,
					OutputIndex: &state.OutputIndex,
					Delta:       input,
				}); err != nil {
					return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				}
			}
			if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
				Type:        "response.custom_tool_call_input.done",
				ItemID:      state.ID,
				OutputIndex: &state.OutputIndex,
				Input:       input,
			}); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			}
		} else {
			// 非 custom：发 function_call_arguments.done（携带完整参数）
			if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
				Type:        "response.function_call_arguments.done",
				ItemID:      state.ID,
				OutputIndex: &state.OutputIndex,
				Arguments:   state.Arguments,
			}); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			}
		}
		// 发送工具调用条目完成事件（item.done）
		if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
			Type:        dto.ResponsesOutputTypeItemDone,
			OutputIndex: &state.OutputIndex,
			Item:        &doneItem,
		}); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}

	// 发送最终的 response.completed 事件，携带完整的响应对象
	finalReasoningText := ""
	finalReasoningIdx := -1
	if reasoningStarted {
		finalReasoningText = reasonText.String()
		finalReasoningIdx = reasoningOutputIndex
	}
	responsesResp := chatStreamToResponsesResponse(responseID, createdAt, model, outputText.String(), usage, sentMessage, messageOutputIndex, toolCalls, finalReasoningText, finalReasoningIdx)
	if info != nil && info.RelayFormat == types.RelayFormatOpenAIResponses {
		service.RememberResponsesChatToolHistory(responsesResp.ID, responsesResp.Output)
	}
	if err := sendResponsesEvent(c, dto.ResponsesStreamResponse{
		Type:     "response.completed",
		Response: responsesResp,
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
	// 推理内容：仅当未合并进正文时，作为独立 reasoning 条目输出（位于 message 之前，索引 0）
	reasoningItemText := ""
	reasoningIdx := -1
	msgIdx := 0
	if info != nil && !info.ChannelSetting.ThinkingToContent && reasoningText != "" {
		reasoningItemText = reasoningText
		reasoningIdx = 0
		msgIdx = 1
	}
	// 解析 chat tool_calls 并经工具上下文恢复成 function_call / custom_tool_call /
	// tool_search_call / namespace function_call（对齐 cc-switch chat_tool_calls_to_response_output_items）。
	toolCalls := map[int]*responsesToolCallState{}
	if len(chatResp.Choices) > 0 {
		parsed := chatResp.Choices[0].Message.ParseToolCalls()
		toolStartIdx := msgIdx
		if text != "" {
			toolStartIdx = msgIdx + 1 // message 条目占用 msgIdx，工具紧随其后
		}
		placed := 0
		for _, tc := range parsed {
			name := strings.TrimSpace(tc.Function.Name)
			// 防御：跳过缺少 name 的 tool_call（部分模型可能产出无名调用）
			if name == "" {
				continue
			}
			callID := strings.TrimSpace(tc.ID)
			if callID == "" {
				callID = fmt.Sprintf("call_%d", placed)
			}
			spec := responsesToolSpec(info, name)
			toolCalls[placed] = &responsesToolCallState{
				ID:          responsesToolItemID(callID, spec),
				CallID:      callID,
				Name:        name,
				Spec:        spec,
				Arguments:   tc.Function.Arguments,
				OutputIndex: toolStartIdx + placed,
			}
			placed++
		}
	}
	responsesResp := chatStreamToResponsesResponse(responseID, createdAt, model, text, usage, text != "", msgIdx, toolCalls, reasoningItemText, reasoningIdx)
	if info != nil && info.RelayFormat == types.RelayFormatOpenAIResponses {
		service.RememberResponsesChatToolHistory(responsesResp.ID, responsesResp.Output)
	}

	// 序列化并发送给客户端
	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	// logger.LogInfo(c, fmt.Sprintf("responses compatibility converted response body: %s", string(responseBody)))
	// [4] chat→response 后响应体：转换后发给客户端的 Responses 响应（非流式）
	// helper.DumpResponsesCompatSection(c, helper.ResponsesCompatDumpResponseAfter, responseBody)
	service.IOCopyBytesGracefully(c, nil, responseBody)
	// 设置内容审计文本
	service.SetModelContentAuditResponseText(c, text)
	service.SetModelContentAuditReasoningText(c, reasoningText)
	return usage, nil
}
