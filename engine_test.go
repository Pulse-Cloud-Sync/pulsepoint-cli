package sync

import (
	"context"
	"testing"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) Initialize(config interfaces.ProviderConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockProvider) Upload(ctx context.Context, file *interfaces.File) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}

func (m *MockProvider) Download(ctx context.Context, path string) (*interfaces.File, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.File), args.Error(1)
}

func (m *MockProvider) Delete(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockProvider) List(ctx context.Context, folder string) ([]*interfaces.File, error) {
	args := m.Called(ctx, folder)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.File), args.Error(1)
}

func (m *MockProvider) GetMetadata(ctx context.Context, path string) (*interfaces.Metadata, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.Metadata), args.Error(1)
}

func (m *MockProvider) CreateFolder(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockProvider) Move(ctx context.Context, sourcePath, destPath string) error {
	args := m.Called(ctx, sourcePath, destPath)
	return args.Error(0)
}

func (m *MockProvider) GetQuota(ctx context.Context) (*interfaces.QuotaInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.QuotaInfo), args.Error(1)
}

func (m *MockProvider) GetProviderName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockProvider) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockProvider) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}

// Mock watcher
type MockWatcher struct {
	mock.Mock
}

func (m *MockWatcher) Start(ctx context.Context, paths []string) error {
	args := m.Called(ctx, paths)
	return args.Error(0)
}

func (m *MockWatcher) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockWatcher) Watch() <-chan interfaces.ChangeEvent {
	args := m.Called()
	return args.Get(0).(<-chan interfaces.ChangeEvent)
}

func (m *MockWatcher) Errors() <-chan error {
	args := m.Called()
	return args.Get(0).(<-chan error)
}

func (m *MockWatcher) AddPath(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockWatcher) RemovePath(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockWatcher) SetIgnorePatterns(patterns []string) error {
	args := m.Called(patterns)
	return args.Error(0)
}

func (m *MockWatcher) GetWatchedPaths() []string {
	args := m.Called()
	if args.Get(0) == nil {
		return []string{}
	}
	return args.Get(0).([]string)
}

func (m *MockWatcher) IsWatching() bool {
	args := m.Called()
	return args.Bool(0)
}

// Mock strategy
type MockStrategy struct {
	mock.Mock
}

func (m *MockStrategy) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockStrategy) Sync(ctx context.Context, source, destination string, changes []interfaces.ChangeEvent) (*interfaces.SyncResult, error) {
	args := m.Called(ctx, source, destination, changes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.SyncResult), args.Error(1)
}

func (m *MockStrategy) ResolveConflict(ctx context.Context, conflict *interfaces.Conflict) (*interfaces.ConflictResolution, error) {
	args := m.Called(ctx, conflict)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.ConflictResolution), args.Error(1)
}

func (m *MockStrategy) ValidateSync(ctx context.Context, source, destination string) error {
	args := m.Called(ctx, source, destination)
	return args.Error(0)
}

func (m *MockStrategy) GetDirection() interfaces.SyncDirection {
	args := m.Called()
	return args.Get(0).(interfaces.SyncDirection)
}

func (m *MockStrategy) SupportsResume() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockStrategy) GetConfiguration() interfaces.StrategyConfig {
	args := m.Called()
	return args.Get(0).(interfaces.StrategyConfig)
}

// Mock state manager
type MockStateManager struct {
	mock.Mock
}

func (m *MockStateManager) Initialize(dbPath string) error {
	args := m.Called(dbPath)
	return args.Error(0)
}

func (m *MockStateManager) SaveState(ctx context.Context, state *interfaces.SyncState) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockStateManager) LoadState(ctx context.Context) (*interfaces.SyncState, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.SyncState), args.Error(1)
}

func (m *MockStateManager) UpdateFileState(ctx context.Context, file *interfaces.FileState) error {
	args := m.Called(ctx, file)
	return args.Error(0)
}

func (m *MockStateManager) GetFileState(ctx context.Context, path string) (*interfaces.FileState, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.FileState), args.Error(1)
}

func (m *MockStateManager) DeleteFileState(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockStateManager) ListFileStates(ctx context.Context) ([]*interfaces.FileState, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.FileState), args.Error(1)
}

func (m *MockStateManager) SaveTransaction(ctx context.Context, transaction *interfaces.SyncTransaction) error {
	args := m.Called(ctx, transaction)
	return args.Error(0)
}

func (m *MockStateManager) GetTransaction(ctx context.Context, id string) (*interfaces.SyncTransaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.SyncTransaction), args.Error(1)
}

func (m *MockStateManager) ListTransactions(ctx context.Context, offset, limit int) ([]*interfaces.SyncTransaction, error) {
	args := m.Called(ctx, offset, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*interfaces.SyncTransaction), args.Error(1)
}

func (m *MockStateManager) Cleanup(ctx context.Context, before time.Time) error {
	args := m.Called(ctx, before)
	return args.Error(0)
}

func (m *MockStateManager) Export(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockStateManager) Import(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockStateManager) Reset(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStateManager) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStateManager) GetStatistics(ctx context.Context) (*interfaces.SyncStatistics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*interfaces.SyncStatistics), args.Error(1)
}

// Tests
func TestNewPulsePointEngine(t *testing.T) {
	mockProvider := new(MockProvider)
	mockWatcher := new(MockWatcher)
	mockStrategy := new(MockStrategy)
	mockStateManager := new(MockStateManager)

	// Setup state manager expectations
	mockStateManager.On("LoadState", mock.Anything).Return(&interfaces.SyncState{}, nil)

	config := &EngineConfig{
		SyncInterval:  time.Minute,
		BatchSize:     10,
		MaxConcurrent: 4,
	}

	engine, err := NewPulsePointEngine(
		mockProvider,
		mockWatcher,
		mockStrategy,
		mockStateManager,
		nil, // DB can be nil for this test
		config,
	)
	require.NoError(t, err)

	assert.NotNil(t, engine)
	assert.Equal(t, mockProvider, engine.provider)
	assert.Equal(t, mockWatcher, engine.watcher)
	assert.Equal(t, mockStrategy, engine.strategy)
	assert.Equal(t, mockStateManager, engine.stateManager)
	assert.Equal(t, config, engine.config)
	assert.False(t, engine.isRunning)
	assert.False(t, engine.isPaused)
}

func TestEngineStartStop(t *testing.T) {
	mockProvider := new(MockProvider)
	mockWatcher := new(MockWatcher)
	mockStrategy := new(MockStrategy)
	mockStateManager := new(MockStateManager)

	// Setup expectations - LoadState is called during initialization
	mockStateManager.On("LoadState", mock.Anything).Return(&interfaces.SyncState{}, nil).Once()

	// Setup other expectations for Start/Stop
	mockWatcher.On("Start", mock.Anything, mock.Anything).Return(nil)
	mockWatcher.On("Stop").Return(nil)
	mockStateManager.On("SaveState", mock.Anything, mock.Anything).Return(nil)
	mockStrategy.On("Name").Return("test-strategy")

	changeChan := make(chan interfaces.ChangeEvent)
	mockWatcher.On("Watch").Return((<-chan interfaces.ChangeEvent)(changeChan))
	errorChan := make(chan error)
	mockWatcher.On("Errors").Return((<-chan error)(errorChan))

	engine, err := NewPulsePointEngine(
		mockProvider,
		mockWatcher,
		mockStrategy,
		mockStateManager,
		nil,
		&EngineConfig{
			SyncInterval: 100 * time.Millisecond,
		},
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Start engine
	err = engine.Start(ctx)
	require.NoError(t, err)
	assert.True(t, engine.isRunning)

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Stop engine
	err = engine.Stop()
	require.NoError(t, err)
	assert.False(t, engine.isRunning)

	// Verify mock expectations
	mockWatcher.AssertExpectations(t)
	mockStateManager.AssertExpectations(t)
}

func TestEnginePauseResume(t *testing.T) {
	mockProvider := new(MockProvider)
	mockWatcher := new(MockWatcher)
	mockStrategy := new(MockStrategy)
	mockStateManager := new(MockStateManager)

	// Setup expectations - LoadState is called during initialization
	mockStateManager.On("LoadState", mock.Anything).Return(&interfaces.SyncState{}, nil).Once()

	// Setup other expectations
	mockWatcher.On("Start", mock.Anything, mock.Anything).Return(nil)
	mockWatcher.On("Stop").Return(nil)
	mockStateManager.On("SaveState", mock.Anything, mock.Anything).Return(nil)
	mockStrategy.On("Name").Return("test-strategy")

	changeChan := make(chan interfaces.ChangeEvent)
	mockWatcher.On("Watch").Return((<-chan interfaces.ChangeEvent)(changeChan))
	errorChan := make(chan error)
	mockWatcher.On("Errors").Return((<-chan error)(errorChan))

	engine, err := NewPulsePointEngine(
		mockProvider,
		mockWatcher,
		mockStrategy,
		mockStateManager,
		nil,
		&EngineConfig{
			SyncInterval: 100 * time.Millisecond,
		},
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Start engine
	err = engine.Start(ctx)
	require.NoError(t, err)

	// Give goroutines time to start and call Watch()/Errors()
	// This prevents race condition in test environments like GitHub Actions
	time.Sleep(50 * time.Millisecond)

	// Pause engine
	err = engine.Pause()
	require.NoError(t, err)
	assert.True(t, engine.isPaused)

	// Resume engine
	err = engine.Resume()
	require.NoError(t, err)
	assert.False(t, engine.isPaused)

	// Stop engine
	err = engine.Stop()
	require.NoError(t, err)

	mockWatcher.AssertExpectations(t)
	mockStateManager.AssertExpectations(t)
}

func TestEngineMetrics(t *testing.T) {
	mockProvider := new(MockProvider)
	mockWatcher := new(MockWatcher)
	mockStrategy := new(MockStrategy)
	mockStateManager := new(MockStateManager)

	// Setup expectation for LoadState called during initialization
	mockStateManager.On("LoadState", mock.Anything).Return(nil, nil).Once()

	engine, err := NewPulsePointEngine(
		mockProvider,
		mockWatcher,
		mockStrategy,
		mockStateManager,
		nil,
		&EngineConfig{},
	)
	require.NoError(t, err)

	// Initial metrics should be zero
	metrics := engine.getMetrics()
	assert.Equal(t, int64(0), metrics.TotalSyncs)
	assert.Equal(t, int64(0), metrics.SuccessfulSyncs)
	assert.Equal(t, int64(0), metrics.FailedSyncs)

	// Simulate successful sync
	result := &interfaces.SyncResult{
		FilesProcessed:   100,
		BytesTransferred: 1024 * 1024,
		Success:          true,
	}
	engine.metrics.recordSuccess(result)

	metrics = engine.getMetrics()
	assert.Equal(t, int64(1), metrics.TotalSyncs)
	assert.Equal(t, int64(1), metrics.SuccessfulSyncs)
	assert.Equal(t, int64(0), metrics.FailedSyncs)
	assert.Equal(t, int64(100), metrics.TotalFiles)
	assert.Equal(t, int64(1024*1024), metrics.TotalBytes)

	// Simulate failed sync
	engine.metrics.recordFailure()

	metrics = engine.getMetrics()
	assert.Equal(t, int64(2), metrics.TotalSyncs)
	assert.Equal(t, int64(1), metrics.SuccessfulSyncs)
	assert.Equal(t, int64(1), metrics.FailedSyncs)
}

func TestEngineStatus(t *testing.T) {
	mockProvider := new(MockProvider)
	mockWatcher := new(MockWatcher)
	mockStrategy := new(MockStrategy)
	mockStateManager := new(MockStateManager)

	// Setup expectation for LoadState called during initialization
	mockStateManager.On("LoadState", mock.Anything).Return(nil, nil).Once()

	engine, err := NewPulsePointEngine(
		mockProvider,
		mockWatcher,
		mockStrategy,
		mockStateManager,
		nil,
		&EngineConfig{},
	)
	require.NoError(t, err)

	// Get initial status
	status, err := engine.GetStatus()
	require.NoError(t, err)
	assert.False(t, status.IsRunning)
	assert.False(t, status.IsPaused)
	assert.Equal(t, "", status.CurrentOperation)
	assert.Equal(t, float64(0), status.Progress)
	assert.NotNil(t, status.State)
	assert.NotNil(t, status.Metrics)
}

func TestEngineConfiguration(t *testing.T) {
	config := &EngineConfig{
		SyncInterval:       5 * time.Minute,
		BatchSize:          20,
		MaxConcurrent:      8,
		RetryAttempts:      5,
		RetryDelay:         2 * time.Second,
		ConflictResolution: "keep-local",
		EnableCaching:      true,
		CacheTTL:           10 * time.Minute,
		MaxMemoryUsage:     1024 * 1024 * 100,  // 100MB
		BandwidthLimit:     1024 * 1024 * 10,   // 10MB/s
		MaxFileSize:        1024 * 1024 * 1024, // 1GB
		IgnorePatterns:     []string{"*.tmp", "*.cache"},
		PreserveTimestamps: true,
		EnableCompression:  true,
	}

	mockProvider := new(MockProvider)
	mockWatcher := new(MockWatcher)
	mockStrategy := new(MockStrategy)
	mockStateManager := new(MockStateManager)

	// Setup expectation for LoadState called during initialization
	mockStateManager.On("LoadState", mock.Anything).Return(nil, nil).Once()

	engine, err := NewPulsePointEngine(
		mockProvider,
		mockWatcher,
		mockStrategy,
		mockStateManager,
		nil,
		config,
	)
	require.NoError(t, err)

	// Verify configuration is properly set
	assert.Equal(t, config.SyncInterval, engine.config.SyncInterval)
	assert.Equal(t, config.BatchSize, engine.config.BatchSize)
	assert.Equal(t, config.MaxConcurrent, engine.config.MaxConcurrent)
	assert.Equal(t, config.RetryAttempts, engine.config.RetryAttempts)
	assert.Equal(t, config.EnableCaching, engine.config.EnableCaching)
	assert.Equal(t, config.MaxFileSize, engine.config.MaxFileSize)
	assert.Equal(t, config.IgnorePatterns, engine.config.IgnorePatterns)
}
