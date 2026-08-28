package sora

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // for text type
	ImageURL *ImageURL `json:"image_url,omitempty"` // for image_url type
}

type ImageURL struct {
	URL string `json:"url"`
}

type perSecondTarget struct {
	ShortEdge       *int     `json:"short_edge,omitempty"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
}

const miniMaxH3RequestBodyKey = "minimax_h3_request_body"

const miniMaxH3FixedShortEdge = 768
const miniMaxH3FixedDurationSeconds = 10

var miniMaxH3AspectRatios = map[string]struct{}{
	"16:9": {},
	"9:16": {},
	"1:1":  {},
	"4:3":  {},
	"3:4":  {},
	"auto": {},
}

type miniMaxH3Target struct {
	ShortEdge       int     `json:"short_edge"`
	AspectRatio     string  `json:"aspect_ratio"`
	DurationSeconds float64 `json:"duration_seconds"`
}

type miniMaxH3Request struct {
	Model               string               `json:"model"`
	Prompt              string               `json:"prompt"`
	Seconds             string               `json:"seconds"`
	Task                string               `json:"task"`
	Conditions          []miniMaxH3Condition `json:"conditions"`
	Target              miniMaxH3Target      `json:"target"`
	NumOutputsPerPrompt int                  `json:"num_outputs_per_prompt"`
	NumInferenceSteps   int                  `json:"num_inference_steps"`
	FlowShift           float64              `json:"flow_shift"`
	AudioFlowShift      float64              `json:"audio_flow_shift"`
	Seed                int64                `json:"seed"`
}

func miniMaxH3RequestedShortEdge(req relaycommon.TaskSubmitReq) (int, error) {
	return service.MiniMaxH3RequestedShortEdge(req.ShortEdge)
}

func validateMiniMaxH3UpscaleConfig(shortEdge int) error {
	return service.ValidateMiniMaxH3UpscaleConfig(shortEdge)
}

type miniMaxH3Condition struct {
	Type             string   `json:"type"`
	URI              string   `json:"uri"`
	Role             string   `json:"role"`
	FrameIndex       *int     `json:"frame_index,omitempty"`
	StartTimeSeconds *float64 `json:"start_time_seconds,omitempty"`
}

type responseTask struct {
	ID                 string `json:"id"`
	TaskID             string `json:"task_id,omitempty"` //兼容旧接口
	Object             string `json:"object"`
	Model              string `json:"model"`
	Status             string `json:"status"`
	Progress           int    `json:"progress"`
	CreatedAt          int64  `json:"created_at"`
	CompletedAt        int64  `json:"completed_at,omitempty"`
	ExpiresAt          int64  `json:"expires_at,omitempty"`
	Seconds            string `json:"seconds,omitempty"`
	Size               string `json:"size,omitempty"`
	NumOutputs         int    `json:"num_outputs,omitempty"`
	RemixedFromVideoID string `json:"remixed_from_video_id,omitempty"`
	Error              *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
	Metadata *struct {
		URL string `json:"url"`
	} `json:"metadata,omitempty"`
	Content *struct {
		URL string `json:"url"`
	} `json:"content,omitempty"`
	URL       string   `json:"url,omitempty"`
	FilePath  string   `json:"file_path,omitempty"`
	FilePaths []string `json:"file_paths,omitempty"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
	proxy       string
	// publicBaseURL 为提交请求时记录的公开域名，轮询时由 FetchTask
	// 从 public_base_url 参数刷新，用于把上游 file_path 拼成公开媒体地址。
	publicBaseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func validateRemixRequest(c *gin.Context) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	// 存储原始请求到 context，与 ValidateMultipartDirect 路径保持一致
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	if info.Action == constant.TaskActionRemix {
		return validateRemixRequest(c)
	}

	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_json", http.StatusBadRequest)
	}
	if req.Model == constant.ModelMiniMaxH3 || req.Model == constant.ModelMiniMaxH3Ref2va {
		return validateMiniMaxH3Request(c, info, req)
	}
	return relaycommon.ValidateMultipartDirect(c, info)
}

func validateMiniMaxH3Request(c *gin.Context, info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) *dto.TaskError {
	if req.Model == constant.ModelMiniMaxH3Ref2va {
		return validateMiniMaxH3Ref2vaRequest(c, info, req)
	}
	return validateMiniMaxH3FramesRequest(c, info, req)
}

func validateMiniMaxH3Ref2vaRequest(c *gin.Context, info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) *dto.TaskError {
	contentType := c.GetHeader("Content-Type")
	isMultipart := strings.Contains(contentType, "multipart/form-data")
	if !isMultipart {
		return service.TaskErrorWrapperLocal(fmt.Errorf("MiniMax-H3-Ref2va requires multipart/form-data"), "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}

	unsupported := make([]string, 0)
	allowedFields := map[string]bool{
		"model": true, "prompt": true, "task": true, "aspect_ratio": true, "seed": true, "short_edge": true,
		"start_time_seconds": true,
	}
	if info.IsPlayground {
		allowedFields["group"] = true
	}

	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_multipart_form", http.StatusBadRequest)
	}
	defer form.RemoveAll()

	for field := range form.Value {
		if !allowedFields[field] {
			unsupported = append(unsupported, field)
		}
	}
	for field := range form.File {
		if field == "first_frame" || field == "last_frame" {
			return service.TaskErrorWrapperLocal(fmt.Errorf("MiniMax-H3-Ref2va does not accept %s; use MiniMax-H3 instead", field), "invalid_request", http.StatusBadRequest)
		}
		if field != "reference_video" {
			unsupported = append(unsupported, field)
		}
	}
	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("MiniMax-H3-Ref2va request contains unsupported fields: %s", strings.Join(unsupported, ", ")),
			"invalid_request",
			http.StatusBadRequest,
		)
	}
	requestedShortEdge, targetErr := miniMaxH3RequestedShortEdge(req)
	if targetErr != nil {
		return service.TaskErrorWrapperLocal(targetErr, "invalid_request", http.StatusBadRequest)
	}
	if configErr := validateMiniMaxH3UpscaleConfig(requestedShortEdge); configErr != nil {
		return service.TaskErrorWrapperLocal(configErr, "upscale_service_not_configured", http.StatusServiceUnavailable)
	}

	taskType := strings.TrimSpace(req.Task)
	if taskType != "" && taskType != "ref2va" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("MiniMax-H3-Ref2va requires task=ref2va"), "invalid_request", http.StatusBadRequest)
	}

	files := form.File["reference_video"]
	if len(files) != 1 {
		return service.TaskErrorWrapperLocal(fmt.Errorf("MiniMax-H3-Ref2va requires exactly one reference_video"), "invalid_request", http.StatusBadRequest)
	}
	videoID, err := service.SaveMiniMaxH3ReferenceVideo(info.UserId, files[0])
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	videoURI, err := service.ResolveMiniMaxH3ReferenceVideoURI(info.UserId, videoID)
	if err != nil {
		_ = service.DeleteMiniMaxH3ReferenceVideo(info.UserId, videoID)
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	var startTime *float64
	startRaw := ""
	if values := form.Value["start_time_seconds"]; len(values) > 0 {
		startRaw = values[0]
	}
	if raw := strings.TrimSpace(startRaw); raw != "" {
		value, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr != nil || value < 0 {
			_ = service.DeleteMiniMaxH3ReferenceVideo(info.UserId, videoID)
			return service.TaskErrorWrapperLocal(fmt.Errorf("start_time_seconds must be non-negative"), "invalid_request", http.StatusBadRequest)
		}
		startTime = &value
	}
	conditions := []miniMaxH3Condition{{Type: "video", URI: videoURI, Role: "reference", StartTimeSeconds: startTime}}
	c.Set(service.MiniMaxH3ReferenceVideoIDsContextKey, []string{videoID})

	return buildMiniMaxH3UpstreamRequest(c, info, req, "ref2va", conditions)
}

func validateMiniMaxH3FramesRequest(c *gin.Context, info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) *dto.TaskError {
	contentType := c.GetHeader("Content-Type")
	isJSON := strings.HasPrefix(contentType, "application/json")
	isMultipart := strings.Contains(contentType, "multipart/form-data")
	if !isJSON && !isMultipart {
		return service.TaskErrorWrapperLocal(fmt.Errorf("MiniMax-H3 requires application/json or multipart/form-data"), "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}

	unsupported := make([]string, 0)
	allowedFields := map[string]bool{
		"model": true, "prompt": true, "task": true, "aspect_ratio": true, "seed": true, "short_edge": true,
	}
	if info.IsPlayground {
		allowedFields["group"] = true
	}

	conditions := make([]miniMaxH3Condition, 0, 2)
	frameIDs := make([]string, 0, 2)
	cleanupFrames := true
	defer func() {
		if cleanupFrames {
			for _, frameID := range frameIDs {
				_ = service.DeleteMiniMaxH3Frame(info.UserId, frameID)
			}
		}
	}()

	if isJSON {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		body, err := storage.Bytes()
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		var fields map[string]any
		if err := common.Unmarshal(body, &fields); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_json", http.StatusBadRequest)
		}
		for field := range fields {
			if !allowedFields[field] {
				unsupported = append(unsupported, field)
			}
		}
	} else {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_multipart_form", http.StatusBadRequest)
		}
		defer form.RemoveAll()
		for field := range form.Value {
			if !allowedFields[field] {
				unsupported = append(unsupported, field)
			}
		}
		for field := range form.File {
			if field == "reference_video" {
				return service.TaskErrorWrapperLocal(fmt.Errorf("MiniMax-H3 does not accept reference_video; use MiniMax-H3-Ref2va instead"), "invalid_request", http.StatusBadRequest)
			}
			if field != "first_frame" && field != "last_frame" {
				unsupported = append(unsupported, field)
			}
		}
		if len(unsupported) == 0 {
			appendFrame := func(field string, frameIndex int) *dto.TaskError {
				files := form.File[field]
				if len(files) == 0 {
					return nil
				}
				if len(files) != 1 {
					return service.TaskErrorWrapperLocal(fmt.Errorf("field %s accepts exactly one image", field), "invalid_request", http.StatusBadRequest)
				}
				frameID, err := service.SaveMiniMaxH3Frame(info.UserId, files[0])
				if err != nil {
					return service.TaskErrorWrapperLocal(fmt.Errorf("invalid %s: %w", field, err), "invalid_request", http.StatusBadRequest)
				}
				frameIDs = append(frameIDs, frameID)
				frameURI, err := service.ResolveMiniMaxH3FrameURI(info.UserId, frameID)
				if err != nil {
					return service.TaskErrorWrapperLocal(fmt.Errorf("resolve %s: %w", field, err), "invalid_request", http.StatusBadRequest)
				}
				conditions = append(conditions, miniMaxH3Condition{
					Type: "image", URI: frameURI, Role: "keyframe", FrameIndex: &frameIndex,
				})
				return nil
			}
			if taskErr := appendFrame("first_frame", 0); taskErr != nil {
				return taskErr
			}
			if taskErr := appendFrame("last_frame", -1); taskErr != nil {
				return taskErr
			}
			if len(frameIDs) > 0 {
				c.Set(service.MiniMaxH3FrameIDsContextKey, frameIDs)
			}
		}
	}
	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("MiniMax-H3 request contains unsupported fields: %s", strings.Join(unsupported, ", ")),
			"invalid_request",
			http.StatusBadRequest,
		)
	}
	requestedShortEdge, targetErr := miniMaxH3RequestedShortEdge(req)
	if targetErr != nil {
		return service.TaskErrorWrapperLocal(targetErr, "invalid_request", http.StatusBadRequest)
	}
	if configErr := validateMiniMaxH3UpscaleConfig(requestedShortEdge); configErr != nil {
		return service.TaskErrorWrapperLocal(configErr, "upscale_service_not_configured", http.StatusServiceUnavailable)
	}

	taskType := strings.TrimSpace(req.Task)
	if taskType == "" {
		if len(conditions) > 0 {
			taskType = "fl2va"
		} else {
			taskType = "t2va"
		}
	}
	if taskType != "t2va" && taskType != "fl2va" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("MiniMax-H3 task must be one of t2va, fl2va"), "invalid_request", http.StatusBadRequest)
	}
	if taskType == "t2va" && len(conditions) > 0 {
		return service.TaskErrorWrapperLocal(fmt.Errorf("t2va does not accept frame conditions"), "invalid_request", http.StatusBadRequest)
	}
	if taskType == "fl2va" && len(conditions) == 0 {
		return service.TaskErrorWrapperLocal(fmt.Errorf("fl2va requires first_frame or last_frame"), "invalid_request", http.StatusBadRequest)
	}

	// prevent frames from being cleaned on successful validation
	cleanupFrames = false
	return buildMiniMaxH3UpstreamRequest(c, info, req, taskType, conditions)
}

func buildMiniMaxH3UpstreamRequest(c *gin.Context, info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq, taskType string, conditions []miniMaxH3Condition) *dto.TaskError {
	aspectRatio := strings.TrimSpace(req.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = "9:16"
	}
	if _, ok := miniMaxH3AspectRatios[aspectRatio]; !ok {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("aspect_ratio must be one of 16:9, 9:16, 1:1, 4:3, 3:4, auto"),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	var (
		seed int64
		err  error
	)
	if rawSeed := strings.TrimSpace(req.Seed); rawSeed != "" {
		parsed, parseErr := strconv.ParseInt(rawSeed, 10, 64)
		if parseErr != nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("seed must be an integer"), "invalid_request", http.StatusBadRequest)
		}
		seed = parsed
	}
	if strings.TrimSpace(req.Seed) == "" && info.IsPlayground {
		seed, err = model.GetOrCreateUserMiniMaxH3Seed(info.UserId)
	} else if strings.TrimSpace(req.Seed) == "" {
		seed, err = model.GetOrCreateTokenMiniMaxH3Seed(info.TokenId)
	}
	if err != nil {
		return service.TaskErrorWrapper(err, "get_or_create_minimax_h3_seed_failed", http.StatusInternalServerError)
	}

	upstreamTarget := miniMaxH3Target{
		ShortEdge:       miniMaxH3FixedShortEdge,
		AspectRatio:     aspectRatio,
		DurationSeconds: miniMaxH3FixedDurationSeconds,
	}
	outputShortEdge, targetErr := miniMaxH3RequestedShortEdge(req)
	if targetErr != nil {
		return service.TaskErrorWrapperLocal(targetErr, "invalid_request", http.StatusBadRequest)
	}
	seconds := strconv.FormatFloat(miniMaxH3FixedDurationSeconds, 'f', -1, 64)
	upstreamRequest := miniMaxH3Request{
		Model:               req.Model,
		Prompt:              req.Prompt,
		Seconds:             seconds,
		Task:                taskType,
		Conditions:          conditions,
		Target:              upstreamTarget,
		NumOutputsPerPrompt: 1,
		NumInferenceSteps:   20,
		FlowShift:           12.0,
		AudioFlowShift:      3.0,
		Seed:                seed,
	}
	outputCount := upstreamRequest.NumOutputsPerPrompt
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:              req.Prompt,
		Model:               req.Model,
		Task:                taskType,
		AspectRatio:         upstreamRequest.Target.AspectRatio,
		Size:                fmt.Sprintf("%dP", outputShortEdge),
		Seconds:             upstreamRequest.Seconds,
		NumOutputsPerPrompt: &outputCount,
	})
	c.Set(miniMaxH3RequestBodyKey, upstreamRequest)
	c.Set("minimax_h3_task_type", taskType)
	if outputShortEdge == 1536 {
		c.Set("minimax_h3_output_short_edge", outputShortEdge)
	}
	info.Action = constant.TaskActionMiniMaxH3Generate
	return nil
}

// EstimateBilling 根据用户请求的 seconds 和 size 计算 OtherRatios。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	// remix 路径的 OtherRatios 已在 ResolveOriginTask 中设置
	if info.Action == constant.TaskActionRemix {
		return nil
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if seconds <= 0 {
		seconds = 4
	}

	size := req.Size
	if size == "" {
		size = "720x1280"
	}

	ratios := map[string]float64{
		"seconds": float64(seconds),
		"size":    1,
	}
	if size == "1792x1024" || size == "1024x1792" {
		ratios["size"] = 1.666667
	}
	return ratios
}

func parsePerSecondTarget(raw []byte) (*perSecondTarget, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var target perSecondTarget
	if err := common.Unmarshal(raw, &target); err != nil {
		return nil, fmt.Errorf("target must contain valid per-second billing parameters: %w", err)
	}
	return &target, nil
}

func parsePerSecondDuration(req relaycommon.TaskSubmitReq, target *perSecondTarget, allowSoraDefault bool) (float64, error) {
	if target != nil && target.DurationSeconds != nil {
		seconds := *target.DurationSeconds
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
			return 0, fmt.Errorf("target.duration_seconds must be a positive number")
		}
		return seconds, nil
	}
	if strings.TrimSpace(req.Seconds) != "" {
		seconds, err := strconv.ParseFloat(strings.TrimSpace(req.Seconds), 64)
		if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
			return 0, fmt.Errorf("seconds must be a positive number")
		}
		return seconds, nil
	}
	if req.Duration != 0 {
		if req.Duration > 0 {
			return float64(req.Duration), nil
		}
		return 0, fmt.Errorf("duration must be positive")
	}
	if allowSoraDefault {
		return 4, nil
	}
	return 0, fmt.Errorf("seconds is required for per-second billing")
}

func parsePerSecondOutputCount(req relaycommon.TaskSubmitReq) (int, error) {
	if req.NumOutputsPerPrompt != nil {
		if *req.NumOutputsPerPrompt <= 0 {
			return 0, fmt.Errorf("num_outputs_per_prompt must be positive")
		}
		return *req.NumOutputsPerPrompt, nil
	}
	if req.NumOutputs != nil {
		if *req.NumOutputs <= 0 {
			return 0, fmt.Errorf("num_outputs must be positive")
		}
		return *req.NumOutputs, nil
	}
	if req.N != nil {
		if *req.N <= 0 {
			return 0, fmt.Errorf("n must be positive")
		}
		return *req.N, nil
	}
	return 1, nil
}

func sizeToBillingResolution(size string) (string, error) {
	size = strings.ToUpper(strings.TrimSpace(size))
	if size == "" {
		return "", nil
	}
	if strings.HasSuffix(size, "P") || strings.HasSuffix(size, "K") {
		return size, nil
	}
	parts := strings.FieldsFunc(size, func(r rune) bool {
		return r == 'X' || r == '*' || r == '×'
	})
	if len(parts) != 2 {
		return "", fmt.Errorf("unsupported video size %s", size)
	}
	width, widthErr := strconv.Atoi(strings.TrimSpace(parts[0]))
	height, heightErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	if widthErr != nil || heightErr != nil || width <= 0 || height <= 0 {
		return "", fmt.Errorf("unsupported video size %s", size)
	}
	shortEdge, longEdge := width, height
	if shortEdge > longEdge {
		shortEdge, longEdge = longEdge, shortEdge
	}
	switch {
	case longEdge >= 3840 || shortEdge >= 2160:
		return "4K", nil
	case longEdge >= 2048 || shortEdge >= 1440:
		return "2K", nil
	default:
		return fmt.Sprintf("%dP", shortEdge), nil
	}
}

func targetToBillingResolution(target *perSecondTarget) (string, error) {
	if target == nil || target.ShortEdge == nil {
		return "", nil
	}
	shortEdge := *target.ShortEdge
	if shortEdge <= 0 {
		return "", fmt.Errorf("target.short_edge must be positive")
	}
	switch {
	case shortEdge >= 2160:
		return "4K", nil
	case shortEdge >= 1440:
		return "2K", nil
	default:
		return fmt.Sprintf("%dP", shortEdge), nil
	}
}

func resolveRemixSourceBilling(originTask *model.Task) (string, float64, error) {
	if originTask == nil {
		return "", 0, fmt.Errorf("origin task is required for remix billing")
	}
	if billingContext := originTask.PrivateData.BillingContext; billingContext != nil &&
		strings.TrimSpace(billingContext.Resolution) != "" && billingContext.UnitCount > 0 {
		originalOutputCount := billingContext.OutputCount
		if originalOutputCount <= 0 {
			originalOutputCount = 1
		}
		seconds := billingContext.UnitCount / float64(originalOutputCount)
		if !math.IsNaN(seconds) && !math.IsInf(seconds, 0) && seconds > 0 {
			return strings.ToUpper(strings.TrimSpace(billingContext.Resolution)), seconds, nil
		}
	}

	var source responseTask
	if err := common.Unmarshal(originTask.Data, &source); err != nil {
		return "", 0, fmt.Errorf("parse origin task billing parameters: %w", err)
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(source.Seconds), 64)
	if err != nil || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return "", 0, fmt.Errorf("origin task has no valid seconds for remix billing")
	}
	resolution, err := sizeToBillingResolution(source.Size)
	if err != nil || resolution == "" {
		return "", 0, fmt.Errorf("origin task has no valid size for remix billing")
	}
	return resolution, seconds, nil
}

// EstimatePerSecondBilling exposes the effective Sora/OpenAI Video request
// parameters used for pre-consumption. Generic Sora-compatible services use
// seconds, size (WxH), and num_outputs/n rather than Hailuo's request fields.
func (a *TaskAdaptor) EstimatePerSecondBilling(c *gin.Context, info *relaycommon.RelayInfo) (string, float64, int, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return "", 0, 0, err
	}
	if info != nil && info.TaskRelayInfo != nil && info.Action == constant.TaskActionRemix {
		originTask, exists, queryErr := model.GetByTaskId(info.UserId, info.OriginTaskID)
		if queryErr != nil {
			return "", 0, 0, fmt.Errorf("query origin task for remix billing: %w", queryErr)
		}
		if !exists {
			return "", 0, 0, fmt.Errorf("origin task not found for remix billing")
		}
		resolution, seconds, resolveErr := resolveRemixSourceBilling(originTask)
		if resolveErr != nil {
			return "", 0, 0, resolveErr
		}
		outputCount, outputErr := parsePerSecondOutputCount(req)
		if outputErr != nil {
			return "", 0, 0, outputErr
		}
		return resolution, seconds, outputCount, nil
	}
	target, err := parsePerSecondTarget(req.Target)
	if err != nil {
		return "", 0, 0, err
	}
	modelName := req.Model
	if info != nil {
		if info.ChannelMeta != nil && info.UpstreamModelName != "" {
			modelName = info.UpstreamModelName
		} else if info.OriginModelName != "" {
			modelName = info.OriginModelName
		}
	}
	allowSoraDefaults := strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "sora-2")
	seconds, err := parsePerSecondDuration(req, target, allowSoraDefaults)
	if err != nil {
		return "", 0, 0, err
	}
	resolution, err := targetToBillingResolution(target)
	if err != nil {
		return "", 0, 0, err
	}
	if resolution == "" {
		size := req.Size
		if strings.TrimSpace(size) == "" && allowSoraDefaults {
			size = "720x1280"
		}
		resolution, err = sizeToBillingResolution(size)
		if err != nil {
			return "", 0, 0, err
		}
	}
	outputCount, err := parsePerSecondOutputCount(req)
	if err != nil {
		return "", 0, 0, err
	}
	return resolution, seconds, outputCount, nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.Action == constant.TaskActionRemix {
		return fmt.Sprintf("%s/v1/videos/%s/remix", a.baseURL, info.OriginTaskID), nil
	}
	return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	if value, exists := c.Get(miniMaxH3RequestBodyKey); exists {
		requestBody, ok := value.(miniMaxH3Request)
		if !ok {
			return nil, errors.New("invalid MiniMax-H3 request body")
		}
		requestBody.Model = info.UpstreamModelName
		body, err := common.Marshal(requestBody)
		if err != nil {
			return nil, errors.Wrap(err, "marshal MiniMax-H3 request body failed")
		}
		return bytes.NewReader(body), nil
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}
	contentType := c.GetHeader("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		var bodyMap map[string]interface{}
		if err := common.Unmarshal(cachedBody, &bodyMap); err == nil {
			bodyMap["model"] = info.UpstreamModelName
			if newBody, err := common.Marshal(bodyMap); err == nil {
				return bytes.NewReader(newBody), nil
			}
		}
		return bytes.NewReader(cachedBody), nil
	}

	if strings.Contains(contentType, "multipart/form-data") {
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return bytes.NewReader(cachedBody), nil
		}
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("model", info.UpstreamModelName)
		for key, values := range formData.Value {
			if key == "model" {
				continue
			}
			for _, v := range values {
				writer.WriteField(key, v)
			}
		}
		for fieldName, fileHeaders := range formData.File {
			for _, fh := range fileHeaders {
				f, err := fh.Open()
				if err != nil {
					continue
				}
				ct := fh.Header.Get("Content-Type")
				if ct == "" || ct == "application/octet-stream" {
					buf512 := make([]byte, 512)
					n, _ := io.ReadFull(f, buf512)
					ct = http.DetectContentType(buf512[:n])
					// Re-open after sniffing so the full content is copied below
					f.Close()
					f, err = fh.Open()
					if err != nil {
						continue
					}
				}
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fh.Filename))
				h.Set("Content-Type", ct)
				part, err := writer.CreatePart(h)
				if err != nil {
					f.Close()
					continue
				}
				io.Copy(part, f)
				f.Close()
			}
		}
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &buf, nil
	}

	return common.ReaderOnly(storage), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Sora response
	var dResp responseTask
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamID := dResp.ID
	if upstreamID == "" {
		upstreamID = dResp.TaskID
	}
	if upstreamID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 使用公开 task_xxxx ID 返回给客户端
	dResp.ID = info.PublicTaskID
	dResp.TaskID = info.PublicTaskID
	c.JSON(http.StatusOK, dResp)
	return upstreamID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	a.proxy = proxy
	if publicBaseURL, ok := body["public_base_url"].(string); ok {
		a.publicBaseURL = strings.TrimSpace(publicBaseURL)
	}
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	if upscaleID, ok := body["upscale_task_id"].(string); ok && strings.TrimSpace(upscaleID) != "" {
		resp, err := service.FetchMiniMaxH3Upscale(context.Background(), upscaleID)
		if err != nil {
			return nil, err
		}
		payload := map[string]any{"status": strings.ToLower(resp.Status), "progress": resp.Progress, "url": resp.URL, "message": resp.Reason, "minimax_h3_upscale": true}
		data, err := common.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
	}

	uri := fmt.Sprintf("%s/v1/videos/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var upscale struct {
		Status   string `json:"status"`
		Progress int    `json:"progress"`
		URL      string `json:"url"`
		Message  string `json:"message"`
		Upscale  bool   `json:"minimax_h3_upscale"`
	}
	if common.Unmarshal(respBody, &upscale) == nil && upscale.Upscale {
		result := &relaycommon.TaskInfo{Code: 0}
		result.Stage = "minimax_h3_upscale"
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
			result.Reason = upscale.Message
			if strings.TrimSpace(result.Reason) == "" {
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
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	switch resTask.Status {
	case "queued", "pending":
		taskResult.Status = model.TaskStatusQueued
	case "processing", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
	case "completed":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Url = a.publicResultURL(resTask)
	case "failed", "cancelled":
		taskResult.Status = model.TaskStatusFailure
		if resTask.Error != nil {
			taskResult.Reason = resTask.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	default:
	}
	if resTask.Progress > 0 && resTask.Progress < 100 {
		taskResult.Progress = fmt.Sprintf("%d%%", resTask.Progress)
	}

	return &taskResult, nil
}

func (r responseTask) MetadataURL() string {
	if r.Metadata == nil {
		return ""
	}
	return r.Metadata.URL
}

func (r responseTask) ContentURL() string {
	if r.Content == nil {
		return ""
	}
	return r.Content.URL
}

// publicResultURL resolves the public media URL for a completed task.
// SGLang 上游以 file_path（容器内路径）上报结果，优先拼成
// {公开域名}/sglang/{截取路径} 的媒体直链；没有 file_path 时透传上游
// http(s) 地址，但跳过其他网关的 /v1/videos/{id}/content 代理路由 ——
// 该地址对最终客户端不可用（任务归属别的实例/用户）。
func (a *TaskAdaptor) publicResultURL(res responseTask) string {
	filePathCandidates := make([]string, 0, 2)
	if strings.TrimSpace(res.FilePath) != "" {
		filePathCandidates = append(filePathCandidates, res.FilePath)
	} else if len(res.FilePaths) > 0 && strings.TrimSpace(res.FilePaths[0]) != "" {
		filePathCandidates = append(filePathCandidates, res.FilePaths[0])
	}
	for _, candidate := range filePathCandidates {
		if mediaURL := service.BuildSGLangMediaURL(a.publicBaseURL, candidate); mediaURL != "" {
			return mediaURL
		}
	}
	for _, candidate := range []string{res.MetadataURL(), res.ContentURL(), res.URL} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || isVideoProxyRoute(candidate) {
			continue
		}
		return candidate
	}
	return ""
}

// isVideoProxyRoute reports whether the URL points at a gateway video-proxy
// route (…/v1/videos/{task_id}/content) instead of a directly downloadable
// media file.
func isVideoProxyRoute(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.HasPrefix(parsed.Path, "/v1/videos/") && strings.HasSuffix(parsed.Path, "/content")
}

// ProcessTaskResultBeforePersist chains a configured 1536P upscale task after
// MiniMax-H3 reports completion. The original task remains in progress until
// the second service reports completion.
func (a *TaskAdaptor) ProcessTaskResultBeforePersist(ctx context.Context, task *model.Task, result *relaycommon.TaskInfo) error {
	if task == nil || result == nil || !constant.IsMiniMaxH3Model(task.Properties.OriginModelName) || result.Status != model.TaskStatusSuccess {
		return nil
	}
	if result.Stage == "minimax_h3_upscale" {
		logger.LogInfo(ctx, fmt.Sprintf("MiniMax-H3 upscale completed, persisting result: task=%s", task.TaskID))
		service.PersistMiniMaxH3UpscaleResult(ctx, task, result)
		return nil
	}
	requestedShortEdge := task.MiniMaxH3RequestedShortEdge()
	logger.LogInfo(ctx, fmt.Sprintf("MiniMax-H3 source completed: task=%s requested_short_edge=%d upscale_task_id_present=%t",
		task.TaskID, requestedShortEdge, strings.TrimSpace(task.PrivateData.MiniMaxH3UpscaleTaskID) != ""))
	if requestedShortEdge != service.MiniMaxH3UpscaleShortEdge || strings.TrimSpace(task.PrivateData.MiniMaxH3UpscaleTaskID) != "" {
		if strings.TrimSpace(task.PrivateData.MiniMaxH3UpscaleTaskID) != "" {
			result.Status = model.TaskStatusInProgress
			result.Progress = "95%"
		}
		return nil
	}
	if strings.TrimSpace(constant.MiniMaxH3UpscaleURL) == "" {
		return fmt.Errorf("MINIMAX_H3_UPSCALE_URL is required for 1536P output")
	}
	sourceURL := fmt.Sprintf("%s/v1/videos/%s/content", strings.TrimRight(a.baseURL, "/"), task.PrivateData.UpstreamTaskID)
	sourceKey := task.PrivateData.Key
	if sourceKey == "" {
		sourceKey = a.apiKey
	}
	upscaleID, err := service.SubmitMiniMaxH3Upscale(ctx, sourceURL, sourceKey, a.proxy, task.TaskID)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("MiniMax-H3 upscale submission failed: task=%s error=%v", task.TaskID, err))
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
	logger.LogInfo(ctx, fmt.Sprintf("MiniMax-H3 upscale submitted: task=%s upscale_task=%s", task.TaskID, upscaleID))
	result.Status = model.TaskStatusInProgress
	result.Progress = "90%"
	return nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	if !constant.IsMiniMaxH3Model(task.Properties.OriginModelName) {
		data, err := sjson.SetBytes(task.Data, "id", task.TaskID)
		if err != nil {
			return nil, errors.Wrap(err, "set id failed")
		}
		return data, nil
	}

	response := make(map[string]any)
	if len(task.Data) > 0 {
		if err := common.Unmarshal(task.Data, &response); err != nil {
			return nil, errors.Wrap(err, "unmarshal Sora task data failed")
		}
	}

	// MiniMax-H3 与 AutoDL 渠道共用统一响应契约（service.NormalizeMiniMaxH3VideoResponse），
	// 客户端无法感知任务实际由哪个渠道承载。
	return common.Marshal(service.NormalizeMiniMaxH3VideoResponse(task, response))
}
