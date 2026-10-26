package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
)

type autoDLUploadResponse struct {
	Code    string `json:"code"`
	Success bool   `json:"success"`
	Data    struct {
		Path string `json:"path"`
	} `json:"data"`
}

// SanitizeURLError removes IP addresses and ports from error messages to avoid exposing internal endpoints
func SanitizeURLError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Remove IP:port patterns (IPv4)
	ipPortPattern := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}:\d+\b`)
	msg = ipPortPattern.ReplaceAllString(msg, "[REDACTED_ENDPOINT]")
	// Remove standalone IPs
	ipPattern := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	msg = ipPattern.ReplaceAllString(msg, "[REDACTED_IP]")
	// Remove http://... URLs containing IPs
	urlPattern := regexp.MustCompile(`https?://[^\s"]+`)
	msg = urlPattern.ReplaceAllStringFunc(msg, func(u string) string {
		if ipPortPattern.MatchString(u) || ipPattern.MatchString(u) {
			return "[REDACTED_URL]"
		}
		return u
	})
	return fmt.Errorf("%s", msg)
}

// UploadAutoDLMedia stores media through the configured upload service and
// returns a URL reachable by AutoDL. The upload service is the same service
// used by the existing MiniMax H3 reference-video integration.
func UploadAutoDLMedia(ctx context.Context, data []byte, filename, contentType, publicBaseURL string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("media is empty")
	}
	uploadURL := strings.TrimSpace(constant.MiniMaxH3ReferenceUploadURL)
	if uploadURL == "" {
		return "", fmt.Errorf("MINIMAX_H3_REFERENCE_UPLOAD_URL is not configured")
	}
	apiKey := strings.TrimSpace(constant.MiniMaxH3ReferenceUploadAPIKey)
	if apiKey == "" {
		return "", fmt.Errorf("MINIMAX_H3_REFERENCE_UPLOAD_API_KEY is not configured")
	}
	if filename == "" {
		filename = "media.bin"
	}
	sum := sha256.Sum256(data)

	logger.LogInfo(ctx, fmt.Sprintf("UploadAutoDLMedia: preparing upload - size=%d bytes, filename=%s, contentType=%s", len(data), filename, contentType))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("build media upload request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("X-Filename", filepath.Base(filename))
	req.Header.Set("X-SHA256", hex.EncodeToString(sum[:]))

	timeout := time.Duration(constant.MiniMaxH3ReferenceUploadTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	logger.LogInfo(ctx, fmt.Sprintf("UploadAutoDLMedia: upload timeout configured as %v", timeout))

	client, err := GetHttpClientWithProxy("")
	if err != nil {
		return "", fmt.Errorf("build media upload client: %w", err)
	}
	client.Timeout = timeout

	logger.LogInfo(ctx, fmt.Sprintf("UploadAutoDLMedia: starting upload request - sha256=%s", hex.EncodeToString(sum[:])))
	resp, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("UploadAutoDLMedia: upload request failed - error=%v, timeout=%v, size=%d", err, timeout, len(data)))
		return "", fmt.Errorf("upload media: %w", SanitizeURLError(err))
	}
	defer CloseResponseBodyGracefully(resp)

	logger.LogInfo(ctx, fmt.Sprintf("UploadAutoDLMedia: received response - status=%d", resp.StatusCode))

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("UploadAutoDLMedia: failed to read response body - error=%v, status=%d", err, resp.StatusCode))
		return "", fmt.Errorf("read media upload response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.LogError(ctx, fmt.Sprintf("UploadAutoDLMedia: non-2xx status - status=%d, body=%s", resp.StatusCode, strings.TrimSpace(string(body))))
		return "", fmt.Errorf("media upload status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed autoDLUploadResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		logger.LogError(ctx, fmt.Sprintf("UploadAutoDLMedia: failed to parse response - error=%v, body=%s", err, string(body)))
		return "", fmt.Errorf("parse media upload response: %w", err)
	}
	if !parsed.Success && parsed.Code != "Success" && parsed.Code != "OK" {
		logger.LogError(ctx, fmt.Sprintf("UploadAutoDLMedia: upload failed - code=%s, body=%s", parsed.Code, string(body)))
		return "", fmt.Errorf("media upload failed: code=%s", parsed.Code)
	}

	logger.LogInfo(ctx, fmt.Sprintf("UploadAutoDLMedia: upload succeeded - path=%s", parsed.Data.Path))

	return PublicAutoDLMediaURL(parsed.Data.Path, publicBaseURL)
}

func PublicAutoDLMediaURL(rawPath, publicBaseURL string) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", fmt.Errorf("media upload response path is empty")
	}

	logger.LogDebug(context.Background(), fmt.Sprintf("PublicAutoDLMediaURL: converting path - rawPath=%s, publicBaseURL=%s", rawPath, publicBaseURL))

	if parsed, err := url.Parse(rawPath); err == nil && parsed.Scheme != "" {
		if (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			if !strings.HasPrefix(strings.TrimLeft(parsed.Path, "/"), "sgl-workspace/sglang/") {
				logger.LogDebug(context.Background(), fmt.Sprintf("PublicAutoDLMediaURL: path is already public URL - url=%s", parsed.String()))
				return parsed.String(), nil
			}
			rawPath = parsed.EscapedPath()
			if parsed.RawQuery != "" {
				rawPath += "?" + parsed.RawQuery
			}
			logger.LogDebug(context.Background(), fmt.Sprintf("PublicAutoDLMediaURL: extracted path from URL - path=%s", rawPath))
		} else {
			return "", fmt.Errorf("media upload response is not a public HTTP URL: %s", rawPath)
		}
	}
	publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if publicBaseURL == "" {
		return "", fmt.Errorf("public base URL is empty")
	}

	// The uploader exposes a container filesystem path. The gateway's public
	// route mounts the same files below /sglang/.
	rawPath = strings.TrimLeft(rawPath, "/")
	rawPath = strings.TrimPrefix(rawPath, "sgl-workspace/")
	finalURL := publicBaseURL + "/" + rawPath

	logger.LogDebug(context.Background(), fmt.Sprintf("PublicAutoDLMediaURL: constructed final URL - url=%s", finalURL))

	return finalURL, nil
}

const maxPersistedVideoBytes int64 = 1 << 30

// PersistVideoResult downloads a temporary video URL to local disk and returns
// the public URL path that should be mapped to MiniMaxH3ResultDir by the web server.
func PersistVideoResult(ctx context.Context, resultURL, publicBaseURL, taskID, filename string) (string, error) {
	resultURL = strings.TrimSpace(resultURL)
	if resultURL == "" {
		return "", fmt.Errorf("result URL is empty")
	}
	if strings.TrimSpace(taskID) == "" {
		return "", fmt.Errorf("task ID is empty")
	}
	resultDir := strings.TrimSpace(constant.MiniMaxH3ResultDir)
	if resultDir == "" {
		return "", fmt.Errorf("MINIMAX_H3_RESULT_DIR is not configured")
	}
	resultURLPath, err := miniMaxH3ResultURL(publicBaseURL, taskID, filename)
	if err != nil {
		return "", err
	}

	fileName := filepath.Base(filename)
	if fileName == "." || fileName == string(filepath.Separator) || fileName == "" {
		fileName = "video.mp4"
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	if extension == "" {
		extension = ".mp4"
	}
	taskHash := sha256.Sum256([]byte(taskID))
	fileName = hex.EncodeToString(taskHash[:]) + extension
	resultDir, err = filepath.Abs(resultDir)
	if err != nil {
		return "", fmt.Errorf("resolve MiniMax-H3 result directory: %w", err)
	}
	if err := os.MkdirAll(resultDir, 0o755); err != nil {
		return "", fmt.Errorf("create MiniMax-H3 result directory: %w", err)
	}
	resultPath := filepath.Join(resultDir, fileName)
	if info, statErr := os.Stat(resultPath); statErr == nil {
		if info.Mode().IsRegular() && info.Size() > 0 {
			return resultURLPath, nil
		}
		if err := os.Remove(resultPath); err != nil {
			return "", fmt.Errorf("remove invalid MiniMax-H3 result file: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect MiniMax-H3 result file: %w", statErr)
	}

	logger.LogInfo(ctx, fmt.Sprintf("PersistVideoResult: starting download - url=%s, task=%s", resultURL, taskID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resultURL, nil)
	if err != nil {
		return "", fmt.Errorf("build result download request: %w", err)
	}
	client, err := GetHttpClientWithProxy("")
	if err != nil {
		return "", fmt.Errorf("build result download client: %w", err)
	}
	timeout := time.Duration(constant.MiniMaxH3ReferenceUploadTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	client.Timeout = timeout

	logger.LogInfo(ctx, fmt.Sprintf("PersistVideoResult: downloading with timeout=%v", timeout))

	resp, err := client.Do(req)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("PersistVideoResult: download failed - error=%v, timeout=%v", err, timeout))
		return "", fmt.Errorf("download video result: %w", SanitizeURLError(err))
	}
	defer CloseResponseBodyGracefully(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.LogError(ctx, fmt.Sprintf("PersistVideoResult: non-2xx status - status=%d", resp.StatusCode))
		return "", fmt.Errorf("download video result status %d", resp.StatusCode)
	}

	logger.LogInfo(ctx, fmt.Sprintf("PersistVideoResult: saving response body - contentType=%s, contentLength=%d", resp.Header.Get("Content-Type"), resp.ContentLength))
	temporaryFile, err := os.CreateTemp(resultDir, ".minimax-h3-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temporary MiniMax-H3 result file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if err := temporaryFile.Chmod(0o644); err != nil {
		_ = temporaryFile.Close()
		return "", fmt.Errorf("set MiniMax-H3 result file permissions: %w", err)
	}
	written, err := io.Copy(temporaryFile, io.LimitReader(resp.Body, maxPersistedVideoBytes+1))
	if err != nil {
		_ = temporaryFile.Close()
		return "", fmt.Errorf("write MiniMax-H3 result file: %w", err)
	}
	if written == 0 {
		_ = temporaryFile.Close()
		return "", fmt.Errorf("downloaded MiniMax-H3 result is empty")
	}
	if written > maxPersistedVideoBytes {
		_ = temporaryFile.Close()
		return "", fmt.Errorf("downloaded MiniMax-H3 result exceeds %d bytes", maxPersistedVideoBytes)
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return "", fmt.Errorf("sync MiniMax-H3 result file: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return "", fmt.Errorf("close MiniMax-H3 result file: %w", err)
	}
	if err := os.Rename(temporaryPath, resultPath); err != nil {
		if info, statErr := os.Stat(resultPath); statErr == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return resultURLPath, nil
		}
		return "", fmt.Errorf("finalize MiniMax-H3 result file: %w", err)
	}

	logger.LogInfo(ctx, fmt.Sprintf("PersistVideoResult: download complete - task=%s size=%d path=%s", taskID, written, resultPath))
	return resultURLPath, nil
}

func miniMaxH3ResultURL(publicBaseURL, taskID, filename string) (string, error) {
	publicBaseURL = strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if publicBaseURL == "" {
		return "", fmt.Errorf("public base URL is empty")
	}
	parsedBase, err := url.Parse(publicBaseURL)
	if err != nil || (parsedBase.Scheme != "http" && parsedBase.Scheme != "https") || parsedBase.Host == "" {
		return "", fmt.Errorf("public base URL must be an absolute HTTP URL")
	}
	urlPath := strings.TrimSpace(constant.MiniMaxH3ResultURLPath)
	if urlPath == "" {
		return "", fmt.Errorf("MINIMAX_H3_RESULT_URL_PATH is not configured")
	}
	if strings.ContainsAny(urlPath, "?#\\") {
		return "", fmt.Errorf("MINIMAX_H3_RESULT_URL_PATH must be a URL path")
	}
	for _, segment := range strings.Split(urlPath, "/") {
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("MINIMAX_H3_RESULT_URL_PATH cannot contain dot segments")
		}
	}
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	fileName := filepath.Base(filename)
	if fileName == "." || fileName == string(filepath.Separator) || fileName == "" {
		fileName = "video.mp4"
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	if extension == "" {
		extension = ".mp4"
	}
	taskHash := sha256.Sum256([]byte(taskID))
	return publicBaseURL + path.Join(urlPath, hex.EncodeToString(taskHash[:])+extension), nil
}

func PersistAutoDLResult(ctx context.Context, resultURL, publicBaseURL, taskID string) (string, error) {
	return PersistVideoResult(ctx, resultURL, publicBaseURL, taskID, "autodl-result.mp4")
}
