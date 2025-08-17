package interfaces

import (
	"context"
)

// SyncStrategy defines the contract for different synchronization strategies
type SyncStrategy interface {
	// Name returns the name of the strategy
	Name() string

	// Sync performs synchronization based on the strategy
	Sync(ctx context.Context, source, destination string, changes []ChangeEvent) (*SyncResult, error)

	// ResolveConflict handles conflict resolution
	ResolveConflict(ctx context.Context, conflict *Conflict) (*ConflictResolution, error)

	// ValidateSync validates if sync can be performed
	ValidateSync(ctx context.Context, source, destination string) error

	// GetDirection returns the sync direction
	GetDirection() SyncDirection

	// SupportsResume checks if the strategy supports resuming interrupted syncs
	SupportsResume() bool

	// GetConfiguration returns the strategy configuration
	GetConfiguration() StrategyConfig
}

// SyncDirection defines the direction of synchronization
type SyncDirection string

const (
	// SyncDirectionOneWay syncs from source to destination only
	SyncDirectionOneWay SyncDirection = "one-way"

	// SyncDirectionTwoWay syncs bidirectionally
	SyncDirectionTwoWay SyncDirection = "two-way"

	// SyncDirectionMirror mirrors source to destination (deletes extra files)
	SyncDirectionMirror SyncDirection = "mirror"

	// SyncDirectionBackup creates backups without deletion
	SyncDirectionBackup SyncDirection = "backup"
)

// SyncResult represents the result of a sync operation
type SyncResult struct {
	StartTime        int64       `json:"start_time"`
	EndTime          int64       `json:"end_time"`
	FilesProcessed   int         `json:"files_processed"`
	FilesUploaded    int         `json:"files_uploaded"`
	FilesDownloaded  int         `json:"files_downloaded"`
	FilesDeleted     int         `json:"files_deleted"`
	FilesSkipped     int         `json:"files_skipped"`
	BytesTransferred int64       `json:"bytes_transferred"`
	Errors           []SyncError `json:"errors,omitempty"`
	Conflicts        []Conflict  `json:"conflicts,omitempty"`
	Success          bool        `json:"success"`
}

// SyncError represents an error during sync
type SyncError struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// Conflict represents a sync conflict
type Conflict struct {
	Path       string             `json:"path"`
	Type       ConflictType       `json:"type"`
	LocalFile  *File              `json:"local_file"`
	RemoteFile *File              `json:"remote_file"`
	DetectedAt int64              `json:"detected_at"`
	Resolution ConflictResolution `json:"resolution,omitempty"`
}

// ConflictType defines the type of conflict
type ConflictType string

const (
	// ConflictTypeModified both files modified
	ConflictTypeModified ConflictType = "both_modified"

	// ConflictTypeDeleted one file deleted, other modified
	ConflictTypeDeleted ConflictType = "delete_modify"

	// ConflictTypeNaming naming conflict
	ConflictTypeNaming ConflictType = "naming"

	// ConflictTypePermission permission conflict
	ConflictTypePermission ConflictType = "permission"
)

// ConflictResolution represents how a conflict was resolved
type ConflictResolution struct {
	Strategy     ResolutionStrategy `json:"strategy"`
	Winner       string             `json:"winner,omitempty"` // "local" or "remote"
	ResolvedPath string             `json:"resolved_path,omitempty"`
	BackupPath   string             `json:"backup_path,omitempty"`
	ResolvedAt   int64              `json:"resolved_at"`
	Manual       bool               `json:"manual"`
}

// ResolutionStrategy defines how to resolve conflicts
type ResolutionStrategy string

const (
	// ResolutionKeepLocal keeps the local version
	ResolutionKeepLocal ResolutionStrategy = "keep_local"

	// ResolutionKeepRemote keeps the remote version
	ResolutionKeepRemote ResolutionStrategy = "keep_remote"

	// ResolutionKeepBoth keeps both versions with rename
	ResolutionKeepBoth ResolutionStrategy = "keep_both"

	// ResolutionMerge attempts to merge changes
	ResolutionMerge ResolutionStrategy = "merge"

	// ResolutionSkip skips the conflicted file
	ResolutionSkip ResolutionStrategy = "skip"

	// ResolutionInteractive prompts user for resolution
	ResolutionInteractive ResolutionStrategy = "interactive"
)

// StrategyConfig holds configuration for a sync strategy
type StrategyConfig struct {
	ConflictResolution ResolutionStrategy     `json:"conflict_resolution"`
	IgnorePatterns     []string               `json:"ignore_patterns"`
	MaxFileSize        int64                  `json:"max_file_size"`
	PreserveDeleted    bool                   `json:"preserve_deleted"`
	VersionControl     bool                   `json:"version_control"`
	CustomSettings     map[string]interface{} `json:"custom_settings,omitempty"`
}
