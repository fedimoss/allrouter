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
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

type autoDLUploadResponse struct {
	Code    string `json:"code"`
	Success bool   `json:"success"`
	Data    struct {
		Path string `json:"path"`
	} `json:"data"`
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
	client, err := GetHttpClientWithProxy("")
	if err != nil {
		return "", fmt.Errorf("build media upload client: %w", err)
	}
	client.Timeout = timeout
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload media: %w", err)
	}
	defer CloseResponseBodyGracefully(resp)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read media upload response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("media upload status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed autoDLUploadResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse media upload response: %w", err)
	}
	if !parsed.Success && parsed.Code != "Success" && parsed.Code != "OK" {
		return "", fmt.Errorf("media upload failed: code=%s", parsed.Code)
	}
	return PublicAutoDLMediaURL(parsed.Data.Path, publicBaseURL)
}

func PublicAutoDLMediaURL(rawPath, publicBaseURL string) (string, error) {
	publicBaseURL = "https://allrouter.ai"
	rawPath = strings.TrimSpace(rawPath)
	if rawPath == "" {
		return "", fmt.Errorf("media upload response path is empty")
	}
	if parsed, err := url.Parse(rawPath); err == nil && parsed.Scheme != "" {
		if (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
			if !strings.HasPrefix(strings.TrimLeft(parsed.Path, "/"), "sgl-workspace/sglang/") {
				return parsed.String(), nil
			}
			rawPath = parsed.EscapedPath()
			if parsed.RawQuery != "" {
				rawPath += "?" + parsed.RawQuery
			}
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
	return publicBaseURL + "/" + rawPath, nil
}

// PersistVideoResult downloads a temporary video URL and stores a durable copy
// through the configured media uploader.
func PersistVideoResult(ctx context.Context, resultURL, publicBaseURL, filename string) (string, error) {
	resultURL = strings.TrimSpace(resultURL)
	if resultURL == "" {
		return "", fmt.Errorf("result URL is empty")
	}
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
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download AutoDL result: %w", err)
	}
	defer CloseResponseBodyGracefully(resp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download AutoDL result status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<30))
	if err != nil {
		return "", fmt.Errorf("read AutoDL result: %w", err)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}
	return UploadAutoDLMedia(ctx, data, filename, contentType, publicBaseURL)
}

func PersistAutoDLResult(ctx context.Context, resultURL, publicBaseURL string) (string, error) {
	return PersistVideoResult(ctx, resultURL, publicBaseURL, "autodl-result.mp4")
}
