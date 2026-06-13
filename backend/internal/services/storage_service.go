package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StorageService handles file saving and path resolution for runtime storage.
// All paths are derived from the configured root directory dynamically.
type StorageService struct {
	settingsService *SettingsService
	defaultRootPath string
}

// NewStorageService creates and initializes a StorageService.
// It automatically creates payments/, events/, and temp/ subdirectories.
func NewStorageService(rootPath string, settingsService *SettingsService) (*StorageService, error) {
	if rootPath == "" {
		rootPath = "/storage"
	}

	s := &StorageService{
		settingsService: settingsService,
		defaultRootPath: rootPath,
	}

	// Create directories if they do not exist
	if err := s.ensureDirs(s.GetRootPath()); err != nil {
		return nil, err
	}

	return s, nil
}

// ensureDirs ensures subdirectories exist under the specified root path.
func (s *StorageService) ensureDirs(root string) error {
	dirs := []string{
		filepath.Join(root, "payments"),
		filepath.Join(root, "events"),
		filepath.Join(root, "temp"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// SavePaymentProof saves a payment proof file to the payments storage path and returns the generated filename.
func (s *StorageService) SavePaymentProof(file *multipart.FileHeader) (string, error) {
	root := s.GetRootPath()
	if err := s.ensureDirs(root); err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := fmt.Sprintf("payment_%d%s", time.Now().UnixNano(), ext)
	fullPath := filepath.Join(root, "payments", filename)

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open upload source file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file in storage: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to write uploaded file to storage: %w", err)
	}

	return filename, nil
}

// SaveEventBanner saves an event banner file to the events storage path and returns the generated filename.
func (s *StorageService) SaveEventBanner(file *multipart.FileHeader) (string, error) {
	root := s.GetRootPath()
	if err := s.ensureDirs(root); err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := fmt.Sprintf("banner_%d%s", time.Now().UnixNano(), ext)
	fullPath := filepath.Join(root, "events", filename)

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open upload source file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file in storage: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to write uploaded file to storage: %w", err)
	}

	return filename, nil
}

// GetPaymentsPath returns the path to the payments directory.
func (s *StorageService) GetPaymentsPath() string {
	return filepath.Join(s.GetRootPath(), "payments")
}

// GetEventsPath returns the path to the events directory.
func (s *StorageService) GetEventsPath() string {
	return filepath.Join(s.GetRootPath(), "events")
}

// GetTempPath returns the path to the temp directory.
func (s *StorageService) GetTempPath() string {
	return filepath.Join(s.GetRootPath(), "temp")
}

// GetRootPath returns the root storage path dynamically.
func (s *StorageService) GetRootPath() string {
	if s.settingsService != nil {
		if path := s.settingsService.Get("storage_path"); path != "" {
			return path
		}
	}
	return s.defaultRootPath
}
