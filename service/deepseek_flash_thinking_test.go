package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

// TestConvertDeepSeekFlashMessagesThinking 验证预算被忽略、显式 effort 优先和关闭语义。
func TestConvertDeepSeekFlashMessagesThinking(t *testing.T) {
	// 每个用例只改变 thinking 或 output_config，便于定位优先级问题。
	tests := []struct {
		name, thinkingType, outputConfig, wantThinking, wantEffort string
		withBudget                                                 bool
		wantErr                                                    bool
	}{
		// 带预算的 enabled 仍开启思考，并补默认 high。
		{name: "enabled ignores budget", thinkingType: "enabled", withBudget: true, wantThinking: `{"type":"enabled"}`, wantEffort: "high"},
		// 带预算的 adaptive 视为 enabled，并补默认 high。
		{name: "adaptive ignores budget", thinkingType: "adaptive", withBudget: true, wantThinking: `{"type":"enabled"}`, wantEffort: "high"},
		// 显式 max 必须覆盖默认 high。
		{name: "explicit max wins", thinkingType: "enabled", withBudget: true, outputConfig: `{"effort":"max"}`, wantThinking: `{"type":"enabled"}`, wantEffort: "max"},
		// 关闭思考时，即使同时传入 effort 也应清空。
		{name: "disabled clears effort", thinkingType: "disabled", withBudget: true, outputConfig: `{"effort":"max"}`, wantThinking: `{"type":"disabled"}`},
		// 只传 effort 时，自动开启思考并使用该档位。
		{name: "effort without thinking enables thinking", outputConfig: `{"effort":"max"}`, wantThinking: `{"type":"enabled"}`, wantEffort: "max"},
		// 不支持的 effort 档位必须拒绝。
		{name: "unsupported effort", thinkingType: "enabled", outputConfig: `{"effort":"low"}`, wantErr: true},
		// 不支持的 thinking 类型必须拒绝。
		{name: "unsupported type", thinkingType: "invalid", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// 按用例构造 Messages 请求中的思考开关。
			messages := &dto.ClaudeRequest{}
			if test.thinkingType != "" {
				// 需要时附带 budget_tokens，验证它不会导致 400。
				messages.Thinking = &dto.Thinking{Type: test.thinkingType}
				if test.withBudget {
					messages.Thinking.BudgetTokens = common.GetPointer(2048)
				}
			}
			// 需要时写入客户端显式 effort。
			if test.outputConfig != "" {
				messages.OutputConfig = []byte(test.outputConfig)
			}
			// 调用被测转换函数，输入保持为空 Chat 请求。
			chat := &dto.GeneralOpenAIRequest{}
			err := convertDeepSeekFlashMessagesThinking(messages, chat)
			if test.wantErr {
				// 非法 effort 或非法 thinking 类型必须报错。
				require.Error(t, err)
				return
			}
			// 合法输入必须成功，并生成预期的开关和 effort。
			require.NoError(t, err)
			require.JSONEq(t, test.wantThinking, string(chat.THINKING))
			require.Equal(t, test.wantEffort, chat.ReasoningEffort)
		})
	}
}
