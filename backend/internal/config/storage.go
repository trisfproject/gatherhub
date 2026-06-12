package config

import (
	"log"
	"os"
	"path/filepath"
)

// StorageConfig holds resolved absolute paths for all runtime storage locations.
// All paths are derived from a single root (STORAGE_PATH env variable).
type StorageConfig struct {
	Root     string // Base storage directory
	Payments string // Payment proof uploads: {Root}/payments
	Events   string // Event banner uploads:  {Root}/events
	Temp     string // Temporary files:       {Root}/temp
}

// NewStorageConfig constructs a StorageConfig from the given root path.
// All sub-paths are joined with filepath.Join — no manual concatenation.
func NewStorageConfig(root string) *StorageConfig {
	return &StorageConfig{
		Root:     root,
		Payments: filepath.Join(root, "payments"),
		Events:   filepath.Join(root, "events"),
		Temp:     filepath.Join(root, "temp"),
	}
}

// Init creates all storage subdirectories if they do not already exist.
// Returns the first error encountered, if any.
func (s *StorageConfig) Init() error {
	dirs := []string{s.Payments, s.Events, s.Temp}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	log.Printf("Storage initialized at: %s", s.Root)
	return nil
}

// CheckLegacy emits a warning if the legacy storage directory still exists
// inside the repository root. This helps operators know they should migrate.
func (s *StorageConfig) CheckLegacy(repoRoot string) {
	legacyPath := filepath.Join(repoRoot, "storage")
	if info, err := os.Stat(legacyPath); err == nil && info.IsDir() {
		log.Printf("⚠  Legacy storage directory detected at %q. "+
			"Please migrate files to STORAGE_PATH=%q and remove the old directory.",
			legacyPath, s.Root)
	}
}
