package models

import (
	"time"
)

// Metadata represents comprehensive metadata for a file or directory
type Metadata struct {
	// Basic identification
	ID   string `json:"id" bolt:"id"`
	Path string `json:"path" bolt:"path"`
	Type string `json:"type" bolt:"type"` // file, directory, symlink

	// Size and content
	Size        int64  `json:"size" bolt:"size"`
	Hash        string `json:"hash,omitempty" bolt:"hash"`
	HashAlgo    string `json:"hash_algo,omitempty" bolt:"hash_algo"` // md5, sha1, sha256
	ContentHash string `json:"content_hash,omitempty" bolt:"content_hash"`

	// Timestamps
	ModifiedTime time.Time `json:"modified_time" bolt:"modified_time"`
	CreatedTime  time.Time `json:"created_time" bolt:"created_time"`
	AccessedTime time.Time `json:"accessed_time,omitempty" bolt:"accessed_time"`
	ChangedTime  time.Time `json:"changed_time,omitempty" bolt:"changed_time"` // ctime

	// File type and format
	MimeType  string `json:"mime_type,omitempty" bolt:"mime_type"`
	Extension string `json:"extension,omitempty" bolt:"extension"`
	IsFolder  bool   `json:"is_folder" bolt:"is_folder"`
	IsSymlink bool   `json:"is_symlink" bolt:"is_symlink"`
	IsHidden  bool   `json:"is_hidden" bolt:"is_hidden"`

	// Ownership and permissions
	Owner       string `json:"owner,omitempty" bolt:"owner"`
	Group       string `json:"group,omitempty" bolt:"group"`
	Permissions string `json:"permissions,omitempty" bolt:"permissions"` // Unix permissions

	// Version and sync information
	Version      string    `json:"version,omitempty" bolt:"version"`
	ETag         string    `json:"etag,omitempty" bolt:"etag"`
	LastSyncTime time.Time `json:"last_sync_time,omitempty" bolt:"last_sync_time"`
	SyncVersion  int       `json:"sync_version" bolt:"sync_version"`

	// Cloud provider specific
	CloudID      string `json:"cloud_id,omitempty" bolt:"cloud_id"`
	CloudVersion string `json:"cloud_version,omitempty" bolt:"cloud_version"`
	WebURL       string `json:"web_url,omitempty" bolt:"web_url"`
	DownloadURL  string `json:"download_url,omitempty" bolt:"download_url"`

	// Extended attributes
	Attributes map[string]interface{} `json:"attributes,omitempty" bolt:"attributes"`
	Tags       []string               `json:"tags,omitempty" bolt:"tags"`
	Labels     map[string]string      `json:"labels,omitempty" bolt:"labels"`

	// Relationships
	ParentID   string   `json:"parent_id,omitempty" bolt:"parent_id"`
	ChildIDs   []string `json:"child_ids,omitempty" bolt:"child_ids"`
	LinkTarget string   `json:"link_target,omitempty" bolt:"link_target"` // For symlinks
}

// NewMetadata creates a new Metadata instance
func NewMetadata(path string, isFolder bool) *Metadata {
	now := time.Now()
	return &Metadata{
		ID:           GenerateMetadataID(path),
		Path:         path,
		IsFolder:     isFolder,
		CreatedTime:  now,
		ModifiedTime: now,
		SyncVersion:  1,
		Attributes:   make(map[string]interface{}),
		Labels:       make(map[string]string),
		Tags:         []string{},
	}
}

// GenerateMetadataID generates a unique ID for metadata
func GenerateMetadataID(path string) string {
	// Implementation will use a hash function or UUID
	// Placeholder for now
	return "meta_" + path
}

// IsFile checks if the metadata represents a file
func (m *Metadata) IsFile() bool {
	return !m.IsFolder && !m.IsSymlink
}

// IsDirectory checks if the metadata represents a directory
func (m *Metadata) IsDirectory() bool {
	return m.IsFolder
}

// GetAge returns the age of the file since creation
func (m *Metadata) GetAge() time.Duration {
	return time.Since(m.CreatedTime)
}

// GetTimeSinceModified returns time since last modification
func (m *Metadata) GetTimeSinceModified() time.Duration {
	return time.Since(m.ModifiedTime)
}

// SetAttribute sets a custom attribute
func (m *Metadata) SetAttribute(key string, value interface{}) {
	if m.Attributes == nil {
		m.Attributes = make(map[string]interface{})
	}
	m.Attributes[key] = value
}

// GetAttribute gets a custom attribute
func (m *Metadata) GetAttribute(key string) (interface{}, bool) {
	if m.Attributes == nil {
		return nil, false
	}
	val, ok := m.Attributes[key]
	return val, ok
}

// AddTag adds a tag to the metadata
func (m *Metadata) AddTag(tag string) {
	for _, t := range m.Tags {
		if t == tag {
			return // Tag already exists
		}
	}
	m.Tags = append(m.Tags, tag)
}

// RemoveTag removes a tag from the metadata
func (m *Metadata) RemoveTag(tag string) {
	for i, t := range m.Tags {
		if t == tag {
			m.Tags = append(m.Tags[:i], m.Tags[i+1:]...)
			return
		}
	}
}

// HasTag checks if a tag exists
func (m *Metadata) HasTag(tag string) bool {
	for _, t := range m.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// SetLabel sets a label
func (m *Metadata) SetLabel(key, value string) {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[key] = value
}

// GetLabel gets a label value
func (m *Metadata) GetLabel(key string) (string, bool) {
	if m.Labels == nil {
		return "", false
	}
	val, ok := m.Labels[key]
	return val, ok
}

// MetadataFilter represents filters for metadata queries
type MetadataFilter struct {
	Path           string            `json:"path,omitempty"`
	Type           string            `json:"type,omitempty"`
	IsFolder       *bool             `json:"is_folder,omitempty"`
	MinSize        int64             `json:"min_size,omitempty"`
	MaxSize        int64             `json:"max_size,omitempty"`
	ModifiedAfter  time.Time         `json:"modified_after,omitempty"`
	ModifiedBefore time.Time         `json:"modified_before,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}
