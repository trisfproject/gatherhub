package services

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/trisfproject/gatherhub/internal/config"
)

// BackupInfo represents descriptive metadata of a backup archive file.
type BackupInfo struct {
	Filename   string    `json:"filename"`
	BackupDate time.Time `json:"backup_date"`
	Size       int64     `json:"size"`
	SizeString string    `json:"size_string"`
}

// FileMetadata represents descriptive metadata of a single uploaded file inside a backup.
type FileMetadata struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

// BackupService implements database and file media backup and recovery controls.
type BackupService struct {
	cfg *config.Config
}

// NewBackupService creates a new BackupService.
func NewBackupService(cfg *config.Config) *BackupService {
	return &BackupService{cfg: cfg}
}

// CreateBackup generates a complete system backup zip containing a PostgreSQL SQL dump, uploads metadata, and physical files.
func (s *BackupService) CreateBackup() (string, error) {
	backupsDir := filepath.Join(s.cfg.StoragePath, "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backups folder: %w", err)
	}

	tempDir := filepath.Join(s.cfg.StoragePath, "temp", fmt.Sprintf("backup_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temporary working folder: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Export PostgreSQL database dump
	sqlFile := filepath.Join(tempDir, "database_backup.sql")
	cmd := exec.Command("pg_dump",
		"-h", s.cfg.DBHost,
		"-p", s.cfg.DBPort,
		"-U", s.cfg.DBUser,
		"-d", s.cfg.DBName,
		"-F", "p", // plain text format
		"-f", sqlFile,
		"--clean",
		"--if-exists",
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.DBPass)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pg_dump failed: %w, stderr: %s", err, errBuf.String())
	}

	// 2. Export uploaded files metadata
	var metadata []FileMetadata
	for _, sub := range []string{"payments", "events"} {
		dir := filepath.Join(s.cfg.StoragePath, sub)
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(s.cfg.StoragePath, path)
			if err == nil {
				metadata = append(metadata, FileMetadata{
					Path:       rel,
					Size:       info.Size(),
					ModifiedAt: info.ModTime(),
				})
			}
			return nil
		})
	}

	metadataFile := filepath.Join(tempDir, "files_metadata.json")
	metaBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(metadataFile, metaBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write metadata file: %w", err)
	}

	// 3. Compress database dump, metadata, and files into ZIP
	timestamp := time.Now().Format("20060102_150405")
	backupFilename := fmt.Sprintf("GH_BACKUP_%s.zip", timestamp)
	zipFilePath := filepath.Join(backupsDir, backupFilename)

	zipFile, err := os.Create(zipFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add database backup SQL to ZIP
	if err := addFileToZip(zipWriter, sqlFile, "database_backup.sql"); err != nil {
		return "", fmt.Errorf("failed to add SQL to ZIP: %w", err)
	}

	// Add metadata JSON to ZIP
	if err := addFileToZip(zipWriter, metadataFile, "files_metadata.json"); err != nil {
		return "", fmt.Errorf("failed to add metadata to ZIP: %w", err)
	}

	// Add actual uploaded files to ZIP under uploads/
	for _, item := range metadata {
		fullPath := filepath.Join(s.cfg.StoragePath, item.Path)
		zipPath := "uploads/" + item.Path
		if err := addFileToZip(zipWriter, fullPath, zipPath); err != nil {
			// Log and skip file if missing physically
			continue
		}
	}

	return backupFilename, nil
}

// ListBackups scans the backups directory and returns a sorted list of backups.
func (s *BackupService) ListBackups() ([]BackupInfo, error) {
	backupsDir := filepath.Join(s.cfg.StoragePath, "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to open backups folder: %w", err)
	}

	files, err := os.ReadDir(backupsDir)
	if err != nil {
		return nil, err
	}

	var list []BackupInfo
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(strings.ToLower(f.Name()), ".zip") {
			continue
		}

		info, err := f.Info()
		if err != nil {
			continue
		}

		// Parse backup date from filename (e.g. GH_BACKUP_20260613_091500.zip)
		backupDate := info.ModTime()
		name := f.Name()
		if len(name) >= 25 {
			dateStr := name[10:18]
			timeStr := name[19:25]
			if t, err := time.Parse("20060102 150405", dateStr+" "+timeStr); err == nil {
				backupDate = t
			}
		}

		list = append(list, BackupInfo{
			Filename:   name,
			BackupDate: backupDate,
			Size:       info.Size(),
			SizeString: formatBytes(info.Size()),
		})
	}

	// Sort backups by date descending (newest first)
	sort.Slice(list, func(i, j int) bool {
		return list[i].BackupDate.After(list[j].BackupDate)
	})

	return list, nil
}

// RestoreBackup restores database dump and extracts uploaded media files from the specified backup ZIP.
func (s *BackupService) RestoreBackup(filename string) error {
	backupsDir := filepath.Join(s.cfg.StoragePath, "backups")
	zipFilePath := filepath.Join(backupsDir, filename)

	if _, err := os.Stat(zipFilePath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	tempDir := filepath.Join(s.cfg.StoragePath, "temp", fmt.Sprintf("restore_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temporary working folder: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Open zip file
	reader, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer reader.Close()

	var sqlFile string

	// Extract files
	for _, f := range reader.File {
		err := func() error {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			if f.FileInfo().IsDir() {
				return nil
			}

			if f.Name == "database_backup.sql" {
				sqlFile = filepath.Join(tempDir, "database_backup.sql")
				dst, err := os.Create(sqlFile)
				if err != nil {
					return err
				}
				defer dst.Close()
				if _, err := io.Copy(dst, rc); err != nil {
					return err
				}
			} else if strings.HasPrefix(f.Name, "uploads/") {
				strippedPath := strings.TrimPrefix(f.Name, "uploads/")
				dstPath := filepath.Join(s.cfg.StoragePath, strippedPath)

				// Prevent path traversal (Zip Slip vulnerability)
				cleanedBase := filepath.Clean(s.cfg.StoragePath)
				cleanedDst := filepath.Clean(dstPath)
				if !strings.HasPrefix(cleanedDst, cleanedBase+string(filepath.Separator)) && cleanedDst != cleanedBase {
					return fmt.Errorf("path traversal detected in ZIP: %s", f.Name)
				}

				if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
					return err
				}

				dst, err := os.Create(dstPath)
				if err != nil {
					return err
				}
				defer dst.Close()
				if _, err := io.Copy(dst, rc); err != nil {
					return err
				}
			}
			return nil
		}()
		if err != nil {
			return err
		}
	}

	if sqlFile == "" {
		return fmt.Errorf("database backup file not found inside the ZIP")
	}

	// 2. Execute SQL restore using psql
	cmd := exec.Command("psql",
		"-h", s.cfg.DBHost,
		"-p", s.cfg.DBPort,
		"-U", s.cfg.DBUser,
		"-d", s.cfg.DBName,
		"-f", sqlFile,
	)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+s.cfg.DBPass)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql restore failed: %w, stderr: %s", err, errBuf.String())
	}

	return nil
}

// DeleteBackup deletes a backup archive from the backups directory.
func (s *BackupService) DeleteBackup(filename string) error {
	// Clean up filename to prevent directory traversal
	cleaned := filepath.Base(filename)
	zipFilePath := filepath.Join(s.cfg.StoragePath, "backups", cleaned)

	if _, err := os.Stat(zipFilePath); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	return os.Remove(zipFilePath)
}

// Helper: formats raw bytes into human readable binary units.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Helper: adds a local file to a zip archive writer.
func addFileToZip(zipWriter *zip.Writer, filePath string, zipPath string) error {
	fileToZip, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fileToZip.Close()

	info, err := fileToZip.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, fileToZip)
	return err
}
