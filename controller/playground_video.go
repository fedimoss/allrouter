package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func playgroundVideoUploadError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"message": message,
			"type":    "invalid_request_error",
		},
	})
}

// UploadPlaygroundVideoFrames stores optional MiniMax-H3 keyframes for the logged-in user.
func UploadPlaygroundVideoFrames(c *gin.Context) {
	if c.GetBool("use_access_token") {
		playgroundVideoUploadError(c, http.StatusForbidden, "Playground frame uploads require a login session")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*service.MiniMaxH3FrameMaxBytes+(1<<20))

	firstFrame, firstErr := c.FormFile("first_frame")
	lastFrame, lastErr := c.FormFile("last_frame")
	if firstErr != nil && !errors.Is(firstErr, http.ErrMissingFile) {
		playgroundVideoUploadError(c, http.StatusBadRequest, "failed to read first frame")
		return
	}
	if lastErr != nil && !errors.Is(lastErr, http.ErrMissingFile) {
		playgroundVideoUploadError(c, http.StatusBadRequest, "failed to read last frame")
		return
	}
	if firstFrame == nil && lastFrame == nil {
		playgroundVideoUploadError(c, http.StatusBadRequest, "at least one frame image is required")
		return
	}

	userId := c.GetInt("id")
	result := gin.H{}
	if firstFrame != nil {
		frameID, err := service.SaveMiniMaxH3Frame(userId, firstFrame)
		if err != nil {
			playgroundVideoUploadError(c, http.StatusBadRequest, err.Error())
			return
		}
		result["first_frame_id"] = frameID
	}
	if lastFrame != nil {
		frameID, err := service.SaveMiniMaxH3Frame(userId, lastFrame)
		if err != nil {
			playgroundVideoUploadError(c, http.StatusBadRequest, err.Error())
			return
		}
		result["last_frame_id"] = frameID
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
