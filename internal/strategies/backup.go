package strategies

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	pperrors "github.com/pulsepoint/pulsepoint/pkg/errors"
	"go.uber.org/zap"
)

// PulsePointBackupStrategy implements backup synchronization
// Backup sync preserves all versions and never deletes files
type PulsePointBackupStrategy struct {
	provider interfaces.CloudProvider
	logger   *zap.Logger
	config   interfaces.StrategyConfig
}

// NewPulsePointBackupStrategy creates a new backup sync strategy
func NewPulsePointBackupStrategy(
	provider interfaces.CloudProvider,
	logger *zap.Logger,
	config *interfaces.StrategyConfig,
) *PulsePointBackupStrategy {
	if config == nil {
		config = &interfaces.StrategyConfig{
			ConflictResolution: interfaces.ResolutionKeepBoth,
			IgnorePatterns:     []string{},
			MaxFileSize:        0,    // No limit
			PreserveDeleted:    true, // Always preserve in backup mode
			VersionControl:     true, // Enable versioning for backups
		}
	}

	// Force settings appropriate for backup
	config.PreserveDeleted = true
	config.VersionControl = true

	return &PulsePointBackupStrategy{
		provider: provider,
		logger:   logger.With(zap.String("strategy", "backup")),
		config:   *config,
	}
}

// Name returns the strategy name
func (s *PulsePointBackupStrategy) Name() string {
	return "backup"
}

// Sync performs backup synchronization
func (s *PulsePointBackupStrategy) Sync(
	ctx context.Context,
	source, destination string,
	changes []interfaces.ChangeEvent,
) (*interfaces.SyncResult, error) {
	s.logger.Info("Starting backup sync",
		zap.String("source", source),
		zap.String("destination", destination),
		zap.Int("changes", len(changes)),
	)

	result := &interfaces.SyncResult{
		StartTime: time.Now().UnixNano(),
		Success:   true,
	}

	// Create backup timestamp for this session
	backupTimestamp := time.Now().Format("20060102_150405")

	// Process each change
	for _, change := range changes {
		if err := s.processChange(ctx, change, backupTimestamp, result); err != nil {
			s.logger.Error("Failed to process change",
				zap.String("path", change.Path),
				zap.String("type", string(change.Type)),
				zap.Error(err),
			)

			result.Errors = append(result.Errors, interfaces.SyncError{
				Path:      change.Path,
				Operation: string(change.Type),
				Message:   err.Error(),
				Timestamp: time.Now().UnixNano(),
			})

			// Continue with other files even if one fails
		}
	}

	result.EndTime = time.Now().UnixNano()

	s.logger.Info("Backup sync completed",
		zap.Int("processed", result.FilesProcessed),
		zap.Int("uploaded", result.FilesUploaded),
		zap.Int("deleted", result.FilesDeleted),
		zap.Int("skipped", result.FilesSkipped),
		zap.Int64("bytes", result.BytesTransferred),
		zap.Bool("success", result.Success),
		zap.String("backup_timestamp", backupTimestamp),
	)

	return result, nil
}

// processChange processes a single change event
func (s *PulsePointBackupStrategy) processChange(
	ctx context.Context,
	change interfaces.ChangeEvent,
	backupTimestamp string,
	result *interfaces.SyncResult,
) error {
	result.FilesProcessed++

	switch change.Type {
	case interfaces.ChangeTypeCreate, interfaces.ChangeTypeModify:
		return s.backupFile(ctx, change, backupTimestamp, result)

	case interfaces.ChangeTypeDelete:
		// In backup mode, we mark files as deleted but don't remove them
		return s.markDeleted(ctx, change, backupTimestamp, result)

	case interfaces.ChangeTypeRename:
		// In backup mode, keep both old and new paths
		// Mark old path as moved
		if err := s.markMoved(ctx, change.OldPath, change.Path, backupTimestamp, result); err != nil {
			return err
		}
		// Upload new file
		return s.backupFile(ctx, change, backupTimestamp, result)

	default:
		s.logger.Warn("Unknown change type",
			zap.String("type", string(change.Type)),
			zap.String("path", change.Path),
		)
		result.FilesSkipped++
		return nil
	}
}

// backupFile backs up a file to the remote with versioning
func (s *PulsePointBackupStrategy) backupFile(
	ctx context.Context,
	change interfaces.ChangeEvent,
	backupTimestamp string,
	result *interfaces.SyncResult,
) error {
	// Check file size limit
	if s.config.MaxFileSize > 0 && change.Size > s.config.MaxFileSize {
		s.logger.Debug("File exceeds size limit, skipping",
			zap.String("path", change.Path),
			zap.Int64("size", change.Size),
			zap.Int64("limit", s.config.MaxFileSize),
		)
		result.FilesSkipped++
		return nil
	}

	// Generate versioned path if file already exists
	remotePath := change.Path
	if s.config.VersionControl {
		// Check if file exists remotely
		metadata, err := s.provider.GetMetadata(ctx, change.Path)
		if err == nil && metadata != nil {
			// File exists, create versioned backup
			remotePath = s.generateVersionedPath(change.Path, backupTimestamp)
			s.logger.Debug("Creating versioned backup",
				zap.String("original", change.Path),
				zap.String("versioned", remotePath),
			)
		}
	}

	// Create file model
	file := &interfaces.File{
		Path:         remotePath,
		Size:         change.Size,
		Hash:         change.Hash,
		ModifiedTime: time.Unix(0, change.Timestamp),
		IsFolder:     change.IsDir,
		// Note: Metadata field doesn't exist in interfaces.File
	}

	// Upload to provider
	if err := s.provider.Upload(ctx, file); err != nil {
		return pperrors.NewSyncError(
			fmt.Sprintf("failed to backup %s", change.Path),
			err,
		)
	}

	result.FilesUploaded++
	result.BytesTransferred += change.Size

	s.logger.Debug("File backed up successfully",
		zap.String("path", change.Path),
		zap.String("remote_path", remotePath),
		zap.Int64("size", change.Size),
	)

	return nil
}

// markDeleted marks a file as deleted without removing it
func (s *PulsePointBackupStrategy) markDeleted(
	ctx context.Context,
	change interfaces.ChangeEvent,
	backupTimestamp string,
	result *interfaces.SyncResult,
) error {
	// Create a deletion marker file
	markerPath := fmt.Sprintf("%s.deleted_%s", change.Path, backupTimestamp)

	markerFile := &interfaces.File{
		Path:         markerPath,
		Size:         0,
		ModifiedTime: time.Now(),
		IsFolder:     false,
	}

	// Upload deletion marker
	if err := s.provider.Upload(ctx, markerFile); err != nil {
		s.logger.Warn("Failed to create deletion marker",
			zap.String("path", change.Path),
			zap.Error(err),
		)
		// Not a fatal error, continue
	}

	s.logger.Debug("File marked as deleted",
		zap.String("path", change.Path),
		zap.String("marker", markerPath),
	)

	result.FilesProcessed++
	return nil
}

// markMoved marks a file as moved/renamed
func (s *PulsePointBackupStrategy) markMoved(
	ctx context.Context,
	oldPath, newPath string,
	backupTimestamp string,
	result *interfaces.SyncResult,
) error {
	// Create a move marker file
	markerPath := fmt.Sprintf("%s.moved_%s", oldPath, backupTimestamp)

	markerFile := &interfaces.File{
		Path:         markerPath,
		Size:         0,
		ModifiedTime: time.Now(),
		IsFolder:     false,
	}

	// Upload move marker
	if err := s.provider.Upload(ctx, markerFile); err != nil {
		s.logger.Warn("Failed to create move marker",
			zap.String("old_path", oldPath),
			zap.String("new_path", newPath),
			zap.Error(err),
		)
		// Not a fatal error, continue
	}

	s.logger.Debug("File marked as moved",
		zap.String("old_path", oldPath),
		zap.String("new_path", newPath),
		zap.String("marker", markerPath),
	)

	return nil
}

// generateVersionedPath generates a versioned path for a file
func (s *PulsePointBackupStrategy) generateVersionedPath(originalPath, timestamp string) string {
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)
	ext := filepath.Ext(base)
	nameWithoutExt := base[:len(base)-len(ext)]

	// Create versioned filename
	versionedName := fmt.Sprintf("%s_v%s%s", nameWithoutExt, timestamp, ext)

	if dir == "." {
		return versionedName
	}

	return filepath.Join(dir, versionedName)
}

// ResolveConflict handles conflict resolution
func (s *PulsePointBackupStrategy) ResolveConflict(
	ctx context.Context,
	conflict *interfaces.Conflict,
) (*interfaces.ConflictResolution, error) {
	// In backup mode, keep both versions
	timestamp := time.Now().Format("20060102_150405")

	resolution := &interfaces.ConflictResolution{
		Strategy:     interfaces.ResolutionKeepBoth,
		ResolvedPath: s.generateVersionedPath(conflict.Path, timestamp),
		ResolvedAt:   time.Now().UnixNano(),
		Manual:       false,
	}

	s.logger.Debug("Conflict resolved (keeping both in backup mode)",
		zap.String("path", conflict.Path),
		zap.String("type", string(conflict.Type)),
		zap.String("versioned_path", resolution.ResolvedPath),
	)

	return resolution, nil
}

// ValidateSync validates if sync can be performed
func (s *PulsePointBackupStrategy) ValidateSync(
	ctx context.Context,
	source, destination string,
) error {
	// Check if source exists and is accessible
	if source == "" {
		return pperrors.NewValidationError("source path is required", nil)
	}

	// Check if provider is initialized
	if s.provider == nil {
		return pperrors.NewValidationError("cloud provider not initialized", nil)
	}

	s.logger.Info("Backup sync will preserve all versions and never delete files")

	return nil
}

// GetDirection returns the sync direction
func (s *PulsePointBackupStrategy) GetDirection() interfaces.SyncDirection {
	return interfaces.SyncDirectionBackup
}

// SupportsResume checks if the strategy supports resuming interrupted syncs
func (s *PulsePointBackupStrategy) SupportsResume() bool {
	return true
}

// GetConfiguration returns the strategy configuration
func (s *PulsePointBackupStrategy) GetConfiguration() interfaces.StrategyConfig {
	return s.config
}
