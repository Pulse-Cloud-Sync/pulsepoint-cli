package integration

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/pulsepoint/pulsepoint/internal/database"
	"github.com/pulsepoint/pulsepoint/internal/database/repositories"
	"github.com/pulsepoint/pulsepoint/internal/strategies"
	"github.com/pulsepoint/pulsepoint/internal/watchers/local"
	pplogger "github.com/pulsepoint/pulsepoint/pkg/logger"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDatabase creates and opens a test database for testing
func setupTestDatabase(t testing.TB, path string) *database.Manager {
	db, err := database.NewManager(&database.Options{
		Path:     path,
		ReadOnly: false,
	})
	if err != nil {
		t.Fatalf("Failed to create database manager: %v", err)
	}

	err = db.Open()
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	return db
}

// Integration test for the complete sync workflow
func TestSyncWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directories for testing
	testDir, err := ioutil.TempDir("", "pulsepoint-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	sourceDir := filepath.Join(testDir, "source")
	err = os.MkdirAll(sourceDir, 0755)
	require.NoError(t, err)

	// Create test database
	dbPath := filepath.Join(testDir, "test.db")
	db := setupTestDatabase(t, dbPath)
	defer db.Close()

	// Create file watcher
	watcher, err := local.NewPulsePointWatcher(100*time.Millisecond, "sha256")
	require.NoError(t, err)

	// Test file creation and detection
	t.Run("FileCreationDetection", func(t *testing.T) {
		// Start watching
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = watcher.Start(ctx, []string{sourceDir})
		assert.NoError(t, err)

		// Create a test file
		testFile := filepath.Join(sourceDir, "test.txt")
		err = ioutil.WriteFile(testFile, []byte("test content"), 0644)
		assert.NoError(t, err)

		// Wait for change detection
		select {
		case change := <-watcher.Watch():
			assert.Equal(t, testFile, change.Path)
			assert.Contains(t, []string{string(interfaces.ChangeTypeCreate), string(interfaces.ChangeTypeModify)}, string(change.Type))
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for file change detection")
		}

		// Stop watcher
		err = watcher.Stop()
		assert.NoError(t, err)
	})
}

// Test database operations integration
func TestDatabaseIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary database
	testDir, err := ioutil.TempDir("", "pulsepoint-db-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	dbPath := filepath.Join(testDir, "test.db")
	db := setupTestDatabase(t, dbPath)
	defer db.Close()

	t.Run("FileRepository", func(t *testing.T) {
		fileRepo := repositories.NewFileRepository(db)
		assert.NotNil(t, fileRepo)

		// Test create and get
		testFile := &models.File{
			Path: "/test/file.txt",
			Name: "file.txt",
			Size: 1024,
			Hash: "abc123",
		}

		err := fileRepo.Create(testFile)
		assert.NoError(t, err)

		retrieved, err := fileRepo.GetByPath("/test/file.txt")
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, testFile.Path, retrieved.Path)
	})

	t.Run("StateRepository", func(t *testing.T) {
		stateRepo := repositories.NewStateRepository(db)
		assert.NotNil(t, stateRepo)

		// Test sync state
		syncState, err := stateRepo.GetSyncState()
		assert.NoError(t, err)
		assert.NotNil(t, syncState)

		// Update and save
		err = stateRepo.UpdateSyncProgress("test-operation", 50.0)
		assert.NoError(t, err)

		// End operation
		err = stateRepo.EndSyncOperation(true)
		assert.NoError(t, err)
	})

	t.Run("ConflictRepository", func(t *testing.T) {
		conflictRepo := repositories.NewConflictRepository(db)
		assert.NotNil(t, conflictRepo)

		// Test conflict operations
		conflicts, err := conflictRepo.ListUnresolved()
		assert.NoError(t, err)
		assert.Empty(t, conflicts)
	})
}

// Test strategy implementations
func TestStrategyIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create mock provider for testing
	mockProvider := &mockCloudProvider{}

	logger := pplogger.Get()
	config := &interfaces.StrategyConfig{
		ConflictResolution: interfaces.ResolutionKeepLocal,
	}

	t.Run("OneWayStrategy", func(t *testing.T) {
		strategy := strategies.NewPulsePointOneWayStrategy(mockProvider, logger, config)
		assert.NotNil(t, strategy)
		assert.Equal(t, "one-way", strategy.Name())
	})

	t.Run("MirrorStrategy", func(t *testing.T) {
		strategy := strategies.NewPulsePointMirrorStrategy(mockProvider, logger, config)
		assert.NotNil(t, strategy)
		assert.Equal(t, "mirror", strategy.Name())
	})

	t.Run("BackupStrategy", func(t *testing.T) {
		strategy := strategies.NewPulsePointBackupStrategy(mockProvider, logger, config)
		assert.NotNil(t, strategy)
		assert.Equal(t, "backup", strategy.Name())
	})
}

// Test watcher ignore patterns
func TestWatcherIgnorePatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory
	testDir, err := ioutil.TempDir("", "pulsepoint-ignore-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(testDir)

	// Create .pulseignore file
	ignoreContent := `
# Ignore temp files
*.tmp
*.cache

# Ignore directories
node_modules/
.git/

# Ignore specific file
secret.txt
`
	ignoreFile := filepath.Join(testDir, ".pulseignore")
	err = ioutil.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
	require.NoError(t, err)

	// Create watcher
	watcher, err := local.NewPulsePointWatcher(100*time.Millisecond, "sha256")
	require.NoError(t, err)

	// Test that ignored files are not detected
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = watcher.Start(ctx, []string{testDir})
	assert.NoError(t, err)

	// Set ignore patterns
	err = watcher.SetIgnorePatterns([]string{"*.tmp", "*.cache"})
	assert.NoError(t, err)

	// Create files that should be ignored
	tmpFile := filepath.Join(testDir, "test.tmp")
	err = ioutil.WriteFile(tmpFile, []byte("temp"), 0644)
	assert.NoError(t, err)

	// Create file that should be detected
	regularFile := filepath.Join(testDir, "regular.txt")
	err = ioutil.WriteFile(regularFile, []byte("content"), 0644)
	assert.NoError(t, err)

	// Check that only regular file is detected
	select {
	case change := <-watcher.Watch():
		assert.Equal(t, regularFile, change.Path)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected file change not detected")
	}

	// Ensure tmp file was not detected
	select {
	case change := <-watcher.Watch():
		t.Fatalf("Unexpected change detected: %s", change.Path)
	case <-time.After(500 * time.Millisecond):
		// Expected: no change for ignored file
	}

	err = watcher.Stop()
	assert.NoError(t, err)
}

// Mock implementations for testing
type mockCloudProvider struct{}

func (m *mockCloudProvider) Initialize(config interfaces.ProviderConfig) error       { return nil }
func (m *mockCloudProvider) Upload(ctx context.Context, file *interfaces.File) error { return nil }
func (m *mockCloudProvider) Download(ctx context.Context, path string) (*interfaces.File, error) {
	return nil, nil
}
func (m *mockCloudProvider) Delete(ctx context.Context, path string) error { return nil }
func (m *mockCloudProvider) List(ctx context.Context, folder string) ([]*interfaces.File, error) {
	return nil, nil
}
func (m *mockCloudProvider) GetMetadata(ctx context.Context, path string) (*interfaces.Metadata, error) {
	return nil, nil
}
func (m *mockCloudProvider) CreateFolder(ctx context.Context, path string) error { return nil }
func (m *mockCloudProvider) Move(ctx context.Context, source, dest string) error { return nil }
func (m *mockCloudProvider) GetQuota(ctx context.Context) (*interfaces.QuotaInfo, error) {
	return nil, nil
}
func (m *mockCloudProvider) GetProviderName() string { return "mock" }
func (m *mockCloudProvider) IsConnected() bool       { return true }
func (m *mockCloudProvider) Disconnect() error       { return nil }

type mockStateManager struct{}

func (m *mockStateManager) Initialize(dbPath string) error { return nil }
func (m *mockStateManager) SaveState(ctx context.Context, state *interfaces.SyncState) error {
	return nil
}
func (m *mockStateManager) LoadState(ctx context.Context) (*interfaces.SyncState, error) {
	return nil, nil
}
func (m *mockStateManager) UpdateFileState(ctx context.Context, file *interfaces.FileState) error {
	return nil
}
func (m *mockStateManager) GetFileState(ctx context.Context, path string) (*interfaces.FileState, error) {
	return nil, nil
}
func (m *mockStateManager) DeleteFileState(ctx context.Context, path string) error { return nil }
func (m *mockStateManager) ListFileStates(ctx context.Context) ([]*interfaces.FileState, error) {
	return nil, nil
}
func (m *mockStateManager) SaveTransaction(ctx context.Context, transaction *interfaces.SyncTransaction) error {
	return nil
}
func (m *mockStateManager) GetTransaction(ctx context.Context, id string) (*interfaces.SyncTransaction, error) {
	return nil, nil
}
func (m *mockStateManager) ListTransactions(ctx context.Context, offset, limit int) ([]*interfaces.SyncTransaction, error) {
	return nil, nil
}
func (m *mockStateManager) Cleanup(ctx context.Context, before time.Time) error { return nil }
func (m *mockStateManager) Export(ctx context.Context, path string) error       { return nil }
func (m *mockStateManager) Import(ctx context.Context, path string) error       { return nil }
func (m *mockStateManager) Reset(ctx context.Context) error                     { return nil }
func (m *mockStateManager) Close() error                                        { return nil }
func (m *mockStateManager) GetStatistics(ctx context.Context) (*interfaces.SyncStatistics, error) {
	return nil, nil
}

// Benchmark tests
func BenchmarkFileWatcher(b *testing.B) {
	testDir, err := ioutil.TempDir("", "pulsepoint-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(testDir)

	watcher, err := local.NewPulsePointWatcher(100*time.Millisecond, "sha256")
	require.NoError(b, err)

	ctx := context.Background()
	err = watcher.Start(ctx, []string{testDir})
	require.NoError(b, err)
	defer watcher.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := watcher.AddPath(testDir)
		if err != nil {
			b.Fatal(err)
		}
		err = watcher.RemovePath(testDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDatabaseOperations(b *testing.B) {
	testDir, err := ioutil.TempDir("", "pulsepoint-db-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(testDir)

	dbPath := filepath.Join(testDir, "bench.db")
	db := setupTestDatabase(b, dbPath)
	defer db.Close()

	fileRepo := repositories.NewFileRepository(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		file := &models.File{
			Path: filepath.Join("/test", string(rune(i))),
			Name: string(rune(i)),
			Size: int64(i * 1024),
			Hash: string(rune(i)),
		}
		err := fileRepo.Create(file)
		if err != nil {
			b.Fatal(err)
		}
	}
}
