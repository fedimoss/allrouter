package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 问卷截图上传限制
const questionnaireImageMaxSize = 5 << 20 // 5MB

// QuestionnaireUploadBodyLimit 上传请求体大小上限（5MB 文件 + multipart 边界开销余量）。
// 供路由层在 multipart 解析前直接拒绝超大 body——gin 的 FormFile 会先读完整个请求体
// （超 32MB 溢写临时文件）才轮到控制器的大小检查，提前截断可避免白白消耗带宽/磁盘。
const QuestionnaireUploadBodyLimit = questionnaireImageMaxSize + 1<<20

// 类型白名单 → 统一扩展名（不收 SVG：匿名可传的 SVG 经 /static 同源直出构成存储型 XSS 通道）
var questionnaireImageExts = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// UploadQuestionnaireImage 上传问卷问题截图
// 接口：POST /api/user/questionnaire/upload（公开，登录可选）
// 存储路径：static/questionnaire/{provider_id}/{user_id}/{sha256}{ext}
// 站点归属与问卷提交一致（域名优先，主站未登录归 0）；匿名用户 user_id=0；
// 文件名用内容哈希：天然去重、无命名冲突。URL 由前端写入 survey_data.screenshots 数组。
func UploadQuestionnaireImage(c *gin.Context) {
	userId := c.GetInt("id") // 未登录为 0

	file, err := c.FormFile("image")
	if err != nil {
		common.ApiErrorMsg(c, "请选择图片")
		return
	}
	if file.Size > questionnaireImageMaxSize {
		common.ApiErrorMsg(c, "图片大小不能超过 5MB")
		return
	}
	contentType := file.Header.Get("Content-Type")
	ext, ok := questionnaireImageExts[contentType]
	if !ok {
		common.ApiErrorMsg(c, "仅支持 JPG、PNG、GIF、WebP 格式图片")
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

	// 魔数校验：multipart 头由客户端声明、可伪造，嗅探首块内容确保确实是声明的图片格式，
	// 防止任意文件伪装成图片落盘
	head := make([]byte, 512)
	headLen, err := io.ReadFull(src, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		common.ApiError(c, err)
		return
	}
	if http.DetectContentType(head[:headLen]) != contentType {
		common.ApiErrorMsg(c, "仅支持 JPG、PNG、GIF、WebP 格式图片")
		return
	}

	// 临时文件落盘 + 同步计算内容哈希（与客服二维码上传同一模式）
	hasher := sha256.New()
	tmpDir := filepath.Join(baseDir, "tmp")
	_ = os.MkdirAll(tmpDir, 0755) // 可能已存在
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

	imageURL := "/static/questionnaire/" + strconv.Itoa(providerId) + "/" +
		strconv.Itoa(userId) + "/" + hash + ext
	common.ApiSuccess(c, gin.H{"url": imageURL})
}

// cleanupTmpFile 落盘/重命名失败时关闭并移除临时文件，避免 tmp 目录被失败残留填满
func cleanupTmpFile(dst *os.File, tmpPath string) {
	if dst != nil {
		_ = dst.Close()
	}
	_ = os.Remove(tmpPath)
}
