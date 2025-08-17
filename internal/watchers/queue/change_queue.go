package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pulsepoint/pulsepoint/pkg/logger"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// PulsePointChangeQueue manages a queue of file changes with deduplication
type PulsePointChangeQueue struct {
	db              *bbolt.DB
	items           map[string]*models.ChangeEvent // In-memory cache keyed by path
	itemsMu         sync.RWMutex
	processingItems map[string]bool // Items currently being processed
	processingMu    sync.RWMutex
	maxSize         int
	batchSize       int
	flushInterval   time.Duration
	processFunc     func([]*models.ChangeEvent) error
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	logger          *zap.Logger
}

// QueueConfig contains configuration for the change queue
type QueueConfig struct {
	MaxSize       int                               // Maximum queue size
	BatchSize     int                               // Batch size for processing
	FlushInterval time.Duration                     // Interval to flush pending changes
	ProcessFunc   func([]*models.ChangeEvent) error // Function to process batches
}

// NewPulsePointChangeQueue creates a new change queue
func NewPulsePointChangeQueue(db *bbolt.DB, config QueueConfig) (*PulsePointChangeQueue, error) {
	if config.MaxSize == 0 {
		config.MaxSize = 10000
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	queue := &PulsePointChangeQueue{
		db:              db,
		items:           make(map[string]*models.ChangeEvent),
		processingItems: make(map[string]bool),
		maxSize:         config.MaxSize,
		batchSize:       config.BatchSize,
		flushInterval:   config.FlushInterval,
		processFunc:     config.ProcessFunc,
		ctx:             ctx,
		cancel:          cancel,
		logger:          logger.Get(),
	}

	// Load persisted queue items from database
	if err := queue.pulsePointLoadFromDB(); err != nil {
		return nil, fmt.Errorf("failed to load queue from database: %w", err)
	}

	return queue, nil
}

// Start begins processing the queue
func (q *PulsePointChangeQueue) Start() error {
	q.wg.Add(1)
	go q.pulsePointProcessor()

	q.logger.Info("PulsePoint change queue started",
		zap.Int("max_size", q.maxSize),
		zap.Int("batch_size", q.batchSize),
		zap.Duration("flush_interval", q.flushInterval),
	)

	return nil
}

// Stop stops the queue processor
func (q *PulsePointChangeQueue) Stop() error {
	q.cancel()
	q.wg.Wait()

	// Persist remaining items
	if err := q.pulsePointPersistToDB(); err != nil {
		q.logger.Error("Failed to persist queue on shutdown", zap.Error(err))
	}

	q.logger.Info("PulsePoint change queue stopped")
	return nil
}

// Add adds a change event to the queue with deduplication
func (q *PulsePointChangeQueue) Add(event *models.ChangeEvent) error {
	q.itemsMu.Lock()
	defer q.itemsMu.Unlock()

	// Check if we're at capacity
	if len(q.items) >= q.maxSize {
		return fmt.Errorf("queue is at maximum capacity (%d items)", q.maxSize)
	}

	// Check if item is being processed
	q.processingMu.RLock()
	if q.processingItems[event.Path] {
		q.processingMu.RUnlock()
		q.logger.Debug("Skipping event for path being processed", zap.String("path", event.Path))
		return nil
	}
	q.processingMu.RUnlock()

	// Deduplication logic
	if existing, exists := q.items[event.Path]; exists {
		// Update with newer event based on rules
		if q.pulsePointShouldReplace(existing, event) {
			q.items[event.Path] = event
			q.logger.Debug("Replaced existing event",
				zap.String("path", event.Path),
				zap.String("old_type", string(existing.Type)),
				zap.String("new_type", string(event.Type)),
			)
		} else {
			q.logger.Debug("Keeping existing event",
				zap.String("path", event.Path),
				zap.String("type", string(existing.Type)),
			)
		}
	} else {
		// Add new event
		q.items[event.Path] = event
		q.logger.Debug("Added new event to queue",
			zap.String("path", event.Path),
			zap.String("type", string(event.Type)),
		)
	}

	// Persist to database
	go q.pulsePointPersistToDB()

	return nil
}

// GetPendingCount returns the number of pending items in the queue
func (q *PulsePointChangeQueue) GetPendingCount() int {
	q.itemsMu.RLock()
	defer q.itemsMu.RUnlock()
	return len(q.items)
}

// GetProcessingCount returns the number of items currently being processed
func (q *PulsePointChangeQueue) GetProcessingCount() int {
	q.processingMu.RLock()
	defer q.processingMu.RUnlock()
	return len(q.processingItems)
}

// pulsePointProcessor is the main processing goroutine
func (q *PulsePointChangeQueue) pulsePointProcessor() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			// Process remaining items before exiting
			q.pulsePointProcessBatch()
			return
		case <-ticker.C:
			q.pulsePointProcessBatch()
		}
	}
}

// pulsePointProcessBatch processes a batch of items from the queue
func (q *PulsePointChangeQueue) pulsePointProcessBatch() {
	q.itemsMu.Lock()

	if len(q.items) == 0 {
		q.itemsMu.Unlock()
		return
	}

	// Get batch of items
	batch := make([]*models.ChangeEvent, 0, q.batchSize)
	batchPaths := make([]string, 0, q.batchSize)

	for path, event := range q.items {
		if len(batch) >= q.batchSize {
			break
		}
		batch = append(batch, event)
		batchPaths = append(batchPaths, path)
	}

	// Mark items as processing
	q.processingMu.Lock()
	for _, path := range batchPaths {
		q.processingItems[path] = true
		delete(q.items, path)
	}
	q.processingMu.Unlock()

	q.itemsMu.Unlock()

	// Process the batch
	if q.processFunc != nil && len(batch) > 0 {
		q.logger.Info("Processing batch of changes",
			zap.Int("batch_size", len(batch)),
			zap.Int("remaining", len(q.items)),
		)

		err := q.processFunc(batch)

		if err != nil {
			q.logger.Error("Failed to process batch", zap.Error(err))

			// Re-add failed items back to queue
			q.itemsMu.Lock()
			for i, path := range batchPaths {
				q.items[path] = batch[i]
			}
			q.itemsMu.Unlock()
		}
	}

	// Remove from processing
	q.processingMu.Lock()
	for _, path := range batchPaths {
		delete(q.processingItems, path)
	}
	q.processingMu.Unlock()

	// Update database
	go q.pulsePointPersistToDB()
}

// pulsePointShouldReplace determines if an existing event should be replaced
func (q *PulsePointChangeQueue) pulsePointShouldReplace(existing, new *models.ChangeEvent) bool {
	// Rules for deduplication:
	// 1. Delete always wins
	if new.Type == models.ChangeTypeDelete {
		return true
	}

	// 2. Create + Modify = Create (keep create)
	if existing.Type == models.ChangeTypeCreate && new.Type == models.ChangeTypeModify {
		return false
	}

	// 3. Modify + Modify = Latest Modify
	if existing.Type == models.ChangeTypeModify && new.Type == models.ChangeTypeModify {
		return new.Timestamp.After(existing.Timestamp)
	}

	// 4. Create + Delete = Skip both (handled elsewhere)
	if existing.Type == models.ChangeTypeCreate && new.Type == models.ChangeTypeDelete {
		return true // Will be removed from queue
	}

	// 5. Default: keep newer event
	return new.Timestamp.After(existing.Timestamp)
}

// pulsePointLoadFromDB loads persisted queue items from the database
func (q *PulsePointChangeQueue) pulsePointLoadFromDB() error {
	return q.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("Queue"))
		if bucket == nil {
			return nil // No queue bucket yet
		}

		return bucket.ForEach(func(k, v []byte) error {
			var event models.ChangeEvent
			if err := json.Unmarshal(v, &event); err != nil {
				q.logger.Warn("Failed to unmarshal queued event", zap.Error(err))
				return nil // Skip invalid entries
			}

			q.items[event.Path] = &event
			return nil
		})
	})
}

// pulsePointPersistToDB persists the current queue to the database
func (q *PulsePointChangeQueue) pulsePointPersistToDB() error {
	q.itemsMu.RLock()
	itemsCopy := make(map[string]*models.ChangeEvent)
	for k, v := range q.items {
		itemsCopy[k] = v
	}
	q.itemsMu.RUnlock()

	return q.db.Update(func(tx *bbolt.Tx) error {
		// Clear and recreate the bucket
		if err := tx.DeleteBucket([]byte("Queue")); err != nil && err != bbolt.ErrBucketNotFound {
			return err
		}

		bucket, err := tx.CreateBucketIfNotExists([]byte("Queue"))
		if err != nil {
			return err
		}

		// Save all current items
		for path, event := range itemsCopy {
			data, err := json.Marshal(event)
			if err != nil {
				q.logger.Warn("Failed to marshal event", zap.String("path", path), zap.Error(err))
				continue
			}

			if err := bucket.Put([]byte(path), data); err != nil {
				return err
			}
		}

		return nil
	})
}

// Clear removes all items from the queue
func (q *PulsePointChangeQueue) Clear() error {
	q.itemsMu.Lock()
	q.items = make(map[string]*models.ChangeEvent)
	q.itemsMu.Unlock()

	// Clear from database
	return q.db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket([]byte("Queue"))
	})
}

// GetQueueStats returns statistics about the queue
func (q *PulsePointChangeQueue) GetQueueStats() map[string]interface{} {
	q.itemsMu.RLock()
	q.processingMu.RLock()
	defer q.itemsMu.RUnlock()
	defer q.processingMu.RUnlock()

	stats := map[string]interface{}{
		"pending_count":    len(q.items),
		"processing_count": len(q.processingItems),
		"max_size":         q.maxSize,
		"batch_size":       q.batchSize,
		"flush_interval":   q.flushInterval.String(),
	}

	// Count by change type
	typeCounts := make(map[models.ChangeType]int)
	for _, event := range q.items {
		typeCounts[event.Type]++
	}
	stats["type_counts"] = typeCounts

	return stats
}
