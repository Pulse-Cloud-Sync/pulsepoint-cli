// Package utils provides utility functions for PulsePoint
package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// PulseID generates a unique ID for pulse operations
func PulseID() string {
	return fmt.Sprintf("pulse-%d", time.Now().UnixNano())
}

// PulseSessionID generates a session ID for a pulse monitoring session
func PulseSessionID() string {
	return fmt.Sprintf("session-%s-%d",
		time.Now().Format("20060102-150405"),
		time.Now().UnixNano()%1000)
}

// ParseDuration parses a duration string with support for additional units
func ParseDuration(s string) (time.Duration, error) {
	// Support for d (days) unit
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		_, err := fmt.Sscanf(days, "%d", &d)
		if err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}

	// Default Go duration parsing
	return time.ParseDuration(s)
}

// FormatDuration formats a duration in human-readable format
func FormatDuration(d time.Duration) string {
	days := d / (24 * time.Hour)
	d = d % (24 * time.Hour)
	hours := d / time.Hour
	d = d % time.Hour
	minutes := d / time.Minute
	d = d % time.Minute
	seconds := d / time.Second

	parts := []string{}

	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	return strings.Join(parts, " ")
}

// MatchPattern checks if a path matches a gitignore-style pattern
func MatchPattern(pattern, path string) (bool, error) {
	// Convert gitignore pattern to regex
	regexPattern := strings.ReplaceAll(pattern, ".", "\\.")
	regexPattern = strings.ReplaceAll(regexPattern, "*", ".*")
	regexPattern = strings.ReplaceAll(regexPattern, "?", ".")

	// Handle directory patterns
	if strings.HasSuffix(pattern, "/") {
		regexPattern = "^" + regexPattern + ".*"
	} else if strings.Contains(pattern, "/") {
		regexPattern = "^" + regexPattern + "$"
	} else {
		// Match anywhere in the path
		regexPattern = "(^|/)" + regexPattern + "($|/)"
	}

	return regexp.MatchString(regexPattern, path)
}

// ShouldIgnore checks if a path should be ignored based on patterns
func ShouldIgnore(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, err := MatchPattern(pattern, path); err == nil && matched {
			return true
		}
	}
	return false
}

// TruncateString truncates a string to a maximum length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// PulseStatus represents the status of a pulse operation
type PulseStatus string

const (
	PulseStatusIdle    PulseStatus = "idle"
	PulseStatusActive  PulseStatus = "active"
	PulseStatusSyncing PulseStatus = "syncing"
	PulseStatusPaused  PulseStatus = "paused"
	PulseStatusError   PulseStatus = "error"
	PulseStatusStopped PulseStatus = "stopped"
)

// GetPulseStatusIcon returns an icon for the given status
func GetPulseStatusIcon(status PulseStatus) string {
	switch status {
	case PulseStatusIdle:
		return "⏸️"
	case PulseStatusActive:
		return "💓"
	case PulseStatusSyncing:
		return "🔄"
	case PulseStatusPaused:
		return "⏸️"
	case PulseStatusError:
		return "❌"
	case PulseStatusStopped:
		return "⏹️"
	default:
		return "❓"
	}
}

// GetPulseStatusColor returns an ANSI color code for the status
func GetPulseStatusColor(status PulseStatus) string {
	switch status {
	case PulseStatusActive, PulseStatusSyncing:
		return "\033[32m" // Green
	case PulseStatusIdle, PulseStatusPaused:
		return "\033[33m" // Yellow
	case PulseStatusError:
		return "\033[31m" // Red
	case PulseStatusStopped:
		return "\033[90m" // Gray
	default:
		return "\033[0m" // Reset
	}
}
