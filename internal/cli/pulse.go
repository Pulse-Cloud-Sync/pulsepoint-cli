package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/watchers"
	pplogger "github.com/pulsepoint/pulsepoint/pkg/logger"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	"github.com/spf13/cobra"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// pulseCmd represents the pulse command (main monitoring command)
var pulseCmd = &cobra.Command{
	Use:   "pulse [path]",
	Short: "Start PulsePoint monitoring and syncing",
	Long: `Start monitoring a local directory and automatically sync changes 
to your configured cloud storage provider.

PulsePoint will continuously monitor the specified directory for changes
and sync them at the configured interval.`,
	Args: cobra.ExactArgs(1),
	RunE: runPulse,
}

func init() {
	pulseCmd.Flags().Duration("interval", 5*time.Second, "Flush interval for processing changes (e.g., 5s, 30s, 1m)")
	pulseCmd.Flags().String("remote", "", "Remote path in cloud storage")
	pulseCmd.Flags().Bool("recursive", true, "Monitor subdirectories recursively")
	pulseCmd.Flags().StringSlice("ignore", []string{}, "Patterns to ignore (gitignore style)")
	pulseCmd.Flags().Bool("dry-run", false, "Show what would be synced without actually syncing")
	pulseCmd.Flags().Bool("daemon", false, "Run as background daemon")
	pulseCmd.Flags().Duration("debounce", 100*time.Millisecond, "Debounce period for file changes")
	pulseCmd.Flags().Int("batch-size", 100, "Number of changes to process in a batch")
	pulseCmd.Flags().String("ignore-file", "", "Path to ignore file (defaults to .pulseignore or .gitignore)")
	pulseCmd.Flags().String("hash", "sha256", "Hash algorithm to use (md5 or sha256)")
}

func runPulse(cmd *cobra.Command, args []string) error {
	localPath := args[0]
	interval, _ := cmd.Flags().GetDuration("interval")
	remotePath, _ := cmd.Flags().GetString("remote")
	recursive, _ := cmd.Flags().GetBool("recursive")
	ignorePatterns, _ := cmd.Flags().GetStringSlice("ignore")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	daemon, _ := cmd.Flags().GetBool("daemon")
	debounce, _ := cmd.Flags().GetDuration("debounce")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	ignoreFile, _ := cmd.Flags().GetString("ignore-file")
	hashAlgorithm, _ := cmd.Flags().GetString("hash")

	// Get absolute path
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Initialize logger
	logConfig := pplogger.DefaultConfig()
	if err := pplogger.Initialize(logConfig); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	zapLogger := pplogger.Get()

	// Open database
	dbPath := filepath.Join(os.Getenv("HOME"), ".pulsepoint", "pulsepoint.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Look for ignore file if not specified
	if ignoreFile == "" {
		// Check for .pulseignore or .gitignore in the directory
		pulseignore := filepath.Join(absPath, ".pulseignore")
		gitignore := filepath.Join(absPath, ".gitignore")

		if _, err := os.Stat(pulseignore); err == nil {
			ignoreFile = pulseignore
		} else if _, err := os.Stat(gitignore); err == nil {
			ignoreFile = gitignore
		}
	}

	// Display startup information
	fmt.Printf("🚀 Starting PulsePoint Monitor\n")
	fmt.Printf("📁 Local Path: %s\n", absPath)
	if remotePath != "" {
		fmt.Printf("☁️  Remote Path: %s\n", remotePath)
	}
	fmt.Printf("⏱️  Flush Interval: %s\n", interval)
	fmt.Printf("⏳ Debounce Period: %s\n", debounce)
	fmt.Printf("📦 Batch Size: %d\n", batchSize)
	fmt.Printf("🔐 Hash Algorithm: %s\n", hashAlgorithm)
	fmt.Printf("🔄 Recursive: %v\n", recursive)

	if ignoreFile != "" {
		fmt.Printf("📝 Using ignore file: %s\n", ignoreFile)
	}

	if len(ignorePatterns) > 0 {
		fmt.Printf("🚫 Ignore Patterns: %v\n", ignorePatterns)
	}

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No actual changes will be made\n")
	}

	if daemon {
		fmt.Printf("👻 Running in daemon mode\n")
	}

	fmt.Printf("\n")

	// Create watcher manager configuration
	managerConfig := watchers.ManagerConfig{
		DebouncePeriod: debounce,
		HashAlgorithm:  hashAlgorithm,
		MaxQueueSize:   10000,
		BatchSize:      batchSize,
		FlushInterval:  interval,
		IgnoreFile:     ignoreFile,
		SyncHandler:    pulsePointCreateSyncHandler(zapLogger, dryRun, remotePath),
	}

	// Create watcher manager
	manager, err := watchers.NewPulsePointWatcherManager(db, managerConfig)
	if err != nil {
		return fmt.Errorf("failed to create watcher manager: %w", err)
	}

	// Add ignore patterns
	if len(ignorePatterns) > 0 {
		if err := manager.AddIgnorePatterns(ignorePatterns); err != nil {
			zapLogger.Warn("Failed to add ignore patterns", zap.Error(err))
		}
	}

	// Start the watcher
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}
	defer manager.Stop()

	// Add the path to watch
	if err := manager.WatchPath(absPath); err != nil {
		return fmt.Errorf("failed to watch path: %w", err)
	}

	fmt.Printf("💓 PulsePoint is monitoring... Press Ctrl+C to stop\n")
	fmt.Printf("\n")

	fmt.Printf("[%s] 👀 Watching for changes...\n", time.Now().Format("15:04:05"))

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a ticker for status updates
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Main monitoring loop
	for {
		select {
		case <-sigChan:
			fmt.Printf("\n[%s] 🛑 Stopping PulsePoint monitor...\n", time.Now().Format("15:04:05"))
			return nil
		case <-ticker.C:
			// Print status update
			stats := manager.GetStats()
			if queueStats, ok := stats["queue_stats"].(map[string]interface{}); ok {
				pending := 0
				processing := 0

				if p, ok := queueStats["pending_count"].(int); ok {
					pending = p
				}
				if p, ok := queueStats["processing_count"].(int); ok {
					processing = p
				}

				if pending > 0 || processing > 0 {
					fmt.Printf("[%s] 📊 Status: %d changes pending, %d processing\n",
						time.Now().Format("15:04:05"), pending, processing)
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// pulsePointCreateSyncHandler creates a sync handler function for processing file changes
func pulsePointCreateSyncHandler(zapLogger *zap.Logger, dryRun bool, remotePath string) func([]*models.ChangeEvent) error {
	return func(events []*models.ChangeEvent) error {
		timestamp := time.Now().Format("15:04:05")

		// Group events by type for summary
		typeCounts := make(map[models.ChangeType]int)
		for _, event := range events {
			typeCounts[event.Type]++
		}

		// Print summary
		parts := []string{}
		if count := typeCounts[models.ChangeTypeCreate]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d created", count))
		}
		if count := typeCounts[models.ChangeTypeModify]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d modified", count))
		}
		if count := typeCounts[models.ChangeTypeDelete]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d deleted", count))
		}
		if count := typeCounts[models.ChangeTypeRename]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d renamed", count))
		}
		if count := typeCounts[models.ChangeTypeMove]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d moved", count))
		}

		if len(parts) > 0 {
			action := "Syncing"
			if dryRun {
				action = "Would sync"
			}
			fmt.Printf("[%s] 🔄 %s batch: %s\n", timestamp, action, strings.Join(parts, ", "))

			// Log individual changes in verbose mode (only first few)
			maxShow := 5
			shown := 0
			for _, event := range events {
				if shown >= maxShow {
					if len(events) > maxShow {
						fmt.Printf("         ... and %d more changes\n", len(events)-maxShow)
					}
					break
				}

				relPath := event.Path
				if home := os.Getenv("HOME"); strings.HasPrefix(relPath, home) {
					relPath = "~" + strings.TrimPrefix(relPath, home)
				}

				emoji := "📄"
				if event.IsDir {
					emoji = "📁"
				}

				var detail string
				switch event.Type {
				case models.ChangeTypeCreate:
					detail = fmt.Sprintf("%s ➕ Created: %s", emoji, relPath)
				case models.ChangeTypeModify:
					detail = fmt.Sprintf("%s ✏️  Modified: %s", emoji, relPath)
				case models.ChangeTypeDelete:
					detail = fmt.Sprintf("%s 🗑️  Deleted: %s", emoji, relPath)
				case models.ChangeTypeRename:
					detail = fmt.Sprintf("%s 🔄 Renamed: %s", emoji, relPath)
				case models.ChangeTypeMove:
					detail = fmt.Sprintf("%s 📦 Moved: %s", emoji, relPath)
				default:
					detail = fmt.Sprintf("%s ❓ Changed: %s", emoji, relPath)
				}

				fmt.Printf("         %s\n", detail)
				shown++
			}
		}

		if !dryRun {
			// TODO: Implement actual sync logic here
			// This will integrate with CloudProvider interface in Phase 5
			zapLogger.Info("Batch processed",
				zap.Int("total_events", len(events)),
				zap.String("remote_path", remotePath),
			)

			// Mark events as processed
			for _, event := range events {
				event.MarkProcessed()
			}
		}

		return nil
	}
}
