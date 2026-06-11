package openaicompat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

// ResponsesRequestToChatCompletionsRequest 将 OpenAI Responses API 请求转换为 Chat Completions API 请求。
// 这是 ChatCompletionsRequestToResponsesRequest 的逆操作。
func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	// 参数校验：请求不能为空
	if req == nil {
		return nil, errors.New("request is nil")
	}
	// 参数校验：模型名称不能为空
	if req.Model == "" {
		return nil, errors.New("model is required")
	}

	// 将 Responses API 的 input 数组转换为 Chat Completions 的 messages 数组
	messages, err := convertResponsesInputToMessages(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert input to messages: %w", err)
	}

	// 构建 Chat Completions 请求对象，映射通用字段
	chatReq := &dto.GeneralOpenAIRequest{
		Model:       req.Model,       // 模型名称
		Messages:    messages,        // 消息列表
		Stream:      req.Stream,      // 是否流式输出
		Temperature: req.Temperature, // 温度参数，控制随机性
		TopP:        req.TopP,        // Top-P 采样参数
		User:        req.User,        // 用户标识
		Metadata:    req.Metadata,    // 元数据
		Store:       req.Store,       // 是否存储
	}

	// 映射 max_output_tokens -> max_completion_tokens
	if req.MaxOutputTokens != nil {
		chatReq.MaxCompletionTokens = req.MaxOutputTokens
	}

	// 映射 reasoning（推理）参数，提取 effort 字段
	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		chatReq.ReasoningEffort = req.Reasoning.Effort
	}

	// 映射工具列表。Chat Completions 仅接受 function 类型工具；
	// Responses 专用工具类型（如 web_search、namespace）无法直接转发。
	if len(req.Tools) > 0 {
		tools, err := convertResponsesTools(req.Tools)
		if err == nil && len(tools) > 0 {
			chatReq.Tools = tools
		}
	}

	// 映射工具选择策略（tool_choice），仅在有可用工具时设置
	if len(req.ToolChoice) > 0 && len(chatReq.Tools) > 0 {
		chatReq.ToolChoice = json.RawMessage(req.ToolChoice)
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
	// 先执行基础转换
	chatReq, err := ResponsesRequestToChatCompletionsRequest(req)
	if err != nil {
		return nil, err
	}
	// 对工具输出消息进行规范化处理，确保上游供应商兼容性
	normalizeResponsesChatToolOutputMessages(chatReq.Messages)
	return chatReq, nil
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
// 解析为 Chat Completions 的 messages 数组。
func convertResponsesInputToMessages(req *dto.OpenAIResponsesRequest) ([]dto.Message, error) {
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
			messages = append(messages, dto.Message{
				Role:    "user",
				Content: inputStr,
			})
			return messages, nil
		}
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 遍历每个 input 条目，根据 type 字段进行分发处理
	for _, itemRaw := range inputItems {
		// 预读 "type" 字段以判断条目类型
		var peek struct {
			Type string `json:"type"`
		}
		_ = common.Unmarshal(itemRaw, &peek)

		switch peek.Type {
		case "message":
			// 处理普通消息条目
			msg, err := convertResponsesMessageItem(itemRaw)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)

		case "function_call":
			// 处理函数调用条目，转换为带 tool_calls 的 assistant 消息
			msg, err := convertResponsesFunctionCallItem(itemRaw)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)

		case "function_call_output":
			// 处理函数调用输出条目，转换为 tool 消息
			msg, err := convertResponsesFunctionCallOutputItem(itemRaw)
			if err != nil {
				continue
			}
			messages = append(messages, *msg)

		default:
			// 尝试解析为带有 role 字段的简单消息
			var simpleMsg struct {
				Role    string          `json:"role"`    // 消息角色
				Content json.RawMessage `json:"content"` // 消息内容
			}
			if err := common.Unmarshal(itemRaw, &simpleMsg); err == nil && simpleMsg.Role != "" {
				content := parseContentToChatFormat(simpleMsg.Content, simpleMsg.Role)
				messages = append(messages, dto.Message{
					Role:    simpleMsg.Role,
					Content: content,
				})
			}
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
	// 将 "developer" 角色映射为 "system"，兼容不支持 developer 角色的供应商
	if role == "developer" {
		role = "system"
	}

	// 解析内容为 Chat Completions 兼容格式
	content := parseContentToChatFormat(item.Content, item.Role)
	return &dto.Message{
		Role:    role,
		Content: content,
	}, nil
}

// convertResponsesFunctionCallItem 将 Responses API 的 function_call 条目
// 转换为包含 tool_calls 的 Chat Completions assistant 消息。
func convertResponsesFunctionCallItem(itemRaw json.RawMessage) (*dto.Message, error) {
	var item struct {
		Type      string `json:"type"`      // 条目类型
		ID        string `json:"id"`        // 条目 ID
		CallID    string `json:"call_id"`   // 函数调用 ID
		Name      string `json:"name"`      // 函数名称
		Arguments string `json:"arguments"` // 函数参数（JSON 字符串）
	}
	if err := common.Unmarshal(itemRaw, &item); err != nil {
		return nil, err
	}

	// 优先使用 call_id，若为空则回退到 id
	callID := item.CallID
	if callID == "" {
		callID = item.ID
	}

	// 构建 tool_calls 数组，符合 Chat Completions 的格式要求
	toolCalls := []map[string]any{
		{
			"id":   callID,
			"type": "function",
			"function": map[string]string{
				"name":      item.Name,
				"arguments": item.Arguments,
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

// parseContentToChatFormat 将 Responses API 的内容转换为 Chat Completions 的内容格式。
// Responses 的 content 可以是纯字符串，也可以是内容部件（content parts）数组。
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
		Type       string `json:"type"`        // 部件类型（input_text/output_text/input_image/input_audio）
		Text       string `json:"text"`        // 文本内容
		ImageURL   any    `json:"image_url"`   // 图片 URL
		InputAudio any    `json:"input_audio"` // 音频输入
	}
	if err := common.Unmarshal(contentRaw, &parts); err == nil {
		// 如果只有一个文本部件，直接返回字符串以简化结构
		if len(parts) == 1 && parts[0].Type == "input_text" {
			return parts[0].Text
		}
		if len(parts) == 1 && parts[0].Type == "output_text" {
			return parts[0].Text
		}

		// 转换为 MediaContent 数组，支持多媒体内容
		mediaParts := make([]dto.MediaContent, 0, len(parts))
		for _, p := range parts {
			switch p.Type {
			case "input_text", "output_text":
				// 文本内容部件
				mediaParts = append(mediaParts, dto.MediaContent{
					Type: dto.ContentTypeText,
					Text: p.Text,
				})
			case "input_image":
				// 图片内容部件
				mediaParts = append(mediaParts, dto.MediaContent{
					Type:     dto.ContentTypeImageURL,
					ImageUrl: p.ImageURL,
				})
			case "input_audio":
				// 音频内容部件
				mediaParts = append(mediaParts, dto.MediaContent{
					Type:       dto.ContentTypeInputAudio,
					InputAudio: p.InputAudio,
				})
			default:
				// 未知类型，尝试作为文本内容处理
				mediaParts = append(mediaParts, dto.MediaContent{
					Type: p.Type,
					Text: p.Text,
				})
			}
		}
		return mediaParts
	}

	// 兜底处理：返回原始 JSON 字符串
	return string(contentRaw)
}

// convertResponsesTools 将 Responses API 的工具定义转换为 Chat Completions 的工具列表。
// 仅保留 function 类型的工具，其他类型（如 web_search）会被过滤掉。
func convertResponsesTools(toolsRaw json.RawMessage) ([]dto.ToolCallRequest, error) {
	// 解析原始工具定义为通用 map 数组
	var rawTools []map[string]any
	if err := common.Unmarshal(toolsRaw, &rawTools); err != nil {
		return nil, err
	}

	// 遍历并筛选 function 类型的工具
	tools := make([]dto.ToolCallRequest, 0, len(rawTools))
	for _, rt := range rawTools {
		toolType, _ := rt["type"].(string)
		switch toolType {
		case "function":
			// 提取函数名称，名称为空则跳过
			name, _ := rt["name"].(string)
			if strings.TrimSpace(name) == "" {
				continue
			}
			// 提取函数描述和参数定义
			desc, _ := rt["description"].(string)
			params := rt["parameters"]

			tools = append(tools, dto.ToolCallRequest{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        name,   // 函数名称
					Description: desc,   // 函数描述
					Parameters:  params, // 函数参数 JSON Schema
				},
			})
		default:
			// 非 function 类型的工具（如 web_search）不支持转发，跳过
			continue
		}
	}
	return tools, nil
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
