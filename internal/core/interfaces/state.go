package interfaces

import (
	"context"
	"time"
)

// StateManager defines the contract for managing sync state
type StateManager interface {
	// Initialize sets up the state manager
	Initialize(dbPath string) error

	// SaveState saves the current sync state
	SaveState(ctx context.Context, state *SyncState) error

	// LoadState loads the sync state
	LoadState(ctx context.Context) (*SyncState, error)

	// UpdateFileState updates state for a specific file
	UpdateFileState(ctx context.Context, file *FileState) error

	// GetFileState retrieves state for a specific file
	GetFileState(ctx context.Context, path string) (*FileState, error)

	// DeleteFileState removes state for a specific file
	DeleteFileState(ctx context.Context, path string) error

	// ListFileStates lists all file states
	ListFileStates(ctx context.Context) ([]*FileState, error)

	// SaveTransaction saves a sync transaction
	SaveTransaction(ctx context.Context, transaction *SyncTransaction) error

	// GetTransaction retrieves a specific transaction
	GetTransaction(ctx context.Context, id string) (*SyncTransaction, error)

	// ListTransactions lists transactions with pagination
	ListTransactions(ctx context.Context, offset, limit int) ([]*SyncTransaction, error)

	// Cleanup performs cleanup of old state data
	Cleanup(ctx context.Context, before time.Time) error

	// Export exports state to a file
	Export(ctx context.Context, path string) error

	// Import imports state from a file
	Import(ctx context.Context, path string) error

	// Reset resets all state data
	Reset(ctx context.Context) error

	// Close closes the state manager
	Close() error

	// GetStatistics returns sync statistics
	GetStatistics(ctx context.Context) (*SyncStatistics, error)
}

// SyncState represents the overall sync state
type SyncState struct {
	Version          string                 `json:"version"`
	LastSyncTime     time.Time              `json:"last_sync_time"`
	LastSuccessTime  time.Time              `json:"last_success_time"`
	TotalFiles       int                    `json:"total_files"`
	SyncedFiles      int                    `json:"synced_files"`
	PendingFiles     int                    `json:"pending_files"`
	FailedFiles      int                    `json:"failed_files"`
	TotalBytes       int64                  `json:"total_bytes"`
	SyncedBytes      int64                  `json:"synced_bytes"`
	CurrentOperation string                 `json:"current_operation,omitempty"`
	IsRunning        bool                   `json:"is_running"`
	Errors           []string               `json:"errors,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// FileState represents the state of a single file
type FileState struct {
	Path          string                 `json:"path"`
	LocalHash     string                 `json:"local_hash"`
	RemoteHash    string                 `json:"remote_hash"`
	LocalModTime  time.Time              `json:"local_mod_time"`
	RemoteModTime time.Time              `json:"remote_mod_time"`
	Size          int64                  `json:"size"`
	Status        FileSyncStatus         `json:"status"`
	LastSyncTime  time.Time              `json:"last_sync_time"`
	LastError     string                 `json:"last_error,omitempty"`
	RetryCount    int                    `json:"retry_count"`
	RemoteID      string                 `json:"remote_id,omitempty"`
	Version       int                    `json:"version"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// FileSyncStatus represents the sync status of a file
type FileSyncStatus string

const (
	// FileSyncStatusPending file is pending sync
	FileSyncStatusPending FileSyncStatus = "pending"

	// FileSyncStatusSynced file is synced
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
)

// SyncTransaction represents a sync transaction
type SyncTransaction struct {
	ID               string            `json:"id"`
	StartTime        time.Time         `json:"start_time"`
	EndTime          time.Time         `json:"end_time,omitempty"`
	Type             TransactionType   `json:"type"`
	Status           TransactionStatus `json:"status"`
	FilesAffected    []string          `json:"files_affected"`
	BytesTransferred int64             `json:"bytes_transferred"`
	Errors           []string          `json:"errors,omitempty"`
	Result           *SyncResult       `json:"result,omitempty"`
}

// TransactionType defines the type of transaction
type TransactionType string

const (
	// TransactionTypeUpload upload transaction
	TransactionTypeUpload TransactionType = "upload"

	// TransactionTypeDownload download transaction
	TransactionTypeDownload TransactionType = "download"

	// TransactionTypeDelete delete transaction
	TransactionTypeDelete TransactionType = "delete"

	// TransactionTypeFullSync full sync transaction
	TransactionTypeFullSync TransactionType = "full_sync"

	// TransactionTypePartialSync partial sync transaction
	TransactionTypePartialSync TransactionType = "partial_sync"
)

// TransactionStatus defines the status of a transaction
type TransactionStatus string

const (
	// TransactionStatusPending transaction is pending
	TransactionStatusPending TransactionStatus = "pending"

	// TransactionStatusRunning transaction is running
	TransactionStatusRunning TransactionStatus = "running"

	// TransactionStatusCompleted transaction completed successfully
	TransactionStatusCompleted TransactionStatus = "completed"

	// TransactionStatusFailed transaction failed
	TransactionStatusFailed TransactionStatus = "failed"

	// TransactionStatusCancelled transaction was cancelled
	TransactionStatusCancelled TransactionStatus = "cancelled"
)

// SyncStatistics represents sync statistics
type SyncStatistics struct {
	TotalSyncs       int64     `json:"total_syncs"`
	SuccessfulSyncs  int64     `json:"successful_syncs"`
	FailedSyncs      int64     `json:"failed_syncs"`
	TotalFiles       int64     `json:"total_files"`
	TotalBytes       int64     `json:"total_bytes"`
	AverageSpeed     float64   `json:"average_speed_mbps"`
	LastSyncDuration int64     `json:"last_sync_duration_seconds"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	Since            time.Time `json:"since"`
}
