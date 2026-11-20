package service

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func convertDeepSeekFlashMessagesThinking(messages *dto.ClaudeRequest, chat *dto.GeneralOpenAIRequest) error {
	invalid := func(message string) error {
		return types.NewError(errors.New(message), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	var options dto.OutputConfigForEffort
	if len(messages.OutputConfig) > 0 {
		if err := common.Unmarshal(messages.OutputConfig, &options); err != nil {
			return invalid("output_config must be an object with a string effort")
		}
	}
	if options.Effort != "" && options.Effort != "high" && options.Effort != "max" {
		return invalid("output_config.effort must be \"high\" or \"max\" for DeepSeek Flash")
	}
	if messages.Thinking == nil && options.Effort == "" {
		return nil
	}

	mode := "enabled"
	if t := messages.Thinking; t != nil {
		if t.BudgetTokens != nil {
			// The upstream Chat API exposes effort levels, not a separate reasoning limit.
			// max_tokens limits the entire completion, including the final answer.
			return invalid("DeepSeek Flash does not support thinking.budget_tokens. Set thinking to type \"adaptive\" and output_config.effort to \"high\" or \"max\"")
		}
		switch t.Type {
		case "adaptive", "enabled":
		case "disabled":
			mode = "disabled"
		default:
			return invalid("DeepSeek Flash requires thinking type \"adaptive\", \"enabled\" or \"disabled\"")
		}
	}
	thinking, err := common.Marshal(dto.Thinking{Type: mode})
	if err != nil {
		return err
	}
	chat.THINKING = thinking
	if mode == "enabled" {
		chat.ReasoningEffort = options.Effort
		if chat.ReasoningEffort == "" {
			chat.ReasoningEffort = relaycommon.DeepSeekFlashDefaultEffort
		}
	}
	return nil
}
