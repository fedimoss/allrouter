package openaicompat

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const responsesChatHistoryLimit = 512

type responsesChatHistoryItem struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	ID     string `json:"id"`
}

func (item responsesChatHistoryItem) resolvedCallID() string {
	if id := strings.TrimSpace(item.CallID); id != "" {
		return id
	}
	return strings.TrimSpace(item.ID)
}

type responsesChatHistoryStore struct {
	mu        sync.RWMutex
	responses map[string][]json.RawMessage
	order     []string
}

var responsesChatHistory = responsesChatHistoryStore{
	responses: make(map[string][]json.RawMessage),
}

// RestoreResponsesChatToolHistory restores the assistant tool-call items that
// are omitted by Responses clients using previous_response_id.
func RestoreResponsesChatToolHistory(req *dto.OpenAIResponsesRequest) int {
	if req == nil || strings.TrimSpace(req.PreviousResponseID) == "" || len(req.Input) == 0 {
		return 0
	}

	items, wasArray, ok := responseInputItems(req.Input)
	if !ok {
		return 0
	}

	outputIDs := make(map[string]bool)
	existingIDs := make(map[string]bool)
	for _, raw := range items {
		var item responsesChatHistoryItem
		if common.Unmarshal(raw, &item) != nil || item.resolvedCallID() == "" {
			continue
		}
		callID := item.resolvedCallID()
		switch item.Type {
		case "function_call", "custom_tool_call", "tool_search_call":
			existingIDs[callID] = true
		case "function_call_output", "custom_tool_call_output", "tool_search_output":
			outputIDs[callID] = true
		}
	}
	if len(outputIDs) == 0 {
		return 0
	}

	requestedIDs := make(map[string]bool, len(outputIDs)+len(existingIDs))
	for callID := range outputIDs {
		requestedIDs[callID] = true
	}
	for callID := range existingIDs {
		requestedIDs[callID] = true
	}
	history := responsesChatHistory.lookup(req.PreviousResponseID, requestedIDs)
	if len(history) == 0 {
		return 0
	}

	missing := make([]json.RawMessage, 0, len(history))
	changed := 0
	for _, raw := range history {
		var item responsesChatHistoryItem
		if common.Unmarshal(raw, &item) != nil || item.resolvedCallID() == "" {
			continue
		}
		callID := item.resolvedCallID()
		if existingIDs[callID] {
			for i := range items {
				var existing responsesChatHistoryItem
				if common.Unmarshal(items[i], &existing) != nil || existing.resolvedCallID() != callID || existing.Type != item.Type {
					continue
				}
				if enrichResponsesToolCallItem(&items[i], raw) {
					// Existing call items stay in their client-supplied position; only
					// missing fields are restored from the cached response.
					changed++
				}
				break
			}
			continue
		}
		missing = append(missing, cloneRawMessage(raw))
	}
	if len(missing) == 0 && changed == 0 {
		return 0
	}

	if len(missing) > 0 {
		insertAt := len(items)
		for i, raw := range items {
			var item responsesChatHistoryItem
			if common.Unmarshal(raw, &item) == nil && isResponsesToolOutputType(item.Type) {
				insertAt = i
				break
			}
		}
		items = append(items[:insertAt], append(missing, items[insertAt:]...)...)
		changed += len(missing)
	}
	if !wasArray && len(items) == 1 {
		req.Input = items[0]
	} else {
		encoded, err := common.Marshal(items)
		if err != nil {
			return 0
		}
		req.Input = encoded
	}
	return changed
}

// enrichResponsesToolCallItem fills only omitted fields on an existing call.
// Responses clients commonly send a minimal function_call when they rely on
// previous_response_id; Chat providers still need its name and arguments.
func enrichResponsesToolCallItem(item *json.RawMessage, cached json.RawMessage) bool {
	var current map[string]json.RawMessage
	var source map[string]json.RawMessage
	if item == nil || common.Unmarshal(*item, &current) != nil || common.Unmarshal(cached, &source) != nil {
		return false
	}
	changed := false
	for _, key := range []string{"id", "call_id", "name", "namespace", "arguments", "input", "status", "execution", "reasoning_content", "reasoning"} {
		value, ok := current[key]
		if ok && !isEmptyJSONValue(value) {
			continue
		}
		value, ok = source[key]
		if !ok || isEmptyJSONValue(value) {
			continue
		}
		current[key] = cloneRawMessage(value)
		changed = true
	}
	if !changed {
		return false
	}
	encoded, err := common.Marshal(current)
	if err != nil {
		return false
	}
	*item = encoded
	return true
}

func isEmptyJSONValue(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null" || trimmed == `""` || trimmed == "{}" || trimmed == "[]"
}

// RememberResponsesChatToolHistory stores tool calls emitted in a converted
// Responses response so the next previous_response_id request can replay them.
func RememberResponsesChatToolHistory(responseID string, output []dto.ResponsesOutput) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" || len(output) == 0 {
		return
	}

	items := make([]json.RawMessage, 0, len(output))
	for _, item := range output {
		if !isResponsesToolCallType(item.Type) {
			continue
		}
		callID := strings.TrimSpace(item.CallId)
		if callID == "" {
			callID = strings.TrimSpace(item.ID)
		}
		if callID == "" {
			continue
		}
		raw, err := common.Marshal(item)
		if err == nil {
			items = append(items, raw)
		}
	}
	if len(items) == 0 {
		return
	}

	responsesChatHistory.mu.Lock()
	if _, exists := responsesChatHistory.responses[responseID]; !exists {
		responsesChatHistory.order = append(responsesChatHistory.order, responseID)
	}
	responsesChatHistory.responses[responseID] = cloneRawMessages(items)
	for len(responsesChatHistory.order) > responsesChatHistoryLimit {
		oldest := responsesChatHistory.order[0]
		responsesChatHistory.order = responsesChatHistory.order[1:]
		delete(responsesChatHistory.responses, oldest)
	}
	responsesChatHistory.mu.Unlock()
}

func (s *responsesChatHistoryStore) lookup(responseID string, requestedIDs map[string]bool) []json.RawMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	selected := make(map[string]json.RawMessage)
	if calls, ok := s.responses[responseID]; ok {
		for _, raw := range calls {
			var item responsesChatHistoryItem
			if common.Unmarshal(raw, &item) == nil && requestedIDs[item.resolvedCallID()] {
				callID := item.resolvedCallID()
				selected[callID] = cloneRawMessage(raw)
			}
		}
	}

	// A call ID is a useful fallback only when it occurs in exactly one cached
	// response; ambiguous IDs must not attach unrelated history.
	for callID := range requestedIDs {
		if _, found := selected[callID]; found {
			continue
		}
		var candidate json.RawMessage
		candidateResponseID := ""
		found := false
		ambiguous := false
		for responseID, calls := range s.responses {
			for _, raw := range calls {
				var item responsesChatHistoryItem
				if common.Unmarshal(raw, &item) != nil || item.resolvedCallID() != callID {
					continue
				}
				if found && candidateResponseID != responseID {
					ambiguous = true
					break
				}
				if !found {
					candidate = raw
					candidateResponseID = responseID
					found = true
				}
			}
			if ambiguous {
				break
			}
		}
		if found && !ambiguous {
			selected[callID] = cloneRawMessage(candidate)
		}
	}

	result := make([]json.RawMessage, 0, len(selected))
	for _, responseID := range s.order {
		for _, raw := range s.responses[responseID] {
			var item responsesChatHistoryItem
			if common.Unmarshal(raw, &item) == nil {
				callID := item.resolvedCallID()
				if selectedItem, ok := selected[callID]; ok {
					result = append(result, selectedItem)
					delete(selected, callID)
				}
			}
		}
	}
	return result
}

func responseInputItems(input json.RawMessage) ([]json.RawMessage, bool, bool) {
	var items []json.RawMessage
	if err := common.Unmarshal(input, &items); err == nil {
		return items, true, true
	}
	var item map[string]any
	if err := common.Unmarshal(input, &item); err == nil {
		return []json.RawMessage{cloneRawMessage(input)}, false, true
	}
	return nil, false, false
}

func isResponsesToolCallType(itemType string) bool {
	return itemType == "function_call" || itemType == "custom_tool_call" || itemType == "tool_search_call"
}

func isResponsesToolOutputType(itemType string) bool {
	return itemType == "function_call_output" || itemType == "custom_tool_call_output" || itemType == "tool_search_output"
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}

func cloneRawMessages(items []json.RawMessage) []json.RawMessage {
	cloned := make([]json.RawMessage, len(items))
	for i, raw := range items {
		cloned[i] = cloneRawMessage(raw)
	}
	return cloned
}
