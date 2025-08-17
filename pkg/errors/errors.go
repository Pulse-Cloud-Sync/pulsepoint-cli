// Package errors defines custom error types for PulsePoint
package errors

import (
	"fmt"
)

// ErrorType represents the category of error
type ErrorType string

const (
	// NetworkError indicates network-related issues
	NetworkError ErrorType = "network"
	// AuthError indicates authentication/authorization issues
	AuthError ErrorType = "auth"
	// FileSystemError indicates file system related issues
	FileSystemError ErrorType = "filesystem"
	// ValidationError indicates input validation issues
	ValidationError ErrorType = "validation"
	// ConfigError indicates configuration issues
	ConfigError ErrorType = "config"
	// SyncError indicates synchronization issues
	SyncError ErrorType = "sync"
	// ProviderError indicates cloud provider specific issues
	ProviderError ErrorType = "provider"
)

// PulseError is the base error type for all PulsePoint errors
type PulseError struct {
	Type       ErrorType
	Message    string
	Err        error
	Retryable  bool
	StatusCode int
	Context    map[string]interface{}
}

// Error implements the error interface
func (e *PulseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *PulseError) Unwrap() error {
	return e.Err
}

// IsRetryable returns whether the error is retryable
func (e *PulseError) IsRetryable() bool {
	return e.Retryable
}

// WithContext adds context to the error
func (e *PulseError) WithContext(key string, value interface{}) *PulseError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// New creates a new PulseError
func New(errType ErrorType, message string, err error) *PulseError {
	return &PulseError{
		Type:      errType,
		Message:   message,
		Err:       err,
		Retryable: false,
	}
}

// NewRetryable creates a new retryable PulseError
func NewRetryable(errType ErrorType, message string, err error) *PulseError {
	return &PulseError{
		Type:      errType,
		Message:   message,
		Err:       err,
		Retryable: true,
	}
}

// IsNetworkError checks if the error is a network error
func IsNetworkError(err error) bool {
	if pe, ok := err.(*PulseError); ok {
		return pe.Type == NetworkError
	}
	return false
}

// IsAuthError checks if the error is an authentication error
func IsAuthError(err error) bool {
	if pe, ok := err.(*PulseError); ok {
		return pe.Type == AuthError
	}
	return false
}

// IsFileSystemError checks if the error is a file system error
func IsFileSystemError(err error) bool {
	if pe, ok := err.(*PulseError); ok {
		return pe.Type == FileSystemError
	}
	return false
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	if pe, ok := err.(*PulseError); ok {
		return pe.Type == ValidationError
	}
	return false
}

// IsConfigError checks if the error is a configuration error
func IsConfigError(err error) bool {
	if pe, ok := err.(*PulseError); ok {
		return pe.Type == ConfigError
	}
	return false
}

// IsSyncError checks if the error is a sync error
func IsSyncError(err error) bool {
	if pe, ok := err.(*PulseError); ok {
		return pe.Type == SyncError
	}
	return false
}

// IsProviderError checks if the error is a provider error
func IsProviderError(err error) bool {
	if pe, ok := err.(*PulseError); ok {
		return pe.Type == ProviderError
	}
	return false
}

// Constructor functions for each error type

// NewNetworkError creates a new network error
func NewNetworkError(message string, err error) *PulseError {
	return NewRetryable(NetworkError, message, err)
}

// NewAuthError creates a new authentication error
func NewAuthError(message string, err error) *PulseError {
	return New(AuthError, message, err)
}

// NewFileSystemError creates a new file system error
func NewFileSystemError(message string, err error) *PulseError {
	return New(FileSystemError, message, err)
}

// NewValidationError creates a new validation error
func NewValidationError(message string, err error) *PulseError {
	return New(ValidationError, message, err)
}

// NewConfigError creates a new configuration error
func NewConfigError(message string, err error) *PulseError {
	return New(ConfigError, message, err)
}

// NewSyncError creates a new sync error
func NewSyncError(message string, err error) *PulseError {
	return New(SyncError, message, err)
}

// NewProviderError creates a new provider error
func NewProviderError(message string, err error) *PulseError {
	return New(ProviderError, message, err)
}

// NewDatabaseError creates a new database error (using FileSystemError type)
func NewDatabaseError(message string, err error) *PulseError {
	return New(FileSystemError, message, err)
}

// IsNotFoundError checks if the error indicates a resource was not found
func IsNotFoundError(err error) bool {
	if pe, ok := err.(*PulseError); ok {
		return pe.StatusCode == 404
	}
	return false
}
