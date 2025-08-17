// Package google implements Google Drive provider for PulsePoint
package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	pperrors "github.com/pulsepoint/pulsepoint/pkg/errors"
	pplogger "github.com/pulsepoint/pulsepoint/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	// Provider name
	providerName = "google-drive"

	// Default MIME types
	mimeTypeFolder = "application/vnd.google-apps.folder"

	// Upload chunk size (8MB)
	uploadChunkSize = 8 * 1024 * 1024

	// Batch operation size
	batchSize = 100
)

// PulsePointGoogleDriveProvider implements CloudProvider for Google Drive
type PulsePointGoogleDriveProvider struct {
	service      *drive.Service
	config       *Config
	logger       *zap.Logger
	rootFolderID string
	tokenSource  oauth2.TokenSource
}

// Config holds Google Drive configuration
type Config struct {
	CredentialsFile          string   `json:"credentials_file"`
	TokenFile                string   `json:"token_file"`
	RootFolderID             string   `json:"root_folder_id"`
	Scopes                   []string `json:"scopes"`
	SimpleUploadThreshold    int64    `json:"simple_upload_threshold"`
	ResumableUploadThreshold int64    `json:"resumable_upload_threshold"`
	ChunkSize                int64    `json:"chunk_size"`
	MaxRetries               int      `json:"max_retries"`
	RateLimit                int      `json:"rate_limit"`
}

// NewPulsePointGoogleDriveProvider creates a new Google Drive provider
func NewPulsePointGoogleDriveProvider(config *Config) (*PulsePointGoogleDriveProvider, error) {
	logger := pplogger.Get()

	if config == nil {
		return nil, pperrors.NewConfigError("Google Drive configuration is required", nil)
	}

	// Set defaults
	if config.SimpleUploadThreshold == 0 {
		config.SimpleUploadThreshold = 5 * 1024 * 1024 // 5MB
	}
	if config.ResumableUploadThreshold == 0 {
		config.ResumableUploadThreshold = 100 * 1024 * 1024 // 100MB
	}
	if config.ChunkSize == 0 {
		config.ChunkSize = uploadChunkSize
	}

	provider := &PulsePointGoogleDriveProvider{
		config:       config,
		logger:       logger,
		rootFolderID: config.RootFolderID,
	}

	// Initialize OAuth2 client
	if err := provider.initializeClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize Google Drive client: %w", err)
	}

	return provider, nil
}

// initializeClient initializes the Google Drive API client
func (p *PulsePointGoogleDriveProvider) initializeClient() error {
	ctx := context.Background()

	// Read credentials file
	b, err := os.ReadFile(p.config.CredentialsFile)
	if err != nil {
		return fmt.Errorf("unable to read credentials file: %w", err)
	}

	// Parse credentials
	config, err := google.ConfigFromJSON(b, p.config.Scopes...)
	if err != nil {
		return fmt.Errorf("unable to parse credentials: %w", err)
	}

	// Get token
	token, err := p.loadToken()
	if err != nil {
		return fmt.Errorf("unable to load token: %w", err)
	}

	// Create token source
	p.tokenSource = config.TokenSource(ctx, token)

	// Create Drive service
	p.service, err = drive.NewService(ctx, option.WithTokenSource(p.tokenSource))
	if err != nil {
		return fmt.Errorf("unable to create Drive service: %w", err)
	}

	// If no root folder specified, use root
	if p.rootFolderID == "" {
		p.rootFolderID = "root"
	}

	p.logger.Info("Google Drive provider initialized",
		zap.String("root_folder", p.rootFolderID))

	return nil
}

// loadToken loads the OAuth2 token from file
func (p *PulsePointGoogleDriveProvider) loadToken() (*oauth2.Token, error) {
	f, err := os.Open(p.config.TokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	token := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(token)
	return token, err
}

// Initialize initializes the provider with configuration
func (p *PulsePointGoogleDriveProvider) Initialize(config interfaces.ProviderConfig) error {
	// Already initialized in constructor
	return nil
}

// Upload uploads a file to Google Drive
func (p *PulsePointGoogleDriveProvider) Upload(ctx context.Context, file *interfaces.File) error {
	if file == nil {
		return pperrors.NewValidationError("file is nil", nil)
	}

	p.logger.Debug("Uploading file to Google Drive",
		zap.String("path", file.Path),
		zap.Int64("size", file.Size))

	// Ensure parent folder exists
	parentID, err := p.ensureParentFolder(ctx, file.Path)
	if err != nil {
		return fmt.Errorf("failed to ensure parent folder: %w", err)
	}

	// Open local file
	localFile, err := os.Open(file.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Create Drive file metadata
	driveFile := &drive.File{
		Name:     filepath.Base(file.Path),
		Parents:  []string{parentID},
		MimeType: file.MimeType,
	}

	// Set modification time if available
	if !file.ModifiedTime.IsZero() {
		driveFile.ModifiedTime = file.ModifiedTime.Format(time.RFC3339)
	}

	// Check if file already exists
	existingFile, err := p.findFileByPath(ctx, file.Path)
	if err == nil && existingFile != nil {
		// Update existing file
		return p.updateFile(ctx, existingFile.Id, localFile, file)
	}

	// Create new file
	var uploadCall *drive.FilesCreateCall

	// Choose upload method based on file size
	if file.Size < p.config.SimpleUploadThreshold {
		// Simple upload for small files
		uploadCall = p.service.Files.Create(driveFile).Media(localFile)
	} else {
		// Resumable upload for large files
		uploadCall = p.service.Files.Create(driveFile).
			Media(localFile, googleapi.ChunkSize(int(p.config.ChunkSize)))
	}

	// Add fields to retrieve
	uploadCall = uploadCall.Fields("id, name, size, modifiedTime, md5Checksum")

	// Execute upload
	uploadedFile, err := uploadCall.Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	p.logger.Info("File uploaded successfully",
		zap.String("path", file.Path),
		zap.String("id", uploadedFile.Id))

	return nil
}

// updateFile updates an existing file
func (p *PulsePointGoogleDriveProvider) updateFile(ctx context.Context, fileID string, reader io.Reader, file *interfaces.File) error {
	updateCall := p.service.Files.Update(fileID, &drive.File{
		ModifiedTime: file.ModifiedTime.Format(time.RFC3339),
	})

	// Choose upload method based on file size
	if file.Size < p.config.SimpleUploadThreshold {
		updateCall = updateCall.Media(reader)
	} else {
		updateCall = updateCall.Media(reader, googleapi.ChunkSize(int(p.config.ChunkSize)))
	}

	_, err := updateCall.Context(ctx).Do()
	return err
}

// Download downloads a file from Google Drive
func (p *PulsePointGoogleDriveProvider) Download(ctx context.Context, path string) (*interfaces.File, error) {
	p.logger.Debug("Downloading file from Google Drive", zap.String("path", path))

	// Find file by path
	driveFile, err := p.findFileByPath(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// Get file content
	response, err := p.service.Files.Get(driveFile.Id).Download()
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer response.Body.Close()

	// Read content
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	// Parse modification time
	modTime, _ := time.Parse(time.RFC3339, driveFile.ModifiedTime)

	return &interfaces.File{
		Path:         path,
		Name:         driveFile.Name,
		Size:         driveFile.Size,
		Hash:         driveFile.Md5Checksum,
		ModifiedTime: modTime,
		MimeType:     driveFile.MimeType,
		Content:      bytes.NewReader(content),
		IsFolder:     false,
	}, nil
}

// Delete deletes a file from Google Drive
func (p *PulsePointGoogleDriveProvider) Delete(ctx context.Context, path string) error {
	p.logger.Debug("Deleting file from Google Drive", zap.String("path", path))

	// Find file by path
	driveFile, err := p.findFileByPath(ctx, path)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Delete file
	err = p.service.Files.Delete(driveFile.Id).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	p.logger.Info("File deleted successfully", zap.String("path", path))
	return nil
}

// List lists files in a folder
func (p *PulsePointGoogleDriveProvider) List(ctx context.Context, folder string) ([]*interfaces.File, error) {
	p.logger.Debug("Listing files in folder", zap.String("folder", folder))

	// Get folder ID
	folderID := p.rootFolderID
	if folder != "" && folder != "/" {
		driveFolder, err := p.findFileByPath(ctx, folder)
		if err != nil {
			return nil, fmt.Errorf("folder not found: %w", err)
		}
		folderID = driveFolder.Id
	}

	// List files
	var files []*interfaces.File
	pageToken := ""

	for {
		query := fmt.Sprintf("'%s' in parents and trashed = false", folderID)
		call := p.service.Files.List().
			Q(query).
			Fields("nextPageToken, files(id, name, size, modifiedTime, md5Checksum, mimeType)").
			PageSize(int64(batchSize))

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		result, err := call.Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("list failed: %w", err)
		}

		for _, driveFile := range result.Files {
			modTime, _ := time.Parse(time.RFC3339, driveFile.ModifiedTime)

			file := &interfaces.File{
				Path:         filepath.Join(folder, driveFile.Name),
				Name:         driveFile.Name,
				Size:         driveFile.Size,
				Hash:         driveFile.Md5Checksum,
				ModifiedTime: modTime,
				MimeType:     driveFile.MimeType,
				IsFolder:     driveFile.MimeType == mimeTypeFolder,
			}
			files = append(files, file)
		}

		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return files, nil
}

// GetMetadata gets file metadata
func (p *PulsePointGoogleDriveProvider) GetMetadata(ctx context.Context, path string) (*interfaces.Metadata, error) {
	p.logger.Debug("Getting file metadata", zap.String("path", path))

	// Find file by path
	driveFile, err := p.findFileByPath(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	// Get detailed metadata
	file, err := p.service.Files.Get(driveFile.Id).
		Fields("id, name, size, modifiedTime, createdTime, md5Checksum, mimeType, parents, webViewLink, owners, permissions").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	modTime, _ := time.Parse(time.RFC3339, file.ModifiedTime)
	createdTime, _ := time.Parse(time.RFC3339, file.CreatedTime)

	metadata := &interfaces.Metadata{
		Path:         path,
		Size:         file.Size,
		ModifiedTime: modTime,
		CreatedTime:  createdTime,
		Hash:         file.Md5Checksum,
		MimeType:     file.MimeType,
		IsFolder:     file.MimeType == mimeTypeFolder,
		Attributes: map[string]interface{}{
			"id":          file.Id,
			"webViewLink": file.WebViewLink,
			"parents":     file.Parents,
		},
	}

	// Add owner info
	if len(file.Owners) > 0 {
		metadata.Owner = file.Owners[0].EmailAddress
		metadata.Attributes["ownerEmail"] = file.Owners[0].EmailAddress
	}

	// Add permissions count
	if file.Permissions != nil {
		metadata.Attributes["permissionsCount"] = len(file.Permissions)
	}

	return metadata, nil
}

// CreateFolder creates a folder in Google Drive
func (p *PulsePointGoogleDriveProvider) CreateFolder(ctx context.Context, path string) error {
	p.logger.Debug("Creating folder", zap.String("path", path))

	// Check if folder already exists
	existing, _ := p.findFileByPath(ctx, path)
	if existing != nil {
		p.logger.Debug("Folder already exists", zap.String("path", path))
		return nil
	}

	// Get parent folder ID
	parentPath := filepath.Dir(path)
	parentID := p.rootFolderID
	if parentPath != "" && parentPath != "/" && parentPath != "." {
		parentFolder, err := p.ensureParentFolder(ctx, path)
		if err != nil {
			return err
		}
		parentID = parentFolder
	}

	// Create folder
	folder := &drive.File{
		Name:     filepath.Base(path),
		MimeType: mimeTypeFolder,
		Parents:  []string{parentID},
	}

	createdFolder, err := p.service.Files.Create(folder).
		Fields("id, name").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	p.logger.Info("Folder created successfully",
		zap.String("path", path),
		zap.String("id", createdFolder.Id))

	return nil
}

// Move moves a file or folder
func (p *PulsePointGoogleDriveProvider) Move(ctx context.Context, sourcePath, destPath string) error {
	p.logger.Debug("Moving file",
		zap.String("source", sourcePath),
		zap.String("destination", destPath))

	// Find source file
	sourceFile, err := p.findFileByPath(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("source file not found: %w", err)
	}

	// Get new parent folder
	newParentPath := filepath.Dir(destPath)
	newParentID := p.rootFolderID
	if newParentPath != "" && newParentPath != "/" && newParentPath != "." {
		newParentFolder, err := p.ensureParentFolder(ctx, destPath)
		if err != nil {
			return err
		}
		newParentID = newParentFolder
	}

	// Get current parents
	file, err := p.service.Files.Get(sourceFile.Id).Fields("parents").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get current parents: %w", err)
	}

	// Update file with new parent and name
	update := &drive.File{
		Name: filepath.Base(destPath),
	}

	_, err = p.service.Files.Update(sourceFile.Id, update).
		AddParents(newParentID).
		RemoveParents(strings.Join(file.Parents, ",")).
		Fields("id, name, parents").
		Context(ctx).
		Do()

	if err != nil {
		return fmt.Errorf("move failed: %w", err)
	}

	p.logger.Info("File moved successfully",
		zap.String("source", sourcePath),
		zap.String("destination", destPath))

	return nil
}

// GetQuota gets storage quota information
func (p *PulsePointGoogleDriveProvider) GetQuota(ctx context.Context) (*interfaces.QuotaInfo, error) {
	about, err := p.service.About.Get().
		Fields("storageQuota, user").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get quota: %w", err)
	}

	quota := &interfaces.QuotaInfo{
		Used:      about.StorageQuota.Usage,
		Available: about.StorageQuota.Limit - about.StorageQuota.Usage,
		Total:     about.StorageQuota.Limit,
	}

	return quota, nil
}

// GetProviderName returns the provider name
func (p *PulsePointGoogleDriveProvider) GetProviderName() string {
	return providerName
}

// IsConnected checks if the provider is connected
func (p *PulsePointGoogleDriveProvider) IsConnected() bool {
	return p.service != nil
}

// Disconnect closes the connection to the provider
func (p *PulsePointGoogleDriveProvider) Disconnect() error {
	// Google Drive client doesn't need explicit disconnect
	// but we can clear the service reference
	p.service = nil
	p.tokenSource = nil
	p.logger.Info("Disconnected from Google Drive")
	return nil
}

// Helper methods

// findFileByPath finds a file by its path
func (p *PulsePointGoogleDriveProvider) findFileByPath(ctx context.Context, path string) (*drive.File, error) {
	// Clean and split path
	path = filepath.Clean(path)
	if path == "/" || path == "." {
		// Return root folder
		return &drive.File{Id: p.rootFolderID}, nil
	}

	parts := strings.Split(path, string(filepath.Separator))
	currentParentID := p.rootFolderID

	// Navigate through path
	for _, part := range parts {
		if part == "" {
			continue
		}

		query := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false", part, currentParentID)
		result, err := p.service.Files.List().
			Q(query).
			Fields("files(id, name, mimeType)").
			PageSize(1).
			Context(ctx).
			Do()

		if err != nil {
			return nil, err
		}

		if len(result.Files) == 0 {
			return nil, fmt.Errorf("file not found: %s", part)
		}

		currentParentID = result.Files[0].Id
	}

	// Get the final file
	file, err := p.service.Files.Get(currentParentID).
		Fields("id, name, size, modifiedTime, md5Checksum, mimeType").
		Context(ctx).
		Do()

	return file, err
}

// ensureParentFolder ensures parent folder exists, creating if necessary
func (p *PulsePointGoogleDriveProvider) ensureParentFolder(ctx context.Context, filePath string) (string, error) {
	parentPath := filepath.Dir(filePath)
	if parentPath == "" || parentPath == "/" || parentPath == "." {
		return p.rootFolderID, nil
	}

	// Try to find existing folder
	folder, err := p.findFileByPath(ctx, parentPath)
	if err == nil {
		return folder.Id, nil
	}

	// Create folder hierarchy
	parts := strings.Split(parentPath, string(filepath.Separator))
	currentParentID := p.rootFolderID
	currentPath := ""

	for _, part := range parts {
		if part == "" {
			continue
		}

		if currentPath == "" {
			currentPath = part
		} else {
			currentPath = filepath.Join(currentPath, part)
		}

		// Check if folder exists
		query := fmt.Sprintf("name = '%s' and '%s' in parents and mimeType = '%s' and trashed = false",
			part, currentParentID, mimeTypeFolder)

		result, err := p.service.Files.List().
			Q(query).
			Fields("files(id)").
			PageSize(1).
			Context(ctx).
			Do()

		if err != nil {
			return "", err
		}

		if len(result.Files) > 0 {
			// Folder exists
			currentParentID = result.Files[0].Id
		} else {
			// Create folder
			folder := &drive.File{
				Name:     part,
				MimeType: mimeTypeFolder,
				Parents:  []string{currentParentID},
			}

			created, err := p.service.Files.Create(folder).
				Fields("id").
				Context(ctx).
				Do()
			if err != nil {
				return "", fmt.Errorf("failed to create folder %s: %w", currentPath, err)
			}

			currentParentID = created.Id
			p.logger.Debug("Created folder", zap.String("path", currentPath))
		}
	}

	return currentParentID, nil
}
