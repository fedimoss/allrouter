package service

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// 模型内容审计链路（Model Content Audit Chain）
// =============================================================================
//
// 整体链路分为 4 段：
//
//   【第 1 段】请求入口（各协议 handler）
//       OpenAI 兼容接口 → TextHelper (relay/compatible_handler.go)
//       Claude 接口      → ClaudeHelper (relay/claude_handler.go)
//       Gemini 接口      → GeminiHelper (relay/gemini_handler.go)
//
//     每个入口在请求处理开始时会：
//       1. 将请求消息整理成审计用的字符串（如 ModelContentAuditOpenAIRequestMessages）
//       2. 注册 defer，确保请求结束时执行 EnqueueModelContentAuditFromRelay
//
//   【第 2 段】响应内容收集（请求处理过程中）
//       - 非流式：上游返回完整内容后，直接调用 SetModelContentAuditResponseText / SetModelContentAuditReasoningText
//         将完整文本存入 gin.Context
//       - 流式：每收到一个 delta，调用 AppendModelContentAuditResponseText / AppendModelContentAuditReasoningText
//         内部逻辑：先从 gin.Context 取出旧内容 → 拼接新 delta → 存回 gin.Context
//
//     关键点：流式内容在 gin.Context 里逐段拼接，不是直接进队列。
//
//   【第 3 段】请求结束后入队（defer 执行）
//       EnqueueModelContentAuditFromRelay 从 gin.Context 取出已拼好的完整 response/reasoning，
//       组装成一条完整的 ModelContentAuditRecord，然后调用 EnqueueModelContentAudit 放入队列。
//       队列里放的是完整记录，不是流式 delta。
//
//   【第 4 段】后台写 CSV
//       第一次 EnqueueModelContentAudit 调用时通过 sync.Once 启动后台 writer goroutine。
//       后台 goroutine 从队列逐条取记录，写入当天 CSV 文件（带重试）。
//       needModelContentAuditHeader 只通过文件大小判断是否需要写 header：
//         - 文件不存在 → 需要 header
//         - 文件大小为 0 → 需要 header
//         - 文件大小 > 0 → 不需要 header
//
// =============================================================================

const (
	// modelContentAuditQueueSize 审计记录队列容量，超出时丢弃新记录并记录日志
	modelContentAuditQueueSize = 5000
	// modelContentAuditDefaultDir 审计 CSV 文件默认存储目录
	modelContentAuditDefaultDir = "docs/modelcontent"
	// modelContentAuditWriteRetries 写入失败时的最大重试次数
	modelContentAuditWriteRetries = 3600
	// modelContentAuditWriteRetryInterval 写入失败后的重试间隔
	modelContentAuditWriteRetryInterval = time.Second
)

var (
	// modelContentAuditQueue 审计记录缓冲队列，第 3 段入队，第 4 段后台消费
	modelContentAuditQueue = make(chan ModelContentAuditRecord, modelContentAuditQueueSize)
	// modelContentAuditOnce 保证后台 writer goroutine 只启动一次（懒启动，首次入队时触发）
	modelContentAuditOnce sync.Once
)

// ModelContentAuditRecord 一条完整的模型内容审计记录。
// 在第 3 段（请求结束后入队）时组装，包含该次请求的所有关键信息。
type ModelContentAuditRecord struct {
	CreatedAt       string // 记录创建时间（RFC3339Nano 格式）
	RequestId       string // 请求唯一标识
	UserId          int    // 用户 ID
	BaseModel       string // 原始模型名称
	RequestMessages string // 审计用的请求内容（JSON 字符串）
	ResponseText    string // 完整响应文本（流式请求在 gin.Context 中拼接完成后取出）
	ReasoningText   string // 完整推理文本（reasoning/thinking 内容）
	Status          string // 请求状态："success" 或 "failed"
	ErrorMessage    string // 失败时的错误信息
}

// =============================================================================
// 第 3 段：入队
// =============================================================================

// EnqueueModelContentAudit 将一条完整的审计记录放入队列。
// 首次调用时会通过 sync.Once 启动后台 CSV writer goroutine（第 4 段）。
// 如果队列已满，丢弃新记录并记录日志（非阻塞）。
func EnqueueModelContentAudit(record ModelContentAuditRecord) {
	// 懒启动：第一次入队时启动后台 writer goroutine
	modelContentAuditOnce.Do(startModelContentAuditWriter)

	// 补全时间戳（如果调用方未设置）
	if strings.TrimSpace(record.CreatedAt) == "" {
		record.CreatedAt = time.Now().Format(time.RFC3339Nano)
	}

	// 非阻塞写入队列，队列满时丢弃并记录日志
	select {
	case modelContentAuditQueue <- record:
	default:
		common.SysLog(fmt.Sprintf("model content audit queue full, dropped request_id=%s", record.RequestId))
	}
}

// EnqueueModelContentAuditFromRelay 请求结束时由 defer 调用（第 3 段入口）。
// 从 gin.Context 中取出在第 2 段收集好的完整 response/reasoning 文本，
// 与请求入口准备好的 requestMessages 一起组装成完整的审计记录并入队。
//
// 参数说明：
//   - c:         gin.Context，用于读取响应文本和 HTTP 状态码
//   - info:      中继信息，包含 RequestId、UserId、OriginModelName 等
//   - requestMessages: 第 1 段中从请求体提取的审计请求内容（JSON 字符串）
//   - newAPIError:    请求处理过程中产生的错误，nil 表示成功
func EnqueueModelContentAuditFromRelay(c *gin.Context, info *relaycommon.RelayInfo, requestMessages string, newAPIError *types.NewAPIError) {
	if info == nil {
		return
	}

	// 判断请求状态：优先看 API 错误，其次看 HTTP 状态码
	status := "success"
	errorMessage := ""
	if newAPIError != nil {
		status = "failed"
		errorMessage = newAPIError.ErrorWithStatusCode()
		if errorMessage == "" {
			errorMessage = fmt.Sprintf("%T", newAPIError)
		}
	} else if c != nil && c.Writer.Status() >= 400 {
		status = "failed"
		errorMessage = fmt.Sprintf("status_code=%d", c.Writer.Status())
	}

	// 组装完整记录并入队：
	//   - RequestMessages 来自第 1 段（入口提取）
	//   - ResponseText / ReasoningText 来自第 2 段（从 gin.Context 取出，流式内容已拼接完整）
	EnqueueModelContentAudit(ModelContentAuditRecord{
		CreatedAt:       time.Now().Format(time.RFC3339Nano),
		RequestId:       info.RequestId,
		UserId:          info.UserId,
		BaseModel:       info.OriginModelName,
		RequestMessages: requestMessages,
		ResponseText:    GetModelContentAuditResponseText(c),
		ReasoningText:   GetModelContentAuditReasoningText(c),
		Status:          status,
		ErrorMessage:    errorMessage,
	})
}

// =============================================================================
// 第 2 段：响应内容收集（gin.Context 作为中间存储）
// =============================================================================
//
// 响应内容的收集使用 gin.Context 的 Keys 作为临时存储：
//   - 非流式：直接调用 Set* 设置完整内容
//   - 流式：  每收到一个 delta 调用 Append* 追加内容
//
// Append* 内部逻辑：先从 gin.Context 取出旧内容 → 拼接新文本 → 存回 gin.Context
// 这样就保证了无论流式还是非流式，请求结束时 gin.Context 里都有完整的内容。

// SetModelContentAuditResponseText 设置完整响应文本（非流式使用）。
// 直接覆盖 gin.Context 中已存储的响应文本。
func SetModelContentAuditResponseText(c *gin.Context, text string) {
	if c == nil || text == "" {
		return
	}
	common.SetContextKey(c, constant.ContextKeyModelContentAuditResponseText, text)
}

// AppendModelContentAuditResponseText 追加响应文本 delta（流式使用）。
// 内部逻辑：取出旧内容 → 拼接新 delta → 存回 gin.Context。
// 每次流式 chunk 到达时调用一次，逐步拼出完整响应。
func AppendModelContentAuditResponseText(c *gin.Context, text string) {
	if c == nil || text == "" {
		return
	}
	current := GetModelContentAuditResponseText(c)
	common.SetContextKey(c, constant.ContextKeyModelContentAuditResponseText, current+text)
}

// GetModelContentAuditResponseText 从 gin.Context 取出当前已收集的完整响应文本。
// 在第 3 段（EnqueueModelContentAuditFromRelay）中被调用，此时流式内容已拼接完毕。
func GetModelContentAuditResponseText(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return common.GetContextKeyString(c, constant.ContextKeyModelContentAuditResponseText)
}

// SetModelContentAuditReasoningText 设置完整推理文本（非流式使用）。
// 直接覆盖 gin.Context 中已存储的推理文本。
func SetModelContentAuditReasoningText(c *gin.Context, text string) {
	if c == nil || text == "" {
		return
	}
	common.SetContextKey(c, constant.ContextKeyModelContentAuditReasoningText, text)
}

// AppendModelContentAuditReasoningText 追加推理文本 delta（流式使用）。
// 内部逻辑：取出旧内容 → 拼接新 delta → 存回 gin.Context。
// 每次流式 chunk 中带有 reasoning/thinking 内容时调用一次。
func AppendModelContentAuditReasoningText(c *gin.Context, text string) {
	if c == nil || text == "" {
		return
	}
	current := GetModelContentAuditReasoningText(c)
	common.SetContextKey(c, constant.ContextKeyModelContentAuditReasoningText, current+text)
}

// GetModelContentAuditReasoningText 从 gin.Context 取出当前已收集的完整推理文本。
// 在第 3 段（EnqueueModelContentAuditFromRelay）中被调用，此时流式内容已拼接完毕。
func GetModelContentAuditReasoningText(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return common.GetContextKeyString(c, constant.ContextKeyModelContentAuditReasoningText)
}

// =============================================================================
// 第 1 段辅助函数：请求内容提取
// =============================================================================
//
// 每种协议有不同的请求结构，需要分别提取。
// 提取策略：
//   - OpenAI / Claude：从后往前找最后一个 user 角色的消息，优先只记录它（减少存储量）；
//     如果找不到 user 消息，则记录全部消息。
//   - Gemini：记录 system_instruction 和 contents。

// ModelContentAuditJSONString 将任意值序列化为 JSON 字符串，用于审计存储。
// 序列化失败时返回空字符串并记录日志。
func ModelContentAuditJSONString(value any) string {
	if value == nil {
		return ""
	}
	data, err := common.Marshal(value)
	if err != nil {
		common.SysLog("model content audit marshal request failed: " + err.Error())
		return ""
	}
	return string(data)
}

// ModelContentAuditOpenAIRequestMessages 从 OpenAI 请求中提取审计用的请求消息。
// 策略：从后往前找最后一条 user 角色消息，优先只记录它；
// 如果找不到 user 消息，则回退为记录全部消息。
func ModelContentAuditOpenAIRequestMessages(request *dto.GeneralOpenAIRequest) string {
	if request == nil {
		return ""
	}
	// 从后往前遍历，找最后一条 user 消息
	for i := len(request.Messages) - 1; i >= 0; i-- {
		message := request.Messages[i]
		if message.Role != "user" {
			continue
		}
		if content := modelContentAuditTextFromContent(message.Content); content != "" {
			return ModelContentAuditJSONString([]map[string]string{{
				"role":    message.Role,
				"content": content,
			}})
		}
	}
	// 回退：没有找到有效的 user 消息，记录全部消息
	return ModelContentAuditJSONString(request.Messages)
}

// ModelContentAuditClaudeRequestMessages 从 Claude 请求中提取审计用的请求消息。
// 策略同 OpenAI：从后往前找最后一条 user 消息，找不到则记录全部。
func ModelContentAuditClaudeRequestMessages(request *dto.ClaudeRequest) string {
	if request == nil {
		return ""
	}
	// 从后往前遍历，找最后一条 user 消息
	for i := len(request.Messages) - 1; i >= 0; i-- {
		message := request.Messages[i]
		if message.Role != "user" {
			continue
		}
		if content := modelContentAuditTextFromContent(message.Content); content != "" {
			return ModelContentAuditJSONString([]map[string]string{{
				"role":    message.Role,
				"content": content,
			}})
		}
	}
	// 回退：没有找到有效的 user 消息，记录全部消息
	return ModelContentAuditJSONString(request.Messages)
}

// ModelContentAuditGeminiRequestMessages 从 Gemini 请求中提取审计用的请求消息。
// Gemini 的请求结构与 OpenAI/Claude 不同，记录 system_instruction 和 contents。
func ModelContentAuditGeminiRequestMessages(request *dto.GeminiChatRequest) string {
	if request == nil {
		return ""
	}
	return ModelContentAuditJSONString(map[string]any{
		"system_instruction": request.SystemInstructions,
		"contents":           request.Contents,
	})
}

// =============================================================================
// 响应内容解析辅助函数
// =============================================================================

// ModelContentAuditOpenAIResponseText 从 OpenAI 响应结构体中提取纯文本响应内容。
func ModelContentAuditOpenAIResponseText(response *dto.OpenAITextResponse) string {
	responseText, _ := ModelContentAuditOpenAIResponseParts(response)
	return responseText
}

// ModelContentAuditOpenAIResponseParts 从 OpenAI 响应结构体中分别提取响应文本和推理文本。
// 遍历所有 choices，提取：
//   - responseText: 消息的字符串内容 + 工具调用的函数名和参数
//   - reasoningText: ReasoningContent 或 Reasoning 字段
func ModelContentAuditOpenAIResponseParts(response *dto.OpenAITextResponse) (string, string) {
	if response == nil {
		return "", ""
	}
	var responseBuilder strings.Builder
	var reasoningBuilder strings.Builder
	for _, choice := range response.Choices {
		if content := choice.Message.StringContent(); content != "" {
			responseBuilder.WriteString(content)
		}
		if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
			reasoningBuilder.WriteString(*choice.Message.ReasoningContent)
		}
		if choice.Message.Reasoning != nil && *choice.Message.Reasoning != "" {
			reasoningBuilder.WriteString(*choice.Message.Reasoning)
		}
		for _, tool := range choice.Message.ParseToolCalls() {
			responseBuilder.WriteString(tool.Function.Name)
			responseBuilder.WriteString(tool.Function.Arguments)
		}
	}
	return responseBuilder.String(), reasoningBuilder.String()
}

// ModelContentAuditResponseTextFromJSON 从 JSON 字节中提取响应文本（含推理文本）。
// 通用解析，不依赖具体响应结构体。
func ModelContentAuditResponseTextFromJSON(data []byte) string {
	responseText, reasoningText := ModelContentAuditResponsePartsFromJSON(data)
	return responseText + reasoningText
}

// ModelContentAuditResponsePartsFromJSON 从 JSON 字节中分别提取响应文本和推理文本。
// 先将 JSON 反序列化为通用 any 类型，再递归提取文本。
func ModelContentAuditResponsePartsFromJSON(data []byte) (string, string) {
	if len(data) == 0 {
		return "", ""
	}
	var value any
	if err := common.Unmarshal(data, &value); err != nil {
		common.SysLog("model content audit unmarshal response failed: " + err.Error())
		return "", ""
	}
	return modelContentAuditTextPartsFromValue(value)
}

func modelContentAuditTextFromValue(value any) string {
	responseText, reasoningText := modelContentAuditTextPartsFromValue(value)
	return responseText + reasoningText
}

// modelContentAuditTextPartsFromValue 递归遍历任意 JSON 结构，提取响应文本和推理文本。
// 提取规则：
//   - 字符串：视为响应文本
//   - map：递归查找 "text"/"output_text"/"completion" 键作为响应文本，
//     查找 "thinking"/"reasoning"/"reasoning_content" 键作为推理文本，
//     查找 "output"/"content"/"items"/"message"/"content_block"/"delta" 键继续递归
//   - 数组：遍历每个元素递归
func modelContentAuditTextPartsFromValue(value any) (string, string) {
	var responseBuilder strings.Builder
	var reasoningBuilder strings.Builder
	appendModelContentAuditTextParts(&responseBuilder, &reasoningBuilder, value)
	return responseBuilder.String(), reasoningBuilder.String()
}

func appendModelContentAuditTextParts(responseBuilder *strings.Builder, reasoningBuilder *strings.Builder, value any) {
	switch v := value.(type) {
	case string:
		responseBuilder.WriteString(v)
	case map[string]any:
		// 推理相关键 → 归入推理文本
		for _, key := range []string{"thinking", "reasoning", "reasoning_content"} {
			if text, ok := v[key].(string); ok {
				reasoningBuilder.WriteString(text)
			}
		}
		// 文本相关键 → 归入响应文本
		for _, key := range []string{"text", "output_text", "completion"} {
			if text, ok := v[key].(string); ok {
				responseBuilder.WriteString(text)
			}
		}
		// 嵌套结构键 → 继续递归
		for _, key := range []string{"output", "content", "items", "message", "content_block", "delta"} {
			if child, ok := v[key]; ok {
				appendModelContentAuditTextParts(responseBuilder, reasoningBuilder, child)
			}
		}
	case []any:
		for _, item := range v {
			appendModelContentAuditTextParts(responseBuilder, reasoningBuilder, item)
		}
	}
}

// =============================================================================
// 请求内容文本提取（用于从消息 Content 字段中提取纯文本）
// =============================================================================

// modelContentAuditTextFromContent 从消息的 Content 字段中提取纯文本字符串。
// Content 字段可能是 string、[]MediaContent、[]ClaudeMediaMessage、map 等多种类型。
// 提取后去除首尾空白并过滤 <system-reminder> 标签。
func modelContentAuditTextFromContent(content any) string {
	var builder strings.Builder
	appendModelContentAuditContentText(&builder, content)
	return strings.TrimSpace(builder.String())
}

// appendModelContentAuditContentText 递归遍历 Content 字段的各种可能类型，提取纯文本。
// 支持的 Content 类型：
//   - string: 清洗后直接提取
//   - []any: 遍历每个元素递归
//   - []dto.MediaContent: 提取 Text 字段
//   - []dto.ClaudeMediaMessage: 提取 GetText() 和 Content 字段
//   - map[string]any: 提取 "text" 和 "content" 键
//   - dto.MediaContent / dto.ClaudeMediaMessage: 单元素递归
func appendModelContentAuditContentText(builder *strings.Builder, content any) {
	switch v := content.(type) {
	case string:
		if text := modelContentAuditCleanText(v); text != "" {
			builder.WriteString(text)
		}
	case []any:
		for _, item := range v {
			appendModelContentAuditContentText(builder, item)
		}
	case []dto.MediaContent:
		for _, item := range v {
			appendModelContentAuditContentText(builder, item)
		}
	case []dto.ClaudeMediaMessage:
		for _, item := range v {
			appendModelContentAuditContentText(builder, item)
		}
	case map[string]any:
		if text, ok := v["text"].(string); ok {
			appendModelContentAuditContentText(builder, text)
		}
		if child, ok := v["content"]; ok {
			appendModelContentAuditContentText(builder, child)
		}
	case dto.MediaContent:
		appendModelContentAuditContentText(builder, v.Text)
	case dto.ClaudeMediaMessage:
		appendModelContentAuditContentText(builder, v.GetText())
		appendModelContentAuditContentText(builder, v.Content)
	}
}

// modelContentAuditCleanText 清洗文本内容，移除 <system-reminder> 标签及其内容。
// system-reminder 是系统注入的提示信息，不应出现在审计记录中。
func modelContentAuditCleanText(text string) string {
	const (
		startTag = "<system-reminder>"
		endTag   = "</system-reminder>"
	)
	for {
		start := strings.Index(text, startTag)
		if start < 0 {
			break
		}
		end := strings.Index(text[start+len(startTag):], endTag)
		if end < 0 {
			// 有开始标签但没有结束标签，截断到开始标签前
			text = text[:start]
			break
		}
		end += start + len(startTag) + len(endTag)
		text = text[:start] + text[end:]
	}
	return strings.TrimSpace(text)
}

// =============================================================================
// 第 4 段：后台 CSV Writer
// =============================================================================

// startModelContentAuditWriter 启动后台 goroutine，从队列中逐条取记录写入 CSV。
// 通过 sync.Once 保证只启动一次。记录写入失败时会重试（最多 modelContentAuditWriteRetries 次）。
func startModelContentAuditWriter() {
	go func() {
		for record := range modelContentAuditQueue {
			if err := writeModelContentAuditRecordWithRetry(record); err != nil {
				common.SysLog("write model content audit failed: " + err.Error())
			}
		}
	}()
}

// writeModelContentAuditRecordWithRetry 带重试的 CSV 写入。
// 写入失败时每秒重试一次，最多重试 modelContentAuditWriteRetries 次（3600 次 = 1 小时）。
// 首次失败和每 30 次失败时记录日志。
func writeModelContentAuditRecordWithRetry(record ModelContentAuditRecord) error {
	var err error
	for attempt := 0; attempt < modelContentAuditWriteRetries; attempt++ {
		err = writeModelContentAuditRecord(record)
		if err == nil {
			return nil
		}
		if attempt == 0 || (attempt+1)%30 == 0 {
			common.SysLog(fmt.Sprintf("write model content audit retrying request_id=%s attempt=%d error=%s", record.RequestId, attempt+1, err.Error()))
		}
		time.Sleep(modelContentAuditWriteRetryInterval)
	}
	return err
}

// writeModelContentAuditRecord 将单条审计记录写入当天 CSV 文件。
//
// 文件命名：模型内容审计目录 / model_qa_YYYY-MM-DD.csv
//   - 目录通过环境变量 MODEL_CONTENT_AUDIT_DIR 配置，默认为 docs/modelcontent
//
// Header 写入策略（needModelContentAuditHeader）：
//   - 文件不存在 → 写入 header
//   - 文件大小为 0 → 写入 header（可能之前创建了空文件）
//   - 文件大小 > 0 → 不写入 header
//
// 使用 os.O_APPEND 模式打开，支持多进程/多实例并发追加写入。
func writeModelContentAuditRecord(record ModelContentAuditRecord) error {
	// 确定审计文件目录
	dir := strings.TrimSpace(os.Getenv("MODEL_CONTENT_AUDIT_DIR"))
	if dir == "" {
		dir = modelContentAuditDefaultDir
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 当天 CSV 文件路径，如 model_qa_2026-07-02.csv
	filePath := filepath.Join(dir, "model_qa_"+time.Now().Format("2006-01-02")+".csv")

	// 判断是否需要写入 CSV header（仅通过文件大小判断，不读取文件内容）
	needHeader, err := needModelContentAuditHeader(filePath)
	if err != nil {
		return err
	}

	// 以追加模式打开文件，不存在时创建
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	// 新文件或空文件：先写 header
	if needHeader {
		if err := writer.Write(modelContentAuditHeader()); err != nil {
			return err
		}
	}

	// 写入审计数据行
	if err := writer.Write([]string{
		record.CreatedAt,
		record.RequestId,
		fmt.Sprintf("%d", record.UserId),
		record.BaseModel,
		record.RequestMessages,
		record.ResponseText,
		record.ReasoningText,
		record.Status,
		record.ErrorMessage,
	}); err != nil {
		return err
	}

	// Flush 确保数据落盘
	writer.Flush()
	return writer.Error()
}

// modelContentAuditHeader 返回 CSV 文件的表头行。
func modelContentAuditHeader() []string {
	return []string{
		"created_at",
		"request_id",
		"user_id",
		"base_model",
		"request_messages",
		"response_text",
		"reasoning_text",
		"status",
		"error_message",
	}
}

// needModelContentAuditHeader 判断 CSV 文件是否需要写入 header 行。
//
// 判断逻辑（仅通过文件元数据，不读取文件内容）：
//   - 文件不存在 → 需要 header（首次写入）
//   - 文件大小为 0 → 需要 header（可能之前创建了空文件但未写入内容）
//   - 文件大小 > 0 → 不需要 header（已有内容）
//
// 这种基于文件大小的判断方式比读取整个 CSV 内容更高效。
func needModelContentAuditHeader(filePath string) (bool, error) {
	stat, err := os.Stat(filePath)
	if err == nil {
		// 文件存在：只有空文件才需要写 header
		return stat.Size() == 0, nil
	}
	if os.IsNotExist(err) {
		// 文件不存在：需要创建并写 header
		return true, nil
	}
	// 其他错误（如权限问题），向上抛出
	return false, err
}
