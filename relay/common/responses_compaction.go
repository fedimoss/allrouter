package common

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// ResponsesCompactionState 记录 Responses→Chat 渠道路径上检测到的远程压缩（v2）请求状态。
// 仅在检测到 compaction_trigger 时非 nil；普通请求保持 nil，路径零影响。
type ResponsesCompactionState struct {
	// Triggered 表示输入中检测到 {"type":"compaction_trigger"}。
	Triggered bool
	// TriggerIndex 是 compaction_trigger 在 input 数组中的索引（-1 表示未找到）。
	TriggerIndex int
	// NeedSyntheticResponse 表示中继需要合成 compaction 响应
	// （上游 chat 模型不认识 compaction_trigger，必须由中继包装 compaction item）。
	// 该字段预留用于未来接入原生 /responses 上游时关闭合成（上游自行产出 compaction item）。
	NeedSyntheticResponse bool
}

// CompactSummaryPrefix 是重放压缩摘要时附加的引导文本前缀。
// 对齐 codex-rs 本地压缩的 SUMMARY_PREFIX（codex-rs/prompts/templates/compact/summary_prefix.md），
// 使摘要信息在后续对话中作为"其他模型产出的摘要"被模型正确理解。
const CompactSummaryPrefix = "Another language model started to solve this problem and produced a summary of its thinking process. You also have access to the state of the tools that were used by that language model. Use this to build on the work that has already been done and avoid duplicating work. Here is the summary produced by the other language model, use the information in this summary to assist with your own analysis:"

// OcxCompactionPrefix 是压缩摘要透明信封前缀（对齐 opencodex：ocx1: + base64(utf8 摘要)）。
// Codex 客户端不解析 encrypted_content 内容、原样保存并重放；
// 中继侧凭此前缀识别"自己生成的摘要"并解码还原为明文，真正的 OpenAI 加密 blob（无前缀）
// 则降级为不可读提示。
const OcxCompactionPrefix = "ocx1:"

// OpaqueCompactionNote 是无法解码的压缩 blob（如真正的 OpenAI 加密内容）在 chat 上游展示的占位提示。
const OpaqueCompactionNote = "[earlier conversation was compacted; the summary is stored in a format this model cannot read]"

// CompactionTriggerItemType 是 codex 远程压缩 v2 的输入触发器条目类型。
const CompactionTriggerItemType = "compaction_trigger"

// CompactionItemType 是 codex 远程压缩 v2 期望的压缩输出条目类型。
const CompactionItemType = "compaction"

// CompactionSummarizePrompt 是压缩请求追加到 chat 上游的 user 摘要指令。
// 对齐 codex-rs 本地压缩提示词（codex-rs/prompts/templates/compact/prompt.md），
// 并明确声明"这不是用户请求"，防止模型继续执行原任务而不是生成摘要。
// 采用追加 user 消息方案而非覆盖 instructions：保留 codex 原始 base instructions，
// 且所有 chat 模型均支持 user 消息（部分模型对 system 指令遵循度低或数量有限制）。
const CompactionSummarizePrompt = `You are performing a CONTEXT CHECKPOINT COMPACTION. This is not a user request. Generate a context checkpoint summary only.

Create a handoff summary for another LLM that will resume the task.

Include:
- Current progress and key decisions made
- Important context, constraints, or user preferences
- What remains to be done (clear next steps)
- Any critical data, examples, or references needed to continue

Be concise, structured, and focused on helping the next LLM seamlessly continue the work. Do not chat with the user. Do not ask follow-up questions. Do not execute any tools. Output only the summary text.`

// EncodeCompactionSummary 把摘要文本编码为透明信封：ocx1: + base64(utf8 摘要)。
// 摘要为空时返回空字符串。
func EncodeCompactionSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	return OcxCompactionPrefix + base64.StdEncoding.EncodeToString([]byte(summary))
}

// DecodeCompactionSummary 解码 ocx1: 信封；无法识别（非前缀/解码失败）时返回 ok=false。
func DecodeCompactionSummary(encryptedContent string) (string, bool) {
	if !strings.HasPrefix(encryptedContent, OcxCompactionPrefix) {
		return "", false
	}
	raw, err := base64.StdEncoding.DecodeString(encryptedContent[len(OcxCompactionPrefix):])
	if err != nil {
		return "", false
	}
	return string(raw), true
}

// CompactionItemToMessageText 把重放的压缩条目转换为 chat 模型可读的文本。
// - ocx1: 信封 → 解码为 SUMMARY_PREFIX + 摘要
// - 无前缀的加密 blob → 不可读占位提示
func CompactionItemToMessageText(encryptedContent string) string {
	if summary, ok := DecodeCompactionSummary(encryptedContent); ok {
		return CompactSummaryPrefix + "\n\n" + summary
	}
	return OpaqueCompactionNote
}

// DetectResponsesCompactionTrigger 扫描 Responses input 数组，检测末尾的 compaction_trigger 条目。
// 返回触发索引（-1 表示未检测到）。同时统计 additional_tools 条目数量供日志使用。
func DetectResponsesCompactionTrigger(input json.RawMessage) (triggerIndex int, additionalToolsCount int) {
	triggerIndex = -1
	if len(input) == 0 {
		return -1, 0
	}
	var items []json.RawMessage
	if err := json.Unmarshal(input, &items); err != nil {
		return -1, 0
	}
	for i, raw := range items {
		var peek struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil {
			continue
		}
		switch peek.Type {
		case CompactionTriggerItemType:
			triggerIndex = i
		case "additional_tools":
			additionalToolsCount++
		}
	}
	return triggerIndex, additionalToolsCount
}
