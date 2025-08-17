package watchers

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/pulsepoint/pulsepoint/internal/watchers/ignore"
	"github.com/pulsepoint/pulsepoint/internal/watchers/local"
	"github.com/pulsepoint/pulsepoint/internal/watchers/queue"
	"github.com/pulsepoint/pulsepoint/pkg/logger"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// PulsePointWatcherManager manages file watching and change processing
type PulsePointWatcherManager struct {
	watcher       interfaces.FileWatcher
	changeQueue   *queue.PulsePointChangeQueue
	ignoreMatcher *ignore.PulsePointIgnoreMatcher
	db            *bbolt.DB
	syncHandler   func([]*models.ChangeEvent) error
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	logger        *zap.Logger
	isRunning     bool
	runningMu     sync.RWMutex
}

// ManagerConfig contains configuration for the watcher manager
type ManagerConfig struct {
	DebouncePeriod time.Duration                     // Debounce period for file events
	HashAlgorithm  string                            // Hash algorithm to use (md5 or sha256)
	MaxQueueSize   int                               // Maximum queue size
	BatchSize      int                               // Batch size for processing
	FlushInterval  time.Duration                     // Interval to flush changes
	SyncHandler    func([]*models.ChangeEvent) error // Handler for processing changes
	IgnoreFile     string                            // Path to ignore file (e.g., .gitignore)
}

// NewPulsePointWatcherManager creates a new watcher manager
func NewPulsePointWatcherManager(db *bbolt.DB, config ManagerConfig) (*PulsePointWatcherManager, error) {
	// Set defaults
	if config.DebouncePeriod == 0 {
		config.DebouncePeriod = 100 * time.Millisecond
	}
	if config.HashAlgorithm == "" {
		config.HashAlgorithm = "sha256"
	}
	if config.MaxQueueSize == 0 {
		config.MaxQueueSize = 10000
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}

	// Create file watcher
	watcher, err := local.NewPulsePointWatcher(config.DebouncePeriod, config.HashAlgorithm)
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Create ignore matcher
	ignoreMatcher := ignore.NewPulsePointIgnoreMatcher()
	if config.IgnoreFile != "" {
		if err := ignoreMatcher.LoadFromFile(config.IgnoreFile); err != nil {
			logger.Get().Warn("Failed to load ignore file",
				zap.String("file", config.IgnoreFile),
				zap.Error(err),
			)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &PulsePointWatcherManager{
		watcher:       watcher,
		ignoreMatcher: ignoreMatcher,
		db:            db,
		syncHandler:   config.SyncHandler,
		ctx:           ctx,
		cancel:        cancel,
		logger:        logger.Get(),
		isRunning:     false,
	}

	// Create change queue with the manager's process function
	queueConfig := queue.QueueConfig{
		MaxSize:       config.MaxQueueSize,
		BatchSize:     config.BatchSize,
		FlushInterval: config.FlushInterval,
		ProcessFunc:   manager.pulsePointProcessChanges,
	}

	changeQueue, err := queue.NewPulsePointChangeQueue(db, queueConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create change queue: %w", err)
	}
	manager.changeQueue = changeQueue

	// Set ignore patterns on the watcher
	watcher.SetIgnorePatterns(ignoreMatcher.GetPatterns())

	return manager, nil
}

// Start starts the watcher manager
func (m *PulsePointWatcherManager) Start() error {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if m.isRunning {
		return fmt.Errorf("watcher manager is already running")
	}

	// Start the file watcher with empty initial paths
	if err := m.watcher.Start(m.ctx, []string{}); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}

	// Start the change queue
	if err := m.changeQueue.Start(); err != nil {
		m.watcher.Stop()
		return fmt.Errorf("failed to start change queue: %w", err)
	}

	// Start the event processor
	m.wg.Add(1)
	go m.pulsePointEventProcessor()

	m.isRunning = true
	m.logger.Info("PulsePoint watcher manager started")

	return nil
}

// Stop stops the watcher manager
func (m *PulsePointWatcherManager) Stop() error {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if !m.isRunning {
		return nil
	}

	// Cancel context
	m.cancel()

	// Stop components
	if err := m.watcher.Stop(); err != nil {
		m.logger.Error("Failed to stop file watcher", zap.Error(err))
	}

	if err := m.changeQueue.Stop(); err != nil {
		m.logger.Error("Failed to stop change queue", zap.Error(err))
	}

	// Wait for goroutines
	m.wg.Wait()

	m.isRunning = false
	m.logger.Info("PulsePoint watcher manager stopped")

	return nil
}

// WatchPath adds a path to watch
func (m *PulsePointWatcherManager) WatchPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if should ignore
	if m.ignoreMatcher.ShouldIgnore(absPath, false) {
		m.logger.Info("Path is ignored", zap.String("path", absPath))
		return nil
	}

	return m.watcher.AddPath(absPath)
}

// UnwatchPath removes a path from watching
func (m *PulsePointWatcherManager) UnwatchPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	return m.watcher.RemovePath(absPath)
}

// AddIgnorePatterns adds ignore patterns
func (m *PulsePointWatcherManager) AddIgnorePatterns(patterns []string) error {
	m.ignoreMatcher.AddPatterns(patterns)
	return m.watcher.SetIgnorePatterns(m.ignoreMatcher.GetPatterns())
}

// GetStats returns statistics about the watcher manager
func (m *PulsePointWatcherManager) GetStats() map[string]interface{} {
	m.runningMu.RLock()
	defer m.runningMu.RUnlock()

	stats := map[string]interface{}{
		"is_running":      m.isRunning,
		"watched_paths":   m.watcher.GetWatchedPaths(),
		"ignore_patterns": m.ignoreMatcher.GetPatterns(),
		"queue_stats":     m.changeQueue.GetQueueStats(),
	}

	return stats
}

// pulsePointEventProcessor processes events from the file watcher
func (m *PulsePointWatcherManager) pulsePointEventProcessor() {
	defer m.wg.Done()

	eventsChan := m.watcher.Watch()
	errorsChan := m.watcher.Errors()

	for {
		select {
		case <-m.ctx.Done():
			return

		case event, ok := <-eventsChan:
			if !ok {
				return
			}

			// Additional filtering with ignore matcher
			if m.ignoreMatcher.ShouldIgnore(event.Path, event.IsDir) {
				m.logger.Debug("Ignoring event for path",
					zap.String("path", event.Path),
					zap.String("type", string(event.Type)),
				)
				continue
			}

			// Convert interfaces.ChangeEvent to models.ChangeEvent
			modelEvent := m.pulsePointConvertEvent(event)

			// Add to queue
			if err := m.changeQueue.Add(modelEvent); err != nil {
				m.logger.Error("Failed to add event to queue",
					zap.String("path", event.Path),
					zap.Error(err),
				)
			}

		case err, ok := <-errorsChan:
			if !ok {
				return
			}
			m.logger.Error("File watcher error", zap.Error(err))
		}
	}
}

// pulsePointProcessChanges processes a batch of changes
func (m *PulsePointWatcherManager) pulsePointProcessChanges(events []*models.ChangeEvent) error {
	if m.syncHandler == nil {
		m.logger.Warn("No sync handler configured, skipping change processing")
		return nil
	}

	m.logger.Info("Processing changes",
		zap.Int("count", len(events)),
	)

	// Group events by type for logging
	typeCounts := make(map[models.ChangeType]int)
	for _, event := range events {
		typeCounts[event.Type]++
	}

	for changeType, count := range typeCounts {
		m.logger.Debug("Change type summary",
			zap.String("type", string(changeType)),
			zap.Int("count", count),
		)
	}

	// Call the sync handler
	return m.syncHandler(events)
}

// IsRunning returns whether the manager is running
func (m *PulsePointWatcherManager) IsRunning() bool {
	m.runningMu.RLock()
	defer m.runningMu.RUnlock()
	return m.isRunning
}

// GetQueuedChanges returns the number of queued changes
func (m *PulsePointWatcherManager) GetQueuedChanges() int {
	return m.changeQueue.GetPendingCount()
}

// GetProcessingChanges returns the number of changes being processed
func (m *PulsePointWatcherManager) GetProcessingChanges() int {
	return m.changeQueue.GetProcessingCount()
}

// ClearQueue clears all pending changes from the queue
func (m *PulsePointWatcherManager) ClearQueue() error {
	return m.changeQueue.Clear()
}

// pulsePointConvertEvent converts an interfaces.ChangeEvent to models.ChangeEvent
func (m *PulsePointWatcherManager) pulsePointConvertEvent(event interfaces.ChangeEvent) *models.ChangeEvent {
	return &models.ChangeEvent{
		ID:        fmt.Sprintf("%d-%s", time.Now().UnixNano(), event.Path),
		Type:      models.ChangeType(event.Type),
		Source:    "local",
		Path:      event.Path,
		OldPath:   event.OldPath,
		Timestamp: time.Unix(event.Timestamp, 0),
		Size:      event.Size,
		Hash:      event.Hash,
		IsDir:     event.IsDir,
		Status:    models.EventStatusPending,
		Processed: false,
		Retries:   0,
		Metadata:  make(map[string]interface{}),
	}
}
