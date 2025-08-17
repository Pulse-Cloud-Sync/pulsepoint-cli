package local

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/pulsepoint/pulsepoint/pkg/logger"
	"go.uber.org/zap"
)

// ChangeTypeUnknown represents an unknown change type
const ChangeTypeUnknown interfaces.ChangeType = "unknown"

// PulsePointWatcher implements the FileWatcher interface using fsnotify
type PulsePointWatcher struct {
	watcher        *fsnotify.Watcher
	paths          map[string]bool // paths being watched
	pathsMu        sync.RWMutex
	ignorePatterns []string
	eventsChan     chan interfaces.ChangeEvent
	errorsChan     chan error
	stopChan       chan struct{}
	debouncePeriod time.Duration
	debounceTimers map[string]*time.Timer
	debounceMu     sync.Mutex
	hashAlgorithm  string // "md5" or "sha256"
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	fileHashes     map[string]string // cache of file hashes
	hashMu         sync.RWMutex
	isRunning      bool
	runningMu      sync.RWMutex
}

// NewPulsePointWatcher creates a new file watcher instance
func NewPulsePointWatcher(debouncePeriod time.Duration, hashAlgorithm string) (interfaces.FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	if debouncePeriod == 0 {
		debouncePeriod = 100 * time.Millisecond // default debounce period
	}

	if hashAlgorithm == "" {
		hashAlgorithm = "sha256" // default to SHA256
	}

	ctx, cancel := context.WithCancel(context.Background())

	pw := &PulsePointWatcher{
		watcher:        w,
		paths:          make(map[string]bool),
		ignorePatterns: []string{},
		eventsChan:     make(chan interfaces.ChangeEvent, 100),
		errorsChan:     make(chan error, 10),
		stopChan:       make(chan struct{}),
		debouncePeriod: debouncePeriod,
		debounceTimers: make(map[string]*time.Timer),
		hashAlgorithm:  hashAlgorithm,
		logger:         logger.Get(),
		ctx:            ctx,
		cancel:         cancel,
		fileHashes:     make(map[string]string),
		isRunning:      false,
	}

	return pw, nil
}

// Start begins watching for file system events
func (pw *PulsePointWatcher) Start(ctx context.Context, paths []string) error {
	pw.runningMu.Lock()
	defer pw.runningMu.Unlock()

	if pw.isRunning {
		return fmt.Errorf("watcher is already running")
	}

	// Update context if provided
	if ctx != nil {
		pw.ctx = ctx
	}

	// Add initial paths
	for _, path := range paths {
		if err := pw.addPathInternal(path); err != nil {
			pw.logger.Warn("Failed to add initial path",
				zap.String("path", path),
				zap.Error(err),
			)
		}
	}

	pw.wg.Add(1)
	go pw.pulsePointMonitor()

	pw.isRunning = true
	pw.logger.Info("PulsePoint file watcher started",
		zap.Duration("debounce_period", pw.debouncePeriod),
		zap.String("hash_algorithm", pw.hashAlgorithm),
		zap.Int("paths_count", len(paths)),
	)

	return nil
}

// Stop stops the file watcher
func (pw *PulsePointWatcher) Stop() error {
	pw.runningMu.Lock()
	defer pw.runningMu.Unlock()

	if !pw.isRunning {
		return nil
	}

	pw.cancel()
	close(pw.stopChan)

	// Cancel all pending debounce timers
	pw.debounceMu.Lock()
	for _, timer := range pw.debounceTimers {
		timer.Stop()
	}
	pw.debounceTimers = make(map[string]*time.Timer)
	pw.debounceMu.Unlock()

	// Wait for monitor goroutine to finish
	pw.wg.Wait()

	// Close the fsnotify watcher
	err := pw.watcher.Close()

	// Close channels
	close(pw.eventsChan)
	close(pw.errorsChan)

	pw.isRunning = false
	pw.logger.Info("PulsePoint file watcher stopped")

	return err
}

// Watch returns the channel for receiving change events
func (pw *PulsePointWatcher) Watch() <-chan interfaces.ChangeEvent {
	return pw.eventsChan
}

// Errors returns the channel for receiving errors
func (pw *PulsePointWatcher) Errors() <-chan error {
	return pw.errorsChan
}

// AddPath adds a path to watch (can be file or directory)
func (pw *PulsePointWatcher) AddPath(path string) error {
	pw.pathsMu.Lock()
	defer pw.pathsMu.Unlock()
	return pw.addPathInternal(path)
}

// addPathInternal adds a path without locking (for internal use)
func (pw *PulsePointWatcher) addPathInternal(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if already watching
	if pw.paths[absPath] {
		return nil
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	// If directory, add recursively
	if info.IsDir() {
		err = pw.pulsePointAddRecursive(absPath)
		if err != nil {
			return err
		}
	} else {
		// Add single file
		err = pw.watcher.Add(absPath)
		if err != nil {
			return fmt.Errorf("failed to add path to watcher: %w", err)
		}
		pw.paths[absPath] = true

		// Calculate initial hash for the file
		if hash, err := pw.pulsePointCalculateHash(absPath); err == nil {
			pw.hashMu.Lock()
			pw.fileHashes[absPath] = hash
			pw.hashMu.Unlock()
		}
	}

	pw.logger.Info("Added path to watcher",
		zap.String("path", absPath),
		zap.Bool("is_directory", info.IsDir()),
	)

	return nil
}

// IsWatching checks if currently watching
func (pw *PulsePointWatcher) IsWatching() bool {
	pw.runningMu.RLock()
	defer pw.runningMu.RUnlock()
	return pw.isRunning
}

// RemovePath removes a path from watching
func (pw *PulsePointWatcher) RemovePath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	pw.pathsMu.Lock()
	defer pw.pathsMu.Unlock()

	// Remove all paths that start with this path (handles directories)
	for watchedPath := range pw.paths {
		if strings.HasPrefix(watchedPath, absPath) {
			err := pw.watcher.Remove(watchedPath)
			if err != nil {
				pw.logger.Warn("Failed to remove path from watcher",
					zap.String("path", watchedPath),
					zap.Error(err),
				)
			}
			delete(pw.paths, watchedPath)

			// Remove from hash cache
			pw.hashMu.Lock()
			delete(pw.fileHashes, watchedPath)
			pw.hashMu.Unlock()
		}
	}

	pw.logger.Info("Removed path from watcher", zap.String("path", absPath))

	return nil
}

// SetIgnorePatterns sets patterns to ignore (gitignore style)
func (pw *PulsePointWatcher) SetIgnorePatterns(patterns []string) error {
	pw.ignorePatterns = patterns
	pw.logger.Info("Updated ignore patterns", zap.Int("count", len(patterns)))
	return nil
}

// GetWatchedPaths returns a list of all watched paths
func (pw *PulsePointWatcher) GetWatchedPaths() []string {
	pw.pathsMu.RLock()
	defer pw.pathsMu.RUnlock()

	paths := make([]string, 0, len(pw.paths))
	for path := range pw.paths {
		paths = append(paths, path)
	}
	return paths
}

// pulsePointMonitor is the main monitoring goroutine
func (pw *PulsePointWatcher) pulsePointMonitor() {
	defer pw.wg.Done()

	for {
		select {
		case <-pw.ctx.Done():
			return
		case <-pw.stopChan:
			return
		case event, ok := <-pw.watcher.Events:
			if !ok {
				return
			}
			pw.pulsePointHandleEvent(event)
		case err, ok := <-pw.watcher.Errors:
			if !ok {
				return
			}
			pw.errorsChan <- err
			pw.logger.Error("File watcher error", zap.Error(err))
		}
	}
}

// pulsePointHandleEvent processes a single fsnotify event
func (pw *PulsePointWatcher) pulsePointHandleEvent(event fsnotify.Event) {
	// Check if should ignore
	if pw.pulsePointShouldIgnore(event.Name) {
		return
	}

	// Debounce the event
	pw.debounceMu.Lock()
	if timer, exists := pw.debounceTimers[event.Name]; exists {
		timer.Stop()
	}

	pw.debounceTimers[event.Name] = time.AfterFunc(pw.debouncePeriod, func() {
		pw.pulsePointProcessEvent(event)

		// Clean up timer
		pw.debounceMu.Lock()
		delete(pw.debounceTimers, event.Name)
		pw.debounceMu.Unlock()
	})
	pw.debounceMu.Unlock()
}

// pulsePointProcessEvent processes a debounced event
func (pw *PulsePointWatcher) pulsePointProcessEvent(event fsnotify.Event) {
	changeType := pw.pulsePointMapEventType(event.Op)
	if changeType == ChangeTypeUnknown {
		return
	}

	// Get file info
	info, err := os.Stat(event.Name)
	if err != nil && !os.IsNotExist(err) {
		pw.logger.Warn("Failed to stat file",
			zap.String("path", event.Name),
			zap.Error(err),
		)
	}

	// Create change event
	changeEvent := interfaces.ChangeEvent{
		Path:      event.Name,
		Type:      changeType,
		Timestamp: time.Now().Unix(),
		Size:      0,
		IsDir:     false,
	}

	if info != nil {
		changeEvent.Size = info.Size()
		changeEvent.IsDir = info.IsDir()

		// Calculate hash for files (not directories)
		if !info.IsDir() && changeType != interfaces.ChangeTypeDelete {
			if hash, err := pw.pulsePointCalculateHash(event.Name); err == nil {
				changeEvent.Hash = hash

				// Check if content actually changed for modify events
				if changeType == interfaces.ChangeTypeModify {
					pw.hashMu.RLock()
					oldHash, exists := pw.fileHashes[event.Name]
					pw.hashMu.RUnlock()

					if exists && oldHash == hash {
						// Content hasn't changed, skip this event
						return
					}
				}

				// Update hash cache
				pw.hashMu.Lock()
				pw.fileHashes[event.Name] = hash
				pw.hashMu.Unlock()
			}
		}
	}

	// If a directory was created, add it to the watcher
	if changeEvent.IsDir && changeType == interfaces.ChangeTypeCreate {
		pw.pathsMu.Lock()
		err := pw.pulsePointAddRecursive(event.Name)
		pw.pathsMu.Unlock()
		if err != nil {
			pw.logger.Warn("Failed to add new directory to watcher",
				zap.String("path", event.Name),
				zap.Error(err),
			)
		}
	}

	// Send the event
	select {
	case pw.eventsChan <- changeEvent:
		pw.logger.Debug("File change detected",
			zap.String("path", event.Name),
			zap.String("type", string(changeType)),
			zap.Int64("size", changeEvent.Size),
		)
	case <-pw.ctx.Done():
		return
	}
}

// pulsePointMapEventType maps fsnotify operations to our change types
func (pw *PulsePointWatcher) pulsePointMapEventType(op fsnotify.Op) interfaces.ChangeType {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return interfaces.ChangeTypeCreate
	case op&fsnotify.Write == fsnotify.Write:
		return interfaces.ChangeTypeModify
	case op&fsnotify.Remove == fsnotify.Remove:
		return interfaces.ChangeTypeDelete
	case op&fsnotify.Rename == fsnotify.Rename:
		return interfaces.ChangeTypeRename
	case op&fsnotify.Chmod == fsnotify.Chmod:
		return interfaces.ChangeTypeChmod
	default:
		return ChangeTypeUnknown
	}
}

// pulsePointShouldIgnore checks if a path should be ignored
func (pw *PulsePointWatcher) pulsePointShouldIgnore(path string) bool {
	// Check against ignore patterns
	for _, pattern := range pw.ignorePatterns {
		// Simple pattern matching (can be enhanced with proper gitignore parsing)
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		// Check if path contains the pattern
		if strings.Contains(path, pattern) {
			return true
		}
	}

	// Always ignore common system files
	base := filepath.Base(path)
	if base == ".DS_Store" || base == "Thumbs.db" || base == ".git" || strings.HasPrefix(base, "~") {
		return true
	}

	return false
}

// pulsePointAddRecursive recursively adds a directory and its contents to the watcher
func (pw *PulsePointWatcher) pulsePointAddRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if should ignore
		if pw.pulsePointShouldIgnore(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Only add directories to the watcher (files are watched through their parent directory)
		if info.IsDir() {
			err = pw.watcher.Add(path)
			if err != nil {
				return fmt.Errorf("failed to add directory %s: %w", path, err)
			}
			pw.paths[path] = true
		} else {
			// Calculate initial hash for files
			if hash, err := pw.pulsePointCalculateHash(path); err == nil {
				pw.hashMu.Lock()
				pw.fileHashes[path] = hash
				pw.hashMu.Unlock()
			}
		}

		return nil
	})
}

// pulsePointCalculateHash calculates the hash of a file
func (pw *PulsePointWatcher) pulsePointCalculateHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var hasher io.Writer
	switch pw.hashAlgorithm {
	case "md5":
		h := md5.New()
		hasher = h
		if _, err := io.Copy(hasher, file); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	case "sha256":
		h := sha256.New()
		hasher = h
		if _, err := io.Copy(hasher, file); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %s", pw.hashAlgorithm)
	}
}
