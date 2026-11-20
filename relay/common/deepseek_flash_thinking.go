package common

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const DeepSeekFlashDefaultEffort = "high"

// UsesDeepSeekFlashChatThinking selects native Flash Chat controls, not
// provider-specific interfaces such as OpenRouter's reasoning object.
func UsesDeepSeekFlashChatThinking(channelType int, model string) bool {
	switch channelType {
	case constant.ChannelTypeOpenAI, constant.ChannelTypeCustom, constant.ChannelTypeResponsesChat, constant.ChannelTypeDeepSeek:
	default:
		return false
	}
	model = strings.ToLower(model[strings.LastIndexByte(model, '/')+1:])
	const flash = "deepseek-v4.1-flash"
	return model == flash || strings.HasPrefix(model, flash+"-")
}

// NormalizeDeepSeekFlashChatThinking can patch a pass-through body without
// dropping fields the gateway does not model.
func NormalizeDeepSeekFlashChatThinking(body []byte) ([]byte, error) {
	thinking := gjson.GetBytes(body, "thinking")
	if !thinking.Exists() {
		return body, nil
	}
	if !thinking.IsObject() {
		return nil, errors.New("thinking must be an object with type \"enabled\" or \"disabled\"")
	}
	effort := gjson.GetBytes(body, "reasoning_effort")
	switch thinking.Get("type").String() {
	case "enabled":
		if effort.Type == gjson.Null || (effort.Type == gjson.String && effort.String() == "") {
			return sjson.SetBytes(body, "reasoning_effort", DeepSeekFlashDefaultEffort)
		}
	case "disabled":
		// Explicitly disabling thinking takes precedence over the effort.
		if effort.Exists() {
			return sjson.DeleteBytes(body, "reasoning_effort")
		}
	default:
		return nil, errors.New("thinking requires type \"enabled\" or \"disabled\"")
	}
	return body, nil
}
