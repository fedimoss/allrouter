package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 问卷附件上传限制：问题截图 5MB、合作申请文档资料 10MB
const (
	questionnaireImageMaxSize = 5 << 20
	questionnaireFileMaxSize  = 10 << 20
)

// QuestionnaireUploadBodyLimit 上传请求体大小上限（10MB 文件 + multipart 边界开销余量）。
// 供路由层在 multipart 解析前直接拒绝超大 body——gin 的 FormFile 会先读完整个请求体
// （超 32MB 溢写临时文件）才轮到控制器的大小检查，提前截断可避免白白消耗带宽/磁盘。
const QuestionnaireUploadBodyLimit = questionnaireFileMaxSize + 1<<20

// questionnaireUploadSpec 单一扩展名的校验规则：大小上限 + 首块魔数嗅探。
// 白名单不含 SVG/HTML：匿名可传的此类文件经 /static 同源直出构成存储型 XSS 通道。
type questionnaireUploadSpec struct {
	maxSize int64
	sniff   func(head []byte) bool
}

// sniffMime 用 http.DetectContentType 精确匹配（图片类）
func sniffMime(mimeType string) func([]byte) bool {
	return func(head []byte) bool {
		return http.DetectContentType(head) == mimeType
	}
}

// sniffPrefix 二进制签名前缀匹配
func sniffPrefix(prefix string) func([]byte) bool {
	return func(head []byte) bool {
		return bytes.HasPrefix(head, []byte(prefix))
	}
}

var (
	// questionnaireSniffOLE OLE2 复合文档签名（老版 Office：doc/xls/ppt）
	questionnaireSniffOLE = sniffPrefix("\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1")
	// questionnaireSniffZIP ZIP 容器签名（docx/xlsx/pptx/zip）
	questionnaireSniffZIP = sniffPrefix("PK\x03\x04")
	// questionnaireSniffText 文本类（txt/md/csv/json）：首块不含 NUL 字节视为文本
	questionnaireSniffText = func(head []byte) bool {
		return bytes.IndexByte(head, 0) < 0
	}
)

// questionnaireUploadSpecs 扩展名白名单。multipart 头 Content-Type 可伪造，
// 一律以文件名扩展名路由，再嗅探首块内容确保与声明的格式相符，防止任意文件伪装落盘。
var questionnaireUploadSpecs = map[string]questionnaireUploadSpec{
	".jpg":  {questionnaireImageMaxSize, sniffMime("image/jpeg")},
	".jpeg": {questionnaireImageMaxSize, sniffMime("image/jpeg")},
	".png":  {questionnaireImageMaxSize, sniffMime("image/png")},
	".gif":  {questionnaireImageMaxSize, sniffMime("image/gif")},
	".webp": {questionnaireImageMaxSize, sniffMime("image/webp")},
	".pdf":  {questionnaireFileMaxSize, sniffPrefix("%PDF-")},
	".doc":  {questionnaireFileMaxSize, questionnaireSniffOLE},
	".xls":  {questionnaireFileMaxSize, questionnaireSniffOLE},
	".ppt":  {questionnaireFileMaxSize, questionnaireSniffOLE},
	".docx": {questionnaireFileMaxSize, questionnaireSniffZIP},
	".xlsx": {questionnaireFileMaxSize, questionnaireSniffZIP},
	".pptx": {questionnaireFileMaxSize, questionnaireSniffZIP},
	".zip":  {questionnaireFileMaxSize, questionnaireSniffZIP},
	".txt":  {questionnaireFileMaxSize, questionnaireSniffText},
	".md":   {questionnaireFileMaxSize, questionnaireSniffText},
	".csv":  {questionnaireFileMaxSize, questionnaireSniffText},
	".json": {questionnaireFileMaxSize, questionnaireSniffText},
}

// UploadQuestionnaireFile 上传问卷附件（问题截图 / 合作申请资料）
// 接口：POST /api/user/questionnaire/upload（公开，登录可选）
// 表单字段：file（合作申请资料）或 image（/userQuestion 截图，兼容保留）
// 存储路径：static/questionnaire/{provider_id}/{user_id}/{sha256}{ext}
// 站点归属与问卷提交一致（域名优先，主站未登录归 0）；匿名用户 user_id=0；
// 文件名用内容哈希：天然去重、无命名冲突。URL 由前端写入 survey_data
// （反馈问卷 screenshots 数组 / 合作申请 files 数组）。
func UploadQuestionnaireFile(c *gin.Context) {
	userId := c.GetInt("id") // 未登录为 0

	file, err := c.FormFile("file")
	if err != nil {
		// 兼容 /userQuestion 截图上传的旧字段名 image
		file, err = c.FormFile("image")
	}
	if err != nil {
		common.ApiErrorMsg(c, "请选择文件")
		return
	}
	spec, ok := questionnaireUploadSpecs[strings.ToLower(filepath.Ext(file.Filename))]
	if !ok {
		common.ApiErrorMsg(c, "不支持的文件类型")
		return
	}
	if file.Size > spec.maxSize {
		common.ApiErrorMsg(c, fmt.Sprintf("文件大小不能超过 %dMB", spec.maxSize>>20))
		return
	}

	// 站点归属：与 SubmitUserQuestionnaire 同一解析逻辑，前端传 domain 保持一致
	providerId, err := resolveQuestionnaireProviderId(c, c.PostForm("domain"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	// 路径: static/questionnaire/站点ID/用户ID
	baseDir := filepath.Join("static", "questionnaire",
		strconv.Itoa(providerId), strconv.Itoa(userId))
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		common.ApiError(c, err)
		return
	}

	src, err := file.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer src.Close()

	// 魔数校验：multipart 头由客户端声明、可伪造，嗅探首块内容确保确实是声明的格式，
	// 防止任意文件伪装成图片/文档落盘
	head := make([]byte, 512)
	headLen, err := io.ReadFull(src, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		common.ApiError(c, err)
		return
	}
	if !spec.sniff(head[:headLen]) {
		common.ApiErrorMsg(c, "文件内容与格式不符")
		return
	}

	// 临时文件落盘 + 同步计算内容哈希（与客服二维码上传同一模式）
	hasher := sha256.New()
	tmpDir := filepath.Join(baseDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0755) // 可能已存在
	tmpPath := filepath.Join(tmpDir, uuid.New().String()+filepath.Ext(file.Filename))
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
	finalPath := filepath.Join(baseDir, hash+strings.ToLower(filepath.Ext(file.Filename)))
	// 内容哈希相同即同一文件。重复上传（例如用户移除后又选回同一文件）
	// 应幂等复用已有文件；Windows 上 os.Rename 不能覆盖已有目标，必须显式判断。
	if _, statErr := os.Stat(finalPath); statErr == nil {
		cleanupTmpFile(nil, tmpPath)
	} else if errors.Is(statErr, os.ErrNotExist) {
		if err := os.Rename(tmpPath, finalPath); err != nil {
			cleanupTmpFile(nil, tmpPath)
			common.ApiError(c, err)
			return
		}
	} else {
		cleanupTmpFile(nil, tmpPath)
		common.ApiError(c, statErr)
		return
	}

	fileURL := "/static/questionnaire/" + strconv.Itoa(providerId) + "/" +
		strconv.Itoa(userId) + "/" + hash + strings.ToLower(filepath.Ext(file.Filename))
	common.ApiSuccess(c, gin.H{"url": fileURL})
}

// cleanupTmpFile 落盘/重命名失败时关闭并移除临时文件，避免 tmp 目录被失败残留填满
func cleanupTmpFile(dst *os.File, tmpPath string) {
	if dst != nil {
		_ = dst.Close()
	}
	_ = os.Remove(tmpPath)
}
