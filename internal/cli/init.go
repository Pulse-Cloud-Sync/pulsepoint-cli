package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize PulsePoint configuration",
	Long: `Initialize PulsePoint configuration in your home directory.
	
This command creates the necessary configuration files and directories
for PulsePoint to operate. It will create:
- ~/.pulsepoint/config.yaml - Main configuration file
- ~/.pulsepoint/logs/ - Directory for log files
- ~/.pulsepoint/db/ - Directory for state database`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().Bool("force", false, "Overwrite existing configuration")
}

func runInit(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	pulsepointDir := filepath.Join(home, ".pulsepoint")

	// Create PulsePoint directory
	if err := os.MkdirAll(pulsepointDir, 0700); err != nil {
		return fmt.Errorf("failed to create PulsePoint directory: %w", err)
	}

	// Create subdirectories
	dirs := []string{"logs", "db", "cache"}
	for _, dir := range dirs {
		dirPath := filepath.Join(pulsepointDir, dir)
		if err := os.MkdirAll(dirPath, 0700); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}

	// Create default configuration
	configPath := filepath.Join(pulsepointDir, "config.yaml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration already exists at %s. Use --force to overwrite", configPath)
	}

	// Default configuration
	defaultConfig := map[string]interface{}{
		"version": "1.0",
		"pulse": map[string]interface{}{
			"interval":          "30s",
			"batch_size":        10,
			"chunk_size":        "5MB",
			"max_retries":       3,
			"conflict_strategy": "keep_both",
		},
		"monitoring": map[string]interface{}{
			"debounce":      "100ms",
			"max_file_size": "100MB",
			"ignore_patterns": []string{
				"*.tmp",
				"*.swp",
				".DS_Store",
				"Thumbs.db",
				".git/*",
				"node_modules/*",
			},
		},
		"performance": map[string]interface{}{
			"max_concurrent_uploads": 3,
			"rate_limit":             10,
			"cache_ttl":              300,
		},
		"logging": map[string]interface{}{
			"level":       "info",
			"file":        filepath.Join(pulsepointDir, "logs", "pulsepoint.log"),
			"max_size":    100,
			"max_backups": 5,
			"max_age":     30,
		},
	}

	// Write configuration file
	configData, err := yaml.Marshal(defaultConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0600); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	fmt.Printf("✅ PulsePoint initialized successfully!\n")
	fmt.Printf("📁 Configuration directory: %s\n", pulsepointDir)
	fmt.Printf("📝 Configuration file: %s\n", configPath)
	fmt.Printf("\n")
	fmt.Printf("Next steps:\n")
	fmt.Printf("1. Run 'pulsepoint auth google' to authenticate with Google Drive\n")
	fmt.Printf("2. Run 'pulsepoint pulse /path/to/folder' to start monitoring\n")

	return nil
}
