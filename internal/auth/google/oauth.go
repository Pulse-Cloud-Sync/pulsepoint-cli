package google

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pulsepoint/pulsepoint/pkg/errors"
	"github.com/pulsepoint/pulsepoint/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// PulsePointOAuthConfig holds OAuth2 configuration
type PulsePointOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
}

// PulsePointGoogleAuth handles Google OAuth2 authentication
type PulsePointGoogleAuth struct {
	config    *oauth2.Config
	tokenFile string
	logger    *zap.Logger
}

// NewPulsePointGoogleAuth creates a new Google authentication handler
func NewPulsePointGoogleAuth(cfg *PulsePointOAuthConfig, tokenFile string) (*PulsePointGoogleAuth, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.NewAuthError("missing client ID or client secret", nil)
	}

	// Default scopes if not provided
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{
			drive.DriveScope,         // Full access to Drive
			drive.DriveFileScope,     // Per-file access
			drive.DriveMetadataScope, // Metadata access
		}
	}

	// Default redirect URI for local callback
	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/callback"
	}

	config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  redirectURI,
		Scopes:       scopes,
		Endpoint:     google.Endpoint,
	}

	return &PulsePointGoogleAuth{
		config:    config,
		tokenFile: tokenFile,
		logger:    logger.Get(),
	}, nil
}

// Authenticate performs the OAuth2 flow and returns an authenticated client
func (a *PulsePointGoogleAuth) Authenticate(ctx context.Context) (*http.Client, error) {
	// Try to load existing token
	token, err := a.loadToken()
	if err == nil && token.Valid() {
		a.logger.Info("Using existing valid token")
		return a.config.Client(ctx, token), nil
	}

	// If token exists but expired, try to refresh
	if token != nil && !token.Valid() && token.RefreshToken != "" {
		a.logger.Info("Refreshing expired token")
		tokenSource := a.config.TokenSource(ctx, token)
		newToken, err := tokenSource.Token()
		if err == nil {
			if err := a.saveToken(newToken); err != nil {
				a.logger.Warn("Failed to save refreshed token", zap.Error(err))
			}
			return a.config.Client(ctx, newToken), nil
		}
		a.logger.Warn("Failed to refresh token, starting new auth flow", zap.Error(err))
	}

	// Perform new authentication
	a.logger.Info("Starting new OAuth2 authentication flow")
	token, err = a.performOAuth2Flow(ctx)
	if err != nil {
		return nil, errors.NewAuthError("OAuth2 flow failed", err)
	}

	// Save token for future use
	if err := a.saveToken(token); err != nil {
		a.logger.Warn("Failed to save token", zap.Error(err))
	}

	return a.config.Client(ctx, token), nil
}

// performOAuth2Flow executes the OAuth2 authorization flow
func (a *PulsePointGoogleAuth) performOAuth2Flow(ctx context.Context) (*oauth2.Token, error) {
	// Generate state token for security
	state := a.generateStateToken()

	// Create authorization URL
	authURL := a.config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	// Start local callback server
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)
	server := a.startCallbackServer(state, codeChan, errChan)
	defer server.Shutdown(ctx)

	// Prompt user to authorize
	fmt.Printf("\nPlease visit this URL to authorize PulsePoint:\n%s\n\n", authURL)
	fmt.Println("Waiting for authorization...")

	// Wait for callback
	select {
	case code := <-codeChan:
		// Exchange code for token
		token, err := a.config.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("failed to exchange code for token: %w", err)
		}
		fmt.Println("✓ Authorization successful!")
		return token, nil
	case err := <-errChan:
		return nil, fmt.Errorf("callback server error: %w", err)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authorization timeout")
	}
}

// startCallbackServer starts a local HTTP server to receive the OAuth callback
func (a *PulsePointGoogleAuth) startCallbackServer(expectedState string, codeChan chan<- string, errChan chan<- error) *http.Server {
	listener, err := net.Listen("tcp", "localhost:8080")
	if err != nil {
		// Try alternative ports if 8080 is busy
		listener, err = net.Listen("tcp", "localhost:0")
		if err != nil {
			errChan <- fmt.Errorf("failed to start callback server: %w", err)
			return nil
		}
	}

	server := &http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Verify state parameter
		if r.URL.Query().Get("state") != expectedState {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			errChan <- fmt.Errorf("invalid state parameter")
			return
		}

		// Check for errors
		if errCode := r.URL.Query().Get("error"); errCode != "" {
			http.Error(w, fmt.Sprintf("Authorization failed: %s", errCode), http.StatusBadRequest)
			errChan <- fmt.Errorf("authorization failed: %s", errCode)
			return
		}

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No authorization code received", http.StatusBadRequest)
			errChan <- fmt.Errorf("no authorization code received")
			return
		}

		// Send success response
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<html>
			<head><title>PulsePoint Authorization</title></head>
			<body>
				<h1>✓ Authorization Successful!</h1>
				<p>You can now close this window and return to PulsePoint.</p>
			</body>
			</html>
		`)

		// Send code through channel
		codeChan <- code
	})

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			a.logger.Error("Callback server error", zap.Error(err))
		}
	}()

	return server
}

// generateStateToken generates a random state token for OAuth2 security
func (a *PulsePointGoogleAuth) generateStateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// GetDriveService creates a Google Drive service client
func (a *PulsePointGoogleAuth) GetDriveService(ctx context.Context) (*drive.Service, error) {
	client, err := a.Authenticate(ctx)
	if err != nil {
		return nil, err
	}

	service, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, errors.NewAuthError("failed to create Drive service", err)
	}

	return service, nil
}

// RevokeToken revokes the stored token
func (a *PulsePointGoogleAuth) RevokeToken() error {
	token, err := a.loadToken()
	if err != nil {
		return err
	}

	// Revoke token via Google API
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		fmt.Sprintf("https://oauth2.googleapis.com/revoke?token=%s", token.AccessToken),
		"application/x-www-form-urlencoded",
		nil,
	)
	if err != nil {
		return errors.NewAuthError("failed to revoke token", err)
	}
	defer resp.Body.Close()

	// Remove token file
	if err := os.Remove(a.tokenFile); err != nil && !os.IsNotExist(err) {
		return errors.NewAuthError("failed to remove token file", err)
	}

	a.logger.Info("Token revoked successfully")
	return nil
}

// Token represents the OAuth2 token with metadata
type Token struct {
	*oauth2.Token
	SavedAt time.Time `json:"saved_at"`
}

// loadToken loads a token from file
func (a *PulsePointGoogleAuth) loadToken() (*oauth2.Token, error) {
	if a.tokenFile == "" {
		return nil, fmt.Errorf("no token file specified")
	}

	data, err := os.ReadFile(a.tokenFile)
	if err != nil {
		return nil, err
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return token.Token, nil
}

// saveToken saves a token to file
func (a *PulsePointGoogleAuth) saveToken(token *oauth2.Token) error {
	if a.tokenFile == "" {
		return fmt.Errorf("no token file specified")
	}

	// Ensure directory exists
	dir := filepath.Dir(a.tokenFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Wrap token with metadata
	wrappedToken := Token{
		Token:   token,
		SavedAt: time.Now(),
	}

	data, err := json.MarshalIndent(wrappedToken, "", "  ")
	if err != nil {
		return err
	}

	// Save with restricted permissions (user read/write only)
	return os.WriteFile(a.tokenFile, data, 0600)
}

// IsAuthenticated checks if valid authentication exists
func (a *PulsePointGoogleAuth) IsAuthenticated() bool {
	token, err := a.loadToken()
	return err == nil && token != nil && token.Valid()
}

// GetTokenInfo returns information about the current token
func (a *PulsePointGoogleAuth) GetTokenInfo() (map[string]interface{}, error) {
	token, err := a.loadToken()
	if err != nil {
		return nil, err
	}

	info := map[string]interface{}{
		"valid":       token.Valid(),
		"expiry":      token.Expiry,
		"has_refresh": token.RefreshToken != "",
	}

	return info, nil
}
