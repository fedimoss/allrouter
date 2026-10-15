package controller

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// pngBytes 生成一张最小可用的 PNG，用于走真实的类型判断与魔数嗅探
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// jpegBytes 生成一张最小可用的 JPEG，配合 image/jpg 变体头测试归一化逻辑
func jpegBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// uploadRequest 构造一次 multipart 上传请求。
// 这里用 CreatePart 显式写上 image/png，模拟浏览器选中 PNG 时发出的 part 头
// （writer.CreateFormFile 会固定成 application/octet-stream，与真实请求不符）。
func uploadRequest(t *testing.T, kind string, content []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	return uploadRequestAs(t, kind, "image/png", content)
}

func uploadRequestAs(t *testing.T, kind, contentType string, content []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if kind != "" {
		require.NoError(t, writer.WriteField("type", kind))
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="image"; filename="a.png"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/douyin_card/upload", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return ctx, recorder
}

// TestUploadDouyinCardImageStoresByKindAndUser 校验落盘目录为
// static/card/{用途}/{用户ID}/，并返回对应的 /static 相对 URL。
// 处理函数以工作目录为基准写盘，测试里临时切到 t.TempDir 避免污染仓库。
func TestUploadDouyinCardImageStoresByKindAndUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := pngBytes(t)

	restore := chdirTemp(t)
	defer restore()

	for _, tc := range []struct {
		kind   string
		userId int
	}{
		{"logo", 7},
		{"weixin", 42},
	} {
		ctx, recorder := uploadRequest(t, tc.kind, content)
		ctx.Set("id", tc.userId)

		UploadDouyinCardImage(ctx)

		require.Equal(t, http.StatusOK, recorder.Code)

		var resp struct {
			Success bool `json:"success"`
			Data    struct {
				URL string `json:"url"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
		require.True(t, resp.Success, "响应应成功: %s", recorder.Body.String())

		prefix := "/static/card/" + tc.kind + "/" + strconv.Itoa(tc.userId) + "/"
		require.True(t, strings.HasPrefix(resp.Data.URL, prefix),
			"URL 应以 %s 开头，实际 %s", prefix, resp.Data.URL)

		// URL 去掉开头的 / 就是磁盘相对路径（/static/card/... → static/card/...），
		// 应确实存在且内容一致
		stored := filepath.Join(".", filepath.FromSlash(strings.TrimPrefix(resp.Data.URL, "/")))
		got, err := os.ReadFile(stored)
		require.NoError(t, err, "落盘文件应存在: %s", stored)
		require.Equal(t, content, got)

		// tmp 目录不应残留文件（重命名成功后临时文件被移走）
		tmpEntries, _ := os.ReadDir(filepath.Join("static", "card", tc.kind, strconv.Itoa(tc.userId), "tmp"))
		require.Empty(t, tmpEntries, "tmp 目录不应有残留")
	}
}

// TestUploadDouyinCardImageRejectsBadInput 覆盖入参校验：
// 用途不在白名单、缺少用途、非图片内容都应被拒绝，且不产生落盘文件。
func TestUploadDouyinCardImageRejectsBadInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := pngBytes(t)

	restore := chdirTemp(t)
	defer restore()

	t.Run("未知用途", func(t *testing.T) {
		ctx, recorder := uploadRequest(t, "../../etc", content)
		ctx.Set("id", 1)
		UploadDouyinCardImage(ctx)
		require.Contains(t, recorder.Body.String(), "上传类型不正确")
	})

	t.Run("缺少用途", func(t *testing.T) {
		ctx, recorder := uploadRequest(t, "", content)
		ctx.Set("id", 1)
		UploadDouyinCardImage(ctx)
		require.Contains(t, recorder.Body.String(), "上传类型不正确")
	})

	t.Run("非图片内容", func(t *testing.T) {
		ctx, recorder := uploadRequest(t, "logo", []byte("not an image at all"))
		ctx.Set("id", 1)
		UploadDouyinCardImage(ctx)
		require.Contains(t, recorder.Body.String(), "仅支持")
	})

	// 以上请求都不该在 static/card 下留下文件
	err := filepath.Walk("static", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		require.True(t, info.IsDir(), "被拒绝的上传不应产生文件: %s", path)
		return nil
	})
	require.NoError(t, err)
}

// chdirTemp 切换到临时目录并返回恢复函数，让上传落盘与断言都在干净目录里进行
func chdirTemp(t *testing.T) func() {
	t.Helper()
	old, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(t.TempDir()))
	return func() { require.NoError(t, os.Chdir(old)) }
}

// TestUploadDouyinCardImageAcceptsByContent 上传格式只认文件内容的魔数嗅探：
// 客户端声明的 Content-Type（大小写变体、缺失、octet-stream、与内容不符）
// 一概不影响接收；只有内容确实不是图片时才拒绝。
func TestUploadDouyinCardImageAcceptsByContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restore := chdirTemp(t)
	defer restore()

	for _, tc := range []struct {
		name        string
		contentType string
		content     []byte
	}{
		{"声明png", "image/png", pngBytes(t)},
		{"声明png实为JPEG（OIP类名不副实）", "image/png", jpegBytes(t)},
		{"声明octet-stream", "application/octet-stream", pngBytes(t)},
		{"无类型声明", "", pngBytes(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := uploadRequestAs(t, "logo", tc.contentType, tc.content)
			ctx.Set("id", 9)
			UploadDouyinCardImage(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"success":true`)
		})
	}
}

// TestUploadDouyinCardImageSniffsRealFormat 扩展名/声明类型与真实内容不符时，
// 按魔数嗅探出的真实格式落盘扩展名（如 OIP.png 实为 JPEG → 存为 .jpg）。
func TestUploadDouyinCardImageSniffsRealFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restore := chdirTemp(t)
	defer restore()

	ctx, recorder := uploadRequestAs(t, "logo", "image/png", jpegBytes(t))
	ctx.Set("id", 11)
	UploadDouyinCardImage(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Success, "响应应成功: %s", recorder.Body.String())
	require.True(t, strings.HasSuffix(resp.Data.URL, ".jpg"),
		"应按真实格式落盘为 .jpg，实际 %s", resp.Data.URL)
}
