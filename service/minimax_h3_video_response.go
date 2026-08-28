package service

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

// MiniMaxH3VideoSeconds 是 MiniMax-H3 两个渠道（sora / AutoDL）固定的生成时长，
// 提交与查询响应统一返回该值（秒）。
const MiniMaxH3VideoSeconds = "10"

// miniMaxH3NullFields 是统一契约中固定出现、当前无值的兼容字段，无值时返回 null。
var miniMaxH3NullFields = []string{
	"action",
	"expires_at",
	"inference_time_s",
	"num_outputs",
	"peak_memory_mb",
	"remixed_from_video_id",
	"url",
}

// BuildMiniMaxH3QueuedVideoResponse 构造 MiniMax-H3 提交即入队时的统一响应，
// sora 与 AutoDL 两个渠道返回完全一致的字段集。
func BuildMiniMaxH3QueuedVideoResponse(taskID, modelName, size string, createdAt int64) map[string]any {
	response := map[string]any{
		"id":           taskID,
		"task_id":      taskID,
		"object":       "video",
		"model":        modelName,
		"status":       dto.VideoStatusQueued,
		"progress":     0,
		"created_at":   createdAt,
		"completed_at": nil,
		"error":        nil,
		"seconds":      MiniMaxH3VideoSeconds,
		"size":         size,
		"quality":      "standard",
	}
	for _, field := range miniMaxH3NullFields {
		response[field] = nil
	}
	return response
}

// NormalizeMiniMaxH3VideoResponse 将 MiniMax-H3 任务的查询响应归一化为统一契约
// （sora / AutoDL 两个渠道返回完全一致的字段集与取值规则）。
// passthrough 为任务提交时保存的响应数据（task.Data），其仅作为兼容字段的透出来源，
// 契约字段（id/status/size 等）一律以任务自身状态为准；可为 nil。
func NormalizeMiniMaxH3VideoResponse(task *model.Task, passthrough map[string]any) map[string]any {
	response := make(map[string]any, len(passthrough)+16)
	for key, value := range passthrough {
		response[key] = value
	}
	// 上游特有字段不在统一契约中透出
	for _, key := range []string{"code", "data", "msg", "request_id", "detail", "metadata", "file_path", "file_paths", "content"} {
		delete(response, key)
	}

	response["id"] = task.TaskID
	response["task_id"] = task.TaskID
	response["object"] = "video"
	response["model"] = task.Properties.OriginModelName
	response["seconds"] = MiniMaxH3VideoSeconds

	requestedUpscale := task.MiniMaxH3RequestedShortEdge() == MiniMaxH3UpscaleShortEdge
	upscaleCompleted := requestedUpscale &&
		strings.TrimSpace(task.PrivateData.MiniMaxH3UpscaleTaskID) != "" &&
		strings.EqualFold(strings.TrimSpace(task.PrivateData.MiniMaxH3UpscaleStatus), "completed") &&
		strings.TrimSpace(task.PrivateData.ResultURL) != ""

	if requestedUpscale {
		// 两个渠道的源视频固定 768P；1536P 请求在生成与超分两个阶段
		// 都对外固定返回 1536P，只有超分真正完成后才暴露结果。
		response["size"] = "1536P"
	} else if _, ok := response["size"]; !ok {
		response["size"] = "768P"
	}

	status := task.Status.ToVideoStatus()
	if status == dto.VideoStatusUnknown {
		status = dto.VideoStatusInProgress
	}
	if requestedUpscale && !upscaleCompleted && task.Status != model.TaskStatusFailure {
		status = dto.VideoStatusInProgress
	}
	response["status"] = status

	progress := dto.NewOpenAIVideo()
	progress.SetProgressStr(task.Progress)
	response["progress"] = progress.Progress

	if task.CreatedAt > 0 {
		response["created_at"] = task.CreatedAt
	} else {
		response["created_at"] = task.SubmitTime
	}

	response["quality"] = "standard"
	resultURL := strings.TrimSpace(task.PrivateData.ResultURL)
	switch {
	case task.Status == model.TaskStatusFailure:
		message := strings.TrimSpace(task.FailReason)
		if message == "" {
			message = "video generation failed"
		}
		response["completed_at"] = task.FinishTime
		response["error"] = map[string]any{
			"code":    "video_generation_failed",
			"message": message,
		}
	case status == dto.VideoStatusCompleted:
		completedAt := task.FinishTime
		if completedAt == 0 {
			completedAt = task.UpdatedAt
		}
		response["completed_at"] = completedAt
		response["error"] = nil
		if upscaleCompleted {
			response["quality"] = "high"
		}
		if resultURL != "" {
			response["content"] = map[string]any{"url": resultURL}
		}
	default:
		response["completed_at"] = nil
		response["error"] = nil
	}

	for _, field := range miniMaxH3NullFields {
		if _, ok := response[field]; !ok {
			response[field] = nil
		}
	}
	return response
}
