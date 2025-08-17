package models

import (
	"fmt"
	"time"
)

// Conflict represents a synchronization conflict between local and remote files
type Conflict struct {
	// Identification
	ID   string `json:"id" bolt:"id"`
	Path string `json:"path" bolt:"path"`

	// Conflict details
	Type        ConflictType     `json:"type" bolt:"type"`
	Description string           `json:"description" bolt:"description"`
	Severity    ConflictSeverity `json:"severity" bolt:"severity"`

	// File information
	LocalFile  *File `json:"local_file" bolt:"local_file"`
	RemoteFile *File `json:"remote_file" bolt:"remote_file"`
	BaseFile   *File `json:"base_file,omitempty" bolt:"base_file"` // For three-way merge

	// Timestamps
	DetectedAt  time.Time `json:"detected_at" bolt:"detected_at"`
	ResolvedAt  time.Time `json:"resolved_at,omitempty" bolt:"resolved_at"`
	LastAttempt time.Time `json:"last_attempt,omitempty" bolt:"last_attempt"`

	// Resolution information
	Resolution       *ConflictResolution `json:"resolution,omitempty" bolt:"resolution"`
	ResolutionStatus ResolutionStatus    `json:"resolution_status" bolt:"resolution_status"`
	AutoResolvable   bool                `json:"auto_resolvable" bolt:"auto_resolvable"`
	UserRequired     bool                `json:"user_required" bolt:"user_required"`

	// Attempts and history
	AttemptCount int      `json:"attempt_count" bolt:"attempt_count"`
	MaxAttempts  int      `json:"max_attempts" bolt:"max_attempts"`
	History      []string `json:"history,omitempty" bolt:"history"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty" bolt:"metadata"`
}

// NewConflict creates a new conflict
func NewConflict(path string, conflictType ConflictType, local, remote *File) *Conflict {
	return &Conflict{
		ID:               GenerateConflictID(path),
		Path:             path,
		Type:             conflictType,
		LocalFile:        local,
		RemoteFile:       remote,
		DetectedAt:       time.Now(),
		ResolutionStatus: ResolutionStatusPending,
		MaxAttempts:      3,
		AutoResolvable:   false,
		UserRequired:     false,
		Metadata:         make(map[string]interface{}),
		History:          []string{},
	}
}

// GenerateConflictID generates a unique conflict ID
func GenerateConflictID(path string) string {
	return fmt.Sprintf("conflict_%s_%d", path, time.Now().UnixNano())
}

// ConflictType defines the type of conflict
type ConflictType string

const (
	// ConflictTypeBothModified both local and remote files were modified
	ConflictTypeBothModified ConflictType = "both_modified"

	// ConflictTypeDeleteModify one file deleted, other modified
	ConflictTypeDeleteModify ConflictType = "delete_modify"

	// ConflictTypeNaming naming conflict (case sensitivity, special chars)
	ConflictTypeNaming ConflictType = "naming"

	// ConflictTypePermission permission conflict
	ConflictTypePermission ConflictType = "permission"

	// ConflictTypeType type conflict (file vs directory)
	ConflictTypeType ConflictType = "type"

	// ConflictTypeSize size conflict (exceeds limits)
	ConflictTypeSize ConflictType = "size"

	// ConflictTypeEncoding encoding conflict
	ConflictTypeEncoding ConflictType = "encoding"

	// ConflictTypeVersion version conflict
	ConflictTypeVersion ConflictType = "version"
)

// ConflictSeverity defines the severity of a conflict
type ConflictSeverity string

const (
	// ConflictSeverityLow low severity, can be auto-resolved
	ConflictSeverityLow ConflictSeverity = "low"

	// ConflictSeverityMedium medium severity, may need user input
	ConflictSeverityMedium ConflictSeverity = "medium"

	// ConflictSeverityHigh high severity, requires user decision
	ConflictSeverityHigh ConflictSeverity = "high"

	// ConflictSeverityCritical critical severity, blocks sync
	ConflictSeverityCritical ConflictSeverity = "critical"
)

// ConflictResolution represents how a conflict was or should be resolved
type ConflictResolution struct {
	// Resolution strategy
	Strategy    ResolutionStrategy `json:"strategy" bolt:"strategy"`
	Description string             `json:"description" bolt:"description"`

	// Resolution details
	Winner       string `json:"winner,omitempty" bolt:"winner"` // "local", "remote", or "merged"
	ResolvedPath string `json:"resolved_path,omitempty" bolt:"resolved_path"`
	BackupPath   string `json:"backup_path,omitempty" bolt:"backup_path"`
	MergedPath   string `json:"merged_path,omitempty" bolt:"merged_path"`

	// Resolution metadata
	ResolvedAt time.Time `json:"resolved_at" bolt:"resolved_at"`
	ResolvedBy string    `json:"resolved_by,omitempty" bolt:"resolved_by"` // user, auto, policy
	Manual     bool      `json:"manual" bolt:"manual"`

	// Merge information (for merge strategy)
	MergeBase      string   `json:"merge_base,omitempty" bolt:"merge_base"`
	MergeConflicts []string `json:"merge_conflicts,omitempty" bolt:"merge_conflicts"`

	// Additional actions taken
	Actions []string `json:"actions,omitempty" bolt:"actions"`
}

// NewConflictResolution creates a new conflict resolution
func NewConflictResolution(strategy ResolutionStrategy) *ConflictResolution {
	return &ConflictResolution{
		Strategy:   strategy,
		ResolvedAt: time.Now(),
		Actions:    []string{},
	}
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

	// ResolutionKeepNewer keeps the newer version
	ResolutionKeepNewer ResolutionStrategy = "keep_newer"

	// ResolutionKeepLarger keeps the larger file
	ResolutionKeepLarger ResolutionStrategy = "keep_larger"

	// ResolutionMerge attempts to merge changes
	ResolutionMerge ResolutionStrategy = "merge"

	// ResolutionRename renames conflicting file
	ResolutionRename ResolutionStrategy = "rename"

	// ResolutionSkip skips the conflicted file
	ResolutionSkip ResolutionStrategy = "skip"

	// ResolutionInteractive prompts user for resolution
	ResolutionInteractive ResolutionStrategy = "interactive"

	// ResolutionCustom uses custom resolution logic
	ResolutionCustom ResolutionStrategy = "custom"
)

// ResolutionStatus represents the status of conflict resolution
type ResolutionStatus string

const (
	// ResolutionStatusPending resolution is pending
	ResolutionStatusPending ResolutionStatus = "pending"

	// ResolutionStatusInProgress resolution in progress
	ResolutionStatusInProgress ResolutionStatus = "in_progress"

	// ResolutionStatusResolved conflict has been resolved
	ResolutionStatusResolved ResolutionStatus = "resolved"

	// ResolutionStatusFailed resolution failed
	ResolutionStatusFailed ResolutionStatus = "failed"

	// ResolutionStatusDeferred resolution deferred
	ResolutionStatusDeferred ResolutionStatus = "deferred"
)

// CanAutoResolve checks if the conflict can be automatically resolved
func (c *Conflict) CanAutoResolve() bool {
	if c.UserRequired || c.ResolutionStatus == ResolutionStatusResolved {
		return false
	}

	// Low severity conflicts can typically be auto-resolved
	if c.Severity == ConflictSeverityLow {
		return true
	}

	// Check specific auto-resolvable scenarios
	switch c.Type {
	case ConflictTypeNaming:
		// Naming conflicts can often be auto-resolved by renaming
		return true
	case ConflictTypeBothModified:
		// Can auto-resolve if one file is significantly newer
		if c.LocalFile != nil && c.RemoteFile != nil {
			timeDiff := c.LocalFile.ModifiedTime.Sub(c.RemoteFile.ModifiedTime)
			if timeDiff > 24*time.Hour || timeDiff < -24*time.Hour {
				return true
			}
		}
	}

	return c.AutoResolvable
}

// SetResolution sets the resolution for the conflict
func (c *Conflict) SetResolution(resolution *ConflictResolution) {
	c.Resolution = resolution
	c.ResolvedAt = resolution.ResolvedAt
	c.ResolutionStatus = ResolutionStatusResolved
}

// MarkAttempted marks that a resolution attempt was made
func (c *Conflict) MarkAttempted() {
	c.AttemptCount++
	c.LastAttempt = time.Now()
	if c.AttemptCount >= c.MaxAttempts {
		c.UserRequired = true
	}
}

// AddHistory adds an entry to the conflict history
func (c *Conflict) AddHistory(entry string) {
	timestamp := time.Now().Format(time.RFC3339)
	c.History = append(c.History, fmt.Sprintf("[%s] %s", timestamp, entry))

	// Keep only last 50 history entries
	if len(c.History) > 50 {
		c.History = c.History[len(c.History)-50:]
	}
}

// IsResolved checks if the conflict is resolved
func (c *Conflict) IsResolved() bool {
	return c.ResolutionStatus == ResolutionStatusResolved
}

// ConflictPolicy defines policies for automatic conflict resolution
type ConflictPolicy struct {
	// Default strategies for different conflict types
	DefaultStrategies map[ConflictType]ResolutionStrategy `json:"default_strategies"`

	// Automatic resolution settings
	AutoResolve   bool `json:"auto_resolve"`
	PreferLocal   bool `json:"prefer_local"`
	PreferRemote  bool `json:"prefer_remote"`
	PreferNewer   bool `json:"prefer_newer"`
	CreateBackups bool `json:"create_backups"`

	// Size-based policies
	MaxAutoResolveSize int64 `json:"max_auto_resolve_size"`

	// Type-specific policies
	TextFileMerge      bool               `json:"text_file_merge"`
	BinaryFileStrategy ResolutionStrategy `json:"binary_file_strategy"`

	// User interaction
	RequireConfirmation bool `json:"require_confirmation"`
	InteractiveMode     bool `json:"interactive_mode"`
}

// NewConflictPolicy creates a new conflict policy with defaults
func NewConflictPolicy() *ConflictPolicy {
	return &ConflictPolicy{
		DefaultStrategies: map[ConflictType]ResolutionStrategy{
			ConflictTypeBothModified: ResolutionKeepNewer,
			ConflictTypeDeleteModify: ResolutionKeepLocal,
			ConflictTypeNaming:       ResolutionRename,
			ConflictTypePermission:   ResolutionKeepLocal,
			ConflictTypeType:         ResolutionSkip,
			ConflictTypeSize:         ResolutionSkip,
			ConflictTypeEncoding:     ResolutionKeepLocal,
			ConflictTypeVersion:      ResolutionKeepNewer,
		},
		AutoResolve:         false,
		PreferNewer:         true,
		CreateBackups:       true,
		MaxAutoResolveSize:  100 * 1024 * 1024, // 100 MB
		TextFileMerge:       false,
		BinaryFileStrategy:  ResolutionKeepNewer,
		RequireConfirmation: true,
		InteractiveMode:     false,
	}
}

// GetStrategy returns the resolution strategy for a conflict type
func (p *ConflictPolicy) GetStrategy(conflictType ConflictType) ResolutionStrategy {
	if strategy, ok := p.DefaultStrategies[conflictType]; ok {
		return strategy
	}

	// Default fallback strategies
	if p.PreferLocal {
		return ResolutionKeepLocal
	}
	if p.PreferRemote {
		return ResolutionKeepRemote
	}
	if p.PreferNewer {
		return ResolutionKeepNewer
	}

	return ResolutionInteractive
}
