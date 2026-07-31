package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

// Responses→Chat 兼容转换的诊断 dump 工具。
//
// 开启后，每个 Responses→Chat 请求会把四个转换点的原始数据
// 写入磁盘文件：{dumpDir}/{request_id}.log
//
// 四个转换点（对应一次请求的完整数据流）：
//  1. request_before  - response→chat 前的请求体（客户端发来的原始 Responses 请求）
//  2. request_after   - response→chat 后的请求体（转换后的 Chat Completions 请求，即发给上游的）
//  3. response_before  - chat→response 前的响应体（上游返回的 Chat Completions 响应）
//  4. response_after   - chat→response 后的响应体（转换后的 Responses 响应，即发给客户端的）
//
// 输出目录（可选，默认 {LogDir}/responses-chat-compat-dump/，LogDir 为空时用 ./responses-chat-compat-dump）：
//   - RESPONSES_CHAT_COMPAT_DUMP_DIR=/path/to/dir
//
// 注意：无开关，只要请求走 Responses→Chat 路径就会写盘，仅供测试排查使用。

const responsesCompatDumpBufKey = "responses_compat_dump_buf"

// DumpField 标识四个转换点之一。
const (
	ResponsesCompatDumpRequestBefore  = "request_before"
	ResponsesCompatDumpRequestAfter   = "request_after"
	ResponsesCompatDumpResponseBefore = "response_before"
	ResponsesCompatDumpResponseAfter  = "response_after"
)

// responsesCompatDumpBuf 在单次请求内累积四个转换点的数据。
// 挂在 gin.Context 上，因此只覆盖 Responses→Chat 这一条路径。
type responsesCompatDumpBuf struct {
	mu             sync.Mutex
	requestID      string
	model          string
	requestBefore  []byte
	requestAfter   []byte
	responseBefore []byte
	responseAfter  []byte
}

// responsesCompatDumpDir 解析输出目录：优先环境变量，其次 LogDir 子目录，最后当前目录。
func responsesCompatDumpDir() string {
	if dir := os.Getenv("RESPONSES_CHAT_COMPAT_DUMP_DIR"); dir != "" {
		return dir
	}
	if common.LogDir != nil && *common.LogDir != "" {
		return filepath.Join(*common.LogDir, "responses-chat-compat-dump")
	}
	return "responses-chat-compat-dump"
}

func getResponsesCompatDumpBuf(c *gin.Context) *responsesCompatDumpBuf {
	v, ok := c.Get(responsesCompatDumpBufKey)
	if !ok || v == nil {
		return nil
	}
	buf, _ := v.(*responsesCompatDumpBuf)
	return buf
}

// BeginResponsesCompatDump 在 Responses→Chat 路径入口初始化 dump 缓冲。
// 无开关，只要进入此路径即生效；同一请求重复调用安全。
func BeginResponsesCompatDump(c *gin.Context, info *relaycommon.RelayInfo) {
	if getResponsesCompatDumpBuf(c) != nil {
		return
	}
	reqID := "SYSTEM"
	if v := c.Value(common.RequestIdKey); v != nil {
		reqID = fmt.Sprintf("%v", v)
	}
	model := ""
	if info != nil {
		model = info.OriginModelName
	}
	c.Set(responsesCompatDumpBufKey, &responsesCompatDumpBuf{
		requestID: reqID,
		model:     model,
	})
}

// DumpResponsesCompatSection 把一段数据追加到指定转换点的缓冲。
// field 取 ResponsesCompatDump* 常量；可对同一 field 多次调用（流式逐块累积）。
// 若当前请求未初始化 dump（非 Responses→Chat 路径或开关关闭），则直接返回。
func DumpResponsesCompatSection(c *gin.Context, field string, body []byte) {
	buf := getResponsesCompatDumpBuf(c)
	if buf == nil {
		return
	}
	buf.mu.Lock()
	defer buf.mu.Unlock()
	switch field {
	case ResponsesCompatDumpRequestBefore:
		buf.requestBefore = append(buf.requestBefore, body...)
	case ResponsesCompatDumpRequestAfter:
		buf.requestAfter = append(buf.requestAfter, body...)
	case ResponsesCompatDumpResponseBefore:
		buf.responseBefore = append(buf.responseBefore, body...)
	case ResponsesCompatDumpResponseAfter:
		buf.responseAfter = append(buf.responseAfter, body...)
	}
}

// FlushResponsesCompatDump 把累积的四个转换点数据写入磁盘文件。
// 应在 Responses→Chat 处理结束时（defer）调用；多次调用安全（第二次起为空操作）。
func FlushResponsesCompatDump(c *gin.Context) {
	buf := getResponsesCompatDumpBuf(c)
	if buf == nil {
		return
	}
	c.Set(responsesCompatDumpBufKey, nil)

	buf.mu.Lock()
	requestID := buf.requestID
	model := buf.model
	requestBefore := buf.requestBefore
	requestAfter := buf.requestAfter
	responseBefore := buf.responseBefore
	responseAfter := buf.responseAfter
	buf.mu.Unlock()

	dir := responsesCompatDumpDir()
	// MkdirAll 幂等且并发安全；每个请求写独立文件，无需全局锁。
	if err := os.MkdirAll(dir, 0777); err != nil {
		logger.LogWarn(c, fmt.Sprintf("responses compat dump: failed to create dir %q: %s", dir, err.Error()))
		return
	}
	path := filepath.Join(dir, sanitizeDumpFilename(requestID)+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		logger.LogWarn(c, fmt.Sprintf("responses compat dump: failed to open file %q: %s", path, err.Error()))
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "time=%s request_id=%s model=%q dir=%s\n", time.Now().Format("2006/01/02 - 15:04:05"), requestID, model, dir)
	fmt.Fprintln(f, strings.Repeat("=", 80))

	writeDumpSection(f, "[1] response→chat 前请求体 (原始 Responses 请求)", requestBefore)
	writeDumpSection(f, "[2] response→chat 后请求体 (转换后 Chat Completions, 发往上游)", requestAfter)
	writeDumpSection(f, "[3] chat→response 前响应体 (上游 Chat Completions 响应)", responseBefore)
	writeDumpSection(f, "[4] chat→response 后响应体 (转换后 Responses, 发往客户端)", responseAfter)
}

// writeDumpSection 写入一个转换点的标题与内容。
func writeDumpSection(f *os.File, label string, body []byte) {
	fmt.Fprintf(f, "\n%s (len=%d)\n", label, len(body))
	if len(body) > 0 {
		f.Write(body)
		if body[len(body)-1] != '\n' {
			fmt.Fprintln(f)
		}
	}
	fmt.Fprintln(f, strings.Repeat("-", 80))
}

// sanitizeDumpFilename 把 request_id 中的路径分隔符替换为安全字符，避免目录穿越。
func sanitizeDumpFilename(name string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return r.Replace(name)
}
