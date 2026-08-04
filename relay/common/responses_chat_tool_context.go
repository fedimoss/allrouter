package common

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// ResponsesChatToolKind 标识 Responses 工具的原始类型，用于响应阶段恢复工具条目类型。
// 对齐 cc-switch CodexToolKind。
type ResponsesChatToolKind string

const (
	ResponsesChatToolKindFunction   ResponsesChatToolKind = "function"
	ResponsesChatToolKindNamespace  ResponsesChatToolKind = "namespace"
	ResponsesChatToolKindCustom     ResponsesChatToolKind = "custom"
	ResponsesChatToolKindToolSearch ResponsesChatToolKind = "tool_search"
)

const (
	// responsesChatToolSearchProxyName 是 tool_search 工具在 chat 侧固定的 function 名。
	responsesChatToolSearchProxyName = "tool_search"
	// responsesChatCustomInputField 是 custom 工具在 chat arguments 中包装原始输入的字段名。
	responsesChatCustomInputField = "input"
	// responsesChatToolNameMaxLen 限制 chat function 名最大长度（对齐 OpenAI 工具名约束）。
	responsesChatToolNameMaxLen = 64
)

// ResponsesChatToolSpec 记录某个 chat function 名对应的原始 Responses 工具规范。
type ResponsesChatToolSpec struct {
	Kind      ResponsesChatToolKind // 原始工具类型
	Name      string                // Responses API 中的原始工具名
	Namespace string                // 仅 namespace 类型非空：所属命名空间
}

// ResponsesChatToolContext 将 chat function 名映射回原始 Responses 工具规范。
// 由请求转换（service/openaicompat）构造，由响应转换（relay/channel/openai）读取，
// 以把上游返回的 chat function_call 恢复成 function_call / custom_tool_call /
// tool_search_call / namespace function_call。仅在 Responses→Chat 渠道路径使用。
type ResponsesChatToolContext struct {
	// ChatNameToSpec 映射 chat function 名 → 原始 Responses 工具规范。
	ChatNameToSpec map[string]*ResponsesChatToolSpec
	// NamespaceToChat 映射 "namespace\x00name" → chat function 名（用于历史项还原）。
	NamespaceToChat map[string]string
	// SeenChatNames 记录已注册的 chat function 名，做去重（同名工具只保留首个）。
	SeenChatNames map[string]bool
	// ChatTools 按注册顺序保存 chat 工具定义，供请求转换直接输出到 chatReq.Tools。
	ChatTools []dto.ToolCallRequest
}

// NewResponsesChatToolContext 创建一个空的工具上下文。
func NewResponsesChatToolContext() *ResponsesChatToolContext {
	return &ResponsesChatToolContext{
		ChatNameToSpec:  make(map[string]*ResponsesChatToolSpec),
		NamespaceToChat: make(map[string]string),
		SeenChatNames:   make(map[string]bool),
	}
}

// IsEmpty 判断上下文是否未注册任何工具映射。
func (ctx *ResponsesChatToolContext) IsEmpty() bool {
	return ctx == nil || len(ctx.ChatNameToSpec) == 0
}

// Lookup 按 chat function 名查找原始工具规范。
func (ctx *ResponsesChatToolContext) Lookup(chatName string) *ResponsesChatToolSpec {
	if ctx == nil {
		return nil
	}
	return ctx.ChatNameToSpec[chatName]
}

// IsCustomToolChatName 判断该 chat function 名是否对应 custom 工具
// （custom 工具在流式阶段不发 function_call_arguments.delta，改发 input 事件）。
func (ctx *ResponsesChatToolContext) IsCustomToolChatName(chatName string) bool {
	spec := ctx.Lookup(chatName)
	return spec != nil && spec.Kind == ResponsesChatToolKindCustom
}

// AddChatTool 注册一个 chat function 名 → spec 的映射。空名或已存在（去重）时返回 false，
// 新注册返回 true。调用方应在返回 true 时同步追加 ChatTools。namespace 类型额外记录
// NamespaceToChat 以便历史 function_call 项还原 chat 名。
func (ctx *ResponsesChatToolContext) AddChatTool(chatName string, spec *ResponsesChatToolSpec) bool {
	if ctx == nil || strings.TrimSpace(chatName) == "" || spec == nil {
		return false
	}
	if ctx.SeenChatNames[chatName] {
		return false
	}
	ctx.SeenChatNames[chatName] = true
	if spec.Kind == ResponsesChatToolKindNamespace && spec.Namespace != "" {
		ctx.NamespaceToChat[spec.Namespace+"\x00"+spec.Name] = chatName
	}
	ctx.ChatNameToSpec[chatName] = spec
	return true
}

// ChatNameForResponseFunction 返回历史 function_call 项（可能带 namespace）对应的 chat function 名。
// 有 namespace 时优先用已注册映射，否则按规则拼接；无 namespace 时直接用原名。
func (ctx *ResponsesChatToolContext) ChatNameForResponseFunction(name, namespace string) string {
	if ctx == nil || namespace == "" {
		return name
	}
	if chatName, ok := ctx.NamespaceToChat[namespace+"\x00"+name]; ok {
		return chatName
	}
	return FlattenResponsesNamespaceToolName(namespace, name)
}

// ResponsesChatToolSearchProxyName 返回 tool_search 工具在 chat 侧的固定 function 名。
func ResponsesChatToolSearchProxyName() string { return responsesChatToolSearchProxyName }

// ResponsesChatCustomInputField 返回 custom 工具包装原始输入的字段名。
func ResponsesChatCustomInputField() string { return responsesChatCustomInputField }

// FlattenResponsesNamespaceToolName 把 namespace + name 拼成 chat function 名（"namespace__name"），
// 超过最大长度时用 sha256 截断后缀，对齐 cc-switch flatten_namespace_tool_name。
func FlattenResponsesNamespaceToolName(namespace, name string) string {
	full := namespace + "__" + name
	if len(full) <= responsesChatToolNameMaxLen {
		return full
	}
	sum := sha256.Sum256([]byte(full))
	suffix := "__" + hex.EncodeToString(sum[:])[:8]
	prefixLen := responsesChatToolNameMaxLen - len(suffix)
	if prefixLen < 0 {
		prefixLen = 0
	}
	if prefixLen > len(full) {
		prefixLen = len(full)
	}
	return full[:prefixLen] + suffix
}
