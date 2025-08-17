package google

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pulsepoint/pulsepoint/pkg/errors"
)

// Credentials represents Google OAuth2 credentials
type Credentials struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURIs []string `json:"redirect_uris,omitempty"`
	AuthURI      string   `json:"auth_uri,omitempty"`
	TokenURI     string   `json:"token_uri,omitempty"`
}

// CredentialsFile represents the structure of a Google credentials JSON file
type CredentialsFile struct {
	Installed *Credentials `json:"installed,omitempty"`
	Web       *Credentials `json:"web,omitempty"`
}

// LoadCredentials loads Google OAuth2 credentials from a JSON file
func LoadCredentials(path string) (*Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.NewConfigError(fmt.Sprintf("failed to read credentials file: %s", path), err)
	}

	var credsFile CredentialsFile
	if err := json.Unmarshal(data, &credsFile); err != nil {
		return nil, errors.NewConfigError("invalid credentials file format", err)
	}

	// Try to get credentials (prefer installed over web)
	var creds *Credentials
	if credsFile.Installed != nil {
		creds = credsFile.Installed
	} else if credsFile.Web != nil {
		creds = credsFile.Web
	} else {
		return nil, errors.NewConfigError("no valid credentials found in file", nil)
	}

	if creds.ClientID == "" || creds.ClientSecret == "" {
		return nil, errors.NewConfigError("client_id and client_secret are required", nil)
	}

	return creds, nil
}

// SaveCredentials saves credentials to a JSON file
func SaveCredentials(path string, creds *Credentials) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.NewFileSystemError("failed to create directory", err)
	}

	// Create credentials file structure
	credsFile := CredentialsFile{
		Installed: creds,
	}

	data, err := json.MarshalIndent(credsFile, "", "  ")
	if err != nil {
		return errors.NewConfigError("failed to marshal credentials", err)
	}

	// Save with restricted permissions
	if err := os.WriteFile(path, data, 0600); err != nil {
		return errors.NewFileSystemError("failed to write credentials file", err)
	}

	return nil
}

// GetDefaultCredentialsPath returns the default path for storing credentials
func GetDefaultCredentialsPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".pulsepoint", "credentials", "google_credentials.json")
}

// GetDefaultTokenPath returns the default path for storing tokens
func GetDefaultTokenPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".pulsepoint", "tokens", "google_token.json")
}
