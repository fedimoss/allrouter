package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const (
	miniMaxH3DispatchTimeout   = 5 * time.Minute
	miniMaxH3RequestTimeout    = 2 * time.Minute
	miniMaxH3DispatchBatchSize = 100
)

type MiniMaxH3SubmitResult struct {
	UpstreamTaskID string
	TaskData       []byte
}

var SubmitMiniMaxH3TaskFunc func(context.Context, *model.Task) (*MiniMaxH3SubmitResult, error)

var (
	miniMaxH3DispatchOnce   sync.Once
	miniMaxH3DispatchSignal = make(chan struct{}, 1)
)

func StartMiniMaxH3Dispatcher() {
	miniMaxH3DispatchOnce.Do(func() {
		go func() {
			for range miniMaxH3DispatchSignal {
				dispatchMiniMaxH3Tasks(context.Background())
			}
		}()
	})
	TriggerMiniMaxH3Dispatch()
}

func TriggerMiniMaxH3Dispatch() {
	select {
	case miniMaxH3DispatchSignal <- struct{}{}:
	default:
	}
}

func dispatchMiniMaxH3Tasks(ctx context.Context) {
	if SubmitMiniMaxH3TaskFunc == nil {
		return
	}
	cleanupStaleMiniMaxH3Dispatches(ctx)

	tasks, err := model.ClaimMiniMaxH3Tasks(constant.MiniMaxH3MaxConcurrency)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("claim MiniMax-H3 tasks failed: %v", err))
		return
	}
	for _, task := range tasks {
		task := task
		go submitMiniMaxH3Task(ctx, task)
	}
}

func submitMiniMaxH3Task(ctx context.Context, task *model.Task) {
	requestCtx, cancel := context.WithTimeout(ctx, miniMaxH3RequestTimeout)
	defer cancel()
	result, err := SubmitMiniMaxH3TaskFunc(requestCtx, task)
	if err != nil {
		failMiniMaxH3Dispatch(ctx, task, fmt.Sprintf("submit MiniMax-H3 task failed: %v", err))
		TriggerMiniMaxH3Dispatch()
		return
	}
	if result == nil || result.UpstreamTaskID == "" {
		failMiniMaxH3Dispatch(ctx, task, "submit MiniMax-H3 task returned empty upstream task id")
		TriggerMiniMaxH3Dispatch()
		return
	}

	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	task.PrivateData.PendingRequest = nil
	task.Data = result.TaskData
	task.Status = model.TaskStatusSubmitted
	task.Progress = "10%"
	won, updateErr := task.UpdateWithStatus(model.TaskStatusDispatching)
	if updateErr != nil {
		logger.LogError(ctx, fmt.Sprintf("persist MiniMax-H3 upstream task %s failed: %v", task.TaskID, updateErr))
		return
	}
	if !won {
		logger.LogWarn(ctx, fmt.Sprintf("MiniMax-H3 task %s left dispatching state before submit completed", task.TaskID))
	}
}

func cleanupStaleMiniMaxH3Dispatches(ctx context.Context) {
	cutoff := time.Now().Add(-miniMaxH3DispatchTimeout).Unix()
	tasks, err := model.GetStaleMiniMaxH3DispatchingTasks(cutoff, miniMaxH3DispatchBatchSize)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("query stale MiniMax-H3 dispatches failed: %v", err))
		return
	}
	for _, task := range tasks {
		failMiniMaxH3Dispatch(ctx, task, "MiniMax-H3 upstream submission timed out")
	}
}

func failMiniMaxH3Dispatch(ctx context.Context, task *model.Task, reason string) {
	if task == nil {
		return
	}
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = reason
	task.PrivateData.PendingRequest = nil
	if failedData, err := common.Marshal(map[string]any{
		"id":       task.TaskID,
		"object":   "video",
		"model":    task.Properties.OriginModelName,
		"status":   "failed",
		"progress": 100,
		"error": map[string]any{
			"code":    "upstream_submit_failed",
			"message": reason,
		},
	}); err == nil {
		task.Data = failedData
	}
	won, err := task.UpdateWithStatus(model.TaskStatusDispatching)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("fail MiniMax-H3 task %s failed: %v", task.TaskID, err))
		return
	}
	if won && task.Quota != 0 {
		RefundTaskQuota(ctx, task, reason)
	}
	if won {
		common.SysLog(fmt.Sprintf("MiniMax-H3 task %s failed before upstream acceptance: %s", task.TaskID, reason))
	}
}
