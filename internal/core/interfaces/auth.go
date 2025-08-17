package interfaces

import (
	"context"
	"time"
)

// AuthProvider defines the contract for authentication providers
type AuthProvider interface {
	// Authenticate performs authentication
	Authenticate(ctx context.Context) (*AuthToken, error)

	// RefreshToken refreshes an expired token
	RefreshToken(ctx context.Context, token *AuthToken) (*AuthToken, error)

	// RevokeToken revokes a token
	RevokeToken(ctx context.Context, token *AuthToken) error

	// ValidateToken validates if a token is still valid
	ValidateToken(ctx context.Context, token *AuthToken) (bool, error)

	// GetAuthURL returns the authorization URL for OAuth flows
	GetAuthURL(state string) (string, error)

	// HandleCallback handles OAuth callback
	HandleCallback(ctx context.Context, code, state string) (*AuthToken, error)

	// StoreToken securely stores a token
	StoreToken(token *AuthToken) error

	// LoadToken loads a stored token
	LoadToken() (*AuthToken, error)

	// DeleteToken deletes a stored token
	DeleteToken() error

	// GetProviderName returns the auth provider name
	GetProviderName() string

	// RequiresInteraction checks if user interaction is needed
	RequiresInteraction() bool
}

// AuthToken represents an authentication token
type AuthToken struct {
	AccessToken  string                 `json:"access_token"`
	RefreshToken string                 `json:"refresh_token,omitempty"`
	TokenType    string                 `json:"token_type"`
	ExpiresAt    time.Time              `json:"expires_at"`
	Scope        string                 `json:"scope,omitempty"`
	Provider     string                 `json:"provider"`
	UserID       string                 `json:"user_id,omitempty"`
	Email        string                 `json:"email,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}

// IsExpired checks if the token is expired
func (t *AuthToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the token is valid
func (t *AuthToken) IsValid() bool {
	return t.AccessToken != "" && !t.IsExpired()
}

// TimeUntilExpiry returns the duration until token expires
func (t *AuthToken) TimeUntilExpiry() time.Duration {
	return time.Until(t.ExpiresAt)
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Provider     string                 `json:"provider"`
	ClientID     string                 `json:"client_id,omitempty"`
	ClientSecret string                 `json:"-"` // Never serialize
	RedirectURL  string                 `json:"redirect_url,omitempty"`
	Scopes       []string               `json:"scopes,omitempty"`
	TokenStorage string                 `json:"token_storage,omitempty"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
}
