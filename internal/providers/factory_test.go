package providers

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewPulsePointProviderFactory(t *testing.T) {
	ctx := context.Background()
	factory := NewPulsePointProviderFactory(ctx)

	assert.NotNil(t, factory)
	assert.Equal(t, ctx, factory.ctx)
}

func TestProviderTypeConstants(t *testing.T) {
	// Verify provider type constants are properly defined
	assert.Equal(t, ProviderType("google"), GoogleDrive)
	assert.Equal(t, ProviderType("dropbox"), Dropbox)
	assert.Equal(t, ProviderType("onedrive"), OneDrive)
	assert.Equal(t, ProviderType("s3"), S3)
}

func TestCreateProvider_UnsupportedProvider(t *testing.T) {
	factory := NewPulsePointProviderFactory(context.Background())

	tests := []struct {
		name         string
		providerType ProviderType
		wantErr      bool
	}{
		{
			name:         "dropbox not implemented",
			providerType: Dropbox,
			wantErr:      true,
		},
		{
			name:         "onedrive not implemented",
			providerType: OneDrive,
			wantErr:      true,
		},
		{
			name:         "s3 not implemented",
			providerType: S3,
			wantErr:      true,
		},
		{
			name:         "unknown provider",
			providerType: ProviderType("unknown"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := factory.CreateProvider(tt.providerType)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
			}
		})
	}
}

func TestCreateGoogleDriveProvider_NotConfigured(t *testing.T) {
	// Reset viper for clean state
	viper.Reset()

	// Ensure Google Drive is not configured
	viper.Set("providers.google.configured", false)

	factory := NewPulsePointProviderFactory(context.Background())
	provider, err := factory.CreateProvider(GoogleDrive)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "not configured")
}

func TestCreateGoogleDriveProvider_MissingCredentials(t *testing.T) {
	// Reset viper and environment for clean state
	viper.Reset()
	os.Unsetenv("GOOGLE_CLIENT_ID")
	os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Mark as configured but without credentials
	viper.Set("providers.google.configured", true)
	viper.Set("providers.google.credentials_file", "/non/existent/file.json")

	factory := NewPulsePointProviderFactory(context.Background())
	provider, err := factory.CreateProvider(GoogleDrive)

	assert.Error(t, err)
	assert.Nil(t, provider)
}

func TestGetConfiguredProviders(t *testing.T) {
	factory := NewPulsePointProviderFactory(context.Background())

	tests := []struct {
		name     string
		setup    func()
		expected []ProviderType
	}{
		{
			name: "no providers configured",
			setup: func() {
				viper.Reset()
			},
			expected: []ProviderType{},
		},
		{
			name: "google drive configured",
			setup: func() {
				viper.Reset()
				viper.Set("providers.google.configured", true)
			},
			expected: []ProviderType{GoogleDrive},
		},
		{
			name: "google drive not configured",
			setup: func() {
				viper.Reset()
				viper.Set("providers.google.configured", false)
			},
			expected: []ProviderType{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			configured := factory.GetConfiguredProviders()

			if len(tt.expected) == 0 {
				assert.Empty(t, configured)
			} else {
				assert.Equal(t, tt.expected, configured)
			}
		})
	}
}

func TestIsProviderConfigured(t *testing.T) {
	factory := NewPulsePointProviderFactory(context.Background())

	tests := []struct {
		name         string
		setup        func()
		providerType ProviderType
		expected     bool
	}{
		{
			name: "google drive configured",
			setup: func() {
				viper.Reset()
				viper.Set("providers.google.configured", true)
			},
			providerType: GoogleDrive,
			expected:     true,
		},
		{
			name: "google drive not configured",
			setup: func() {
				viper.Reset()
				viper.Set("providers.google.configured", false)
			},
			providerType: GoogleDrive,
			expected:     false,
		},
		{
			name: "unknown provider",
			setup: func() {
				viper.Reset()
			},
			providerType: ProviderType("unknown"),
			expected:     false,
		},
		{
			name: "dropbox always false",
			setup: func() {
				viper.Reset()
			},
			providerType: Dropbox,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			result := factory.IsProviderConfigured(tt.providerType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProviderConfiguration(t *testing.T) {
	// Test configuration loading for Google Drive
	viper.Reset()

	// Set test configuration
	viper.Set("providers.google.configured", true)
	viper.Set("providers.google.root_folder_id", "test-folder-id")
	viper.Set("providers.google.simple_upload_threshold", 1024*1024)       // 1MB
	viper.Set("providers.google.resumable_upload_threshold", 10*1024*1024) // 10MB
	viper.Set("providers.google.chunk_size", 512*1024)                     // 512KB
	viper.Set("providers.google.max_retries", 5)

	// Verify configuration can be read
	assert.True(t, viper.GetBool("providers.google.configured"))
	assert.Equal(t, "test-folder-id", viper.GetString("providers.google.root_folder_id"))
	assert.Equal(t, int64(1024*1024), viper.GetInt64("providers.google.simple_upload_threshold"))
	assert.Equal(t, int64(10*1024*1024), viper.GetInt64("providers.google.resumable_upload_threshold"))
	assert.Equal(t, int64(512*1024), viper.GetInt64("providers.google.chunk_size"))
	assert.Equal(t, 5, viper.GetInt("providers.google.max_retries"))
}
