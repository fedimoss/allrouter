package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// ResolveMappedModelName follows the channel model mapping chain and returns
// the final upstream model name. A self-mapping is treated as no mapping.
// 逻辑：逐级解析渠道 model_mapping，返回最终上游模型名。
// 注意：能力判断必须使用解析后的上游名，不能依赖公开模型名前缀。
func ResolveMappedModelName(mappingJSON, originModel string) (string, bool, error) {
	originModel = strings.TrimSpace(originModel)
	mappingJSON = strings.TrimSpace(mappingJSON)
	if mappingJSON == "" || mappingJSON == "{}" || originModel == "" {
		return originModel, false, nil
	}

	modelMap := make(map[string]string)
	if err := common.Unmarshal([]byte(mappingJSON), &modelMap); err != nil {
		return "", false, errors.New("unmarshal_model_mapping_failed")
	}

	current := originModel
	visited := map[string]bool{current: true}
	mapped := false
	for {
		next, exists := modelMap[current]
		next = strings.TrimSpace(next)
		if !exists || next == "" || next == current {
			break
		}
		// 兼容：映射可能链式配置；visited 用于阻止 A→B→A 死循环。
		if visited[next] {
			return "", false, fmt.Errorf("model_mapping_contains_cycle")
		}
		visited[next] = true
		current = next
		mapped = true
	}
	return current, mapped, nil
}
