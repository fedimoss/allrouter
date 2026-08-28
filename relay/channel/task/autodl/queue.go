package autodl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
)

// AutoDLQueuedRequest 是 AutoDL 渠道排队任务持久化在
// TaskPrivateData.PendingRequest 中的待提交请求：
// Workflow 为 ComfyUI 工作流名称，Size 为统一契约的展示分辨率，
// Body 为提交时已构建好的最终请求体（媒体上传已完成）。
type AutoDLQueuedRequest struct {
	Workflow string         `json:"workflow"`
	Size     string         `json:"size"`
	Body     map[string]any `json:"body"`
}

// BuildQueuedRequest 由任务类型（t2va/fl2va/ref2va）与提交时构建好的请求体
// 构造排队持久化用的请求包装。
func BuildQueuedRequest(taskType, size string, body map[string]any) (*AutoDLQueuedRequest, error) {
	workflow, err := workflowForTaskType(taskType)
	if err != nil {
		return nil, err
	}
	return &AutoDLQueuedRequest{Workflow: workflow, Size: size, Body: body}, nil
}

// SubmitQueuedTask 将排队的 MiniMax-H3 任务提交到 AutoDL ComfyUI 工作流接口，
// 返回上游任务 ID。由本地并发队列派发（relay.SubmitQueuedMiniMaxH3Task）调用。
func SubmitQueuedTask(ctx context.Context, baseURL, key, proxy string, queued *AutoDLQueuedRequest) (string, error) {
	if queued == nil || strings.TrimSpace(queued.Workflow) == "" {
		return "", fmt.Errorf("AutoDL queued request is missing workflow")
	}
	payload, err := common.Marshal(queued.Body)
	if err != nil {
		return "", fmt.Errorf("marshal AutoDL request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/api/v1/comfyui/comfyui_workflow/"+queued.Workflow,
		bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build AutoDL request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return "", fmt.Errorf("build AutoDL HTTP client: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("submit AutoDL request: %w", err)
	}
	defer service.CloseResponseBodyGracefully(resp)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("read AutoDL submit response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AutoDL submit status %d: %s", resp.StatusCode, string(body))
	}
	var parsed autoDLResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse AutoDL submit response: %w", err)
	}
	if !strings.EqualFold(parsed.Code, "success") || parsed.Data.TaskID == "" {
		message := strings.TrimSpace(parsed.Msg)
		if message == "" {
			message = strings.TrimSpace(parsed.Data.Message)
		}
		if message == "" {
			message = fmt.Sprintf("AutoDL submit failed: %s", string(body))
		}
		return "", fmt.Errorf("%s", message)
	}
	return parsed.Data.TaskID, nil
}
