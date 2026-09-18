package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 抖音私信卡片管理：数据保存在外部服务（系统设置中的"基础URL"+"增删改查接口"），
// 本服务仅作管理端代理转发，不在本地建表。转发请求统一携带
// `Authorization: Bearer <ApiKey>`（系统设置中的 API Key）。

// douyinCardProxyTimeout 单次转发到外部服务的超时时间
const douyinCardProxyTimeout = 30 * time.Second

// douyinCardTargetURL 拼接"基础URL + 接口路径"并做 SSRF 校验。
// baseURL 为空表示管理员尚未配置；接口路径以 http(s) 开头时视为完整地址直接使用。
func douyinCardTargetURL(endpoint system_setting.DouyinCardEndpoint) (string, error) {
	s := system_setting.GetDouyinCardSettings()
	path := strings.TrimSpace(endpoint.URL)
	if path == "" {
		return "", fmt.Errorf("接口地址未配置，请先在系统设置中填写抖音私信卡片接口")
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}
	base := strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
	if base == "" {
		return "", fmt.Errorf("基础URL未配置，请先在系统设置中填写抖音私信卡片基础URL")
	}
	target := base + "/" + strings.TrimLeft(path, "/")

	fetchSetting := system_setting.GetFetchSetting()
	if err := common.ValidateURLWithFetchSetting(target, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return "", fmt.Errorf("目标地址被 SSRF 防护拦截: %v", err)
	}
	return target, nil
}

// douyinCardTransformBody 添加/修改转发前对 JSON 请求体做统一加工（单次解析）：
//   - 图片地址（cover/wxAvatar/wxQr）：统一用系统设置「通用设置 → 服务器地址」拼接为
//     绝对 URL——上传接口返回的是本站相对路径（/static/card/...），外部服务/Douyin 侧
//     访问不到管理端浏览器所在的主机（如 http://127.0.0.1:5173），也不能用相对路径；
//     已是绝对地址的值（外部服务返回的旧数据）原样保留；
//   - overrideLink：把卡片跳转链接（link 字段）统一替换为系统设置
//     「抖音私信卡片 → 微信小程序页面地址」的值，卡片表单不再逐卡片填写；
//     地址未配置时不动该字段（新建缺省、编辑保留记录原值）；
//   - fillAudit：补上审计字段——createdAt/updatedAt 为秒级时间戳、
//     createdBy/updatedBy 为当前登录用户 ID（字段名与格式和列表接口返回一致，
//     外部服务按调用方传入的值记录）。
//
// 请求体缺失或不是 JSON 对象时原样透传，不因此阻断转发。
func douyinCardTransformBody(body io.Reader, userId int, overrideLink, fillAudit bool) io.Reader {
	raw, err := io.ReadAll(body)
	if err != nil || len(raw) == 0 {
		return bytes.NewReader(raw)
	}
	var payload map[string]any
	if err := common.Unmarshal(raw, &payload); err != nil || payload == nil {
		return bytes.NewReader(raw)
	}
	changed := false
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	for _, key := range []string{"cover", "wxAvatar", "wxQr"} {
		v, ok := payload[key].(string)
		if !ok || v == "" || base == "" {
			continue
		}
		lower := strings.ToLower(v)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			continue
		}
		payload[key] = base + "/" + strings.TrimLeft(v, "/")
		changed = true
	}
	if overrideLink {
		if link := strings.TrimSpace(system_setting.GetDouyinCardSettings().WxAppPageURL); link != "" {
			payload["link"] = link
			changed = true
		}
	}
	if fillAudit && userId > 0 {
		now := time.Now().Unix()
		payload["createdAt"] = now
		payload["updatedAt"] = now
		payload["createdBy"] = strconv.Itoa(userId)
		payload["updatedBy"] = strconv.Itoa(userId)
		changed = true
	}
	if !changed {
		return bytes.NewReader(raw)
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return bytes.NewReader(raw)
	}
	return bytes.NewReader(data)
}

// douyinCardProxy 统一转发入口：
//   - method 为空时使用接口配置中的 method（默认 POST/GET/PUT/DELETE）；
//   - GET/DELETE 将查询参数原样透传给外部服务；
//   - POST/PUT/PATCH 将请求体原样透传；overrideLink/fillAudit 为 true 时（添加/修改），
//     先对请求体做统一加工（注入跳转链接/审计字段，见 douyinCardTransformBody）再转发；
//   - 外部服务的 JSON 响应体原样返回给前端（success/data/message 结构由外部服务定义）。
func douyinCardProxy(c *gin.Context, endpoint system_setting.DouyinCardEndpoint, method string, overrideLink, fillAudit bool) {
	target, err := douyinCardTargetURL(endpoint)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if method == "" {
		method = endpoint.Method
	}

	s := system_setting.GetDouyinCardSettings()

	var body io.Reader
	switch method {
	case http.MethodGet, http.MethodDelete:
		body = nil
		// 透传查询参数（分页、关键词等由外部接口自行解析）
		if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
			if idx := strings.Index(target, "?"); idx >= 0 {
				target = target + "&" + rawQuery
			} else {
				target = target + "?" + rawQuery
			}
		}
	default:
		body = c.Request.Body
		if overrideLink || fillAudit {
			body = douyinCardTransformBody(body, c.GetInt("id"), overrideLink, fillAudit)
		}
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), method, target, body)
	if err != nil {
		common.ApiErrorMsg(c, "构造转发请求失败: "+err.Error())
		return
	}
	// 外部服务要求 Bearer 鉴权：Authorization: Bearer <api-key>
	req.Header.Set("Authorization", "Bearer "+s.ApiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// 透传调用方语言，便于外部服务返回对应语言的提示文案
	if acceptLanguage := c.GetHeader("Accept-Language"); acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}

	client := service.GetHttpClient()
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		common.ApiErrorMsg(c, "请求外部服务失败: "+err.Error())
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 上限 10MB，防止异常响应撑爆内存
	if err != nil {
		common.ApiErrorMsg(c, "读取外部服务响应失败: "+err.Error())
		return
	}

	// 外部服务不可达或网关错误时给出可读提示，其余状态码（含业务错误）原样透传
	if resp.StatusCode >= http.StatusInternalServerError {
		common.ApiErrorMsg(c, fmt.Sprintf("外部服务异常(HTTP %d): %s", resp.StatusCode, string(respBody)))
		return
	}
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)
}

// GetDouyinCards 查询卡片列表（查询接口）
// 查询接口以 JSON 请求体调用外部服务（系统设置中该接口的方法需配置为 POST）：
//
//	{ "keyword": "...", "page": { "pageNo": 1, "pageSize": 10 }, "userId": "..." }
//
// userId 为当前登录用户 ID；keyword/page/page_size 为本服务的查询参数，映射为上述结构。
func GetDouyinCards(c *gin.Context) {
	pageNo, _ := strconv.Atoi(c.Query("page"))
	if pageNo <= 0 {
		pageNo = 1
	}
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize <= 0 {
		pageSize = 10
	}
	body := map[string]any{
		"page": map[string]any{"pageNo": pageNo, "pageSize": pageSize},
	}
	if userId := c.GetInt("id"); userId > 0 {
		body["userId"] = strconv.Itoa(userId)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		body["keyword"] = keyword
	}
	// 构造失败时保持原请求（GET 无请求体），由外部服务报错暴露问题
	if raw, err := common.Marshal(body); err == nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	}
	douyinCardProxy(c, system_setting.GetDouyinCardSettings().Query, "", false, false)
}

// AddDouyinCard 添加卡片（POST，添加接口），转发前注入统一的跳转链接与审计字段
func AddDouyinCard(c *gin.Context) {
	douyinCardProxy(c, system_setting.GetDouyinCardSettings().Add, "", true, true)
}

// UpdateDouyinCard 修改卡片（PUT/PATCH，修改接口），转发前注入统一的跳转链接；
// 审计字段随行数据原样带回，由外部服务自行维护。
// 接口路径支持 {id} 占位符（如 /openapi/card/{id}）：配置了占位符时取请求体中的
// 卡片主键（id 字段）替换后转发，请求体原样保留。
func UpdateDouyinCard(c *gin.Context) {
	endpoint := system_setting.GetDouyinCardSettings().Update
	if strings.Contains(endpoint.URL, "{id}") {
		cardId, err := douyinCardBodyId(c)
		if err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
		endpoint.URL = strings.ReplaceAll(endpoint.URL, "{id}", url.PathEscape(cardId))
	}
	douyinCardProxy(c, endpoint, "", true, false)
}

// douyinCardBodyId 取请求体中的卡片主键（id 字段，兼容字符串/数字写法），
// 读取后把请求体重置为未读状态，供后续转发正常使用。
func douyinCardBodyId(c *gin.Context) (string, error) {
	raw, err := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("读取请求体失败: %v", err)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("缺少卡片ID，无法修改")
	}
	var payload map[string]any
	if err := common.Unmarshal(raw, &payload); err != nil || payload == nil {
		return "", fmt.Errorf("请求体不是有效的 JSON，无法取到卡片ID")
	}
	switch v := payload["id"].(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
	case float64:
		if v > 0 {
			// JSON 数字解析到 any 后是 float64，按最短写法还原（1001.0 → "1001"）
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		}
	}
	return "", fmt.Errorf("缺少卡片ID，无法修改")
}

// DeleteDouyinCard 删除卡片（DELETE，删除接口）
// 接口路径支持 {id} 占位符（如 /openapi/card/{id}）：配置了占位符时替换为真实主键后转发，
// 不再追加查询参数；未配置占位符时沿用查询参数约定——DELETE /:id 中的 :id 作为
// 查询参数 id 透传给外部服务，便于外部服务按默认 DELETE 方法（无请求体）取到待删除的主键。
func DeleteDouyinCard(c *gin.Context) {
	cardId := c.Param("id")
	endpoint := system_setting.GetDouyinCardSettings().Delete
	if strings.Contains(endpoint.URL, "{id}") {
		if cardId == "" {
			common.ApiErrorMsg(c, "缺少卡片ID，无法删除")
			return
		}
		endpoint.URL = strings.ReplaceAll(endpoint.URL, "{id}", url.PathEscape(cardId))
		douyinCardProxy(c, endpoint, "", false, false)
		return
	}
	query := c.Request.URL.RawQuery
	if cardId != "" {
		if query == "" {
			query = "id=" + cardId
		} else {
			query = query + "&id=" + cardId
		}
		c.Request.URL.RawQuery = query
	}
	douyinCardProxy(c, endpoint, "", false, false)
}

// ---------- 图片上传 ----------
// 卡片数据保存在外部服务，卡片图片保存在本站 static 目录（/static 由 gin 静态服务直出），
// 上传成功返回相对 URL，前端拼接为绝对 URL 后随卡片数据一起提交给外部接口。
// 存储路径按「用途 / 用户」分目录：static/card/{logo|weixin}/{user_id}/{sha256}{ext}
// 校验/落盘与问卷截图上传（questionnaire_upload.go）同一模式：类型白名单 + 魔数嗅探 +
// 内容哈希命名 + 临时文件原子重命名。

const douyinCardImageMaxSize = 5 << 20 // 5MB

// DouyinCardImageBodyLimit 上传请求体大小上限（5MB 文件 + multipart 边界开销余量），
// 供路由层在 multipart 解析前直接拒绝超大 body。
const DouyinCardImageBodyLimit = douyinCardImageMaxSize + 1<<20

var douyinCardImageExts = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// douyinCardImageKinds 上传用途白名单（同时也是 static/card 下的子目录名）：
// logo —— 私信卡片 Logo；weixin —— 推广链接（微信落地页）的微信头像与二维码。
// 用白名单而非直接拼接入参，避免请求方通过 type 构造出目标目录之外的路径。
var douyinCardImageKinds = map[string]bool{
	"logo":   true,
	"weixin": true,
}

// UploadDouyinCardImage 上传卡片图片（私信卡片 Logo / 推广链接微信头像、二维码）
// 接口：POST /api/douyin_card/upload（multipart 字段 image、type）
// 存储路径：static/card/{type}/{user_id}/{sha256}{ext}
func UploadDouyinCardImage(c *gin.Context) {
	kind := strings.TrimSpace(c.PostForm("type"))
	if !douyinCardImageKinds[kind] {
		common.ApiErrorMsg(c, "上传类型不正确")
		return
	}
	userId := c.GetInt("id")

	file, err := c.FormFile("image")
	if err != nil {
		common.ApiErrorMsg(c, "请选择图片")
		return
	}
	if file.Size > douyinCardImageMaxSize {
		common.ApiErrorMsg(c, "图片大小不能超过 5MB")
		return
	}

	src, err := file.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer src.Close()

	// 魔数嗅探判定真实格式，以文件内容为准，不看扩展名与 multipart 声明的类型：
	// 声明头写法五花八门（大写 image/PNG、非标准 image/jpg 等）且可伪造，
	// 搜索引擎下载的图片还常名不副实（如 OIP.png 实为 WebP/JPEG 数据），
	// 按声明校验会把这类图片误拒，这里按真实格式接收并落盘对应的扩展名。
	// DetectContentType 对非图片内容返回 text/* 或 application/octet-stream，
	// 不在白名单内自然被拒（SVG 亦按文本识别，避免把可脚本内容当图片存入 static）。
	head := make([]byte, 512)
	headLen, err := io.ReadFull(src, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		common.ApiError(c, err)
		return
	}
	ext, ok := douyinCardImageExts[http.DetectContentType(head[:headLen])]
	if !ok {
		common.ApiErrorMsg(c, "仅支持 JPG、PNG、GIF、WebP 格式图片")
		return
	}

	// 路径: static/card/{用途}/{用户ID}
	baseDir := filepath.Join("static", "card", kind, strconv.Itoa(userId))
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		common.ApiError(c, err)
		return
	}

	// 临时文件落盘 + 同步计算内容哈希
	hasher := sha256.New()
	tmpDir := filepath.Join(baseDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0755)
	tmpPath := filepath.Join(tmpDir, uuid.New().String()+ext)
	dst, err := os.Create(tmpPath)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sink := io.MultiWriter(dst, hasher)
	if _, err := sink.Write(head[:headLen]); err != nil {
		cleanupTmpFile(dst, tmpPath)
		common.ApiError(c, err)
		return
	}
	if _, err := io.Copy(sink, src); err != nil {
		cleanupTmpFile(dst, tmpPath)
		common.ApiError(c, err)
		return
	}
	if err := dst.Close(); err != nil {
		cleanupTmpFile(nil, tmpPath)
		common.ApiError(c, err)
		return
	}

	// 哈希命名 + 原子重命名到最终位置
	hash := hex.EncodeToString(hasher.Sum(nil))
	finalPath := filepath.Join(baseDir, hash+ext)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		cleanupTmpFile(nil, tmpPath)
		common.ApiError(c, err)
		return
	}

	imageURL := "/static/card/" + kind + "/" + strconv.Itoa(userId) + "/" + hash + ext
	common.ApiSuccess(c, gin.H{"url": imageURL})
}
