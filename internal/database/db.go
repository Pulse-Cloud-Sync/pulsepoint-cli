// Package database provides database management for PulsePoint
package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pulsepoint/pulsepoint/pkg/logger"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// Database buckets
const (
	// BucketFiles stores file metadata
	BucketFiles = "files"

	// BucketState stores sync state
	BucketState = "state"

	// BucketFileState stores individual file states
	BucketFileState = "file_state"

	// BucketHistory stores sync history
	BucketHistory = "history"

	// BucketQueue stores upload/download queue
	BucketQueue = "queue"

	// BucketConfig stores configuration cache
	BucketConfig = "config"

	// BucketConflicts stores conflict information
	BucketConflicts = "conflicts"

	// BucketTransactions stores sync transactions
	BucketTransactions = "transactions"

	// BucketEvents stores change events
	BucketEvents = "events"

	// BucketMetadata stores general metadata
	BucketMetadata = "metadata"
)

// DB is an alias for Manager for backward compatibility
type DB = Manager

// Manager manages the BoltDB database connection
type Manager struct {
	DB      *bolt.DB // Exported for direct access
	path    string
	logger  *zap.Logger
	mu      sync.RWMutex
	isOpen  bool
	options *Options
}

// Options represents database options
type Options struct {
	Path            string        `json:"path"`
	FileMode        uint32        `json:"file_mode"`
	Timeout         time.Duration `json:"timeout"`
	NoGrowSync      bool          `json:"no_grow_sync"`
	NoFreelistSync  bool          `json:"no_freelist_sync"`
	ReadOnly        bool          `json:"read_only"`
	MmapFlags       int           `json:"mmap_flags"`
	InitialMmapSize int           `json:"initial_mmap_size"`
	PageSize        int           `json:"page_size"`
	NoSync          bool          `json:"no_sync"`
}

// DefaultOptions returns default database options
func DefaultOptions() *Options {
	return &Options{
		Path:           filepath.Join("~/.pulsepoint", "pulsepoint.db"),
		FileMode:       0600,
		Timeout:        1 * time.Second,
		NoGrowSync:     false,
		NoFreelistSync: false,
		ReadOnly:       false,
		PageSize:       4096,
		NoSync:         false,
	}
}

// NewManager creates a new database manager
func NewManager(options *Options) (*Manager, error) {
	if options == nil {
		options = DefaultOptions()
	}

	log := logger.Get()

	return &Manager{
		path:    options.Path,
		logger:  log,
		options: options,
	}, nil
}

// Open opens the database connection
func (m *Manager) Open() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isOpen {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(m.path)
	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open BoltDB
	boltOptions := &bolt.Options{
		Timeout:         m.options.Timeout,
		NoGrowSync:      m.options.NoGrowSync,
		NoFreelistSync:  m.options.NoFreelistSync,
		ReadOnly:        m.options.ReadOnly,
		MmapFlags:       m.options.MmapFlags,
		InitialMmapSize: m.options.InitialMmapSize,
		PageSize:        m.options.PageSize,
		NoSync:          m.options.NoSync,
	}

	db, err := bolt.Open(m.path, os.FileMode(m.options.FileMode), boltOptions)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	m.DB = db
	m.isOpen = true

	// Initialize buckets
	if err := m.initBuckets(); err != nil {
		m.DB.Close()
		m.isOpen = false
		return fmt.Errorf("failed to initialize buckets: %w", err)
	}

	m.logger.Info("Database opened successfully", zap.String("path", m.path))
	return nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isOpen || m.DB == nil {
		return nil
	}

	if err := m.DB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	m.isOpen = false
	m.logger.Info("Database closed successfully")
	return nil
}

// initBuckets initializes all required buckets
func (m *Manager) initBuckets() error {
	buckets := []string{
		BucketFiles,
		BucketState,
		BucketHistory,
		BucketQueue,
		BucketConfig,
		BucketConflicts,
		BucketTransactions,
		BucketEvents,
		BucketMetadata,
	}

	return m.DB.Update(func(tx *bolt.Tx) error {
		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
}

// IsOpen checks if the database is open
func (m *Manager) IsOpen() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isOpen
}

// GetDB returns the underlying BoltDB instance
func (m *Manager) GetDB() *bolt.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.DB
}

// Transaction executes a function within a database transaction
func (m *Manager) Transaction(writable bool, fn func(*bolt.Tx) error) error {
	if !m.IsOpen() {
		return fmt.Errorf("database is not open")
	}

	if writable {
		return m.DB.Update(fn)
	}
	return m.DB.View(fn)
}

// Put stores a key-value pair in a bucket
func (m *Manager) Put(bucket, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return m.Transaction(true, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}
		return b.Put([]byte(key), data)
	})
}

// Get retrieves a value from a bucket
func (m *Manager) Get(bucket, key string, value interface{}) error {
	return m.Transaction(false, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}

		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("key %s not found in bucket %s", key, bucket)
		}

		return json.Unmarshal(data, value)
	})
}

// Delete removes a key from a bucket
func (m *Manager) Delete(bucket, key string) error {
	return m.Transaction(true, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}
		return b.Delete([]byte(key))
	})
}

// List lists all keys in a bucket
func (m *Manager) List(bucket string) ([]string, error) {
	var keys []string

	err := m.Transaction(false, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}

		return b.ForEach(func(k, v []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	})

	return keys, err
}

// ListWithPrefix lists all keys with a specific prefix
func (m *Manager) ListWithPrefix(bucket, prefix string) ([]string, error) {
	var keys []string
	prefixBytes := []byte(prefix)

	err := m.Transaction(false, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}

		c := b.Cursor()
		for k, _ := c.Seek(prefixBytes); k != nil && bytes.HasPrefix(k, prefixBytes); k, _ = c.Next() {
			keys = append(keys, string(k))
		}
		return nil
	})

	return keys, err
}

// Count returns the number of items in a bucket
func (m *Manager) Count(bucket string) (int, error) {
	count := 0

	err := m.Transaction(false, func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}

		stats := b.Stats()
		count = stats.KeyN
		return nil
	})

	return count, err
}

// Clear removes all items from a bucket
func (m *Manager) Clear(bucket string) error {
	return m.Transaction(true, func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(bucket)); err != nil {
			return err
		}
		_, err := tx.CreateBucket([]byte(bucket))
		return err
	})
}

// Backup creates a backup of the database
func (m *Manager) Backup(path string) error {
	if !m.IsOpen() {
		return fmt.Errorf("database is not open")
	}

	// Ensure backup directory exists
	dir := filepath.Dir(path)
	if err := ensureDir(dir); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	return m.DB.View(func(tx *bolt.Tx) error {
		return tx.CopyFile(path, 0600)
	})
}

// Restore restores the database from a backup
func (m *Manager) Restore(backupPath string) error {
	// Close current database if open
	if m.IsOpen() {
		if err := m.Close(); err != nil {
			return err
		}
	}

	// Copy backup file to database path
	if err := copyFile(backupPath, m.path); err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	// Reopen database
	return m.Open()
}

// Stats returns database statistics
func (m *Manager) Stats() (*bolt.Stats, error) {
	if !m.IsOpen() {
		return nil, fmt.Errorf("database is not open")
	}

	stats := m.DB.Stats()
	return &stats, nil
}

// Compact compacts the database to reclaim space
func (m *Manager) Compact() error {
	tempPath := m.path + ".compact"

	// Create compacted copy
	if err := m.Backup(tempPath); err != nil {
		return fmt.Errorf("failed to create compact copy: %w", err)
	}

	// Close current database
	if err := m.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Replace with compacted version
	if err := os.Rename(tempPath, m.path); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace with compacted database: %w", err)
	}

	// Reopen database
	return m.Open()
}

// ensureDir ensures a directory exists
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// copyFile copies a file from source to destination
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
