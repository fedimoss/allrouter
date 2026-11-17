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
// ffprobe 只是本地预校验；缺失或探测失败时必须放行，不能阻断任务。
// 上游仍会做最终校验，因此这里不能把探测失败当成用户输入错误。
func InitSeedanceDurationProbe() {
	seedanceFFprobeOnce.Do(func() {
		path, err := exec.LookPath("ffprobe")
		if err != nil {
			// 找不到 ffprobe 只记录日志，不阻断服务启动。
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
		// 不可用时返回明确错误，由调用方决定是否放行。
		return 0, fmt.Errorf("ffprobe is not available")
	}
	mediaPath, err := seedanceMediaPath(userId, mediaID)
	if err != nil {
		// mediaID 非法时不能执行外部命令。
		return 0, err
	}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// 只读取容器级 duration；单文件探测失败由调用方记录警告并放行。
	// 不能因为某个文件读不出时长就拒绝整个请求。
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
		// 把 ffprobe stderr 带出来，便于排查编码/文件损坏问题。
		return 0, fmt.Errorf("ffprobe failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		// 输出不是合法浮点数时按未知时长处理。
		return 0, fmt.Errorf("parse ffprobe duration: %w", err)
	}
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		// 非正数/NaN/Inf 都不是有效时长。
		return 0, fmt.Errorf("ffprobe returned invalid duration")
	}
	return duration, nil
}

func seedanceMediaPath(userId int, mediaID string) (string, error) {
	// mediaID 只能是不含路径分隔符的纯文件名，防止目录穿越。
	// 不能放宽这段校验，否则删除/读取可能越出用户目录。
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
