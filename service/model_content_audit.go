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

const (
	modelContentAuditQueueSize          = 5000
	modelContentAuditDefaultDir         = "docs/modelcontent"
	modelContentAuditWriteRetries       = 3600
	modelContentAuditWriteRetryInterval = time.Second
)

var (
	modelContentAuditQueue = make(chan ModelContentAuditRecord, modelContentAuditQueueSize)
	modelContentAuditOnce  sync.Once
)

type ModelContentAuditRecord struct {
	CreatedAt       string
	RequestId       string
	UserId          int
	BaseModel       string
	RequestMessages string
	ResponseText    string
	ReasoningText   string
	Status          string
	ErrorMessage    string
}

func EnqueueModelContentAudit(record ModelContentAuditRecord) {
	modelContentAuditOnce.Do(startModelContentAuditWriter)

	if strings.TrimSpace(record.CreatedAt) == "" {
		record.CreatedAt = time.Now().Format(time.RFC3339Nano)
	}

	select {
	case modelContentAuditQueue <- record:
	default:
		common.SysLog(fmt.Sprintf("model content audit queue full, dropped request_id=%s", record.RequestId))
	}
}

func EnqueueModelContentAuditFromRelay(c *gin.Context, info *relaycommon.RelayInfo, requestMessages string, newAPIError *types.NewAPIError) {
	if info == nil {
		return
	}

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

func SetModelContentAuditResponseText(c *gin.Context, text string) {
	if c == nil || text == "" {
		return
	}
	common.SetContextKey(c, constant.ContextKeyModelContentAuditResponseText, text)
}

func AppendModelContentAuditResponseText(c *gin.Context, text string) {
	if c == nil || text == "" {
		return
	}
	current := GetModelContentAuditResponseText(c)
	common.SetContextKey(c, constant.ContextKeyModelContentAuditResponseText, current+text)
}

func GetModelContentAuditResponseText(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return common.GetContextKeyString(c, constant.ContextKeyModelContentAuditResponseText)
}

func SetModelContentAuditReasoningText(c *gin.Context, text string) {
	if c == nil || text == "" {
		return
	}
	common.SetContextKey(c, constant.ContextKeyModelContentAuditReasoningText, text)
}

func AppendModelContentAuditReasoningText(c *gin.Context, text string) {
	if c == nil || text == "" {
		return
	}
	current := GetModelContentAuditReasoningText(c)
	common.SetContextKey(c, constant.ContextKeyModelContentAuditReasoningText, current+text)
}

func GetModelContentAuditReasoningText(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return common.GetContextKeyString(c, constant.ContextKeyModelContentAuditReasoningText)
}

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

func ModelContentAuditOpenAIRequestMessages(request *dto.GeneralOpenAIRequest) string {
	if request == nil {
		return ""
	}
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
	return ModelContentAuditJSONString(request.Messages)
}

func ModelContentAuditClaudeRequestMessages(request *dto.ClaudeRequest) string {
	if request == nil {
		return ""
	}
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
	return ModelContentAuditJSONString(request.Messages)
}

func ModelContentAuditGeminiRequestMessages(request *dto.GeminiChatRequest) string {
	if request == nil {
		return ""
	}
	return ModelContentAuditJSONString(map[string]any{
		"system_instruction": request.SystemInstructions,
		"contents":           request.Contents,
	})
}

func ModelContentAuditOpenAIResponseText(response *dto.OpenAITextResponse) string {
	responseText, _ := ModelContentAuditOpenAIResponseParts(response)
	return responseText
}

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

func ModelContentAuditResponseTextFromJSON(data []byte) string {
	responseText, reasoningText := ModelContentAuditResponsePartsFromJSON(data)
	return responseText + reasoningText
}

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
		for _, key := range []string{"thinking", "reasoning", "reasoning_content"} {
			if text, ok := v[key].(string); ok {
				reasoningBuilder.WriteString(text)
			}
		}
		for _, key := range []string{"text", "output_text", "completion"} {
			if text, ok := v[key].(string); ok {
				responseBuilder.WriteString(text)
			}
		}
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

func modelContentAuditTextFromContent(content any) string {
	var builder strings.Builder
	appendModelContentAuditContentText(&builder, content)
	return strings.TrimSpace(builder.String())
}

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
			text = text[:start]
			break
		}
		end += start + len(startTag) + len(endTag)
		text = text[:start] + text[end:]
	}
	return strings.TrimSpace(text)
}

func startModelContentAuditWriter() {
	go func() {
		for record := range modelContentAuditQueue {
			if err := writeModelContentAuditRecordWithRetry(record); err != nil {
				common.SysLog("write model content audit failed: " + err.Error())
			}
		}
	}()
}

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

func writeModelContentAuditRecord(record ModelContentAuditRecord) error {
	dir := strings.TrimSpace(os.Getenv("MODEL_CONTENT_AUDIT_DIR"))
	if dir == "" {
		dir = modelContentAuditDefaultDir
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dir, "model_qa_"+time.Now().Format("2006-01-02")+".csv")
	needHeader, err := ensureModelContentAuditHeader(filePath)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if needHeader {
		if err := writer.Write(modelContentAuditHeader()); err != nil {
			return err
		}
	}

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
	writer.Flush()
	return writer.Error()
}

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

func ensureModelContentAuditHeader(filePath string) (bool, error) {
	needHeader := true
	if stat, err := os.Stat(filePath); err == nil && stat.Size() > 0 {
		needHeader = false
	} else if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if needHeader {
		return true, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	rows, err := csv.NewReader(file).ReadAll()
	closeErr := file.Close()
	if err != nil {
		return false, err
	}
	if closeErr != nil {
		return false, closeErr
	}
	if len(rows) == 0 {
		return true, nil
	}
	for _, col := range rows[0] {
		if col == "user_id" {
			for _, col := range rows[0] {
				if col == "reasoning_text" {
					return false, nil
				}
			}
			break
		}
	}

	rows[0] = modelContentAuditHeader()
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) >= 9 {
			continue
		}
		padded := make([]string, 0, 9)
		if len(row) == 7 {
			padded = append(padded, row[:min(len(row), 2)]...)
			padded = append(padded, "")
			if len(row) > 2 {
				padded = append(padded, row[2:min(len(row), 5)]...)
			}
			padded = append(padded, "")
			if len(row) > 5 {
				padded = append(padded, row[5:]...)
			}
		} else {
			padded = append(padded, row[:min(len(row), 6)]...)
			padded = append(padded, "")
			if len(row) > 6 {
				padded = append(padded, row[6:]...)
			}
		}
		for len(padded) < 9 {
			padded = append(padded, "")
		}
		rows[i] = padded
	}

	file, err = os.OpenFile(filePath, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return false, err
	}
	writer := csv.NewWriter(file)
	err = writer.WriteAll(rows)
	if writerErr := writer.Error(); err == nil {
		err = writerErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return false, err
}
