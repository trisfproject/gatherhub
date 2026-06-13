package services

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"gorm.io/gorm"
)

// ─────────────────────── Data Types ───────────────────────

// HealthStatus represents an overall status level.
type HealthStatus string

const (
	StatusHealthy  HealthStatus = "healthy"
	StatusWarning  HealthStatus = "warning"
	StatusCritical HealthStatus = "critical"
)

// DBHealth holds the result of a PostgreSQL connectivity check.
type DBHealth struct {
	Status       HealthStatus `json:"status"`
	ResponseTime string       `json:"response_time"`
	Message      string       `json:"message"`
}

// StorageHealth holds the result of a storage path check.
type StorageHealth struct {
	Status      HealthStatus `json:"status"`
	Path        string       `json:"path"`
	Available   string       `json:"available"`
	Used        string       `json:"used"`
	Total       string       `json:"total"`
	UsedPercent float64      `json:"used_percent"`
	Writable    bool         `json:"writable"`
	Message     string       `json:"message"`
}

// SystemInfo holds static runtime information about the application.
type SystemInfo struct {
	AppVersion string `json:"app_version"`
	GoVersion  string `json:"go_version"`
	BuildTime  string `json:"build_time"`
	GitCommit  string `json:"git_commit"`
	Env        string `json:"env"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	NumCPU     int    `json:"num_cpu"`
}

// HealthReport is the top-level aggregated health response.
type HealthReport struct {
	Overall   HealthStatus  `json:"overall"`
	Uptime    string        `json:"uptime"`
	UptimeSec int64         `json:"uptime_sec"`
	DB        DBHealth      `json:"db"`
	Storage   StorageHealth `json:"storage"`
	System    SystemInfo    `json:"system"`
}

// ─────────────────────── Build-time variables ───────────────────────

// These are injected at link time via -ldflags if desired.
// Sensible defaults are provided as fallbacks.
var (
	AppVersion = "1.0.0"
	BuildTime  = "unknown"
	GitCommit  = "unknown"
)

// ─────────────────────── Service ───────────────────────

// HealthService performs all application health checks.
type HealthService struct {
	db          *gorm.DB
	storagePath string
	startTime   time.Time
}

// NewHealthService creates a HealthService.
// storagePath is the value stored in the settings (e.g. "/storage").
func NewHealthService(db *gorm.DB, storagePath string) *HealthService {
	return &HealthService{
		db:          db,
		storagePath: storagePath,
		startTime:   time.Now(),
	}
}

// UpdateStoragePath refreshes the storage path whenever settings change.
func (s *HealthService) UpdateStoragePath(path string) {
	s.storagePath = path
}

// ─────────────────────── Individual checks ───────────────────────

// CheckDB verifies PostgreSQL connectivity and measures response time.
func (s *HealthService) CheckDB() DBHealth {
	start := time.Now()
	sqlDB, err := s.db.DB()
	if err != nil {
		return DBHealth{
			Status:       StatusCritical,
			ResponseTime: "-",
			Message:      fmt.Sprintf("failed to obtain DB handle: %v", err),
		}
	}
	if err := sqlDB.Ping(); err != nil {
		elapsed := time.Since(start)
		return DBHealth{
			Status:       StatusCritical,
			ResponseTime: elapsed.Round(time.Millisecond).String(),
			Message:      fmt.Sprintf("ping failed: %v", err),
		}
	}
	elapsed := time.Since(start)
	status := StatusHealthy
	msg := "Connected"
	if elapsed > 200*time.Millisecond {
		status = StatusWarning
		msg = "Connected (slow response)"
	}
	return DBHealth{
		Status:       status,
		ResponseTime: elapsed.Round(time.Millisecond).String(),
		Message:      msg,
	}
}

// CheckStorage verifies the storage directory is present, writable, and measures disk usage.
func (s *HealthService) CheckStorage() StorageHealth {
	path := s.storagePath
	if path == "" {
		path = "/storage"
	}

	h := StorageHealth{Path: path}

	// Check writable
	testFile := path + "/.healthcheck"
	if err := os.MkdirAll(path, 0o755); err != nil {
		h.Status = StatusCritical
		h.Message = fmt.Sprintf("Cannot create storage directory: %v", err)
		h.Writable = false
		return h
	}
	f, err := os.CreateTemp(path, ".healthcheck_*")
	if err != nil {
		h.Writable = false
		h.Status = StatusCritical
		h.Message = fmt.Sprintf("Storage not writable: %v", err)
	} else {
		h.Writable = true
		f.Close()
		os.Remove(f.Name())
		_ = testFile
	}

	// Disk usage via syscall
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		if h.Status != StatusCritical {
			h.Status = StatusWarning
			h.Message = fmt.Sprintf("Cannot read disk stats: %v", err)
		}
		return h
	}

	total := stat.Blocks * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)
	used := total - (stat.Bfree * uint64(stat.Bsize))
	var usedPct float64
	if total > 0 {
		usedPct = float64(used) / float64(total) * 100
	}

	h.Total = formatDiskBytes(total)
	h.Available = formatDiskBytes(available)
	h.Used = formatDiskBytes(used)
	h.UsedPercent = usedPct

	// Determine status based on usage
	switch {
	case usedPct >= 90:
		h.Status = StatusCritical
		h.Message = fmt.Sprintf("Disk critically full (%.1f%%)", usedPct)
	case usedPct >= 75:
		h.Status = StatusWarning
		h.Message = fmt.Sprintf("Disk usage high (%.1f%%)", usedPct)
	default:
		if h.Status != StatusCritical {
			h.Status = StatusHealthy
			if h.Writable {
				h.Message = "OK"
			} else {
				h.Message = "Not writable"
			}
		}
	}

	return h
}

// GetSystemInfo returns static runtime metadata.
func (s *HealthService) GetSystemInfo() SystemInfo {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("ENV")
	}
	if env == "" {
		env = "production"
	}
	bt := BuildTime
	if bt == "" || bt == "unknown" {
		bt = "not set (dev build)"
	}
	gc := GitCommit
	if gc == "" || gc == "unknown" {
		gc = "not set (dev build)"
	}
	return SystemInfo{
		AppVersion: AppVersion,
		GoVersion:  runtime.Version(),
		BuildTime:  bt,
		GitCommit:  gc,
		Env:        env,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		NumCPU:     runtime.NumCPU(),
	}
}

// GetUptime returns a human-readable uptime string and the raw seconds.
func (s *HealthService) GetUptime() (string, int64) {
	dur := time.Since(s.startTime)
	secs := int64(dur.Seconds())
	return formatDuration(dur), secs
}

// Report runs all checks and returns an aggregated HealthReport.
func (s *HealthService) Report() HealthReport {
	db := s.CheckDB()
	stor := s.CheckStorage()
	sys := s.GetSystemInfo()
	uptimeStr, uptimeSec := s.GetUptime()

	overall := StatusHealthy
	if db.Status == StatusCritical || stor.Status == StatusCritical {
		overall = StatusCritical
	} else if db.Status == StatusWarning || stor.Status == StatusWarning {
		overall = StatusWarning
	}

	return HealthReport{
		Overall:   overall,
		Uptime:    uptimeStr,
		UptimeSec: uptimeSec,
		DB:        db,
		Storage:   stor,
		System:    sys,
	}
}

// ─────────────────────── Helpers ───────────────────────

func formatDiskBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
