package sora

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
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
	return relaycommon.ValidateMultipartDirect(c, info)
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
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
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
		// Url intentionally left empty — the caller constructs the proxy URL using the public task ID
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

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	data := task.Data
	var err error
	if data, err = sjson.SetBytes(data, "id", task.TaskID); err != nil {
		return nil, errors.Wrap(err, "set id failed")
	}
	return data, nil
}
