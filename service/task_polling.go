package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/samber/lo"
)

// TaskPollingAdaptor 定义轮询所需的最小适配器接口，避免 service -> relay 的循环依赖
type TaskPollingAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	FetchTask(baseURL string, key string, body map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error)
	// AdjustBillingOnComplete 在任务到达终态（成功/失败）时由轮询循环调用。
	// 返回正数触发差额结算（补扣/退还），返回 0 保持预扣费金额不变。
	AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int
}

// GetTaskAdaptorFunc 由 main 包注入，用于获取指定平台的任务适配器。
// 打破 service -> relay -> relay/channel -> service 的循环依赖。
var GetTaskAdaptorFunc func(platform constant.TaskPlatform) TaskPollingAdaptor

// sweepTimedOutTasks 在主轮询之前独立清理超时任务。
// 每次最多处理 100 条，剩余的下个周期继续处理。
// 使用 per-task CAS (UpdateWithStatus) 防止覆盖被正常轮询已推进的任务。
func sweepTimedOutTasks(ctx context.Context) {
	if constant.TaskTimeoutMinutes <= 0 {
		return
	}
	cutoff := time.Now().Unix() - int64(constant.TaskTimeoutMinutes)*60
	tasks := model.GetTimedOutUnfinishedTasks(cutoff, 100)
	if len(tasks) == 0 {
		return
	}

	const legacyTaskCutoff int64 = 1740182400 // 2026-02-22 00:00:00 UTC
	reason := fmt.Sprintf("任务超时（%d分钟）", constant.TaskTimeoutMinutes)
	legacyReason := "任务超时（旧系统遗留任务，不进行退款，请联系管理员）"
	now := time.Now().Unix()
	timedOutCount := 0

	for _, task := range tasks {
		if isLocalMiniMaxH3Task(task) {
			continue
		}
		isLegacy := task.SubmitTime > 0 && task.SubmitTime < legacyTaskCutoff

		oldStatus := task.Status
		task.Status = model.TaskStatusFailure
		task.Progress = "100%"
		task.FinishTime = now
		if isLegacy {
			task.FailReason = legacyReason
		} else {
			task.FailReason = reason
		}

		won, err := task.UpdateWithStatus(oldStatus)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("sweepTimedOutTasks CAS update error for task %s: %v", task.TaskID, err))
			continue
		}
		if !won {
			logger.LogInfo(ctx, fmt.Sprintf("sweepTimedOutTasks: task %s already transitioned, skip", task.TaskID))
			continue
		}
		timedOutCount++
		if !isLegacy && task.Quota != 0 {
			RefundTaskQuota(ctx, task, reason)
		}
	}

	if timedOutCount > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("sweepTimedOutTasks: timed out %d tasks", timedOutCount))
	}
}

// TaskPollingLoop 主轮询循环，每 15 秒检查一次未完成的任务
func TaskPollingLoop() {
	for {
		time.Sleep(time.Duration(15) * time.Second)
		common.SysLog("任务进度轮询开始")
		ctx := context.TODO()
		sweepTimedOutTasks(ctx)
		allTasks := model.GetAllUnFinishSyncTasks(constant.TaskQueryLimit)
		// During a rolling deployment, an older instance can persist the 768P
		// source as terminal before a current instance sees it. Reclaim one recent
		// 1536P source per cycle and feed it through the normal polling pipeline.
		recoveryCutoff := time.Now().Add(-time.Hour).Unix()
		recoveryTask, recoveryErr := model.ClaimMiniMaxH3UpscaleRecoveryTask(recoveryCutoff)
		if recoveryErr != nil {
			logger.LogError(ctx, fmt.Sprintf("claim MiniMax-H3 upscale recovery task: %v", recoveryErr))
		} else if recoveryTask != nil {
			logger.LogWarn(ctx, fmt.Sprintf("recovering MiniMax-H3 1536P upscale after source was finalized early: task=%s type=%s",
				recoveryTask.TaskID, recoveryTask.PrivateData.MiniMaxH3TaskType))
			allTasks = append(allTasks, recoveryTask)
		}
		platformTask := make(map[constant.TaskPlatform][]*model.Task)
		for _, t := range allTasks {
			platformTask[t.Platform] = append(platformTask[t.Platform], t)
		}
		for platform, tasks := range platformTask {
			if len(tasks) == 0 {
				continue
			}
			taskChannelM := make(map[int][]string)
			taskM := make(map[string]*model.Task)
			nullTaskIds := make([]int64, 0)
			for _, task := range tasks {
				upstreamID, shouldPoll := upstreamTaskIDForPolling(task)
				if !shouldPoll {
					logger.LogDebug(ctx, fmt.Sprintf("skip polling local MiniMax-H3 task %s without upstream task id", task.TaskID))
					continue
				}
				if upstreamID == "" {
					// 统计失败的未完成任务
					nullTaskIds = append(nullTaskIds, task.ID)
					continue
				}
				taskMapKey := taskMapKeyForPolling(task, upstreamID)
				taskM[taskMapKey] = task
				taskChannelM[task.ChannelId] = append(taskChannelM[task.ChannelId], taskMapKey)
			}
			if len(nullTaskIds) > 0 {
				err := model.TaskBulkUpdateByID(nullTaskIds, map[string]any{
					"status":   "FAILURE",
					"progress": "100%",
				})
				if err != nil {
					logger.LogError(ctx, fmt.Sprintf("Fix null task_id task error: %v", err))
				} else {
					logger.LogInfo(ctx, fmt.Sprintf("Fix null task_id task success: %v", nullTaskIds))
				}
			}
			if len(taskChannelM) == 0 {
				continue
			}

			DispatchPlatformUpdate(platform, taskChannelM, taskM)
		}
		TriggerMiniMaxH3Dispatch()
		common.SysLog("任务进度轮询完成")
	}
}

func upstreamTaskIDForPolling(task *model.Task) (string, bool) {
	if task == nil {
		return "", false
	}
	if constant.IsMiniMaxH3Model(task.Properties.OriginModelName) {
		upstreamID := strings.TrimSpace(task.PrivateData.UpstreamTaskID)
		return upstreamID, upstreamID != ""
	}
	upstreamID := task.GetUpstreamTaskID()
	return upstreamID, upstreamID != ""
}

func isLocalMiniMaxH3Task(task *model.Task) bool {
	if task == nil || !constant.IsMiniMaxH3Model(task.Properties.OriginModelName) {
		return false
	}
	if strings.TrimSpace(task.PrivateData.UpstreamTaskID) != "" {
		return false
	}
	return task.Status == model.TaskStatusNotStart || task.Status == model.TaskStatusDispatching
}

const miniMaxH3TaskMapKeyPrefix = "\x00minimax-h3\x00"

// taskMapKeyForPolling keeps the legacy lookup key for every other task type.
// MiniMax-H3 upstream task IDs are only unique within their source channel, so
// its in-memory polling key must include both channel ID and upstream task ID.
func taskMapKeyForPolling(task *model.Task, upstreamID string) string {
	if task == nil || !constant.IsMiniMaxH3Model(task.Properties.OriginModelName) {
		return upstreamID
	}
	return miniMaxH3TaskMapKeyPrefix + strconv.Itoa(task.ChannelId) + "\x00" + upstreamID
}

// authorizationKeyForPolling pins MiniMax-H3 status requests to the channel
// and key saved when the task was created. Other task types keep their prior behavior.
func authorizationKeyForPolling(task *model.Task, ch *model.Channel) (string, error) {
	if task == nil {
		return "", errors.New("task is required")
	}
	if ch == nil {
		return "", errors.New("channel is required")
	}
	if constant.IsMiniMaxH3Model(task.Properties.OriginModelName) && task.ChannelId != ch.Id {
		return "", fmt.Errorf("refuse to poll MiniMax-H3 task %s through channel #%d: source channel is #%d", task.TaskID, ch.Id, task.ChannelId)
	}
	if task.PrivateData.Key != "" {
		return task.PrivateData.Key, nil
	}
	return ch.Key, nil
}

// DispatchPlatformUpdate 按平台分发轮询更新
func DispatchPlatformUpdate(platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) {
	switch platform {
	case constant.TaskPlatformMidjourney:
		// MJ 轮询由其自身处理，这里预留入口
	case constant.TaskPlatformSuno:
		_ = UpdateSunoTasks(context.Background(), taskChannelM, taskM)
	default:
		if err := UpdateVideoTasks(context.Background(), platform, taskChannelM, taskM); err != nil {
			common.SysLog(fmt.Sprintf("UpdateVideoTasks fail: %s", err))
		}
	}
}

// UpdateSunoTasks 按渠道更新所有 Suno 任务
func UpdateSunoTasks(ctx context.Context, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelId, taskIds := range taskChannelM {
		err := updateSunoTasks(ctx, channelId, taskIds, taskM)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("渠道 #%d 更新异步任务失败: %s", channelId, err.Error()))
		}
	}
	return nil
}

func updateSunoTasks(ctx context.Context, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("渠道 #%d 未完成的任务有: %d", channelId, len(taskIds)))
	if len(taskIds) == 0 {
		return nil
	}
	ch, err := model.CacheGetChannel(channelId)
	if err != nil {
		common.SysLog(fmt.Sprintf("CacheGetChannel: %v", err))
		// Collect DB primary key IDs for bulk update (taskIds are upstream IDs, not task_id column values)
		var failedIDs []int64
		for _, upstreamID := range taskIds {
			if t, ok := taskM[upstreamID]; ok {
				failedIDs = append(failedIDs, t.ID)
			}
		}
		err = model.TaskBulkUpdateByID(failedIDs, map[string]any{
			"fail_reason": fmt.Sprintf("获取渠道信息失败，请联系管理员，渠道ID：%d", channelId),
			"status":      "FAILURE",
			"progress":    "100%",
		})
		if err != nil {
			common.SysLog(fmt.Sprintf("UpdateSunoTask error: %v", err))
		}
		return err
	}
	adaptor := GetTaskAdaptorFunc(constant.TaskPlatformSuno)
	if adaptor == nil {
		return errors.New("adaptor not found")
	}
	proxy := ch.GetSetting().Proxy
	resp, err := adaptor.FetchTask(*ch.BaseURL, ch.Key, map[string]any{
		"ids": taskIds,
	}, proxy)
	if err != nil {
		common.SysLog(fmt.Sprintf("Get Task Do req error: %v", err))
		return err
	}
	defer CloseResponseBodyGracefully(resp)
	if resp.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("Get Task status code: %d", resp.StatusCode))
		return fmt.Errorf("Get Task status code: %d", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		common.SysLog(fmt.Sprintf("Get Suno Task parse body error: %v", err))
		return err
	}
	var responseItems dto.TaskResponse[[]dto.SunoDataResponse]
	err = common.Unmarshal(responseBody, &responseItems)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Get Suno Task parse body error2: %v, body: %s", err, string(responseBody)))
		return err
	}
	if !responseItems.IsSuccess() {
		common.SysLog(fmt.Sprintf("渠道 #%d 未完成的任务有: %d, 成功获取到任务数: %s", channelId, len(taskIds), string(responseBody)))
		return err
	}

	for _, responseItem := range responseItems.Data {
		task := taskM[responseItem.TaskID]
		if !taskNeedsUpdate(task, responseItem) {
			continue
		}

		// 记录轮询前的状态，用于判断本次是否发生了终态转换。
		oldStatus := task.Status
		// shouldRefund 和 shouldFinalizeRebate 先计算后执行，确保 UpdateWithStatus CAS 成功后才触发。
		shouldRefund := false
		shouldFinalizeRebate := false
		task.Status = lo.If(model.TaskStatus(responseItem.Status) != "", model.TaskStatus(responseItem.Status)).Else(task.Status)
		task.FailReason = lo.If(responseItem.FailReason != "", responseItem.FailReason).Else(task.FailReason)
		task.SubmitTime = lo.If(responseItem.SubmitTime != 0, responseItem.SubmitTime).Else(task.SubmitTime)
		task.StartTime = lo.If(responseItem.StartTime != 0, responseItem.StartTime).Else(task.StartTime)
		task.FinishTime = lo.If(responseItem.FinishTime != 0, responseItem.FinishTime).Else(task.FinishTime)
		// 失败：标记退款（仅在首次进入 FAILURE 时触发，避免重复退款）。
		if responseItem.FailReason != "" || task.Status == model.TaskStatusFailure {
			logger.LogInfo(ctx, task.TaskID+" 构建失败，"+task.FailReason)
			task.Progress = "100%"
			shouldRefund = task.Quota != 0 && oldStatus != model.TaskStatusFailure
		}
		// 成功：标记消费返利终态结算（仅在首次进入 SUCCESS 时触发）。
		if responseItem.Status == model.TaskStatusSuccess {
			task.Progress = "100%"
			shouldFinalizeRebate = oldStatus != model.TaskStatusSuccess
		}
		task.Data = responseItem.Data

		// 终态转换使用 UpdateWithStatus CAS 保护，防止并发轮询重复触发计费操作。
		isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
		if isDone && oldStatus != task.Status {
			won, updateErr := task.UpdateWithStatus(oldStatus)
			if updateErr != nil {
				common.SysLog("UpdateSunoTask task error: " + updateErr.Error())
				continue
			}
			if !won {
				// 另一个轮询实例已抢先处理了此任务的状态转换，跳过计费操作。
				logger.LogWarn(ctx, fmt.Sprintf("Suno task %s already transitioned, skip billing", task.TaskID))
				continue
			}
		} else if err = task.Update(); err != nil {
			common.SysLog("UpdateSunoTask task error: " + err.Error())
			continue
		}
		// 只有在 CAS 成功（won=true）后才执行计费操作，确保终态转换和计费是一对一的。
		if shouldRefund {
			RefundTaskQuota(ctx, task, task.FailReason)
		}
		if shouldFinalizeRebate {
			FinalizeTaskConsumeRebate(ctx, task)
		}
	}
	return nil
}

// taskNeedsUpdate 检查 Suno 任务是否需要更新
func taskNeedsUpdate(oldTask *model.Task, newTask dto.SunoDataResponse) bool {
	if oldTask.SubmitTime != newTask.SubmitTime {
		return true
	}
	if oldTask.StartTime != newTask.StartTime {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if string(oldTask.Status) != newTask.Status {
		return true
	}
	if oldTask.FailReason != newTask.FailReason {
		return true
	}

	if (oldTask.Status == model.TaskStatusFailure || oldTask.Status == model.TaskStatusSuccess) && oldTask.Progress != "100%" {
		return true
	}

	oldData, _ := common.Marshal(oldTask.Data)
	newData, _ := common.Marshal(newTask.Data)

	sort.Slice(oldData, func(i, j int) bool {
		return oldData[i] < oldData[j]
	})
	sort.Slice(newData, func(i, j int) bool {
		return newData[i] < newData[j]
	})

	if string(oldData) != string(newData) {
		return true
	}
	return false
}

// UpdateVideoTasks 按渠道更新所有视频任务
func UpdateVideoTasks(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelId, taskIds := range taskChannelM {
		if err := updateVideoTasks(ctx, platform, channelId, taskIds, taskM); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Channel #%d failed to update video async tasks: %s", channelId, err.Error()))
		}
	}
	return nil
}

func updateVideoTasks(ctx context.Context, platform constant.TaskPlatform, channelId int, taskMapKeys []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("Channel #%d pending video tasks: %d", channelId, len(taskMapKeys)))
	if len(taskMapKeys) == 0 {
		return nil
	}
	cacheGetChannel, err := model.CacheGetChannel(channelId)
	if err != nil {
		// Collect DB primary key IDs for bulk update (map keys are internal lookup keys, not task_id column values)
		var failedIDs []int64
		for _, taskMapKey := range taskMapKeys {
			if t, ok := taskM[taskMapKey]; ok {
				failedIDs = append(failedIDs, t.ID)
			}
		}
		failureReason := fmt.Sprintf("Failed to get channel info, channel ID: %d", channelId)
		errUpdate := model.TaskBulkUpdateByID(failedIDs, map[string]any{
			"fail_reason": failureReason,
			"status":      "FAILURE",
			"progress":    "100%",
		})
		if errUpdate != nil {
			common.SysLog(fmt.Sprintf("UpdateVideoTask error: %v", errUpdate))
		}
		return fmt.Errorf("CacheGetChannel failed: %w", err)
	}
	adaptor := GetTaskAdaptorFunc(platform)
	if adaptor == nil {
		return fmt.Errorf("video adaptor not found")
	}
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelBaseUrl: cacheGetChannel.GetBaseURL(),
	}
	info.ApiKey = cacheGetChannel.Key
	adaptor.Init(info)
	for _, taskMapKey := range taskMapKeys {
		if err := updateVideoSingleTask(ctx, adaptor, cacheGetChannel, taskMapKey, taskM); err != nil {
			taskID := taskMapKey
			if task := taskM[taskMapKey]; task != nil {
				taskID = task.TaskID
			}
			logger.LogError(ctx, fmt.Sprintf("Failed to update video task %s: %s", taskID, err.Error()))
		}
		// sleep 1 second between each task to avoid hitting rate limits of upstream platforms
		time.Sleep(1 * time.Second)
	}
	return nil
}

func updateVideoSingleTask(ctx context.Context, adaptor TaskPollingAdaptor, ch *model.Channel, taskMapKey string, taskM map[string]*model.Task) error {
	baseURL := constant.ChannelBaseURLs[ch.Type]
	if ch.GetBaseURL() != "" {
		baseURL = ch.GetBaseURL()
	}
	proxy := ch.GetSetting().Proxy

	task := taskM[taskMapKey]
	if task == nil {
		logger.LogError(ctx, "Task not found in taskM")
		return errors.New("task not found in taskM")
	}
	key, err := authorizationKeyForPolling(task, ch)
	if err != nil {
		return err
	}
	upstreamID, shouldPoll := upstreamTaskIDForPolling(task)
	if !shouldPoll {
		logger.LogWarn(ctx, fmt.Sprintf("refuse to poll MiniMax-H3 task %s without a real upstream task id", task.TaskID))
		return nil
	}
	resp, err := adaptor.FetchTask(baseURL, key, map[string]any{
		"task_id":         upstreamID,
		"upscale_task_id": task.PrivateData.MiniMaxH3UpscaleTaskID,
		"action":          task.Action,
		"public_base_url": task.PrivateData.PublicBaseURL,
	}, proxy)
	if err != nil {
		return fmt.Errorf("fetchTask failed for task %s: %w", task.TaskID, err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("readAll failed for task %s: %w", task.TaskID, err)
	}

	logger.LogDebug(ctx, fmt.Sprintf("updateVideoSingleTask response: %s", string(responseBody)))

	snap := task.Snapshot()

	taskResult := &relaycommon.TaskInfo{}
	// try parse as All Router response format
	var responseItems dto.TaskResponse[model.Task]
	if err = common.Unmarshal(responseBody, &responseItems); err == nil && responseItems.IsSuccess() {
		logger.LogDebug(ctx, fmt.Sprintf("updateVideoSingleTask parsed as All Router response format: %+v", responseItems))
		t := responseItems.Data
		taskResult.TaskID = t.TaskID
		taskResult.Status = string(t.Status)
		taskResult.Url = t.GetResultURL()
		taskResult.Progress = t.Progress
		taskResult.Reason = t.FailReason
		task.Data = t.Data
	} else if taskResult, err = adaptor.ParseTaskResult(responseBody); err != nil {
		return fmt.Errorf("parseTaskResult failed for task %s: %w", task.TaskID, err)
	}
	if processor, ok := adaptor.(interface {
		ProcessTaskResultBeforePersist(context.Context, *model.Task, *relaycommon.TaskInfo) error
	}); ok {
		if err := processor.ProcessTaskResultBeforePersist(ctx, task, taskResult); err != nil {
			return fmt.Errorf("process task result failed for task %s: %w", task.TaskID, err)
		}
	}
	if taskResult.Stage == "minimax_h3_upscale" {
		switch model.TaskStatus(taskResult.Status) {
		case model.TaskStatusSuccess:
			task.PrivateData.MiniMaxH3UpscaleStatus = "completed"
		case model.TaskStatusFailure:
			task.PrivateData.MiniMaxH3UpscaleStatus = "failed"
		case model.TaskStatusQueued:
			task.PrivateData.MiniMaxH3UpscaleStatus = "queued"
		default:
			task.PrivateData.MiniMaxH3UpscaleStatus = "running"
		}
	}
	if taskResult.Stage == "minimax_h3_upscale" &&
		taskResult.Status != model.TaskStatusSuccess &&
		taskResult.Status != model.TaskStatusFailure {
		// Keep a terminal sentinel in storage while the chained service is still
		// running. Older gateway instances sharing this database will ignore the
		// row instead of overwriting private_data with the 768P source result.
		task.Status = model.TaskStatusSuccess
		task.Action = constant.TaskActionMiniMaxH3Upscale
		if taskResult.Progress != "" {
			task.Progress = minimaxH3UpscalePublicProgress(taskResult.Progress)
		} else {
			task.Progress = "90%"
		}
		if !snap.Equal(task.Snapshot()) {
			won, updateErr := task.UpdateWithStatus(snap.Status)
			if updateErr != nil {
				return fmt.Errorf("persist MiniMax-H3 upscale progress for task %s: %w", task.TaskID, updateErr)
			}
			if !won {
				logger.LogWarn(ctx, fmt.Sprintf("Task %s changed concurrently, skip stale upscale progress", task.TaskID))
			}
		}
		return nil
	}

	preserveTaskData := constant.IsMiniMaxH3Model(task.Properties.OriginModelName)
	if policy, ok := adaptor.(interface{ PreserveTaskDataOnPoll() bool }); ok && policy.PreserveTaskDataOnPoll() {
		preserveTaskData = true
	}
	if !preserveTaskData {
		task.Data = redactVideoResponseBody(responseBody)
	}

	logger.LogDebug(ctx, fmt.Sprintf("updateVideoSingleTask taskResult: %+v", taskResult))

	now := time.Now().Unix()
	if taskResult.Status == "" {
		//taskResult = relaycommon.FailTaskInfo("upstream returned empty status")
		errorResult := &dto.GeneralErrorResponse{}
		if err = common.Unmarshal(responseBody, &errorResult); err == nil {
			openaiError := errorResult.TryToOpenAIError()
			if openaiError != nil {
				// 返回规范的 OpenAI 错误格式，提取错误信息，判断错误是否为任务失败
				if openaiError.Code == "429" {
					// 429 错误通常表示请求过多或速率限制，暂时不认为是任务失败，保持原状态等待下一轮轮询
					return nil
				}

				// 其他错误认为是任务失败，记录错误信息并更新任务状态
				taskResult = relaycommon.FailTaskInfo("upstream returned error")
			} else if constant.IsMiniMaxH3Model(task.Properties.OriginModelName) && errorResult.ToMessage() != "" {
				taskResult = relaycommon.FailTaskInfo(errorResult.ToMessage())
			} else {
				// unknown error format, log original response
				logger.LogError(ctx, fmt.Sprintf("Task %s returned empty status with unrecognized error format, response: %s", task.TaskID, string(responseBody)))
				taskResult = relaycommon.FailTaskInfo("upstream returned unrecognized message")
			}
		}
	}
	if taskResult.Stage == "minimax_h3_upscale" && taskResult.Status == model.TaskStatusSuccess {
		task.PrivateData.ResultURL = taskResult.Url
	}

	shouldRefund := false
	shouldSettle := false
	quota := task.Quota

	task.Status = model.TaskStatus(taskResult.Status)
	switch taskResult.Status {
	case model.TaskStatusSubmitted:
		task.Progress = taskcommon.ProgressSubmitted
	case model.TaskStatusQueued:
		task.Progress = taskcommon.ProgressQueued
	case model.TaskStatusInProgress:
		task.Progress = taskcommon.ProgressInProgress
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case model.TaskStatusSuccess:
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if strings.HasPrefix(taskResult.Url, "data:") {
			// data: URI (e.g. Vertex base64 encoded video) — keep in Data, not in ResultURL
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		} else if taskResult.Url != "" {
			// Direct upstream URL (e.g. Kling, Ali, Doubao, etc.)
			task.PrivateData.ResultURL = taskResult.Url
		} else {
			// No URL from adaptor — construct proxy URL using public task ID
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		}
		shouldSettle = true
	case model.TaskStatusFailure:
		logger.LogJson(ctx, fmt.Sprintf("Task %s failed", task.TaskID), task)
		task.Status = model.TaskStatusFailure
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Reason
		logger.LogInfo(ctx, fmt.Sprintf("Task %s failed: %s", task.TaskID, task.FailReason))
		taskResult.Progress = taskcommon.ProgressComplete
		if quota != 0 {
			shouldRefund = true
		}
	default:
		return fmt.Errorf("unknown task status %s for task %s", taskResult.Status, task.TaskID)
	}
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}

	isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
	if isDone && snap.Status != task.Status {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("UpdateWithStatus failed for task %s: %s", task.TaskID, err.Error()))
			shouldRefund = false
			shouldSettle = false
		} else if !won {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s already transitioned by another process, skip billing", task.TaskID))
			shouldRefund = false
			shouldSettle = false
		}
	} else if !snap.Equal(task.Snapshot()) {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("Failed to update task %s: %s", task.TaskID, err.Error()))
		} else if !won {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s changed concurrently, skip stale non-terminal update", task.TaskID))
		}
	} else {
		// No changes, skip update
		logger.LogDebug(ctx, fmt.Sprintf("No update needed for task %s", task.TaskID))
	}

	if shouldSettle {
		settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)
	}
	if shouldRefund {
		RefundTaskQuota(ctx, task, task.FailReason)
	}
	if isDone && task.Action == constant.TaskActionMiniMaxH3Generate {
		TriggerMiniMaxH3Dispatch()
	}

	return nil
}

// minimaxH3UpscalePublicProgress reserves 90-99% for the second stage so the
// public progress never moves backwards to the upscale service's raw 0-100%.
func minimaxH3UpscalePublicProgress(raw string) string {
	value, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(raw), "%"))
	if err != nil || value < 0 {
		return "90%"
	}
	if value > 100 {
		value = 100
	}
	if value >= 100 {
		return "99%"
	}
	return fmt.Sprintf("%d%%", 90+value/10)
}

func redactVideoResponseBody(body []byte) []byte {
	var m map[string]any
	if err := common.Unmarshal(body, &m); err != nil {
		return body
	}
	resp, _ := m["response"].(map[string]any)
	if resp != nil {
		delete(resp, "bytesBase64Encoded")
		if v, ok := resp["video"].(string); ok {
			resp["video"] = truncateBase64(v)
		}
		if vs, ok := resp["videos"].([]any); ok {
			for i := range vs {
				if vm, ok := vs[i].(map[string]any); ok {
					delete(vm, "bytesBase64Encoded")
				}
			}
		}
	}
	b, err := common.Marshal(m)
	if err != nil {
		return body
	}
	return b
}

func truncateBase64(s string) string {
	const maxKeep = 256
	if len(s) <= maxKeep {
		return s
	}
	return s[:maxKeep] + "..."
}

// settleTaskBillingOnComplete 任务完成时的统一计费调整。
// 优先级：1. adaptor.AdjustBillingOnComplete 返回正数 → 使用 adaptor 计算的额度
//
//  2. taskResult.TotalTokens > 0 → 按 token 重算
//  3. 都不满足 → 保持预扣额度不变
func settleTaskBillingOnComplete(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo) {
	// 无论哪种结算路径（按次/按token/adaptor），任务完成后都需要触发消费返利终态结算。
	// 使用 defer 确保 FinalizeTaskConsumeRebate 在所有差额调整完成后执行，
	// 此时 task.PrivateData.WalletPaidUsed 已经是最终的充值消耗值。
	defer FinalizeTaskConsumeRebate(ctx, task)
	defer FinalizeTaskProviderProfit(ctx, task)
	RecordTaskTotalTokenUsage(ctx, task, taskResult.TotalTokens)

	// 0. 按次计费的任务不做差额结算
	if bc := task.PrivateData.BillingContext; bc != nil && bc.PerCallBilling {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 按次计费，跳过差额结算", task.TaskID))
		return
	}
	// 1. 优先让 adaptor 决定最终额度
	if actualQuota := adaptor.AdjustBillingOnComplete(task, taskResult); actualQuota > 0 {
		RecalculateTaskQuota(ctx, task, actualQuota, "adaptor计费调整")
		return
	}
	// 2. 回退到 token 重算
	if taskResult.TotalTokens > 0 {
		RecalculateTaskQuotaByTokens(ctx, task, taskResult.TotalTokens)
		return
	}
	// 3. 无调整，保持预扣额度
}
