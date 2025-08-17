// Package logger provides a centralized logging configuration for PulsePoint
package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	// Global logger instance
	pulseLogger *zap.Logger
	// Sugar logger for convenient logging
	sugar *zap.SugaredLogger
)

// LogConfig holds the logging configuration
type LogConfig struct {
	Level       string
	OutputPath  string
	MaxSize     int // megabytes
	MaxBackups  int
	MaxAge      int // days
	Compress    bool
	Development bool
	EnableJSON  bool
}

// DefaultConfig returns the default logging configuration
func DefaultConfig() *LogConfig {
	home, _ := os.UserHomeDir()
	return &LogConfig{
		Level:       "info",
		OutputPath:  filepath.Join(home, ".pulsepoint", "logs", "pulsepoint.log"),
		MaxSize:     100,
		MaxBackups:  5,
		MaxAge:      30,
		Compress:    true,
		Development: false,
		EnableJSON:  false,
	}
}

// Initialize sets up the global logger with the given configuration
func Initialize(cfg *LogConfig) error {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Configure encoder based on development mode
	var encoder zapcore.Encoder
	if cfg.Development && !cfg.EnableJSON {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else if cfg.EnableJSON {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(cfg.OutputPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// Configure file output with rotation
	fileWriter := &lumberjack.Logger{
		Filename:   cfg.OutputPath,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	// Create writers
	var writers []zapcore.WriteSyncer

	// Always log to file
	writers = append(writers, zapcore.AddSync(fileWriter))

	// In development mode, also log to console
	if cfg.Development {
		writers = append(writers, zapcore.AddSync(os.Stdout))
	}

	// Create core
	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(writers...),
		zap.NewAtomicLevelAt(level),
	)

	// Build logger
	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	if cfg.Development {
		opts = append(opts, zap.Development())
	}

	pulseLogger = zap.New(core, opts...)
	sugar = pulseLogger.Sugar()

	// Replace global logger
	zap.ReplaceGlobals(pulseLogger)

	return nil
}

// Get returns the global logger instance
func Get() *zap.Logger {
	if pulseLogger == nil {
		// Initialize with default config if not already initialized
		Initialize(DefaultConfig())
	}
	return pulseLogger
}

// GetSugar returns the sugared logger for convenient logging
func GetSugar() *zap.SugaredLogger {
	if sugar == nil {
		Get() // Ensure logger is initialized
	}
	return sugar
}

// Sync flushes any buffered log entries
func Sync() error {
	if pulseLogger != nil {
		return pulseLogger.Sync()
	}
	return nil
}

// Debug logs a debug message
func Debug(msg string, fields ...zap.Field) {
	Get().Debug(msg, fields...)
}

// Info logs an info message
func Info(msg string, fields ...zap.Field) {
	Get().Info(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zap.Field) {
	Get().Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zap.Field) {
	Get().Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zap.Field) {
	Get().Fatal(msg, fields...)
}

// WithCorrelationID creates a logger with a correlation ID field
func WithCorrelationID(correlationID string) *zap.Logger {
	return Get().With(zap.String("correlation_id", correlationID))
}
