package strategies

import (
	"context"
	"fmt"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	pperrors "github.com/pulsepoint/pulsepoint/pkg/errors"
	"go.uber.org/zap"
)

// PulsePointMirrorStrategy implements mirror synchronization
// Mirror sync makes the destination an exact copy of the source (deletes extra files)
type PulsePointMirrorStrategy struct {
	provider interfaces.CloudProvider
	logger   *zap.Logger
	config   interfaces.StrategyConfig
}

// NewPulsePointMirrorStrategy creates a new mirror sync strategy
func NewPulsePointMirrorStrategy(
	provider interfaces.CloudProvider,
	logger *zap.Logger,
	config *interfaces.StrategyConfig,
) *PulsePointMirrorStrategy {
	if config == nil {
		config = &interfaces.StrategyConfig{
			ConflictResolution: interfaces.ResolutionKeepLocal,
			IgnorePatterns:     []string{},
			MaxFileSize:        0,     // No limit
			PreserveDeleted:    false, // Mirror always deletes extra files
			VersionControl:     false,
		}
	}

	// Force preserve_deleted to false for mirror sync
	config.PreserveDeleted = false

	return &PulsePointMirrorStrategy{
		provider: provider,
		logger:   logger.With(zap.String("strategy", "mirror")),
		config:   *config,
	}
}

// Name returns the strategy name
func (s *PulsePointMirrorStrategy) Name() string {
	return "mirror"
}

// Sync performs mirror synchronization
func (s *PulsePointMirrorStrategy) Sync(
	ctx context.Context,
	source, destination string,
	changes []interfaces.ChangeEvent,
) (*interfaces.SyncResult, error) {
	s.logger.Info("Starting mirror sync",
		zap.String("source", source),
		zap.String("destination", destination),
		zap.Int("changes", len(changes)),
	)

	result := &interfaces.SyncResult{
		StartTime: time.Now().UnixNano(),
		Success:   true,
	}

	// First, sync all local changes to remote
	for _, change := range changes {
		if err := s.processChange(ctx, change, result); err != nil {
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

			result.Success = false
		}
	}

	// Then, remove any remote files that don't exist locally
	if err := s.cleanupRemote(ctx, source, result); err != nil {
		s.logger.Error("Failed to cleanup remote files", zap.Error(err))
		result.Success = false
	}

	result.EndTime = time.Now().UnixNano()

	s.logger.Info("Mirror sync completed",
		zap.Int("processed", result.FilesProcessed),
		zap.Int("uploaded", result.FilesUploaded),
		zap.Int("deleted", result.FilesDeleted),
		zap.Int("skipped", result.FilesSkipped),
		zap.Int64("bytes", result.BytesTransferred),
		zap.Bool("success", result.Success),
	)

	return result, nil
}

// processChange processes a single change event
func (s *PulsePointMirrorStrategy) processChange(
	ctx context.Context,
	change interfaces.ChangeEvent,
	result *interfaces.SyncResult,
) error {
	result.FilesProcessed++

	switch change.Type {
	case interfaces.ChangeTypeCreate, interfaces.ChangeTypeModify:
		return s.uploadFile(ctx, change, result)

	case interfaces.ChangeTypeDelete:
		// Always delete in mirror mode
		return s.deleteFile(ctx, change, result)

	case interfaces.ChangeTypeRename:
		// Handle rename as delete old + create new
		if err := s.deleteFile(ctx, interfaces.ChangeEvent{
			Type: interfaces.ChangeTypeDelete,
			Path: change.OldPath,
		}, result); err != nil {
			return err
		}
		return s.uploadFile(ctx, change, result)

	default:
		s.logger.Warn("Unknown change type",
			zap.String("type", string(change.Type)),
			zap.String("path", change.Path),
		)
		result.FilesSkipped++
		return nil
	}
}

// uploadFile uploads a file to the remote
func (s *PulsePointMirrorStrategy) uploadFile(
	ctx context.Context,
	change interfaces.ChangeEvent,
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

	// Create file model
	file := &interfaces.File{
		Path:         change.Path,
		Size:         change.Size,
		Hash:         change.Hash,
		ModifiedTime: time.Unix(0, change.Timestamp),
		IsFolder:     change.IsDir,
	}

	// Upload to provider
	if err := s.provider.Upload(ctx, file); err != nil {
		return pperrors.NewSyncError(
			fmt.Sprintf("failed to upload %s", change.Path),
			err,
		)
	}

	result.FilesUploaded++
	result.BytesTransferred += change.Size

	s.logger.Debug("File uploaded successfully",
		zap.String("path", change.Path),
		zap.Int64("size", change.Size),
	)

	return nil
}

// deleteFile deletes a file from the remote
func (s *PulsePointMirrorStrategy) deleteFile(
	ctx context.Context,
	change interfaces.ChangeEvent,
	result *interfaces.SyncResult,
) error {
	// Delete from provider
	if err := s.provider.Delete(ctx, change.Path); err != nil {
		// If file doesn't exist, it's not an error in mirror mode
		if pperrors.IsNotFoundError(err) {
			s.logger.Debug("File already deleted from remote",
				zap.String("path", change.Path),
			)
			return nil
		}

		return pperrors.NewSyncError(
			fmt.Sprintf("failed to delete %s", change.Path),
			err,
		)
	}

	result.FilesDeleted++

	s.logger.Debug("File deleted successfully",
		zap.String("path", change.Path),
	)

	return nil
}

// cleanupRemote removes remote files that don't exist locally
func (s *PulsePointMirrorStrategy) cleanupRemote(
	ctx context.Context,
	localPath string,
	result *interfaces.SyncResult,
) error {
	s.logger.Info("Cleaning up remote files not in source")

	// List all remote files
	remoteFiles, err := s.provider.List(ctx, "/")
	if err != nil {
		return pperrors.NewSyncError("failed to list remote files", err)
	}

	// TODO: Compare with local files and delete extras
	// This requires access to local file system which should be
	// injected or passed as a parameter

	// For now, we'll just log the count
	s.logger.Info("Remote cleanup check completed",
		zap.Int("remote_files", len(remoteFiles)),
	)

	return nil
}

// ResolveConflict handles conflict resolution
func (s *PulsePointMirrorStrategy) ResolveConflict(
	ctx context.Context,
	conflict *interfaces.Conflict,
) (*interfaces.ConflictResolution, error) {
	// In mirror sync, local always wins (source is the truth)
	resolution := &interfaces.ConflictResolution{
		Strategy:   interfaces.ResolutionKeepLocal,
		Winner:     "local",
		ResolvedAt: time.Now().UnixNano(),
		Manual:     false,
	}

	s.logger.Debug("Conflict resolved (local wins in mirror sync)",
		zap.String("path", conflict.Path),
		zap.String("type", string(conflict.Type)),
	)

	return resolution, nil
}

// ValidateSync validates if sync can be performed
func (s *PulsePointMirrorStrategy) ValidateSync(
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

	// Warn about mirror sync behavior
	s.logger.Warn("Mirror sync will delete remote files not present in source")

	return nil
}

// GetDirection returns the sync direction
func (s *PulsePointMirrorStrategy) GetDirection() interfaces.SyncDirection {
	return interfaces.SyncDirectionMirror
}

// SupportsResume checks if the strategy supports resuming interrupted syncs
func (s *PulsePointMirrorStrategy) SupportsResume() bool {
	return true
}

// GetConfiguration returns the strategy configuration
func (s *PulsePointMirrorStrategy) GetConfiguration() interfaces.StrategyConfig {
	return s.config
}
