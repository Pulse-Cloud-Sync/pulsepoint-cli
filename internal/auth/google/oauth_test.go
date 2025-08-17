package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestNewPulsePointGoogleAuth(t *testing.T) {
	tests := []struct {
		name    string
		config  *PulsePointOAuthConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &PulsePointOAuthConfig{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
			},
			wantErr: false,
		},
		{
			name: "missing client ID",
			config: &PulsePointOAuthConfig{
				ClientSecret: "test-client-secret",
			},
			wantErr: true,
		},
		{
			name: "missing client secret",
			config: &PulsePointOAuthConfig{
				ClientID: "test-client-id",
			},
			wantErr: true,
		},
		{
			name:    "empty config",
			config:  &PulsePointOAuthConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := NewPulsePointGoogleAuth(tt.config, "")
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
			}
		})
	}
}

func TestGenerateStateToken(t *testing.T) {
	auth := &PulsePointGoogleAuth{}

	// Generate multiple tokens
	token1 := auth.generateStateToken()
	token2 := auth.generateStateToken()

	// Verify tokens are not empty
	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)

	// Verify tokens are unique
	assert.NotEqual(t, token1, token2)

	// Verify token length (base64 encoded 32 bytes)
	assert.True(t, len(token1) > 40)
}

func TestTokenPersistence(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "pulsepoint-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tokenFile := filepath.Join(tmpDir, "token.json")

	auth := &PulsePointGoogleAuth{
		tokenFile: tokenFile,
	}

	// Create test token
	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	// Save token
	err = auth.saveToken(testToken)
	assert.NoError(t, err)

	// Verify file was created with correct permissions
	info, err := os.Stat(tokenFile)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Load token
	loadedToken, err := auth.loadToken()
	assert.NoError(t, err)
	assert.Equal(t, testToken.AccessToken, loadedToken.AccessToken)
	assert.Equal(t, testToken.RefreshToken, loadedToken.RefreshToken)
}

func TestIsAuthenticated(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "pulsepoint-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tokenFile := filepath.Join(tmpDir, "token.json")

	auth := &PulsePointGoogleAuth{
		tokenFile: tokenFile,
	}

	// Test when no token exists
	assert.False(t, auth.IsAuthenticated())

	// Save valid token
	validToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	err = auth.saveToken(validToken)
	require.NoError(t, err)

	// Test with valid token
	assert.True(t, auth.IsAuthenticated())

	// Save expired token
	expiredToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	err = auth.saveToken(expiredToken)
	require.NoError(t, err)

	// Test with expired token (should still return false as it's not valid)
	assert.False(t, auth.IsAuthenticated())
}

func TestCallbackServer(t *testing.T) {
	auth := &PulsePointGoogleAuth{}

	expectedState := "test-state-123"
	expectedCode := "test-auth-code"

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start callback server
	server := auth.startCallbackServer(expectedState, codeChan, errChan)
	require.NotNil(t, server)
	defer server.Shutdown(context.Background())

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test successful callback
	req := httptest.NewRequest("GET", "/callback?state="+expectedState+"&code="+expectedCode, nil)
	w := httptest.NewRecorder()

	http.DefaultServeMux.ServeHTTP(w, req)

	// Wait for code or timeout
	select {
	case code := <-codeChan:
		assert.Equal(t, expectedCode, code)
	case err := <-errChan:
		t.Fatalf("Unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for code")
	}
}

func TestGetTokenInfo(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "pulsepoint-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tokenFile := filepath.Join(tmpDir, "token.json")

	auth := &PulsePointGoogleAuth{
		tokenFile: tokenFile,
	}

	// Test when no token exists
	_, err = auth.GetTokenInfo()
	assert.Error(t, err)

	// Save token
	testToken := &oauth2.Token{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
	err = auth.saveToken(testToken)
	require.NoError(t, err)

	// Get token info
	info, err := auth.GetTokenInfo()
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.True(t, info["valid"].(bool))
	assert.True(t, info["has_refresh"].(bool))

	// Compare time with tolerance for minor differences
	expectedExpiry := testToken.Expiry
	actualExpiry := info["expiry"].(time.Time)
	assert.WithinDuration(t, expectedExpiry, actualExpiry, time.Second)
}
