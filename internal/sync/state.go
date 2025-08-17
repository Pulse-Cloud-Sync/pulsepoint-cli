package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/pulsepoint/pulsepoint/internal/database"
	pperrors "github.com/pulsepoint/pulsepoint/pkg/errors"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// PulsePointStateManager manages sync state persistence
type PulsePointStateManager struct {
	db     *database.DB
	logger *zap.Logger
	config *StateManagerConfig
}

// StateManagerConfig holds configuration for state management
type StateManagerConfig struct {
	AutoSave         bool          `json:"auto_save"`
	SaveInterval     time.Duration `json:"save_interval"`
	CompactInterval  time.Duration `json:"compact_interval"`
	RetentionPeriod  time.Duration `json:"retention_period"`
	MaxTransactions  int           `json:"max_transactions"`
	EnableEncryption bool          `json:"enable_encryption"`
}

// NewPulsePointStateManager creates a new state manager
func NewPulsePointStateManager(db *database.DB, logger *zap.Logger, config *StateManagerConfig) *PulsePointStateManager {
	if config == nil {
		config = &StateManagerConfig{
			AutoSave:        true,
			SaveInterval:    30 * time.Second,
			CompactInterval: 24 * time.Hour,
			RetentionPeriod: 30 * 24 * time.Hour, // 30 days
			MaxTransactions: 1000,
		}
	}

	return &PulsePointStateManager{
		db:     db,
		logger: logger.With(zap.String("component", "state_manager")),
		config: config,
	}
}

// Initialize sets up the state manager
func (m *PulsePointStateManager) Initialize(dbPath string) error {
	m.logger.Info("Initializing state manager", zap.String("db_path", dbPath))

	// Database is already initialized via DB instance
	// Just ensure buckets exist
	err := m.db.DB.Update(func(tx *bbolt.Tx) error {
		buckets := []string{
			database.BucketState,
			database.BucketFileState,
			database.BucketTransactions,
		}

		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}

		return nil
	})

	if err != nil {
		return pperrors.NewDatabaseError("failed to initialize state buckets", err)
	}

	// Start auto-save if enabled
	if m.config.AutoSave {
		go m.autoSaveLoop()
	}

	// Start compaction if configured
	if m.config.CompactInterval > 0 {
		go m.compactLoop()
	}

	return nil
}

// SaveState saves the current sync state
func (m *PulsePointStateManager) SaveState(ctx context.Context, state *interfaces.SyncState) error {
	if state == nil {
		return pperrors.NewValidationError("state cannot be nil", nil)
	}

	m.logger.Debug("Saving sync state")

	// Convert to model state for storage
	modelState := &models.SyncState{
		Version:          state.Version,
		LastSyncTime:     state.LastSyncTime,
		LastSuccessTime:  state.LastSuccessTime,
		TotalFiles:       state.TotalFiles,
		SyncedFiles:      state.SyncedFiles,
		PendingFiles:     state.PendingFiles,
		FailedFiles:      state.FailedFiles,
		TotalBytes:       state.TotalBytes,
		SyncedBytes:      state.SyncedBytes,
		CurrentOperation: state.CurrentOperation,
		IsRunning:        state.IsRunning,
		Errors:           state.Errors,
		Metadata:         state.Metadata,
	}

	data, err := json.Marshal(modelState)
	if err != nil {
		return pperrors.NewDatabaseError("failed to marshal state", err)
	}

	err = m.db.DB.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketState))
		if bucket == nil {
			return fmt.Errorf("state bucket not found")
		}
		return bucket.Put([]byte("current"), data)
	})

	if err != nil {
		return pperrors.NewDatabaseError("failed to save state", err)
	}

	m.logger.Debug("Sync state saved successfully")
	return nil
}

// LoadState loads the sync state
func (m *PulsePointStateManager) LoadState(ctx context.Context) (*interfaces.SyncState, error) {
	m.logger.Debug("Loading sync state")

	var data []byte
	err := m.db.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketState))
		if bucket == nil {
			return fmt.Errorf("state bucket not found")
		}
		data = bucket.Get([]byte("current"))
		return nil
	})

	if err != nil {
		return nil, pperrors.NewDatabaseError("failed to load state", err)
	}

	if data == nil {
		m.logger.Debug("No existing state found")
		return nil, nil
	}

	var modelState models.SyncState
	if err := json.Unmarshal(data, &modelState); err != nil {
		return nil, pperrors.NewDatabaseError("failed to unmarshal state", err)
	}

	// Convert to interface state
	state := &interfaces.SyncState{
		Version:          modelState.Version,
		LastSyncTime:     modelState.LastSyncTime,
		LastSuccessTime:  modelState.LastSuccessTime,
		TotalFiles:       modelState.TotalFiles,
		SyncedFiles:      modelState.SyncedFiles,
		PendingFiles:     modelState.PendingFiles,
		FailedFiles:      modelState.FailedFiles,
		TotalBytes:       modelState.TotalBytes,
		SyncedBytes:      modelState.SyncedBytes,
		CurrentOperation: modelState.CurrentOperation,
		IsRunning:        modelState.IsRunning,
		Errors:           modelState.Errors,
		Metadata:         modelState.Metadata,
	}

	m.logger.Debug("Sync state loaded successfully")
	return state, nil
}

// UpdateFileState updates state for a specific file
func (m *PulsePointStateManager) UpdateFileState(ctx context.Context, file *interfaces.FileState) error {
	if file == nil || file.Path == "" {
		return pperrors.NewValidationError("invalid file state", nil)
	}

	m.logger.Debug("Updating file state", zap.String("path", file.Path))

	// Convert to model file state
	modelFile := &models.FileState{
		Path:          file.Path,
		LocalHash:     file.LocalHash,
		RemoteHash:    file.RemoteHash,
		LocalModTime:  file.LocalModTime,
		RemoteModTime: file.RemoteModTime,
		Size:          file.Size,
		Status:        models.FileSyncStatus(file.Status),
		LastSyncTime:  file.LastSyncTime,
		LastError:     file.LastError,
		RetryCount:    file.RetryCount,
		RemoteID:      file.RemoteID,
		Version:       file.Version,
		Metadata:      file.Metadata,
	}

	data, err := json.Marshal(modelFile)
	if err != nil {
		return pperrors.NewDatabaseError("failed to marshal file state", err)
	}

	err = m.db.DB.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketFileState))
		if bucket == nil {
			return fmt.Errorf("file state bucket not found")
		}
		return bucket.Put([]byte(file.Path), data)
	})

	if err != nil {
		return pperrors.NewDatabaseError("failed to update file state", err)
	}

	return nil
}

// GetFileState retrieves state for a specific file
func (m *PulsePointStateManager) GetFileState(ctx context.Context, path string) (*interfaces.FileState, error) {
	m.logger.Debug("Getting file state", zap.String("path", path))

	var data []byte
	err := m.db.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketFileState))
		if bucket == nil {
			return fmt.Errorf("file state bucket not found")
		}
		data = bucket.Get([]byte(path))
		return nil
	})

	if err != nil {
		return nil, pperrors.NewDatabaseError("failed to get file state", err)
	}

	if data == nil {
		return nil, nil
	}

	var modelFile models.FileState
	if err := json.Unmarshal(data, &modelFile); err != nil {
		return nil, pperrors.NewDatabaseError("failed to unmarshal file state", err)
	}

	// Convert to interface file state
	fileState := &interfaces.FileState{
		Path:          modelFile.Path,
		LocalHash:     modelFile.LocalHash,
		RemoteHash:    modelFile.RemoteHash,
		LocalModTime:  modelFile.LocalModTime,
		RemoteModTime: modelFile.RemoteModTime,
		Size:          modelFile.Size,
		Status:        interfaces.FileSyncStatus(modelFile.Status),
		LastSyncTime:  modelFile.LastSyncTime,
		LastError:     modelFile.LastError,
		RetryCount:    modelFile.RetryCount,
		RemoteID:      modelFile.RemoteID,
		Version:       modelFile.Version,
		Metadata:      modelFile.Metadata,
	}

	return fileState, nil
}

// DeleteFileState removes state for a specific file
func (m *PulsePointStateManager) DeleteFileState(ctx context.Context, path string) error {
	m.logger.Debug("Deleting file state", zap.String("path", path))

	err := m.db.DB.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketFileState))
		if bucket == nil {
			return fmt.Errorf("file state bucket not found")
		}
		return bucket.Delete([]byte(path))
	})

	if err != nil {
		return pperrors.NewDatabaseError("failed to delete file state", err)
	}

	return nil
}

// ListFileStates lists all file states
func (m *PulsePointStateManager) ListFileStates(ctx context.Context) ([]*interfaces.FileState, error) {
	m.logger.Debug("Listing all file states")

	var states []*interfaces.FileState

	err := m.db.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketFileState))
		if bucket == nil {
			return fmt.Errorf("file state bucket not found")
		}

		return bucket.ForEach(func(k, v []byte) error {
			var modelFile models.FileState
			if err := json.Unmarshal(v, &modelFile); err != nil {
				m.logger.Warn("Failed to unmarshal file state",
					zap.String("path", string(k)),
					zap.Error(err),
				)
				return nil // Continue iteration
			}

			fileState := &interfaces.FileState{
				Path:          modelFile.Path,
				LocalHash:     modelFile.LocalHash,
				RemoteHash:    modelFile.RemoteHash,
				LocalModTime:  modelFile.LocalModTime,
				RemoteModTime: modelFile.RemoteModTime,
				Size:          modelFile.Size,
				Status:        interfaces.FileSyncStatus(modelFile.Status),
				LastSyncTime:  modelFile.LastSyncTime,
				LastError:     modelFile.LastError,
				RetryCount:    modelFile.RetryCount,
				RemoteID:      modelFile.RemoteID,
				Version:       modelFile.Version,
				Metadata:      modelFile.Metadata,
			}

			states = append(states, fileState)
			return nil
		})
	})

	if err != nil {
		return nil, pperrors.NewDatabaseError("failed to list file states", err)
	}

	m.logger.Debug("Listed file states", zap.Int("count", len(states)))
	return states, nil
}

// SaveTransaction saves a sync transaction
func (m *PulsePointStateManager) SaveTransaction(ctx context.Context, transaction *interfaces.SyncTransaction) error {
	if transaction == nil || transaction.ID == "" {
		return pperrors.NewValidationError("invalid transaction", nil)
	}

	m.logger.Debug("Saving transaction", zap.String("id", transaction.ID))

	data, err := json.Marshal(transaction)
	if err != nil {
		return pperrors.NewDatabaseError("failed to marshal transaction", err)
	}

	err = m.db.DB.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketTransactions))
		if bucket == nil {
			return fmt.Errorf("transactions bucket not found")
		}

		// Use transaction ID as key
		return bucket.Put([]byte(transaction.ID), data)
	})

	if err != nil {
		return pperrors.NewDatabaseError("failed to save transaction", err)
	}

	// Clean up old transactions if limit exceeded
	go m.cleanupOldTransactions()

	return nil
}

// GetTransaction retrieves a specific transaction
func (m *PulsePointStateManager) GetTransaction(ctx context.Context, id string) (*interfaces.SyncTransaction, error) {
	m.logger.Debug("Getting transaction", zap.String("id", id))

	var data []byte
	err := m.db.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketTransactions))
		if bucket == nil {
			return fmt.Errorf("transactions bucket not found")
		}
		data = bucket.Get([]byte(id))
		return nil
	})

	if err != nil {
		return nil, pperrors.NewDatabaseError("failed to get transaction", err)
	}

	if data == nil {
		return nil, nil
	}

	var transaction interfaces.SyncTransaction
	if err := json.Unmarshal(data, &transaction); err != nil {
		return nil, pperrors.NewDatabaseError("failed to unmarshal transaction", err)
	}

	return &transaction, nil
}

// ListTransactions lists transactions with pagination
func (m *PulsePointStateManager) ListTransactions(ctx context.Context, offset, limit int) ([]*interfaces.SyncTransaction, error) {
	m.logger.Debug("Listing transactions", zap.Int("offset", offset), zap.Int("limit", limit))

	var transactions []*interfaces.SyncTransaction
	count := 0
	skipped := 0

	err := m.db.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketTransactions))
		if bucket == nil {
			return fmt.Errorf("transactions bucket not found")
		}

		cursor := bucket.Cursor()
		for k, v := cursor.Last(); k != nil && count < limit; k, v = cursor.Prev() {
			if skipped < offset {
				skipped++
				continue
			}

			var transaction interfaces.SyncTransaction
			if err := json.Unmarshal(v, &transaction); err != nil {
				m.logger.Warn("Failed to unmarshal transaction",
					zap.String("id", string(k)),
					zap.Error(err),
				)
				continue
			}

			transactions = append(transactions, &transaction)
			count++
		}

		return nil
	})

	if err != nil {
		return nil, pperrors.NewDatabaseError("failed to list transactions", err)
	}

	m.logger.Debug("Listed transactions", zap.Int("count", len(transactions)))
	return transactions, nil
}

// Cleanup performs cleanup of old state data
func (m *PulsePointStateManager) Cleanup(ctx context.Context, before time.Time) error {
	m.logger.Info("Cleaning up old state data", zap.Time("before", before))

	deleted := 0

	err := m.db.DB.Update(func(tx *bbolt.Tx) error {
		// Clean up old transactions
		bucket := tx.Bucket([]byte(database.BucketTransactions))
		if bucket == nil {
			return fmt.Errorf("transactions bucket not found")
		}

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var transaction interfaces.SyncTransaction
			if err := json.Unmarshal(v, &transaction); err != nil {
				continue
			}

			if transaction.EndTime.Before(before) {
				if err := cursor.Delete(); err != nil {
					m.logger.Warn("Failed to delete transaction",
						zap.String("id", string(k)),
						zap.Error(err),
					)
				} else {
					deleted++
				}
			}
		}

		return nil
	})

	if err != nil {
		return pperrors.NewDatabaseError("failed to cleanup state data", err)
	}

	m.logger.Info("Cleanup completed", zap.Int("deleted", deleted))
	return nil
}

// Export exports state to a file
func (m *PulsePointStateManager) Export(ctx context.Context, path string) error {
	m.logger.Info("Exporting state", zap.String("path", path))

	// Use database backup functionality
	return m.db.Backup(path)
}

// Import imports state from a file
func (m *PulsePointStateManager) Import(ctx context.Context, path string) error {
	m.logger.Info("Importing state", zap.String("path", path))

	// Use database restore functionality
	return m.db.Restore(path)
}

// Reset resets all state data
func (m *PulsePointStateManager) Reset(ctx context.Context) error {
	m.logger.Warn("Resetting all state data")

	err := m.db.DB.Update(func(tx *bbolt.Tx) error {
		buckets := []string{
			database.BucketState,
			database.BucketFileState,
			database.BucketTransactions,
		}

		for _, bucketName := range buckets {
			if err := tx.DeleteBucket([]byte(bucketName)); err != nil && err != bbolt.ErrBucketNotFound {
				return fmt.Errorf("failed to delete bucket %s: %w", bucketName, err)
			}

			if _, err := tx.CreateBucket([]byte(bucketName)); err != nil {
				return fmt.Errorf("failed to recreate bucket %s: %w", bucketName, err)
			}
		}

		return nil
	})

	if err != nil {
		return pperrors.NewDatabaseError("failed to reset state", err)
	}

	m.logger.Info("State reset completed")
	return nil
}

// Close closes the state manager
func (m *PulsePointStateManager) Close() error {
	m.logger.Info("Closing state manager")
	// Database closing is handled by the DB instance
	return nil
}

// GetStatistics returns sync statistics
func (m *PulsePointStateManager) GetStatistics(ctx context.Context) (*interfaces.SyncStatistics, error) {
	m.logger.Debug("Getting sync statistics")

	stats := &interfaces.SyncStatistics{
		Since: time.Now().Add(-30 * 24 * time.Hour), // Default to last 30 days
	}

	// Count transactions
	err := m.db.DB.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(database.BucketTransactions))
		if bucket == nil {
			return fmt.Errorf("transactions bucket not found")
		}

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var transaction interfaces.SyncTransaction
			if err := json.Unmarshal(v, &transaction); err != nil {
				continue
			}

			stats.TotalSyncs++

			if transaction.Status == interfaces.TransactionStatusCompleted {
				stats.SuccessfulSyncs++
			} else if transaction.Status == interfaces.TransactionStatusFailed {
				stats.FailedSyncs++
			}

			stats.TotalBytes += transaction.BytesTransferred

			if transaction.Result != nil {
				stats.TotalFiles += int64(transaction.Result.FilesProcessed)
			}
		}

		return nil
	})

	if err != nil {
		return nil, pperrors.NewDatabaseError("failed to get statistics", err)
	}

	// Calculate average speed
	if stats.SuccessfulSyncs > 0 && stats.TotalBytes > 0 {
		stats.AverageSpeed = float64(stats.TotalBytes) / float64(stats.SuccessfulSyncs) / (1024 * 1024)
	}

	return stats, nil
}

// autoSaveLoop automatically saves state at intervals
func (m *PulsePointStateManager) autoSaveLoop() {
	ticker := time.NewTicker(m.config.SaveInterval)
	defer ticker.Stop()

	for range ticker.C {
		// Auto-save is handled by the sync engine
		// This is just a placeholder for future enhancements
	}
}

// compactLoop periodically compacts the database
func (m *PulsePointStateManager) compactLoop() {
	ticker := time.NewTicker(m.config.CompactInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := m.db.Compact(); err != nil {
			m.logger.Error("Failed to compact database", zap.Error(err))
		} else {
			m.logger.Info("Database compacted successfully")
		}
	}
}

// cleanupOldTransactions removes old transactions
func (m *PulsePointStateManager) cleanupOldTransactions() {
	ctx := context.Background()
	before := time.Now().Add(-m.config.RetentionPeriod)

	if err := m.Cleanup(ctx, before); err != nil {
		m.logger.Error("Failed to cleanup old transactions", zap.Error(err))
	}
}
