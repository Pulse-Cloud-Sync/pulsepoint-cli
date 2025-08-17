package strategies

import (
	"context"
	"fmt"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	pperrors "github.com/pulsepoint/pulsepoint/pkg/errors"
	"go.uber.org/zap"
)

// PulsePointOneWayStrategy implements one-way synchronization (local to remote)
type PulsePointOneWayStrategy struct {
	provider interfaces.CloudProvider
	logger   *zap.Logger
	config   interfaces.StrategyConfig
}

// NewPulsePointOneWayStrategy creates a new one-way sync strategy
func NewPulsePointOneWayStrategy(
	provider interfaces.CloudProvider,
	logger *zap.Logger,
	config *interfaces.StrategyConfig,
) *PulsePointOneWayStrategy {
	if config == nil {
		config = &interfaces.StrategyConfig{
			ConflictResolution: interfaces.ResolutionKeepLocal,
			IgnorePatterns:     []string{},
			MaxFileSize:        0, // No limit
			PreserveDeleted:    false,
			VersionControl:     false,
		}
	}

	return &PulsePointOneWayStrategy{
		provider: provider,
		logger:   logger.With(zap.String("strategy", "oneway")),
		config:   *config,
	}
}

// Name returns the strategy name
func (s *PulsePointOneWayStrategy) Name() string {
	return "one-way"
}

// Sync performs one-way synchronization from local to remote
func (s *PulsePointOneWayStrategy) Sync(
	ctx context.Context,
	source, destination string,
	changes []interfaces.ChangeEvent,
) (*interfaces.SyncResult, error) {
	s.logger.Info("Starting one-way sync",
		zap.String("source", source),
		zap.String("destination", destination),
		zap.Int("changes", len(changes)),
	)

	result := &interfaces.SyncResult{
		StartTime: time.Now().UnixNano(),
		Success:   true,
	}

	// Process each change
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

	result.EndTime = time.Now().UnixNano()

	s.logger.Info("One-way sync completed",
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
func (s *PulsePointOneWayStrategy) processChange(
	ctx context.Context,
	change interfaces.ChangeEvent,
	result *interfaces.SyncResult,
) error {
	result.FilesProcessed++

	switch change.Type {
	case interfaces.ChangeTypeCreate, interfaces.ChangeTypeModify:
		return s.uploadFile(ctx, change, result)

	case interfaces.ChangeTypeDelete:
		if s.config.PreserveDeleted {
			s.logger.Debug("Skipping delete (preserve_deleted enabled)",
				zap.String("path", change.Path),
			)
			result.FilesSkipped++
			return nil
		}
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
func (s *PulsePointOneWayStrategy) uploadFile(
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
func (s *PulsePointOneWayStrategy) deleteFile(
	ctx context.Context,
	change interfaces.ChangeEvent,
	result *interfaces.SyncResult,
) error {
	// Delete from provider
	if err := s.provider.Delete(ctx, change.Path); err != nil {
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

// ResolveConflict handles conflict resolution
func (s *PulsePointOneWayStrategy) ResolveConflict(
	ctx context.Context,
	conflict *interfaces.Conflict,
) (*interfaces.ConflictResolution, error) {
	// In one-way sync, local always wins
	resolution := &interfaces.ConflictResolution{
		Strategy:   interfaces.ResolutionKeepLocal,
		Winner:     "local",
		ResolvedAt: time.Now().UnixNano(),
		Manual:     false,
	}

	s.logger.Debug("Conflict resolved (local wins in one-way sync)",
		zap.String("path", conflict.Path),
		zap.String("type", string(conflict.Type)),
	)

	return resolution, nil
}

// ValidateSync validates if sync can be performed
func (s *PulsePointOneWayStrategy) ValidateSync(
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

	// Additional validation can be added here
	return nil
}

// GetDirection returns the sync direction
func (s *PulsePointOneWayStrategy) GetDirection() interfaces.SyncDirection {
	return interfaces.SyncDirectionOneWay
}

// SupportsResume checks if the strategy supports resuming interrupted syncs
func (s *PulsePointOneWayStrategy) SupportsResume() bool {
	return true
}

// GetConfiguration returns the strategy configuration
func (s *PulsePointOneWayStrategy) GetConfiguration() interfaces.StrategyConfig {
	return s.config
}
