package openai

import (
	"fmt"
	"net/http"
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

// OaiChatToResponsesCompactionStreamHandler 处理压缩请求（检测到 compaction_trigger）的
// 流式上游 Chat Completions 响应：读取上游流，提取纯文本摘要，合成恰好一个
// {"type":"compaction","encrypted_content":"ocx1:base64(摘要)"} output item 返回给 codex 客户端。
//
// 压缩请求的响应不包含 message / reasoning / tool_call 条目——codex 客户端的
// collect_compaction_output 要求流中恰好一个 compaction item，其余 output item 允许共存
// 但无意义，因此这里只合成 compaction item + response.completed。
func OaiChatToResponsesCompactionStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	responseID := helper.GetResponseID(c)
	createdAt := time.Now().Unix()
	model := info.UpstreamModelName

	var (
		usage      = &dto.Usage{}
		outputText strings.Builder
		reasonText strings.Builder
		sentStart  bool
		streamErr  *types.NewAPIError
	)

	sendEvent := func(event dto.ResponsesStreamResponse) bool {
		data, err := common.Marshal(event)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
			return false
		}
		helper.ResponseChunkData(c, event, string(data))
		return true
	}

	sendCreated := func() bool {
		if sentStart {
			return true
		}
		if responseID == "" {
			responseID = helper.GetResponseID(c)
		}
		if !sendEvent(dto.ResponsesStreamResponse{
			Type: "response.created",
			Response: &dto.OpenAIResponsesResponse{
				ID:        responseID,
				Object:    "response",
				CreatedAt: int(createdAt),
				Status:    responsesStatus("in_progress"),
				Model:     model,
				Output:    []dto.ResponsesOutput{},
			},
		}) {
			return false
		}
		sentStart = true
		return true
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			logger.LogError(c, "failed to unmarshal chat stream event for responses compaction: "+err.Error())
			sr.Error(err)
			return
		}
		if chunk.Id != "" {
			responseID = chunk.Id
		}
		if chunk.Created != 0 {
			createdAt = chunk.Created
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if service.ValidUsage(chunk.Usage) {
			usage = responsesUsageFromChat(chunk.Usage)
		}
		for _, choice := range chunk.Choices {
			if reasoningDelta := choice.Delta.GetReasoningContent(); reasoningDelta != "" {
				reasonText.WriteString(reasoningDelta)
			}
			// 摘要文本是压缩 item 的唯一内容来源（reasoning 不进入摘要）。
			if content := choice.Delta.GetContentString(); content != "" {
				outputText.WriteString(content)
				service.AppendModelContentAuditResponseText(c, content)
			}
			// 工具调用在压缩路径上不应出现（请求侧已剥离 tools）；
			// 若上游仍返回（如模型忽略指令），内容不会进入摘要，由最终校验兜底报错。
		}
	})

	if streamErr != nil {
		return nil, streamErr
	}

	summary := strings.TrimSpace(outputText.String())
	if summary == "" {
		// 上游未产出任何文本（空响应 / 工具调用 / 模型拒绝）。
		// 不静默失败：返回明确错误让 codex 客户端回退本地压缩（compact_model_fallback），
		// 比返回无效 compaction item 更稳。
		return nil, types.NewOpenAIError(
			fmt.Errorf("responses compaction failed: upstream returned no summary text (reasoning=%d chars)", reasonText.Len()),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	// 估算用量（上游未返回 usage 时按摘要文本估算）。
	if usage == nil || usage.TotalTokens == 0 {
		usage = service.ResponseText2Usage(c, summary, info.UpstreamModelName, info.GetEstimatePromptTokens())
	}

	// 合成压缩响应：response.created → output_item.done(compaction) → response.completed。
	if !sendCreated() {
		return nil, streamErr
	}

	compactionItem := dto.ResponsesOutput{
		Type:             relaycommon.CompactionItemType,
		ID:               "cmp_" + responseID,
		Status:           "completed",
		EncryptedContent: relaycommon.EncodeCompactionSummary(summary),
	}
	if !sendEvent(dto.ResponsesStreamResponse{
		Type: dto.ResponsesOutputTypeItemDone,
		Item: &compactionItem,
	}) {
		return nil, streamErr
	}
	if !sendEvent(dto.ResponsesStreamResponse{
		Type: "response.completed",
		Response: &dto.OpenAIResponsesResponse{
			ID:        responseID,
			Object:    "response",
			CreatedAt: int(createdAt),
			Status:    responsesStatus("completed"),
			Model:     model,
			Output:    []dto.ResponsesOutput{compactionItem},
			Usage:     responsesUsageFromChat(usage),
		},
	}) {
		return nil, streamErr
	}

	helper.Done(c)
	service.SetModelContentAuditResponseText(c, summary)
	return usage, nil
}

// OaiChatToResponsesCompactionHandler 处理压缩请求的非流式上游 Chat Completions 响应，
// 返回包含恰好一个 compaction item 的非流式 Responses 响应。
func OaiChatToResponsesCompactionHandler(c *gin.Context, info *relaycommon.RelayInfo, chatResp *dto.OpenAITextResponse) (*dto.Usage, *types.NewAPIError) {
	responseID := chatResp.Id
	if responseID == "" {
		responseID = helper.GetResponseID(c)
	}

	createdAt := time.Now().Unix()
	switch created := chatResp.Created.(type) {
	case int64:
		createdAt = created
	case int:
		createdAt = int64(created)
	case float64:
		createdAt = int64(created)
	}

	model := chatResp.Model
	if model == "" && info != nil {
		model = info.UpstreamModelName
	}

	// 提取纯文本摘要（不含 reasoning 与工具调用）。
	summary := ""
	if len(chatResp.Choices) > 0 {
		summary = strings.TrimSpace(chatResp.Choices[0].Message.StringContent())
	}
	if summary == "" {
		return nil, types.NewOpenAIError(
			fmt.Errorf("responses compaction failed: upstream returned no summary text"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	usage := responsesUsageFromChat(&chatResp.Usage)
	if usage == nil || usage.TotalTokens == 0 {
		usage = service.ResponseText2Usage(c, summary, info.UpstreamModelName, info.GetEstimatePromptTokens())
	}

	compactionItem := dto.ResponsesOutput{
		Type:             relaycommon.CompactionItemType,
		ID:               "cmp_" + responseID,
		Status:           "completed",
		EncryptedContent: relaycommon.EncodeCompactionSummary(summary),
	}
	responsesResp := &dto.OpenAIResponsesResponse{
		ID:        responseID,
		Object:    "response",
		CreatedAt: int(createdAt),
		Status:    responsesStatus("completed"),
		Model:     model,
		Output:    []dto.ResponsesOutput{compactionItem},
		Usage:     usage,
	}

	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, nil, responseBody)
	service.SetModelContentAuditResponseText(c, summary)
	return usage, nil
}
