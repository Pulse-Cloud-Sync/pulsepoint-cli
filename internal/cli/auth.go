package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/auth/google"
	pplogger "github.com/pulsepoint/pulsepoint/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth [provider]",
	Short: "Manage authentication with cloud providers",
	Long: `Authenticate PulsePoint with cloud storage providers.
	
Currently supported providers:
- google (Google Drive)

Future support planned for:
- dropbox (Dropbox)
- onedrive (Microsoft OneDrive)
- s3 (Amazon S3)`,
	Args: cobra.ExactArgs(1),
	RunE: runAuth,
}

func init() {
	authCmd.Flags().Bool("revoke", false, "Revoke existing authentication")
	authCmd.Flags().Bool("status", false, "Check authentication status")
	authCmd.Flags().String("credentials", "", "Path to Google credentials JSON file")
	authCmd.Flags().String("token-file", "", "Path to store OAuth2 token (default: ~/.pulsepoint/tokens/google_token.json)")
}

func runAuth(cmd *cobra.Command, args []string) error {
	provider := args[0]
	revoke, _ := cmd.Flags().GetBool("revoke")
	status, _ := cmd.Flags().GetBool("status")

	switch provider {
	case "google", "gdrive":
		if status {
			return checkGoogleAuthStatus()
		}
		if revoke {
			return revokeGoogleAuth()
		}
		return authenticateGoogle()
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
}

func authenticateGoogle() error {
	log := pplogger.Get()
	fmt.Println("🔐 Initiating Google Drive authentication...")

	// Get credentials path from environment or config
	credentialsPath := os.Getenv("GOOGLE_CREDENTIALS_FILE")
	if credentialsPath == "" {
		// Check for credentials in config
		credentialsPath = viper.GetString("providers.google.credentials_file")
		if credentialsPath == "" {
			// Try default location
			credentialsPath = google.GetDefaultCredentialsPath()

			// Check if file exists
			if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
				fmt.Println("\n⚠️  No Google credentials file found!")
				fmt.Println("\nTo authenticate with Google Drive, you need to:")
				fmt.Println("1. Go to https://console.cloud.google.com/")
				fmt.Println("2. Create a new project or select existing one")
				fmt.Println("3. Enable Google Drive API")
				fmt.Println("4. Create OAuth2 credentials (Desktop application type)")
				fmt.Println("5. Download the credentials JSON file")
				fmt.Printf("6. Save it to: %s\n", credentialsPath)
				fmt.Println("   Or use --credentials flag to specify a different path")
				fmt.Println("\nAlternatively, set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables")
				return fmt.Errorf("credentials not configured")
			}
		}
	}

	// Get token file path
	tokenFile := os.Getenv("GOOGLE_TOKEN_FILE")
	if tokenFile == "" {
		tokenFile = viper.GetString("providers.google.token_file")
		if tokenFile == "" {
			tokenFile = google.GetDefaultTokenPath()
		}
	}

	// Try to load credentials
	var clientID, clientSecret string

	// First check environment variables
	clientID = os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")

	// If not in env, try to load from file
	if clientID == "" || clientSecret == "" {
		if credentialsPath != "" {
			creds, err := google.LoadCredentials(credentialsPath)
			if err != nil {
				return fmt.Errorf("failed to load credentials: %w", err)
			}
			clientID = creds.ClientID
			clientSecret = creds.ClientSecret
		}
	}

	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("Google client ID and secret are required")
	}

	// Create OAuth config
	oauthConfig := &google.PulsePointOAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	// Create auth handler
	auth, err := google.NewPulsePointGoogleAuth(oauthConfig, tokenFile)
	if err != nil {
		return fmt.Errorf("failed to create auth handler: %w", err)
	}

	// Check if already authenticated
	if auth.IsAuthenticated() {
		fmt.Println("✅ Already authenticated with Google Drive")
		fmt.Println("   Use --revoke to remove existing authentication")
		return nil
	}

	// Perform authentication
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_, err = auth.Authenticate(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Test the authentication by creating a Drive service
	service, err := auth.GetDriveService(ctx)
	if err != nil {
		return fmt.Errorf("failed to create Drive service: %w", err)
	}

	// Get user info
	about, err := service.About.Get().Fields("user").Do()
	if err != nil {
		log.Warn("Failed to get user info", zap.Error(err))
	} else if about.User != nil {
		fmt.Printf("✅ Authenticated as: %s\n", about.User.EmailAddress)
	}

	fmt.Println("🔑 Credentials saved securely to:", tokenFile)

	// Update config with paths
	viper.Set("providers.google.credentials_file", credentialsPath)
	viper.Set("providers.google.token_file", tokenFile)
	viper.Set("providers.google.configured", true)

	if err := viper.WriteConfig(); err != nil {
		log.Warn("Failed to update config file", zap.Error(err))
	}

	return nil
}

func revokeGoogleAuth() error {
	fmt.Println("🔓 Revoking Google Drive authentication...")

	// Get token file path
	tokenFile := os.Getenv("GOOGLE_TOKEN_FILE")
	if tokenFile == "" {
		tokenFile = viper.GetString("providers.google.token_file")
		if tokenFile == "" {
			tokenFile = google.GetDefaultTokenPath()
		}
	}

	// Create minimal OAuth config for revocation
	oauthConfig := &google.PulsePointOAuthConfig{
		ClientID:     "temp", // Will be ignored for revocation
		ClientSecret: "temp", // Will be ignored for revocation
	}

	// Create auth handler
	auth, err := google.NewPulsePointGoogleAuth(oauthConfig, tokenFile)
	if err != nil {
		return fmt.Errorf("failed to create auth handler: %w", err)
	}

	// Revoke token
	if err := auth.RevokeToken(); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	// Clear config
	viper.Set("providers.google.configured", false)
	viper.WriteConfig()

	fmt.Println("✅ Authentication revoked successfully")
	return nil
}

func checkGoogleAuthStatus() error {
	fmt.Println("🔍 Checking Google Drive authentication status...")

	// Get token file path
	tokenFile := os.Getenv("GOOGLE_TOKEN_FILE")
	if tokenFile == "" {
		tokenFile = viper.GetString("providers.google.token_file")
		if tokenFile == "" {
			tokenFile = google.GetDefaultTokenPath()
		}
	}

	// Check if token file exists
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		fmt.Println("❌ Not authenticated")
		fmt.Println("   Run 'pulsepoint auth google' to authenticate")
		return nil
	}

	// Create minimal OAuth config
	oauthConfig := &google.PulsePointOAuthConfig{
		ClientID:     "temp", // Will be loaded from token
		ClientSecret: "temp", // Will be loaded from token
	}

	// Create auth handler
	auth, err := google.NewPulsePointGoogleAuth(oauthConfig, tokenFile)
	if err != nil {
		return fmt.Errorf("failed to create auth handler: %w", err)
	}

	// Check authentication status
	if !auth.IsAuthenticated() {
		fmt.Println("⚠️  Token exists but is not valid")
		fmt.Println("   Run 'pulsepoint auth google' to re-authenticate")
		return nil
	}

	// Get token info
	info, err := auth.GetTokenInfo()
	if err != nil {
		return fmt.Errorf("failed to get token info: %w", err)
	}

	fmt.Println("✅ Authenticated with Google Drive")

	if expiry, ok := info["expiry"].(time.Time); ok {
		fmt.Printf("📅 Token expires: %s\n", expiry.Format("2006-01-02 15:04:05"))
		if time.Until(expiry) < 24*time.Hour {
			fmt.Println("⚠️  Token expires soon, consider re-authenticating")
		}
	}

	if hasRefresh, ok := info["has_refresh"].(bool); ok && hasRefresh {
		fmt.Println("🔄 Refresh token available (auto-renewal enabled)")
	}

	// Try to get user info
	ctx := context.Background()

	// Load actual credentials for service creation
	credentialsPath := viper.GetString("providers.google.credentials_file")
	if credentialsPath == "" {
		credentialsPath = google.GetDefaultCredentialsPath()
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if (clientID == "" || clientSecret == "") && credentialsPath != "" {
		if creds, err := google.LoadCredentials(credentialsPath); err == nil {
			clientID = creds.ClientID
			clientSecret = creds.ClientSecret
		}
	}

	if clientID != "" && clientSecret != "" {
		oauthConfig := &google.PulsePointOAuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		}

		if auth, err := google.NewPulsePointGoogleAuth(oauthConfig, tokenFile); err == nil {
			if service, err := auth.GetDriveService(ctx); err == nil {
				if about, err := service.About.Get().Fields("user,storageQuota").Do(); err == nil {
					if about.User != nil {
						fmt.Printf("👤 User: %s\n", about.User.EmailAddress)
					}
					if about.StorageQuota != nil {
						used := about.StorageQuota.Usage
						limit := about.StorageQuota.Limit
						if limit > 0 {
							usedGB := float64(used) / (1024 * 1024 * 1024)
							limitGB := float64(limit) / (1024 * 1024 * 1024)
							percentage := (float64(used) / float64(limit)) * 100
							fmt.Printf("💾 Storage: %.2f GB / %.2f GB (%.1f%% used)\n", usedGB, limitGB, percentage)
						}
					}
				}
			}
		}
	}

	return nil
}
