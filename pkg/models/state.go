package models

import (
	"time"
)

// SyncState represents the overall synchronization state
type SyncState struct {
	// Version information
	Version       string `json:"version" bolt:"version"`
	SchemaVersion int    `json:"schema_version" bolt:"schema_version"`

	// Timing information
	LastSyncTime      time.Time `json:"last_sync_time" bolt:"last_sync_time"`
	LastSuccessTime   time.Time `json:"last_success_time" bolt:"last_success_time"`
	NextScheduledSync time.Time `json:"next_scheduled_sync,omitempty" bolt:"next_scheduled_sync"`

	// File counts
	TotalFiles    int `json:"total_files" bolt:"total_files"`
	SyncedFiles   int `json:"synced_files" bolt:"synced_files"`
	PendingFiles  int `json:"pending_files" bolt:"pending_files"`
	FailedFiles   int `json:"failed_files" bolt:"failed_files"`
	IgnoredFiles  int `json:"ignored_files" bolt:"ignored_files"`
	ConflictFiles int `json:"conflict_files" bolt:"conflict_files"`

	// Size information
	TotalBytes   int64 `json:"total_bytes" bolt:"total_bytes"`
	SyncedBytes  int64 `json:"synced_bytes" bolt:"synced_bytes"`
	PendingBytes int64 `json:"pending_bytes" bolt:"pending_bytes"`

	// Current operation
	CurrentOperation  string    `json:"current_operation,omitempty" bolt:"current_operation"`
	OperationProgress float64   `json:"operation_progress" bolt:"operation_progress"` // 0-100
	OperationStarted  time.Time `json:"operation_started,omitempty" bolt:"operation_started"`

	// Status
	IsRunning         bool   `json:"is_running" bolt:"is_running"`
	IsPaused          bool   `json:"is_paused" bolt:"is_paused"`
	IsInitialized     bool   `json:"is_initialized" bolt:"is_initialized"`
	LastError         string `json:"last_error,omitempty" bolt:"last_error"`
	ConsecutiveErrors int    `json:"consecutive_errors" bolt:"consecutive_errors"`

	// History
	Errors   []string `json:"errors,omitempty" bolt:"errors"`
	Warnings []string `json:"warnings,omitempty" bolt:"warnings"`

	// Configuration snapshot
	ConfigHash string `json:"config_hash,omitempty" bolt:"config_hash"`
	Provider   string `json:"provider,omitempty" bolt:"provider"`
	Strategy   string `json:"strategy,omitempty" bolt:"strategy"`

	// Extended metadata
	Metadata map[string]interface{} `json:"metadata,omitempty" bolt:"metadata"`
}

// NewSyncState creates a new SyncState instance
func NewSyncState() *SyncState {
	return &SyncState{
		Version:       "1.0.0",
		SchemaVersion: 1,
		IsInitialized: true,
		Metadata:      make(map[string]interface{}),
		Errors:        []string{},
		Warnings:      []string{},
	}
}

// UpdateProgress updates the operation progress
func (s *SyncState) UpdateProgress(progress float64) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	s.OperationProgress = progress
}

// StartOperation marks the start of an operation
func (s *SyncState) StartOperation(operation string) {
	s.CurrentOperation = operation
	s.OperationStarted = time.Now()
	s.OperationProgress = 0
	s.IsRunning = true
	s.IsPaused = false
}

// EndOperation marks the end of an operation
func (s *SyncState) EndOperation(success bool) {
	s.CurrentOperation = ""
	s.OperationProgress = 100
	s.IsRunning = false

	if success {
		s.LastSuccessTime = time.Now()
		s.ConsecutiveErrors = 0
	} else {
		s.ConsecutiveErrors++
	}
	s.LastSyncTime = time.Now()
}

// AddError adds an error to the state
func (s *SyncState) AddError(err string) {
	s.LastError = err
	s.Errors = append(s.Errors, err)
	s.ConsecutiveErrors++

	// Keep only last 100 errors
	if len(s.Errors) > 100 {
		s.Errors = s.Errors[len(s.Errors)-100:]
	}
}

// AddWarning adds a warning to the state
func (s *SyncState) AddWarning(warning string) {
	s.Warnings = append(s.Warnings, warning)

	// Keep only last 100 warnings
	if len(s.Warnings) > 100 {
		s.Warnings = s.Warnings[len(s.Warnings)-100:]
	}
}

// FileState represents the state of a single file
type FileState struct {
	// Identification
	Path     string `json:"path" bolt:"path"`
	FileID   string `json:"file_id" bolt:"file_id"`
	RemoteID string `json:"remote_id,omitempty" bolt:"remote_id"`

	// Hashes for comparison
	LocalHash  string `json:"local_hash" bolt:"local_hash"`
	RemoteHash string `json:"remote_hash" bolt:"remote_hash"`
	HashAlgo   string `json:"hash_algo" bolt:"hash_algo"`

	// Timestamps
	LocalModTime  time.Time `json:"local_mod_time" bolt:"local_mod_time"`
	RemoteModTime time.Time `json:"remote_mod_time" bolt:"remote_mod_time"`
	LastSyncTime  time.Time `json:"last_sync_time" bolt:"last_sync_time"`
	LastCheckTime time.Time `json:"last_check_time" bolt:"last_check_time"`

	// Size
	Size       int64 `json:"size" bolt:"size"`
	LocalSize  int64 `json:"local_size" bolt:"local_size"`
	RemoteSize int64 `json:"remote_size" bolt:"remote_size"`

	// Status
	Status     FileSyncStatus `json:"status" bolt:"status"`
	LastError  string         `json:"last_error,omitempty" bolt:"last_error"`
	RetryCount int            `json:"retry_count" bolt:"retry_count"`
	MaxRetries int            `json:"max_retries" bolt:"max_retries"`

	// Version tracking
	Version       int `json:"version" bolt:"version"`
	LocalVersion  int `json:"local_version" bolt:"local_version"`
	RemoteVersion int `json:"remote_version" bolt:"remote_version"`

	// Conflict information
	HasConflict      bool      `json:"has_conflict" bolt:"has_conflict"`
	ConflictType     string    `json:"conflict_type,omitempty" bolt:"conflict_type"`
	ConflictDetected time.Time `json:"conflict_detected,omitempty" bolt:"conflict_detected"`

	// Metadata
	IsFolder bool                   `json:"is_folder" bolt:"is_folder"`
	MimeType string                 `json:"mime_type,omitempty" bolt:"mime_type"`
	Metadata map[string]interface{} `json:"metadata,omitempty" bolt:"metadata"`
}

// NewFileState creates a new FileState instance
func NewFileState(path string) *FileState {
	return &FileState{
		Path:          path,
		FileID:        GenerateFileID(path),
		Status:        FileSyncStatusPending,
		Version:       1,
		MaxRetries:    3,
		LastCheckTime: time.Now(),
		Metadata:      make(map[string]interface{}),
	}
}

// FileSyncStatus represents the synchronization status of a file
type FileSyncStatus string

const (
	// FileSyncStatusPending file is pending synchronization
	FileSyncStatusPending FileSyncStatus = "pending"

	// FileSyncStatusSynced file is synchronized
	FileSyncStatusSynced FileSyncStatus = "synced"

	// FileSyncStatusModified file has been modified
	FileSyncStatusModified FileSyncStatus = "modified"

	// FileSyncStatusConflict file has a conflict
	FileSyncStatusConflict FileSyncStatus = "conflict"

	// FileSyncStatusError file had an error during sync
	FileSyncStatusError FileSyncStatus = "error"

	// FileSyncStatusDeleted file has been deleted
	FileSyncStatusDeleted FileSyncStatus = "deleted"

	// FileSyncStatusIgnored file is ignored
	FileSyncStatusIgnored FileSyncStatus = "ignored"

	// FileSyncStatusUploading file is being uploaded
	FileSyncStatusUploading FileSyncStatus = "uploading"

	// FileSyncStatusDownloading file is being downloaded
	FileSyncStatusDownloading FileSyncStatus = "downloading"
)

// NeedsSync checks if the file needs synchronization
func (fs *FileState) NeedsSync() bool {
	return fs.Status != FileSyncStatusSynced || fs.LocalHash != fs.RemoteHash
}

// CanRetry checks if the file can be retried
func (fs *FileState) CanRetry() bool {
	return fs.RetryCount < fs.MaxRetries
}

// IncrementRetry increments the retry count
func (fs *FileState) IncrementRetry() {
	fs.RetryCount++
}

// ResetRetry resets the retry count
func (fs *FileState) ResetRetry() {
	fs.RetryCount = 0
	fs.LastError = ""
}

// SetError sets an error on the file state
func (fs *FileState) SetError(err string) {
	fs.Status = FileSyncStatusError
	fs.LastError = err
	fs.IncrementRetry()
}

// SetConflict marks the file as having a conflict
func (fs *FileState) SetConflict(conflictType string) {
	fs.Status = FileSyncStatusConflict
	fs.HasConflict = true
	fs.ConflictType = conflictType
	fs.ConflictDetected = time.Now()
}

// ResolveConflict resolves the conflict
func (fs *FileState) ResolveConflict() {
	fs.HasConflict = false
	fs.ConflictType = ""
	fs.Status = FileSyncStatusSynced
}

// UpdateLocalInfo updates local file information
func (fs *FileState) UpdateLocalInfo(hash string, modTime time.Time, size int64) {
	fs.LocalHash = hash
	fs.LocalModTime = modTime
	fs.LocalSize = size
	fs.LocalVersion++
	fs.LastCheckTime = time.Now()

	if fs.LocalHash != fs.RemoteHash {
		fs.Status = FileSyncStatusModified
	}
}

// UpdateRemoteInfo updates remote file information
func (fs *FileState) UpdateRemoteInfo(hash string, modTime time.Time, size int64, remoteID string) {
	fs.RemoteHash = hash
	fs.RemoteModTime = modTime
	fs.RemoteSize = size
	fs.RemoteID = remoteID
	fs.RemoteVersion++
	fs.LastSyncTime = time.Now()

	if fs.LocalHash == fs.RemoteHash {
		fs.Status = FileSyncStatusSynced
	}
}
