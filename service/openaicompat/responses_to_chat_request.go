package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// ResponsesRequestToChatCompletionsRequest 将 OpenAI Responses API 请求转换为 Chat Completions API 请求。
// 这是 ChatCompletionsRequestToResponsesRequest 的逆操作。
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	return responsesRequestToChatCompletions(req, relaycommon.NewResponsesChatToolContext())
}

// responsesRequestToChatCompletions 执行实际的 Responses→Chat 转换，同时把 Responses 专用工具
// （function / custom / tool_search / namespace）的映射写入 ctx，供响应阶段恢复工具类型。
func responsesRequestToChatCompletions(req *dto.OpenAIResponsesRequest, ctx *relaycommon.ResponsesChatToolContext) (*dto.GeneralOpenAIRequest, error) {
	// 参数校验：请求不能为空
	if req == nil {
		return nil, errors.New("request is nil")
	}
	// 参数校验：模型名称不能为空
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	// 先构建工具上下文：解析顶层 tools 定义，并扫描 input 中的 tool_search_output
	// 收集动态加载的 namespace 工具。响应阶段依据该上下文恢复工具条目类型。
	buildResponsesChatToolContext(req, ctx)

	// 将 Responses API 的 input 数组和 instructions 解析为 Chat Completions 的 messages 数组。
	// 返回前把多条 system 消息合并到首位（MiniMax 等严格要求 system 只能出现在首位）。
	messages, err := convertResponsesInputToMessages(req, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to convert input to messages: %w", err)
	}
	messages = collapseSystemMessagesToHead(messages)

	// 构建 Chat Completions 请求对象，映射通用字段。
	// 注意：按 cc-switch 行为，不透传 `store`（chat/completions 无此字段，
	// 严格上游会拒），stream/temperature/top_p/user/metadata 直接透传。
	chatReq := &dto.GeneralOpenAIRequest{
		Model:       req.Model,       // 模型名称
		Messages:    messages,        // 消息列表
		Stream:      req.Stream,      // 是否流式输出
		Temperature: req.Temperature, // 温度参数，控制随机性
		TopP:        req.TopP,        // Top-P 采样参数
		User:        req.User,        // 用户标识
		Metadata:    req.Metadata,    // 元数据
	}

	// 映射 max_output_tokens：o 系列 / gpt-5 → max_completion_tokens，其余 → max_tokens。
	// 非思考型模型发 max_completion_tokens 会被部分严格上游（vLLM、老网关）拒绝。
	if req.MaxOutputTokens != nil {
		if isOpenOSeriesOrGpt5(req.Model) {
			chatReq.MaxCompletionTokens = req.MaxOutputTokens
		} else {
			chatReq.MaxTokens = req.MaxOutputTokens
		}
	}

	// 透传 service_tier（参考 cc-switch EXTRA_CHAT_PASSTHROUGH_FIELDS）。
	// 注意：默认会被 RemoveDisabledFields 过滤，需通道开启 allow_service_tier 才会真正发往上游。
	if req.ServiceTier != "" {
		if b, err := common.Marshal(req.ServiceTier); err == nil {
			chatReq.ServiceTier = b
		}
	}

	// 映射 reasoning（推理）参数，提取 effort 字段
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}

	// 映射工具列表。工具上下文已在上面构建（buildResponsesChatToolContext），
	// 这里按上下文里注册的 chat function 顺序生成工具定义。
	if !ctx.IsEmpty() {
		tools := chatToolsFromContext(ctx)
		if len(tools) > 0 {
			chatReq.Tools = tools
		}
	}

	// 映射工具选择策略（tool_choice）：function/custom/tool_search/namespace 都归一成 chat function choice。
	if len(req.ToolChoice) > 0 && len(chatReq.Tools) > 0 {
		if choice := convertResponsesToolChoice(req.ToolChoice, ctx); choice != nil {
			chatReq.ToolChoice = choice
		}
	}

	// 映射 parallel_tool_calls（是否允许并行工具调用）
	if len(req.ParallelToolCalls) > 0 {
		var parallel bool // 是否允许并行调用的布尔值
		if err := common.Unmarshal(req.ParallelToolCalls, &parallel); err == nil {
			chatReq.ParallelTooCalls = &parallel
		}
	}

	// 映射 text 字段到 response_format（响应格式）
	if len(req.Text) > 0 {
		chatReq.ResponseFormat = convertResponsesTextToResponseFormat(req.Text)
	}

	return chatReq, nil
}

// ResponsesRequestToChatCompletionsCompatRequest 将 Responses API 请求转换为 Chat Completions 兼容请求。
// 适用于仅支持 Chat Completions 而不支持 Responses API 的上游供应商。
func ResponsesRequestToChatCompletionsCompatRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	chatReq, _, err := ResponsesRequestToChatCompletionsCompatRequestWithContext(req)
	return chatReq, err
}

// ResponsesRequestToChatCompletionsCompatRequestWithContext 在执行兼容转换的同时，
// 返回请求阶段构造的工具上下文。Responses→Chat 渠道用该上下文在响应阶段恢复
// function/custom/tool_search/namespace 工具类型。
func ResponsesRequestToChatCompletionsCompatRequestWithContext(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, *relaycommon.ResponsesChatToolContext, error) {
	ctx := relaycommon.NewResponsesChatToolContext()
	// 先执行基础转换
	chatReq, err := responsesRequestToChatCompletions(req, ctx)
	if err != nil {
		return nil, nil, err
	}
	// 对工具输出消息进行规范化处理，确保上游供应商兼容性
	normalizeResponsesChatToolOutputMessages(chatReq.Messages)
	return chatReq, ctx, nil
}

// normalizeResponsesChatToolOutputMessages 规范化工具输出消息。
// 某些上游供应商会拒绝包含 "Exit code:" 的工具输出内容，
// 而 Codex 仅需要命令状态的语义信息，因此将 "Exit code:" 替换为 "Command status:"。
func normalizeResponsesChatToolOutputMessages(messages []dto.Message) {
	for i := range messages {
		// 仅处理 role 为 "tool" 且内容为字符串类型的消息
		if messages[i].Role != "tool" || !messages[i].IsStringContent() {
			continue
		}
		// 将 "Exit code:" 替换为 "Command status:" 以提高上游兼容性
		messages[i].Content = strings.ReplaceAll(messages[i].StringContent(), "Exit code:", "Command status:")
	}
}

// convertResponsesInputToMessages 将 Responses API 的 input 数组和 instructions
// 解析为 Chat Completions 的 messages 数组。ctx 用于把 function_call/custom_tool_call/
// tool_search_call 历史项里的工具名还原为 chat function 名（与工具定义保持一致）。
func convertResponsesInputToMessages(req *dto.OpenAIResponsesRequest, ctx *relaycommon.ResponsesChatToolContext) ([]dto.Message, error) {
	var messages []dto.Message

	// 将 instructions（指令）作为 system 消息添加到消息列表头部
	if len(req.Instructions) > 0 {
		var instructions string
		if err := common.Unmarshal(req.Instructions, &instructions); err == nil && strings.TrimSpace(instructions) != "" {
			messages = append(messages, dto.Message{
				Role:    "system",
				Content: instructions,
			})
		}
	}

	// 解析 input 数组，如果为空则直接返回
	if len(req.Input) == 0 {
		return messages, nil
	}

	// 尝试将 input 解析为 JSON 数组
	var inputItems []json.RawMessage
	if err := common.Unmarshal(req.Input, &inputItems); err != nil {
		// 如果不是数组，尝试作为纯字符串处理
		var inputStr string
		if err2 := common.Unmarshal(req.Input, &inputStr); err2 == nil {
			if strings.TrimSpace(inputStr) != "" {
				messages = append(messages, dto.Message{
					Role:    "user",
					Content: inputStr,
				})
			}
			return messages, nil
		}
		// 既非数组也非字符串：尝试作为单个对象处理（包成单元素数组）
		var objMap map[string]any
		if err3 := common.Unmarshal(req.Input, &objMap); err3 == nil {
			inputItems = []json.RawMessage{req.Input}
		} else {
			return nil, fmt.Errorf("failed to parse input: %w", err)
		}
	}

	// pendingReasoning 累积顶层 reasoning 条目的文本，前向附挂到其后生成的 assistant 消息
	// （对应 cc-switch pending_reasoning 语义：思考型模型多轮历史需要保留 reasoning_content）。
	var pendingReasoning strings.Builder

	// 遍历每个 input 条目，根据 type 字段进行分发处理
	for _, itemRaw := range inputItems {
		// 预读 "type" / "role" 字段以判断条目类型
		var peek struct {
			Type string `json:"type"`
			Role string `json:"role"`
		}
		_ = common.Unmarshal(itemRaw, &peek)

		// 远程压缩协议条目显式过滤（不依赖 default 分支的惰性忽略）：
		// - compaction_trigger：codex 压缩 v2 的请求侧触发器，仅在中继层消费，不进入 chat messages
		// - additional_tools：codex desktop responses-lite 的运行时工具载体，chat 上游无此概念
		// 压缩摘要重放（{"type":"compaction"/"compaction_summary"/"context_compaction",
		// "encrypted_content":"ocx1:..."}）保序转换为 summary user 消息，见下方 case。
		switch peek.Type {
		case "compaction_trigger", "additional_tools":
			continue
		}

		switch peek.Type {
		case "message":
			// 处理普通消息条目
			msg, err := convertResponsesMessageItem(itemRaw)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)
			if msg.Role == "assistant" {
				consumePendingReasoning(messages, &pendingReasoning)
			}

		case "function_call":
			// 处理函数调用条目，转换为带 tool_calls 的 assistant 消息
			msg, err := convertResponsesFunctionCallItem(itemRaw, ctx)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)
			consumePendingReasoning(messages, &pendingReasoning)

		case "custom_tool_call":
			// custom 工具调用历史：chat function 名与原始名一致，arguments 包装成 {input: ...}
			msg, err := convertResponsesCustomToolCallItem(itemRaw)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)
			consumePendingReasoning(messages, &pendingReasoning)

		case "tool_search_call":
			// tool_search 调用历史：chat function 名固定为 tool_search
			msg, err := convertResponsesToolSearchCallItem(itemRaw)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)
			consumePendingReasoning(messages, &pendingReasoning)

		case "function_call_output":
			// 处理函数调用输出条目，转换为 tool 消息
			msg, err := convertResponsesFunctionCallOutputItem(itemRaw)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)

		case "custom_tool_call_output", "tool_search_output":
			// custom/tool_search 输出历史：cc-switch 把整个条目规范化成 JSON 字符串作为 tool 消息内容
			msg, err := convertResponsesRawToolCallOutputItem(itemRaw)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)

		case "input_text", "input_image", "input_file", "input_audio", "text", "output_text":
			// 顶层 content-part 条目（不包在 message 内）：按 cc-switch 行为，
			// 包成单元素数组交给 parseContentToChatFormat，role 取条目 role 或默认 user。
			role := normalizeResponsesRole(peek.Role)
			wrapped := make([]byte, 0, len(itemRaw)+2)
			wrapped = append(wrapped, '[')
			wrapped = append(wrapped, itemRaw...)
			wrapped = append(wrapped, ']')
			content := parseContentToChatFormat(wrapped, role)
			messages = append(messages, dto.Message{
				Role:    role,
				Content: content,
			})

		case "reasoning":
			// 顶层 reasoning 条目：累积 summary 文本，待下一条 assistant 消息生成时附挂为其 reasoning_content。
			if r := extractReasoningItemText(itemRaw); r != "" {
				if pendingReasoning.Len() > 0 {
					pendingReasoning.WriteString("\n\n")
				}
				pendingReasoning.WriteString(r)
			}

		case "compaction", "compaction_summary", "context_compaction":
			// 压缩摘要重放：codex 客户端把之前压缩返回的 compaction item 原样带回。
			// 保序就地转换为 summary user 消息（不最后统一追加，保持 input 顺序）：
			// - ocx1: 信封 → 解码还原明文摘要（SUMMARY_PREFIX + 摘要）
			// - 无前缀的加密 blob（真正 OpenAI 压缩内容）→ 不可读占位提示
			// context_compaction 无 encrypted_content 时（纯本地压缩标记，摘要紧随其后的
			// user 消息）跳过，避免产生空消息。
			var compactItem struct {
				EncryptedContent string `json:"encrypted_content"`
			}
			if err := common.Unmarshal(itemRaw, &compactItem); err != nil {
				continue
			}
			if peek.Type == "context_compaction" && compactItem.EncryptedContent == "" {
				continue
			}
			// compaction item 是历史边界：其后的 reasoning 不应再回溯附挂到摘要之前的内容。
			pendingReasoning.Reset()
			messages = append(messages, dto.Message{
				Role:    "user",
				Content: relaycommon.CompactionItemToMessageText(compactItem.EncryptedContent),
			})

		default:
			// 尝试解析为带有 role 字段的简单消息
			var simpleMsg struct {
				Role    string          `json:"role"`    // 消息角色
				Content json.RawMessage `json:"content"` // 消息内容
			}
			if err := common.Unmarshal(itemRaw, &simpleMsg); err == nil && simpleMsg.Role != "" {
				content := parseContentToChatFormat(simpleMsg.Content, simpleMsg.Role)
				messages = append(messages, dto.Message{
					Role:    normalizeResponsesRole(simpleMsg.Role),
					Content: content,
				})
			}
		}
	}

	// 收尾：仍未消费的 reasoning（其后没有再出现 assistant）回溯附挂到最后一条 assistant 消息
	if remaining := strings.TrimSpace(pendingReasoning.String()); remaining != "" {
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role != "assistant" {
				continue
			}
			existing := ""
			if messages[i].ReasoningContent != nil {
				existing = strings.TrimSpace(*messages[i].ReasoningContent)
			}
			if existing != "" {
				remaining = existing + "\n\n" + remaining
			}
			messages[i].ReasoningContent = &remaining
			break
		}
	}

	return messages, nil
}

// convertResponsesMessageItem 将 Responses API 的 message 条目转换为 Chat Completions 的 message。
func convertResponsesMessageItem(itemRaw json.RawMessage) (*dto.Message, error) {
	var item struct {
		Type    string          `json:"type"`    // 条目类型
		Role    string          `json:"role"`    // 消息角色（user/assistant/developer）
		Content json.RawMessage `json:"content"` // 消息内容
	}
	if err := common.Unmarshal(itemRaw, &item); err != nil {
		return nil, err
	}

	role := item.Role
	// 规范化角色：developer→system、latest_reminder/未知→user 等，兼容上游角色枚举
	role = normalizeResponsesRole(role)

	// 解析内容为 Chat Completions 兼容格式
	content := parseContentToChatFormat(item.Content, item.Role)
	return &dto.Message{
		Role:    role,
		Content: content,
	}, nil
}

// convertResponsesFunctionCallItem 将 Responses API 的 function_call 条目
// 转换为包含 tool_calls 的 Chat Completions assistant 消息。
// namespace 历史项通过 ctx 还原为 chat function 名（对齐 cc-switch responses_function_call_to_chat_tool_call）。
func convertResponsesFunctionCallItem(itemRaw json.RawMessage, ctx *relaycommon.ResponsesChatToolContext) (*dto.Message, error) {
	var item struct {
		Type      string          `json:"type"`      // 条目类型
		ID        string          `json:"id"`        // 条目 ID
		CallID    string          `json:"call_id"`   // 函数调用 ID
		Name      string          `json:"name"`      // 函数名称
		Namespace string          `json:"namespace"` // 命名空间（namespace 工具）
		Arguments json.RawMessage `json:"arguments"` // 函数参数（JSON 字符串或对象）
	}
	if err := common.Unmarshal(itemRaw, &item); err != nil {
		return nil, err
	}

	// 优先使用 call_id，若为空则回退到 id
	callID := item.CallID
	if callID == "" {
		callID = item.ID
	}
	// 通过 ctx 把 (name, namespace) 还原成与工具定义一致的 chat function 名
	chatName := ctx.ChatNameForResponseFunction(item.Name, item.Namespace)
	// arguments 规范化为 JSON 字符串
	arguments := canonicalToolArgumentsString(item.Arguments)

	// 构建 tool_calls 数组，符合 Chat Completions 的格式要求
	toolCalls := []map[string]any{
		{
			"id":   callID,
			"type": "function",
			"function": map[string]string{
				"name":      chatName,
				"arguments": arguments,
			},
		},
	}

	// 将 tool_calls 序列化为 JSON
	toolCallsJSON, _ := common.Marshal(toolCalls)
	return &dto.Message{
		Role:      "assistant",
		Content:   "",
		ToolCalls: toolCallsJSON,
	}, nil
}

// convertResponsesCustomToolCallItem 将 Responses API 的 custom_tool_call 条目转换成
// assistant tool_calls。chat function 名与原始名一致；input 包装成 {input: ...} JSON 字符串
// （对齐 cc-switch responses_custom_tool_call_to_chat_tool_call）。
func convertResponsesCustomToolCallItem(itemRaw json.RawMessage) (*dto.Message, error) {
	var item struct {
		Type   string          `json:"type"` // 条目类型
		ID     string          `json:"id"`   // 条目 ID
		CallID string          `json:"call_id"`
		Name   string          `json:"name"`
		Input  json.RawMessage `json:"input"`
	}
	if err := common.Unmarshal(itemRaw, &item); err != nil {
		return nil, err
	}
	callID := item.CallID
	if callID == "" {
		callID = item.ID
	}
	// input 缺省为空字符串
	input := json.RawMessage(`""`)
	if len(strings.TrimSpace(string(item.Input))) > 0 {
		input = item.Input
	}
	arguments, _ := common.Marshal(map[string]json.RawMessage{
		relaycommon.ResponsesChatCustomInputField(): input,
	})

	toolCalls := []map[string]any{
		{
			"id":   callID,
			"type": "function",
			"function": map[string]string{
				"name":      item.Name,
				"arguments": string(arguments),
			},
		},
	}
	toolCallsJSON, _ := common.Marshal(toolCalls)
	return &dto.Message{
		Role:      "assistant",
		Content:   "",
		ToolCalls: toolCallsJSON,
	}, nil
}

// convertResponsesToolSearchCallItem 将 Responses API 的 tool_search_call 条目转换成
// assistant tool_calls，function 名固定为 tool_search，arguments 规范化为 JSON 字符串
// （对齐 cc-switch responses_tool_search_call_to_chat_tool_call）。
func convertResponsesToolSearchCallItem(itemRaw json.RawMessage) (*dto.Message, error) {
	var item struct {
		Type      string          `json:"type"`
		ID        string          `json:"id"`
		CallID    string          `json:"call_id"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := common.Unmarshal(itemRaw, &item); err != nil {
		return nil, err
	}
	callID := item.CallID
	if callID == "" {
		callID = item.ID
	}
	arguments := canonicalToolArgumentsString(item.Arguments)
	if arguments == "" {
		arguments = "{}"
	}

	toolCalls := []map[string]any{
		{
			"id":   callID,
			"type": "function",
			"function": map[string]string{
				"name":      relaycommon.ResponsesChatToolSearchProxyName(),
				"arguments": arguments,
			},
		},
	}
	toolCallsJSON, _ := common.Marshal(toolCalls)
	return &dto.Message{
		Role:      "assistant",
		Content:   "",
		ToolCalls: toolCallsJSON,
	}, nil
}

// convertResponsesFunctionCallOutputItem 将 Responses API 的 function_call_output 条目
// 转换为 Chat Completions 的 tool 消息。
func convertResponsesFunctionCallOutputItem(itemRaw json.RawMessage) (*dto.Message, error) {
	var item struct {
		Type   string `json:"type"`    // 条目类型
		CallID string `json:"call_id"` // 对应的函数调用 ID
		Output string `json:"output"`  // 函数输出内容
	}
	if err := common.Unmarshal(itemRaw, &item); err != nil {
		return nil, err
	}

	return &dto.Message{
		Role:       "tool",      // 工具消息角色
		Content:    item.Output, // 函数输出内容
		ToolCallId: item.CallID, // 关联的函数调用 ID
	}, nil
}

// convertResponsesRawToolCallOutputItem 将 custom_tool_call_output / tool_search_output 条目
// 转换成 tool 消息：cc-switch 把整个条目规范化成 JSON 字符串作为 content（保留原始结构）
// （对齐 cc-switch custom_tool_call_output | tool_search_output 分支）。
func convertResponsesRawToolCallOutputItem(itemRaw json.RawMessage) (*dto.Message, error) {
	var item struct {
		CallID string `json:"call_id"`
	}
	if err := common.Unmarshal(itemRaw, &item); err != nil {
		return nil, err
	}
	content := canonicalJSONStringFromRaw(itemRaw)
	return &dto.Message{
		Role:       "tool",
		Content:    content,
		ToolCallId: item.CallID,
	}, nil
}

// canonicalToolArgumentsString 把 Responses 工具调用的 arguments 规整为 Chat Completions 要求的 JSON 字符串。
// 对齐 cc-switch canonicalize_tool_arguments：
//   - JSON 字符串值：取其内容再当作 JSON 解析规整（Responses 的 function_call.arguments 是"JSON 字符串包 JSON"，
//     直接 Marshal 会双重编码成带外层引号的字符串字面量，被严格上游以 "arguments must be a JSON object" 拒绝）；
//     内容为空 → "{}"；内容非合法 JSON → 原样返回。
//   - JSON 对象/数组等结构化值：直接规整序列化（tool_search_call.arguments 即对象）。
//   - null/缺失 → "{}"。
//
// 严格上游（Kimi-K3、Minimax 等）要求 assistant tool_call.arguments 必须是 JSON 对象，空串也会被 400 拒绝。
func canonicalToolArgumentsString(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return "{}"
	}
	var parsed any
	if err := common.Unmarshal(raw, &parsed); err != nil {
		return "{}"
	}
	// 字符串值：内容本身是 JSON 文本，需再次解析规整（避免双重编码）
	if strVal, ok := parsed.(string); ok {
		trimmed := strings.TrimSpace(strVal)
		if trimmed == "" {
			return "{}"
		}
		var inner any
		if err := common.Unmarshal([]byte(strVal), &inner); err != nil {
			return strVal // 内容非合法 JSON：原样返回（对齐 cc-switch fallback）
		}
		b, err := common.Marshal(inner)
		if err != nil {
			return "{}"
		}
		return string(b)
	}
	// 结构化值（对象/数组等）：直接规整序列化
	b, err := common.Marshal(parsed)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// canonicalJSONStringFromRaw 把任意 JSON 原始字节规整为紧凑 JSON 字符串（用于 tool 输出内容）。
func canonicalJSONStringFromRaw(raw json.RawMessage) string {
	var parsed any
	if err := common.Unmarshal(raw, &parsed); err == nil {
		if b, err := common.Marshal(parsed); err == nil {
			return string(b)
		}
	}
	return string(raw)
}

// parseContentToChatFormat 将 Responses API 的内容转换为 Chat Completions 的内容格式。
// Responses 的 content 可以是纯字符串，也可以是内容部件（content parts）数组。
// 按 cc-switch 行为：仅当存在非文本部件（图片/音频/文件）时才返回数组；否则把所有
// 文本部件用 "\n" 拼接成纯字符串，避免严格上游对 user 角色 string-only content 的校验失败。
func parseContentToChatFormat(contentRaw json.RawMessage, role string) any {
	if len(contentRaw) == 0 {
		return ""
	}

	// 首先尝试作为纯字符串解析
	var str string
	if err := common.Unmarshal(contentRaw, &str); err == nil {
		return str
	}

	// 尝试作为内容部件数组解析
	var parts []struct {
		Type       string          `json:"type"`        // 部件类型（input_text/output_text/input_image/input_audio/input_file/refusal）
		Text       string          `json:"text"`        // 文本内容
		Refusal    string          `json:"refusal"`     // 拒绝文本
		ImageURL   json.RawMessage `json:"image_url"`   // 图片 URL（字符串或 {url:...} 对象）
		InputAudio json.RawMessage `json:"input_audio"` // 音频输入对象 {data,format}
		FileID     string          `json:"file_id"`     // 文件 ID
		Filename   string          `json:"filename"`    // 文件名
	}
	if err := common.Unmarshal(contentRaw, &parts); err != nil {
		// 兜底处理：返回原始 JSON 字符串
		return string(contentRaw)
	}

	mediaParts := make([]dto.MediaContent, 0, len(parts))
	textOnly := make([]string, 0, len(parts))
	hasNonTextPart := false

	for _, p := range parts {
		switch p.Type {
		case "input_text", "output_text", "text":
			// 文本内容部件
			if p.Text != "" {
				mediaParts = append(mediaParts, dto.MediaContent{
					Type: dto.ContentTypeText,
					Text: p.Text,
				})
				textOnly = append(textOnly, p.Text)
			}
		case "refusal":
			// 拒绝文本当作普通文本处理
			if p.Refusal != "" {
				mediaParts = append(mediaParts, dto.MediaContent{
					Type: dto.ContentTypeText,
					Text: p.Refusal,
				})
				textOnly = append(textOnly, p.Refusal)
			}
		case "input_image":
			// 图片内容部件：Responses 的 image_url 是字符串，chat/completions 要求 {url:...} 对象
			imgUrl := normalizeImageURLRaw(p.ImageURL)
			if imgUrl != nil {
				mediaParts = append(mediaParts, dto.MediaContent{
					Type:     dto.ContentTypeImageURL,
					ImageUrl: imgUrl,
				})
				hasNonTextPart = true
			}
		case "input_audio":
			// 音频内容部件：input_audio 已是 {data,format} 对象，原样保留
			if len(p.InputAudio) > 0 {
				mediaParts = append(mediaParts, dto.MediaContent{
					Type:       dto.ContentTypeInputAudio,
					InputAudio: json.RawMessage(p.InputAudio),
				})
				hasNonTextPart = true
			}
		case "input_file":
			// 文件内容部件：保留 file_id + filename（参考 cc-switch chat_file_from_input_file）
			f := normalizeInputFileRaw(p.FileID, p.Filename)
			if f != nil {
				mediaParts = append(mediaParts, dto.MediaContent{
					Type: dto.ContentTypeFile,
					File: f,
				})
				hasNonTextPart = true
			}
		default:
			// 未知类型：带文本则作为文本部件，否则跳过
			if p.Text != "" {
				mediaParts = append(mediaParts, dto.MediaContent{
					Type: dto.ContentTypeText,
					Text: p.Text,
				})
				textOnly = append(textOnly, p.Text)
			}
		}
	}

	// 无媒体部件：折叠成纯字符串
	if !hasNonTextPart {
		return strings.Join(textOnly, "\n")
	}
	return mediaParts
}

// buildResponsesChatToolContext 解析 Responses 请求中的工具定义并注册到 ctx。
// 支持 function / custom / tool_search / namespace 四种 Codex 工具类型，
// 同时扫描 input 中的 tool_search_output 收集动态加载的 namespace 工具
// （对齐 cc-switch build_codex_tool_context_from_request）。
func buildResponsesChatToolContext(req *dto.OpenAIResponsesRequest, ctx *relaycommon.ResponsesChatToolContext) {
	if ctx == nil || req == nil {
		return
	}
	// 顶层 tools 定义
	if len(req.Tools) > 0 {
		var rawTools []json.RawMessage
		if err := common.Unmarshal(req.Tools, &rawTools); err == nil {
			for _, rawTool := range rawTools {
				addResponsesToolToContext(rawTool, "", ctx)
			}
		}
	}
	// input 中 tool_search_output 动态加载的工具
	if len(req.Input) > 0 {
		collectToolSearchOutputTools(req.Input, ctx)
	}
}

// addResponsesToolToContext 把单个 Responses 工具定义注册到 ctx。
// namespace 非空时，function 子工具归属该命名空间。
func addResponsesToolToContext(rawTool json.RawMessage, namespace string, ctx *relaycommon.ResponsesChatToolContext) {
	// 工具可能是裸字符串（视为 custom 工具名）
	var nameStr string
	if err := common.Unmarshal(rawTool, &nameStr); err == nil {
		if strings.TrimSpace(nameStr) != "" {
			addCustomToolToContext(nameStr, nil, ctx)
		}
		return
	}

	var tool map[string]any
	if err := common.Unmarshal(rawTool, &tool); err != nil {
		return
	}
	switch toolType, _ := tool["type"].(string); toolType {
	case "function":
		addFunctionToolToContext(tool, namespace, ctx)
	case "custom":
		if name, ok := responsesToolName(tool); ok {
			addCustomToolToContext(name, tool, ctx)
		}
	case "tool_search":
		addToolSearchToolToContext(ctx)
	case "namespace":
		addNamespaceToolToContext(tool, ctx)
	}
}

// addFunctionToolToContext 注册 function 工具。namespace 非空时归为 namespace 类型，
// chat 名按 namespace__name 规则拼接（对齐 cc-switch add_function_tool）。
func addFunctionToolToContext(tool map[string]any, namespace string, ctx *relaycommon.ResponsesChatToolContext) {
	originalName, ok := responsesToolName(tool)
	if !ok {
		return
	}
	var chatName string
	kind := relaycommon.ResponsesChatToolKindFunction
	if namespace != "" {
		chatName = relaycommon.FlattenResponsesNamespaceToolName(namespace, originalName)
		kind = relaycommon.ResponsesChatToolKindNamespace
	} else {
		chatName = originalName
	}
	chatTool := responsesFunctionToolToChatTool(tool, chatName)
	if chatTool == nil {
		return
	}
	spec := &relaycommon.ResponsesChatToolSpec{Kind: kind, Name: originalName, Namespace: namespace}
	if ctx.AddChatTool(chatName, spec) {
		ctx.ChatTools = append(ctx.ChatTools, *chatTool)
	}
}

// addCustomToolToContext 注册 custom 工具。chat 名直接用原始名（对齐 cc-switch add_custom_tool），
// parameters 固定为 {input: string}，description 内嵌原始工具定义以便上游理解输入语义。
func addCustomToolToContext(name string, tool map[string]any, ctx *relaycommon.ResponsesChatToolContext) {
	if strings.TrimSpace(name) == "" {
		return
	}
	chatTool := dto.ToolCallRequest{
		Type: "function",
		Function: dto.FunctionRequest{
			Name:        name,
			Description: responsesCustomToolDescription(tool),
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					relaycommon.ResponsesChatCustomInputField(): map[string]any{
						"type":        "string",
						"description": "Raw string input for the original custom tool. Preserve formatting exactly and follow the original tool definition embedded in the description.",
					},
				},
				"required": []string{relaycommon.ResponsesChatCustomInputField()},
			},
		},
	}
	spec := &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindCustom, Name: name}
	if ctx.AddChatTool(name, spec) {
		ctx.ChatTools = append(ctx.ChatTools, chatTool)
	}
}

// addToolSearchToolToContext 注册固定的 tool_search 代理工具（对齐 cc-switch add_tool_search_tool）。
func addToolSearchToolToContext(ctx *relaycommon.ResponsesChatToolContext) {
	name := relaycommon.ResponsesChatToolSearchProxyName()
	chatTool := dto.ToolCallRequest{
		Type: "function",
		Function: dto.FunctionRequest{
			Name:        name,
			Description: "Search and load Codex tools, plugins, connectors, and MCP namespaces for the current task.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Search query for tools or connectors to load.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of tool groups to return.",
					},
				},
				"required": []string{"query"},
			},
		},
	}
	spec := &relaycommon.ResponsesChatToolSpec{Kind: relaycommon.ResponsesChatToolKindToolSearch, Name: name}
	if ctx.AddChatTool(name, spec) {
		ctx.ChatTools = append(ctx.ChatTools, chatTool)
	}
}

// addNamespaceToolToContext 展开命名空间工具：遍历其 tools/children，
// 把每个 function 子工具按所属命名空间注册（对齐 cc-switch add_namespace_tool）。
func addNamespaceToolToContext(namespaceTool map[string]any, ctx *relaycommon.ResponsesChatToolContext) {
	namespace, _ := namespaceTool["name"].(string)
	if strings.TrimSpace(namespace) == "" {
		return
	}
	childrenAny, _ := namespaceTool["tools"]
	if childrenAny == nil {
		childrenAny = namespaceTool["children"]
	}
	children, ok := childrenAny.([]any)
	if !ok {
		return
	}
	for _, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := childMap["type"].(string); t == "function" {
			addFunctionToolToContext(childMap, namespace, ctx)
		}
	}
}

// collectToolSearchOutputTools 递归扫描 input，把 tool_search_output 中加载的工具
// 注册到 ctx（对齐 cc-switch collect_tool_search_output_tools）。
func collectToolSearchOutputTools(value json.RawMessage, ctx *relaycommon.ResponsesChatToolContext) {
	var arr []json.RawMessage
	if err := common.Unmarshal(value, &arr); err == nil {
		for _, item := range arr {
			collectToolSearchOutputTools(item, ctx)
		}
		return
	}
	var obj map[string]json.RawMessage
	if err := common.Unmarshal(value, &obj); err != nil {
		return
	}
	if t, ok := obj["type"]; ok {
		var typeStr string
		if common.Unmarshal(t, &typeStr) == nil && typeStr == "tool_search_output" {
			if toolsRaw, ok := obj["tools"]; ok {
				var tools []json.RawMessage
				if common.Unmarshal(toolsRaw, &tools) == nil {
					for _, tool := range tools {
						addResponsesToolToContext(tool, "", ctx)
					}
				}
			}
		}
	}
	for _, v := range obj {
		collectToolSearchOutputTools(v, ctx)
	}
}

// chatToolsFromContext 按 ctx 注册顺序输出 Chat Completions 工具定义。
func chatToolsFromContext(ctx *relaycommon.ResponsesChatToolContext) []dto.ToolCallRequest {
	if ctx == nil {
		return nil
	}
	return ctx.ChatTools
}

// responsesToolName 从 Responses 工具定义中提取名称（兼容 function.name / 顶层 name）。
// 对齐 cc-switch responses_tool_name：仅返回非空 trim 后的名称。
func responsesToolName(tool map[string]any) (string, bool) {
	if tool == nil {
		return "", false
	}
	// function 工具可能是 {type:function, function:{name:...}} 或扁平 {type:function, name:...}
	if fn, ok := tool["function"].(map[string]any); ok {
		if name, ok := fn["name"].(string); ok {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				return trimmed, true
			}
		}
	}
	if name, ok := tool["name"].(string); ok {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed, true
		}
	}
	return "", false
}

// responsesFunctionToolToChatTool 把 Responses function 工具转为 Chat Completions function 工具。
// 强制 parameters.type=object（对齐 cc-switch responses_function_tool_to_chat_tool + normalize）。
func responsesFunctionToolToChatTool(tool map[string]any, chatName string) *dto.ToolCallRequest {
	// 嵌套形式：{type:function, function:{...}}
	if fn, ok := tool["function"].(map[string]any); ok {
		fnCopy := make(map[string]any, len(fn))
		for k, v := range fn {
			fnCopy[k] = v
		}
		fnCopy["name"] = chatName
		fnCopy["parameters"] = normalizeFunctionParameters(fnCopy["parameters"])
		if strict, ok := tool["strict"]; ok {
			if _, exists := fnCopy["strict"]; !exists {
				fnCopy["strict"] = strict
			}
		}
		return &dto.ToolCallRequest{Type: "function", Function: dto.FunctionRequest{
			Name:        chatName,
			Description: stringFromAny(fnCopy["description"]),
			Parameters:  fnCopy["parameters"],
		}}
	}
	// 扁平形式：{type:function, name, description, parameters}
	params := normalizeFunctionParameters(tool["parameters"])
	return &dto.ToolCallRequest{Type: "function", Function: dto.FunctionRequest{
		Name:        chatName,
		Description: stringFromAny(tool["description"]),
		Parameters:  params,
	}}
}

// responsesCustomToolDescription 构造 custom 工具的 description，内嵌原始工具定义
// （对齐 cc-switch responses_custom_tool_description）。
func responsesCustomToolDescription(tool map[string]any) string {
	def := "{}"
	if tool != nil {
		if b, err := common.Marshal(tool); err == nil {
			def = string(b)
		}
	}
	return "Original tool definition:\n```json\n" + def + "\n```"
}

// stringFromAny 安全地把 any 转成 string。
func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// convertResponsesToolChoice 把 Responses tool_choice 转成 Chat Completions tool_choice。
// function/custom/tool_search/namespace 全部归一成 chat function choice（对齐 cc-switch）。
func convertResponsesToolChoice(toolChoiceRaw json.RawMessage, ctx *relaycommon.ResponsesChatToolContext) json.RawMessage {
	var tc map[string]any
	if err := common.Unmarshal(toolChoiceRaw, &tc); err != nil {
		// 非对象（如 "auto"/"none"/"required"）原样透传
		return toolChoiceRaw
	}
	tcType, _ := tc["type"].(string)
	switch tcType {
	case "function":
		name, _ := tc["name"].(string)
		namespace, _ := tc["namespace"].(string)
		chatName := ctx.ChatNameForResponseFunction(name, namespace)
		out := map[string]any{
			"type":     "function",
			"function": map[string]any{"name": chatName},
		}
		b, _ := common.Marshal(out)
		return b
	case "tool_search":
		out := map[string]any{
			"type":     "function",
			"function": map[string]any{"name": relaycommon.ResponsesChatToolSearchProxyName()},
		}
		b, _ := common.Marshal(out)
		return b
	case "custom":
		name, _ := tc["name"].(string)
		out := map[string]any{
			"type":     "function",
			"function": map[string]any{"name": name},
		}
		b, _ := common.Marshal(out)
		return b
	default:
		return toolChoiceRaw
	}
}

// convertResponsesTextToResponseFormat 将 Responses API 的 "text" 字段
// （包含格式信息）转换为 Chat Completions 的 response_format。
func convertResponsesTextToResponseFormat(textRaw json.RawMessage) *dto.ResponseFormat {
	var text struct {
		Format map[string]any `json:"format"` // 格式定义
	}
	if err := common.Unmarshal(textRaw, &text); err != nil || text.Format == nil {
		return nil
	}

	// 提取格式类型
	formatType, _ := text.Format["type"].(string)
	if formatType == "" {
		return nil
	}

	rf := &dto.ResponseFormat{
		Type: formatType,
	}

	// 处理 json_schema 格式：从 format.json_schema 字段提取 schema
	if formatType == "json_schema" {
		if schema, ok := text.Format["json_schema"]; ok {
			schemaBytes, err := common.Marshal(schema)
			if err == nil {
				rf.JsonSchema = schemaBytes
			}
		}
	}

	// 处理顶层 schema 字段（部分供应商的嵌套方式不同）
	if schema, ok := text.Format["schema"]; ok && rf.JsonSchema == nil {
		schemaBytes, err := common.Marshal(schema)
		if err == nil {
			rf.JsonSchema = schemaBytes
		}
	}

	return rf
}

// normalizeResponsesRole 将 Responses API 的角色归一化为 Chat Completions 兼容角色。
// developer→system，latest_reminder/空/未知→user，其余保持。对齐 cc-switch 行为。
func normalizeResponsesRole(role string) string {
	switch role {
	case "system", "developer":
		return "system"
	case "assistant":
		return "assistant"
	case "tool":
		return "tool"
	default:
		return "user"
	}
}

// collapseSystemMessagesToHead 把所有字符串内容的 system 消息合并到消息列表首位。
// MiniMax 等严格要求 system 只能出现在首位（否则 invalid message role: system），
// 该重排对 OpenAI / DeepSeek 等宽松兼容层也是无损的。非字符串内容的 system 原地保留。
func collapseSystemMessagesToHead(messages []dto.Message) []dto.Message {
	var systemChunks []string
	rest := make([]dto.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" {
			if s, ok := msg.Content.(string); ok {
				if strings.TrimSpace(s) != "" {
					systemChunks = append(systemChunks, s)
				}
				continue
			}
		}
		rest = append(rest, msg)
	}
	out := make([]dto.Message, 0, len(rest)+1)
	if len(systemChunks) > 0 {
		out = append(out, dto.Message{
			Role:    "system",
			Content: strings.Join(systemChunks, "\n\n"),
		})
	}
	return append(out, rest...)
}

// normalizeFunctionParameters 强制 function 工具的 parameters 为 {"type":"object",...}。
// OpenAI Chat Completions 严格要求 parameters 为 object 类型；null/缺 type 会被严格上游拒绝。
func normalizeFunctionParameters(raw any) any {
	obj, ok := raw.(map[string]any)
	if !ok {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}
	out := make(map[string]any, len(obj))
	for k, v := range obj {
		out[k] = v
	}
	if t, _ := out["type"].(string); t != "object" {
		out["type"] = "object"
	}
	return out
}

// isOpenOSeriesOrGpt5 判断模型是否为 OpenAI o 系列或 gpt-5 系列，
// 这类模型用 max_completion_tokens 而非 max_tokens。
func isOpenOSeriesOrGpt5(model string) bool {
	return strings.HasPrefix(model, "o") || strings.HasPrefix(model, "gpt-5")
}

// normalizeImageURLRaw 将 Responses 的 input_image.image_url 规范化为 chat/completions 要求的对象。
// 字符串（data URL 或 http URL）→ {"url": "..."}；已是对象则原样保留；空/null → nil。
func normalizeImageURLRaw(raw json.RawMessage) any {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return nil
	}
	// 对象形式：原样保留
	if s[0] == '{' {
		var obj map[string]any
		if err := common.Unmarshal(raw, &obj); err == nil && len(obj) > 0 {
			return obj
		}
	}
	// 字符串形式：包成 {"url": "..."}
	var str string
	if err := common.Unmarshal(raw, &str); err == nil && str != "" {
		return map[string]any{"url": str}
	}
	return nil
}

// normalizeInputFileRaw 将 Responses 的 input_file 部件归一化为 chat/completions 的 file 对象。
// 仅保留 file_id + filename（参考 cc-switch chat_file_from_input_file，丢弃 file_url）。
func normalizeInputFileRaw(fileID, filename string) any {
	if fileID == "" && filename == "" {
		return nil
	}
	f := map[string]any{}
	if fileID != "" {
		f["file_id"] = fileID
	}
	if filename != "" {
		f["filename"] = filename
	}
	return f
}

// extractReasoningItemText 从 Responses 的顶层 reasoning 条目中提取 summary 文本。
// reasoning 条目形如 {"type":"reasoning","summary":[{"type":"summary_text","text":"..."}]}。
func extractReasoningItemText(itemRaw json.RawMessage) string {
	var item struct {
		Summary []struct {
			Text string `json:"text"`
		} `json:"summary"`
	}
	if err := common.Unmarshal(itemRaw, &item); err != nil {
		return ""
	}
	var texts []string
	for _, s := range item.Summary {
		if strings.TrimSpace(s.Text) != "" {
			texts = append(texts, s.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// consumePendingReasoning 把累积的 reasoning 附挂到列表最后一条 assistant 消息的 reasoning_content，
// 并清空缓冲；若最后一条不是 assistant 则保留缓冲，等待后续 assistant（对应 cc-switch 前向附挂语义）。
func consumePendingReasoning(messages []dto.Message, pending *strings.Builder) {
	text := strings.TrimSpace(pending.String())
	if text == "" || len(messages) == 0 {
		return
	}
	last := &messages[len(messages)-1]
	if last.Role != "assistant" {
		return
	}
	if last.ReasoningContent != nil && strings.TrimSpace(*last.ReasoningContent) != "" {
		text = *last.ReasoningContent + "\n\n" + text
	}
	last.ReasoningContent = &text
	pending.Reset()
}
