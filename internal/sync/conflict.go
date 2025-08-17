package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	pperrors "github.com/pulsepoint/pulsepoint/pkg/errors"
	"go.uber.org/zap"
)

// PulsePointConflictResolver handles conflict resolution during sync
type PulsePointConflictResolver struct {
	logger   *zap.Logger
	strategy interfaces.ResolutionStrategy
	config   *ConflictResolverConfig
}

// ConflictResolverConfig holds configuration for conflict resolution
type ConflictResolverConfig struct {
	DefaultStrategy    interfaces.ResolutionStrategy `json:"default_strategy"`
	AutoResolve        bool                          `json:"auto_resolve"`
	BackupConflicts    bool                          `json:"backup_conflicts"`
	BackupDir          string                        `json:"backup_dir"`
	PreferNewer        bool                          `json:"prefer_newer"`
	PreferLarger       bool                          `json:"prefer_larger"`
	MergeTextFiles     bool                          `json:"merge_text_files"`
	InteractiveTimeout time.Duration                 `json:"interactive_timeout"`
	Rules              []ResolutionRule              `json:"rules"`
}

// ResolutionRule defines a rule for automatic conflict resolution
type ResolutionRule struct {
	Pattern    string                        `json:"pattern"`
	FileType   string                        `json:"file_type"`
	Strategy   interfaces.ResolutionStrategy `json:"strategy"`
	Priority   int                           `json:"priority"`
	Conditions map[string]interface{}        `json:"conditions"`
}

// NewPulsePointConflictResolver creates a new conflict resolver
func NewPulsePointConflictResolver(
	logger *zap.Logger,
	strategy interfaces.ResolutionStrategy,
	config *ConflictResolverConfig,
) *PulsePointConflictResolver {
	if config == nil {
		config = &ConflictResolverConfig{
			DefaultStrategy: interfaces.ResolutionKeepLocal,
			AutoResolve:     false,
			BackupConflicts: true,
			BackupDir:       ".conflicts",
			PreferNewer:     true,
			PreferLarger:    false,
			MergeTextFiles:  false,
		}
	}

	return &PulsePointConflictResolver{
		logger:   logger.With(zap.String("component", "conflict_resolver")),
		strategy: strategy,
		config:   config,
	}
}

// ResolveConflict resolves a single conflict
func (r *PulsePointConflictResolver) ResolveConflict(
	ctx context.Context,
	conflict *interfaces.Conflict,
) (*interfaces.ConflictResolution, error) {
	r.logger.Info("Resolving conflict",
		zap.String("path", conflict.Path),
		zap.String("type", string(conflict.Type)),
	)

	// Determine resolution strategy
	strategy := r.determineStrategy(conflict)

	// Apply resolution based on strategy
	var resolution *interfaces.ConflictResolution
	var err error

	switch strategy {
	case interfaces.ResolutionKeepLocal:
		resolution, err = r.keepLocal(ctx, conflict)
	case interfaces.ResolutionKeepRemote:
		resolution, err = r.keepRemote(ctx, conflict)
	case interfaces.ResolutionKeepBoth:
		resolution, err = r.keepBoth(ctx, conflict)
	case interfaces.ResolutionMerge:
		resolution, err = r.merge(ctx, conflict)
	case interfaces.ResolutionSkip:
		resolution, err = r.skip(ctx, conflict)
	case interfaces.ResolutionInteractive:
		resolution, err = r.interactive(ctx, conflict)
	default:
		return nil, pperrors.NewSyncError(
			fmt.Sprintf("unknown resolution strategy: %s", strategy),
			nil,
		)
	}

	if err != nil {
		return nil, pperrors.NewSyncError("conflict resolution failed", err)
	}

	r.logger.Info("Conflict resolved",
		zap.String("path", conflict.Path),
		zap.String("strategy", string(resolution.Strategy)),
		zap.String("winner", resolution.Winner),
	)

	return resolution, nil
}

// ResolveMultiple resolves multiple conflicts
func (r *PulsePointConflictResolver) ResolveMultiple(
	ctx context.Context,
	conflicts []interfaces.Conflict,
) ([]*interfaces.ConflictResolution, error) {
	resolutions := make([]*interfaces.ConflictResolution, 0, len(conflicts))

	for _, conflict := range conflicts {
		resolution, err := r.ResolveConflict(ctx, &conflict)
		if err != nil {
			r.logger.Error("Failed to resolve conflict",
				zap.String("path", conflict.Path),
				zap.Error(err),
			)
			// Continue with other conflicts
			continue
		}
		resolutions = append(resolutions, resolution)
	}

	return resolutions, nil
}

// determineStrategy determines the resolution strategy for a conflict
func (r *PulsePointConflictResolver) determineStrategy(conflict *interfaces.Conflict) interfaces.ResolutionStrategy {
	// Check rules first
	for _, rule := range r.config.Rules {
		if r.matchesRule(conflict, rule) {
			r.logger.Debug("Matched resolution rule",
				zap.String("path", conflict.Path),
				zap.String("pattern", rule.Pattern),
				zap.String("strategy", string(rule.Strategy)),
			)
			return rule.Strategy
		}
	}

	// Auto-resolution based on preferences
	if r.config.AutoResolve {
		if r.config.PreferNewer {
			if conflict.LocalFile.ModifiedTime.After(conflict.RemoteFile.ModifiedTime) {
				return interfaces.ResolutionKeepLocal
			}
			return interfaces.ResolutionKeepRemote
		}

		if r.config.PreferLarger {
			if conflict.LocalFile.Size > conflict.RemoteFile.Size {
				return interfaces.ResolutionKeepLocal
			}
			return interfaces.ResolutionKeepRemote
		}
	}

	// Use default strategy
	return r.config.DefaultStrategy
}

// matchesRule checks if a conflict matches a resolution rule
func (r *PulsePointConflictResolver) matchesRule(conflict *interfaces.Conflict, rule ResolutionRule) bool {
	// Check pattern
	if rule.Pattern != "" {
		matched, _ := filepath.Match(rule.Pattern, conflict.Path)
		if !matched {
			return false
		}
	}

	// Check file type
	if rule.FileType != "" {
		ext := filepath.Ext(conflict.Path)
		if ext != rule.FileType {
			return false
		}
	}

	// Check conditions
	for key, value := range rule.Conditions {
		switch key {
		case "conflict_type":
			if string(conflict.Type) != value.(string) {
				return false
			}
		case "size_greater_than":
			if conflict.LocalFile.Size <= value.(int64) {
				return false
			}
		case "size_less_than":
			if conflict.LocalFile.Size >= value.(int64) {
				return false
			}
		}
	}

	return true
}

// keepLocal keeps the local version
func (r *PulsePointConflictResolver) keepLocal(ctx context.Context, conflict *interfaces.Conflict) (*interfaces.ConflictResolution, error) {
	resolution := &interfaces.ConflictResolution{
		Strategy:   interfaces.ResolutionKeepLocal,
		Winner:     "local",
		ResolvedAt: time.Now().UnixNano(),
		Manual:     false,
	}

	// Backup remote version if configured
	if r.config.BackupConflicts {
		backupPath, err := r.backupFile(conflict.Path, conflict.RemoteFile, "remote")
		if err != nil {
			r.logger.Warn("Failed to backup remote file",
				zap.String("path", conflict.Path),
				zap.Error(err),
			)
		} else {
			resolution.BackupPath = backupPath
		}
	}

	return resolution, nil
}

// keepRemote keeps the remote version
func (r *PulsePointConflictResolver) keepRemote(ctx context.Context, conflict *interfaces.Conflict) (*interfaces.ConflictResolution, error) {
	resolution := &interfaces.ConflictResolution{
		Strategy:   interfaces.ResolutionKeepRemote,
		Winner:     "remote",
		ResolvedAt: time.Now().UnixNano(),
		Manual:     false,
	}

	// Backup local version if configured
	if r.config.BackupConflicts {
		backupPath, err := r.backupFile(conflict.Path, conflict.LocalFile, "local")
		if err != nil {
			r.logger.Warn("Failed to backup local file",
				zap.String("path", conflict.Path),
				zap.Error(err),
			)
		} else {
			resolution.BackupPath = backupPath
		}
	}

	return resolution, nil
}

// keepBoth keeps both versions with rename
func (r *PulsePointConflictResolver) keepBoth(ctx context.Context, conflict *interfaces.Conflict) (*interfaces.ConflictResolution, error) {
	// Generate new names for both files
	timestamp := time.Now().Format("20060102_150405")
	baseName := filepath.Base(conflict.Path)
	dir := filepath.Dir(conflict.Path)
	ext := filepath.Ext(baseName)
	nameWithoutExt := baseName[:len(baseName)-len(ext)]

	localNewPath := filepath.Join(dir, fmt.Sprintf("%s_local_%s%s", nameWithoutExt, timestamp, ext))
	remoteNewPath := filepath.Join(dir, fmt.Sprintf("%s_remote_%s%s", nameWithoutExt, timestamp, ext))

	resolution := &interfaces.ConflictResolution{
		Strategy:     interfaces.ResolutionKeepBoth,
		ResolvedPath: localNewPath,  // Path for local version
		BackupPath:   remoteNewPath, // Path for remote version
		ResolvedAt:   time.Now().UnixNano(),
		Manual:       false,
	}

	return resolution, nil
}

// merge attempts to merge changes (for text files)
func (r *PulsePointConflictResolver) merge(ctx context.Context, conflict *interfaces.Conflict) (*interfaces.ConflictResolution, error) {
	// Check if file is mergeable (text file)
	if !r.isMergeable(conflict.Path) {
		// Fall back to keep both
		return r.keepBoth(ctx, conflict)
	}

	// TODO: Implement actual merge logic using three-way merge
	// For now, fall back to keep both
	r.logger.Warn("Merge not yet implemented, falling back to keep both",
		zap.String("path", conflict.Path),
	)

	return r.keepBoth(ctx, conflict)
}

// skip skips the conflicted file
func (r *PulsePointConflictResolver) skip(ctx context.Context, conflict *interfaces.Conflict) (*interfaces.ConflictResolution, error) {
	resolution := &interfaces.ConflictResolution{
		Strategy:   interfaces.ResolutionSkip,
		ResolvedAt: time.Now().UnixNano(),
		Manual:     false,
	}

	return resolution, nil
}

// interactive prompts user for resolution
func (r *PulsePointConflictResolver) interactive(ctx context.Context, conflict *interfaces.Conflict) (*interfaces.ConflictResolution, error) {
	// TODO: Implement interactive resolution
	// For now, fall back to default strategy
	r.logger.Warn("Interactive resolution not yet implemented, using default strategy",
		zap.String("path", conflict.Path),
		zap.String("default_strategy", string(r.config.DefaultStrategy)),
	)

	r.strategy = r.config.DefaultStrategy
	return r.ResolveConflict(ctx, conflict)
}

// backupFile creates a backup of a file
func (r *PulsePointConflictResolver) backupFile(originalPath string, file *interfaces.File, suffix string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	baseName := filepath.Base(originalPath)
	backupName := fmt.Sprintf("%s_%s_%s", baseName, suffix, timestamp)
	backupPath := filepath.Join(r.config.BackupDir, backupName)

	// TODO: Actually copy/download the file to backup location
	// For now, just return the path
	return backupPath, nil
}

// isMergeable checks if a file can be merged
func (r *PulsePointConflictResolver) isMergeable(path string) bool {
	if !r.config.MergeTextFiles {
		return false
	}

	// Check file extension
	ext := filepath.Ext(path)
	mergeableExts := []string{
		".txt", ".md", ".json", ".xml", ".yaml", ".yml",
		".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h",
		".html", ".css", ".scss", ".less",
		".sh", ".bash", ".zsh",
		".conf", ".config", ".ini",
	}

	for _, mergeableExt := range mergeableExts {
		if ext == mergeableExt {
			return true
		}
	}

	return false
}

// GetStatistics returns conflict resolution statistics
func (r *PulsePointConflictResolver) GetStatistics() *ConflictStatistics {
	// TODO: Implement statistics tracking
	return &ConflictStatistics{
		TotalConflicts:    0,
		ResolvedConflicts: 0,
		PendingConflicts:  0,
		SkippedConflicts:  0,
		AutoResolved:      0,
		ManuallyResolved:  0,
		ResolutionsByType: make(map[string]int),
	}
}

// ConflictStatistics holds conflict resolution statistics
type ConflictStatistics struct {
	TotalConflicts    int            `json:"total_conflicts"`
	ResolvedConflicts int            `json:"resolved_conflicts"`
	PendingConflicts  int            `json:"pending_conflicts"`
	SkippedConflicts  int            `json:"skipped_conflicts"`
	AutoResolved      int            `json:"auto_resolved"`
	ManuallyResolved  int            `json:"manually_resolved"`
	ResolutionsByType map[string]int `json:"resolutions_by_type"`
}
