package doubao

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 把提交请求转成上游 payload 并反序列化，便于断言默认值和字段裁剪。
func marshalPayload(t *testing.T, req *relaycommon.TaskSubmitReq) map[string]any {
	t.Helper()
	adaptor := &TaskAdaptor{}
	payload, err := adaptor.convertToRequestPayload(req, req.Model)
	require.NoError(t, err)
	data, err := common.Marshal(payload)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, common.Unmarshal(data, &result))
	return result
}

// 测试意图：覆盖 2.0 的默认值矩阵，以及不应发送的 1.x/2.5 专属字段。
func TestSeedance20DefaultsAndUnsupportedFields(t *testing.T) {
	payload := marshalPayload(t, &relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "a test video",
	})

	require.Equal(t, "adaptive", payload["ratio"])
	require.Equal(t, "720p", payload["resolution"])
	require.Equal(t, float64(5), payload["duration"])
	require.Equal(t, true, payload["generate_audio"])
	require.Equal(t, false, payload["watermark"])
	require.Equal(t, false, payload["return_last_frame"])
	require.Equal(t, float64(172800), payload["execution_expires_after"])
	require.Equal(t, float64(0), payload["priority"])
	require.NotContains(t, payload, "seed")
	require.NotContains(t, payload, "camera_fixed")
	require.NotContains(t, payload, "service_tier")
	require.NotContains(t, payload, "frames")
	require.NotContains(t, payload, "draft")
	require.NotContains(t, payload, "omni_reference_task_type")
	require.NotContains(t, payload, "output_format")

	content, ok := payload["content"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, content)
	first, ok := content[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "text", first["type"])
	require.Equal(t, "a test video", first["text"])
}

// 测试意图：无论生成中、成功还是失败，查询结果都要保留原始提示词。
func TestConvertToOpenAIVideoKeepsPromptForEveryStatus(t *testing.T) {
	testCases := []struct {
		name       string
		taskStatus model.TaskStatus
		upstream   string
	}{
		{name: "generating", taskStatus: model.TaskStatusInProgress, upstream: "running"},
		{name: "completed", taskStatus: model.TaskStatusSuccess, upstream: "succeeded"},
		{name: "failed", taskStatus: model.TaskStatusFailure, upstream: "failed"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			data, err := common.Marshal(responseTask{
				ID:     "task-prompt-test",
				Status: testCase.upstream,
				Content: struct {
					VideoURL string `json:"video_url"`
				}{VideoURL: "https://example.com/result.mp4"},
			})
			require.NoError(t, err)

			originTask := &model.Task{
				TaskID:   "task-prompt-test",
				Status:   testCase.taskStatus,
				Progress: "50%",
				Properties: model.Properties{
					Input: "first-person fruit tea advertisement",
				},
				Data: data,
			}

			body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(originTask)
			require.NoError(t, err)

			var video dto.OpenAIVideo
			require.NoError(t, common.Unmarshal(body, &video))
			require.Equal(t, "first-person fruit tea advertisement", video.Metadata["prompt"])
		})
	}
}

func TestSeedance25Defaults(t *testing.T) {
	payload := marshalPayload(t, &relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-5-260628",
		Prompt: "a test video",
	})

	require.Equal(t, false, payload["draft"])
	require.NotContains(t, payload, "omni_reference_task_type")
	require.Equal(t, "mp4", payload["output_format"])
	require.NotContains(t, payload, "seed")
	require.NotContains(t, payload, "camera_fixed")
	require.NotContains(t, payload, "service_tier")
}

// 测试意图：仅多模态参考场景携带 omni 参数，首帧请求不能携带。
// 风险：上游会严格拒绝文生视频或首尾帧请求中的该字段。
func TestSeedance25OmniReferenceTaskTypeOnlyForReferenceMedia(t *testing.T) {
	referencePayload := marshalPayload(t, &relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-5-260628",
		Prompt: "a test video",
		Metadata: map[string]interface{}{
			"content": []ContentItem{{
				Type:     "image_url",
				ImageURL: &MediaURL{URL: "https://example.com/reference.png"},
				Role:     "reference_image",
			}},
		},
	})
	require.Equal(t, "auto", referencePayload["omni_reference_task_type"])

	firstFramePayload := marshalPayload(t, &relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-5-260628",
		Prompt: "a test video",
		Metadata: map[string]interface{}{
			"content": []ContentItem{{
				Type:     "image_url",
				ImageURL: &MediaURL{URL: "https://example.com/first-frame.png"},
				Role:     "first_frame",
			}},
		},
	})
	require.NotContains(t, firstFramePayload, "omni_reference_task_type")
}

func TestSeedanceMappedPublicNameUsesUpstreamCapabilities(t *testing.T) {
	adaptor := &TaskAdaptor{}
	payload, err := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:  "my-public-video-model",
		Prompt: "a test video",
	}, "doubao-seedance-2-5-260628")
	require.NoError(t, err)
	require.Empty(t, payload.OmniReferenceTaskType)
	require.Equal(t, "mp4", payload.OutputFormat)
	require.NotNil(t, payload.Draft)
}

func TestSeedance10Defaults(t *testing.T) {
	payload := marshalPayload(t, &relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-1-0-pro-250528",
		Prompt: "a test video",
	})

	require.Equal(t, float64(-1), payload["seed"])
	require.Equal(t, false, payload["camera_fixed"])
	require.Equal(t, "default", payload["service_tier"])
	require.Equal(t, false, payload["return_last_frame"])
	require.Equal(t, float64(172800), payload["execution_expires_after"])
	require.NotContains(t, payload, "priority")
	require.NotContains(t, payload, "tools")
	require.NotContains(t, payload, "draft")
}

func TestSeedanceRejectsUnsupportedExplicitParams(t *testing.T) {
	adaptor := &TaskAdaptor{}
	_, err := adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "a test video",
		Metadata: map[string]interface{}{
			"seed": 7,
		},
	}, "doubao-seedance-2-0-260128")
	require.ErrorContains(t, err, "seed")

	_, err = adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-0-260128",
		Prompt: "a test video",
		Metadata: map[string]interface{}{
			"output_format": "mp4",
		},
	}, "doubao-seedance-2-0-260128")
	require.ErrorContains(t, err, "output_format")

	_, err = adaptor.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Model:  "doubao-seedance-2-5-260628",
		Prompt: "a test video",
		Metadata: map[string]interface{}{
			"seed": 7,
		},
	}, "doubao-seedance-2-5-260628")
	require.ErrorContains(t, err, "seed")
}

func TestSeedanceBuildURLs(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://example.com/"}
	createURL, err := adaptor.BuildRequestURL(nil)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/api/v3/contents/generations/tasks", createURL)

	fetchURL, err := buildFetchTaskURL("https://example.com/", "cgt/with space")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/api/v3/contents/generations/tasks/cgt%2Fwith%20space", fetchURL)
}

func TestHasVideoInMetadataWithContentItems(t *testing.T) {
	require.True(t, hasVideoInMetadata(map[string]interface{}{
		"content": []ContentItem{{Type: "video_url", VideoURL: &MediaURL{URL: "https://example.com/v.mp4"}}},
	}))
	require.False(t, hasVideoInMetadata(map[string]interface{}{
		"content": []ContentItem{{Type: "image_url", ImageURL: &MediaURL{URL: "https://example.com/i.png"}}},
	}))
}

// 测试意图：成功结果先下载到本地，再把持久化 URL 写回任务。
// 风险：下载失败会进入重试，达到上限后任务按失败处理。
func TestProcessTaskResultBeforePersist(t *testing.T) {
	previousDir := constant.MiniMaxH3ResultDir
	previousPath := constant.MiniMaxH3ResultURLPath
	constant.MiniMaxH3ResultDir = t.TempDir()
	constant.MiniMaxH3ResultURLPath = "/sglang/minimax-h3-results"
	t.Cleanup(func() {
		constant.MiniMaxH3ResultDir = previousDir
		constant.MiniMaxH3ResultURLPath = previousPath
	})

	fetchSetting := system_setting.GetFetchSetting()
	previousSSRF := fetchSetting.EnableSSRFProtection
	fetchSetting.EnableSSRFProtection = false
	t.Cleanup(func() {
		fetchSetting.EnableSSRFProtection = previousSSRF
	})
	service.InitHttpClient()

	videoBytes := []byte("fake mp4 bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(videoBytes)
	}))
	defer server.Close()

	task := &model.Task{
		TaskID: "task_test",
		Data:   []byte(`{"output_format":"mp4"}`),
	}
	task.PrivateData.PublicBaseURL = "https://gateway.example"
	result := &relaycommon.TaskInfo{
		Status:   model.TaskStatusSuccess,
		Progress: "100%",
		Url:      server.URL + "/video.mp4",
	}

	adaptor := &TaskAdaptor{}
	require.NoError(t, adaptor.ProcessTaskResultBeforePersist(context.Background(), task, result))
	taskHash := sha256.Sum256([]byte(task.TaskID))
	expectedURL := "https://gateway.example/sglang/minimax-h3-results/" + hex.EncodeToString(taskHash[:]) + ".mp4"
	require.Equal(t, expectedURL, result.Url)

	content, err := os.ReadFile(filepath.Join(constant.MiniMaxH3ResultDir, hex.EncodeToString(taskHash[:])+".mp4"))
	require.NoError(t, err)
	require.Equal(t, videoBytes, content)
}

// 测试意图：Playground multipart 请求会把参考图落盘并生成公网 URL。
// 风险：测试会临时改写 H3 共享上传目录，结束后必须恢复。
func TestSeedanceMultipartValidationStoresMedia(t *testing.T) {
	previousDir := constant.MiniMaxH3FrameUploadDir
	previousPath := constant.MiniMaxH3FrameURLPath
	constant.MiniMaxH3FrameUploadDir = t.TempDir()
	constant.MiniMaxH3FrameURLPath = "/sglang/minimax-h3-frames"
	t.Cleanup(func() {
		constant.MiniMaxH3FrameUploadDir = previousDir
		constant.MiniMaxH3FrameURLPath = previousPath
	})

	imageBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	require.NoError(t, err)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "doubao-seedance-2-0-260128"))
	require.NoError(t, writer.WriteField("prompt", "a test video"))
	require.NoError(t, writer.WriteField("ratio", "16:9"))
	require.NoError(t, writer.WriteField("resolution", "720p"))
	require.NoError(t, writer.WriteField("duration", "5"))
	require.NoError(t, writer.WriteField("generate_audio", "true"))
	require.NoError(t, writer.WriteField("watermark", "false"))
	part, err := writer.CreateFormFile("reference_images", "test.png")
	require.NoError(t, err)
	_, err = part.Write(imageBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/videos", &body)
	c.Request.Host = "gateway.example"
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request.Header.Set("X-Forwarded-Proto", "https")

	info := &relaycommon.RelayInfo{
		UserId:        7,
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	adaptor := &TaskAdaptor{}
	require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))

	req, err := relaycommon.GetTaskRequest(c)
	require.NoError(t, err)
	content, ok := req.Metadata["content"].([]ContentItem)
	require.True(t, ok)
	require.Len(t, content, 1)
	require.Equal(t, "image_url", content[0].Type)
	require.Equal(t, "reference_image", content[0].Role)
	require.Contains(t, content[0].ImageURL.URL, "https://gateway.example/sglang/minimax-h3-frames/7/")

	mediaIDs := c.GetStringSlice(service.SeedanceMediaIDsContextKey)
	require.Len(t, mediaIDs, 1)
	_, err = os.Stat(filepath.Join(constant.MiniMaxH3FrameUploadDir, "7", mediaIDs[0]))
	require.NoError(t, err)
}
