package autodl

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	workflowText      = "minimax_h3_lightx2v_no_pic"
	workflowFrames    = "minimax_h3_lightx2v"
	workflowReference = "minimax_h3_image_audio_to_video_v2"
	fixedDuration     = 10
	resultUploadLimit = 5
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
	proxy   string
}

type autoDLResult struct {
	URL      string `json:"url"`
	Type     string `json:"type"`
	FileType string `json:"file_type"`
}

type autoDLData struct {
	TaskID  string         `json:"task_id"`
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Results []autoDLResult `json:"results"`
}

type autoDLResponse struct {
	Msg       string     `json:"msg"`
	Code      string     `json:"code"`
	Data      autoDLData `json:"data"`
	RequestID string     `json:"request_id"`
}

type autoDLErrorResponse struct {
	Code  string `json:"code"`
	Msg   string `json:"msg"`
	Error struct {
		Code    string `json:"code"`
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func normalizeTask(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "t2va", "t2v", "text2video", "text-to-video":
		return "t2va"
	case "fl2va", "fl2v", "firsttail", "first_tail", "first-tail":
		return "fl2va"
	case "ref2va", "ref2v", "reference", "reference_video":
		return "ref2va"
	default:
		return ""
	}
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if err := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionTextGenerate); err != nil {
		return err
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	task := normalizeTask(req.Task)
	if task == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("task must be one of t2va, fl2va, ref2va"), "invalid_task", http.StatusBadRequest)
	}
	req.Task = task
	c.Set("task_request", req)
	if len(req.Target) > 0 && string(req.Target) != "null" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("target is not supported; use short_edge"), "invalid_request", http.StatusBadRequest)
	}
	shortEdge, err := service.MiniMaxH3RequestedShortEdge(req.ShortEdge)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if err := service.ValidateMiniMaxH3UpscaleConfig(shortEdge); err != nil {
		return service.TaskErrorWrapperLocal(err, "upscale_service_not_configured", http.StatusServiceUnavailable)
	}
	// Persist the requested target and task type through the same context keys
	// used by the Sora adaptor. The controller otherwise defaults the dedicated
	// field to 768P, which would make a 1536P AutoDL task look like a source task
	// during polling and response conversion.
	c.Set("minimax_h3_output_short_edge", shortEdge)
	c.Set("minimax_h3_task_type", task)
	// 与 sora 渠道统一：三个任务类型（t2va/fl2va/ref2va）共用 minimaxH3Generate，
	// 使 AutoDL 提交同样进入本地 MiniMax-H3 并发队列统计与派发；具体任务类型
	// 持久化在 PrivateData.MiniMaxH3TaskType，由派发阶段选择对应 workflow。
	info.Action = constant.TaskActionMiniMaxH3Generate
	return nil
}

// workflow 仅供直连提交路径兼容使用。MiniMax-H3 提交统一走本地并发队列，
// 队列派发按持久化的任务类型选择端点（见 workflowForTaskType / SubmitQueuedTask）。
func (a *TaskAdaptor) workflow(info *relaycommon.RelayInfo) (string, error) {
	switch info.Action {
	case constant.TaskActionTextGenerate:
		return workflowText, nil
	case constant.TaskActionFirstTailGenerate:
		return workflowFrames, nil
	case constant.TaskActionReferenceGenerate:
		return workflowReference, nil
	default:
		return "", fmt.Errorf("unsupported AutoDL task action %s", info.Action)
	}
}

func (a *TaskAdaptor) EstimatePerSecondBilling(c *gin.Context, _ *relaycommon.RelayInfo) (string, float64, int, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return "", 0, 0, err
	}
	shortEdge, err := service.MiniMaxH3RequestedShortEdge(req.ShortEdge)
	if err != nil {
		return "", 0, 0, err
	}
	return fmt.Sprintf("%dP", shortEdge), fixedDuration, 1, nil
}
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	workflow, err := a.workflow(info)
	if err != nil {
		return "", err
	}
	return a.baseURL + "/api/v1/comfyui/comfyui_workflow/" + workflow, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func requestRawFields(c *gin.Context) map[string]any {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil
	}
	var fields map[string]any
	if common.Unmarshal(body, &fields) == nil {
		return fields
	}
	return nil
}

func stringField(fields map[string]any, name string) string {
	if value, ok := fields[name].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func (a *TaskAdaptor) uploadMultipartField(ctx context.Context, c *gin.Context, field, filename, contentType, publicBaseURL string) (string, error) {
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return "", err
	}
	files := form.File[field]
	if len(files) == 0 {
		return "", nil
	}
	if len(files) != 1 {
		return "", fmt.Errorf("field %s accepts exactly one file", field)
	}
	file, err := files[0].Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	uploadFilename := files[0].Filename
	if contentType != "" {
		uploadFilename = filename
	}
	if contentType == "" {
		contentType = files[0].Header.Get("Content-Type")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return service.UploadAutoDLMedia(ctx, data, uploadFilename, contentType, publicBaseURL)
}

func decodeMediaValue(value string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "data:") {
		comma := strings.IndexByte(value, ',')
		if comma < 0 {
			return nil, "", fmt.Errorf("invalid data URL")
		}
		header := value[:comma]
		data, err := base64.StdEncoding.DecodeString(value[comma+1:])
		if err != nil {
			return nil, "", err
		}
		mime := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
		return data, mime, nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, "", err
	}
	return data, "application/octet-stream", nil
}

func (a *TaskAdaptor) resolveValue(ctx context.Context, value, filename, contentType, publicBaseURL string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value, nil
	}
	data, mime, err := decodeMediaValue(value)
	if err != nil {
		return "", fmt.Errorf("invalid %s: %w", filename, err)
	}
	if mime == "application/octet-stream" && contentType != "" {
		mime = contentType
	}
	return service.UploadAutoDLMedia(ctx, data, filename, mime, publicBaseURL)
}

func blankJPEG() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 768, 768))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *TaskAdaptor) resolveMedia(ctx context.Context, c *gin.Context, fields map[string]any, names []string, filename, contentType, publicBaseURL string) (string, error) {
	for _, name := range names {
		if value := stringField(fields, name); value != "" {
			return a.resolveValue(ctx, value, filename, contentType, publicBaseURL)
		}
		uploaded, err := a.uploadMultipartField(ctx, c, name, filename, contentType, publicBaseURL)
		if err != nil {
			return "", err
		}
		if uploaded != "" {
			return uploaded, nil
		}
	}
	return "", nil
}

func resolutionFor(req relaycommon.TaskSubmitReq) string {
	value := strings.ToLower(strings.TrimSpace(req.Size))
	if value == "" {
		value = strings.ToLower(strings.TrimSpace(req.AspectRatio))
	}
	if strings.Contains(value, "16:9") {
		return "768p\u6a2a"
	}
	if strings.Contains(value, "9:16") {
		return "768p\u7ad6"
	}
	if strings.Contains(value, "x") {
		parts := strings.SplitN(value, "x", 2)
		if len(parts) == 2 {
			w, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			h, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if e1 == nil && e2 == nil && w > h {
				return "768p\u6a2a"
			}
		}
	}
	return "768p\u7ad6"
}
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	task := normalizeTask(req.Task)
	publicBaseURL := common.GetRequestBaseURL(c, system_setting.ServerAddress)
	fields := requestRawFields(c)
	body := map[string]any{
		"prompt":     req.Prompt,
		"duration":   fixedDuration,
		"resolution": resolutionFor(req),
	}
	if task == "ref2va" {
		videoValue := req.InputReference
		if videoValue == "" && len(req.Images) > 0 {
			videoValue = req.Images[0]
		}
		videoURL, err := a.resolveMedia(c.Request.Context(), c, fields, []string{"input_reference", "reference_video", "ref_video", "ref_audio_0", "images"}, "reference.m4a", "audio/mp4", publicBaseURL)
		if videoURL == "" && videoValue != "" {
			videoURL, err = a.resolveValue(c.Request.Context(), videoValue, "reference.m4a", "audio/mp4", publicBaseURL)
		}
		if err != nil {
			return nil, err
		}
		if videoURL == "" {
			return nil, fmt.Errorf("ref2va requires one reference video")
		}
		blank, err := blankJPEG()
		if err != nil {
			return nil, err
		}
		imageURL, err := service.UploadAutoDLMedia(c.Request.Context(), blank, "autodl-blank.jpg", "image/jpeg", publicBaseURL)
		if err != nil {
			return nil, fmt.Errorf("upload blank reference image: %w", err)
		}
		body["ref_audio_0"] = videoURL
		body["ref_image_0"] = imageURL
		if strings.TrimSpace(req.Seed) != "" {
			seed, err := strconv.ParseInt(strings.TrimSpace(req.Seed), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("seed must be an integer")
			}
			body["seed"] = seed
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf(
			"AutoDL ref2va request: duration=%d resolution=%s seed=%s ref_audio_0=%s ref_image_0=%s",
			fixedDuration,
			body["resolution"],
			strings.TrimSpace(req.Seed),
			videoURL,
			imageURL,
		))
	} else if task == "fl2va" {
		first, err := a.resolveMedia(c.Request.Context(), c, fields, []string{"first_frame"}, "first-frame.jpg", "", publicBaseURL)
		if first == "" && len(req.Images) > 0 {
			first, err = a.resolveValue(c.Request.Context(), req.Images[0], "first-frame.jpg", "image/jpeg", publicBaseURL)
		}
		if err != nil {
			return nil, err
		}
		last, err := a.resolveMedia(c.Request.Context(), c, fields, []string{"last_frame"}, "last-frame.jpg", "", publicBaseURL)
		if last == "" && len(req.Images) > 1 {
			last, err = a.resolveValue(c.Request.Context(), req.Images[1], "last-frame.jpg", "image/jpeg", publicBaseURL)
		}
		if err != nil {
			return nil, err
		}
		if first == "" || last == "" {
			return nil, fmt.Errorf("fl2va requires first_frame and last_frame")
		}
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("AutoDL fl2va first_frame URL: %s", first))
		logger.LogInfo(c.Request.Context(), fmt.Sprintf("AutoDL fl2va last_frame URL: %s", last))
		body["first_frame"] = first
		body["last_frame"] = last
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *dto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	resp.Body.Close()
	var parsed autoDLResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		return "", body, service.TaskErrorWrapper(err, "invalid_response", http.StatusBadGateway)
	}
	if !strings.EqualFold(parsed.Code, "success") || parsed.Data.TaskID == "" {
		return "", body, a.NormalizeHTTPError(body, http.StatusBadGateway)
	}
	video := dto.NewOpenAIVideo()
	video.ID = info.PublicTaskID
	video.TaskID = info.PublicTaskID
	video.Model = info.OriginModelName
	video.CreatedAt = time.Now().Unix()
	video.Seconds = strconv.Itoa(fixedDuration)
	video.Size = "768P"
	if req, requestErr := relaycommon.GetTaskRequest(c); requestErr == nil {
		if shortEdge, resolutionErr := service.MiniMaxH3RequestedShortEdge(req.ShortEdge); resolutionErr == nil && shortEdge == service.MiniMaxH3UpscaleShortEdge {
			video.Size = "1536P"
		} else if size := strings.TrimSpace(req.Size); size != "" {
			video.Size = size
		} else if aspectRatio := strings.TrimSpace(req.AspectRatio); aspectRatio != "" {
			video.Size = aspectRatio
		}
	}
	taskData, err := common.Marshal(video)
	if err != nil {
		return "", body, service.TaskErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
	}
	c.Data(http.StatusOK, "application/json", taskData)
	return parsed.Data.TaskID, taskData, nil
}

func (a *TaskAdaptor) NormalizeHTTPError(body []byte, statusCode int) *dto.TaskError {
	var upstream autoDLErrorResponse
	_ = common.Unmarshal(body, &upstream)

	message := strings.TrimSpace(upstream.Error.Message)
	if message == "" {
		message = strings.TrimSpace(upstream.Msg)
	}
	code := strings.TrimSpace(upstream.Error.Code)
	if code == "" {
		code = strings.TrimSpace(upstream.Error.Type)
	}
	if code == "" && !strings.EqualFold(upstream.Code, "success") {
		code = strings.TrimSpace(upstream.Code)
	}

	if message == "" {
		var parsed autoDLResponse
		if common.Unmarshal(body, &parsed) == nil {
			message = strings.TrimSpace(parsed.Data.Message)
			if message == "" {
				message = strings.TrimSpace(parsed.Msg)
			}
			if code == "" && !strings.EqualFold(parsed.Code, "success") {
				code = strings.TrimSpace(parsed.Code)
			}
		}
	}
	if message == "" {
		message = "AutoDL request failed"
	}
	if code == "" {
		code = "autodl_error"
	}
	if strings.EqualFold(code, "RequestParameterIsWrong") {
		statusCode = http.StatusBadRequest
	}
	if statusCode < 400 || statusCode > 599 {
		statusCode = http.StatusBadGateway
	}
	return service.TaskErrorWrapper(fmt.Errorf("%s", message), code, statusCode)
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	a.proxy = proxy
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	if upscaleID, ok := body["upscale_task_id"].(string); ok && strings.TrimSpace(upscaleID) != "" {
		status, err := service.FetchMiniMaxH3Upscale(context.Background(), upscaleID)
		if err != nil {
			return nil, err
		}
		payload, err := common.Marshal(map[string]any{
			"status": strings.ToLower(status.Status), "progress": status.Progress,
			"url": status.URL, "message": status.Reason, "minimax_h3_upscale": true,
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v1/comfyui/comfyui_workflow/result/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var upscale struct {
		Status   string `json:"status"`
		Progress int    `json:"progress"`
		URL      string `json:"url"`
		Message  string `json:"message"`
		Upscale  bool   `json:"minimax_h3_upscale"`
	}
	if common.Unmarshal(body, &upscale) == nil && upscale.Upscale {
		result := &relaycommon.TaskInfo{Code: 0, Stage: "minimax_h3_upscale"}
		switch strings.ToLower(strings.TrimSpace(upscale.Status)) {
		case "queued", "pending":
			result.Status = model.TaskStatusQueued
		case "running", "processing", "in_progress":
			result.Status = model.TaskStatusInProgress
		case "completed", "success", "succeeded", "done", "finished":
			result.Status = model.TaskStatusSuccess
			result.Url = upscale.URL
		case "failed", "failure", "error", "cancelled", "canceled":
			result.Status = model.TaskStatusFailure
			result.Reason = strings.TrimSpace(upscale.Message)
			if result.Reason == "" {
				result.Reason = "video upscale failed"
			}
		default:
			return nil, fmt.Errorf("unknown upscale task status: %s", upscale.Status)
		}
		if upscale.Progress > 0 && upscale.Progress < 100 {
			result.Progress = fmt.Sprintf("%d%%", upscale.Progress)
		}
		return result, nil
	}
	var parsed autoDLResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		return nil, errors.Wrap(err, "unmarshal AutoDL result")
	}
	if !strings.EqualFold(parsed.Code, "success") {
		reason := parsed.Msg
		if reason == "" {
			reason = parsed.Data.Message
		}
		return &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Reason: reason}, nil
	}
	result := &relaycommon.TaskInfo{}
	switch strings.ToUpper(strings.TrimSpace(parsed.Data.Status)) {
	case "QUEUED", "PENDING", "SUBMITTED", "CREATED", "WAITING":
		result.Status = model.TaskStatusQueued
		result.Progress = "20%"
	case "RUNNING", "PROCESSING", "IN_PROGRESS", "EXECUTING":
		result.Status = model.TaskStatusInProgress
		result.Progress = "30%"
	case "SUCCESS", "SUCCEEDED", "COMPLETED":
		result.Status = model.TaskStatusSuccess
		result.Progress = "100%"
		if len(parsed.Data.Results) == 0 || strings.TrimSpace(parsed.Data.Results[0].URL) == "" {
			result.Status = model.TaskStatusFailure
			result.Reason = "AutoDL returned no result URL"
		} else {
			result.Url = parsed.Data.Results[0].URL
		}
	case "FAILED", "FAILURE", "ERROR", "CANCELED", "CANCELLED":
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = parsed.Data.Message
		if result.Reason == "" {
			result.Reason = parsed.Msg
		}
	default:
		return nil, fmt.Errorf("unknown AutoDL task status: %s", parsed.Data.Status)
	}
	return result, nil
}

// ProcessTaskResultBeforePersist makes the durable media upload part of the
// task's success condition. One upload is attempted per polling cycle.
func (a *TaskAdaptor) ProcessTaskResultBeforePersist(ctx context.Context, task *model.Task, result *relaycommon.TaskInfo) error {
	if task == nil || result == nil || result.Status != model.TaskStatusSuccess {
		return nil
	}
	if result.Stage == "minimax_h3_upscale" {
		service.PersistMiniMaxH3UpscaleResult(ctx, task, result)
		return nil
	}
	recoveringSource := strings.EqualFold(strings.TrimSpace(task.PrivateData.MiniMaxH3UpscaleStatus), "recovering_source")
	if (task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure) && !recoveringSource {
		return nil
	}
	if task.MiniMaxH3RequestedShortEdge() == service.MiniMaxH3UpscaleShortEdge {
		if strings.TrimSpace(task.PrivateData.MiniMaxH3UpscaleTaskID) != "" {
			result.Status = model.TaskStatusInProgress
			result.Progress = "90%"
			result.Url = ""
			return nil
		}
		if err := service.ValidateMiniMaxH3UpscaleConfig(service.MiniMaxH3UpscaleShortEdge); err != nil {
			return err
		}
		upscaleID, err := service.SubmitMiniMaxH3Upscale(ctx, result.Url, "", a.proxy, task.TaskID)
		if err != nil {
			task.PrivateData.MiniMaxH3UpscaleStatus = "failed"
			result.Status = model.TaskStatusFailure
			result.Progress = "100%"
			result.Reason = fmt.Sprintf("submit video upscale failed: %v", err)
			result.Url = ""
			return nil
		}
		task.PrivateData.MiniMaxH3UpscaleTaskID = upscaleID
		task.PrivateData.MiniMaxH3UpscaleStatus = "queued"
		task.Action = constant.TaskActionMiniMaxH3Upscale
		if task.ID > 0 {
			persisted, persistErr := task.PersistMiniMaxH3UpscaleSubmission()
			if persistErr != nil {
				return fmt.Errorf("persist MiniMax-H3 upscale submission: %w", persistErr)
			}
			if !persisted {
				return fmt.Errorf("persist MiniMax-H3 upscale submission: task is already terminal")
			}
		}
		result.Status = model.TaskStatusInProgress
		result.Progress = "90%"
		result.Url = ""
		logger.LogInfo(ctx, fmt.Sprintf("MiniMax-H3 upscale submitted: task=%s upscale_task=%s", task.TaskID, upscaleID))
		return nil
	}

	persistedURL, err := service.PersistAutoDLResult(ctx, result.Url, task.PrivateData.PublicBaseURL)
	if err == nil {
		task.PrivateData.AutoDLResultUploadFailures = 0
		result.Url = persistedURL
		logger.LogInfo(ctx, fmt.Sprintf("AutoDL result uploaded successfully: task=%s url=%s", task.TaskID, persistedURL))
		return nil
	}

	task.PrivateData.AutoDLResultUploadFailures++
	failures := task.PrivateData.AutoDLResultUploadFailures
	result.Url = ""
	if failures >= resultUploadLimit {
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = fmt.Sprintf("upload AutoDL result failed after %d attempts: %v", resultUploadLimit, err)
		logger.LogError(ctx, fmt.Sprintf("AutoDL result upload permanently failed: task=%s attempts=%d error=%v", task.TaskID, failures, err))
		return nil
	}

	result.Status = model.TaskStatusInProgress
	result.Progress = "95%"
	logger.LogWarn(ctx, fmt.Sprintf("AutoDL result upload failed, will retry: task=%s attempt=%d/%d error=%v", task.TaskID, failures, resultUploadLimit, err))
	return nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{constant.ModelMiniMaxH3, constant.ModelMiniMaxH3Ref2va}
}

func (a *TaskAdaptor) GetChannelName() string { return "autodl" }

// PreserveTaskDataOnPoll keeps the canonical gateway response stored at submit time.
// AutoDL status responses do not contain request fields such as size and seconds.
func (a *TaskAdaptor) PreserveTaskDataOnPoll() bool { return true }

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	response := make(map[string]any)
	if len(task.Data) > 0 {
		_ = common.Unmarshal(task.Data, &response)
	}
	// MiniMax-H3 与 sora 渠道共用统一响应契约（service.NormalizeMiniMaxH3VideoResponse），
	// 客户端无法感知任务实际由哪个渠道承载。
	return common.Marshal(service.NormalizeMiniMaxH3VideoResponse(task, response))
}

// workflowForTaskType 由任务类型（t2va/fl2va/ref2va）选择 AutoDL workflow。
// 队列派发阶段无法依赖 info.Action（已统一为 minimaxH3Generate），
// 需要按持久化的任务类型选择提交端点。
func workflowForTaskType(taskType string) (string, error) {
	switch taskType {
	case "t2va":
		return workflowText, nil
	case "fl2va":
		return workflowFrames, nil
	case "ref2va":
		return workflowReference, nil
	default:
		return "", fmt.Errorf("unsupported AutoDL MiniMax-H3 task type %q", taskType)
	}
}
