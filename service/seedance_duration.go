package service

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var (
	seedanceFFprobeOnce sync.Once
	seedanceFFprobePath string
)

// InitSeedanceDurationProbe detects ffprobe once at startup. Missing ffprobe is
// non-fatal: duration validation is skipped and the upstream remains
// authoritative.
// 注意：ffprobe 只是本地预校验；缺失或探测失败时必须放行，不能阻断任务。
func InitSeedanceDurationProbe() {
	seedanceFFprobeOnce.Do(func() {
		path, err := exec.LookPath("ffprobe")
		if err != nil {
			common.SysLog("ffprobe not found; Seedance reference media duration validation will be skipped")
			return
		}
		seedanceFFprobePath = path
		common.SysLog("ffprobe found for Seedance duration validation: " + path)
	})
}

func SeedanceDurationProbeAvailable() bool {
	InitSeedanceDurationProbe()
	return seedanceFFprobePath != ""
}

// ProbeSeedanceMediaDuration reads the container-level duration of a stored
// reference media file. Callers treat errors as "unknown duration" and allow
// the upstream to perform the final validation.
func ProbeSeedanceMediaDuration(ctx context.Context, userId int, mediaID string) (float64, error) {
	if !SeedanceDurationProbeAvailable() {
		return 0, fmt.Errorf("ffprobe is not available")
	}
	mediaPath, err := seedanceMediaPath(userId, mediaID)
	if err != nil {
		return 0, err
	}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// 只读取容器级 duration；单文件探测失败由调用方记录警告并放行。
	cmd := exec.CommandContext(
		probeCtx,
		seedanceFFprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		mediaPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, fmt.Errorf("parse ffprobe duration: %w", err)
	}
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 0, fmt.Errorf("ffprobe returned invalid duration")
	}
	return duration, nil
}

func seedanceMediaPath(userId int, mediaID string) (string, error) {
	// 安全：mediaID 只能是不含路径分隔符的纯文件名，防止目录穿越。
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" || filepath.Base(mediaID) != mediaID || strings.ContainsAny(mediaID, `/\`) {
		return "", fmt.Errorf("invalid Seedance media id")
	}
	if !seedanceMediaExtensions[strings.ToLower(filepath.Ext(mediaID))] {
		return "", fmt.Errorf("invalid Seedance media id")
	}
	userDir, err := miniMaxH3FrameUserDir(userId)
	if err != nil {
		return "", err
	}
	return filepath.Join(userDir, mediaID), nil
}
