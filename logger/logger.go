package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

const (
	loggerINFO  = "INFO"
	loggerWarn  = "WARN"
	loggerError = "ERR"
	loggerDebug = "DEBUG"
)

var setupLogLock sync.Mutex
var dailyRotationOnce sync.Once
var currentLogPath string
var currentLogPathMu sync.RWMutex
var currentLogFile *os.File

// Gin captures its writers when its logging middleware is created. Keep these
// writer instances stable and rotate only the file they write to.
var stdoutDailyWriter = &dailyLogWriter{console: os.Stdout}
var stderrDailyWriter = &dailyLogWriter{console: os.Stderr}

type dailyLogWriter struct {
	console io.Writer
}

func (w *dailyLogWriter) Write(p []byte) (int, error) {
	ensureLoggerForDate(time.Now())

	// Keep the read lock for the whole file write so a rotation cannot close
	// the file while a captured Gin writer is still using it.
	currentLogPathMu.RLock()
	defer currentLogPathMu.RUnlock()

	n, err := w.console.Write(p)
	if err != nil {
		return n, err
	}
	if n != len(p) {
		return n, io.ErrShortWrite
	}
	if currentLogFile == nil {
		return n, nil
	}
	n, err = currentLogFile.Write(p)
	if err != nil {
		return n, err
	}
	if n != len(p) {
		return n, io.ErrShortWrite
	}
	return n, nil
}

func GetCurrentLogPath() string {
	currentLogPathMu.RLock()
	defer currentLogPathMu.RUnlock()
	return currentLogPath
}

func logPathForDate(logDir string, now time.Time) string {
	return filepath.Join(logDir, fmt.Sprintf("oneapi-%s.log", now.Format("20060102")))
}

func SetupLogger() {
	if common.LogDir == nil || *common.LogDir == "" {
		return
	}
	installDailyWriters()
	if err := setupLoggerAt(time.Now()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to open daily log file: %v\n", err)
		return
	}
	startDailyRotation()
}

func installDailyWriters() {
	common.LogWriterMu.Lock()
	gin.DefaultWriter = stdoutDailyWriter
	gin.DefaultErrorWriter = stderrDailyWriter
	common.LogWriterMu.Unlock()
}

func setupLoggerAt(now time.Time) error {
	if common.LogDir == nil || *common.LogDir == "" {
		return nil
	}

	setupLogLock.Lock()
	defer setupLogLock.Unlock()

	logPath := logPathForDate(*common.LogDir, now)
	currentLogPathMu.RLock()
	alreadyActive := currentLogPath == logPath && currentLogFile != nil
	currentLogPathMu.RUnlock()
	if alreadyActive {
		return nil
	}

	fd, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", logPath, err)
	}

	currentLogPathMu.Lock()
	oldFile := currentLogFile
	currentLogPath = logPath
	currentLogFile = fd
	if oldFile != nil {
		_ = oldFile.Close()
	}
	currentLogPathMu.Unlock()
	return nil
}

func startDailyRotation() {
	dailyRotationOnce.Do(func() {
		go func() {
			for {
				now := time.Now()
				nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
				timer := time.NewTimer(time.Until(nextMidnight))
				<-timer.C
				if err := setupLoggerAt(time.Now()); err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "failed to rotate daily log file: %v\n", err)
				}
			}
		}()
	})
}

func ensureLoggerForDate(now time.Time) {
	if common.LogDir == nil || *common.LogDir == "" {
		return
	}
	logPath := logPathForDate(*common.LogDir, now)
	currentLogPathMu.RLock()
	alreadyActive := currentLogPath == logPath && currentLogFile != nil
	currentLogPathMu.RUnlock()
	if alreadyActive {
		return
	}
	if err := setupLoggerAt(now); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to rotate daily log file: %v\n", err)
	}
}

func LogInfo(ctx context.Context, msg string) {
	logHelper(ctx, loggerINFO, msg)
}

func LogWarn(ctx context.Context, msg string) {
	logHelper(ctx, loggerWarn, msg)
}

func LogError(ctx context.Context, msg string) {
	logHelper(ctx, loggerError, msg)
}

func LogDebug(ctx context.Context, msg string, args ...any) {
	if common.DebugEnabled {
		if len(args) > 0 {
			msg = fmt.Sprintf(msg, args...)
		}
		logHelper(ctx, loggerDebug, msg)
	}
}

func logHelper(ctx context.Context, level string, msg string) {
	var id any = "SYSTEM"
	if ctx != nil {
		if requestID := ctx.Value(common.RequestIdKey); requestID != nil {
			id = requestID
		}
	}
	now := time.Now()
	common.LogWriterMu.RLock()
	writer := gin.DefaultErrorWriter
	if level == loggerINFO {
		writer = gin.DefaultWriter
	}
	_, _ = fmt.Fprintf(writer, "[%s] %v | %s | %s \n", level, now.Format("2006/01/02 - 15:04:05"), id, msg)
	common.LogWriterMu.RUnlock()
}

func LogQuota(quota int) string {
	// 新逻辑：根据额度展示类型输出
	q := float64(quota)
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		usd := q / common.QuotaPerUnit
		cny := usd * operation_setting.USDExchangeRate
		return fmt.Sprintf("¥%.6f 额度", cny)
	case operation_setting.QuotaDisplayTypeCustom:
		usd := q / common.QuotaPerUnit
		rate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
		symbol := operation_setting.GetGeneralSetting().CustomCurrencySymbol
		if symbol == "" {
			symbol = "¤"
		}
		if rate <= 0 {
			rate = 1
		}
		v := usd * rate
		return fmt.Sprintf("%s%.6f 额度", symbol, v)
	case operation_setting.QuotaDisplayTypeTokens:
		return fmt.Sprintf("%d 点额度", quota)
	default: // USD
		return fmt.Sprintf("＄%.6f 额度", q/common.QuotaPerUnit)
	}
}

func FormatQuota(quota int) string {
	q := float64(quota)
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		usd := q / common.QuotaPerUnit
		cny := usd * operation_setting.USDExchangeRate
		return fmt.Sprintf("¥%.6f", cny)
	case operation_setting.QuotaDisplayTypeCustom:
		usd := q / common.QuotaPerUnit
		rate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
		symbol := operation_setting.GetGeneralSetting().CustomCurrencySymbol
		if symbol == "" {
			symbol = "¤"
		}
		if rate <= 0 {
			rate = 1
		}
		v := usd * rate
		return fmt.Sprintf("%s%.6f", symbol, v)
	case operation_setting.QuotaDisplayTypeTokens:
		return fmt.Sprintf("%d", quota)
	default:
		return fmt.Sprintf("＄%.6f", q/common.QuotaPerUnit)
	}
}

// LogJson 仅供测试使用 only for test
func LogJson(ctx context.Context, msg string, obj any) {
	if !common.DebugEnabled {
		return
	}
	jsonStr, err := common.Marshal(obj)
	if err != nil {
		LogError(ctx, fmt.Sprintf("json marshal failed: %s", err.Error()))
		return
	}
	LogDebug(ctx, "%s | %s", msg, jsonStr)
}
