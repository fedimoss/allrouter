package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

// convertDeepSeekFlashMessagesThinking 把 Messages 思考参数转换为 DeepSeek Flash 的 Chat 参数。
// budget_tokens 按官方兼容规则忽略，实际强度由 output_config.effort 决定。
func convertDeepSeekFlashMessagesThinking(messages *dto.ClaudeRequest, chat *dto.GeneralOpenAIRequest) error {
	// invalid 统一构造带字段定位、且禁止重试的请求参数错误。
	invalid := func(message string) error {
		return types.NewError(errors.New(message), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	// 先解析客户端显式传入的 output_config。
	var options dto.OutputConfigForEffort
	if len(messages.OutputConfig) > 0 {
		// 形状不合法时明确指出 output_config 需要包含字符串 effort。
		if err := common.Unmarshal(messages.OutputConfig, &options); err != nil {
			return invalid("output_config must be an object with a string effort")
		}
	}
	// DeepSeek Flash 只支持 high 和 max 两档，其他值直接拒绝。
	if options.Effort != "" && options.Effort != "high" && options.Effort != "max" {
		return invalid("output_config.effort must be \"high\" or \"max\" for DeepSeek Flash")
	}
	// 客户端既没开启思考也没指定 effort 时，不需要改写 Chat 请求。
	if messages.Thinking == nil && options.Effort == "" {
		return nil
	}

	// 默认按开启处理，再根据客户端 thinking.type 调整。
	mode := "enabled"
	if t := messages.Thinking; t != nil {
		// 官方兼容规则会忽略 budget_tokens，实际强度由 output_config.effort 决定。
		switch t.Type {
		case "adaptive", "enabled":
			// adaptive 与 enabled 都映射为上游 Chat 的 enabled。
		case "disabled":
			// 明确关闭时保留关闭语义。
			mode = "disabled"
		default:
			// 未知类型直接拒绝，避免静默猜测客户端意图。
			return invalid("DeepSeek Flash requires thinking type \"adaptive\", \"enabled\" or \"disabled\"")
		}
	}
	// 序列化最终 thinking 对象，只保留 type，不携带被忽略的预算。
	thinking, err := common.Marshal(dto.Thinking{Type: mode})
	if err != nil {
		// 序列化失败属于网关内部错误，直接向上返回。
		return err
	}
	// 把转换后的开关写回 Chat 请求。
	chat.THINKING = thinking
	if mode == "enabled" {
		// 客户端显式指定的 effort 优先。
		chat.ReasoningEffort = options.Effort
		if chat.ReasoningEffort == "" {
			// 客户端未显式给 effort 时，才补默认档位。
			chat.ReasoningEffort = relaycommon.DeepSeekFlashDefaultEffort
		}
	}
	// 关闭模式不设置 effort，避免上游误开启思考。
	return nil
}
