package gdrive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/auth/google"
	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	"github.com/pulsepoint/pulsepoint/pkg/errors"
	"github.com/pulsepoint/pulsepoint/pkg/logger"

	"go.uber.org/zap"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// PulsePointGoogleDriveProvider implements CloudProvider for Google Drive
type PulsePointGoogleDriveProvider struct {
	auth         *google.PulsePointGoogleAuth
	service      *drive.Service
	rootFolderID string
	logger       *zap.Logger

	// Upload configuration
	simpleUploadThreshold    int64
	resumableUploadThreshold int64
	chunkSize                int64
	maxRetries               int
	retryDelay               time.Duration
}

// Config holds configuration for Google Drive provider
type Config struct {
	ClientID                 string
	ClientSecret             string
	TokenFile                string
	RootFolderID             string // Optional: specific folder to use as root
	SimpleUploadThreshold    int64  // Files smaller than this use simple upload (default 5MB)
	ResumableUploadThreshold int64  // Files larger than this use resumable upload (default 100MB)
	ChunkSize                int64  // Chunk size for uploads (default 8MB)
	MaxRetries               int    // Maximum retry attempts
}

// NewPulsePointGoogleDriveProvider creates a new Google Drive provider
func NewPulsePointGoogleDriveProvider(ctx context.Context, cfg *Config) (interfaces.CloudProvider, error) {
	// Set defaults
	if cfg.SimpleUploadThreshold == 0 {
		cfg.SimpleUploadThreshold = 5 * 1024 * 1024 // 5MB
	}
	if cfg.ResumableUploadThreshold == 0 {
		cfg.ResumableUploadThreshold = 100 * 1024 * 1024 // 100MB
	}
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = 8 * 1024 * 1024 // 8MB
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	// Create OAuth config
	oauthConfig := &google.PulsePointOAuthConfig{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
	}

	// Create auth handler
	auth, err := google.NewPulsePointGoogleAuth(oauthConfig, cfg.TokenFile)
	if err != nil {
		return nil, err
	}

	// Get Drive service
	service, err := auth.GetDriveService(ctx)
	if err != nil {
		return nil, err
	}

	provider := &PulsePointGoogleDriveProvider{
		auth:                     auth,
		service:                  service,
		rootFolderID:             cfg.RootFolderID,
		logger:                   logger.Get(),
		simpleUploadThreshold:    cfg.SimpleUploadThreshold,
		resumableUploadThreshold: cfg.ResumableUploadThreshold,
		chunkSize:                cfg.ChunkSize,
		maxRetries:               cfg.MaxRetries,
		retryDelay:               2 * time.Second,
	}

	// Verify root folder if specified
	if cfg.RootFolderID != "" {
		if err := provider.verifyRootFolder(ctx); err != nil {
			return nil, err
		}
	}

	return provider, nil
}

// Initialize sets up the provider with configuration
func (p *PulsePointGoogleDriveProvider) Initialize(config interfaces.ProviderConfig) error {
	// Already initialized in constructor
	return nil
}

// Upload uploads a file to Google Drive
func (p *PulsePointGoogleDriveProvider) Upload(ctx context.Context, file *interfaces.File) error {
	if file == nil {
		return errors.NewValidationError("file cannot be nil", nil)
	}

	p.logger.Info("Uploading file to Google Drive",
		zap.String("path", file.Path),
		zap.Int64("size", file.Size))

	// Prepare file metadata
	driveFile := &drive.File{
		Name:     filepath.Base(file.Path),
		MimeType: p.getMimeType(file.Path),
	}

	// Set parent folder
	parentID, err := p.ensureParentFolder(ctx, file.Path)
	if err != nil {
		return errors.NewProviderError("failed to ensure parent folder", err)
	}
	if parentID != "" {
		driveFile.Parents = []string{parentID}
	}

	// Check if file exists (for update vs create)
	existingFile, err := p.findFileByPath(ctx, file.Path)
	if err != nil && !isNotFoundError(err) {
		return errors.NewProviderError("failed to check existing file", err)
	}

	// Open local file
	localFile, err := os.Open(file.LocalPath)
	if err != nil {
		return errors.NewFileSystemError(fmt.Sprintf("failed to open file: %s", file.LocalPath), err)
	}
	defer localFile.Close()

	// Choose upload strategy based on file size
	var uploadErr error
	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			p.logger.Debug("Retrying upload",
				zap.Int("attempt", attempt),
				zap.String("file", file.Path))
			time.Sleep(p.retryDelay * time.Duration(attempt))
		}

		if existingFile != nil {
			// Update existing file
			uploadErr = p.updateFile(ctx, existingFile.Id, driveFile, localFile, file.Size)
		} else {
			// Create new file
			uploadErr = p.createFile(ctx, driveFile, localFile, file.Size)
		}

		if uploadErr == nil {
			break
		}

		// Check if error is retryable
		if !isRetryableError(uploadErr) {
			break
		}
	}

	if uploadErr != nil {
		return errors.NewProviderError("upload failed", uploadErr)
	}

	p.logger.Info("File uploaded successfully", zap.String("path", file.Path))
	return nil
}

// createFile creates a new file in Google Drive
func (p *PulsePointGoogleDriveProvider) createFile(ctx context.Context, driveFile *drive.File, content io.Reader, size int64) error {
	var err error

	if size < p.simpleUploadThreshold {
		// Simple upload for small files
		_, err = p.service.Files.Create(driveFile).
			Media(content).
			Context(ctx).
			Do()
	} else {
		// Resumable upload for larger files
		_, err = p.service.Files.Create(driveFile).
			Media(content).
			Context(ctx).
			Do()
	}

	return err
}

// updateFile updates an existing file in Google Drive
func (p *PulsePointGoogleDriveProvider) updateFile(ctx context.Context, fileID string, driveFile *drive.File, content io.Reader, size int64) error {
	var err error

	if size < p.simpleUploadThreshold {
		// Simple update for small files
		_, err = p.service.Files.Update(fileID, driveFile).
			Media(content).
			Context(ctx).
			Do()
	} else {
		// Resumable update for larger files
		_, err = p.service.Files.Update(fileID, driveFile).
			Media(content).
			Context(ctx).
			Do()
	}

	return err
}

// Download downloads a file from Google Drive
func (p *PulsePointGoogleDriveProvider) Download(ctx context.Context, path string) (*interfaces.File, error) {
	p.logger.Info("Downloading file from Google Drive", zap.String("path", path))

	// Find file by path
	driveFile, err := p.findFileByPath(ctx, path)
	if err != nil {
		return nil, errors.NewProviderError("file not found", err)
	}

	// Get file content
	resp, err := p.service.Files.Get(driveFile.Id).Download()
	if err != nil {
		return nil, errors.NewProviderError("download failed", err)
	}
	defer resp.Body.Close()

	// Read content
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewProviderError("failed to read content", err)
	}

	// Create file model
	file := &interfaces.File{
		Path:         path,
		Size:         driveFile.Size,
		Content:      bytes.NewReader(content),
		Hash:         driveFile.Md5Checksum,
		MimeType:     driveFile.MimeType,
		ModifiedTime: parseTime(driveFile.ModifiedTime),
		IsFolder:     driveFile.MimeType == "application/vnd.google-apps.folder",
	}

	return file, nil
}

// Delete deletes a file from Google Drive
func (p *PulsePointGoogleDriveProvider) Delete(ctx context.Context, path string) error {
	p.logger.Info("Deleting file from Google Drive", zap.String("path", path))

	// Find file by path
	driveFile, err := p.findFileByPath(ctx, path)
	if err != nil {
		if isNotFoundError(err) {
			// File doesn't exist, consider it deleted
			return nil
		}
		return errors.NewProviderError("failed to find file", err)
	}

	// Delete file
	err = p.service.Files.Delete(driveFile.Id).Context(ctx).Do()
	if err != nil {
		return errors.NewProviderError("delete failed", err)
	}

	p.logger.Info("File deleted successfully", zap.String("path", path))
	return nil
}

// List lists files in a folder
func (p *PulsePointGoogleDriveProvider) List(ctx context.Context, folder string) ([]*interfaces.File, error) {
	p.logger.Debug("Listing files in folder", zap.String("folder", folder))

	// Find folder
	var parentID string
	if folder != "" && folder != "/" {
		folderFile, err := p.findFileByPath(ctx, folder)
		if err != nil {
			return nil, errors.NewProviderError("folder not found", err)
		}
		parentID = folderFile.Id
	} else {
		parentID = p.rootFolderID
		if parentID == "" {
			parentID = "root"
		}
	}

	// Build query
	query := fmt.Sprintf("'%s' in parents and trashed = false", parentID)

	// List files
	var files []*interfaces.File
	pageToken := ""

	for {
		call := p.service.Files.List().
			Q(query).
			Fields("nextPageToken, files(id, name, mimeType, size, modifiedTime, md5Checksum)").
			PageSize(100).
			Context(ctx)

		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return nil, errors.NewProviderError("list failed", err)
		}

		for _, driveFile := range resp.Files {
			file := &interfaces.File{
				Path:         filepath.Join(folder, driveFile.Name),
				Name:         driveFile.Name,
				Size:         driveFile.Size,
				Hash:         driveFile.Md5Checksum,
				MimeType:     driveFile.MimeType,
				ModifiedTime: parseTime(driveFile.ModifiedTime),
				IsFolder:     driveFile.MimeType == "application/vnd.google-apps.folder",
			}
			files = append(files, file)
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return files, nil
}

// GetMetadata gets metadata for a file
func (p *PulsePointGoogleDriveProvider) GetMetadata(ctx context.Context, path string) (*interfaces.Metadata, error) {
	p.logger.Debug("Getting metadata", zap.String("path", path))

	// Find file by path
	driveFile, err := p.findFileByPath(ctx, path)
	if err != nil {
		return nil, errors.NewProviderError("file not found", err)
	}

	// Get full metadata
	file, err := p.service.Files.Get(driveFile.Id).
		Fields("*").
		Context(ctx).
		Do()
	if err != nil {
		return nil, errors.NewProviderError("failed to get metadata", err)
	}

	metadata := &interfaces.Metadata{
		Path:         path,
		Size:         file.Size,
		ModifiedTime: parseTime(file.ModifiedTime),
		Hash:         file.Md5Checksum,
		MimeType:     file.MimeType,
		IsFolder:     file.MimeType == "application/vnd.google-apps.folder",
		ID:           file.Id,
		Version:      strconv.FormatInt(file.Version, 10),
		Attributes: map[string]interface{}{
			"webViewLink":     file.WebViewLink,
			"webContentLink":  file.WebContentLink,
			"iconLink":        file.IconLink,
			"thumbnailLink":   file.ThumbnailLink,
			"owners":          file.Owners,
			"shared":          file.Shared,
			"starred":         file.Starred,
			"writersCanShare": file.WritersCanShare,
		},
	}

	return metadata, nil
}

// CreateFolder creates a folder in Google Drive
func (p *PulsePointGoogleDriveProvider) CreateFolder(ctx context.Context, path string) error {
	p.logger.Info("Creating folder", zap.String("path", path))

	// Check if folder already exists
	existing, _ := p.findFileByPath(ctx, path)
	if existing != nil {
		if existing.MimeType == "application/vnd.google-apps.folder" {
			// Folder already exists
			return nil
		}
		return errors.NewProviderError("path exists but is not a folder", nil)
	}

	// Prepare folder metadata
	folder := &drive.File{
		Name:     filepath.Base(path),
		MimeType: "application/vnd.google-apps.folder",
	}

	// Set parent folder
	parentPath := filepath.Dir(path)
	if parentPath != "." && parentPath != "/" {
		parentID, err := p.ensureParentFolder(ctx, path)
		if err != nil {
			return errors.NewProviderError("failed to ensure parent folder", err)
		}
		if parentID != "" {
			folder.Parents = []string{parentID}
		}
	} else if p.rootFolderID != "" {
		folder.Parents = []string{p.rootFolderID}
	}

	// Create folder
	_, err := p.service.Files.Create(folder).Context(ctx).Do()
	if err != nil {
		return errors.NewProviderError("failed to create folder", err)
	}

	p.logger.Info("Folder created successfully", zap.String("path", path))
	return nil
}

// Helper methods

// findFileByPath finds a file by its path
func (p *PulsePointGoogleDriveProvider) findFileByPath(ctx context.Context, path string) (*drive.File, error) {
	// Split path into components
	parts := strings.Split(strings.Trim(path, "/"), "/")

	// Start from root
	parentID := p.rootFolderID
	if parentID == "" {
		parentID = "root"
	}

	var currentFile *drive.File

	// Traverse path
	for _, part := range parts {
		if part == "" {
			continue
		}

		query := fmt.Sprintf("name = '%s' and '%s' in parents and trashed = false",
			escapeQueryString(part), parentID)

		resp, err := p.service.Files.List().
			Q(query).
			Fields("files(id, name, mimeType)").
			PageSize(1).
			Context(ctx).
			Do()

		if err != nil {
			return nil, err
		}

		if len(resp.Files) == 0 {
			return nil, fmt.Errorf("file not found: %s", path)
		}

		currentFile = resp.Files[0]
		parentID = currentFile.Id
	}

	return currentFile, nil
}

// ensureParentFolder ensures parent folder exists, creating if necessary
func (p *PulsePointGoogleDriveProvider) ensureParentFolder(ctx context.Context, filePath string) (string, error) {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "/" {
		if p.rootFolderID != "" {
			return p.rootFolderID, nil
		}
		return "", nil
	}

	// Try to find existing folder
	folder, err := p.findFileByPath(ctx, dir)
	if err == nil {
		return folder.Id, nil
	}

	// Create folder hierarchy
	parts := strings.Split(strings.Trim(dir, "/"), "/")
	parentID := p.rootFolderID
	if parentID == "" {
		parentID = "root"
	}

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Check if folder exists
		query := fmt.Sprintf("name = '%s' and '%s' in parents and mimeType = 'application/vnd.google-apps.folder' and trashed = false",
			escapeQueryString(part), parentID)

		resp, err := p.service.Files.List().
			Q(query).
			Fields("files(id)").
			PageSize(1).
			Context(ctx).
			Do()

		if err != nil {
			return "", err
		}

		if len(resp.Files) > 0 {
			// Folder exists
			parentID = resp.Files[0].Id
		} else {
			// Create folder
			folder := &drive.File{
				Name:     part,
				MimeType: "application/vnd.google-apps.folder",
				Parents:  []string{parentID},
			}

			created, err := p.service.Files.Create(folder).Context(ctx).Do()
			if err != nil {
				return "", err
			}

			parentID = created.Id
		}
	}

	return parentID, nil
}

// verifyRootFolder verifies that the root folder exists
func (p *PulsePointGoogleDriveProvider) verifyRootFolder(ctx context.Context) error {
	_, err := p.service.Files.Get(p.rootFolderID).
		Fields("id, name, mimeType").
		Context(ctx).
		Do()

	if err != nil {
		return errors.NewProviderError("root folder not found or inaccessible", err)
	}

	return nil
}

// getMimeType gets MIME type for a file
func (p *PulsePointGoogleDriveProvider) getMimeType(path string) string {
	// Basic MIME type detection by extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}

// Helper functions

func parseTime(timeStr string) time.Time {
	t, _ := time.Parse(time.RFC3339, timeStr)
	return t
}

func escapeQueryString(s string) string {
	// Escape single quotes for Drive API queries
	return strings.ReplaceAll(s, "'", "\\'")
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404")
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for Google API errors
	if apiErr, ok := err.(*googleapi.Error); ok {
		// Retry on rate limit, server errors, and timeout
		return apiErr.Code == 429 || apiErr.Code >= 500 || apiErr.Code == 408
	}

	// Check for network errors
	errStr := err.Error()
	return strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "temporary")
}

// Move moves/renames a file or folder
func (p *PulsePointGoogleDriveProvider) Move(ctx context.Context, sourcePath, destPath string) error {
	p.logger.Info("Moving file",
		zap.String("from", sourcePath),
		zap.String("to", destPath))

	// Find source file
	sourceFile, err := p.findFileByPath(ctx, sourcePath)
	if err != nil {
		return errors.NewProviderError("source file not found", err)
	}

	// Prepare update with new name and/or parent
	update := &drive.File{
		Name: filepath.Base(destPath),
	}

	// If moving to different folder, update parent
	sourceDir := filepath.Dir(sourcePath)
	destDir := filepath.Dir(destPath)
	if sourceDir != destDir {
		// Find new parent folder
		newParentID, err := p.ensureParentFolder(ctx, destPath)
		if err != nil {
			return errors.NewProviderError("failed to find destination folder", err)
		}

		// Remove from old parent and add to new parent
		_, err = p.service.Files.Update(sourceFile.Id, update).
			AddParents(newParentID).
			RemoveParents(sourceFile.Parents[0]).
			Context(ctx).
			Do()
		if err != nil {
			return errors.NewProviderError("move failed", err)
		}
	} else {
		// Just rename
		_, err = p.service.Files.Update(sourceFile.Id, update).
			Context(ctx).
			Do()
		if err != nil {
			return errors.NewProviderError("rename failed", err)
		}
	}

	p.logger.Info("File moved successfully")
	return nil
}

// GetQuota returns storage quota information
func (p *PulsePointGoogleDriveProvider) GetQuota(ctx context.Context) (*interfaces.QuotaInfo, error) {
	about, err := p.service.About.Get().
		Fields("storageQuota").
		Context(ctx).
		Do()
	if err != nil {
		return nil, errors.NewProviderError("failed to get quota", err)
	}

	quota := &interfaces.QuotaInfo{
		Used:  about.StorageQuota.Usage,
		Total: about.StorageQuota.Limit,
	}

	if quota.Total > 0 {
		quota.Available = quota.Total - quota.Used
	}

	return quota, nil
}

// GetProviderName returns the name of the provider
func (p *PulsePointGoogleDriveProvider) GetProviderName() string {
	return "Google Drive"
}

// IsConnected checks if the provider is connected
func (p *PulsePointGoogleDriveProvider) IsConnected() bool {
	// Try a simple API call to check connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := p.service.About.Get().
		Fields("user").
		Context(ctx).
		Do()

	return err == nil
}

// Disconnect closes the connection to the provider
func (p *PulsePointGoogleDriveProvider) Disconnect() error {
	// Google Drive doesn't require explicit disconnect
	// Token remains valid until revoked
	p.logger.Info("Disconnecting from Google Drive")
	return nil
}
