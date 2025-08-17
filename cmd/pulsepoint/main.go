// Package main is the entry point for the PulsePoint CLI application
package main

import (
	"fmt"
	"os"

	"github.com/pulsepoint/pulsepoint/internal/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Version information (set during build)
var (
	Version   = "dev"
	BuildDate = "unknown"
)

func main() {
	// Initialize zap logger
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Set version info for CLI
	cli.SetVersionInfo(Version, BuildDate)

	// Execute the root command
	if err := cli.Execute(); err != nil {
		logger.Error("PulsePoint execution failed", zap.Error(err))
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
