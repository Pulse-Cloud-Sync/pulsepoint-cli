// Package repositories provides database repository implementations
package repositories

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/database"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	bolt "go.etcd.io/bbolt"
)

// FileRepository manages file data in the database
type FileRepository struct {
	db *database.Manager
}

// NewFileRepository creates a new file repository
func NewFileRepository(db *database.Manager) *FileRepository {
	return &FileRepository{db: db}
}

// Create creates a new file record
func (r *FileRepository) Create(file *models.File) error {
	if file.ID == "" {
		file.ID = models.GenerateFileID(file.Path)
	}
	file.CreatedTime = time.Now()
	file.ModifiedTime = time.Now()

	return r.db.Put(database.BucketFiles, file.ID, file)
}

// Get retrieves a file by ID
func (r *FileRepository) Get(id string) (*models.File, error) {
	var file models.File
	err := r.db.Get(database.BucketFiles, id, &file)
	if err != nil {
		return nil, err
	}
	return &file, nil
}

// GetByPath retrieves a file by path
func (r *FileRepository) GetByPath(path string) (*models.File, error) {
	id := models.GenerateFileID(path)
	return r.Get(id)
}

// Update updates an existing file record
func (r *FileRepository) Update(file *models.File) error {
	if file.ID == "" {
		return fmt.Errorf("file ID cannot be empty")
	}
	file.ModifiedTime = time.Now()
	file.Version++

	return r.db.Put(database.BucketFiles, file.ID, file)
}

// Delete deletes a file record
func (r *FileRepository) Delete(id string) error {
	return r.db.Delete(database.BucketFiles, id)
}

// DeleteByPath deletes a file by path
func (r *FileRepository) DeleteByPath(path string) error {
	id := models.GenerateFileID(path)
	return r.Delete(id)
}

// List lists all files
func (r *FileRepository) List() ([]*models.File, error) {
	var files []*models.File

	err := r.db.Transaction(false, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketFiles))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketFiles)
		}

		return b.ForEach(func(k, v []byte) error {
			var file models.File
			if err := json.Unmarshal(v, &file); err != nil {
				return err
			}
			files = append(files, &file)
			return nil
		})
	})

	return files, err
}

// ListByFilter lists files matching a filter
func (r *FileRepository) ListByFilter(filter *models.FileFilter) ([]*models.File, error) {
	allFiles, err := r.List()
	if err != nil {
		return nil, err
	}

	var filtered []*models.File
	for _, file := range allFiles {
		if r.matchesFilter(file, filter) {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

// matchesFilter checks if a file matches the filter criteria
func (r *FileRepository) matchesFilter(file *models.File, filter *models.FileFilter) bool {
	if filter == nil {
		return true
	}

	// Check path filter
	if filter.Path != "" && file.Path != filter.Path {
		return false
	}

	// Check parent ID filter
	if filter.ParentID != "" && file.ParentID != filter.ParentID {
		return false
	}

	// Check folder filter
	if filter.IsFolder != nil && file.IsFolder != *filter.IsFolder {
		return false
	}

	// Check size filters
	if filter.MinSize > 0 && file.Size < filter.MinSize {
		return false
	}
	if filter.MaxSize > 0 && file.Size > filter.MaxSize {
		return false
	}

	// Check modification time filters
	if !filter.ModifiedAfter.IsZero() && file.ModifiedTime.Before(filter.ModifiedAfter) {
		return false
	}
	if !filter.ModifiedBefore.IsZero() && file.ModifiedTime.After(filter.ModifiedBefore) {
		return false
	}

	// Check MIME type filters
	if len(filter.MimeTypes) > 0 {
		found := false
		for _, mimeType := range filter.MimeTypes {
			if file.MimeType == mimeType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check sync status filter
	if filter.SyncStatus != "" && file.SyncStatus != filter.SyncStatus {
		return false
	}

	return true
}

// Count returns the total number of files
func (r *FileRepository) Count() (int, error) {
	return r.db.Count(database.BucketFiles)
}

// CountByStatus counts files by sync status
func (r *FileRepository) CountByStatus(status string) (int, error) {
	filter := &models.FileFilter{SyncStatus: status}
	files, err := r.ListByFilter(filter)
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// GetPendingFiles returns files pending synchronization
func (r *FileRepository) GetPendingFiles() ([]*models.File, error) {
	filter := &models.FileFilter{SyncStatus: "pending"}
	return r.ListByFilter(filter)
}

// GetModifiedFiles returns files modified after a given time
func (r *FileRepository) GetModifiedFiles(since time.Time) ([]*models.File, error) {
	filter := &models.FileFilter{ModifiedAfter: since}
	return r.ListByFilter(filter)
}

// GetChildFiles returns all child files of a directory
func (r *FileRepository) GetChildFiles(parentID string) ([]*models.File, error) {
	filter := &models.FileFilter{ParentID: parentID}
	return r.ListByFilter(filter)
}

// UpdateSyncStatus updates the sync status of a file
func (r *FileRepository) UpdateSyncStatus(id, status string) error {
	file, err := r.Get(id)
	if err != nil {
		return err
	}

	file.SyncStatus = status
	file.LastSyncTime = time.Now()
	return r.Update(file)
}

// BatchCreate creates multiple files in a transaction
func (r *FileRepository) BatchCreate(files []*models.File) error {
	return r.db.Transaction(true, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketFiles))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketFiles)
		}

		for _, file := range files {
			if file.ID == "" {
				file.ID = models.GenerateFileID(file.Path)
			}
			file.CreatedTime = time.Now()
			file.ModifiedTime = time.Now()

			data, err := json.Marshal(file)
			if err != nil {
				return err
			}

			if err := b.Put([]byte(file.ID), data); err != nil {
				return err
			}
		}

		return nil
	})
}

// BatchUpdate updates multiple files in a transaction
func (r *FileRepository) BatchUpdate(files []*models.File) error {
	return r.db.Transaction(true, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketFiles))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketFiles)
		}

		for _, file := range files {
			if file.ID == "" {
				return fmt.Errorf("file ID cannot be empty")
			}
			file.ModifiedTime = time.Now()
			file.Version++

			data, err := json.Marshal(file)
			if err != nil {
				return err
			}

			if err := b.Put([]byte(file.ID), data); err != nil {
				return err
			}
		}

		return nil
	})
}

// BatchDelete deletes multiple files in a transaction
func (r *FileRepository) BatchDelete(ids []string) error {
	return r.db.Transaction(true, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketFiles))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketFiles)
		}

		for _, id := range ids {
			if err := b.Delete([]byte(id)); err != nil {
				return err
			}
		}

		return nil
	})
}

// Clear removes all files from the database
func (r *FileRepository) Clear() error {
	return r.db.Clear(database.BucketFiles)
}
