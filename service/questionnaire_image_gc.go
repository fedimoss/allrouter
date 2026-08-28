package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	questionnaireImageGCTickInterval = 1 * time.Hour  // 扫描周期
	questionnaireImageGCGrace        = 24 * time.Hour // 宽限期：保护填写中的表单与刚上传的截图
	questionnaireImageGCBatchSize    = 1000
)

var questionnaireImageGCOnce sync.Once

// questionnaireScreenshotURLPattern survey_data 文本中的截图 URL：
// /static/questionnaire/{providerId}/{userId}/{sha256}{ext}。
// 直接在原始 JSON 文本上匹配，不依赖 screenshots 字段的嵌套结构。
var questionnaireScreenshotURLPattern = regexp.MustCompile(
	`/static/questionnaire/(\d+)/(\d+)/([0-9a-f]{64})\.(jpg|png|gif|webp)`)

// questionnaireImageNamePattern 落盘文件名：64 位内容哈希 + 白名单扩展名
var questionnaireImageNamePattern = regexp.MustCompile(
	`^([0-9a-f]{64})\.(jpg|png|gif|webp)$`)

// StartQuestionnaireImageGCTask 启动问卷截图孤儿文件清理任务。
// "孤儿"定义：未被任何问卷记录 survey_data 引用、且超过宽限期的文件，包括：
//   - 上传后从未提交的截图（用户放弃表单）；
//   - 问卷记录被后台删除后遗留的截图；
//   - 上传中断/进程崩溃残留在 tmp 目录的临时文件。
//
// 已被引用的文件只要对应记录还在就永不删除，后台回显不受影响；
// 宽限期内的文件一律跳过，避免误删正在填写的表单刚传的图。
func StartQuestionnaireImageGCTask() {
	questionnaireImageGCOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"questionnaire image gc task started: tick=%s grace=%s",
				questionnaireImageGCTickInterval, questionnaireImageGCGrace))
			ticker := time.NewTicker(questionnaireImageGCTickInterval)
			defer ticker.Stop()

			runQuestionnaireImageGCOnce()
			for range ticker.C {
				runQuestionnaireImageGCOnce()
			}
		})
	})
}

func runQuestionnaireImageGCOnce() {
	// 1. 收集所有问卷记录引用的截图相对路径（providerId/userId/hash.ext）
	referenced := make(map[string]struct{})
	err := model.IterateUserQuestionnaireSurveyData(questionnaireImageGCBatchSize,
		func(surveyData string) {
			for _, m := range questionnaireScreenshotURLPattern.FindAllStringSubmatch(surveyData, -1) {
				key := m[1] + "/" + m[2] + "/" + m[3] + "." + m[4]
				referenced[key] = struct{}{}
			}
		})
	if err != nil {
		logger.LogError(context.Background(),
			"questionnaire image gc: scan survey_data failed: "+err.Error())
		return
	}

	// 2. 扫描目录，删除未引用且超过宽限期的文件
	root := filepath.Join("static", "questionnaire")
	cutoff := time.Now().Add(-questionnaireImageGCGrace)
	var deleted, freed int64
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// 根目录不存在：无文件可清理，直接结束；单个条目读不了跳过，不中断整轮
			if path == root {
				return filepath.SkipAll
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().After(cutoff) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		// 只动两类文件，其他一概不碰：
		//   providerId/userId/tmp/xxx —— 超过宽限期的上传残留
		//   providerId/userId/hash.ext —— 哈希命名且未被任何记录引用
		shouldDelete := false
		switch {
		case len(parts) == 4 && parts[2] == "tmp":
			shouldDelete = true
		case len(parts) == 3 && questionnaireImageNamePattern.MatchString(parts[2]):
			_, referencedYet := referenced[rel]
			shouldDelete = !referencedYet
		}
		if shouldDelete && os.Remove(path) == nil {
			deleted++
			freed += info.Size()
		}
		return nil
	})
	if err != nil {
		logger.LogError(context.Background(),
			"questionnaire image gc: walk files failed: "+err.Error())
	}
	if deleted > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf(
			"questionnaire image gc: removed %d orphan files, freed %d bytes", deleted, freed))
	}
}
