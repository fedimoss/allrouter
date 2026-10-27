package controller

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func questionnaireUploadRequest(t *testing.T, content []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "sample.png")
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("domain", "localhost"))
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/questionnaire/upload", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return ctx, recorder
}

func decodeQuestionnaireUploadURL(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, "上传应成功: %s", recorder.Body.String())
	require.NotEmpty(t, response.Data.URL)
	return response.Data.URL
}

func TestUploadQuestionnaireFileIsIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := pngBytes(t)

	restore := chdirTemp(t)
	defer restore()

	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.UsingSQLite = true
	t.Cleanup(func() {
		model.DB = previousDB
	})

	firstCtx, firstRecorder := questionnaireUploadRequest(t, content)
	UploadQuestionnaireFile(firstCtx)
	require.Equal(t, http.StatusOK, firstRecorder.Code)
	firstURL := decodeQuestionnaireUploadURL(t, firstRecorder)

	secondCtx, secondRecorder := questionnaireUploadRequest(t, content)
	UploadQuestionnaireFile(secondCtx)
	require.Equal(t, http.StatusOK, secondRecorder.Code)
	secondURL := decodeQuestionnaireUploadURL(t, secondRecorder)

	require.Equal(t, firstURL, secondURL)
	storedPath := filepath.Join(".", filepath.FromSlash(strings.TrimPrefix(firstURL, "/")))
	require.FileExists(t, storedPath)

	tmpEntries, err := os.ReadDir(filepath.Join("static", "questionnaire", "0", "0", "tmp"))
	require.NoError(t, err)
	require.Empty(t, tmpEntries, "重复上传不应在 tmp 目录残留文件")
}
