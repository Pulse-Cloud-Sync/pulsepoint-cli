// Package cli implements the command-line interface for PulsePoint
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	cfgFile     string
	verboseMode bool
	logger      *zap.Logger
	version     string
	buildDate   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pulsepoint",
	Short: "PulsePoint - Keep your files in sync with a steady pulse",
	Long: `PulsePoint is a production-ready CLI tool that monitors local directories 
and automatically synchronizes changes with cloud storage providers.

Starting with Google Drive support, PulsePoint features an extensible 
architecture that allows easy integration with additional cloud providers.`,
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo sets the version information for the CLI
func SetVersionInfo(v, bd string) {
	version = v
	buildDate = bd
	rootCmd.Version = fmt.Sprintf("%s (built %s)", version, buildDate)
}

func init() {
	// Initialize logger (will be replaced in initConfig)
	logger, _ = zap.NewProduction()

	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pulsepoint/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verboseMode, "verbose", "v", false, "verbose output")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Add all subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(pulseCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(logsCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			logger.Error("Failed to get home directory", zap.Error(err))
			os.Exit(1)
		}

		// Search config in home directory with name ".pulsepoint" (without extension).
		configPath := filepath.Join(home, ".pulsepoint")
		viper.AddConfigPath(configPath)
		viper.AddConfigPath("/etc/pulsepoint/")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Set environment variable prefix
	viper.SetEnvPrefix("PULSEPOINT")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if verboseMode {
			logger.Info("Using config file", zap.String("file", viper.ConfigFileUsed()))
		}
	}

	// Configure logger based on settings
	var config zap.Config
	if verboseMode {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
		// Check config for log level
		logLevel := viper.GetString("logging.level")
		switch logLevel {
		case "debug":
			config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		case "info":
			config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		case "warn":
			config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
		case "error":
			config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
		}
	}

	// Create new logger with configuration
	newLogger, err := config.Build()
	if err == nil {
		logger = newLogger
	}
}
