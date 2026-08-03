package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	responsesImageToolType                 = "image_generation"
	responsesImagePlannerToolName          = "__new_api_responses_image_generation"
	responsesImageChildRequestIDMaxRunes   = 64
	responsesImageChildRequestIDHashLength = 12
)

type responsesImageToolDefaults struct {
	Size              string
	Quality           string
	Background        string
	OutputFormat      string
	Moderation        string
	OutputCompression *int
}

type responsesImageToolArguments struct {
	Prompt            string `json:"prompt"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	Background        string `json:"background,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	OutputCompression *int   `json:"output_compression,omitempty"`
}

type responsesImageChannelSelection struct {
	Channel *model.Channel
	Group   string
}

type responsesImageResult struct {
	Base64        string
	RevisedPrompt string
}

func responsesImagePublicModelName(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	// 服务商模式下优先使用客户端实际看到的公开模型名：
	// 模型匹配规则和最终响应都应基于公开名称，而不是内部基础模型名，
	// 从而避免因为模型映射导致桥接规则失效或泄露内部模型名称。
	if publicModelName := strings.TrimSpace(info.ProviderPublicModel); publicModelName != "" {
		return publicModelName
	}
	// 非服务商模式或未配置公开模型名时，回退到进入中继前记录的原始模型名。
	return strings.TrimSpace(info.OriginModelName)
}

// tryResponsesImageGenerationBridge 为 Responses→Chat 渠道提供本地生图工具桥接。
//
// Responses→Chat 渠道的上游通常只支持 Chat Completions 和普通 function tool，
// 无法直接识别 Responses API 的 image_generation 内置工具。本方法负责完成以下流程：
//  1. 判断当前请求是否满足桥接条件；
//  2. 将 image_generation 临时转换为唯一的内部 function tool；
//  3. 让文本模型充当规划器，生成 prompt、size、quality 等图片参数；
//  4. 截获内部函数调用，并通过现有图片渠道执行真实生图；
//  5. 将内部 function_call 原位替换为标准 image_generation_call；
//  6. 按客户端原始 stream 设置返回普通 JSON 或合成的 Responses SSE。
//
// 返回值中的 handled 表示本方法是否已经接管请求：
//   - false, nil：不满足桥接条件，调用方继续执行普通 Responses→Chat 流程；
//   - true, nil：桥接处理成功，响应已经写入 gin.Context；
//   - true, err：桥接已经接管请求，但处理过程中发生错误。
func tryResponsesImageGenerationBridge(c *gin.Context, info *relaycommon.RelayInfo) (bool, *types.NewAPIError) {
	// -------------------- 第一阶段：判断请求是否适合桥接 --------------------

	// 桥接只处理普通 Responses 请求；压缩接口有自己的请求 DTO 和计费语义，
	// 不能把它误当成带工具调用的对话请求。
	if c == nil || info == nil || info.RelayMode != relayconstant.RelayModeResponses {
		return false, nil
	}
	// Responses→Chat 是桥接的主要渠道类型。普通 OpenAI 兼容渠道也允许
	// “隐式生图”模式：当客户端没有声明 image_generation，但最新用户输入明确
	// 表示要生成图片时，借用该渠道的 Chat Completions 端点作为规划器。
	// 如果客户端已经显式声明 image_generation，则仍交给原生 Responses 渠道处理，
	// 以保留上游原生图片工具的完整能力（例如编辑和 partial_images）。
	if info.ApiType != constant.APITypeResponsesChat && info.ApiType != constant.APITypeOpenAI {
		return false, nil
	}

	// 标准桥接只处理 /v1/responses 对应的 OpenAIResponsesRequest。
	// 请求 DTO 类型不匹配时，将处理权交回原有中继流程。
	request, ok := info.Request.(*dto.OpenAIResponsesRequest)

	// 优先取得服务商公开模型名，其次使用原始模型名。
	// 少数测试或早期初始化场景中 RelayInfo 尚未设置模型名，此时再从请求体回退获取。
	publicModelName := responsesImagePublicModelName(info)
	if ok && request != nil && publicModelName == "" {
		publicModelName = strings.TrimSpace(request.Model)
	}

	// 根据 responses_image_generation_setting 判断该模型是否允许桥接。
	// 默认匹配 ^gpt-.*；配置关闭、图片模型为空或模型名不匹配时均不接管请求。
	if !ok || request == nil || !operation_setting.ShouldBridgeResponsesImageGeneration(publicModelName) {
		return false, nil
	}

	// 从客户端 tools 中查找 image_generation。
	// 即使声明了该工具，如果 tool_choice=none，或 allowed_tools 没有允许图片工具，
	// 也应视为本次请求不允许生图，并回退到普通 Responses→Chat 流程。
	imageTool, found := findResponsesImageTool(request)
	if !responsesImageToolChoiceAllowsImage(request.ToolChoice) {
		return false, nil
	}

	originalRequest := request
	plannerSourceRequest := request
	implicitImageRequest := false
	if !found {
		// Codex 等客户端可能没有把内置 image_gen 工具挂载到会话，因此请求中
		// 根本不会出现 image_generation。只在最新用户文本明确表达生图意图时
		// 注入，避免普通文本请求被无谓地改成非流式规划请求。
		if !operation_setting.ShouldAutoInjectResponsesImageGeneration(responsesImageLatestUserPrompt(request)) {
			return false, nil
		}
		var injectErr error
		plannerSourceRequest, imageTool, injectErr = injectResponsesImageTool(request)
		if injectErr != nil {
			return true, types.NewErrorWithStatusCode(injectErr, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(), types.ErrOptionWithSkipChannelErrorHandling())
		}
		implicitImageRequest = true
	}

	// 对普通 OpenAI 兼容渠道只接管“隐式生图”。显式 image_generation 请求
	// 保持原生 Responses 透传，避免覆盖上游已经实现的图片编辑/中间帧能力。
	if info.ApiType == constant.APITypeOpenAI && !implicitImageRequest {
		return false, nil
	}

	// 记录客户端是否“明确要求”执行图片工具。
	// 该状态会影响后续的失败策略：
	// - 显式要求生图：无图片渠道或规划器未调用工具时不能静默退化；
	// - 自动选择图片：允许模型直接回答文字，也允许无图片渠道时回退普通流程。
	// 隐式模式已经由后端根据用户文本判定为生图请求，因此同样采用显式失败策略。
	explicitImageChoice := implicitImageRequest || isExplicitResponsesImageRequest(plannerSourceRequest)

	// 当前本地桥接复用的是图片生成接口，只支持 auto/generate。
	// action=edit 需要原图、mask、input_fidelity 等编辑语义，应交给原生支持编辑的渠道，
	// 因此在调用规划器前直接返回明确的 400 请求错误。
	if err := validateResponsesImageToolForBridge(imageTool); err != nil {
		return true, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(), types.ErrOptionWithSkipChannelErrorHandling())
	}

	// -------------------- 第二阶段：预先确认图片执行渠道 --------------------

	// 读取桥接配置，并取得内部真正调用的图片模型名称，默认通常为 gpt-image-2。
	bridgeSetting := operation_setting.GetResponsesImageGenerationSetting()
	imageModel := strings.TrimSpace(bridgeSetting.ImageModel)

	// 在当前令牌分组中，为配置的图片模型预选一个普通图片渠道。
	// 在文本规划之前检查渠道，可以避免规划器已经成功并计费后才发现没有图片执行能力。
	selection, selectionErr := selectResponsesImageChannel(c, info.TokenGroup, imageModel, 0)
	if selectionErr != nil || selection == nil || selection.Channel == nil {
		// 客户端明确要求生图时，缺少图片渠道意味着请求无法完成，返回 503。
		// 这里设置 SkipRetry，是为了避免外层错误地重试文本规划渠道；真正的问题是图片渠道不可用。
		if explicitImageChoice {
			if selectionErr == nil {
				selectionErr = fmt.Errorf("no image channel available for model %s", imageModel)
			}
			return true, types.NewErrorWithStatusCode(selectionErr, types.ErrorCodeGetChannelFailed, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(), types.ErrOptionWithSkipChannelErrorHandling())
		}
		// 自动工具选择场景允许安全退化：不接管请求，调用方继续执行普通文本流程。
		return false, nil
	}
	logger.LogInfo(c, fmt.Sprintf(
		"responses image bridge activated: api_type=%d channel_id=%d model=%q image_model=%q implicit=%t",
		info.ApiType,
		info.ChannelId,
		publicModelName,
		imageModel,
		implicitImageRequest,
	))

	// -------------------- 第三阶段：构造并执行文本规划请求 --------------------

	// 构造发给文本规划模型的请求：
	// - 深拷贝原始请求，避免修改客户端的 stream 等原始语义；
	// - 将 image_generation 替换为内部 function tool；
	// - 提取 size、quality、background 等图片默认参数；
	// - 生成不会与用户函数重名的内部工具名称；
	// - 强制规划请求非流式，便于一次性取得完整函数参数。
	plannerRequest, defaults, plannerToolName, err := prepareResponsesImagePlannerRequest(plannerSourceRequest, imageTool, bridgeSetting)
	if err != nil {
		return true, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(), types.ErrOptionWithSkipChannelErrorHandling())
	}

	// 通过当前 Responses→Chat 文本渠道执行规划请求。
	// plannerResponse 是解析后的 Responses 对象，rawPlannerResponse 是未经重新序列化的原始响应体。
	// 如果规划渠道自身失败，则保留原错误，让外层中继逻辑判断是否需要重试或切换文本渠道。
	plannerResponse, rawPlannerResponse, newAPIError := executeResponsesImagePlanner(c, info, plannerRequest)
	if newAPIError != nil {
		return true, newAPIError
	}

	// 只查找“类型为 function_call 且名称等于内部图片工具名”的输出索引。
	// 文本消息和用户自己的函数调用不会被误识别为图片调用。
	callIndexes := responsesImageFunctionCallIndexes(plannerResponse, plannerToolName)

	// 某些只部分支持工具调用的模型可能忽略强制 tool_choice。
	// 当客户端明确要求生图但规划器没有调用内部工具时，人工追加一个空参数函数调用；
	// 后续参数解析会使用客户端原始 input 作为 prompt 兜底，保证显式生图请求仍可执行。
	if len(callIndexes) == 0 && explicitImageChoice {
		appendForcedResponsesImagePlannerCall(plannerResponse, plannerToolName)
		callIndexes = responsesImageFunctionCallIndexes(plannerResponse, plannerToolName)
	}

	// 自动工具选择时，规划器可以判断本次只需要返回文字而不需要生成图片。
	// 此时直接返回规划器响应；优先复用原始响应字节，避免无意义的重新序列化和字段变化。
	if len(callIndexes) == 0 {
		writeResponsesBridgeResult(c, request.IsStream(c), plannerResponse, rawPlannerResponse)
		return true, nil
	}

	// -------------------- 第四阶段：限制并执行图片工具调用 --------------------

	// 限制单次 Responses 请求中允许执行的图片调用数量，
	// 防止模型异常重复调用造成重复生图、重复收费或过大的响应体。
	// 配置值小于等于 0 时仍按 1 次处理，保证默认行为始终有明确上限。
	maxCalls := bridgeSetting.MaxCalls
	if maxCalls <= 0 {
		maxCalls = 1
	}
	if len(callIndexes) > maxCalls {
		return true, types.NewErrorWithStatusCode(
			fmt.Errorf("image generation tool requested %d calls, maximum is %d", len(callIndexes), maxCalls),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
			types.ErrOptionWithSkipChannelErrorHandling(),
		)
	}

	// 从客户端原始 input 中提取文字作为最终 prompt 兜底。
	// 图片参数的总体优先级为：规划器函数参数 > 客户端图片工具参数 > 全局默认配置；
	// prompt 缺失时则回退到这里提取的最新用户文本。
	// background、output_format、output_compression、moderation 属于客户端
	// 图片工具配置，不接受规划模型自行补写，避免模型臆测出图片渠道不支持的参数。
	fallbackPrompt := responsesImageFallbackPrompt(originalRequest)

	// 一个规划响应可能同时包含文本、用户函数和一个或多个内部图片调用。
	// 这里只串行执行图片调用，并在原输出位置替换对应的内部 function_call，其他输出保持不变。
	for callNumber, outputIndex := range callIndexes {
		// 保存当前内部函数调用；其 call_id 用于构造图片子请求 ID 和最终图片输出 ID。
		output := plannerResponse.Output[outputIndex]

		// 解析规划器生成的 JSON 参数，并补齐 size、quality 以及客户端显式设置的高级参数。
		// 如果参数 JSON 非法或最终 prompt 仍为空，则按客户端请求错误处理，
		// 避免向图片渠道发送无法执行的请求。
		arguments, parseErr := parseResponsesImageToolArguments(output.ArgumentsString(), defaults, fallbackPrompt)
		if parseErr != nil {
			return true, types.NewErrorWithStatusCode(parseErr, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog(), types.ErrOptionWithSkipChannelErrorHandling())
		}

		// 将内部函数参数转换成标准图片请求，并使用配置的图片模型和 Base64 响应格式。
		imageRequest := responsesImageRequestFromArguments(imageModel, arguments)

		// 通过独立图片渠道执行真实生图。
		// 该过程拥有自己的渠道上下文、计费、退款和状态码重试逻辑；
		// 图片渠道错误也会被隔离，避免外层误将故障归因到已经成功的文本规划渠道。
		result, imageErr := executeResponsesImageWithRetry(c, info, selection, imageRequest, output.CallId, callNumber)
		if imageErr != nil {
			return true, imageErr
		}

		// 优先返回图片服务提供的 revised_prompt。
		// 如果上游没有返回，则使用实际提交给图片服务的 prompt，确保客户端能看到对应提示词。
		revisedPrompt := strings.TrimSpace(result.RevisedPrompt)
		if revisedPrompt == "" {
			revisedPrompt = arguments.Prompt
		}

		// 内部 function_call 只是让 Chat 模型表达“请执行生图”的实现细节，不能直接暴露给客户端。
		// 图片已经由网关执行完成，因此在原输出位置将其替换成标准 image_generation_call：
		// - 保留规划响应中的整体输出顺序；
		// - 避免客户端误以为还需要自行执行内部函数并回传 function_call_output；
		// - 使 Responses 客户端能够直接识别 result 中的 Base64 图片。
		plannerResponse.Output[outputIndex] = dto.ResponsesOutput{
			Type:          dto.ResponsesOutputTypeImageGenerationCall,
			ID:            responsesImageOutputID(output, callNumber),
			Status:        "completed",
			Quality:       arguments.Quality,
			Size:          arguments.Size,
			Result:        result.Base64,
			RevisedPrompt: revisedPrompt,
		}
	}

	// -------------------- 第五阶段：返回客户端可见结果 --------------------

	// 将模型名恢复成客户端看到的公开模型名，避免返回内部映射后的规划模型名称。
	plannerResponse.Model = publicModelName

	// 根据客户端原始请求的 stream 设置输出：
	// - stream=false：重新序列化已经替换过图片输出的 plannerResponse；
	// - stream=true：根据完整结果合成 image_generation_call 相关 Responses SSE。
	// raw 参数传 nil，是因为响应对象已经被修改，不能再复用规划器的原始响应字节。
	writeResponsesBridgeResult(c, request.IsStream(c), plannerResponse, nil)
	return true, nil
}

func findResponsesImageTool(request *dto.OpenAIResponsesRequest) (map[string]any, bool) {
	if request == nil {
		return nil, false
	}
	for _, tool := range request.GetToolsMap() {
		if strings.EqualFold(strings.TrimSpace(common.Interface2String(tool["type"])), responsesImageToolType) {
			return tool, true
		}
	}
	return nil, false
}

// injectResponsesImageTool 为没有声明内置图片工具的客户端创建一个等价的
// Responses 请求副本。原请求保持不变，副本只用于规划器，因此客户端的
// tool_choice、stream 和后续计费语义不会被本地准备阶段直接改写。
func injectResponsesImageTool(request *dto.OpenAIResponsesRequest) (*dto.OpenAIResponsesRequest, map[string]any, error) {
	if request == nil {
		return nil, nil, fmt.Errorf("cannot inject image tool into a nil Responses request")
	}
	plannerRequest, err := common.DeepCopy(request)
	if err != nil {
		return nil, nil, fmt.Errorf("copy responses request for implicit image tool: %w", err)
	}

	imageTool := map[string]any{
		"type":   responsesImageToolType,
		"action": "generate",
	}
	tools := append(plannerRequest.GetToolsMap(), imageTool)
	plannerRequest.Tools, err = common.Marshal(tools)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal implicit image tool: %w", err)
	}
	plannerRequest.ToolChoice, err = common.Marshal(map[string]any{
		"type": responsesImageToolType,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal implicit image tool choice: %w", err)
	}
	return plannerRequest, imageTool, nil
}

// responsesImageLatestUserPrompt 提取 Responses input 中最后一条用户文本。
// Codex 会在后续轮次携带工具输出和历史消息；只看最后一条 user 消息可以
// 避免旧的图片请求或 imagegen 技能说明触发新的隐式生图。
func responsesImageLatestUserPrompt(request *dto.OpenAIResponsesRequest) string {
	if request == nil || len(request.Input) == 0 {
		return ""
	}
	if common.GetJsonType(request.Input) == "string" {
		var text string
		if common.Unmarshal(request.Input, &text) == nil {
			return strings.TrimSpace(text)
		}
		return ""
	}

	var items []map[string]any
	if common.Unmarshal(request.Input, &items) != nil {
		return ""
	}
	for index := len(items) - 1; index >= 0; index-- {
		item := items[index]
		role := strings.ToLower(strings.TrimSpace(common.Interface2String(item["role"])))
		if role != "" && role != "user" {
			continue
		}
		if text := responsesImageTextFromValue(item["content"]); text != "" {
			return text
		}
		if role == "user" {
			if text := responsesImageTextFromValue(item["text"]); text != "" {
				return text
			}
		}
	}
	return ""
}

// responsesImageTextFromValue 只读取 input_text/text 字段，不读取 function
// call 的 output 字段，避免把工具结果误当成用户的生图意图。
func responsesImageTextFromValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		typeName := strings.ToLower(strings.TrimSpace(common.Interface2String(typed["type"])))
		if typeName == "" || typeName == "input_text" || typeName == "text" {
			return strings.TrimSpace(common.Interface2String(typed["text"]))
		}
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := responsesImageTextFromValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return ""
}

func isExplicitResponsesImageToolChoice(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	var choiceString string
	if common.Unmarshal(raw, &choiceString) == nil {
		return choiceString == responsesImageToolType
	}
	var choice map[string]any
	if common.Unmarshal(raw, &choice) != nil {
		return false
	}
	if common.Interface2String(choice["type"]) == responsesImageToolType {
		return true
	}
	if common.Interface2String(choice["type"]) != "allowed_tools" ||
		common.Interface2String(choice["mode"]) != "required" {
		return false
	}
	allowed, ok := choice["tools"].([]any)
	if !ok || len(allowed) == 0 {
		return false
	}
	for _, item := range allowed {
		tool, ok := item.(map[string]any)
		if !ok || common.Interface2String(tool["type"]) != responsesImageToolType {
			return false
		}
	}
	return true
}

func isExplicitResponsesImageRequest(request *dto.OpenAIResponsesRequest) bool {
	if request == nil {
		return false
	}
	if isExplicitResponsesImageToolChoice(request.ToolChoice) {
		return true
	}
	var choiceString string
	if common.Unmarshal(request.ToolChoice, &choiceString) != nil || choiceString != "required" {
		return false
	}
	tools := request.GetToolsMap()
	if len(tools) == 0 {
		return false
	}
	for _, tool := range tools {
		if common.Interface2String(tool["type"]) != responsesImageToolType {
			return false
		}
	}
	return true
}

func responsesImageToolChoiceAllowsImage(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var choiceString string
	if common.Unmarshal(raw, &choiceString) == nil {
		return choiceString != "none"
	}
	var choice map[string]any
	if common.Unmarshal(raw, &choice) != nil {
		return true
	}
	choiceType := common.Interface2String(choice["type"])
	switch choiceType {
	case "", responsesImageToolType:
		return true
	case "allowed_tools":
		allowed, ok := choice["tools"].([]any)
		if !ok {
			return false
		}
		for _, item := range allowed {
			if tool, ok := item.(map[string]any); ok && common.Interface2String(tool["type"]) == responsesImageToolType {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func prepareResponsesImagePlannerRequest(
	request *dto.OpenAIResponsesRequest,
	imageTool map[string]any,
	setting *operation_setting.ResponsesImageGenerationSetting,
) (*dto.OpenAIResponsesRequest, responsesImageToolDefaults, string, error) {
	plannerRequest, err := common.DeepCopy(request)
	if err != nil {
		return nil, responsesImageToolDefaults{}, "", fmt.Errorf("copy responses request: %w", err)
	}
	defaults := responsesImageToolDefaults{
		Size:         strings.TrimSpace(setting.DefaultSize),
		Quality:      strings.TrimSpace(setting.DefaultQuality),
		Background:   common.Interface2String(imageTool["background"]),
		OutputFormat: common.Interface2String(imageTool["output_format"]),
		Moderation:   common.Interface2String(imageTool["moderation"]),
	}
	//如果请求中有size则覆盖默认配置
	if value := common.Interface2String(imageTool["size"]); value != "" {
		defaults.Size = value
	}
	//如果请求中有quality则覆盖默认配置
	if value := common.Interface2String(imageTool["quality"]); value != "" {
		defaults.Quality = value
	}
	defaults.OutputCompression = interfaceIntPointer(imageTool["output_compression"]) //图片压缩率
	//生成唯一的内部函数名
	plannerToolName := uniqueResponsesImagePlannerToolName(request.GetToolsMap())
	allowedFunctionNames, restrictFunctions := responsesImageAllowedFunctionNames(plannerRequest.ToolChoice)

	tools := request.GetToolsMap()
	convertedTools := make([]map[string]any, 0, len(tools))
	addedImagePlannerTool := false
	for _, tool := range tools {
		if common.Interface2String(tool["type"]) != responsesImageToolType {
			if restrictFunctions && common.Interface2String(tool["type"]) == "function" {
				if _, allowed := allowedFunctionNames[responsesFunctionToolName(tool)]; !allowed {
					continue
				}
			}
			convertedTools = append(convertedTools, tool)
			continue
		}
		if !addedImagePlannerTool {
			convertedTools = append(convertedTools, responsesImagePlannerTool(plannerToolName))
			addedImagePlannerTool = true
		}
	}
	toolsJSON, err := common.Marshal(convertedTools)
	if err != nil {
		return nil, responsesImageToolDefaults{}, "", fmt.Errorf("marshal planner tools: %w", err)
	}
	plannerRequest.Tools = toolsJSON
	plannerRequest.Stream = common.GetPointer(false)
	plannerRequest.StreamOptions = nil

	toolChoice, err := responsesImagePlannerToolChoice(plannerRequest.ToolChoice, plannerToolName)
	if err != nil {
		return nil, responsesImageToolDefaults{}, "", err
	}
	plannerRequest.ToolChoice = toolChoice
	return plannerRequest, defaults, plannerToolName, nil
}

func validateResponsesImageToolForBridge(imageTool map[string]any) error {
	action := strings.ToLower(strings.TrimSpace(common.Interface2String(imageTool["action"])))
	switch action {
	case "", "auto", "generate":
		return nil
	case "edit":
		return fmt.Errorf("responses image bridge only supports image generation; action=edit requires a native Responses image tool channel")
	default:
		return fmt.Errorf("invalid image_generation action %q", action)
	}
}

// uniqueResponsesImagePlannerToolName 函数用于生成一个唯一的工具名称
// 它会检查已使用的工具名称，并在冲突时添加后缀以确保名称的唯一性
// 参数:
//
//	tools - 包含多个工具信息的map切片，每个map代表一个工具
//
// 返回值:
//
//	string - 返回一个唯一的工具名称，如果原始名称可用则直接返回，否则添加数字后缀
func uniqueResponsesImagePlannerToolName(tools []map[string]any) string {
	// usedNames 用于记录已使用的工具名称，使用map结构提高查找效率
	// 初始化时分配与tools相同大小的容量，避免频繁扩容
	usedNames := make(map[string]struct{}, len(tools))
	// 遍历所有工具
	for _, tool := range tools {
		// 跳过类型不为"function"的工具
		if common.Interface2String(tool["type"]) != "function" {
			continue
		}
		// 获取当前工具的名称
		name := responsesFunctionToolName(tool)
		// 如果名称不为空，则将其添加到已使用名称的集合中
		if name != "" {
			usedNames[name] = struct{}{}
		}
	}
	// 检查默认的图片规划器工具名称是否已被使用
	if _, exists := usedNames[responsesImagePlannerToolName]; !exists {
		// 如果未被使用，直接返回默认名称
		return responsesImagePlannerToolName
	}
	// 如果默认名称已被使用，则尝试添加数字后缀
	// 从2开始递增，直到找到一个未被使用的名称
	for suffix := 2; ; suffix++ {
		// 生成带后缀的候选名称
		candidate := fmt.Sprintf("%s_%d", responsesImagePlannerToolName, suffix)
		// 检查候选名称是否已被使用
		if _, exists := usedNames[candidate]; !exists {
			// 如果未被使用，则返回该候选名称
			return candidate
		}
	}
}

func responsesFunctionToolName(tool map[string]any) string {
	name := strings.TrimSpace(common.Interface2String(tool["name"]))
	if name != "" {
		return name
	}
	if function, ok := tool["function"].(map[string]any); ok {
		return strings.TrimSpace(common.Interface2String(function["name"]))
	}
	return ""
}

func responsesImageAllowedFunctionNames(raw []byte) (map[string]struct{}, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var choice map[string]any
	if common.Unmarshal(raw, &choice) != nil || common.Interface2String(choice["type"]) != "allowed_tools" {
		return nil, false
	}
	allowedNames := make(map[string]struct{})
	allowed, _ := choice["tools"].([]any)
	for _, item := range allowed {
		tool, ok := item.(map[string]any)
		if !ok || common.Interface2String(tool["type"]) != "function" {
			continue
		}
		if name := responsesFunctionToolName(tool); name != "" {
			allowedNames[name] = struct{}{}
		}
	}
	return allowedNames, true
}

func responsesImagePlannerTool(toolName string) map[string]any {
	return map[string]any{
		"type":        "function",
		"name":        toolName,
		"description": "Generate an image. Call this whenever the user asks to create, draw, render, design, or generate an image. Convert the request into a complete standalone visual prompt.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt":  map[string]any{"type": "string", "description": "Complete standalone image prompt."},
				"size":    map[string]any{"type": "string", "description": "Requested output size, for example 1024x1024, 1024x1536, or 1536x1024."},
				"quality": map[string]any{"type": "string", "description": "Requested image quality, for example low, medium, high, standard, or hd."},
			},
			"required":             []string{"prompt"},
			"additionalProperties": false,
		},
	}
}

func responsesImagePlannerToolChoice(raw []byte, plannerToolName string) ([]byte, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var choiceString string
	if common.Unmarshal(raw, &choiceString) == nil {
		if choiceString != responsesImageToolType {
			return raw, nil
		}
		return common.Marshal(map[string]any{
			"type":     "function",
			"function": map[string]any{"name": plannerToolName},
		})
	}

	var choice map[string]any
	if err := common.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("parse tool_choice: %w", err)
	}
	choiceType := common.Interface2String(choice["type"])
	if choiceType == "allowed_tools" {
		mode := common.Interface2String(choice["mode"])
		if mode == "required" && isExplicitResponsesImageToolChoice(raw) {
			return common.Marshal(map[string]any{
				"type":     "function",
				"function": map[string]any{"name": plannerToolName},
			})
		}
		if mode == "auto" || mode == "required" {
			return common.Marshal(mode)
		}
		return raw, nil
	}
	if choiceType == "function" {
		name := responsesFunctionToolName(choice)
		if name == "" {
			return raw, nil
		}
		return common.Marshal(map[string]any{
			"type":     "function",
			"function": map[string]any{"name": name},
		})
	}
	if choiceType != responsesImageToolType {
		return raw, nil
	}
	return common.Marshal(map[string]any{
		"type":     "function",
		"function": map[string]any{"name": plannerToolName},
	})
}

func executeResponsesImagePlanner(
	parent *gin.Context,
	info *relaycommon.RelayInfo,
	request *dto.OpenAIResponsesRequest,
) (*dto.OpenAIResponsesResponse, []byte, *types.NewAPIError) {
	body, err := common.Marshal(request)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
	}
	plannerContext, recorder, cleanup, err := newResponsesInternalContext(parent, http.MethodPost, "/v1/responses", body, true)
	if err != nil {
		return nil, nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	defer cleanup()

	plannerInfo := cloneResponsesPlannerRelayInfo(info, request)
	newAPIError := responsesViaChatCompletions(plannerContext, plannerInfo)
	if newAPIError != nil {
		return nil, nil, newAPIError
	}

	rawResponse := append([]byte(nil), recorder.Body.Bytes()...)
	var response dto.OpenAIResponsesResponse
	if err := common.Unmarshal(rawResponse, &response); err != nil {
		return nil, nil, types.NewErrorWithStatusCode(
			fmt.Errorf("parse planner responses output: %w", err),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}
	if openAIError := response.GetOpenAIError(); openAIError != nil && openAIError.Type != "" {
		return nil, nil, types.WithOpenAIError(*openAIError, recorder.Code)
	}
	return &response, rawResponse, nil
}

func cloneResponsesPlannerRelayInfo(info *relaycommon.RelayInfo, request *dto.OpenAIResponsesRequest) *relaycommon.RelayInfo {
	cloned := *info
	cloned.Request = request
	cloned.IsStream = false
	cloned.RequestConversionChain = append([]types.RelayFormat(nil), info.RequestConversionChain...)
	if info.ChannelMeta != nil {
		channelMeta := *info.ChannelMeta
		cloned.ChannelMeta = &channelMeta
	}
	if info.PriceData.OtherRatios != nil {
		cloned.PriceData.OtherRatios = make(map[string]float64, len(info.PriceData.OtherRatios))
		for key, value := range info.PriceData.OtherRatios {
			cloned.PriceData.OtherRatios[key] = value
		}
	}
	return &cloned
}

func responsesImageFunctionCallIndexes(response *dto.OpenAIResponsesResponse, plannerToolName string) []int {
	if response == nil {
		return nil
	}
	indexes := make([]int, 0, 1)
	for index, output := range response.Output {
		if output.Type == "function_call" && output.Name == plannerToolName {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func appendForcedResponsesImagePlannerCall(response *dto.OpenAIResponsesResponse, plannerToolName string) {
	if response == nil {
		return
	}
	callID := "call_" + common.GetRandomString(16)
	response.Output = append(response.Output, dto.ResponsesOutput{
		Type:      "function_call",
		ID:        "fc_" + callID,
		Status:    "completed",
		CallId:    callID,
		Name:      plannerToolName,
		Arguments: rawString("{}"),
	})
}

func responsesImageFallbackPrompt(request *dto.OpenAIResponsesRequest) string {
	if request == nil {
		return ""
	}
	// Responses 请求通常会携带多轮历史、开发者指令和工具结果。规划器没有
	// 返回 prompt 时，优先只使用最新一条用户消息，避免把整段会话发送给图片渠道。
	if prompt := responsesImageLatestUserPrompt(request); prompt != "" {
		return prompt
	}
	// 少数兼容客户端使用不带 role 的旧式 input 结构；只有无法识别最新用户
	// 消息时，才回退到 DTO 的通用文本提取逻辑。
	parts := request.ParseInput()
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part.Text) != "" {
			texts = append(texts, strings.TrimSpace(part.Text))
		}
	}
	return strings.Join(texts, "\n")
}

func parseResponsesImageToolArguments(
	raw string,
	defaults responsesImageToolDefaults,
	fallbackPrompt string,
) (responsesImageToolArguments, error) {
	arguments := responsesImageToolArguments{}
	if strings.TrimSpace(raw) != "" {
		if err := common.UnmarshalJsonStr(raw, &arguments); err != nil {
			return arguments, fmt.Errorf("parse image generation arguments: %w", err)
		}
	}
	if strings.TrimSpace(arguments.Prompt) == "" {
		arguments.Prompt = strings.TrimSpace(fallbackPrompt)
	}
	if arguments.Prompt == "" {
		return arguments, fmt.Errorf("image generation prompt is empty")
	}
	if arguments.Size == "" {
		arguments.Size = defaults.Size
	}
	if arguments.Quality == "" {
		arguments.Quality = defaults.Quality
	}
	// 这些高级参数只能来自客户端显式提供的 image_generation 工具配置。
	// 规划模型即使违反函数 schema 返回了额外字段，也必须在这里覆盖掉；否则它可能
	// 自行选择 transparent 等当前图片模型不支持的值，造成原本可执行的请求返回 400。
	arguments.Background = defaults.Background
	arguments.OutputFormat = defaults.OutputFormat
	arguments.Moderation = defaults.Moderation
	arguments.OutputCompression = defaults.OutputCompression
	return arguments, nil
}

func responsesImageRequestFromArguments(modelName string, arguments responsesImageToolArguments) *dto.ImageRequest {
	one := uint(1)
	request := &dto.ImageRequest{
		Model:          modelName,
		Prompt:         arguments.Prompt,
		N:              &one,
		Size:           arguments.Size,
		Quality:        arguments.Quality,
		ResponseFormat: "b64_json",
	}
	request.Background = rawString(arguments.Background)
	request.OutputFormat = rawString(arguments.OutputFormat)
	request.Moderation = rawString(arguments.Moderation)
	request.OutputCompression = rawIntPointer(arguments.OutputCompression)
	return request
}

func executeResponsesImageWithRetry(
	parent *gin.Context,
	parentInfo *relaycommon.RelayInfo,
	initialSelection *responsesImageChannelSelection,
	request *dto.ImageRequest,
	callID string,
	callNumber int,
) (*responsesImageResult, *types.NewAPIError) {
	var lastError *types.NewAPIError
	for retry := 0; retry <= common.RetryTimes; retry++ {
		selection := initialSelection
		if retry > 0 || selection == nil || selection.Channel == nil {
			var err error
			selection, err = selectResponsesImageChannel(parent, parentInfo.TokenGroup, request.Model, retry)
			if err != nil {
				lastError = types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
				break
			}
		}
		if selection == nil || selection.Channel == nil {
			lastError = types.NewErrorWithStatusCode(
				fmt.Errorf("no image channel available for model %s", request.Model),
				types.ErrorCodeGetChannelFailed,
				http.StatusServiceUnavailable,
				types.ErrOptionWithSkipRetry(),
			)
			break
		}

		result, imageInfo, requestCompleted, newAPIError := executeResponsesImageRequest(parent, parentInfo, selection, request, callID, callNumber)
		if newAPIError == nil {
			return result, nil
		}
		lastError = newAPIError
		logger.LogError(parent, fmt.Sprintf(
			"responses image bridge request failed: image_channel_id=%d image_model=%q retry=%d status_code=%d error=%s",
			selection.Channel.Id,
			request.Model,
			retry,
			newAPIError.StatusCode,
			newAPIError.Error(),
		))
		if imageInfo != nil && imageInfo.Billing != nil && imageInfo.Billing.NeedsRefund() {
			imageInfo.Billing.Refund(parent)
		}
		// 图片适配器一旦完成请求并结算，后续错误只可能来自本地响应转换。
		// 此时禁止再次调用图片渠道，避免重复生成和重复扣费。
		if requestCompleted {
			break
		}
		if !shouldRetryResponsesImage(newAPIError, common.RetryTimes-retry) {
			break
		}
	}
	return nil, responsesImageBridgeClientError(lastError)
}

func responsesImageBridgeClientError(source *types.NewAPIError) *types.NewAPIError {
	if source == nil {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("image generation service failed"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
			types.ErrOptionWithSkipChannelErrorHandling(),
		)
	}

	statusCode := source.StatusCode
	if statusCode < 400 || statusCode > 599 {
		statusCode = http.StatusBadGateway
	}
	errorCode := source.GetErrorCode()
	if errorCode == "" {
		errorCode = types.ErrorCodeBadResponse
	}
	if types.IsChannelError(source) {
		errorCode = types.ErrorCodeBadResponse
		statusCode = http.StatusBadGateway
	}
	message := "image generation service failed"
	switch errorCode {
	case types.ErrorCodeGetChannelFailed:
		statusCode = http.StatusServiceUnavailable
		message = "no image generation channel is available"
	case types.ErrorCodeInsufficientUserQuota, types.ErrorCodePreConsumeTokenQuotaFailed:
		message = "image generation quota check failed"
	}
	return types.NewErrorWithStatusCode(
		fmt.Errorf("%s", message),
		errorCode,
		statusCode,
		types.ErrOptionWithSkipRetry(),
		types.ErrOptionWithNoRecordErrorLog(),
		types.ErrOptionWithSkipChannelErrorHandling(),
	)
}

func executeResponsesImageRequest(
	parent *gin.Context,
	parentInfo *relaycommon.RelayInfo,
	selection *responsesImageChannelSelection,
	request *dto.ImageRequest,
	callID string,
	callNumber int,
) (*responsesImageResult, *relaycommon.RelayInfo, bool, *types.NewAPIError) {
	body, err := common.Marshal(request)
	if err != nil {
		return nil, nil, false, types.NewError(err, types.ErrorCodeJsonMarshalFailed, types.ErrOptionWithSkipRetry())
	}
	imageContext, recorder, cleanup, err := newResponsesInternalContext(parent, http.MethodPost, "/v1/images/generations", body, false)
	if err != nil {
		return nil, nil, false, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	defer cleanup()

	common.SetContextKey(imageContext, constant.ContextKeyUsingGroup, selection.Group)
	if parentInfo.TokenGroup == "auto" {
		common.SetContextKey(imageContext, constant.ContextKeyAutoGroup, selection.Group)
	}
	common.SetContextKey(imageContext, constant.ContextKeyRequestStartTime, time.Now())
	requestID := parentInfo.RequestId
	if requestID == "" {
		requestID = common.GetTimeString() + common.GetRandomString(8)
	}
	requestID = responsesImageChildRequestID(requestID, callID, callNumber)
	imageContext.Set(common.RequestIdKey, requestID)

	if newAPIError := middleware.SetupContextForSelectedChannel(imageContext, selection.Channel, request.Model); newAPIError != nil {
		return nil, nil, false, newAPIError
	}

	imageInfo, err := relaycommon.GenRelayInfo(imageContext, types.RelayFormatOpenAIImage, request, nil)
	if err != nil {
		return nil, nil, false, types.NewError(err, types.ErrorCodeGenRelayInfoFailed, types.ErrOptionWithSkipRetry())
	}
	meta := request.GetTokenCountMeta()
	tokens, err := service.EstimateRequestToken(imageContext, meta, imageInfo)
	if err != nil {
		return nil, imageInfo, false, types.NewError(err, types.ErrorCodeCountTokenFailed, types.ErrOptionWithSkipRetry())
	}
	imageInfo.SetEstimatePromptTokens(tokens)
	priceData, err := helper.ModelPriceHelper(imageContext, imageInfo, tokens, meta)
	if err != nil {
		return nil, imageInfo, false, types.NewErrorWithStatusCode(err, types.ErrorCodeModelPriceError, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if !priceData.FreeModel {
		if newAPIError := service.PreConsumeBilling(imageContext, priceData.QuotaToPreConsume, imageInfo); newAPIError != nil {
			return nil, imageInfo, false, newAPIError
		}
	}

	if newAPIError := ImageHelper(imageContext, imageInfo); newAPIError != nil {
		return nil, imageInfo, false, newAPIError
	}

	var imageResponse dto.ImageResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &imageResponse); err != nil {
		return nil, imageInfo, true, types.NewErrorWithStatusCode(
			fmt.Errorf("parse image service response: %w", err),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
			types.ErrOptionWithSkipRetry(),
		)
	}
	if len(imageResponse.Data) == 0 {
		return nil, imageInfo, true, types.NewErrorWithStatusCode(
			fmt.Errorf("image service returned no image data"),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
			types.ErrOptionWithSkipRetry(),
		)
	}
	result, err := responsesImageDataToResult(imageResponse.Data[0])
	if err != nil {
		return nil, imageInfo, true, types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
	}
	return result, imageInfo, true, nil
}

// responsesImageChildRequestID 为图片子请求生成可追踪且适合持久化的请求 ID。
//
// logs.request_id 在三种受支持数据库中统一为 varchar(64)。父 Responses 请求 ID
// 本身可能已经接近 64 个字符，如果直接拼接完整 function call_id，图片已成功生成后
// 记录消费日志时会触发 PostgreSQL SQLSTATE 22001。短 ID 保留原来的可读格式；只有
// 超长时才把完整候选值压缩成 12 位稳定摘要，并尽量保留父请求 ID，便于日志检索。
// 使用 rune 计数是为了与数据库 varchar(64) 的字符长度语义一致，并避免截断 UTF-8。
func responsesImageChildRequestID(parentRequestID, callID string, callNumber int) string {
	callID = strings.TrimPrefix(strings.TrimSpace(callID), "call_")
	if callID == "" {
		callID = "unknown"
	}

	candidate := fmt.Sprintf("%s-img-%d-%s", parentRequestID, callNumber, callID)
	if len([]rune(candidate)) <= responsesImageChildRequestIDMaxRunes {
		return candidate
	}

	fingerprint := common.Sha1([]byte(candidate))[:responsesImageChildRequestIDHashLength]
	suffix := fmt.Sprintf("-img-%d-%s", callNumber, fingerprint)
	maxParentRunes := responsesImageChildRequestIDMaxRunes - len([]rune(suffix))
	if maxParentRunes <= 0 {
		// callNumber 来自受 MaxCalls 限制的切片下标，正常运行不会进入该分支；
		// 保留防御性处理，确保未来调用方式变化后仍不会超过数据库字段长度。
		return fingerprint
	}

	parentRunes := []rune(parentRequestID)
	if len(parentRunes) > maxParentRunes {
		parentRunes = parentRunes[:maxParentRunes]
	}
	return string(parentRunes) + suffix
}

func selectResponsesImageChannel(parent *gin.Context, tokenGroup, modelName string, retry int) (*responsesImageChannelSelection, error) {
	selectionContext, _, cleanup, err := newResponsesInternalContext(parent, http.MethodPost, "/v1/images/generations", nil, false)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if tokenGroup == "" {
		tokenGroup = common.GetContextKeyString(selectionContext, constant.ContextKeyUserGroup)
	}
	param := &service.RetryParam{
		Ctx:        selectionContext,
		TokenGroup: tokenGroup,
		ModelName:  modelName,
		Retry:      common.GetPointer(retry),
	}
	channel, selectedGroup, err := service.CacheGetRandomSatisfiedChannel(param)
	if err != nil {
		return nil, err
	}
	if selectedGroup == "" {
		selectedGroup = tokenGroup
	}
	return &responsesImageChannelSelection{Channel: channel, Group: selectedGroup}, nil
}

func shouldRetryResponsesImage(newAPIError *types.NewAPIError, retriesRemaining int) bool {
	if newAPIError == nil || retriesRemaining <= 0 || types.IsSkipRetryError(newAPIError) {
		return false
	}
	if types.IsChannelError(newAPIError) {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(newAPIError.GetErrorCode()) {
		return false
	}
	return operation_setting.ShouldRetryByStatusCode(newAPIError.StatusCode)
}

func responsesImageDataToResult(data dto.ImageData) (*responsesImageResult, error) {
	base64Data := strings.TrimSpace(data.B64Json)
	if base64Data == "" {
		imageURL := strings.TrimSpace(data.Url)
		if strings.HasPrefix(imageURL, "data:") {
			if comma := strings.IndexByte(imageURL, ','); comma >= 0 {
				base64Data = imageURL[comma+1:]
			}
		} else if imageURL != "" {
			_, downloaded, err := service.GetImageFromUrl(imageURL)
			if err != nil {
				return nil, err
			}
			base64Data = downloaded
		}
	}
	if base64Data == "" {
		return nil, fmt.Errorf("image service response has neither b64_json nor url")
	}
	return &responsesImageResult{Base64: base64Data, RevisedPrompt: data.RevisedPrompt}, nil
}

func responsesImageOutputID(output dto.ResponsesOutput, index int) string {
	base := strings.TrimSpace(output.CallId)
	if base == "" {
		base = strings.TrimPrefix(strings.TrimSpace(output.ID), "fc_")
	}
	if base == "" {
		base = fmt.Sprintf("%d_%s", index, common.GetRandomString(8))
	}
	return "ig_" + strings.TrimPrefix(base, "call_")
}

func writeResponsesBridgeResult(c *gin.Context, stream bool, response *dto.OpenAIResponsesResponse, raw []byte) {
	if stream {
		writeSyntheticResponsesStream(c, response)
		return
	}
	if raw == nil {
		raw, _ = common.Marshal(response)
	}
	c.Header("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, nil, raw)
}

func writeSyntheticResponsesStream(c *gin.Context, response *dto.OpenAIResponsesResponse) {
	if response == nil {
		return
	}
	helper.SetEventStreamHeaders(c)
	sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{
		Type: "response.created",
		Response: &dto.OpenAIResponsesResponse{
			ID:        response.ID,
			Object:    response.Object,
			CreatedAt: response.CreatedAt,
			Status:    rawString("in_progress"),
			Model:     response.Model,
			Output:    []dto.ResponsesOutput{},
		},
	})

	for outputIndex := range response.Output {
		output := response.Output[outputIndex]
		index := outputIndex
		switch output.Type {
		case "message":
			inProgress := output
			inProgress.Status = "in_progress"
			inProgress.Content = []dto.ResponsesOutputContent{}
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemAdded, OutputIndex: &index, Item: &inProgress})
			for contentIndex := range output.Content {
				content := output.Content[contentIndex]
				ci := contentIndex
				part := &dto.ResponsesReasoningSummaryPart{Type: content.Type, Text: ""}
				sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.content_part.added", ItemID: output.ID, OutputIndex: &index, ContentIndex: &ci, Part: part})
				if content.Text != "" {
					sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.output_text.delta", ItemID: output.ID, OutputIndex: &index, ContentIndex: &ci, Delta: content.Text})
					sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.output_text.done", ItemID: output.ID, OutputIndex: &index, ContentIndex: &ci, Delta: content.Text})
				}
				sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.content_part.done", ItemID: output.ID, OutputIndex: &index, ContentIndex: &ci, Part: &dto.ResponsesReasoningSummaryPart{Type: content.Type, Text: content.Text}})
			}
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemDone, OutputIndex: &index, Item: &output})
		case "function_call":
			inProgress := output
			inProgress.Status = "in_progress"
			inProgress.Arguments = rawString("")
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemAdded, OutputIndex: &index, Item: &inProgress})
			arguments := output.ArgumentsString()
			if arguments != "" {
				sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.function_call_arguments.delta", ItemID: output.ID, OutputIndex: &index, Delta: arguments})
			}
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.function_call_arguments.done", ItemID: output.ID, OutputIndex: &index, Delta: arguments})
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemDone, OutputIndex: &index, Item: &output})
		case dto.ResponsesOutputTypeImageGenerationCall:
			inProgress := output
			inProgress.Status = "in_progress"
			inProgress.Result = ""
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemAdded, OutputIndex: &index, Item: &inProgress})
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.image_generation_call.in_progress", OutputIndex: &index, ItemID: output.ID})
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.image_generation_call.generating", OutputIndex: &index, ItemID: output.ID})
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.image_generation_call.completed", OutputIndex: &index, ItemID: output.ID})
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemDone, OutputIndex: &index, Item: &output})
		default:
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemAdded, OutputIndex: &index, Item: &output})
			sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: dto.ResponsesOutputTypeItemDone, OutputIndex: &index, Item: &output})
		}
	}
	sendResponsesBridgeEvent(c, dto.ResponsesStreamResponse{Type: "response.completed", Response: response})
	helper.Done(c)
}

func sendResponsesBridgeEvent(c *gin.Context, event dto.ResponsesStreamResponse) {
	data, err := common.Marshal(event)
	if err != nil {
		logger.LogError(c, "marshal responses image bridge event failed: "+err.Error())
		return
	}
	helper.ResponseChunkData(c, event, string(data))
}

func newResponsesInternalContext(
	parent *gin.Context,
	method string,
	path string,
	body []byte,
	copyAllKeys bool,
) (*gin.Context, *httptest.ResponseRecorder, func(), error) {
	recorder := httptest.NewRecorder()
	child, _ := gin.CreateTestContext(recorder)

	if parent != nil {
		if copyAllKeys {
			snapshot := parent.Copy()
			for key, value := range snapshot.Keys {
				child.Set(key, value)
			}
		} else {
			copyResponsesInternalContextKeys(parent, child)
		}
	}

	var request *http.Request
	if parent != nil && parent.Request != nil {
		request = parent.Request.Clone(parent.Request.Context())
		request.Header = parent.Request.Header.Clone()
	} else {
		request, _ = http.NewRequest(method, path, nil)
	}
	request.Method = method
	request.URL.Path = path
	request.URL.RawPath = ""
	request.URL.RawQuery = ""
	request.RequestURI = path
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	request.Header.Set("Content-Type", "application/json")
	child.Request = request

	storage, err := common.CreateBodyStorage(body)
	if err != nil {
		return nil, nil, func() {}, err
	}
	child.Set(common.KeyBodyStorage, storage)
	cleanup := func() {
		common.CleanupBodyStorage(child)
	}
	return child, recorder, cleanup, nil
}

func copyResponsesInternalContextKeys(parent, child *gin.Context) {
	keys := []string{
		string(constant.ContextKeyTokenUnlimited),
		string(constant.ContextKeyTokenKey),
		string(constant.ContextKeyTokenId),
		string(constant.ContextKeyTokenGroup),
		string(constant.ContextKeyTokenCrossGroupRetry),
		string(constant.ContextKeyUserId),
		string(constant.ContextKeyUserSetting),
		string(constant.ContextKeyUserQuota),
		string(constant.ContextKeyUserStatus),
		string(constant.ContextKeyUserEmail),
		string(constant.ContextKeyUserGroup),
		string(constant.ContextKeyUsingGroup),
		string(constant.ContextKeyUserName),
		string(constant.ContextKeyLanguage),
		string(constant.ContextKeyLocalCountTokens),
		"token_name",
	}
	for _, key := range keys {
		if value, exists := parent.Get(key); exists {
			child.Set(key, value)
		}
	}
	if value, exists := parent.Get(common.RequestIdKey); exists {
		child.Set(common.RequestIdKey, value)
	}
}

func interfaceIntPointer(value any) *int {
	switch typed := value.(type) {
	case int:
		return common.GetPointer(typed)
	case int64:
		converted := int(typed)
		return &converted
	case float64:
		converted := int(typed)
		return &converted
	default:
		return nil
	}
}

func rawString(value string) []byte {
	if value == "" {
		return nil
	}
	raw, _ := common.Marshal(value)
	return raw
}

func rawIntPointer(value *int) []byte {
	if value == nil {
		return nil
	}
	raw, _ := common.Marshal(*value)
	return raw
}
