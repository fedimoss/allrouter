package openai

import "strings"

const (
	responsesThinkOpenTag  = "<think>"
	responsesThinkCloseTag = "</think>"
)

// responsesThinkTagFilter separates inline reasoning tags from visible text
// while retaining partial tags between streaming chunks.
type responsesThinkTagFilter struct {
	inThink bool
	pending string
}

func (f *responsesThinkTagFilter) Write(chunk string) (visible string, reasoning string) {
	data := f.pending + chunk
	f.pending = ""

	var visibleBuilder strings.Builder
	var reasoningBuilder strings.Builder
	writeContent := func(content string) {
		if f.inThink {
			reasoningBuilder.WriteString(content)
		} else {
			visibleBuilder.WriteString(content)
		}
	}

	for data != "" {
		index, tag := nextResponsesThinkTag(data)
		if index >= 0 {
			writeContent(data[:index])
			f.inThink = tag == responsesThinkOpenTag
			data = data[index+len(tag):]
			continue
		}

		pendingLength := trailingResponsesThinkTagPrefixLength(data)
		writeContent(data[:len(data)-pendingLength])
		if pendingLength > 0 {
			f.pending = data[len(data)-pendingLength:]
		}
		break
	}

	return visibleBuilder.String(), reasoningBuilder.String()
}

func (f *responsesThinkTagFilter) Flush() (visible string, reasoning string) {
	pending := f.pending
	f.pending = ""
	if f.inThink {
		return "", pending
	}
	return pending, ""
}

func nextResponsesThinkTag(data string) (int, string) {
	openIndex := strings.Index(data, responsesThinkOpenTag)
	closeIndex := strings.Index(data, responsesThinkCloseTag)

	switch {
	case openIndex < 0:
		return closeIndex, responsesThinkCloseTag
	case closeIndex < 0:
		return openIndex, responsesThinkOpenTag
	case openIndex < closeIndex:
		return openIndex, responsesThinkOpenTag
	default:
		return closeIndex, responsesThinkCloseTag
	}
}

func trailingResponsesThinkTagPrefixLength(data string) int {
	maxLength := len(responsesThinkCloseTag) - 1
	if len(data) < maxLength {
		maxLength = len(data)
	}
	for length := maxLength; length > 0; length-- {
		suffix := data[len(data)-length:]
		if strings.HasPrefix(responsesThinkOpenTag, suffix) || strings.HasPrefix(responsesThinkCloseTag, suffix) {
			return length
		}
	}
	return 0
}

func filterResponsesInlineThinking(text string) (visible string, reasoning string) {
	filter := responsesThinkTagFilter{}
	visible, reasoning = filter.Write(text)
	remainingVisible, remainingReasoning := filter.Flush()
	return visible + remainingVisible, reasoning + remainingReasoning
}
