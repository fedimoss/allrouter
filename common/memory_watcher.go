package common

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultMemoryWatcherIntervalSeconds = 10
	defaultMemoryThresholdMB            = 4096
	defaultDumpCooldownSeconds          = 60
	defaultRetentionMinutes             = 1440
	defaultMaxDumpBatches               = 5

	SaveDir = "/data/pprof"
)

var (
	lastDumpTime time.Time
	mu           sync.Mutex
)

// StartMemoryWatcher monitors RSS and saves pprof files when memory exceeds the threshold.
func StartMemoryWatcher() {
	go func() {
		interval := time.Duration(
			getPositiveEnvInt("MEMORY_WATCHER_INTERVAL_SECONDS", defaultMemoryWatcherIntervalSeconds),
		) * time.Second

		threshold := getPositiveEnvInt("MEMORY_WATCHER_THRESHOLD_MB", defaultMemoryThresholdMB)
		retention := getPositiveEnvInt("MEMORY_RETENTION_MINUTES", defaultRetentionMinutes)
		maxDumpBatches := getPositiveEnvInt("MEMORY_MAX_DUMP_BATCHES", defaultMaxDumpBatches)

		if err := os.MkdirAll(SaveDir, 0755); err != nil {
			log.Println("[MemoryWatcher] create dir error:", err)
			return
		}

		log.Printf("[MemoryWatcher] start: interval=%s threshold=%dMB retention=%dmin max_batches=%d dir=%s",
			interval, threshold, retention, maxDumpBatches, SaveDir)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			rssMB := getRSSMB()

			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			log.Printf("[MemoryWatcher] RSS=%dMB HeapAlloc=%dMB HeapSys=%dMB Goroutines=%d",
				rssMB,
				m.HeapAlloc/1024/1024,
				m.HeapSys/1024/1024,
				runtime.NumGoroutine(),
			)

			if rssMB >= threshold && canDump() {
				log.Printf("[MemoryWatcher] trigger dump, RSS=%dMB", rssMB)
				dumpAll()
			}

			cleanupOldFiles(retention, maxDumpBatches)
		}
	}()
}

// canDump prevents too many dumps from being written during a long high-memory period.
func canDump() bool {
	cooldown := time.Duration(
		getPositiveEnvInt("MEMORY_DUMP_COOLDOWN_SECONDS", defaultDumpCooldownSeconds),
	) * time.Second

	mu.Lock()
	defer mu.Unlock()

	now := time.Now()
	if now.Sub(lastDumpTime) < cooldown {
		return false
	}

	lastDumpTime = now
	return true
}

func dumpAll() {
	ts := time.Now().Format("20060102_150405")

	saveHeap(ts)
	saveGoroutine(ts)
	saveMemStats(ts)
}

func saveHeap(ts string) {
	file := filepath.Join(SaveDir, fmt.Sprintf("heap_%s.pb.gz", ts))

	f, err := os.Create(file)
	if err != nil {
		log.Println("[heap dump error]", err)
		return
	}
	defer f.Close()

	runtime.GC()

	if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
		log.Println("[heap write error]", err)
	}
}

func saveGoroutine(ts string) {
	file := filepath.Join(SaveDir, fmt.Sprintf("goroutine_%s.txt", ts))

	f, err := os.Create(file)
	if err != nil {
		log.Println("[goroutine dump error]", err)
		return
	}
	defer f.Close()

	_ = pprof.Lookup("goroutine").WriteTo(f, 2)
}

func saveMemStats(ts string) {
	file := filepath.Join(SaveDir, fmt.Sprintf("memstats_%s.json", ts))

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	b, err := Marshal(m)
	if err != nil {
		log.Println("[memstats marshal error]", err)
		return
	}

	_ = os.WriteFile(file, b, 0644)
}

type memoryDumpFile struct {
	path string
}

type memoryDumpBatch struct {
	modTime time.Time
	files   []memoryDumpFile
}

func cleanupOldFiles(retentionMinutes int, maxDumpBatches int) {
	files, err := os.ReadDir(SaveDir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-time.Duration(retentionMinutes) * time.Minute)
	batches := make(map[string]*memoryDumpBatch)

	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}

		path := filepath.Join(SaveDir, f.Name())
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
			continue
		}

		batchID, ok := memoryDumpBatchID(f.Name())
		if !ok {
			continue
		}

		batch, ok := batches[batchID]
		if !ok {
			batch = &memoryDumpBatch{}
			batches[batchID] = batch
		}
		if info.ModTime().After(batch.modTime) {
			batch.modTime = info.ModTime()
		}
		batch.files = append(batch.files, memoryDumpFile{
			path: path,
		})
	}

	if len(batches) <= maxDumpBatches {
		return
	}

	sortedBatches := make([]memoryDumpBatch, 0, len(batches))
	for _, batch := range batches {
		sortedBatches = append(sortedBatches, *batch)
	}

	sort.Slice(sortedBatches, func(i, j int) bool {
		return sortedBatches[i].modTime.After(sortedBatches[j].modTime)
	})

	for _, batch := range sortedBatches[maxDumpBatches:] {
		for _, file := range batch.files {
			_ = os.Remove(file.path)
		}
	}
}

func memoryDumpBatchID(name string) (string, bool) {
	for _, prefix := range []string{"heap_", "goroutine_", "memstats_"} {
		if strings.HasPrefix(name, prefix) {
			rest := strings.TrimPrefix(name, prefix)
			dot := strings.Index(rest, ".")
			if dot <= 0 {
				return "", false
			}
			return rest[:dot], true
		}
	}
	return "", false
}

func getRSSMB() int {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.Atoi(fields[1])
				return kb / 1024
			}
		}
	}
	return 0
}

func getPositiveEnvInt(env string, def int) int {
	v := GetEnvOrDefault(env, def)
	if v <= 0 {
		log.Printf("[MemoryWatcher] invalid %s=%d use default=%d", env, v, def)
		return def
	}
	return v
}
