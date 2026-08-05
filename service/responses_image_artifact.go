package service

import (
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/constant"
)

const (
	responsesImageArtifactDefaultDirectory = "data/responses-images"
	responsesImageArtifactSigningKeyFile   = ".signing-key"
	responsesImageArtifactIDBytes          = 32
	responsesImageArtifactDefaultMaxMB     = 64
)

var (
	ErrResponsesImageArtifactNotFound = errors.New("responses image artifact not found")
	ErrResponsesImageArtifactExpired  = errors.New("responses image artifact expired")
	ErrResponsesImageArtifactAccess   = errors.New("responses image artifact access denied")
)

// ResponsesImageArtifact 是网关保存的图片工件及其短期访问能力信息。
// ID 与签名 URL 都不包含用户令牌，因此可以安全地放进 Markdown 图片链接，供
// 不会携带 Authorization 头的渲染器直接加载。
type ResponsesImageArtifact struct {
	ID          string
	ExpiresAt   time.Time
	ContentType string
	Extension   string
	Signature   string
}

// ResponsesImageArtifactFile 是已通过签名和过期校验、可以交给 HTTP handler 输出的文件。
type ResponsesImageArtifactFile struct {
	File        *os.File
	Size        int64
	ContentType string
	Filename    string
}

// SaveResponsesImageArtifact decodes one completed image result and writes it to a private
// local artifact directory. The returned artifact has a signed capability URL that survives
// process restarts because the signing key is persisted alongside the private files.
func SaveResponsesImageArtifact(base64Data, directory string, ttl time.Duration) (*ResponsesImageArtifact, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("responses image artifact TTL must be positive")
	}

	data, err := decodeResponsesImageArtifactBase64(base64Data)
	if err != nil {
		return nil, err
	}
	contentType, extension, err := responsesImageArtifactType(data)
	if err != nil {
		return nil, err
	}

	directory = responsesImageArtifactDirectory(directory)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("create responses image artifact directory: %w", err)
	}
	// Cleanup is intentionally best-effort: a completed image must still be returned even when
	// an old filesystem entry cannot be removed at this moment.
	_ = cleanupExpiredResponsesImageArtifacts(directory, time.Now())

	signingKey, err := responsesImageArtifactSigningKey(directory)
	if err != nil {
		return nil, err
	}

	id, err := responsesImageArtifactID()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(ttl).UTC()
	expiresUnix := expiresAt.Unix()
	artifact := &ResponsesImageArtifact{
		ID:          id,
		ExpiresAt:   expiresAt,
		ContentType: contentType,
		Extension:   extension,
	}
	artifact.Signature = responsesImageArtifactSignature(signingKey, artifact.ID, expiresUnix, artifact.Extension)

	path := responsesImageArtifactPath(directory, artifact.ID, expiresUnix, artifact.Extension)
	if err := writeResponsesImageArtifact(path, data); err != nil {
		return nil, err
	}
	return artifact, nil
}

// PublicURL returns an absolute or relative signed capability URL for this artifact.
func (artifact *ResponsesImageArtifact) PublicURL(baseURL string) string {
	if artifact == nil || artifact.ID == "" || artifact.ExpiresAt.IsZero() || artifact.Signature == "" {
		return ""
	}
	query := url.Values{}
	query.Set("expires", strconv.FormatInt(artifact.ExpiresAt.Unix(), 10))
	query.Set("format", artifact.Extension)
	query.Set("sig", artifact.Signature)
	path := "/v1/responses/images/" + artifact.ID
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return path + "?" + query.Encode()
	}
	return baseURL + path + "?" + query.Encode()
}

// OpenResponsesImageArtifact validates a signed artifact URL and returns the private file.
// Invalid, expired, and unknown artifacts deliberately share opaque errors so the reader
// endpoint does not disclose whether an ID exists.
func OpenResponsesImageArtifact(id, expires, extension, signature, directory string) (*ResponsesImageArtifactFile, error) {
	if !isResponsesImageArtifactID(id) {
		return nil, ErrResponsesImageArtifactAccess
	}
	expiresUnix, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || expiresUnix <= 0 {
		return nil, ErrResponsesImageArtifactAccess
	}
	contentType, normalizedExtension, ok := responsesImageArtifactContentType(extension)
	if !ok || normalizedExtension != extension {
		return nil, ErrResponsesImageArtifactAccess
	}

	directory = responsesImageArtifactDirectory(directory)
	signingKey, err := loadResponsesImageArtifactSigningKey(directory)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrResponsesImageArtifactNotFound
		}
		return nil, fmt.Errorf("load responses image artifact signing key: %w", err)
	}
	expectedSignature := responsesImageArtifactSignature(signingKey, id, expiresUnix, extension)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, ErrResponsesImageArtifactAccess
	}
	if time.Now().UTC().Unix() > expiresUnix {
		_ = os.Remove(responsesImageArtifactPath(directory, id, expiresUnix, extension))
		return nil, ErrResponsesImageArtifactExpired
	}

	file, err := os.Open(responsesImageArtifactPath(directory, id, expiresUnix, extension))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrResponsesImageArtifactNotFound
		}
		return nil, fmt.Errorf("open responses image artifact: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat responses image artifact: %w", err)
	}
	return &ResponsesImageArtifactFile{
		File:        file,
		Size:        info.Size(),
		ContentType: contentType,
		Filename:    "generated-image." + extension,
	}, nil
}

func responsesImageArtifactDirectory(directory string) string {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return responsesImageArtifactDefaultDirectory
	}
	return directory
}

func decodeResponsesImageArtifactBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[comma+1:]
	}
	if value == "" {
		return nil, errors.New("responses image artifact data is empty")
	}
	maxBytes := responsesImageArtifactMaxBytes()
	// 先按 Base64 最大解码长度拒绝明显超限的数据，避免为异常上游响应分配过大的
	// 解码缓冲；解码后还会按实际字节数复核一次。
	if int64(base64.StdEncoding.DecodedLen(len(value))) > maxBytes {
		return nil, fmt.Errorf("responses image artifact exceeds maximum size of %d bytes", maxBytes)
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return validateResponsesImageArtifactSize(data, maxBytes)
	}
	data, rawErr := base64.RawStdEncoding.DecodeString(value)
	if rawErr == nil {
		return validateResponsesImageArtifactSize(data, maxBytes)
	}
	return nil, fmt.Errorf("decode responses image artifact Base64: %w", err)
}

func responsesImageArtifactMaxBytes() int64 {
	limitMB := constant.MaxFileDownloadMB
	if limitMB <= 0 {
		limitMB = responsesImageArtifactDefaultMaxMB
	}
	return int64(limitMB) << 20
}

func validateResponsesImageArtifactSize(data []byte, maxBytes int64) ([]byte, error) {
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("responses image artifact exceeds maximum size of %d bytes", maxBytes)
	}
	return data, nil
}

func responsesImageArtifactType(data []byte) (string, string, error) {
	contentType := http.DetectContentType(data)
	for extension, candidateType := range map[string]string{
		"png":  "image/png",
		"jpg":  "image/jpeg",
		"gif":  "image/gif",
		"webp": "image/webp",
	} {
		if contentType == candidateType {
			return candidateType, extension, nil
		}
	}
	return "", "", fmt.Errorf("unsupported responses image artifact content type: %s", contentType)
}

func responsesImageArtifactContentType(extension string) (string, string, bool) {
	switch extension {
	case "png":
		return "image/png", "png", true
	case "jpg":
		return "image/jpeg", "jpg", true
	case "gif":
		return "image/gif", "gif", true
	case "webp":
		return "image/webp", "webp", true
	default:
		return "", "", false
	}
}

func responsesImageArtifactID() (string, error) {
	data := make([]byte, responsesImageArtifactIDBytes)
	if _, err := io.ReadFull(crand.Reader, data); err != nil {
		return "", fmt.Errorf("generate responses image artifact ID: %w", err)
	}
	return hex.EncodeToString(data), nil
}

func isResponsesImageArtifactID(id string) bool {
	if len(id) != responsesImageArtifactIDBytes*2 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func responsesImageArtifactSigningKey(directory string) ([]byte, error) {
	path := filepath.Join(directory, responsesImageArtifactSigningKeyFile)
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) < responsesImageArtifactIDBytes {
			return nil, errors.New("responses image artifact signing key is too short")
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read responses image artifact signing key: %w", err)
	}

	key = make([]byte, responsesImageArtifactIDBytes)
	if _, err := io.ReadFull(crand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate responses image artifact signing key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, writeErr := file.Write(key)
		syncErr := file.Sync()
		closeErr := file.Close()
		if writeErr != nil || syncErr != nil || closeErr != nil {
			_ = os.Remove(path)
			if writeErr != nil {
				return nil, fmt.Errorf("write responses image artifact signing key: %w", writeErr)
			}
			if syncErr != nil {
				return nil, fmt.Errorf("sync responses image artifact signing key: %w", syncErr)
			}
			return nil, fmt.Errorf("close responses image artifact signing key: %w", closeErr)
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create responses image artifact signing key: %w", err)
	}
	key, err = os.ReadFile(path)
	if err != nil || len(key) < responsesImageArtifactIDBytes {
		if err != nil {
			return nil, fmt.Errorf("read concurrent responses image artifact signing key: %w", err)
		}
		return nil, errors.New("concurrent responses image artifact signing key is too short")
	}
	return key, nil
}

// loadResponsesImageArtifactSigningKey 只读取已有密钥，不在下载请求中创建目录或密钥。
// 这样无效 URL 不会产生副作用，也不会把一个不存在的工件误判为服务端故障。
func loadResponsesImageArtifactSigningKey(directory string) ([]byte, error) {
	key, err := os.ReadFile(filepath.Join(responsesImageArtifactDirectory(directory), responsesImageArtifactSigningKeyFile))
	if err != nil {
		return nil, err
	}
	if len(key) < responsesImageArtifactIDBytes {
		return nil, errors.New("responses image artifact signing key is too short")
	}
	return key, nil
}

func responsesImageArtifactSignature(key []byte, id string, expiresUnix int64, extension string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(id))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(strconv.FormatInt(expiresUnix, 10)))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(extension))
	return hex.EncodeToString(mac.Sum(nil))
}

func responsesImageArtifactPath(directory, id string, expiresUnix int64, extension string) string {
	return filepath.Join(directory, fmt.Sprintf("%s-%d.%s", id, expiresUnix, extension))
}

func writeResponsesImageArtifact(path string, data []byte) error {
	directory := filepath.Dir(path)
	file, err := os.CreateTemp(directory, ".responses-image-*")
	if err != nil {
		return fmt.Errorf("create responses image artifact temporary file: %w", err)
	}
	temporaryPath := file.Name()
	defer func() {
		_ = os.Remove(temporaryPath)
	}()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("set responses image artifact permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write responses image artifact: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync responses image artifact: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close responses image artifact: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("commit responses image artifact: %w", err)
	}
	return nil
}

func cleanupExpiredResponsesImageArtifacts(directory string, now time.Time) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == responsesImageArtifactSigningKeyFile {
			continue
		}
		name := entry.Name()
		extension := filepath.Ext(name)
		if extension == "" {
			continue
		}
		base := strings.TrimSuffix(name, extension)
		separator := strings.LastIndexByte(base, '-')
		if separator < 0 {
			continue
		}
		expiresUnix, err := strconv.ParseInt(base[separator+1:], 10, 64)
		if err != nil || now.UTC().Unix() <= expiresUnix {
			continue
		}
		_ = os.Remove(filepath.Join(directory, name))
	}
	return nil
}
