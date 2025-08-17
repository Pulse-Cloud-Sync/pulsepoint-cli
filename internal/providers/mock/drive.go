// Package mock provides a mock implementation of Google Drive for testing
package mock

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/pulsepoint/pulsepoint/pkg/logger"
	"go.uber.org/zap"
)

// MockDriveProvider implements a mock Google Drive provider for testing
type MockDriveProvider struct {
	// Mock storage - maps file paths to content
	files   map[string][]byte
	folders map[string]bool
	mu      sync.RWMutex
	logger  *zap.Logger

	// Mock configuration
	rootPath string
	quota    int64
	used     int64
}

// NewMockDriveProvider creates a new mock provider
func NewMockDriveProvider() *MockDriveProvider {
	return &MockDriveProvider{
		files:    make(map[string][]byte),
		folders:  make(map[string]bool),
		logger:   logger.Get(),
		rootPath: "/mock-drive",
		quota:    10 * 1024 * 1024 * 1024, // 10GB mock quota
		used:     0,
	}
}

// Initialize initializes the mock provider
func (m *MockDriveProvider) Initialize(config interfaces.ProviderConfig) error {
	m.logger.Info("Mock Drive provider initialized")
	m.folders["/"] = true
	return nil
}

// Upload uploads a file to mock storage
func (m *MockDriveProvider) Upload(ctx context.Context, file *interfaces.File) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read content from the file
	var content []byte
	if file.Content != nil {
		var err error
		content, err = io.ReadAll(file.Content)
		if err != nil {
			return fmt.Errorf("failed to read content: %w", err)
		}
	} else if file.LocalPath != "" {
		var err error
		content, err = os.ReadFile(file.LocalPath)
		if err != nil {
			return fmt.Errorf("failed to read local file: %w", err)
		}
	} else {
		return fmt.Errorf("no content or local path provided")
	}

	// Store in mock storage
	remotePath := file.Path
	m.files[remotePath] = content
	m.used += int64(len(content))

	// Create parent directories
	dir := filepath.Dir(remotePath)
	for dir != "/" && dir != "." {
		m.folders[dir] = true
		dir = filepath.Dir(dir)
	}

	m.logger.Info("Mock upload successful",
		zap.String("path", remotePath),
		zap.Int64("size", int64(len(content))))

	return nil
}

// Download downloads a file from mock storage
func (m *MockDriveProvider) Download(ctx context.Context, path string) (*interfaces.File, error) {
	m.mu.RLock()
	content, exists := m.files[path]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	file := &interfaces.File{
		Path:         path,
		Name:         filepath.Base(path),
		Size:         int64(len(content)),
		Hash:         fmt.Sprintf("mock-hash-%s", filepath.Base(path)),
		MimeType:     "application/octet-stream",
		ModifiedTime: time.Now(),
		Content:      bytes.NewReader(content),
		RemoteID:     fmt.Sprintf("mock-id-%s", path),
	}

	m.logger.Info("Mock download successful",
		zap.String("path", path),
		zap.Int("size", len(content)))

	return file, nil
}

// Delete deletes a file from mock storage
func (m *MockDriveProvider) Delete(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if content, exists := m.files[path]; exists {
		m.used -= int64(len(content))
		delete(m.files, path)
		m.logger.Info("Mock file deleted", zap.String("path", path))
		return nil
	}

	if _, exists := m.folders[path]; exists {
		delete(m.folders, path)
		m.logger.Info("Mock folder deleted", zap.String("path", path))
		return nil
	}

	return fmt.Errorf("path not found: %s", path)
}

// List lists files in mock storage
func (m *MockDriveProvider) List(ctx context.Context, folder string) ([]*interfaces.File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*interfaces.File

	// Add folders
	for path := range m.folders {
		if filepath.Dir(path) == folder || (folder == "/" && path != "/") {
			results = append(results, &interfaces.File{
				Path:     path,
				Name:     filepath.Base(path),
				IsFolder: true,
				RemoteID: fmt.Sprintf("folder-%s", path),
			})
		}
	}

	// Add files
	for path, content := range m.files {
		if filepath.Dir(path) == folder {
			results = append(results, &interfaces.File{
				Path:         path,
				Name:         filepath.Base(path),
				Size:         int64(len(content)),
				IsFolder:     false,
				ModifiedTime: time.Now(),
				Hash:         fmt.Sprintf("mock-hash-%s", filepath.Base(path)),
				MimeType:     "application/octet-stream",
				RemoteID:     fmt.Sprintf("file-%s", path),
			})
		}
	}

	m.logger.Info("Mock list completed",
		zap.String("folder", folder),
		zap.Int("results", len(results)))

	return results, nil
}

// GetMetadata gets metadata for a file
func (m *MockDriveProvider) GetMetadata(ctx context.Context, path string) (*interfaces.Metadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if content, exists := m.files[path]; exists {
		return &interfaces.Metadata{
			ID:           fmt.Sprintf("file-%s", path),
			Path:         path,
			Size:         int64(len(content)),
			ModifiedTime: time.Now(),
			IsFolder:     false,
			Hash:         fmt.Sprintf("mock-hash-%s", filepath.Base(path)),
			MimeType:     "application/octet-stream",
		}, nil
	}

	if _, exists := m.folders[path]; exists {
		return &interfaces.Metadata{
			ID:       fmt.Sprintf("folder-%s", path),
			Path:     path,
			IsFolder: true,
			MimeType: "application/vnd.google-apps.folder",
		}, nil
	}

	return nil, fmt.Errorf("path not found: %s", path)
}

// CreateFolder creates a folder in mock storage
func (m *MockDriveProvider) CreateFolder(ctx context.Context, path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.folders[path] = true

	// Create parent directories
	dir := filepath.Dir(path)
	for dir != "/" && dir != "." {
		m.folders[dir] = true
		dir = filepath.Dir(dir)
	}

	m.logger.Info("Mock folder created", zap.String("path", path))
	return nil
}

// Move moves a file in mock storage
func (m *MockDriveProvider) Move(ctx context.Context, sourcePath, destPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Move file
	if content, exists := m.files[sourcePath]; exists {
		m.files[destPath] = content
		delete(m.files, sourcePath)
		m.logger.Info("Mock file moved",
			zap.String("source", sourcePath),
			zap.String("dest", destPath))
		return nil
	}

	// Move folder
	if _, exists := m.folders[sourcePath]; exists {
		m.folders[destPath] = true
		delete(m.folders, sourcePath)

		// Move all children
		for path, content := range m.files {
			if len(path) > len(sourcePath) && path[:len(sourcePath)] == sourcePath {
				newPath := filepath.Join(destPath, path[len(sourcePath):])
				m.files[newPath] = content
				delete(m.files, path)
			}
		}

		m.logger.Info("Mock folder moved",
			zap.String("source", sourcePath),
			zap.String("dest", destPath))
		return nil
	}

	return fmt.Errorf("source path not found: %s", sourcePath)
}

// GetQuota returns mock quota information
func (m *MockDriveProvider) GetQuota(ctx context.Context) (*interfaces.QuotaInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &interfaces.QuotaInfo{
		Total:     m.quota,
		Used:      m.used,
		Available: m.quota - m.used,
	}, nil
}

// GetProviderName returns the provider name
func (m *MockDriveProvider) GetProviderName() string {
	return "mock-drive"
}

// IsConnected checks if the provider is connected
func (m *MockDriveProvider) IsConnected() bool {
	return true
}

// Disconnect disconnects from the provider
func (m *MockDriveProvider) Disconnect() error {
	m.logger.Info("Mock Drive provider disconnected")
	return nil
}

// GetMockStats returns statistics about the mock storage (for testing)
func (m *MockDriveProvider) GetMockStats() (files int, folders int, totalSize int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.files), len(m.folders), m.used
}
