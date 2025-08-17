package interfaces

import (
	"context"
)

// FileWatcher defines the contract for monitoring file system changes
type FileWatcher interface {
	// Start begins watching the specified paths
	Start(ctx context.Context, paths []string) error

	// Stop stops the file watcher
	Stop() error

	// Watch returns a channel that receives file change events
	Watch() <-chan ChangeEvent

	// Errors returns a channel for error notifications
	Errors() <-chan error

	// AddPath adds a new path to watch
	AddPath(path string) error

	// RemovePath removes a path from watching
	RemovePath(path string) error

	// SetIgnorePatterns sets patterns to ignore (gitignore style)
	SetIgnorePatterns(patterns []string) error

	// GetWatchedPaths returns list of currently watched paths
	GetWatchedPaths() []string

	// IsWatching checks if currently watching
	IsWatching() bool
}

// ChangeEvent represents a file system change event
type ChangeEvent struct {
	Type      ChangeType `json:"type"`
	Path      string     `json:"path"`
	OldPath   string     `json:"old_path,omitempty"` // For rename/move operations
	Timestamp int64      `json:"timestamp"`
	Size      int64      `json:"size,omitempty"`
	Hash      string     `json:"hash,omitempty"`
	IsDir     bool       `json:"is_dir"`
	Error     error      `json:"-"`
}

// ChangeType defines the type of file system change
type ChangeType string

const (
	// ChangeTypeCreate indicates a file or directory was created
	ChangeTypeCreate ChangeType = "create"

	// ChangeTypeModify indicates a file was modified
	ChangeTypeModify ChangeType = "modify"

	// ChangeTypeDelete indicates a file or directory was deleted
	ChangeTypeDelete ChangeType = "delete"

	// ChangeTypeRename indicates a file or directory was renamed
	ChangeTypeRename ChangeType = "rename"

	// ChangeTypeMove indicates a file or directory was moved
	ChangeTypeMove ChangeType = "move"

	// ChangeTypeChmod indicates file permissions changed
	ChangeTypeChmod ChangeType = "chmod"
)

// String returns the string representation of the change type
func (ct ChangeType) String() string {
	return string(ct)
}
