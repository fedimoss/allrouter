package doubao

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Request / Response structures
// ============================

type DraftTask struct {
	ID string `json:"id"`
}

type ContentItem struct {
	Type      string     `json:"type,omitempty"`
	Text      string     `json:"text,omitempty"`
	ImageURL  *MediaURL  `json:"image_url,omitempty"`
	VideoURL  *MediaURL  `json:"video_url,omitempty"`
	AudioURL  *MediaURL  `json:"audio_url,omitempty"`
	DraftTask *DraftTask `json:"draft_task,omitempty"`
	Role      string     `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type requestPayload struct {
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content,omitempty"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *dto.IntValue  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	OutputFormat          string         `json:"output_format,omitempty"`
	OmniReferenceTaskType string         `json:"omni_reference_task_type,omitempty"`
	Tools                 []struct {
		Type string `json:"type,omitempty"`
	} `json:"tools,omitempty"`
	SafetyIdentifier string         `json:"safety_identifier,omitempty"`
	Priority         *dto.IntValue  `json:"priority,omitempty"`
	Resolution       string         `json:"resolution,omitempty"`
	Ratio            string         `json:"ratio,omitempty"`
	Duration         *dto.IntValue  `json:"duration,omitempty"`
	Frames           *dto.IntValue  `json:"frames,omitempty"`
	Seed             *dto.IntValue  `json:"seed,omitempty"`
	CameraFixed      *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark        *dto.BoolValue `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseTask struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Seed            int    `json:"seed"`
	Resolution      string `json:"resolution"`
	Duration        int    `json:"duration"`
	Ratio           string `json:"ratio"`
	FramesPerSecond int    `json:"framespersecond"`
	ServiceTier     string `json:"service_tier"`
	OutputFormat    string `json:"output_format"`
	Tools           []struct {
		Type string `json:"type"`
	} `json:"tools"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		ToolUsage        struct {
			WebSearch int `json:"web_search"`
		} `json:"tool_usage"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

const seedanceResultPersistAttemptLimit = 3

const (
	seedanceReferenceImagesField = "reference_images"
	seedanceReferenceVideosField = "reference_videos"
	seedanceReferenceAudiosField = "reference_audios"
)

func applySeedanceDefaults(payload *requestPayload, capabilityModel string) {
	if payload == nil {
		return
	}
	if payload.Ratio == "" {
		payload.Ratio = "adaptive"
	}
	if payload.Resolution == "" {
		payload.Resolution = "720p"
	}
	if payload.Duration == nil && payload.Frames == nil {
		payload.Duration = lo.ToPtr(dto.IntValue(5))
	}
	if payload.GenerateAudio == nil {
		payload.GenerateAudio = lo.ToPtr(dto.BoolValue(true))
	}
	if payload.Watermark == nil {
		payload.Watermark = lo.ToPtr(dto.BoolValue(false))
	}

	family := service.SeedanceFamilyForModel(capabilityModel)
	switch family {
	case service.SeedanceFamilyV1:
		if payload.Seed == nil {
			payload.Seed = lo.ToPtr(dto.IntValue(-1))
		}
		if payload.CameraFixed == nil {
			payload.CameraFixed = lo.ToPtr(dto.BoolValue(false))
		}
		if payload.ServiceTier == "" {
			payload.ServiceTier = "default"
		}
		if payload.ReturnLastFrame == nil {
			payload.ReturnLastFrame = lo.ToPtr(dto.BoolValue(false))
		}
		if payload.ExecutionExpiresAfter == nil {
			payload.ExecutionExpiresAfter = lo.ToPtr(dto.IntValue(172800))
		}
	case service.SeedanceFamilyV2, service.SeedanceFamilyV25:
		if payload.ReturnLastFrame == nil {
			payload.ReturnLastFrame = lo.ToPtr(dto.BoolValue(false))
		}
		if payload.ExecutionExpiresAfter == nil {
			payload.ExecutionExpiresAfter = lo.ToPtr(dto.IntValue(172800))
		}
		if payload.Priority == nil {
			payload.Priority = lo.ToPtr(dto.IntValue(0))
		}
	}

	if family == service.SeedanceFamilyV25 {
		if payload.Draft == nil {
			payload.Draft = lo.ToPtr(dto.BoolValue(false))
		}
		if payload.OmniReferenceTaskType == "" && hasSeedanceReferenceContent(payload.Content) {
			payload.OmniReferenceTaskType = "auto"
		}
		if payload.OutputFormat == "" {
			payload.OutputFormat = "mp4"
		}
	}
}

func hasSeedanceReferenceContent(items []ContentItem) bool {
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Role)) {
		case "reference_image", "reference_video", "reference_audio":
			return true
		}
	}
	return false
}

func validateSeedanceModelParams(payload *requestPayload, capabilityModel string) error {
	if payload == nil {
		return nil
	}
	family := service.SeedanceFamilyForModel(capabilityModel)
	switch family {
	case service.SeedanceFamilyV1:
		if payload.Priority != nil {
			return errors.New("Seedance 1.x does not support parameter priority")
		}
		if len(payload.Tools) > 0 {
			return errors.New("Seedance 1.x does not support parameter tools")
		}
		if payload.Draft != nil {
			return errors.New("Seedance 1.x does not support parameter draft")
		}
	case service.SeedanceFamilyV2:
		if payload.Seed != nil {
			return errors.New("Seedance 2.0 does not support parameter seed")
		}
		if payload.CameraFixed != nil {
			return errors.New("Seedance 2.0 does not support parameter camera_fixed")
		}
		if payload.ServiceTier != "" {
			return errors.New("Seedance 2.0 does not support parameter service_tier")
		}
		if payload.Frames != nil {
			return errors.New("Seedance 2.0 does not support parameter frames")
		}
		if payload.Draft != nil {
			return errors.New("Seedance 2.0 does not support parameter draft")
		}
	}

	if family != service.SeedanceFamilyV25 {
		if payload.OmniReferenceTaskType != "" {
			return errors.New("Seedance 2.0 does not support parameter omni_reference_task_type")
		}
		if payload.OutputFormat != "" {
			return errors.New("Seedance 2.0 does not support parameter output_format")
		}
		if containsDraftTask(payload.Content) {
			return errors.New("Seedance 2.0 does not support content type draft_task")
		}
	}
	if family == service.SeedanceFamilyV25 {
		if payload.Seed != nil {
			return errors.New("Seedance 2.5 does not support parameter seed")
		}
		if payload.CameraFixed != nil {
			return errors.New("Seedance 2.5 does not support parameter camera_fixed")
		}
		if payload.ServiceTier != "" {
			return errors.New("Seedance 2.5 does not support parameter service_tier")
		}
		if payload.Frames != nil {
			return errors.New("Seedance 2.5 does not support parameter frames")
		}
	}
	return nil
}

func containsDraftTask(items []ContentItem) bool {
	for _, item := range items {
		if item.Type == "draft_task" || item.DraftTask != nil {
			return true
		}
	}
	return false
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction accepts JSON API requests and the Playground
// multipart format. Multipart reference files are stored locally first so the
// upstream only receives public URLs.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	if strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "multipart/form-data") {
		return a.validateSeedanceMultipartRequest(c, info)
	}
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}
	return a.validateSeedanceJSONRequest(c)
}

func (a *TaskAdaptor) validateSeedanceJSONRequest(c *gin.Context) *dto.TaskError {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	capabilityModel := req.Model
	if resolved, _, resolveErr := model.ResolveMappedModelName(c.GetString("model_mapping"), req.Model); resolveErr != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("resolve Seedance model mapping failed: %v", resolveErr))
	} else {
		capabilityModel = resolved
	}
	capabilities := service.SeedanceCapabilitiesForModel(capabilityModel)

	duration := req.Duration
	if duration == 0 {
		duration, _ = strconv.Atoi(req.Seconds)
	}
	if duration != 0 && duration != -1 && (duration < capabilities.MinDuration || duration > capabilities.MaxDuration) {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("duration must be -1 or between %d and %d seconds", capabilities.MinDuration, capabilities.MaxDuration),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	images, videos, audios := len(req.Images), 0, 0
	if contentRaw, ok := req.Metadata["content"]; ok {
		images, videos, audios = 0, 0, 0
		switch content := contentRaw.(type) {
		case []ContentItem:
			for _, item := range content {
				switch item.Type {
				case "image_url":
					images++
				case "video_url":
					videos++
				case "audio_url":
					audios++
				}
			}
		case []interface{}:
			for _, rawItem := range content {
				item, ok := rawItem.(map[string]interface{})
				if !ok {
					continue
				}
				switch item["type"] {
				case "image_url":
					images++
				case "video_url":
					videos++
				case "audio_url":
					audios++
				}
			}
		}
	}
	if images > capabilities.MaxImages {
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference images are allowed", capabilities.MaxImages), "invalid_request", http.StatusBadRequest)
	}
	if videos > capabilities.MaxVideos {
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference videos are allowed", capabilities.MaxVideos), "invalid_request", http.StatusBadRequest)
	}
	if audios > capabilities.MaxAudios {
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference audios are allowed", capabilities.MaxAudios), "invalid_request", http.StatusBadRequest)
	}
	if !capabilities.AllowAudioOnly && audios > 0 && images == 0 && videos == 0 {
		return service.TaskErrorWrapperLocal(fmt.Errorf("audio-only requests are not supported; add an image or video"), "invalid_request", http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) validateSeedanceMultipartRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_multipart_form", http.StatusBadRequest)
	}
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_multipart_form", http.StatusBadRequest)
	}
	defer form.RemoveAll()

	formValue := func(name string) string {
		if values := form.Value[name]; len(values) > 0 {
			return values[0]
		}
		return ""
	}

	req.Model = strings.TrimSpace(formValue("model"))
	req.Prompt = formValue("prompt")
	if req.Model == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field model is required"), "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	capabilityModel := req.Model
	if resolved, _, resolveErr := model.ResolveMappedModelName(c.GetString("model_mapping"), req.Model); resolveErr != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("resolve Seedance model mapping failed: %v", resolveErr))
	} else {
		capabilityModel = resolved
	}
	capabilities := service.SeedanceCapabilitiesForModel(capabilityModel)

	allowedFields := map[string]bool{
		"model": true, "prompt": true, "group": true, "task": true,
		"ratio": true, "resolution": true, "duration": true, "seconds": true,
		"generate_audio": true, "watermark": true, "output_format": true,
	}
	unsupported := make([]string, 0)
	for field := range form.Value {
		if !allowedFields[field] {
			unsupported = append(unsupported, field)
		}
	}
	allowedFiles := map[string]bool{
		seedanceReferenceImagesField: true,
		seedanceReferenceVideosField: true,
		seedanceReferenceAudiosField: true,
	}
	for field := range form.File {
		if !allowedFiles[field] {
			unsupported = append(unsupported, field)
		}
	}
	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("Seedance request contains unsupported fields: %s", strings.Join(unsupported, ", ")),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	imageFiles := form.File[seedanceReferenceImagesField]
	videoFiles := form.File[seedanceReferenceVideosField]
	audioFiles := form.File[seedanceReferenceAudiosField]
	if len(imageFiles) > capabilities.MaxImages {
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference images are allowed", capabilities.MaxImages), "invalid_request", http.StatusBadRequest)
	}
	if len(videoFiles) > capabilities.MaxVideos {
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference videos are allowed", capabilities.MaxVideos), "invalid_request", http.StatusBadRequest)
	}
	if len(audioFiles) > capabilities.MaxAudios {
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference audios are allowed", capabilities.MaxAudios), "invalid_request", http.StatusBadRequest)
	}
	if !capabilities.AllowAudioOnly && len(audioFiles) > 0 && len(imageFiles) == 0 && len(videoFiles) == 0 {
		return service.TaskErrorWrapperLocal(fmt.Errorf("audio-only requests are not supported; add an image or video"), "invalid_request", http.StatusBadRequest)
	}

	var totalBytes int64
	for _, files := range form.File {
		for _, fileHeader := range files {
			totalBytes += fileHeader.Size
		}
	}
	if totalBytes > service.SeedanceMediaMaxTotalBytes {
		return service.TaskErrorWrapperLocal(fmt.Errorf("reference media exceeds %d MB in total", service.SeedanceMediaMaxTotalBytes>>20), "invalid_request", http.StatusBadRequest)
	}

	publicBaseURL := common.GetRequestBaseURL(c, system_setting.ServerAddress)
	if publicBaseURL == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("public base URL is not configured"), "invalid_request", http.StatusBadRequest)
	}

	content := make([]ContentItem, 0, len(imageFiles)+len(videoFiles)+len(audioFiles))
	mediaIDs := make([]string, 0, cap(content))
	videoMediaIDs := make([]string, 0, len(videoFiles))
	audioMediaIDs := make([]string, 0, len(audioFiles))
	cleanup := true
	defer func() {
		if cleanup {
			service.CleanupSeedanceMedia(info.UserId, mediaIDs)
		}
	}()

	appendFiles := func(field, kind, role string) *dto.TaskError {
		for _, fileHeader := range form.File[field] {
			mediaID, saveErr := service.SaveSeedanceMediaFile(info.UserId, fileHeader, kind)
			if saveErr != nil {
				return service.TaskErrorWrapperLocal(fmt.Errorf("invalid %s: %w", field, saveErr), "invalid_request", http.StatusBadRequest)
			}
			mediaIDs = append(mediaIDs, mediaID)
			switch kind {
			case "video":
				videoMediaIDs = append(videoMediaIDs, mediaID)
			case "audio":
				audioMediaIDs = append(audioMediaIDs, mediaID)
			}
			publicURL, urlErr := service.SeedanceMediaPublicURL(publicBaseURL, info.UserId, mediaID)
			if urlErr != nil {
				return service.TaskErrorWrapperLocal(urlErr, "invalid_request", http.StatusBadRequest)
			}
			content = append(content, seedanceContentItem(kind, publicURL, role))
		}
		return nil
	}

	if taskErr := appendFiles(seedanceReferenceImagesField, "image", "reference_image"); taskErr != nil {
		return taskErr
	}
	if taskErr := appendFiles(seedanceReferenceVideosField, "video", "reference_video"); taskErr != nil {
		return taskErr
	}
	if taskErr := appendFiles(seedanceReferenceAudiosField, "audio", "reference_audio"); taskErr != nil {
		return taskErr
	}
	if service.SeedanceDurationProbeAvailable() {
		var videoSeconds, audioSeconds float64
		videoChecked, audioChecked := false, false
		for _, mediaID := range videoMediaIDs {
			duration, probeErr := service.ProbeSeedanceMediaDuration(c.Request.Context(), info.UserId, mediaID)
			if probeErr != nil {
				logger.LogWarn(c.Request.Context(), fmt.Sprintf("probe Seedance video duration failed: %v", probeErr))
				continue
			}
			videoSeconds += duration
			videoChecked = true
		}
		for _, mediaID := range audioMediaIDs {
			duration, probeErr := service.ProbeSeedanceMediaDuration(c.Request.Context(), info.UserId, mediaID)
			if probeErr != nil {
				logger.LogWarn(c.Request.Context(), fmt.Sprintf("probe Seedance audio duration failed: %v", probeErr))
				continue
			}
			audioSeconds += duration
			audioChecked = true
		}
		if videoChecked && videoSeconds > capabilities.MaxVideoSeconds {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("total reference video duration %.2fs exceeds %.0fs", videoSeconds, capabilities.MaxVideoSeconds),
				"invalid_request",
				http.StatusBadRequest,
			)
		}
		if audioChecked && audioSeconds > capabilities.MaxAudioSeconds {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("total reference audio duration %.2fs exceeds %.0fs", audioSeconds, capabilities.MaxAudioSeconds),
				"invalid_request",
				http.StatusBadRequest,
			)
		}
	}

	metadata := map[string]interface{}{"content": content}
	setStringField := func(name string) {
		if value := strings.TrimSpace(formValue(name)); value != "" {
			metadata[name] = value
		}
	}
	setStringField("ratio")
	setStringField("resolution")
	setStringField("output_format")

	durationRaw := strings.TrimSpace(formValue("duration"))
	if durationRaw == "" {
		durationRaw = strings.TrimSpace(formValue("seconds"))
	}
	if durationRaw != "" {
		duration, parseErr := strconv.Atoi(durationRaw)
		if parseErr != nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("duration must be an integer"), "invalid_request", http.StatusBadRequest)
		}
		req.Duration = duration
		metadata["duration"] = duration
		if duration != -1 && (duration < capabilities.MinDuration || duration > capabilities.MaxDuration) {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("duration must be -1 or between %d and %d seconds", capabilities.MinDuration, capabilities.MaxDuration),
				"invalid_request",
				http.StatusBadRequest,
			)
		}
	}
	if raw := strings.TrimSpace(formValue("generate_audio")); raw != "" {
		value, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("generate_audio must be a boolean"), "invalid_request", http.StatusBadRequest)
		}
		metadata["generate_audio"] = value
	}
	if raw := strings.TrimSpace(formValue("watermark")); raw != "" {
		value, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return service.TaskErrorWrapperLocal(fmt.Errorf("watermark must be a boolean"), "invalid_request", http.StatusBadRequest)
		}
		metadata["watermark"] = value
	}

	req.Task = strings.TrimSpace(formValue("task"))
	req.Metadata = metadata
	c.Set("task_request", req)
	info.Action = constant.TaskActionGenerate
	if len(mediaIDs) > 0 {
		existing := c.GetStringSlice(service.SeedanceMediaIDsContextKey)
		c.Set(service.SeedanceMediaIDsContextKey, append(existing, mediaIDs...))
	}
	cleanup = false
	return nil
}

func seedanceContentItem(kind, publicURL, role string) ContentItem {
	switch kind {
	case "image":
		return ContentItem{Type: "image_url", ImageURL: &MediaURL{URL: publicURL}, Role: role}
	case "video":
		return ContentItem{Type: "video_url", VideoURL: &MediaURL{URL: publicURL}, Role: role}
	case "audio":
		return ContentItem{Type: "audio_url", AudioURL: &MediaURL{URL: publicURL}, Role: role}
	default:
		return ContentItem{}
	}
}

// BuildRequestURL constructs the upstream URL. The channel BaseURL is a
// domain-only address for both the Ark-compatible Cloudwise endpoint and the
// official Ark endpoint.
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(a.baseURL), "/")
	if baseURL == "" {
		return "", errors.New("doubao video base URL is empty")
	}
	return baseURL + "/api/v3/contents/generations/tasks", nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// EstimateBilling returns the billing ratio for Seedance output resolution and video input.
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	hasVideo := hasVideoInMetadata(req.Metadata)
	resolution, _ := req.Metadata["resolution"].(string)
	ratio, ok := GetVideoInputRatio(info.OriginModelName, resolution, hasVideo)
	if !ok || ratio == 1.0 {
		return nil
	}
	return map[string]float64{"video_input": ratio}
}

// hasVideoInMetadata checks whether metadata content contains a video_url item.
func hasVideoInMetadata(metadata map[string]interface{}) bool {
	if metadata == nil {
		return false
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return false
	}
	switch content := contentRaw.(type) {
	case []ContentItem:
		for _, item := range content {
			if item.Type == "video_url" || item.VideoURL != nil {
				return true
			}
		}
	case []interface{}:
		for _, rawItem := range content {
			itemMap, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}
			if itemMap["type"] == "video_url" {
				return true
			}
			if _, has := itemMap["video_url"]; has {
				return true
			}
		}
	}
	return false
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	capabilityModel := info.UpstreamModelName
	if capabilityModel == "" {
		capabilityModel = req.Model
	}
	body, err := a.convertToRequestPayload(&req, capabilityModel)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
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

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return dResp.ID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri, err := buildFetchTaskURL(baseUrl, taskID)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func buildFetchTaskURL(baseURL, taskID string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", errors.New("doubao video base URL is empty")
	}
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseURL, url.PathEscape(taskID)), nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, capabilityModel string) (*requestPayload, error) {
	r := requestPayload{
		Model:   req.Model,
		Content: []ContentItem{},
	}

	// Add images if present
	if req.HasImage() {
		for _, imgURL := range req.Images {
			r.Content = append(r.Content, ContentItem{
				Type: "image_url",
				ImageURL: &MediaURL{
					URL: imgURL,
				},
			})
		}
	}

	if req.Duration != 0 {
		r.Duration = lo.ToPtr(dto.IntValue(req.Duration))
	} else if seconds, _ := strconv.Atoi(req.Seconds); seconds != 0 {
		r.Duration = lo.ToPtr(dto.IntValue(seconds))
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	applySeedanceDefaults(&r, capabilityModel)
	if err := validateSeedanceModelParams(&r, capabilityModel); err != nil {
		return nil, err
	}

	r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
	r.Content = append([]ContentItem{{
		Type: "text",
		Text: req.Prompt,
	}}, r.Content...)

	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	status := strings.ToLower(strings.TrimSpace(resTask.Status))
	switch status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = strings.TrimSpace(resTask.Content.VideoURL)
		// Record usage for ratio-based billing.
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	case "failed", "expired", "cancelled", "canceled":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.Error.Message
		if taskResult.Reason == "" {
			taskResult.Reason = fmt.Sprintf("upstream task %s", status)
		}
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

// ProcessTaskResultBeforePersist downloads the temporary upstream video into
// the shared result directory before the polling loop records the public URL.
func (a *TaskAdaptor) ProcessTaskResultBeforePersist(ctx context.Context, task *model.Task, result *relaycommon.TaskInfo) error {
	if task == nil || result == nil || result.Status != model.TaskStatusSuccess {
		return nil
	}
	resultURL := strings.TrimSpace(result.Url)
	if resultURL == "" || strings.HasPrefix(resultURL, "data:") {
		return nil
	}
	if strings.TrimSpace(task.PrivateData.PublicBaseURL) == "" {
		logger.LogWarn(ctx, fmt.Sprintf("Seedance result persistence skipped for task %s: public base URL is empty", task.TaskID))
		return nil
	}

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(
		resultURL,
		fetchSetting.EnableSSRFProtection,
		fetchSetting.AllowPrivateIp,
		fetchSetting.DomainFilterMode,
		fetchSetting.IpFilterMode,
		fetchSetting.DomainList,
		fetchSetting.IpList,
		fetchSetting.AllowedPorts,
		fetchSetting.ApplyIPFilterForDomain,
	); err != nil {
		return handleSeedancePersistFailure(ctx, task, result, err)
	}

	filename := "seedance-result.mp4"
	var upstreamTask responseTask
	if err := common.Unmarshal(task.Data, &upstreamTask); err == nil &&
		strings.EqualFold(strings.TrimSpace(upstreamTask.OutputFormat), "mov") {
		filename = "seedance-result.mov"
	}

	persistedURL, err := service.PersistVideoResult(ctx, resultURL, task.PrivateData.PublicBaseURL, task.TaskID, filename)
	if err != nil {
		return handleSeedancePersistFailure(ctx, task, result, err)
	}
	task.PrivateData.SeedanceResultPersistFailures = 0
	result.Url = persistedURL
	logger.LogInfo(ctx, fmt.Sprintf("Seedance result saved locally: task=%s url=%s", task.TaskID, persistedURL))
	return nil
}

func handleSeedancePersistFailure(ctx context.Context, task *model.Task, result *relaycommon.TaskInfo, persistErr error) error {
	task.PrivateData.SeedanceResultPersistFailures++
	failures := task.PrivateData.SeedanceResultPersistFailures
	result.Url = ""
	if failures >= seedanceResultPersistAttemptLimit {
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = fmt.Sprintf("save video result failed after %d attempts: %v", seedanceResultPersistAttemptLimit, service.SanitizeURLError(persistErr))
		logger.LogError(ctx, fmt.Sprintf("Seedance result persistence permanently failed: task=%s attempts=%d error=%v", task.TaskID, failures, persistErr))
		return nil
	}
	result.Status = model.TaskStatusInProgress
	result.Progress = "95%"
	logger.LogWarn(ctx, fmt.Sprintf("Seedance result persistence failed, will retry: task=%s attempt=%d/%d error=%v", task.TaskID, failures, seedanceResultPersistAttemptLimit, persistErr))
	return nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	videoURL := strings.TrimSpace(dResp.Content.VideoURL)
	if persistedURL := strings.TrimSpace(originTask.PrivateData.ResultURL); persistedURL != "" {
		videoURL = persistedURL
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", videoURL)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName
	if prompt := strings.TrimSpace(originTask.Properties.Input); prompt != "" {
		openAIVideo.SetMetadata("prompt", prompt)
	}
	if dResp.Duration > 0 {
		openAIVideo.Seconds = strconv.Itoa(dResp.Duration)
	}

	if strings.EqualFold(strings.TrimSpace(dResp.Status), "failed") {
		message := strings.TrimSpace(dResp.Error.Message)
		if message == "" {
			message = originTask.FailReason
		}
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: message,
			Code:    dResp.Error.Code,
		}
	}

	return common.Marshal(openAIVideo)
}
