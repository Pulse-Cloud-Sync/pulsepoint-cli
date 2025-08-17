// Package models defines the data structures used throughout PulsePoint
package models

import (
	"io"
	"time"
)

// File represents a file or directory in the PulsePoint system
type File struct {
	// Core identifiers
	ID       string `json:"id" bolt:"id"`
	Path     string `json:"path" bolt:"path"`
	Name     string `json:"name" bolt:"name"`
	RemoteID string `json:"remote_id,omitempty" bolt:"remote_id"`
	ParentID string `json:"parent_id,omitempty" bolt:"parent_id"`

	// File properties
	Size     int64  `json:"size" bolt:"size"`
	Hash     string `json:"hash,omitempty" bolt:"hash"`
	MimeType string `json:"mime_type,omitempty" bolt:"mime_type"`
	IsFolder bool   `json:"is_folder" bolt:"is_folder"`

	// Timestamps
	ModifiedTime time.Time `json:"modified_time" bolt:"modified_time"`
	CreatedTime  time.Time `json:"created_time" bolt:"created_time"`
	AccessedTime time.Time `json:"accessed_time,omitempty" bolt:"accessed_time"`

	// Local file properties
	LocalPath   string `json:"local_path,omitempty" bolt:"local_path"`
	Permissions string `json:"permissions,omitempty" bolt:"permissions"`
	Owner       string `json:"owner,omitempty" bolt:"owner"`
	Group       string `json:"group,omitempty" bolt:"group"`

	// Content (not stored in DB)
	Content io.Reader `json:"-" bolt:"-"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty" bolt:"metadata"`

	// Sync properties
	SyncStatus   string    `json:"sync_status,omitempty" bolt:"sync_status"`
	LastSyncTime time.Time `json:"last_sync_time,omitempty" bolt:"last_sync_time"`
	Version      int       `json:"version" bolt:"version"`
}

// NewFile creates a new File instance
func NewFile(path, name string) *File {
	return &File{
		ID:           GenerateFileID(path),
		Path:         path,
		Name:         name,
		CreatedTime:  time.Now(),
		ModifiedTime: time.Now(),
		Version:      1,
		Metadata:     make(map[string]interface{}),
	}
}

// GenerateFileID generates a unique ID for a file based on its path
func GenerateFileID(path string) string {
	// Implementation will use a hash function or UUID
	// Placeholder for now
	return path
}

// IsDirectory checks if the file is a directory
func (f *File) IsDirectory() bool {
	return f.IsFolder
}

// GetSize returns the file size in bytes
func (f *File) GetSize() int64 {
	return f.Size
}

// GetModTime returns the modification time
func (f *File) GetModTime() time.Time {
	return f.ModifiedTime
}

// NeedsSync checks if the file needs to be synced
func (f *File) NeedsSync() bool {
	return f.SyncStatus != "synced" || f.ModifiedTime.After(f.LastSyncTime)
}

// UpdateHash updates the file hash
func (f *File) UpdateHash(hash string) {
	f.Hash = hash
	f.Version++
}

// FileList represents a list of files
type FileList struct {
	Files      []*File `json:"files"`
	TotalCount int     `json:"total_count"`
	PageToken  string  `json:"page_token,omitempty"`
	HasMore    bool    `json:"has_more"`
}

// FileFilter represents filters for file queries
type FileFilter struct {
	Path           string    `json:"path,omitempty"`
	ParentID       string    `json:"parent_id,omitempty"`
	IsFolder       *bool     `json:"is_folder,omitempty"`
	MinSize        int64     `json:"min_size,omitempty"`
	MaxSize        int64     `json:"max_size,omitempty"`
	ModifiedAfter  time.Time `json:"modified_after,omitempty"`
	ModifiedBefore time.Time `json:"modified_before,omitempty"`
	MimeTypes      []string  `json:"mime_types,omitempty"`
	SyncStatus     string    `json:"sync_status,omitempty"`
}

// FileSort represents sorting options for file queries
type FileSort struct {
	Field     string `json:"field"`
	Ascending bool   `json:"ascending"`
}

// Common file sort fields
const (
	SortByName     = "name"
	SortBySize     = "size"
	SortByModified = "modified_time"
	SortByCreated  = "created_time"
	SortByPath     = "path"
)
