package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

// syncUSDExchangeRateToCurrencyConfig 将美元人民币汇率同步到 currency_stripe_config 表（CNY 行）。
// 仅做数据同步，不写 HTTP 响应，调用方负责返回结果。
func syncUSDExchangeRateToCurrencyConfig(value string) error {
	price, err := strconv.ParseFloat(value, 64)
	if err != nil || price <= 0 {
		return fmt.Errorf("美元人民币汇率必须大于 0")
	}
	existing, err := model.GetCurrencyConfig("CNY")
	if err != nil {
		existing = &model.CurrencyStripeConfig{
			Currency: "CNY",
			Symbol:   "¥",
		}
	}
	existing.UnitPrice = price
	return model.UpdateCurrencyConfig(existing)
}

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		value := common.Interface2String(v)
		if strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "Password") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "password") ||
			strings.HasSuffix(k, "api_key") {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
	return
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	switch option.Value.(type) {
	case bool:
		option.Value = common.Interface2String(option.Value.(bool))
	case float64:
		option.Value = common.Interface2String(option.Value.(float64))
	case int:
		option.Value = common.Interface2String(option.Value.(int))
	default:
		option.Value = fmt.Sprintf("%v", option.Value)
	}
	switch option.Key {
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！",
			})
			return
		}
	case "discord.enabled":
		if option.Value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！",
			})
			return
		}
	case "oidc.enabled":
		if option.Value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！",
			})
			return
		}
	case "LinuxDOOAuthEnabled":
		if option.Value == "true" && common.LinuxDOClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！",
			})
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(common.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用邮箱域名限制，请先填入限制的邮箱域名！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})

			return
		}
	case "TelegramOAuthEnabled":
		if option.Value == "true" && common.TelegramBotToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Telegram OAuth，请先填入 Telegram Bot Token！",
			})
			return
		}
	case "GroupRatio":
		err = ratio_setting.CheckGroupRatio(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "图片倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频补全倍率设置失败: " + err.Error(),
			})
			return
		}
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "缓存创建倍率设置失败: " + err.Error(),
			})
			return
		}
	case "ModelRequestRateLimitGroup":
		err = setting.CheckModelRequestRateLimitGroup(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticDisableStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticRetryStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "InviteTopupRebateRatio", "InviteConsumeRebateRatioLevel2":
		var ratio float64
		ratio, err = strconv.ParseFloat(option.Value.(string), 64)
		if err != nil || ratio < 0 || ratio > 100 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "消费返利比例必须在 0 到 100 之间",
			})
			return
		}
	case "USDExchangeRate":
		if err := syncUSDExchangeRateToCurrencyConfig(option.Value.(string)); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.api_info":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "ApiInfo")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.announcements":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.faq":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "FAQ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.uptime_kuma_groups":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "UptimeKumaGroups")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	err = model.UpdateOption(option.Key, option.Value.(string))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

// UploadWebLogo 上传网站logo
func UploadWebLogo(c *gin.Context) {
	// 校验文件是否存在
	file, err := c.FormFile("logo")
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgWebLogoNotSelected)
		return
	}

	// 校验文件大小（比如限制 5MB）
	if file.Size > 5<<20 {
		common.ApiErrorI18n(c, i18n.MsgWebLogoSizeExceeded)
		return
	}

	// 校验文件类型(JPG、PNG、GIF、SVG)
	contentType := file.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" && contentType != "image/svg+xml" {
		common.ApiErrorI18n(c, i18n.MsgWebLogoFormatUnsupported)
		return
	}

	// 打开文件内容
	src, err := file.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer src.Close()

	// 路径: static/logo
	baseDir := filepath.Join("static", "logo")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		common.ApiError(c, err)
		return
	}

	// 创建SHA256哈希计算器，用于生成文件唯一标识
	hasher := sha256.New()

	// 创建临时目录，用于暂存文件
	// 路径: static/logo/tmp
	tmpDir := filepath.Join(baseDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0755) // 忽略创建错误，可能已存在

	// 获取文件扩展名
	ext := strings.ToLower(filepath.Ext(file.Filename))

	// 构建临时文件路径（使用随机文件名）,防止上传文件名重复
	// 路径: static/logo/tmp/xxxxxx.jpg
	tmpPath := filepath.Join(tmpDir, uuid.New().String()+ext)

	// 创建临时文件
	dst, err := os.Create(tmpPath)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 手动控制关闭时机：将文件内容同时复制到临时文件和哈希计算器
	// 使用io.MultiWriter实现一次读取，同时写入两个目标
	_, err = io.Copy(io.MultiWriter(dst, hasher), src)
	if err != nil {
		dst.Close() // 出错时立即关闭目标文件
		common.ApiError(c, err)
	}

	// 先关闭目标文件，确保数据完全写入
	if err := dst.Close(); err != nil {
		common.ApiError(c, err)
		return
	}

	// 生成文件哈希值（十六进制字符串）
	hash := hex.EncodeToString(hasher.Sum(nil))

	// 构建最终文件路径（使用哈希值+原始扩展名）
	// 路径: static/logo/xxxxxx.jpg
	finalPath := filepath.Join(baseDir, hash+ext)

	// 将临时文件移动到最终位置（原子操作，比复制更高效）
	if err := os.Rename(tmpPath, finalPath); err != nil {
		common.ApiError(c, err)
		return
	}

	// 返回成功响应
	logoURL := "/static/logo/" + hash + ext
	common.ApiSuccess(c, gin.H{"url": logoURL})
}

func saveUploadedLogo(c *gin.Context) (string, bool) {
	file, err := c.FormFile("logo")
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgWebLogoNotSelected)
		return "", false
	}
	if file.Size > 5<<20 {
		common.ApiErrorI18n(c, i18n.MsgWebLogoSizeExceeded)
		return "", false
	}
	contentType := file.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" && contentType != "image/svg+xml" {
		common.ApiErrorI18n(c, i18n.MsgWebLogoFormatUnsupported)
		return "", false
	}

	src, err := file.Open()
	if err != nil {
		common.ApiError(c, err)
		return "", false
	}
	defer src.Close()

	baseDir := filepath.Join("static", "logo")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		common.ApiError(c, err)
		return "", false
	}
	tmpDir := filepath.Join(baseDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		common.ApiError(c, err)
		return "", false
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	tmpPath := filepath.Join(tmpDir, uuid.New().String()+ext)
	dst, err := os.Create(tmpPath)
	if err != nil {
		common.ApiError(c, err)
		return "", false
	}

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(dst, hasher), src); err != nil {
		_ = dst.Close()
		common.ApiError(c, err)
		return "", false
	}
	if err := dst.Close(); err != nil {
		common.ApiError(c, err)
		return "", false
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	finalPath := filepath.Join(baseDir, hash+ext)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		common.ApiError(c, err)
		return "", false
	}
	return "/static/logo/" + hash + ext, true
}

// GetCryptoChainConfig 获取加密货币链配置
func GetCryptoChainConfig(c *gin.Context) {
	// 定义链配置列表
	var cfgs []model.CryptoChainConfig

	// 查询所有链配置
	cfgs, err := model.GetCryptoChainConfigList()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cfgs)
}

// UpdateCryptoChainConfig 更新加密货币链配置
func UpdateCryptoChainConfig(c *gin.Context) {
	// 从请求中接收 CryptoChainConfig 数组
	var req struct {
		Crypto []model.CryptoChainConfig `json:"crypto"`
	}

	// 校验请求参数是否为空
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	// 规范化和默认值：token_symbol 为空时默认 USDT
	// 该字段是为了后续每个网络对应多个加密货币的币种设计的, 目前只有 USDT, 所以默认 USDT
	// 如果后续添加了多币种, 则是: {"network": "Sepolia", "token_symbol": "USDT"}, {"network": "Sepolia", "token_symbol": "ETH"}
	for i := range req.Crypto {
		if strings.TrimSpace(req.Crypto[i].TokenSymbol) == "" {
			req.Crypto[i].TokenSymbol = "USDT"
		}
	}

	// 更新数据库配置
	if err := model.UpdateCryptoChainConfigList(req.Crypto); err != nil {
		common.ApiError(c, err)
		return
	}

	// 返回成功响应
	common.ApiSuccess(c, nil)
}

// GetCryptoRate 获取加密货币汇率
func GetCryptoRate(c *gin.Context) {
	usdToToken := getCryptoUSDtoTokenRate()
	cnyToToken := getCryptoCNYtoTokenRate()
	common.ApiSuccess(c, gin.H{
		"usd_to_token_rate": usdToToken,
		"cny_to_token_rate": cnyToToken,
	})
}

// updateCryptoRateRequest 更新加密货币汇率请求
type updateCryptoRateRequest struct {
	USDtoTokenRate string `json:"usd_to_token_rate"`
	CNYtoTokenRate string `json:"cny_to_token_rate"`
}

// UpdateCryptoRate 更新加密货币汇率
func UpdateCryptoRate(c *gin.Context) {
	var req updateCryptoRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	// 更新 USD→Token 汇率
	if strings.TrimSpace(req.USDtoTokenRate) != "" {
		if _, err := decimal.NewFromString(req.USDtoTokenRate); err != nil {
			common.ApiErrorMsg(c, "USD 汇率格式错误")
			return
		}
		if err := model.UpdateOption("CryptoUSDtoTokenRate", req.USDtoTokenRate); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	// 更新 CNY→Token 汇率
	if strings.TrimSpace(req.CNYtoTokenRate) != "" {
		if _, err := decimal.NewFromString(req.CNYtoTokenRate); err != nil {
			common.ApiErrorMsg(c, "CNY 汇率格式错误")
			return
		}
		if err := model.UpdateOption("CryptoCNYtoTokenRate", req.CNYtoTokenRate); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	common.ApiSuccess(c, nil)
}

// setWebColorsRequest 设置网站主色和辅色请求
type setWebColorsRequest struct {
	PrimaryColor    string `json:"primary_color"`     // 主色
	SecondaryColor  string `json:"secondary_color"`   // 辅色
	ButtonTextColor string `json:"button_text_color"` // 按钮文本颜色
}

// SetWebColors 设置网站主色和辅色
func SetWebColors(c *gin.Context) {
	var req setWebColorsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	// 更新主色
	if strings.TrimSpace(req.PrimaryColor) != "" {
		if err := model.UpdateOption("WebPrimaryColor", strings.TrimSpace(req.PrimaryColor)); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	// 更新辅色
	if strings.TrimSpace(req.SecondaryColor) != "" {
		if err := model.UpdateOption("WebSecondaryColor", strings.TrimSpace(req.SecondaryColor)); err != nil {
			common.ApiError(c, err)
			return
		}
	}

	// 更新按钮文本颜色
	if strings.TrimSpace(req.ButtonTextColor) != "" {
		if err := model.UpdateOption("WebButtonTextColor", strings.TrimSpace(req.ButtonTextColor)); err != nil {
			common.ApiError(c, err)
			return
		}
	}

	common.ApiSuccess(c, nil)
}

// GetWebColors 获取网站主题色（无需登录）
func GetWebColors(c *gin.Context) {
	primary := ""         // 主色
	secondary := ""       // 辅色
	buttonTextColor := "" // 按钮文本颜色
	common.OptionMapRWMutex.RLock()
	if v, ok := common.OptionMap["WebPrimaryColor"]; ok {
		primary = common.Interface2String(v)
	}
	if v, ok := common.OptionMap["WebSecondaryColor"]; ok {
		secondary = common.Interface2String(v)
	}
	if v, ok := common.OptionMap["WebButtonTextColor"]; ok {
		buttonTextColor = common.Interface2String(v)
	}
	common.OptionMapRWMutex.RUnlock()
	common.ApiSuccess(c, gin.H{
		"primary_color":     primary,
		"secondary_color":   secondary,
		"button_text_color": buttonTextColor,
	})
}

// UploadWechatCustomerQrcode 上传微信客服二维码
func UploadWechatCustomerQrcode(c *gin.Context) {
	userId := c.GetInt("id") // 用户ID

	// 校验文件是否存在
	file, err := c.FormFile("wechat_qrcode")
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgWebLogoNotSelected)
		return
	}

	// 校验文件大小（比如限制 5MB）
	if file.Size > 5<<20 {
		common.ApiErrorI18n(c, i18n.MsgWechatCustomerQrcodeSizeExceeded)
		return
	}

	// 校验文件类型(JPG、PNG、GIF、SVG)
	contentType := file.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" && contentType != "image/svg+xml" {
		common.ApiErrorI18n(c, i18n.MsgWechatCustomerQrcodeFormatUnsupported)
		return
	}

	// 打开文件内容
	src, err := file.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer src.Close()

	// 路径: static/wechatQrcode/用户ID
	baseDir := filepath.Join("static", "wechatQrcode", strconv.Itoa(userId))
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		common.ApiError(c, err)
		return
	}

	// 创建SHA256哈希计算器，用于生成文件唯一标识
	hasher := sha256.New()

	// 创建临时目录，用于暂存文件
	// 路径: static/wechatQrcode/用户ID/tmp
	tmpDir := filepath.Join(baseDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0755) // 忽略创建错误，可能已存在

	// 获取文件扩展名
	ext := strings.ToLower(filepath.Ext(file.Filename))

	// 构建临时文件路径（使用随机文件名）,防止上传文件名重复
	// 路径: static/wechatQrcode/用户ID/tmp/xxxxxx.jpg
	tmpPath := filepath.Join(tmpDir, uuid.New().String()+ext)

	// 创建临时文件
	dst, err := os.Create(tmpPath)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 手动控制关闭时机：将文件内容同时复制到临时文件和哈希计算器
	// 使用io.MultiWriter实现一次读取，同时写入两个目标
	_, err = io.Copy(io.MultiWriter(dst, hasher), src)
	if err != nil {
		dst.Close() // 出错时立即关闭目标文件
		common.ApiError(c, err)
	}

	// 先关闭目标文件，确保数据完全写入
	if err := dst.Close(); err != nil {
		common.ApiError(c, err)
		return
	}

	// 生成文件哈希值（十六进制字符串）
	hash := hex.EncodeToString(hasher.Sum(nil))

	// 构建最终文件路径（使用哈希值+原始扩展名）
	// 路径: static/avatar/用户ID/xxxxxx.jpg
	finalPath := filepath.Join(baseDir, hash+ext)

	// 将临时文件移动到最终位置（原子操作，比复制更高效）
	if err := os.Rename(tmpPath, finalPath); err != nil {
		common.ApiError(c, err)
		return
	}

	// 返回成功响应
	qrcodeURL := "/static/wechatQrcode/" + strconv.Itoa(userId) + "/" + hash + ext
	common.ApiSuccess(c, gin.H{"url": qrcodeURL})
}

// setWebSupportRequest 设置网站客服请求
type setWebSupportRequest struct {
	WechatSupport string `json:"wechat_support"` // 微信客服
	QQSupport     string `json:"qq_support"`     // QQ客服
}

// SetWebSupport 设置网站客服
func SetWebSupport(c *gin.Context) {
	var req setWebSupportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	// 更新微信客服
	if err := model.UpdateOption("WechatSupport", strings.TrimSpace(req.WechatSupport)); err != nil {
		common.ApiError(c, err)
		return
	}
	// 更新QQ客服
	if err := model.UpdateOption("QQSupport", strings.TrimSpace(req.QQSupport)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type addVersionLogRequest struct {
	Version string `json:"version"` // 版本号
	Log     string `json:"log"`     // 日志
}

// AddVersionLog 添加版本日志
func AddVersionLog(c *gin.Context) {
	// 接收前端参数:"版本号"和"版本日志"
	var req addVersionLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	// 参数校验
	if req.Version == "" || req.Log == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	// 提取版本号
	version := req.Version

	// 事务写入: 更新版本号 + 插入更新日志
	tx := model.DB.Begin()
	if tx.Error != nil {
		common.ApiError(c, tx.Error)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新options表中的版本号
	option := model.Option{Key: "Version"}
	tx.FirstOrCreate(&option, model.Option{Key: "Version"})
	option.Value = version
	if err := tx.Save(&option).Error; err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}

	// 插入版本日志
	versionLog, err := model.InsertVersionLog(tx, version, req.Log)
	if err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		common.ApiError(c, err)
		return
	}

	// 更新内存中的OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap["Version"] = version
	common.OptionMapRWMutex.Unlock()

	// 更新缓存中的最新版本日志
	if err := model.SetLatestVersionLogCache(versionLog); err != nil {
		common.SysLog("failed to update latest version log cache: " + err.Error())
	}

	common.ApiSuccess(c, nil)
}

// GetAllVersionLog 获取所有版本日志
func GetAllVersionLog(c *gin.Context) {
	// 从数据库查询所有版本日志
	logs, err := model.GetAllVersionLogs()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, logs)
}

// GetVersionLogById 获取版本日志详情
func GetVersionLogById(c *gin.Context) {
	// 从URL参数中获取ID
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 从数据库查询版本日志
	log, err := model.GetVersionLogById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 返回版本日志
	common.ApiSuccess(c, log)
}

// UpdateVersionLogById 更新版本日志详情
func UpdateVersionLogById(c *gin.Context) {
	// 从URL参数中获取ID
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 接收前端参数
	var req addVersionLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, i18n.T(c, i18n.MsgInvalidParams))
		return
	}
	if req.Version == "" || req.Log == "" {
		common.ApiErrorMsg(c, i18n.T(c, i18n.MsgInvalidParams))
		return
	}

	// 事务: 查询 + 更新
	tx := model.DB.Begin()
	if tx.Error != nil {
		common.ApiError(c, tx.Error)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 从数据库查询版本日志
	var log model.VersionLog
	if err := tx.Where("id = ?", id).First(&log).Error; err != nil {
		tx.Rollback()
		common.ApiErrorMsg(c, i18n.T(c, i18n.MsgVersionLogNotFound))
		return
	}

	// 更新版本日志
	log.Version = req.Version
	log.Log = req.Log
	log.UpdatedAt = common.GetTimestamp()
	if err := tx.Model(&log).Updates(map[string]interface{}{
		"version":    log.Version,
		"log":        log.Log,
		"updated_at": log.UpdatedAt,
	}).Error; err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}

	if err := tx.Commit().Error; err != nil {
		common.ApiError(c, err)
		return
	}

	// 更新缓存中的最新版本日志
	if err := model.RefreshLatestVersionLogCacheFromDB(); err != nil {
		common.SysLog("failed to refresh latest version log cache: " + err.Error())
	}
	common.ApiSuccess(c, nil)
}

// DeleteVersionLogById 删除版本日志
func DeleteVersionLogById(c *gin.Context) {
	// 获取URL参数中的ID
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 事务: 查询 + 删除
	tx := model.DB.Begin()
	if tx.Error != nil {
		common.ApiError(c, tx.Error)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 先检查是否存在
	var log model.VersionLog
	if err := tx.Where("id = ?", id).First(&log).Error; err != nil {
		tx.Rollback()
		common.ApiErrorMsg(c, i18n.T(c, i18n.MsgVersionLogNotFound))
		return
	}

	// 删除版本日志
	if err := model.DeleteVersionLogById(tx, id); err != nil {
		tx.Rollback()
		common.ApiError(c, err)
		return
	}

	if err := tx.Commit().Error; err != nil {
		common.ApiError(c, err)
		return
	}

	// 更新缓存中的最新版本日志
	if err := model.RefreshLatestVersionLogCacheFromDB(); err != nil {
		common.SysLog("failed to refresh latest version log cache: " + err.Error())
	}
	common.ApiSuccess(c, nil)
}
