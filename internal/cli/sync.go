package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/pulsepoint/pulsepoint/internal/database"
	"github.com/pulsepoint/pulsepoint/internal/strategies"
	"github.com/pulsepoint/pulsepoint/internal/sync"
	"github.com/pulsepoint/pulsepoint/internal/watchers/local"
	pplogger "github.com/pulsepoint/pulsepoint/pkg/logger"
	"github.com/spf13/cobra"
)

// syncCmd represents the sync command for manual synchronization
var syncCmd = &cobra.Command{
	Use:   "sync [path]",
	Short: "Manually trigger a sync operation",
	Long: `Perform a manual synchronization of the specified directory
with your cloud storage provider.

Unlike 'pulse' which continuously monitors, 'sync' performs a 
one-time synchronization and then exits.`,
	Args: cobra.ExactArgs(1),
	RunE: runSync,
}

func init() {
	syncCmd.Flags().String("remote", "", "Remote path in cloud storage")
	syncCmd.Flags().Bool("force", false, "Force sync even if no changes detected")
	syncCmd.Flags().Bool("full", false, "Perform full sync instead of incremental")
	syncCmd.Flags().Bool("dry-run", false, "Show what would be synced without actually syncing")
	syncCmd.Flags().String("strategy", "one-way", "Sync strategy: one-way, mirror, or backup")
	syncCmd.Flags().String("conflict", "keep-local", "Conflict resolution: keep-local, keep-remote, keep-both, skip")
	syncCmd.Flags().Int("workers", 4, "Number of concurrent workers")
}

func runSync(cmd *cobra.Command, args []string) error {
	localPath := args[0]
	remotePath, _ := cmd.Flags().GetString("remote")
	force, _ := cmd.Flags().GetBool("force")
	full, _ := cmd.Flags().GetBool("full")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	strategyName, _ := cmd.Flags().GetString("strategy")
	conflictRes, _ := cmd.Flags().GetString("conflict")
	workers, _ := cmd.Flags().GetInt("workers")

	log := pplogger.Get()

	fmt.Printf("🔄 Starting PulsePoint Sync Operation\n")
	fmt.Printf("📁 Local Path: %s\n", localPath)

	if remotePath != "" {
		fmt.Printf("☁️  Remote Path: %s\n", remotePath)
	}

	fmt.Printf("🎯 Strategy: %s\n", strategyName)
	fmt.Printf("⚔️  Conflict Resolution: %s\n", conflictRes)

	if force {
		fmt.Printf("💪 Force sync enabled\n")
	}

	if full {
		fmt.Printf("📊 Full sync mode\n")
	} else {
		fmt.Printf("⚡ Incremental sync mode\n")
	}

	if dryRun {
		fmt.Printf("🔍 DRY RUN MODE - No actual changes will be made\n")
	}

	// Initialize database
	dbOptions := &database.Options{
		Path: getDBPath(),
	}
	db, err := database.NewManager(dbOptions)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	// Open the database connection
	if err := db.Open(); err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create cloud provider
	ctx := context.Background()
	provider, err := sync.CreateDefaultProvider(ctx)
	if err != nil {
		// If no provider configured, show helpful message
		fmt.Println("\n⚠️  No cloud provider configured!")
		fmt.Println("   Run 'pulsepoint auth google' to set up Google Drive")
		return fmt.Errorf("cloud provider not configured: %w", err)
	}

	// Initialize file watcher
	watcher, err := local.NewPulsePointWatcher(100*time.Millisecond, "sha256")
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	// Create sync strategy
	var strategy interfaces.SyncStrategy
	strategyConfig := &interfaces.StrategyConfig{
		ConflictResolution: parseConflictResolution(conflictRes),
		MaxFileSize:        100 * 1024 * 1024, // 100MB limit
	}

	switch strategyName {
	case "one-way":
		strategy = strategies.NewPulsePointOneWayStrategy(provider, log, strategyConfig)
	case "mirror":
		strategy = strategies.NewPulsePointMirrorStrategy(provider, log, strategyConfig)
	case "backup":
		strategy = strategies.NewPulsePointBackupStrategy(provider, log, strategyConfig)
	default:
		return fmt.Errorf("unknown strategy: %s", strategyName)
	}

	// Create state manager
	stateManager := sync.NewPulsePointStateManager(db, log, nil)
	if err := stateManager.Initialize(getDBPath()); err != nil {
		return fmt.Errorf("failed to initialize state manager: %w", err)
	}

	// Create sync engine configuration
	engineConfig := &sync.EngineConfig{
		SyncInterval:       5 * time.Minute,
		BatchSize:          50,
		MaxConcurrent:      workers,
		RetryAttempts:      3,
		RetryDelay:         5 * time.Second,
		ConflictResolution: conflictRes,
		MaxFileSize:        100 * 1024 * 1024,
		IgnorePatterns:     []string{".git", "node_modules", "*.tmp"},
	}

	// Create sync engine
	engine, err := sync.NewPulsePointEngine(
		provider,
		watcher,
		strategy,
		stateManager,
		db,
		engineConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to create sync engine: %w", err)
	}

	// Start the engine
	if err := engine.Start(ctx); err != nil {
		return fmt.Errorf("failed to start sync engine: %w", err)
	}
	defer engine.Stop()

	fmt.Printf("\n🔍 Scanning for changes...\n")

	// Wait a moment for the watcher to collect changes
	time.Sleep(2 * time.Second)

	// Perform sync
	startTime := time.Now()

	if !dryRun {
		result, err := engine.Sync(ctx)
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		duration := time.Since(startTime)

		fmt.Printf("\n")
		if result.Success {
			fmt.Printf("✅ Sync completed successfully!\n")
		} else {
			fmt.Printf("⚠️  Sync completed with errors\n")
		}

		fmt.Printf("📊 Summary:\n")
		fmt.Printf("   📤 Uploaded: %d files\n", result.FilesUploaded)
		fmt.Printf("   📥 Downloaded: %d files\n", result.FilesDownloaded)
		fmt.Printf("   🗑️  Deleted: %d files\n", result.FilesDeleted)
		fmt.Printf("   ⏭️  Skipped: %d files\n", result.FilesSkipped)
		fmt.Printf("   📦 Transferred: %.2f MB\n", float64(result.BytesTransferred)/(1024*1024))

		if len(result.Conflicts) > 0 {
			fmt.Printf("   ⚔️  Conflicts: %d\n", len(result.Conflicts))
		}

		if len(result.Errors) > 0 {
			fmt.Printf("   ❌ Errors: %d\n", len(result.Errors))
			for i, err := range result.Errors {
				if i < 5 { // Show first 5 errors
					fmt.Printf("      - %s: %s\n", err.Path, err.Message)
				}
			}
			if len(result.Errors) > 5 {
				fmt.Printf("      ... and %d more\n", len(result.Errors)-5)
			}
		}

		fmt.Printf("   ⏱️  Time taken: %s\n", duration.Round(time.Millisecond))
	} else {
		// Dry run - just show what would be done
		status, err := engine.GetStatus()
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}

		fmt.Printf("\n📝 DRY RUN Results:\n")
		fmt.Printf("   Would process %d files\n", status.State.TotalFiles)
		fmt.Printf("   Total size: %.2f MB\n", float64(status.State.TotalBytes)/(1024*1024))
	}

	return nil
}

// parseConflictResolution converts string to ResolutionStrategy
func parseConflictResolution(resolution string) interfaces.ResolutionStrategy {
	switch resolution {
	case "keep-local":
		return interfaces.ResolutionKeepLocal
	case "keep-remote":
		return interfaces.ResolutionKeepRemote
	case "keep-both":
		return interfaces.ResolutionKeepBoth
	case "skip":
		return interfaces.ResolutionSkip
	default:
		return interfaces.ResolutionKeepLocal
	}
}

// getDBPath returns the database path
func getDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pulsepoint", "pulsepoint.db")
}
