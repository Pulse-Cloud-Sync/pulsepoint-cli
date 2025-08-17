package sync

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/pulsepoint/pulsepoint/internal/database"
	"github.com/pulsepoint/pulsepoint/internal/providers"
	pperrors "github.com/pulsepoint/pulsepoint/pkg/errors"
	pplogger "github.com/pulsepoint/pulsepoint/pkg/logger"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	"go.uber.org/zap"
)

// PulsePointEngine represents the main sync engine
type PulsePointEngine struct {
	// Core components
	provider     interfaces.CloudProvider
	watcher      interfaces.FileWatcher
	strategy     interfaces.SyncStrategy
	stateManager interfaces.StateManager
	db           *database.DB
	logger       *zap.Logger

	// Pipeline components
	pipeline *PulsePointPipeline

	// Configuration
	config *EngineConfig

	// Runtime state
	mu           sync.RWMutex
	isRunning    bool
	isPaused     bool
	currentState *models.SyncState
	stopChan     chan struct{}
	pauseChan    chan struct{}

	// Metrics
	metrics *SyncMetrics
}

// EngineConfig holds configuration for the sync engine
type EngineConfig struct {
	// Sync settings
	SyncInterval       time.Duration `json:"sync_interval"`
	BatchSize          int           `json:"batch_size"`
	MaxConcurrent      int           `json:"max_concurrent"`
	RetryAttempts      int           `json:"retry_attempts"`
	RetryDelay         time.Duration `json:"retry_delay"`
	ConflictResolution string        `json:"conflict_resolution"`

	// Performance settings
	EnableCaching  bool          `json:"enable_caching"`
	CacheTTL       time.Duration `json:"cache_ttl"`
	MaxMemoryUsage int64         `json:"max_memory_usage"`
	BandwidthLimit int64         `json:"bandwidth_limit"`

	// File handling
	MaxFileSize         int64    `json:"max_file_size"`
	IgnorePatterns      []string `json:"ignore_patterns"`
	IncludePatterns     []string `json:"include_patterns"`
	PreserveTimestamps  bool     `json:"preserve_timestamps"`
	PreservePermissions bool     `json:"preserve_permissions"`

	// Advanced settings
	EnableDeltaSync   bool `json:"enable_delta_sync"`
	EnableCompression bool `json:"enable_compression"`
	EnableEncryption  bool `json:"enable_encryption"`
	EnableVersioning  bool `json:"enable_versioning"`
}

// SyncMetrics tracks sync performance metrics
type SyncMetrics struct {
	mu sync.RWMutex

	// Counters
	TotalSyncs      int64
	SuccessfulSyncs int64
	FailedSyncs     int64
	TotalFiles      int64
	TotalBytes      int64

	// Performance
	LastSyncDuration time.Duration
	AverageSpeed     float64
	CurrentSpeed     float64

	// Current operation
	CurrentFiles int64
	CurrentBytes int64
	StartTime    time.Time
}

// NewPulsePointEngine creates a new sync engine instance
func NewPulsePointEngine(
	provider interfaces.CloudProvider,
	watcher interfaces.FileWatcher,
	strategy interfaces.SyncStrategy,
	stateManager interfaces.StateManager,
	db *database.DB,
	config *EngineConfig,
) (*PulsePointEngine, error) {
	if provider == nil || watcher == nil || strategy == nil || stateManager == nil {
		return nil, pperrors.NewValidationError("missing required components", nil)
	}

	logger := pplogger.Get()

	engine := &PulsePointEngine{
		provider:     provider,
		watcher:      watcher,
		strategy:     strategy,
		stateManager: stateManager,
		db:           db,
		logger:       logger,
		config:       config,
		currentState: models.NewSyncState(),
		stopChan:     make(chan struct{}),
		pauseChan:    make(chan struct{}),
		metrics:      &SyncMetrics{StartTime: time.Now()},
	}

	// Initialize pipeline
	engine.pipeline = NewPulsePointPipeline(engine)

	// Load existing state
	if err := engine.loadState(); err != nil {
		logger.Warn("Failed to load existing state", zap.Error(err))
		// Not a fatal error, continue with new state
	}

	return engine, nil
}

// Start starts the sync engine
func (e *PulsePointEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.isRunning {
		e.mu.Unlock()
		return pperrors.NewSyncError("engine already running", nil)
	}
	e.isRunning = true
	e.isPaused = false
	e.mu.Unlock()

	e.logger.Info("Starting PulsePoint sync engine",
		zap.String("strategy", e.strategy.Name()),
		zap.Duration("interval", e.config.SyncInterval),
	)

	// Start file watcher
	if err := e.watcher.Start(ctx, []string{"."}); err != nil {
		e.mu.Lock()
		e.isRunning = false
		e.mu.Unlock()
		return pperrors.NewSyncError("failed to start file watcher", err)
	}

	// Start sync loop
	go e.syncLoop(ctx)

	// Start monitoring file changes
	go e.monitorFileChanges(ctx)

	// Update state
	e.currentState.StartOperation("sync_engine_running")
	e.saveState()

	return nil
}

// Stop stops the sync engine
func (e *PulsePointEngine) Stop() error {
	e.mu.Lock()
	if !e.isRunning {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	e.logger.Info("Stopping PulsePoint sync engine")

	// Signal stop
	close(e.stopChan)

	// Stop file watcher
	if err := e.watcher.Stop(); err != nil {
		e.logger.Error("Failed to stop file watcher", zap.Error(err))
	}

	// Update state
	e.mu.Lock()
	e.isRunning = false
	e.isPaused = false
	e.mu.Unlock()

	e.currentState.EndOperation(true)
	e.saveState()

	return nil
}

// Pause pauses the sync engine
func (e *PulsePointEngine) Pause() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return pperrors.NewSyncError("engine not running", nil)
	}

	if e.isPaused {
		return pperrors.NewSyncError("engine already paused", nil)
	}

	e.isPaused = true
	close(e.pauseChan)

	e.logger.Info("Paused PulsePoint sync engine")
	e.currentState.IsPaused = true
	e.saveState()

	return nil
}

// Resume resumes the sync engine
func (e *PulsePointEngine) Resume() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.isRunning {
		return pperrors.NewSyncError("engine not running", nil)
	}

	if !e.isPaused {
		return pperrors.NewSyncError("engine not paused", nil)
	}

	e.isPaused = false
	e.pauseChan = make(chan struct{})

	e.logger.Info("Resumed PulsePoint sync engine")
	e.currentState.IsPaused = false
	e.saveState()

	return nil
}

// Sync performs a manual sync operation
func (e *PulsePointEngine) Sync(ctx context.Context) (*interfaces.SyncResult, error) {
	e.mu.RLock()
	if !e.isRunning {
		e.mu.RUnlock()
		return nil, pperrors.NewSyncError("engine not running", nil)
	}
	e.mu.RUnlock()

	e.logger.Info("Starting manual sync operation")

	// Create transaction
	transaction := e.createTransaction(interfaces.TransactionTypeFullSync)

	// Execute sync pipeline
	result, err := e.pipeline.Execute(ctx, transaction)
	if err != nil {
		e.logger.Error("Sync operation failed", zap.Error(err))
		e.metrics.recordFailure()
		transaction.Status = interfaces.TransactionStatusFailed
		transaction.EndTime = time.Now()
		e.saveTransaction(transaction)
		return nil, err
	}

	// Update metrics
	e.metrics.recordSuccess(result)

	// Update transaction
	transaction.Status = interfaces.TransactionStatusCompleted
	transaction.EndTime = time.Now()
	transaction.Result = result
	e.saveTransaction(transaction)

	e.logger.Info("Sync operation completed",
		zap.Int("files_processed", result.FilesProcessed),
		zap.Int64("bytes_transferred", result.BytesTransferred),
		zap.Bool("success", result.Success),
	)

	return result, nil
}

// GetStatus returns the current engine status
func (e *PulsePointEngine) GetStatus() (*EngineStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := &EngineStatus{
		IsRunning:        e.isRunning,
		IsPaused:         e.isPaused,
		CurrentOperation: e.currentState.CurrentOperation,
		Progress:         e.currentState.OperationProgress,
		State:            e.currentState,
		Metrics:          e.getMetrics(),
	}

	return status, nil
}

// syncLoop is the main sync loop
func (e *PulsePointEngine) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(e.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-e.pauseChan:
			// Wait until resumed
			<-e.pauseChan
		case <-ticker.C:
			// Check if paused
			e.mu.RLock()
			paused := e.isPaused
			e.mu.RUnlock()

			if !paused {
				// Perform sync
				if _, err := e.Sync(ctx); err != nil {
					e.logger.Error("Scheduled sync failed", zap.Error(err))
				}
			}
		}
	}
}

// monitorFileChanges monitors the file watcher for changes
func (e *PulsePointEngine) monitorFileChanges(ctx context.Context) {
	watchChan := e.watcher.Watch()
	errorChan := e.watcher.Errors()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case event, ok := <-watchChan:
			if !ok {
				e.logger.Warn("Watch channel closed")
				return
			}
			// Handle file change event
			e.handleFileChange(ctx, event)
		case err, ok := <-errorChan:
			if !ok {
				e.logger.Warn("Error channel closed")
				return
			}
			e.logger.Error("File watcher error", zap.Error(err))
		}
	}
}

// handleFileChange handles a file change event
func (e *PulsePointEngine) handleFileChange(ctx context.Context, event interfaces.ChangeEvent) {
	e.mu.RLock()
	paused := e.isPaused
	e.mu.RUnlock()

	if paused {
		return
	}

	e.logger.Info("File change detected",
		zap.String("path", event.Path),
		zap.String("type", string(event.Type)),
	)

	// TODO: Implement actual file change handling
	// This would typically trigger a sync for the changed file
}

// loadState loads the sync state from storage
func (e *PulsePointEngine) loadState() error {
	ctx := context.Background()
	state, err := e.stateManager.LoadState(ctx)
	if err != nil {
		return err
	}

	if state != nil {
		// Convert from interface state to model state
		e.currentState = &models.SyncState{
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
	}

	return nil
}

// saveState saves the sync state to storage
func (e *PulsePointEngine) saveState() error {
	ctx := context.Background()

	// Convert model state to interface state
	state := &interfaces.SyncState{
		Version:          e.currentState.Version,
		LastSyncTime:     e.currentState.LastSyncTime,
		LastSuccessTime:  e.currentState.LastSuccessTime,
		TotalFiles:       e.currentState.TotalFiles,
		SyncedFiles:      e.currentState.SyncedFiles,
		PendingFiles:     e.currentState.PendingFiles,
		FailedFiles:      e.currentState.FailedFiles,
		TotalBytes:       e.currentState.TotalBytes,
		SyncedBytes:      e.currentState.SyncedBytes,
		CurrentOperation: e.currentState.CurrentOperation,
		IsRunning:        e.currentState.IsRunning,
		Errors:           e.currentState.Errors,
		Metadata:         e.currentState.Metadata,
	}

	return e.stateManager.SaveState(ctx, state)
}

// createTransaction creates a new sync transaction
func (e *PulsePointEngine) createTransaction(txType interfaces.TransactionType) *interfaces.SyncTransaction {
	return &interfaces.SyncTransaction{
		ID:        uuid.New().String(),
		StartTime: time.Now(),
		Type:      txType,
		Status:    interfaces.TransactionStatusRunning,
	}
}

// saveTransaction saves a transaction to storage
func (e *PulsePointEngine) saveTransaction(tx *interfaces.SyncTransaction) error {
	ctx := context.Background()
	return e.stateManager.SaveTransaction(ctx, tx)
}

// getMetrics returns current metrics
func (e *PulsePointEngine) getMetrics() *SyncMetrics {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()

	return &SyncMetrics{
		TotalSyncs:       e.metrics.TotalSyncs,
		SuccessfulSyncs:  e.metrics.SuccessfulSyncs,
		FailedSyncs:      e.metrics.FailedSyncs,
		TotalFiles:       e.metrics.TotalFiles,
		TotalBytes:       e.metrics.TotalBytes,
		LastSyncDuration: e.metrics.LastSyncDuration,
		AverageSpeed:     e.metrics.AverageSpeed,
		CurrentSpeed:     e.metrics.CurrentSpeed,
		CurrentFiles:     e.metrics.CurrentFiles,
		CurrentBytes:     e.metrics.CurrentBytes,
		StartTime:        e.metrics.StartTime,
	}
}

// recordSuccess records a successful sync
func (m *SyncMetrics) recordSuccess(result *interfaces.SyncResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalSyncs++
	m.SuccessfulSyncs++
	m.TotalFiles += int64(result.FilesProcessed)
	m.TotalBytes += result.BytesTransferred

	duration := time.Duration(result.EndTime-result.StartTime) * time.Nanosecond
	m.LastSyncDuration = duration

	if duration > 0 {
		mbps := float64(result.BytesTransferred) / (1024 * 1024) / duration.Seconds()
		m.CurrentSpeed = mbps
		// Update average speed
		if m.AverageSpeed == 0 {
			m.AverageSpeed = mbps
		} else {
			m.AverageSpeed = (m.AverageSpeed*float64(m.SuccessfulSyncs-1) + mbps) / float64(m.SuccessfulSyncs)
		}
	}
}

// recordFailure records a failed sync
func (m *SyncMetrics) recordFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalSyncs++
	m.FailedSyncs++
}

// EngineStatus represents the current engine status
type EngineStatus struct {
	IsRunning        bool              `json:"is_running"`
	IsPaused         bool              `json:"is_paused"`
	CurrentOperation string            `json:"current_operation"`
	Progress         float64           `json:"progress"`
	State            *models.SyncState `json:"state"`
	Metrics          *SyncMetrics      `json:"metrics"`
}

// CreateDefaultProvider creates a default cloud provider based on configuration
func CreateDefaultProvider(ctx context.Context) (interfaces.CloudProvider, error) {
	factory := providers.NewPulsePointProviderFactory(ctx)

	// Check if we should use mock provider for testing
	if os.Getenv("PULSEPOINT_USE_MOCK_PROVIDER") == "true" {
		return factory.CreateProvider(providers.Mock)
	}

	// Get configured providers
	configured := factory.GetConfiguredProviders()
	if len(configured) == 0 {
		return nil, pperrors.NewConfigError("no cloud providers configured. Run 'pulsepoint auth google' first", nil)
	}

	// Use the first configured provider (for now, it's Google Drive)
	return factory.CreateProvider(configured[0])
}
