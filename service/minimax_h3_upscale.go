package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const MiniMaxH3UpscalePersistLimit = 5

type MiniMaxH3UpscaleStatus struct {
	Status   string
	Progress int
	URL      string
	Reason   string
}

// SubmitMiniMaxH3Upscale downloads the completed 768P source and submits it
// to the configured upscale service as a multipart file.
func SubmitMiniMaxH3Upscale(ctx context.Context, sourceURL, sourceKey, sourceProxy, idempotencyKey string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(constant.MiniMaxH3UpscaleURL), "/")
	if baseURL == "" || strings.TrimSpace(constant.MiniMaxH3UpscaleAPIKey) == "" {
		return "", fmt.Errorf("MiniMax-H3 upscale service is not configured")
	}
	endpoint := baseURL + "/api/video/upscale"
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", fmt.Errorf("build source request: %w", err)
	}
	if sourceKey != "" {
		getReq.Header.Set("Authorization", "Bearer "+sourceKey)
	}
	sourceClient, err := miniMaxH3UpscaleHTTPClient(sourceProxy)
	if err != nil {
		return "", err
	}
	resp, err := sourceClient.Do(getReq)
	if err != nil {
		return "", fmt.Errorf("download source video: %w", err)
	}
	defer CloseResponseBodyGracefully(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download source video status %d", resp.StatusCode)
	}
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	go func() {
		part, writeErr := multipartWriter.CreateFormFile("file", "minimax-h3.mp4")
		if writeErr == nil {
			_, writeErr = io.Copy(part, resp.Body)
		}
		if closeErr := multipartWriter.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
	}()
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		return "", err
	}
	defer pipeReader.Close()
	postReq.Header.Set("Authorization", "Bearer "+constant.MiniMaxH3UpscaleAPIKey)
	postReq.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	if strings.TrimSpace(idempotencyKey) != "" {
		postReq.Header.Set("Idempotency-Key", idempotencyKey)
	}
	upscaleClient, err := miniMaxH3UpscaleHTTPClient("")
	if err != nil {
		return "", err
	}
	postResp, err := upscaleClient.Do(postReq)
	if err != nil {
		return "", fmt.Errorf("submit upscale task: %w", err)
	}
	defer CloseResponseBodyGracefully(postResp)
	responseBody, err := io.ReadAll(io.LimitReader(postResp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if postResp.StatusCode < 200 || postResp.StatusCode >= 300 {
		return "", fmt.Errorf("upscale submit status %d: %s", postResp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	var parsed struct {
		TaskID string `json:"task_id"`
	}
	if err := common.Unmarshal(responseBody, &parsed); err != nil || strings.TrimSpace(parsed.TaskID) == "" {
		return "", fmt.Errorf("upscale submit response has no task_id")
	}
	return parsed.TaskID, nil
}

func FetchMiniMaxH3Upscale(ctx context.Context, taskID string) (*MiniMaxH3UpscaleStatus, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(constant.MiniMaxH3UpscaleURL), "/")
	if baseURL == "" || strings.TrimSpace(constant.MiniMaxH3UpscaleAPIKey) == "" {
		return nil, fmt.Errorf("MiniMax-H3 upscale service is not configured")
	}
	endpoint := baseURL + "/api/video/tasks/" + url.PathEscape(strings.TrimSpace(taskID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+constant.MiniMaxH3UpscaleAPIKey)
	client, err := miniMaxH3UpscaleHTTPClient("")
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer CloseResponseBodyGracefully(resp)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upscale status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw map[string]any
	if err := common.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	result := &MiniMaxH3UpscaleStatus{}
	result.Status, _ = raw["status"].(string)
	result.URL, _ = raw["url"].(string)
	result.Reason, _ = raw["message"].(string)
	if result.Reason == "" {
		result.Reason, _ = raw["error"].(string)
	}
	if result.URL == "" {
		result.URL, _ = raw["output_url"].(string)
	}
	if result.URL != "" {
		if err := ValidateMiniMaxH3UpscaleOutputURL(result.URL); err != nil {
			result.Status = "failed"
			result.Reason = err.Error()
			result.URL = ""
		}
	}
	if progress, ok := raw["progress"].(float64); ok {
		result.Progress = int(progress)
	}
	return result, nil
}

func ValidateMiniMaxH3UpscaleOutputURL(outputURL string) error {
	trustedOrigin := strings.TrimSpace(constant.MiniMaxH3UpscaleOutputOrigin)
	if trustedOrigin == "" {
		trustedOrigin = strings.TrimSpace(constant.MiniMaxH3UpscaleURL)
	}
	base, err := url.Parse(trustedOrigin)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return fmt.Errorf("invalid MiniMax-H3 upscale output origin")
	}
	output, err := url.Parse(strings.TrimSpace(outputURL))
	if err != nil || (output.Scheme != "http" && output.Scheme != "https") || output.Host == "" {
		return fmt.Errorf("invalid MiniMax-H3 upscale output URL")
	}
	if !strings.EqualFold(base.Scheme, output.Scheme) || !strings.EqualFold(base.Host, output.Host) {
		return fmt.Errorf("MiniMax-H3 upscale output URL must use the configured output origin")
	}
	return nil
}

// PersistMiniMaxH3UpscaleResult copies the converter's temporary output into
// the existing media hub. The original task remains active while retries are
// available and only becomes successful after the durable URL is available.
func PersistMiniMaxH3UpscaleResult(ctx context.Context, task *model.Task, result *relaycommon.TaskInfo) {
	if task == nil || result == nil {
		return
	}
	rawURL := strings.TrimSpace(result.Url)
	if rawURL == "" {
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = "upscale completed without output URL"
		return
	}

	task.PrivateData.MiniMaxH3UpscaleURL = rawURL
	persistedURL, err := PersistVideoResult(ctx, rawURL, task.PrivateData.PublicBaseURL, "minimax-h3-1536p.mp4")
	if err == nil {
		task.PrivateData.MiniMaxH3PersistFailures = 0
		task.PrivateData.MiniMaxH3UpscaleStatus = "completed"
		task.PrivateData.MiniMaxH3UpscaleURL = ""
		result.Url = persistedURL
		result.Status = model.TaskStatusSuccess
		result.Progress = "100%"
		result.Reason = ""
		logger.LogInfo(ctx, fmt.Sprintf("MiniMax-H3 upscale result persisted: task=%s url=%s", task.TaskID, persistedURL))
		return
	}

	task.PrivateData.MiniMaxH3PersistFailures++
	failures := task.PrivateData.MiniMaxH3PersistFailures
	result.Url = ""
	if failures >= MiniMaxH3UpscalePersistLimit {
		task.PrivateData.MiniMaxH3UpscaleStatus = "failed"
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = fmt.Sprintf("persist upscale result failed after %d attempts: %v", MiniMaxH3UpscalePersistLimit, err)
		logger.LogError(ctx, fmt.Sprintf("MiniMax-H3 upscale result persistence permanently failed: task=%s attempts=%d error=%v", task.TaskID, failures, err))
		return
	}

	task.PrivateData.MiniMaxH3UpscaleStatus = "persisting"
	result.Status = model.TaskStatusInProgress
	result.Progress = "98%"
	result.Reason = ""
	logger.LogWarn(ctx, fmt.Sprintf("MiniMax-H3 upscale result persistence failed, will retry: task=%s attempt=%d/%d error=%v", task.TaskID, failures, MiniMaxH3UpscalePersistLimit, err))
}

func miniMaxH3UpscaleHTTPClient(proxy string) (*http.Client, error) {
	baseClient, err := GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	if baseClient == nil {
		baseClient = http.DefaultClient
	}
	client := *baseClient
	client.Timeout = time.Duration(constant.MiniMaxH3UpscaleTimeoutSeconds) * time.Second
	return &client, nil
}
