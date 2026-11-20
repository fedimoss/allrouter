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
	// 2.5 样片二阶段的草稿任务 ID，仅 draft_task 内容项使用。
	ID string `json:"id"`
}

// Ark content 同时承载文本、图片、视频、音频和草稿任务。
// 字段顺序和 role 会影响上游对素材的理解，不能随意调整。
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

// 提交给 Ark 的请求体；新增字段均由能力矩阵决定是否发送。
// 这里保留指针类型，确保显式 0/false 不会被 omitempty 静默丢弃。
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

// Ark 查询响应；OutputFormat 用于决定结果落盘扩展名。
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

// 注意：结果落盘失败重试上限，达到后任务转为失败，避免无限轮询。
const seedanceResultPersistAttemptLimit = 3

// Playground multipart 中三类参考素材的字段名，前后端必须一致。
const (
	seedanceReferenceImagesField = "reference_images"
	seedanceReferenceVideosField = "reference_videos"
	seedanceReferenceAudiosField = "reference_audios"
)

func applySeedanceDefaults(payload *requestPayload, capabilityModel string) {
	if payload == nil {
		return
	}
	// 先补齐所有 Seedance 版本共用的基础默认值。
	// 这些值必须在能力校验前写入，否则上游会收到缺字段请求。
	if payload.Ratio == "" {
		// 未指定比例时交给模型自适应，避免默认 16:9 改变用户构图。
		payload.Ratio = "adaptive"
	}
	if payload.Resolution == "" {
		// 720p 是兼容性最好的默认档，避免默认 1080p 增加费用。
		payload.Resolution = "720p"
	}
	if payload.Duration == nil && payload.Frames == nil {
		// duration 和 frames 二选一；都没传时用 5 秒兜底。
		payload.Duration = lo.ToPtr(dto.IntValue(5))
	}
	if payload.GenerateAudio == nil {
		// 默认生成音频，与 Playground 默认开关保持一致。
		payload.GenerateAudio = lo.ToPtr(dto.BoolValue(true))
	}
	if payload.Watermark == nil {
		// 默认无水印，避免用户未选择时产出带水印素材。
		payload.Watermark = lo.ToPtr(dto.BoolValue(false))
	}

	family := service.SeedanceFamilyForModel(capabilityModel)
	// 按模型族只补该版本支持的字段，避免把 1.x/2.5 参数发给错误版本。
	switch family {
	case service.SeedanceFamilyV1:
		// 1.x 专属默认值：seed、camera_fixed、service_tier 等。
		// 这些字段不能下发给 2.x，否则上游会报参数不支持。
		if payload.Seed == nil {
			// -1 表示随机种子，保证同一提示词也能产生不同结果。
			payload.Seed = lo.ToPtr(dto.IntValue(-1))
		}
		if payload.CameraFixed == nil {
			// 默认不固定镜头，保持画面运动自然。
			payload.CameraFixed = lo.ToPtr(dto.BoolValue(false))
		}
		if payload.ServiceTier == "" {
			// default 是 1.x 的普通服务档位，避免误用高优队列。
			payload.ServiceTier = "default"
		}
		if payload.ReturnLastFrame == nil {
			// 默认不返回尾帧，减少额外产物和存储占用。
			payload.ReturnLastFrame = lo.ToPtr(dto.BoolValue(false))
		}
		if payload.ExecutionExpiresAfter == nil {
			// 48 小时过期，覆盖大多数异步排队场景。
			payload.ExecutionExpiresAfter = lo.ToPtr(dto.IntValue(172800))
		}
	case service.SeedanceFamilyV2, service.SeedanceFamilyV25:
		// 2.x 共用默认值：不发送 1.x 专属字段。
		if payload.ReturnLastFrame == nil {
			// 2.x 同样默认不返回尾帧。
			payload.ReturnLastFrame = lo.ToPtr(dto.BoolValue(false))
		}
		if payload.ExecutionExpiresAfter == nil {
			// 2.x 异步任务同样使用 48 小时过期时间。
			payload.ExecutionExpiresAfter = lo.ToPtr(dto.IntValue(172800))
		}
		if payload.Priority == nil {
			// 默认普通优先级，避免无谓占用高优队列。
			payload.Priority = lo.ToPtr(dto.IntValue(0))
		}
	}

	if family == service.SeedanceFamilyV25 {
		// 2.5 专属字段；omni 只在多模态参考场景出现，否则上游会拒绝。
		// 纯文生视频或首帧/首尾帧请求必须保持该字段为空。
		if payload.Draft == nil {
			// 默认不是草稿任务，走正式出片流程。
			payload.Draft = lo.ToPtr(dto.BoolValue(false))
		}
		if payload.OmniReferenceTaskType == "" && hasSeedanceReferenceContent(payload.Content) {
			// 只有 reference_* 素材才标记为多模态参考任务。
			payload.OmniReferenceTaskType = "auto"
		}
		if payload.OutputFormat == "" {
			// 默认 mp4，兼容绝大多数播放器和前端预览。
			payload.OutputFormat = "mp4"
		}
	}
}

func hasSeedanceReferenceContent(items []ContentItem) bool {
	// 只有 reference_* 角色才算多模态参考，首帧/尾帧不算。
	// 该判断直接决定是否发送 omni_reference_task_type，不能放宽。
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
	// 显式传入当前版本不支持的参数时直接返回 400，避免静默丢弃。
	// 这样客户端能及时发现参数错误，而不是拿到一个“成功但少参数”的任务。
	switch family {
	case service.SeedanceFamilyV1:
		// 1.x 不支持 2.x 的 priority/tools/draft；显式传入说明调用方理解错了模型。
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
		// 2.0 不支持 1.x 的 seed/camera_fixed/service_tier，也不支持 frames/draft。
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
		// 2.0 及更早版本不允许多模态参考专属字段。
		// 这些字段一旦发给旧版本，上游会直接 400，因此本地提前拦截。
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
		// 2.5 也不支持 1.x 的 seed/camera_fixed/service_tier/frames。
		// 不能因为版本更高就默认兼容旧参数，上游仍然会拒绝。
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
	// 识别 draft_task 内容项，供旧版本参数校验使用。
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
	// Playground 走 multipart，外部 API 走 JSON，两条路径分别校验。
	// 不能只保留一条路径，否则会丢失文件落盘或 JSON 参数校验能力。
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
	// 用渠道映射后的上游名判断能力，公开模型名可能被重定向。
	// 映射解析失败时回退公开名，避免因配置错误直接拒绝请求。
	if resolved, _, resolveErr := model.ResolveMappedModelName(c.GetString("model_mapping"), req.Model); resolveErr != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("resolve Seedance model mapping failed: %v", resolveErr))
	} else {
		capabilityModel = resolved
	}
	capabilities := service.SeedanceCapabilitiesForModel(capabilityModel)

	duration := req.Duration
	if duration == 0 {
		// 兼容旧客户端：没有 duration 时尝试读取 seconds 字段。
		duration, _ = strconv.Atoi(req.Seconds)
	}
	// -1 表示自动时长，不能套用 min/max 范围。
	if duration != 0 && duration != -1 && (duration < capabilities.MinDuration || duration > capabilities.MaxDuration) {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("duration must be -1 or between %d and %d seconds", capabilities.MinDuration, capabilities.MaxDuration),
			"invalid_request",
			http.StatusBadRequest,
		)
	}

	images, videos, audios := len(req.Images), 0, 0
	if contentRaw, ok := req.Metadata["content"]; ok {
		// metadata 可能是强类型切片或 JSON 反序列化后的 []interface{}。
		// 两种形态都要统计，否则数量限制会被绕过。
		images, videos, audios = 0, 0, 0
		switch content := contentRaw.(type) {
		case []ContentItem:
			for _, item := range content {
				// 强类型切片直接按 Type 统计素材数量。
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
					// 非对象项无法识别类型，跳过而不是误计数。
					continue
				}
				// JSON 反序列化后的 map 需要按字符串 key 读取 type。
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
	// 数量限制由后端强制校验，不能只依赖前端面板。
	if images > capabilities.MaxImages {
		// 图片数量超限直接拒绝，避免上游返回难以理解的错误。
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference images are allowed", capabilities.MaxImages), "invalid_request", http.StatusBadRequest)
	}
	if videos > capabilities.MaxVideos {
		// 视频数量单独限制，不能和图片共用上限。
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference videos are allowed", capabilities.MaxVideos), "invalid_request", http.StatusBadRequest)
	}
	if audios > capabilities.MaxAudios {
		// 音频数量单独限制，2.0/2.5 上限不同。
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference audios are allowed", capabilities.MaxAudios), "invalid_request", http.StatusBadRequest)
	}
	if !capabilities.AllowAudioOnly && audios > 0 && images == 0 && videos == 0 {
		// 2.0 不支持仅音频；必须至少有一张图片或一个视频作为画面参考。
		return service.TaskErrorWrapperLocal(fmt.Errorf("audio-only requests are not supported; add an image or video"), "invalid_request", http.StatusBadRequest)
	}
	return nil
}

func (a *TaskAdaptor) validateSeedanceMultipartRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	// 解析 multipart 表单和文件，复用 Gin 的缓存表单避免重复读取 body。
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
		// Gin 的 multipart 表单一个字段可能有多个值，这里只取第一个。
		if values := form.Value[name]; len(values) > 0 {
			return values[0]
		}
		return ""
	}

	req.Model = strings.TrimSpace(formValue("model"))
	req.Prompt = formValue("prompt")
	if req.Model == "" {
		// 模型是路由和计费的前提，缺失时不能继续。
		return service.TaskErrorWrapperLocal(fmt.Errorf("field model is required"), "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		// 空提示词无法生成有效视频，直接拒绝。
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	capabilityModel := req.Model
	// multipart 路径同样要先做模型映射，否则能力判断会落在公开名上。
	if resolved, _, resolveErr := model.ResolveMappedModelName(c.GetString("model_mapping"), req.Model); resolveErr != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("resolve Seedance model mapping failed: %v", resolveErr))
	} else {
		capabilityModel = resolved
	}
	capabilities := service.SeedanceCapabilitiesForModel(capabilityModel)

	// 只允许已知字段，未知字段直接报错，防止参数被静默忽略。
	// 这里刻意严格，避免客户端误传字段却以为已经生效。
	allowedFields := map[string]bool{
		"model": true, "prompt": true, "group": true, "task": true,
		"ratio": true, "resolution": true, "duration": true, "seconds": true,
		"generate_audio": true, "watermark": true, "output_format": true,
	}
	unsupported := make([]string, 0)
	// 校验普通表单字段，防止客户端传了后端不认识的参数。
	for field := range form.Value {
		if !allowedFields[field] {
			unsupported = append(unsupported, field)
		}
	}
	allowedFiles := map[string]bool{
		// 只接受三类参考素材字段，其他文件字段一律拒绝。
		seedanceReferenceImagesField: true,
		seedanceReferenceVideosField: true,
		seedanceReferenceAudiosField: true,
	}
	for field := range form.File {
		// 文件字段也必须走白名单，避免任意文件写入上传目录。
		if !allowedFiles[field] {
			unsupported = append(unsupported, field)
		}
	}
	if len(unsupported) > 0 {
		// 排序后返回，保证错误信息稳定、方便排查。
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
	// 文件数量按参考素材类型分别限制。
	if len(imageFiles) > capabilities.MaxImages {
		// 图片文件数量超限，直接拒绝，不再落盘。
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference images are allowed", capabilities.MaxImages), "invalid_request", http.StatusBadRequest)
	}
	if len(videoFiles) > capabilities.MaxVideos {
		// 视频文件数量超限，避免占用大量磁盘和上传带宽。
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference videos are allowed", capabilities.MaxVideos), "invalid_request", http.StatusBadRequest)
	}
	if len(audioFiles) > capabilities.MaxAudios {
		// 音频文件数量超限，2.0/2.5 上限不同。
		return service.TaskErrorWrapperLocal(fmt.Errorf("at most %d reference audios are allowed", capabilities.MaxAudios), "invalid_request", http.StatusBadRequest)
	}
	if !capabilities.AllowAudioOnly && len(audioFiles) > 0 && len(imageFiles) == 0 && len(videoFiles) == 0 {
		// 2.0 仅音频请求没有画面参考，必须在落盘前拒绝。
		return service.TaskErrorWrapperLocal(fmt.Errorf("audio-only requests are not supported; add an image or video"), "invalid_request", http.StatusBadRequest)
	}

	var totalBytes int64
	// 所有参考文件合计大小也受限，防止单次请求占用过多磁盘。
	for _, files := range form.File {
		for _, fileHeader := range files {
			totalBytes += fileHeader.Size
		}
	}
	if totalBytes > service.SeedanceMediaMaxTotalBytes {
		// 总大小超限时拒绝，避免一次请求写满磁盘。
		return service.TaskErrorWrapperLocal(fmt.Errorf("reference media exceeds %d MB in total", service.SeedanceMediaMaxTotalBytes>>20), "invalid_request", http.StatusBadRequest)
	}

	publicBaseURL := common.GetRequestBaseURL(c, system_setting.ServerAddress)
	if publicBaseURL == "" {
		// 上游只能访问公网 URL；没有公网基地址时无法完成提交。
		return service.TaskErrorWrapperLocal(fmt.Errorf("public base URL is not configured"), "invalid_request", http.StatusBadRequest)
	}

	content := make([]ContentItem, 0, len(imageFiles)+len(videoFiles)+len(audioFiles))
	mediaIDs := make([]string, 0, cap(content))
	videoMediaIDs := make([]string, 0, len(videoFiles))
	audioMediaIDs := make([]string, 0, len(audioFiles))
	cleanup := true
	// 提交失败或提前返回时必须清理已落盘素材，避免孤儿文件。
	// cleanup 只在请求成功入库后置为 false。
	defer func() {
		if cleanup {
			service.CleanupSeedanceMedia(info.UserId, mediaIDs)
		}
	}()

	appendFiles := func(field, kind, role string) *dto.TaskError {
		// 逐个文件落盘、生成公网 URL，并记录 mediaID 供终态清理。
		// 任何一步失败都会由外层 defer 清理此前已写入的文件。
		for _, fileHeader := range form.File[field] {
			// 落盘失败立即返回；外层 defer 会清理此前已成功的文件。
			mediaID, saveErr := service.SaveSeedanceMediaFile(info.UserId, fileHeader, kind)
			if saveErr != nil {
				return service.TaskErrorWrapperLocal(fmt.Errorf("invalid %s: %w", field, saveErr), "invalid_request", http.StatusBadRequest)
			}
			mediaIDs = append(mediaIDs, mediaID)
			// 单独记录视频/音频 ID，后续 ffprobe 只需要探测这两类。
			switch kind {
			case "video":
				videoMediaIDs = append(videoMediaIDs, mediaID)
			case "audio":
				audioMediaIDs = append(audioMediaIDs, mediaID)
			}
			publicURL, urlErr := service.SeedanceMediaPublicURL(publicBaseURL, info.UserId, mediaID)
			if urlErr != nil {
				// URL 生成失败同样视为提交失败，避免把不可访问地址发给上游。
				return service.TaskErrorWrapperLocal(urlErr, "invalid_request", http.StatusBadRequest)
			}
			// content 顺序与表单字段顺序一致：图片、视频、音频。
			content = append(content, seedanceContentItem(kind, publicURL, role))
		}
		return nil
	}

	if taskErr := appendFiles(seedanceReferenceImagesField, "image", "reference_image"); taskErr != nil {
		// 图片先处理，保持 [Image N] 编号稳定。
		return taskErr
	}
	if taskErr := appendFiles(seedanceReferenceVideosField, "video", "reference_video"); taskErr != nil {
		// 视频其次，保持 [Video N] 编号稳定。
		return taskErr
	}
	if taskErr := appendFiles(seedanceReferenceAudiosField, "audio", "reference_audio"); taskErr != nil {
		// 音频最后，保持 [Audio N] 编号稳定。
		return taskErr
	}
	if service.SeedanceDurationProbeAvailable() {
		// ffprobe 可用时累加视频/音频时长；单文件失败只告警并放行。
		// 时长校验是 best-effort，不能因为本地探测失败阻断用户请求。
		var videoSeconds, audioSeconds float64
		videoChecked, audioChecked := false, false
		for _, mediaID := range videoMediaIDs {
			// 单个视频探测失败只记录警告，继续累加其他已知时长。
			duration, probeErr := service.ProbeSeedanceMediaDuration(c.Request.Context(), info.UserId, mediaID)
			if probeErr != nil {
				logger.LogWarn(c.Request.Context(), fmt.Sprintf("probe Seedance video duration failed: %v", probeErr))
				continue
			}
			videoSeconds += duration
			videoChecked = true
		}
		for _, mediaID := range audioMediaIDs {
			// 单个音频探测失败同样放行，最终由上游做权威校验。
			duration, probeErr := service.ProbeSeedanceMediaDuration(c.Request.Context(), info.UserId, mediaID)
			if probeErr != nil {
				logger.LogWarn(c.Request.Context(), fmt.Sprintf("probe Seedance audio duration failed: %v", probeErr))
				continue
			}
			audioSeconds += duration
			audioChecked = true
		}
		if videoChecked && videoSeconds > capabilities.MaxVideoSeconds {
			// 只有至少一个视频探测成功时才做总时长判断。
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("total reference video duration %.2fs exceeds %.0fs", videoSeconds, capabilities.MaxVideoSeconds),
				"invalid_request",
				http.StatusBadRequest,
			)
		}
		if audioChecked && audioSeconds > capabilities.MaxAudioSeconds {
			// 只有至少一个音频探测成功时才做总时长判断。
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("total reference audio duration %.2fs exceeds %.0fs", audioSeconds, capabilities.MaxAudioSeconds),
				"invalid_request",
				http.StatusBadRequest,
			)
		}
	}

	metadata := map[string]interface{}{"content": content}
	// 把表单里的可选参数写回 metadata，后续统一转成上游 payload。
	setStringField := func(name string) {
		// 空字符串不写入 metadata，让后端默认值继续生效。
		if value := strings.TrimSpace(formValue(name)); value != "" {
			metadata[name] = value
		}
	}
	setStringField("ratio")
	setStringField("resolution")
	setStringField("output_format")

	durationRaw := strings.TrimSpace(formValue("duration"))
	if durationRaw == "" {
		// 兼容旧字段 seconds；两者都存在时 duration 优先。
		durationRaw = strings.TrimSpace(formValue("seconds"))
	}
	if durationRaw != "" {
		duration, parseErr := strconv.Atoi(durationRaw)
		if parseErr != nil {
			// 时长必须是整数秒，解析失败直接返回 400。
			return service.TaskErrorWrapperLocal(fmt.Errorf("duration must be an integer"), "invalid_request", http.StatusBadRequest)
		}
		req.Duration = duration
		metadata["duration"] = duration
		if duration != -1 && (duration < capabilities.MinDuration || duration > capabilities.MaxDuration) {
			// -1 表示自动时长，不参与 min/max 判断。
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
			// 布尔字段必须严格可解析，避免把 "yes" 当成 true。
			return service.TaskErrorWrapperLocal(fmt.Errorf("generate_audio must be a boolean"), "invalid_request", http.StatusBadRequest)
		}
		metadata["generate_audio"] = value
	}
	if raw := strings.TrimSpace(formValue("watermark")); raw != "" {
		value, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			// watermark 同样只接受标准布尔字符串。
			return service.TaskErrorWrapperLocal(fmt.Errorf("watermark must be a boolean"), "invalid_request", http.StatusBadRequest)
		}
		metadata["watermark"] = value
	}

	req.Task = strings.TrimSpace(formValue("task"))
	req.Metadata = metadata
	c.Set("task_request", req)
	info.Action = constant.TaskActionGenerate
	// 把 mediaID 写入 context，任务入库后由轮询终态统一清理。
	if len(mediaIDs) > 0 {
		// 追加而不是覆盖，兼容同一请求里多次调用适配器的场景。
		existing := c.GetStringSlice(service.SeedanceMediaIDsContextKey)
		c.Set(service.SeedanceMediaIDsContextKey, append(existing, mediaIDs...))
	}
	cleanup = false
	// 走到这里说明请求已成功交给任务流程，素材不能再由本函数清理。
	return nil
}

func seedanceContentItem(kind, publicURL, role string) ContentItem {
	// 按素材类型构造 Ark content item，role 决定上游如何理解素材。
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
	// 渠道 BaseURL 只填域名，路径在这里统一拼接。
	// 后续切换官方 Ark 时只改 BaseURL/Key，不能改这里的路径。
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
	// metadata 可能来自强类型结构，也可能来自 JSON map，两种都要识别。
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

	// 优先使用重定向后的上游名判断能力，缺失时回退请求模型名。
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

	// taskID 必须转义，避免上游 task ID 中包含路径字符。
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
	// 统一处理 BaseURL 末尾斜杠并校验非空。
	// BaseURL 为空时直接报错，避免构造出相对路径请求。
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
		// OpenAI 风格的 images 字段没有 role，按普通图片输入处理。
		for _, imgURL := range req.Images {
			// 这里只保留 URL，role 为空，避免误判成 reference_image。
			r.Content = append(r.Content, ContentItem{
				Type: "image_url",
				ImageURL: &MediaURL{
					URL: imgURL,
				},
			})
		}
	}

	// duration 优先于旧字段 seconds，保证新接口语义正确。
	if req.Duration != 0 {
		// 新字段 duration 存在时优先使用。
		r.Duration = lo.ToPtr(dto.IntValue(req.Duration))
	} else if seconds, _ := strconv.Atoi(req.Seconds); seconds != 0 {
		// 旧字段 seconds 只在 duration 缺失时兜底。
		r.Duration = lo.ToPtr(dto.IntValue(seconds))
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		// metadata 解析失败说明请求结构非法，不能继续生成 payload。
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	// 先补默认值再校验，显式参数不会被默认值覆盖。
	applySeedanceDefaults(&r, capabilityModel)
	if err := validateSeedanceModelParams(&r, capabilityModel); err != nil {
		// 显式参数校验失败直接返回，避免把错误参数发给上游。
		return nil, err
	}

	// 提示词固定放在 content[0]，参考素材保持在后，顺序符合 Ark 协议。
	// 这会影响参考素材编号（如 [Image 1]），不能随手改成追加到末尾。
	r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
	// 提示词固定插到 content[0]，保证参考素材编号从后文开始。
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

	// 上游状态大小写和空白不稳定，统一后再映射内部状态。
	status := strings.ToLower(strings.TrimSpace(resTask.Status))
	switch status {
	case "pending", "queued":
		// 排队阶段还没有任何视频产物。
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		// 生成中给 50% 进度，前端用于展示进度条。
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		// 成功时保存临时视频 URL，后续落盘钩子会替换成本地地址。
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = strings.TrimSpace(resTask.Content.VideoURL)
		// Record usage for ratio-based billing.
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	case "failed", "expired", "cancelled", "canceled":
		// 终态失败统一映射；上游没给原因时补一个可读兜底。
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.Error.Message
		if taskResult.Reason == "" {
			taskResult.Reason = fmt.Sprintf("upstream task %s", status)
		}
	default:
		// Unknown status, treat as processing
		// 未知状态按处理中兜底，避免把正常任务误判为失败。
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

// ProcessTaskResultBeforePersist downloads the temporary upstream video into
// the shared result directory before the polling loop records the public URL.
func (a *TaskAdaptor) ProcessTaskResultBeforePersist(ctx context.Context, task *model.Task, result *relaycommon.TaskInfo) error {
	// 只有成功结果才需要下载落盘，其他状态直接返回。
	if task == nil || result == nil || result.Status != model.TaskStatusSuccess {
		// 只有成功终态才需要下载结果；其他状态直接跳过。
		return nil
	}
	resultURL := strings.TrimSpace(result.Url)
	if resultURL == "" || strings.HasPrefix(resultURL, "data:") {
		// data URL 不适合落盘，空 URL 也没有可下载内容。
		return nil
	}
	if strings.TrimSpace(task.PrivateData.PublicBaseURL) == "" {
		// 没有公网基地址时无法生成持久化 URL，只能保留上游临时地址。
		logger.LogWarn(ctx, fmt.Sprintf("Seedance result persistence skipped for task %s: public base URL is empty", task.TaskID))
		return nil
	}

	// 下载上游结果前必须经过 SSRF 校验，避免访问内网地址。
	// 不能因为 URL 来自上游就跳过该检查。
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
		// SSRF 校验失败也走统一重试/失败处理，避免绕过安全检查。
		return handleSeedancePersistFailure(ctx, task, result, err)
	}

	// 根据上游 output_format 决定保存扩展名，默认 mp4。
	filename := "seedance-result.mp4"
	var upstreamTask responseTask
	if err := common.Unmarshal(task.Data, &upstreamTask); err == nil &&
		strings.EqualFold(strings.TrimSpace(upstreamTask.OutputFormat), "mov") {
		// 只有明确 mov 时才改扩展名，解析失败继续按 mp4 保存。
		filename = "seedance-result.mov"
	}

	// 下载成功后用本地公网 URL 覆盖临时上游 URL。
	// 这样客户端拿到的地址不会过期，也不会暴露上游存储域名。
	persistedURL, err := service.PersistVideoResult(ctx, resultURL, task.PrivateData.PublicBaseURL, task.TaskID, filename)
	if err != nil {
		return handleSeedancePersistFailure(ctx, task, result, err)
	}
	task.PrivateData.SeedanceResultPersistFailures = 0
	result.Url = persistedURL
	// 成功后清零重试计数，避免后续任务误继承失败次数。
	logger.LogInfo(ctx, fmt.Sprintf("Seedance result saved locally: task=%s url=%s", task.TaskID, persistedURL))
	return nil
}

func handleSeedancePersistFailure(ctx context.Context, task *model.Task, result *relaycommon.TaskInfo, persistErr error) error {
	// 落盘失败按轮询周期重试，达到上限后任务转为失败。
	// 重试次数存 PrivateData；不能只放在内存里，否则重启后会无限重试。
	task.PrivateData.SeedanceResultPersistFailures++
	failures := task.PrivateData.SeedanceResultPersistFailures
	result.Url = ""
	// 失败时先清空 URL，防止前端把不可用地址当成成功结果。
	if failures >= seedanceResultPersistAttemptLimit {
		// 达到上限后终止任务，避免无限轮询和重复下载。
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = fmt.Sprintf("save video result failed after %d attempts: %v", seedanceResultPersistAttemptLimit, service.SanitizeURLError(persistErr))
		logger.LogError(ctx, fmt.Sprintf("Seedance result persistence permanently failed: task=%s attempts=%d error=%v", task.TaskID, failures, persistErr))
		return nil
	}
	// 未达到上限时保持生成中，并给一个接近完成的进度。
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

	// 查询接口优先返回持久化 URL，避免把上游临时地址暴露给用户。
	videoURL := strings.TrimSpace(dResp.Content.VideoURL)
	if persistedURL := strings.TrimSpace(originTask.PrivateData.ResultURL); persistedURL != "" {
		// 持久化 URL 优先于上游临时 URL。
		videoURL = persistedURL
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	// metadata.url 是新旧接口兼容字段，客户端仍可从 content 代理取流。
	openAIVideo.SetMetadata("url", videoURL)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName
	// 上游响应通常不含提示词，从任务属性回填，供操练场展示。
	if prompt := strings.TrimSpace(originTask.Properties.Input); prompt != "" {
		// 提示词可能含换行，原样写入 metadata 由前端负责展示。
		openAIVideo.SetMetadata("prompt", prompt)
	}
	if dResp.Duration > 0 {
		// 上游返回了实际时长时一并透出，便于客户端展示。
		openAIVideo.Seconds = strconv.Itoa(dResp.Duration)
	}

	// 失败状态大小写不固定，错误信息为空时回退任务失败原因。
	if strings.EqualFold(strings.TrimSpace(dResp.Status), "failed") {
		// 错误信息优先用上游原始 message，缺失时回退任务失败原因。
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
