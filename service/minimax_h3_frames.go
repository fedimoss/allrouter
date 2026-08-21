package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

const MiniMaxH3FrameMaxBytes int64 = 10 << 20

const MiniMaxH3FrameIDsContextKey = "minimax_h3_frame_ids"
const MiniMaxH3ReferenceVideoIDsContextKey = "minimax_h3_reference_video_ids"

var miniMaxH3FrameExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func miniMaxH3FrameUserDir(userId int) (string, error) {
	if userId <= 0 {
		return "", errors.New("user id is required")
	}
	root := strings.TrimSpace(constant.MiniMaxH3FrameUploadDir)
	if root == "" {
		return "", errors.New("MINIMAX_H3_FRAME_UPLOAD_DIR is not configured")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve MiniMax-H3 frame upload directory: %w", err)
	}
	return filepath.Join(absRoot, strconv.Itoa(userId)), nil
}

func randomMiniMaxH3FrameID(extension string) (string, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate frame id: %w", err)
	}
	return hex.EncodeToString(randomBytes) + extension, nil
}

// SaveMiniMaxH3Frame stores a Playground frame in the per-user shared directory.
func SaveMiniMaxH3Frame(userId int, fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader == nil {
		return "", errors.New("frame file is required")
	}
	if fileHeader.Size <= 0 {
		return "", errors.New("frame file is empty")
	}
	if fileHeader.Size > MiniMaxH3FrameMaxBytes {
		return "", fmt.Errorf("frame file exceeds %d MB", MiniMaxH3FrameMaxBytes>>20)
	}

	source, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("open frame file: %w", err)
	}
	defer source.Close()

	header := make([]byte, 512)
	headerSize, err := io.ReadFull(source, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read frame file: %w", err)
	}
	header = header[:headerSize]
	contentType := http.DetectContentType(header)
	extension, supported := miniMaxH3FrameExtensions[contentType]
	if !supported {
		return "", errors.New("frame must be a JPEG, PNG, or WebP image")
	}

	userDir, err := miniMaxH3FrameUserDir(userId)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(userDir, 0o750); err != nil {
		return "", fmt.Errorf("create frame upload directory: %w", err)
	}

	frameID, err := randomMiniMaxH3FrameID(extension)
	if err != nil {
		return "", err
	}
	framePath := filepath.Join(userDir, frameID)
	destination, err := os.OpenFile(framePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", fmt.Errorf("create frame file: %w", err)
	}
	keepFile := false
	defer func() {
		_ = destination.Close()
		if !keepFile {
			_ = os.Remove(framePath)
		}
	}()

	reader := io.MultiReader(bytes.NewReader(header), source)
	written, err := io.Copy(destination, io.LimitReader(reader, MiniMaxH3FrameMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("save frame file: %w", err)
	}
	if written > MiniMaxH3FrameMaxBytes {
		return "", fmt.Errorf("frame file exceeds %d MB", MiniMaxH3FrameMaxBytes>>20)
	}
	if err := destination.Close(); err != nil {
		return "", fmt.Errorf("close frame file: %w", err)
	}
	keepFile = true
	return frameID, nil
}

// ResolveMiniMaxH3FrameURI resolves a user-owned opaque frame id to a local file URI.
func miniMaxH3FramePath(userId int, frameID string) (string, error) {
	frameID = strings.TrimSpace(frameID)
	if frameID == "" {
		return "", errors.New("frame id is required")
	}
	if filepath.Base(frameID) != frameID || strings.ContainsAny(frameID, `/\\`) {
		return "", errors.New("invalid frame id")
	}
	extension := strings.ToLower(filepath.Ext(frameID))
	if extension != ".jpg" && extension != ".png" && extension != ".webp" {
		return "", errors.New("invalid frame id")
	}

	userDir, err := miniMaxH3FrameUserDir(userId)
	if err != nil {
		return "", err
	}
	return filepath.Join(userDir, frameID), nil
}

func ResolveMiniMaxH3FrameURI(userId int, frameID string) (string, error) {
	frameID = strings.TrimSpace(frameID)
	if frameID == "" {
		return "", nil
	}
	framePath, err := miniMaxH3FramePath(userId, frameID)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(framePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("uploaded frame does not exist")
		}
		return "", fmt.Errorf("inspect uploaded frame: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("uploaded frame is not a regular file")
	}

	uriPath, err := miniMaxH3FrameURIPath(constant.MiniMaxH3FrameUploadDir, framePath, userId, frameID)
	if err != nil {
		return "", err
	}
	return (&url.URL{Scheme: "file", Path: uriPath}).String(), nil
}

// DeleteMiniMaxH3Frame removes a user-owned temporary frame by its opaque id.
func DeleteMiniMaxH3Frame(userId int, frameID string) error {
	framePath, err := miniMaxH3FramePath(userId, frameID)
	if err != nil {
		return err
	}
	if err := os.Remove(framePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete MiniMax-H3 frame: %w", err)
	}
	return nil
}

func miniMaxH3FrameURIPath(configuredRoot string, localFramePath string, userId int, frameID string) (string, error) {
	configuredRoot = strings.TrimSpace(configuredRoot)
	uriPath := ""
	if filepath.VolumeName(configuredRoot) == "" && strings.HasPrefix(filepath.ToSlash(configuredRoot), "/") {
		// Preserve an explicitly configured POSIX path even when new-api runs on
		// Windows. This supports a shared persistent volume across gateway nodes.
		uriPath = path.Join(filepath.ToSlash(configuredRoot), strconv.Itoa(userId), frameID)
	} else {
		absPath, err := filepath.Abs(localFramePath)
		if err != nil {
			return "", fmt.Errorf("resolve uploaded frame URI path: %w", err)
		}
		uriPath = filepath.ToSlash(absPath)
	}
	if filepath.VolumeName(uriPath) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	return uriPath, nil
}

func miniMaxH3LocalPathFromURI(frameURI string) (string, bool, error) {
	parsed, err := url.Parse(strings.TrimSpace(frameURI))
	if err != nil {
		return "", false, fmt.Errorf("parse MiniMax-H3 frame URI: %w", err)
	}
	if parsed.Scheme != "file" {
		return "", false, nil
	}

	configuredRoot := strings.TrimSpace(constant.MiniMaxH3FrameUploadDir)
	configuredURIPath := filepath.ToSlash(configuredRoot)
	if filepath.VolumeName(configuredRoot) != "" || !strings.HasPrefix(configuredURIPath, "/") {
		absRoot, err := filepath.Abs(configuredRoot)
		if err != nil {
			return "", false, fmt.Errorf("resolve MiniMax-H3 frame root: %w", err)
		}
		configuredURIPath = filepath.ToSlash(absRoot)
		if filepath.VolumeName(absRoot) != "" && !strings.HasPrefix(configuredURIPath, "/") {
			configuredURIPath = "/" + configuredURIPath
		}
	}

	cleanRootPath := strings.TrimRight(path.Clean(configuredURIPath), "/")
	cleanFramePath := path.Clean(parsed.Path)
	if cleanFramePath != cleanRootPath && !strings.HasPrefix(cleanFramePath, cleanRootPath+"/") {
		return "", false, errors.New("MiniMax-H3 frame URI is outside the upload directory")
	}
	relativePath := strings.TrimPrefix(strings.TrimPrefix(cleanFramePath, cleanRootPath), "/")
	absRoot, err := filepath.Abs(configuredRoot)
	if err != nil {
		return "", false, fmt.Errorf("resolve MiniMax-H3 local frame root: %w", err)
	}
	localPath := filepath.Join(absRoot, filepath.FromSlash(relativePath))
	localRelativePath, err := filepath.Rel(absRoot, localPath)
	if err != nil || localRelativePath == ".." || strings.HasPrefix(localRelativePath, ".."+string(filepath.Separator)) {
		return "", false, errors.New("MiniMax-H3 local frame path is outside the upload directory")
	}
	return localPath, true, nil
}

// DeleteMiniMaxH3FrameURI removes a temporary local frame after it is no
// longer needed by the persistent task queue. Non-file URIs are ignored.
func DeleteMiniMaxH3FrameURI(frameURI string) error {
	localPath, local, err := miniMaxH3LocalPathFromURI(frameURI)
	if err != nil || !local {
		return err
	}
	if err := os.Remove(localPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete MiniMax-H3 frame: %w", err)
	}
	return nil
}

// EmbedMiniMaxH3FrameURI converts a locally stored Playground frame into a
// data URI before dispatching it to a remote model server. This avoids
// requiring the new-api host and the model container to share a filesystem.
func EmbedMiniMaxH3FrameURI(frameURI string) (string, error) {
	localPath, local, err := miniMaxH3LocalPathFromURI(frameURI)
	if err != nil {
		return "", err
	}
	if !local {
		return frameURI, nil
	}

	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open MiniMax-H3 frame for dispatch: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MiniMaxH3FrameMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("read MiniMax-H3 frame for dispatch: %w", err)
	}
	if len(data) > int(MiniMaxH3FrameMaxBytes) {
		return "", fmt.Errorf("frame file exceeds %d MB", MiniMaxH3FrameMaxBytes>>20)
	}
	contentType := http.DetectContentType(data)
	if _, supported := miniMaxH3FrameExtensions[contentType]; !supported {
		return "", errors.New("stored MiniMax-H3 frame has an unsupported image type")
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
