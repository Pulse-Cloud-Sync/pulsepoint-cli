package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage PulsePoint configuration",
	Long:  `View and modify PulsePoint configuration settings.`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open configuration file in editor",
	RunE:  runConfigEdit,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configEditCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	fmt.Printf("📋 PulsePoint Configuration\n")
	fmt.Printf("═══════════════════════════════════════\n\n")

	// Get config file path
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		home, _ := os.UserHomeDir()
		configFile = filepath.Join(home, ".pulsepoint", "config.yaml")
	}

	fmt.Printf("📁 Config File: %s\n\n", configFile)

	// Display all settings
	settings := viper.AllSettings()

	yamlData, err := yaml.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	fmt.Println(string(yamlData))

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Set the value
	viper.Set(key, value)

	// Write config to file
	if err := viper.WriteConfig(); err != nil {
		if err := viper.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to write configuration: %w", err)
		}
	}

	fmt.Printf("✅ Configuration updated\n")
	fmt.Printf("   %s = %s\n", key, value)

	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := viper.Get(key)

	if value == nil {
		return fmt.Errorf("configuration key '%s' not found", key)
	}

	fmt.Printf("%v\n", value)
	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		home, _ := os.UserHomeDir()
		configFile = filepath.Join(home, ".pulsepoint", "config.yaml")
	}

	// Try to find an editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Default to common editors
		editors := []string{"nano", "vim", "vi"}
		for _, e := range editors {
			if _, err := os.Stat("/usr/bin/" + e); err == nil {
				editor = e
				break
			}
		}
	}

	if editor == "" {
		return fmt.Errorf("no editor found. Please set EDITOR environment variable")
	}

	fmt.Printf("📝 Opening %s in %s...\n", configFile, editor)

	// TODO: Implement actual editor opening
	fmt.Printf("⚠️  Editor integration not yet implemented\n")
	fmt.Printf("   Please edit the file manually: %s\n", configFile)

	return nil
}
