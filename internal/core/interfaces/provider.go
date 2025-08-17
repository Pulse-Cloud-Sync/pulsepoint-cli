// Package interfaces defines the core contracts for the PulsePoint system
package interfaces

import (
	"context"
	"io"
	"time"
)

// CloudProvider defines the contract for interacting with cloud storage providers
type CloudProvider interface {
	// Initialize sets up the provider with configuration
	Initialize(config ProviderConfig) error

	// Upload uploads a file to the cloud storage
	Upload(ctx context.Context, file *File) error

	// Download downloads a file from the cloud storage
	Download(ctx context.Context, path string) (*File, error)

	// Delete deletes a file from the cloud storage
	Delete(ctx context.Context, path string) error

	// List lists files in a folder
	List(ctx context.Context, folder string) ([]*File, error)

	// GetMetadata retrieves metadata for a file
	GetMetadata(ctx context.Context, path string) (*Metadata, error)

	// CreateFolder creates a folder in the cloud storage
	CreateFolder(ctx context.Context, path string) error

	// Move moves/renames a file or folder
	Move(ctx context.Context, sourcePath, destPath string) error

	// GetQuota returns storage quota information
	GetQuota(ctx context.Context) (*QuotaInfo, error)

	// GetProviderName returns the name of the provider
	GetProviderName() string

	// IsConnected checks if the provider is connected
	IsConnected() bool

	// Disconnect closes the connection to the provider
	Disconnect() error
}

// ProviderConfig holds configuration for a cloud provider
type ProviderConfig struct {
	Type        string                 `json:"type"`
	Credentials map[string]interface{} `json:"credentials"`
	Settings    map[string]interface{} `json:"settings"`
}

// File represents a file in the system
type File struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	Hash         string    `json:"hash"`
	MimeType     string    `json:"mime_type"`
	ModifiedTime time.Time `json:"modified_time"`
	CreatedTime  time.Time `json:"created_time"`
	IsFolder     bool      `json:"is_folder"`
	Content      io.Reader `json:"-"`
	LocalPath    string    `json:"local_path,omitempty"`
	RemoteID     string    `json:"remote_id,omitempty"`
	ParentID     string    `json:"parent_id,omitempty"`
	Permissions  string    `json:"permissions,omitempty"`
}

// Metadata represents file metadata
type Metadata struct {
	ID           string                 `json:"id"`
	Path         string                 `json:"path"`
	Size         int64                  `json:"size"`
	Hash         string                 `json:"hash"`
	ModifiedTime time.Time              `json:"modified_time"`
	CreatedTime  time.Time              `json:"created_time"`
	MimeType     string                 `json:"mime_type"`
	IsFolder     bool                   `json:"is_folder"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
	Version      string                 `json:"version,omitempty"`
	Owner        string                 `json:"owner,omitempty"`
}

// QuotaInfo represents storage quota information
type QuotaInfo struct {
	Used      int64 `json:"used"`
	Available int64 `json:"available"`
	Total     int64 `json:"total"`
}
