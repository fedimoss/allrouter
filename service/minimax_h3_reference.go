package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

type miniMaxH3ReferenceUploadResponse struct {
	Code    string `json:"code"`
	Success bool   `json:"success"`
	Data    struct {
		Filename string `json:"filename"`
		Path     string `json:"path"`
		SHA256   string `json:"sha256"`
		Size     int64  `json:"size"`
	} `json:"data"`
}

// SaveMiniMaxH3ReferenceVideo stores a reference video in the same temporary
// media root used by MiniMax-H3 frame uploads. The file is kept until the
// queued task is dispatched or fails.
func SaveMiniMaxH3ReferenceVideo(userId int, fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader == nil {
		return "", errors.New("reference video is required")
	}
	if fileHeader.Size <= 0 {
		return "", errors.New("reference video is empty")
	}
	if fileHeader.Size > constant.MiniMaxH3ReferenceVideoMaxBytes {
		return "", fmt.Errorf("reference video exceeds %d MB", constant.MiniMaxH3ReferenceVideoMaxBytes>>20)
	}

	extension := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if extension != ".mp4" {
		return "", errors.New("reference video must be an MP4 file")
	}
	userDir, err := miniMaxH3FrameUserDir(userId)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(userDir, 0o750); err != nil {
		return "", fmt.Errorf("create reference video directory: %w", err)
	}

	videoID, err := randomMiniMaxH3FrameID(".mp4")
	if err != nil {
		return "", err
	}
	videoPath := filepath.Join(userDir, videoID)
	source, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("open reference video: %w", err)
	}
	defer source.Close()

	destination, err := os.OpenFile(videoPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", fmt.Errorf("create reference video: %w", err)
	}
	keepFile := false
	defer func() {
		_ = destination.Close()
		if !keepFile {
			_ = os.Remove(videoPath)
		}
	}()
	written, err := io.Copy(destination, io.LimitReader(source, constant.MiniMaxH3ReferenceVideoMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("save reference video: %w", err)
	}
	if written > constant.MiniMaxH3ReferenceVideoMaxBytes {
		return "", fmt.Errorf("reference video exceeds %d MB", constant.MiniMaxH3ReferenceVideoMaxBytes>>20)
	}
	if err := destination.Close(); err != nil {
		return "", fmt.Errorf("close reference video: %w", err)
	}
	keepFile = true
	return videoID, nil
}

// ResolveMiniMaxH3ReferenceVideoURI returns the local file URI persisted in a
// queued request. It intentionally uses the configured POSIX path when one is
// configured, so the URI can be understood by a shared SGLang media volume.
func ResolveMiniMaxH3ReferenceVideoURI(userId int, videoID string) (string, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" || filepath.Base(videoID) != videoID || strings.ToLower(filepath.Ext(videoID)) != ".mp4" {
		return "", errors.New("invalid reference video id")
	}
	userDir, err := miniMaxH3FrameUserDir(userId)
	if err != nil {
		return "", err
	}
	videoPath := filepath.Join(userDir, videoID)
	info, err := os.Stat(videoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("uploaded reference video does not exist")
		}
		return "", fmt.Errorf("inspect uploaded reference video: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("uploaded reference video is not a regular file")
	}
	uriPath, err := miniMaxH3FrameURIPath(constant.MiniMaxH3FrameUploadDir, videoPath, userId, videoID)
	if err != nil {
		return "", err
	}
	return (&url.URL{Scheme: "file", Path: uriPath}).String(), nil
}

func DeleteMiniMaxH3ReferenceVideo(userId int, videoID string) error {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" || filepath.Base(videoID) != videoID || strings.ToLower(filepath.Ext(videoID)) != ".mp4" {
		return errors.New("invalid reference video id")
	}
	userDir, err := miniMaxH3FrameUserDir(userId)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(userDir, videoID)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete MiniMax-H3 reference video: %w", err)
	}
	return nil
}

// DeleteMiniMaxH3MediaURI removes a temporary image/video file owned by the
// configured MiniMax-H3 media root. Non-file URIs are intentionally ignored.
func DeleteMiniMaxH3MediaURI(mediaURI string) error {
	localPath, local, err := miniMaxH3LocalPathFromURI(mediaURI)
	if err != nil || !local {
		return err
	}
	if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete MiniMax-H3 media: %w", err)
	}
	return nil
}

func fileURIFromUploadedPath(uploadedPath string) (string, error) {
	uploadedPath = strings.TrimSpace(uploadedPath)
	if uploadedPath == "" {
		return "", errors.New("reference upload response path is empty")
	}
	if parsed, err := url.Parse(uploadedPath); err == nil && parsed.Scheme != "" {
		if parsed.Scheme == "file" || parsed.Scheme == "http" || parsed.Scheme == "https" {
			return parsed.String(), nil
		}
		return "", fmt.Errorf("unsupported reference upload URI scheme: %s", parsed.Scheme)
	}
	if !filepath.IsAbs(uploadedPath) && !strings.HasPrefix(filepath.ToSlash(uploadedPath), "/") {
		return "", errors.New("reference upload response path must be absolute")
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(uploadedPath)}).String(), nil
}

// UploadMiniMaxH3ReferenceVideo uploads a locally stored MP4 to the configured
// media server and returns a URI suitable for conditions[].uri.
func UploadMiniMaxH3ReferenceVideo(ctx context.Context, localURI, filename string) (string, error) {
	uploadURL := strings.TrimSpace(constant.MiniMaxH3ReferenceUploadURL)
	if uploadURL == "" {
		return "", errors.New("MINIMAX_H3_REFERENCE_UPLOAD_URL is not configured")
	}
	if strings.TrimSpace(constant.MiniMaxH3ReferenceUploadAPIKey) == "" {
		return "", errors.New("MINIMAX_H3_REFERENCE_UPLOAD_API_KEY is not configured")
	}
	localPath, local, err := miniMaxH3LocalPathFromURI(localURI)
	if err != nil {
		return "", err
	}
	if !local {
		return localURI, nil
	}
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open reference video for upload: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect reference video for upload: %w", err)
	}
	if stat.Size() <= 0 || stat.Size() > constant.MiniMaxH3ReferenceVideoMaxBytes {
		return "", fmt.Errorf("reference video size is invalid: %d", stat.Size())
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash reference video: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind reference video: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, file)
	if err != nil {
		return "", fmt.Errorf("build reference upload request: %w", err)
	}
	request.ContentLength = stat.Size()
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("X-API-Key", constant.MiniMaxH3ReferenceUploadAPIKey)
	safeFilename := filepath.Base(strings.TrimSpace(filename))
	if safeFilename == "" || strings.ToLower(filepath.Ext(safeFilename)) != ".mp4" {
		safeFilename = "reference.mp4"
	}
	request.Header.Set("X-Filename", safeFilename)
	request.Header.Set("X-SHA256", hex.EncodeToString(hash.Sum(nil)))

	baseClient, err := GetHttpClientWithProxy("")
	if err != nil {
		return "", fmt.Errorf("build reference upload client: %w", err)
	}
	client := &http.Client{
		Transport: baseClient.Transport,
		Timeout:   time.Duration(constant.MiniMaxH3ReferenceUploadTimeoutSeconds) * time.Second,
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("upload reference video: %w", err)
	}
	defer CloseResponseBodyGracefully(response)
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read reference upload response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("reference upload status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var uploadResponse miniMaxH3ReferenceUploadResponse
	if err := common.Unmarshal(body, &uploadResponse); err != nil {
		return "", fmt.Errorf("parse reference upload response: %w", err)
	}
	if !uploadResponse.Success || (uploadResponse.Code != "" && uploadResponse.Code != "OK") {
		return "", fmt.Errorf("reference upload failed: code=%s", uploadResponse.Code)
	}
	return fileURIFromUploadedPath(uploadResponse.Data.Path)
}
