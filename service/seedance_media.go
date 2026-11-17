package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const SeedanceMediaIDsContextKey = "seedance_media_ids"

// 参考素材复用 H3 的落盘目录与公网 URL 规则，任务终态后删除。
// 这些上限与前端提示必须保持一致；改动时要同步面板文案和测试。
const (
	SeedanceMediaMaxTotalBytes int64 = 512 << 20
	seedanceImageMaxBytes      int64 = 30 << 20
	seedanceVideoMaxBytes      int64 = 200 << 20
	seedanceAudioMaxBytes      int64 = 15 << 20
)

var seedanceMediaExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".mp4":  true,
	".mov":  true,
	".mp3":  true,
	".wav":  true,
}

// SaveSeedanceMediaFile stores a reference image, video or audio file in the
// same per-user directory used by MiniMax-H3 uploads. It returns an opaque ID
// that can be turned into a public URL with SeedanceMediaPublicURL.
// 先校验类型和大小，再嗅探文件头确定扩展名，最后落盘并返回随机 ID。
// 风险：不能直接信任客户端文件名，文件头嗅探优先于声明 MIME。
func SaveSeedanceMediaFile(userId int, fileHeader *multipart.FileHeader, kind string) (string, error) {
	if fileHeader == nil {
		// 没有文件头说明调用方参数错误。
		return "", errors.New("media file is required")
	}
	if userId <= 0 {
		// 用户 ID 决定存储目录，不能为空。
		return "", errors.New("user id is required")
	}
	maxBytes, ok := seedanceMaxBytesForKind(kind)
	if !ok {
		// 只允许 image/video/audio，其他类型不应进入 Seedance 流程。
		return "", fmt.Errorf("unsupported Seedance media kind: %s", kind)
	}
	if fileHeader.Size <= 0 {
		// 空文件无法作为参考素材。
		return "", errors.New("media file is empty")
	}
	if fileHeader.Size > maxBytes {
		// 单文件大小超限直接拒绝，避免先落盘再失败。
		return "", fmt.Errorf("%s file exceeds %d MB", kind, maxBytes>>20)
	}

	source, err := fileHeader.Open()
	if err != nil {
		// 文件打开失败通常意味着上传流已损坏。
		return "", fmt.Errorf("open Seedance %s file: %w", kind, err)
	}
	defer source.Close()

	header := make([]byte, 512)
	headerSize, err := io.ReadFull(source, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		// 读取文件头失败要清理，不能继续写半成品。
		return "", fmt.Errorf("read Seedance %s file: %w", kind, err)
	}
	header = header[:headerSize]

	extension, ok := seedanceMediaExtension(kind, http.DetectContentType(header), fileHeader.Header.Get("Content-Type"), fileHeader.Filename)
	if !ok {
		// 扩展名无法确定时拒绝，避免把未知文件暴露为公网素材。
		return "", fmt.Errorf("unsupported Seedance %s file type", kind)
	}

	userDir, err := miniMaxH3FrameUserDir(userId)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(userDir, 0o750); err != nil {
		// 目录创建失败通常意味着权限或磁盘问题。
		return "", fmt.Errorf("create Seedance media directory: %w", err)
	}

	mediaID, err := randomMiniMaxH3FrameID(extension)
	if err != nil {
		// 随机 ID 失败时不能退回可预测文件名。
		return "", err
	}
	mediaPath := filepath.Join(userDir, mediaID)
	destination, err := os.OpenFile(mediaPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		// 使用 O_EXCL 防止覆盖已有素材。
		return "", fmt.Errorf("create Seedance media file: %w", err)
	}
	keepFile := false
	defer func() {
		_ = destination.Close()
		// 写入失败必须删除半成品，避免残留文件被当成有效素材。
		// 不能只依赖上层清理，因为这里可能还没拿到 mediaID。
		if !keepFile {
			_ = os.Remove(mediaPath)
		}
	}()

	reader := io.MultiReader(bytes.NewReader(header), source)
	written, err := io.Copy(destination, io.LimitReader(reader, maxBytes+1))
	if err != nil {
		// 复制失败时保留 keepFile=false，由 defer 删除半成品。
		return "", fmt.Errorf("save Seedance media file: %w", err)
	}
	if written > maxBytes {
		// 限制读取上限，防止并发或伪造 Content-Length 绕过大小检查。
		return "", fmt.Errorf("%s file exceeds %d MB", kind, maxBytes>>20)
	}
	if err := destination.Close(); err != nil {
		// 关闭失败可能意味着数据未完全落盘，不能返回成功。
		return "", fmt.Errorf("close Seedance media file: %w", err)
	}
	keepFile = true
	return mediaID, nil
}

func SeedanceMediaPublicURL(publicBaseURL string, userId int, mediaID string) (string, error) {
	return MiniMaxH3MediaPublicURL(publicBaseURL, userId, mediaID)
}

func DeleteSeedanceMedia(userId int, mediaID string) error {
	// 与读取路径一致，拒绝带目录的 mediaID，避免误删其他用户文件。
	// 不能为了兼容而放宽为 filepath.Clean 后继续执行。
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" || filepath.Base(mediaID) != mediaID || strings.ContainsAny(mediaID, `/\`) {
		return errors.New("invalid Seedance media id")
	}
	if !seedanceMediaExtensions[strings.ToLower(filepath.Ext(mediaID))] {
		return errors.New("invalid Seedance media id")
	}
	userDir, err := miniMaxH3FrameUserDir(userId)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(userDir, mediaID)); err != nil && !os.IsNotExist(err) {
		// 不存在视为已清理；其他错误返回给调用方记录。
		return fmt.Errorf("delete Seedance media file: %w", err)
	}
	return nil
}

func CleanupSeedanceMedia(userId int, mediaIDs []string) {
	// 清理是 best-effort：单个文件失败不能阻断任务终态处理。
	for _, mediaID := range mediaIDs {
		_ = DeleteSeedanceMedia(userId, mediaID)
	}
}

func seedanceMaxBytesForKind(kind string) (int64, bool) {
	switch kind {
	case "image":
		return seedanceImageMaxBytes, true
	case "video":
		return seedanceVideoMaxBytes, true
	case "audio":
		return seedanceAudioMaxBytes, true
	default:
		return 0, false
	}
}

func seedanceMediaExtension(kind, detectedType, declaredType, filename string) (string, bool) {
	// 优先使用文件头嗅探结果，其次信任声明 MIME，最后才看文件名。
	// 兼容老客户端只传文件名，但不能让错误扩展名覆盖真实文件类型。
	for _, contentType := range []string{detectedType, declaredType} {
		if extension, ok := seedanceExtensionForContentType(kind, cleanMediaContentType(contentType)); ok {
			return extension, true
		}
	}
	return seedanceExtensionForFilename(kind, filename)
}

func cleanMediaContentType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
}

func seedanceExtensionForContentType(kind, contentType string) (string, bool) {
	switch kind {
	case "image":
		switch contentType {
		case "image/jpeg":
			return ".jpg", true
		case "image/png":
			return ".png", true
		case "image/webp":
			return ".webp", true
		}
	case "video":
		switch contentType {
		case "video/mp4":
			return ".mp4", true
		case "video/quicktime":
			return ".mov", true
		}
	case "audio":
		switch contentType {
		case "audio/mpeg":
			return ".mp3", true
		case "audio/wav", "audio/x-wav", "audio/wave":
			return ".wav", true
		}
	}
	return "", false
}

func seedanceExtensionForFilename(kind, filename string) (string, bool) {
	extension := strings.ToLower(filepath.Ext(filename))
	switch kind {
	case "image":
		switch extension {
		case ".jpg", ".jpeg":
			return ".jpg", true
		case ".png":
			return ".png", true
		case ".webp":
			return ".webp", true
		}
	case "video":
		switch extension {
		case ".mp4":
			return ".mp4", true
		case ".mov":
			return ".mov", true
		}
	case "audio":
		switch extension {
		case ".mp3":
			return ".mp3", true
		case ".wav":
			return ".wav", true
		}
	}
	return "", false
}
