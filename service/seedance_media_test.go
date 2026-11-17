package service

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

const testPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

// 逻辑：用真实 multipart 表单构造 FileHeader，贴近上传接口行为。
func multipartFileHeader(t *testing.T, fieldName, filename string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	form, err := multipart.NewReader(&body, writer.Boundary()).ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	require.Len(t, form.File[fieldName], 1)
	return form.File[fieldName][0]
}

// 逻辑：覆盖落盘、公网 URL 拼接和删除；测试结束恢复全局目录配置。
// 注意：该测试会改写 H3 共享目录常量，必须使用 t.TempDir 隔离。
func TestSaveSeedanceMediaFileAndPublicURL(t *testing.T) {
	previousDir := constant.MiniMaxH3FrameUploadDir
	previousPath := constant.MiniMaxH3FrameURLPath
	constant.MiniMaxH3FrameUploadDir = t.TempDir()
	constant.MiniMaxH3FrameURLPath = "/sglang/minimax-h3-frames"
	t.Cleanup(func() {
		constant.MiniMaxH3FrameUploadDir = previousDir
		constant.MiniMaxH3FrameURLPath = previousPath
	})

	data, err := base64.StdEncoding.DecodeString(testPNGBase64)
	require.NoError(t, err)
	header := multipartFileHeader(t, "reference_images", "test.png", data)

	mediaID, err := SaveSeedanceMediaFile(7, header, "image")
	require.NoError(t, err)
	require.Equal(t, ".png", filepath.Ext(mediaID))
	mediaPath := filepath.Join(constant.MiniMaxH3FrameUploadDir, "7", mediaID)
	_, err = os.Stat(mediaPath)
	require.NoError(t, err)

	publicURL, err := SeedanceMediaPublicURL("https://gateway.example", 7, mediaID)
	require.NoError(t, err)
	require.Equal(t, "https://gateway.example/sglang/minimax-h3-frames/7/"+mediaID, publicURL)

	require.NoError(t, DeleteSeedanceMedia(7, mediaID))
	_, err = os.Stat(mediaPath)
	require.True(t, os.IsNotExist(err))
}

func TestSaveSeedanceMediaRejectsUnsupportedKind(t *testing.T) {
	header := multipartFileHeader(t, "file", "test.png", []byte("not an image"))
	_, err := SaveSeedanceMediaFile(7, header, "document")
	require.ErrorContains(t, err, "unsupported Seedance media kind")
}
