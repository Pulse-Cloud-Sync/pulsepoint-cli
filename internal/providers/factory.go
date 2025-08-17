package providers

import (
	"context"
	"fmt"
	"os"

	ppauth "github.com/pulsepoint/pulsepoint/internal/auth/google"
	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	gdrive "github.com/pulsepoint/pulsepoint/internal/providers/google"
	"github.com/pulsepoint/pulsepoint/internal/providers/mock"
	"github.com/pulsepoint/pulsepoint/pkg/errors"
	"github.com/spf13/viper"
)

// ProviderType represents the type of cloud provider
type ProviderType string

const (
	// GoogleDrive provider type
	GoogleDrive ProviderType = "google"
	// Dropbox provider type (future)
	Dropbox ProviderType = "dropbox"
	// OneDrive provider type (future)
	OneDrive ProviderType = "onedrive"
	// S3 provider type (future)
	S3 ProviderType = "s3"
	// Mock provider type (for testing)
	Mock ProviderType = "mock"
)

// PulsePointProviderFactory creates cloud provider instances
type PulsePointProviderFactory struct {
	ctx context.Context
}

// NewPulsePointProviderFactory creates a new provider factory
func NewPulsePointProviderFactory(ctx context.Context) *PulsePointProviderFactory {
	return &PulsePointProviderFactory{
		ctx: ctx,
	}
}

// CreateProvider creates a provider instance based on type
func (f *PulsePointProviderFactory) CreateProvider(providerType ProviderType) (interfaces.CloudProvider, error) {
	switch providerType {
	case GoogleDrive:
		return f.createGoogleDriveProvider()
	case Mock:
		provider := mock.NewMockDriveProvider()
		config := interfaces.ProviderConfig{
			Type: "mock",
			Settings: map[string]interface{}{
				"enabled": true,
			},
		}
		provider.Initialize(config)
		return provider, nil
	case Dropbox:
		return nil, errors.NewProviderError("Dropbox provider not yet implemented", nil)
	case OneDrive:
		return nil, errors.NewProviderError("OneDrive provider not yet implemented", nil)
	case S3:
		return nil, errors.NewProviderError("S3 provider not yet implemented", nil)
	default:
		return nil, errors.NewProviderError(fmt.Sprintf("unknown provider type: %s", providerType), nil)
	}
}

// createGoogleDriveProvider creates a Google Drive provider instance
func (f *PulsePointProviderFactory) createGoogleDriveProvider() (interfaces.CloudProvider, error) {
	// Check if Google Drive is configured
	if !viper.GetBool("providers.google.configured") {
		return nil, errors.NewConfigError("Google Drive is not configured. Run 'pulsepoint auth google' first", nil)
	}

	// Get credentials
	credentialsPath := viper.GetString("providers.google.credentials_file")
	if credentialsPath == "" {
		credentialsPath = ppauth.GetDefaultCredentialsPath()
	}

	tokenFile := viper.GetString("providers.google.token_file")
	if tokenFile == "" {
		tokenFile = ppauth.GetDefaultTokenPath()
	}

	// Load credentials
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if (clientID == "" || clientSecret == "") && credentialsPath != "" {
		creds, err := ppauth.LoadCredentials(credentialsPath)
		if err != nil {
			// Try to provide helpful error message
			if os.IsNotExist(err) {
				return nil, errors.NewConfigError("Google credentials file not found. Run 'pulsepoint auth google' to set up authentication", err)
			}
			return nil, errors.NewConfigError("failed to load Google credentials", err)
		}
		clientID = creds.ClientID
		clientSecret = creds.ClientSecret
	}

	if clientID == "" || clientSecret == "" {
		return nil, errors.NewConfigError("Google client ID and secret are required", nil)
	}

	// Create provider config
	config := &gdrive.Config{
		CredentialsFile:          credentialsPath,
		TokenFile:                tokenFile,
		RootFolderID:             viper.GetString("providers.google.root_folder_id"),
		Scopes:                   []string{"https://www.googleapis.com/auth/drive"},
		SimpleUploadThreshold:    viper.GetInt64("providers.google.simple_upload_threshold"),
		ResumableUploadThreshold: viper.GetInt64("providers.google.resumable_upload_threshold"),
		ChunkSize:                viper.GetInt64("providers.google.chunk_size"),
		MaxRetries:               viper.GetInt("providers.google.max_retries"),
		RateLimit:                viper.GetInt("providers.google.rate_limit"),
	}

	// Create provider
	provider, err := gdrive.NewPulsePointGoogleDriveProvider(config)
	if err != nil {
		return nil, errors.NewProviderError("failed to create Google Drive provider", err)
	}

	return provider, nil
}

// GetConfiguredProviders returns a list of configured providers
func (f *PulsePointProviderFactory) GetConfiguredProviders() []ProviderType {
	var providers []ProviderType

	if viper.GetBool("providers.google.configured") {
		providers = append(providers, GoogleDrive)
	}
	// Future: check for other providers

	return providers
}

// IsProviderConfigured checks if a provider is configured
func (f *PulsePointProviderFactory) IsProviderConfigured(providerType ProviderType) bool {
	switch providerType {
	case GoogleDrive:
		return viper.GetBool("providers.google.configured")
	default:
		return false
	}
}
