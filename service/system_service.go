package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vvb13a/godam/domain"
	"github.com/vvb13a/godam/store"
	"gorm.io/gorm"
)

// SystemService exposes deployment-level health and metrics for a fleet
// console. It deliberately exposes no asset contents, storage credentials, or
// user data.
type SystemService struct {
	db          *gorm.DB
	dbPath      string
	storagePath string
	version     string
	started     time.Time
	libraries   *LibraryService
	backups     *BackupService

	mu                sync.Mutex
	previewBytesCache int64
	previewAt         time.Time
}

func NewSystemService(db *gorm.DB, dbPath, storagePath, version string, libraries *LibraryService, backups *BackupService) *SystemService {
	return &SystemService{
		db:          db,
		dbPath:      dbPath,
		storagePath: storagePath,
		version:     version,
		started:     time.Now().UTC(),
		libraries:   libraries,
		backups:     backups,
	}
}

type SystemLibrary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SystemInfo struct {
	Version       string          `json:"version"`
	SchemaVersion int64           `json:"schema_version"`
	StartedAt     time.Time       `json:"started_at"`
	UptimeSeconds int64           `json:"uptime_seconds"`
	Libraries     []SystemLibrary `json:"libraries"`
}

type LibraryMetrics struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Assets        int64  `json:"assets"`
	OriginalBytes int64  `json:"original_bytes"`
}

type StorageMetrics struct {
	OriginalBytes int64 `json:"original_bytes"`
	PreviewBytes  int64 `json:"preview_bytes"`
	DatabaseBytes int64 `json:"database_bytes"`
	WALBytes      int64 `json:"wal_bytes"`
}

type JobMetrics struct {
	Total    int64 `json:"total"`
	Pending  int64 `json:"pending"`
	Retrying int64 `json:"retrying"`
}

type ProcessMetrics struct {
	MemoryAllocBytes uint64 `json:"memory_alloc_bytes"`
	MemorySysBytes   uint64 `json:"memory_sys_bytes"`
	Goroutines       int    `json:"goroutines"`
	GCCycles         uint32 `json:"gc_cycles"`
}

type ContainerMetrics struct {
	MemoryCurrentBytes *int64   `json:"memory_current_bytes,omitempty"`
	MemoryLimitBytes   *int64   `json:"memory_limit_bytes,omitempty"`
	CPUUsageSeconds    *float64 `json:"cpu_usage_seconds,omitempty"`
}

type SystemMetrics struct {
	Version       string                `json:"version"`
	SchemaVersion int64                 `json:"schema_version"`
	UptimeSeconds int64                 `json:"uptime_seconds"`
	Libraries     []LibraryMetrics      `json:"libraries"`
	Storage       StorageMetrics        `json:"storage"`
	Jobs          JobMetrics            `json:"jobs"`
	Backups       []domain.BackupStatus `json:"backups"`
	Process       ProcessMetrics        `json:"process"`
	Container     ContainerMetrics      `json:"container"`
}

func (s *SystemService) Info(ctx context.Context) (*SystemInfo, error) {
	var libraries []store.Library
	if err := s.db.WithContext(ctx).Order("sort ASC, created_at ASC").Find(&libraries).Error; err != nil {
		return nil, err
	}
	out := &SystemInfo{
		Version:       s.version,
		SchemaVersion: s.schemaVersion(ctx),
		StartedAt:     s.started,
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
		Libraries:     make([]SystemLibrary, 0, len(libraries)),
	}
	for _, lib := range libraries {
		out.Libraries = append(out.Libraries, SystemLibrary{ID: lib.ID, Name: lib.Name})
	}
	return out, nil
}

func (s *SystemService) Metrics(ctx context.Context) (*SystemMetrics, error) {
	var libraries []store.Library
	if err := s.db.WithContext(ctx).Order("sort ASC, created_at ASC").Find(&libraries).Error; err != nil {
		return nil, err
	}
	out := &SystemMetrics{
		Version:       s.version,
		SchemaVersion: s.schemaVersion(ctx),
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
		Libraries:     make([]LibraryMetrics, 0, len(libraries)),
		Backups:       make([]domain.BackupStatus, 0, len(libraries)),
	}
	for _, lib := range libraries {
		var row struct {
			Assets int64
			Bytes  int64
		}
		_ = s.db.WithContext(ctx).
			Model(&store.Asset{}).
			Select("COUNT(*) as assets, COALESCE(SUM(byte_size), 0) as bytes").
			Where("library_id = ?", lib.ID).
			Scan(&row).Error
		out.Libraries = append(out.Libraries, LibraryMetrics{
			ID:            lib.ID,
			Name:          lib.Name,
			Assets:        row.Assets,
			OriginalBytes: row.Bytes,
		})
		out.Storage.OriginalBytes += row.Bytes
		if s.backups != nil {
			if status, err := s.backups.Status(ctx, lib.ID); err == nil && status != nil {
				out.Backups = append(out.Backups, *status)
			}
		}
	}

	out.Storage.PreviewBytes = s.previewBytes()
	s.db.WithContext(ctx).Model(&store.StorageJob{}).Count(&out.Jobs.Total)
	s.db.WithContext(ctx).Model(&store.StorageJob{}).Where("attempts = 0").Count(&out.Jobs.Pending)
	s.db.WithContext(ctx).Model(&store.StorageJob{}).Where("attempts > 0").Count(&out.Jobs.Retrying)

	if info, err := os.Stat(s.dbPath); err == nil {
		out.Storage.DatabaseBytes = info.Size()
	}
	if info, err := os.Stat(s.dbPath + "-wal"); err == nil {
		out.Storage.WALBytes = info.Size()
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	out.Process = ProcessMetrics{
		MemoryAllocBytes: mem.Alloc,
		MemorySysBytes:   mem.Sys,
		Goroutines:       runtime.NumGoroutine(),
		GCCycles:         mem.NumGC,
	}
	out.Container = containerMetrics()
	return out, nil
}

func (s *SystemService) schemaVersion(ctx context.Context) int64 {
	var version int64
	_ = s.db.WithContext(ctx).
		Raw("SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version").
		Scan(&version).Error
	return version
}

func (s *SystemService) previewBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.previewAt) < time.Minute {
		return s.previewBytesCache
	}
	var total int64
	root := filepath.Join(s.storagePath, "previews")
	_ = filepath.WalkDir(root, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if info, err := entry.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	s.previewBytesCache = total
	s.previewAt = time.Now()
	return total
}

func containerMetrics() ContainerMetrics {
	var out ContainerMetrics
	if value, ok := readIntFile("/sys/fs/cgroup/memory.current"); ok {
		out.MemoryCurrentBytes = &value
	} else if value, ok := readIntFile("/sys/fs/cgroup/memory/memory.usage_in_bytes"); ok {
		out.MemoryCurrentBytes = &value
	}
	if value, ok := readIntFile("/sys/fs/cgroup/memory.max"); ok {
		out.MemoryLimitBytes = &value
	} else if value, ok := readIntFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok {
		out.MemoryLimitBytes = &value
	}
	if seconds, ok := readCPUUsageSeconds(); ok {
		out.CPUUsageSeconds = &seconds
	}
	return out
}

func readIntFile(path string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "max" {
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 || value >= 1<<60 {
		return 0, false
	}
	return value, true
}

func readCPUUsageSeconds() (float64, bool) {
	if data, err := os.ReadFile("/sys/fs/cgroup/cpu.stat"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "usage_usec" {
				if value, err := strconv.ParseFloat(fields[1], 64); err == nil {
					return value / 1_000_000, true
				}
			}
		}
	}
	if value, ok := readIntFile("/sys/fs/cgroup/cpuacct/cpuacct.usage"); ok {
		return float64(value) / 1_000_000_000, true
	}
	return 0, false
}
