package repositories

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/database"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	bolt "go.etcd.io/bbolt"
)

const (
	// StateKeyMain is the key for the main sync state
	StateKeyMain = "main"

	// StateKeyFilePrefix is the prefix for file state keys
	StateKeyFilePrefix = "file:"
)

// StateRepository manages state data in the database
type StateRepository struct {
	db *database.Manager
}

// NewStateRepository creates a new state repository
func NewStateRepository(db *database.Manager) *StateRepository {
	return &StateRepository{db: db}
}

// SaveSyncState saves the main sync state
func (r *StateRepository) SaveSyncState(state *models.SyncState) error {
	state.LastSyncTime = time.Now()
	return r.db.Put(database.BucketState, StateKeyMain, state)
}

// GetSyncState retrieves the main sync state
func (r *StateRepository) GetSyncState() (*models.SyncState, error) {
	var state models.SyncState
	err := r.db.Get(database.BucketState, StateKeyMain, &state)
	if err != nil {
		// If state doesn't exist, return a new one
		if err.Error() == fmt.Sprintf("key %s not found in bucket %s", StateKeyMain, database.BucketState) {
			return models.NewSyncState(), nil
		}
		return nil, err
	}
	return &state, nil
}

// SaveFileState saves a file state
func (r *StateRepository) SaveFileState(state *models.FileState) error {
	key := StateKeyFilePrefix + state.Path
	state.LastCheckTime = time.Now()
	return r.db.Put(database.BucketState, key, state)
}

// GetFileState retrieves a file state
func (r *StateRepository) GetFileState(path string) (*models.FileState, error) {
	key := StateKeyFilePrefix + path
	var state models.FileState
	err := r.db.Get(database.BucketState, key, &state)
	if err != nil {
		// If state doesn't exist, return a new one
		if err.Error() == fmt.Sprintf("key %s not found in bucket %s", key, database.BucketState) {
			return models.NewFileState(path), nil
		}
		return nil, err
	}
	return &state, nil
}

// DeleteFileState deletes a file state
func (r *StateRepository) DeleteFileState(path string) error {
	key := StateKeyFilePrefix + path
	return r.db.Delete(database.BucketState, key)
}

// ListFileStates lists all file states
func (r *StateRepository) ListFileStates() ([]*models.FileState, error) {
	var states []*models.FileState

	err := r.db.Transaction(false, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketState))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketState)
		}

		prefix := []byte(StateKeyFilePrefix)
		c := b.Cursor()

		for k, v := c.Seek(prefix); k != nil && len(k) > len(prefix); k, v = c.Next() {
			// Skip if not a file state key
			if string(k[:len(prefix)]) != StateKeyFilePrefix {
				continue
			}

			var state models.FileState
			if err := json.Unmarshal(v, &state); err != nil {
				return err
			}
			states = append(states, &state)
		}

		return nil
	})

	return states, err
}

// ListFileStatesByStatus lists file states with a specific status
func (r *StateRepository) ListFileStatesByStatus(status models.FileSyncStatus) ([]*models.FileState, error) {
	allStates, err := r.ListFileStates()
	if err != nil {
		return nil, err
	}

	var filtered []*models.FileState
	for _, state := range allStates {
		if state.Status == status {
			filtered = append(filtered, state)
		}
	}

	return filtered, nil
}

// GetPendingFileStates returns file states pending synchronization
func (r *StateRepository) GetPendingFileStates() ([]*models.FileState, error) {
	return r.ListFileStatesByStatus(models.FileSyncStatusPending)
}

// GetConflictedFileStates returns file states with conflicts
func (r *StateRepository) GetConflictedFileStates() ([]*models.FileState, error) {
	return r.ListFileStatesByStatus(models.FileSyncStatusConflict)
}

// GetErrorFileStates returns file states with errors
func (r *StateRepository) GetErrorFileStates() ([]*models.FileState, error) {
	return r.ListFileStatesByStatus(models.FileSyncStatusError)
}

// UpdateFileStatus updates the status of a file
func (r *StateRepository) UpdateFileStatus(path string, status models.FileSyncStatus) error {
	state, err := r.GetFileState(path)
	if err != nil {
		return err
	}

	state.Status = status
	state.LastCheckTime = time.Now()

	if status == models.FileSyncStatusSynced {
		state.LastSyncTime = time.Now()
		state.ResetRetry()
	}

	return r.SaveFileState(state)
}

// IncrementFileRetry increments the retry count for a file
func (r *StateRepository) IncrementFileRetry(path string) error {
	state, err := r.GetFileState(path)
	if err != nil {
		return err
	}

	state.IncrementRetry()
	return r.SaveFileState(state)
}

// SetFileConflict marks a file as having a conflict
func (r *StateRepository) SetFileConflict(path string, conflictType string) error {
	state, err := r.GetFileState(path)
	if err != nil {
		return err
	}

	state.SetConflict(conflictType)
	return r.SaveFileState(state)
}

// ResolveFileConflict resolves a file conflict
func (r *StateRepository) ResolveFileConflict(path string) error {
	state, err := r.GetFileState(path)
	if err != nil {
		return err
	}

	state.ResolveConflict()
	return r.SaveFileState(state)
}

// BatchSaveFileStates saves multiple file states in a transaction
func (r *StateRepository) BatchSaveFileStates(states []*models.FileState) error {
	return r.db.Transaction(true, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketState))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketState)
		}

		for _, state := range states {
			key := StateKeyFilePrefix + state.Path
			state.LastCheckTime = time.Now()

			data, err := json.Marshal(state)
			if err != nil {
				return err
			}

			if err := b.Put([]byte(key), data); err != nil {
				return err
			}
		}

		return nil
	})
}

// UpdateSyncProgress updates the sync operation progress
func (r *StateRepository) UpdateSyncProgress(operation string, progress float64) error {
	state, err := r.GetSyncState()
	if err != nil {
		return err
	}

	if state.CurrentOperation != operation {
		state.StartOperation(operation)
	}
	state.UpdateProgress(progress)

	return r.SaveSyncState(state)
}

// EndSyncOperation marks the end of a sync operation
func (r *StateRepository) EndSyncOperation(success bool) error {
	state, err := r.GetSyncState()
	if err != nil {
		return err
	}

	state.EndOperation(success)
	return r.SaveSyncState(state)
}

// AddSyncError adds an error to the sync state
func (r *StateRepository) AddSyncError(errMsg string) error {
	state, err := r.GetSyncState()
	if err != nil {
		return err
	}

	state.AddError(errMsg)
	return r.SaveSyncState(state)
}

// AddSyncWarning adds a warning to the sync state
func (r *StateRepository) AddSyncWarning(warning string) error {
	state, err := r.GetSyncState()
	if err != nil {
		return err
	}

	state.AddWarning(warning)
	return r.SaveSyncState(state)
}

// GetStatistics calculates sync statistics
func (r *StateRepository) GetStatistics() (*SyncStatistics, error) {
	state, err := r.GetSyncState()
	if err != nil {
		return nil, err
	}

	fileStates, err := r.ListFileStates()
	if err != nil {
		return nil, err
	}

	stats := &SyncStatistics{
		TotalFiles:   int64(len(fileStates)),
		TotalSynced:  0,
		TotalPending: 0,
		TotalError:   0,
		TotalBytes:   0,
		SyncedBytes:  0,
	}

	for _, fs := range fileStates {
		stats.TotalBytes += fs.Size

		switch fs.Status {
		case models.FileSyncStatusSynced:
			stats.TotalSynced++
			stats.SyncedBytes += fs.Size
		case models.FileSyncStatusPending:
			stats.TotalPending++
		case models.FileSyncStatusError:
			stats.TotalError++
		}
	}

	// Add info from main state
	stats.LastSyncTime = state.LastSyncTime
	stats.LastSuccessTime = state.LastSuccessTime
	stats.IsRunning = state.IsRunning

	return stats, nil
}

// ClearState clears all state data
func (r *StateRepository) ClearState() error {
	return r.db.Clear(database.BucketState)
}

// ResetSyncState resets the sync state to initial values
func (r *StateRepository) ResetSyncState() error {
	state := models.NewSyncState()
	return r.SaveSyncState(state)
}

// SyncStatistics represents statistics about sync operations
type SyncStatistics struct {
	TotalFiles      int64     `json:"total_files"`
	TotalSynced     int64     `json:"total_synced"`
	TotalPending    int64     `json:"total_pending"`
	TotalError      int64     `json:"total_error"`
	TotalBytes      int64     `json:"total_bytes"`
	SyncedBytes     int64     `json:"synced_bytes"`
	LastSyncTime    time.Time `json:"last_sync_time"`
	LastSuccessTime time.Time `json:"last_success_time"`
	IsRunning       bool      `json:"is_running"`
}
