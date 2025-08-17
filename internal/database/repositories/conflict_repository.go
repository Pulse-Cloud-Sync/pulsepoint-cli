package repositories

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/database"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	bolt "go.etcd.io/bbolt"
)

// ConflictRepository manages conflict data in the database
type ConflictRepository struct {
	db *database.Manager
}

// NewConflictRepository creates a new conflict repository
func NewConflictRepository(db *database.Manager) *ConflictRepository {
	return &ConflictRepository{db: db}
}

// Create creates a new conflict record
func (r *ConflictRepository) Create(conflict *models.Conflict) error {
	if conflict.ID == "" {
		conflict.ID = models.GenerateConflictID(conflict.Path)
	}
	conflict.DetectedAt = time.Now()

	return r.db.Put(database.BucketConflicts, conflict.ID, conflict)
}

// Get retrieves a conflict by ID
func (r *ConflictRepository) Get(id string) (*models.Conflict, error) {
	var conflict models.Conflict
	err := r.db.Get(database.BucketConflicts, id, &conflict)
	if err != nil {
		return nil, err
	}
	return &conflict, nil
}

// GetByPath retrieves conflicts for a specific path
func (r *ConflictRepository) GetByPath(path string) ([]*models.Conflict, error) {
	allConflicts, err := r.List()
	if err != nil {
		return nil, err
	}

	var pathConflicts []*models.Conflict
	for _, conflict := range allConflicts {
		if conflict.Path == path {
			pathConflicts = append(pathConflicts, conflict)
		}
	}

	return pathConflicts, nil
}

// Update updates an existing conflict record
func (r *ConflictRepository) Update(conflict *models.Conflict) error {
	if conflict.ID == "" {
		return fmt.Errorf("conflict ID cannot be empty")
	}

	return r.db.Put(database.BucketConflicts, conflict.ID, conflict)
}

// Delete deletes a conflict record
func (r *ConflictRepository) Delete(id string) error {
	return r.db.Delete(database.BucketConflicts, id)
}

// List lists all conflicts
func (r *ConflictRepository) List() ([]*models.Conflict, error) {
	var conflicts []*models.Conflict

	err := r.db.Transaction(false, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketConflicts))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketConflicts)
		}

		return b.ForEach(func(k, v []byte) error {
			var conflict models.Conflict
			if err := json.Unmarshal(v, &conflict); err != nil {
				return err
			}
			conflicts = append(conflicts, &conflict)
			return nil
		})
	})

	return conflicts, err
}

// ListUnresolved lists all unresolved conflicts
func (r *ConflictRepository) ListUnresolved() ([]*models.Conflict, error) {
	allConflicts, err := r.List()
	if err != nil {
		return nil, err
	}

	var unresolved []*models.Conflict
	for _, conflict := range allConflicts {
		if conflict.ResolutionStatus != models.ResolutionStatusResolved {
			unresolved = append(unresolved, conflict)
		}
	}

	return unresolved, nil
}

// ListByType lists conflicts of a specific type
func (r *ConflictRepository) ListByType(conflictType models.ConflictType) ([]*models.Conflict, error) {
	allConflicts, err := r.List()
	if err != nil {
		return nil, err
	}

	var filtered []*models.Conflict
	for _, conflict := range allConflicts {
		if conflict.Type == conflictType {
			filtered = append(filtered, conflict)
		}
	}

	return filtered, nil
}

// ListBySeverity lists conflicts of a specific severity
func (r *ConflictRepository) ListBySeverity(severity models.ConflictSeverity) ([]*models.Conflict, error) {
	allConflicts, err := r.List()
	if err != nil {
		return nil, err
	}

	var filtered []*models.Conflict
	for _, conflict := range allConflicts {
		if conflict.Severity == severity {
			filtered = append(filtered, conflict)
		}
	}

	return filtered, nil
}

// ListAutoResolvable lists conflicts that can be automatically resolved
func (r *ConflictRepository) ListAutoResolvable() ([]*models.Conflict, error) {
	allConflicts, err := r.List()
	if err != nil {
		return nil, err
	}

	var autoResolvable []*models.Conflict
	for _, conflict := range allConflicts {
		if conflict.CanAutoResolve() {
			autoResolvable = append(autoResolvable, conflict)
		}
	}

	return autoResolvable, nil
}

// ListRequiringUser lists conflicts requiring user interaction
func (r *ConflictRepository) ListRequiringUser() ([]*models.Conflict, error) {
	allConflicts, err := r.List()
	if err != nil {
		return nil, err
	}

	var userRequired []*models.Conflict
	for _, conflict := range allConflicts {
		if conflict.UserRequired && !conflict.IsResolved() {
			userRequired = append(userRequired, conflict)
		}
	}

	return userRequired, nil
}

// Resolve marks a conflict as resolved
func (r *ConflictRepository) Resolve(id string, resolution *models.ConflictResolution) error {
	conflict, err := r.Get(id)
	if err != nil {
		return err
	}

	conflict.SetResolution(resolution)
	conflict.AddHistory(fmt.Sprintf("Resolved with strategy: %s", resolution.Strategy))

	return r.Update(conflict)
}

// MarkAttempted marks that a resolution attempt was made
func (r *ConflictRepository) MarkAttempted(id string) error {
	conflict, err := r.Get(id)
	if err != nil {
		return err
	}

	conflict.MarkAttempted()
	conflict.AddHistory(fmt.Sprintf("Resolution attempt %d of %d", conflict.AttemptCount, conflict.MaxAttempts))

	return r.Update(conflict)
}

// Count returns the total number of conflicts
func (r *ConflictRepository) Count() (int, error) {
	return r.db.Count(database.BucketConflicts)
}

// CountUnresolved returns the number of unresolved conflicts
func (r *ConflictRepository) CountUnresolved() (int, error) {
	unresolved, err := r.ListUnresolved()
	if err != nil {
		return 0, err
	}
	return len(unresolved), nil
}

// CountByType returns the number of conflicts of a specific type
func (r *ConflictRepository) CountByType(conflictType models.ConflictType) (int, error) {
	conflicts, err := r.ListByType(conflictType)
	if err != nil {
		return 0, err
	}
	return len(conflicts), nil
}

// GetOldestUnresolved returns the oldest unresolved conflict
func (r *ConflictRepository) GetOldestUnresolved() (*models.Conflict, error) {
	unresolved, err := r.ListUnresolved()
	if err != nil {
		return nil, err
	}

	if len(unresolved) == 0 {
		return nil, fmt.Errorf("no unresolved conflicts found")
	}

	oldest := unresolved[0]
	for _, conflict := range unresolved {
		if conflict.DetectedAt.Before(oldest.DetectedAt) {
			oldest = conflict
		}
	}

	return oldest, nil
}

// BatchResolve resolves multiple conflicts in a transaction
func (r *ConflictRepository) BatchResolve(resolutions map[string]*models.ConflictResolution) error {
	return r.db.Transaction(true, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketConflicts))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketConflicts)
		}

		for id, resolution := range resolutions {
			// Get the conflict
			data := b.Get([]byte(id))
			if data == nil {
				continue // Skip if conflict doesn't exist
			}

			var conflict models.Conflict
			if err := json.Unmarshal(data, &conflict); err != nil {
				return err
			}

			// Apply resolution
			conflict.SetResolution(resolution)
			conflict.AddHistory(fmt.Sprintf("Batch resolved with strategy: %s", resolution.Strategy))

			// Save back
			updatedData, err := json.Marshal(conflict)
			if err != nil {
				return err
			}

			if err := b.Put([]byte(id), updatedData); err != nil {
				return err
			}
		}

		return nil
	})
}

// Clear removes all conflicts from the database
func (r *ConflictRepository) Clear() error {
	return r.db.Clear(database.BucketConflicts)
}

// ClearResolved removes all resolved conflicts
func (r *ConflictRepository) ClearResolved() error {
	resolved, err := r.List()
	if err != nil {
		return err
	}

	var toDelete []string
	for _, conflict := range resolved {
		if conflict.IsResolved() {
			toDelete = append(toDelete, conflict.ID)
		}
	}

	return r.db.Transaction(true, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(database.BucketConflicts))
		if b == nil {
			return fmt.Errorf("bucket %s not found", database.BucketConflicts)
		}

		for _, id := range toDelete {
			if err := b.Delete([]byte(id)); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetStatistics returns conflict statistics
func (r *ConflictRepository) GetStatistics() (*ConflictStatistics, error) {
	conflicts, err := r.List()
	if err != nil {
		return nil, err
	}

	stats := &ConflictStatistics{
		Total:              len(conflicts),
		Resolved:           0,
		Unresolved:         0,
		AutoResolvable:     0,
		UserRequired:       0,
		ByType:             make(map[models.ConflictType]int),
		BySeverity:         make(map[models.ConflictSeverity]int),
		ByResolutionStatus: make(map[models.ResolutionStatus]int),
	}

	for _, conflict := range conflicts {
		// Count by resolution status
		stats.ByResolutionStatus[conflict.ResolutionStatus]++

		if conflict.IsResolved() {
			stats.Resolved++
		} else {
			stats.Unresolved++

			if conflict.CanAutoResolve() {
				stats.AutoResolvable++
			}
			if conflict.UserRequired {
				stats.UserRequired++
			}
		}

		// Count by type
		stats.ByType[conflict.Type]++

		// Count by severity
		stats.BySeverity[conflict.Severity]++
	}

	return stats, nil
}

// ConflictStatistics represents statistics about conflicts
type ConflictStatistics struct {
	Total              int                             `json:"total"`
	Resolved           int                             `json:"resolved"`
	Unresolved         int                             `json:"unresolved"`
	AutoResolvable     int                             `json:"auto_resolvable"`
	UserRequired       int                             `json:"user_required"`
	ByType             map[models.ConflictType]int     `json:"by_type"`
	BySeverity         map[models.ConflictSeverity]int `json:"by_severity"`
	ByResolutionStatus map[models.ResolutionStatus]int `json:"by_resolution_status"`
}
