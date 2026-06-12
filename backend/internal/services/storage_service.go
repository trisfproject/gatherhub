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
// All paths are derived from the configured root directory.
type StorageService struct {
	rootPath     string
	paymentsPath string
	eventsPath   string
	tempPath     string
}

// NewStorageService creates and initializes a StorageService.
// It automatically creates payments/, events/, and temp/ subdirectories.
func NewStorageService(rootPath string) (*StorageService, error) {
	if rootPath == "" {
		rootPath = "/storage"
	}

	s := &StorageService{
		rootPath:     rootPath,
		paymentsPath: filepath.Join(rootPath, "payments"),
		eventsPath:   filepath.Join(rootPath, "events"),
		tempPath:     filepath.Join(rootPath, "temp"),
	}

	// Create directories if they do not exist
	dirs := []string{s.paymentsPath, s.eventsPath, s.tempPath}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return s, nil
}

// SavePaymentProof saves a payment proof file to the payments storage path and returns the generated filename.
func (s *StorageService) SavePaymentProof(file *multipart.FileHeader) (string, error) {
	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := fmt.Sprintf("payment_%d%s", time.Now().UnixNano(), ext)
	fullPath := filepath.Join(s.paymentsPath, filename)

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
	ext := strings.ToLower(filepath.Ext(file.Filename))
	filename := fmt.Sprintf("banner_%d%s", time.Now().UnixNano(), ext)
	fullPath := filepath.Join(s.eventsPath, filename)

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
	return s.paymentsPath
}

// GetEventsPath returns the path to the events directory.
func (s *StorageService) GetEventsPath() string {
	return s.eventsPath
}

// GetTempPath returns the path to the temp directory.
func (s *StorageService) GetTempPath() string {
	return s.tempPath
}

// GetRootPath returns the root storage path.
func (s *StorageService) GetRootPath() string {
	return s.rootPath
}
