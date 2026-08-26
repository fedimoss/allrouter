package openaicompat

// Responses→Chat 转换的 tool 输出媒体提取（对齐 cc-switch src-tauri/src/proxy/tool_media.rs）。
//
// Responses 的 function_call_output.content 可携带结构化媒体块（截图工具的图片、
// 语音工具的音频等），而 Chat Completions 的 role=tool 消息只接受文本：base64 媒体
// 留在 JSON 文本里模型既看不见又烧 token。这里把识别到的媒体块提取成 chat content
// part，暂存 pending 队列，由回合边界处的合成 user 消息承载（原位置留替换标记）。
// 同时对已确认含媒体的输出中残留的裸 base64/data-URL 长串做省略钳制。

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	// wholeDataURLMinBytes 整串 data URL 的最小长度：小值（内联小图标）保持文本，
	// 避免破坏有意检查小图的工作流（对齐 cc-switch WHOLE_DATA_URL_MIN_BYTES）。
	wholeDataURLMinBytes = 8 * 1024
	// base64ishMinBytes 裸 base64 载荷的最小长度（对齐 cc-switch BASE64ISH_MIN_BYTES）。
	base64ishMinBytes = 16 * 1024
	// maxMediaTraversalDepth 递归深度上限（对齐 cc-switch MAX_MEDIA_TRAVERSAL_DEPTH）。
	maxMediaTraversalDepth = 32

	// toolResultMediaMovedMarker 媒体块被移走后，原结构位置的替换文本块。
	toolResultMediaMovedMarker = "[media moved to the following user message]"
)

// responsesToolOutputMediaPlan 描述一次 tool 输出的媒体提取结果。
type responsesToolOutputMediaPlan struct {
	// toolContent 提取后（含替换标记与钳制）的 tool 消息文本内容。
	toolContent string
	// mediaParts 提取出的 chat content parts（image_url / file / input_audio）。
	mediaParts []dto.MediaContent
}

// queuePendingToolMedia 把提取到的媒体块连同出处标注加入 pending 队列
// （对齐 cc-switch queue_chat_tool_output_media：先一条出处文本，再媒体本体）。
func queuePendingToolMedia(pending *[]dto.MediaContent, callID string, mediaParts []dto.MediaContent) {
	if len(mediaParts) == 0 {
		return
	}
	*pending = append(*pending, dto.MediaContent{
		Type: dto.ContentTypeText,
		Text: fmt.Sprintf("[media output of tool call %s]", callID),
	})
	*pending = append(*pending, mediaParts...)
}

// flushPendingToolMedia 把 pending 媒体作为合成 user 消息刷出
// （对齐 cc-switch flush_pending_chat_tool_media）。
// 调用点：新 assistant 工具调用回合之前、回合边界（user/system）消息之前、
// 整个 input 处理完毕后——保证媒体总是紧跟其所属 tool 结果、不跨 user 回合。
func flushPendingToolMedia(messages *[]dto.Message, pending *[]dto.MediaContent) {
	if len(*pending) == 0 {
		return
	}
	*messages = append(*messages, dto.Message{
		Role:    "user",
		Content: *pending,
	})
	*pending = nil
}

// planResponsesToolOutputMedia 尝试从 tool 输出中提取媒体块。
// 无媒体时返回 nil，调用方保持原有纯文本路径不变（缓存敏感的零改动语义）。
// 对齐 cc-switch plan_chat_tool_output_media。
func planResponsesToolOutputMedia(raw []byte) *responsesToolOutputMediaPlan {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		// 非 JSON：可能是整串 data URL（标量字符串路径）
		if s, ok := parseJSONStringValue(raw); ok {
			value = s
		} else {
			return nil
		}
	}

	var mediaParts []dto.MediaContent
	replaced := stripMediaFromToolValue(&value, &mediaParts, 0)
	if replaced == 0 {
		return nil
	}
	clampBase64ishStrings(&value)

	// 原始为字符串则结果也保持字符串（避免给模型多包一层 JSON 引号）；
	// 否则规整序列化回 JSON 文本。
	if _, isStr := value.(string); isStr {
		return &responsesToolOutputMediaPlan{toolContent: value.(string), mediaParts: mediaParts}
	}
	return &responsesToolOutputMediaPlan{toolContent: canonicalAnyJSONString(value), mediaParts: mediaParts}
}

// stripMediaFromToolValue 递归剥离媒体块（对齐 cc-switch strip_media_from_tool_value_at_depth）。
// 识别入口见 chatMediaPartFromToolPart；JSON 字符串递归解析后仅在发生替换时回写规整文本。
func stripMediaFromToolValue(value *any, mediaParts *[]dto.MediaContent, depth int) int {
	if depth > maxMediaTraversalDepth {
		return 0
	}
	switch v := (*value).(type) {
	case string:
		// 整串即完整图片 data URL：提取，原位换替换文本（字符串保字符串）
		if part, ok := wholeStringImageDataURL(v); ok {
			*mediaParts = append(*mediaParts, part)
			*value = toolResultMediaMovedMarker
			return 1
		}
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0
		}
		var parsed any
		if err := common.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return 0
		}
		replaced := stripMediaFromToolValue(&parsed, mediaParts, depth+1)
		if replaced > 0 {
			clampBase64ishStrings(&parsed)
			*value = canonicalAnyJSONString(parsed)
		}
		return replaced
	case []any:
		replaced := 0
		for i := range v {
			replaced += stripMediaFromToolValue(&v[i], mediaParts, depth+1)
		}
		return replaced
	case map[string]any:
		// 对象本身是媒体块：提取，原位换替换文本块
		if part, ok := chatMediaPartFromToolPart(v); ok {
			*mediaParts = append(*mediaParts, part)
			*value = map[string]any{
				"type": "text",
				"text": toolResultMediaMovedMarker,
			}
			return 1
		}
		// 仅沿 content 字段下钻（对齐 cc-switch 边界，避免误伤任意对象）
		if content, ok := v["content"]; ok {
			return stripMediaFromToolValue(&content, mediaParts, depth+1)
		}
		return 0
	default:
		return 0
	}
}

// chatMediaPartFromToolPart 把一个识别为媒体的 tool part 转成 chat content part。
// 对齐 cc-switch chat_media_part_from_tool_part（AllSupported scope：图片/文件/音频）。
func chatMediaPartFromToolPart(part map[string]any) (dto.MediaContent, bool) {
	partType, _ := part["type"].(string)

	switch partType {
	case "input_image", "image_url":
		if url, ok := normalizedImageURLObject(part); ok {
			return dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: url}, true
		}
	case "input_file":
		if file := chatFileFromInputFile(part); file != nil {
			return dto.MediaContent{Type: dto.ContentTypeFile, File: file}, true
		}
	case "input_audio":
		if audio, ok := part["input_audio"].(map[string]any); ok && len(audio) > 0 {
			return dto.MediaContent{Type: dto.ContentTypeInputAudio, InputAudio: audio}, true
		}
	case "image":
		if url, ok := typedImageURLObject(part); ok {
			return dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: url}, true
		}
	default:
		// 无 type 的松散形态：image_url 为 data URL 时识别（对齐 loose_data_image_url）
		if _, hasType := part["type"]; !hasType {
			if url, ok := normalizedImageURLObject(part); ok {
				if s, _ := url["url"].(string); strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "data:") {
					return dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: url}, true
				}
			}
		}
	}
	return dto.MediaContent{}, false
}

// normalizedImageURLObject 归一 input_image/image_url 的 image_url 字段：
// 字符串 → {url:...}；对象 → 原样（要求 url 非空）；顶层 detail 补充进对象。
func normalizedImageURLObject(part map[string]any) (map[string]any, bool) {
	raw, ok := part["image_url"]
	if !ok {
		return nil, false
	}
	var obj map[string]any
	switch u := raw.(type) {
	case string:
		if strings.TrimSpace(u) == "" {
			return nil, false
		}
		obj = map[string]any{"url": u}
	case map[string]any:
		url, _ := u["url"].(string)
		if strings.TrimSpace(url) == "" {
			return nil, false
		}
		obj = make(map[string]any, len(u))
		for k, v := range u {
			obj[k] = v
		}
	default:
		return nil, false
	}
	if _, hasDetail := obj["detail"]; !hasDetail {
		if detail, ok := part["detail"]; ok {
			obj["detail"] = detail
		}
	}
	return obj, true
}

// typedImageURLObject 处理 Anthropic/MCP 图片形态：{type:image, source:{data,media_type}}
// 与 {type:image, mimeType, data}，统一转成 data URL。
func typedImageURLObject(part map[string]any) (map[string]any, bool) {
	// Anthropic source 形态
	if source, ok := part["source"].(map[string]any); ok {
		if !sourceMediaTypeIsImage(source) {
			return nil, false
		}
		if url, ok := source["url"].(string); ok && strings.TrimSpace(url) != "" {
			return withTopLevelDetail(part, map[string]any{"url": url}), true
		}
		if data, ok := source["data"].(string); ok && data != "" {
			mediaType := firstNonEmptyString(source["media_type"], source["mime_type"], source["mimeType"])
			if mediaType == "" {
				mediaType = "image/png"
			}
			url := data
			if !strings.HasPrefix(strings.ToLower(data[:min(11, len(data))]), "data:image/") {
				url = fmt.Sprintf("data:%s;base64,%s", mediaType, data)
			}
			return withTopLevelDetail(part, map[string]any{"url": url}), true
		}
		return nil, false
	}
	// MCP mimeType+data 形态
	data, _ := part["data"].(string)
	mediaType := firstNonEmptyString(part["mimeType"], part["mime_type"])
	if data == "" || !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return nil, false
	}
	return withTopLevelDetail(part, map[string]any{
		"url": fmt.Sprintf("data:%s;base64,%s", mediaType, data),
	}), true
}

// chatFileFromInputFile 把 input_file part 映射为 chat file 对象
// （仅保留 file_id/file_data/filename，对齐 cc-switch chat_file_from_input_file）。
func chatFileFromInputFile(part map[string]any) map[string]any {
	_, hasID := part["file_id"]
	_, hasData := part["file_data"]
	if !hasID && !hasData {
		return nil
	}
	file := map[string]any{}
	for _, key := range []string{"file_id", "file_data", "filename"} {
		if v, ok := part[key]; ok {
			file[key] = v
		}
	}
	return file
}

// wholeStringImageDataURL 识别整串完整图片 base64 data URL（≥8KB）。
func wholeStringImageDataURL(s string) (dto.MediaContent, bool) {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < wholeDataURLMinBytes || !isImageBase64DataURL(trimmed) {
		return dto.MediaContent{}, false
	}
	return dto.MediaContent{
		Type:     dto.ContentTypeImageURL,
		ImageUrl: map[string]any{"url": trimmed},
	}, true
}

// clampBase64ishStrings 对已确认含媒体的输出中残留的裸 base64/data-URL 长串做省略钳制
// （对齐 cc-switch clamp_base64ish_strings；仅在媒体提取发生后调用，普通长文本不动）。
func clampBase64ishStrings(value *any) {
	switch v := (*value).(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		shouldOmit := (len(trimmed) >= wholeDataURLMinBytes && strings.HasPrefix(strings.ToLower(trimmed), "data:")) ||
			looksLikeBase64Payload(trimmed)
		if shouldOmit {
			*value = fmt.Sprintf("[omitted %d bytes]", len(v))
		}
	case []any:
		for i := range v {
			clampBase64ishStrings(&v[i])
		}
	case map[string]any:
		for k, val := range v {
			clampBase64ishStrings(&val)
			v[k] = val
		}
	}
}

func looksLikeBase64Payload(s string) bool {
	if len(s) < base64ishMinBytes {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=') {
			return false
		}
	}
	return true
}

func isImageBase64DataURL(s string) bool {
	comma := strings.Index(s, ",")
	if comma < 0 {
		return false
	}
	header := strings.ToLower(s[:comma])
	return strings.HasPrefix(header, "data:image/") && strings.HasSuffix(header, ";base64")
}

func sourceMediaTypeIsImage(source map[string]any) bool {
	mediaType := firstNonEmptyString(source["media_type"], source["mime_type"], source["mimeType"])
	if mediaType == "" {
		return true // 未标注类型视为图片（对齐 cc-switch is_none_or）
	}
	return strings.HasPrefix(strings.ToLower(mediaType), "image/")
}

func withTopLevelDetail(part, obj map[string]any) map[string]any {
	if _, hasDetail := obj["detail"]; !hasDetail {
		if detail, ok := part["detail"]; ok {
			obj["detail"] = detail
		}
	}
	return obj
}

func firstNonEmptyString(values ...any) string {
	for _, v := range values {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// parseJSONStringValue 尝试把 JSON 字节解析为字符串标量（失败返回 false）。
func parseJSONStringValue(raw []byte) (string, bool) {
	var s string
	if err := common.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	return "", false
}

// canonicalAnyJSONString 把任意值规整序列化为紧凑 JSON 字符串。
func canonicalAnyJSONString(value any) string {
	b, err := common.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
