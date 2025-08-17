package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/pulsepoint/pulsepoint/internal/core/interfaces"
	pperrors "github.com/pulsepoint/pulsepoint/pkg/errors"
	"github.com/pulsepoint/pulsepoint/pkg/models"
	"go.uber.org/zap"
)

// PulsePointPipeline represents the sync pipeline with multiple phases
type PulsePointPipeline struct {
	engine *PulsePointEngine
	logger *zap.Logger

	// Pipeline phases
	phases []PipelinePhase

	// Configuration
	config *PipelineConfig
}

// PipelinePhase represents a phase in the sync pipeline
type PipelinePhase interface {
	Name() string
	Execute(ctx context.Context, input *PipelineInput) (*PipelineOutput, error)
	Validate(input *PipelineInput) error
}

// PipelineConfig holds pipeline configuration
type PipelineConfig struct {
	MaxRetries       int           `json:"max_retries"`
	RetryDelay       time.Duration `json:"retry_delay"`
	Timeout          time.Duration `json:"timeout"`
	EnableValidation bool          `json:"enable_validation"`
	EnableMetrics    bool          `json:"enable_metrics"`
}

// PipelineInput represents input to a pipeline phase
type PipelineInput struct {
	Transaction *interfaces.SyncTransaction
	Changes     []interfaces.ChangeEvent
	FileStates  map[string]*models.FileState
	Metadata    map[string]interface{}
}

// PipelineOutput represents output from a pipeline phase
type PipelineOutput struct {
	ProcessedFiles   []*models.File
	FailedFiles      []*models.File
	Conflicts        []interfaces.Conflict
	BytesTransferred int64
	Metadata         map[string]interface{}
}

// NewPulsePointPipeline creates a new sync pipeline
func NewPulsePointPipeline(engine *PulsePointEngine) *PulsePointPipeline {
	logger := engine.logger.With(zap.String("component", "pipeline"))

	pipeline := &PulsePointPipeline{
		engine: engine,
		logger: logger,
		config: &PipelineConfig{
			MaxRetries:       3,
			RetryDelay:       5 * time.Second,
			Timeout:          30 * time.Minute,
			EnableValidation: true,
			EnableMetrics:    true,
		},
	}

	// Initialize pipeline phases
	pipeline.phases = []PipelinePhase{
		NewCollectionPhase(engine),
		NewAnalysisPhase(engine),
		NewExecutionPhase(engine),
		NewVerificationPhase(engine),
	}

	return pipeline
}

// Execute runs the sync pipeline
func (p *PulsePointPipeline) Execute(ctx context.Context, transaction *interfaces.SyncTransaction) (*interfaces.SyncResult, error) {
	startTime := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, p.config.Timeout)
	defer cancel()

	p.logger.Info("Starting sync pipeline",
		zap.String("transaction_id", transaction.ID),
		zap.String("type", string(transaction.Type)),
	)

	// Initialize pipeline input
	input := &PipelineInput{
		Transaction: transaction,
		Changes:     []interfaces.ChangeEvent{},
		FileStates:  make(map[string]*models.FileState),
		Metadata:    make(map[string]interface{}),
	}

	// Get changes from watcher
	// TODO: Implement GetChanges method or use a different approach
	changes := []*models.ChangeEvent{} // Placeholder for now
	for _, change := range changes {
		input.Changes = append(input.Changes, interfaces.ChangeEvent{
			Type:      interfaces.ChangeType(change.Type),
			Path:      change.Path,
			OldPath:   change.OldPath,
			Timestamp: change.Timestamp.UnixNano(),
			Size:      change.Size,
			Hash:      change.Hash,
			IsDir:     change.IsDir,
		})
	}

	// Execute each phase
	var output *PipelineOutput
	for _, phase := range p.phases {
		p.logger.Info("Executing pipeline phase",
			zap.String("phase", phase.Name()),
			zap.String("transaction_id", transaction.ID),
		)

		// Validate input if enabled
		if p.config.EnableValidation {
			if err := phase.Validate(input); err != nil {
				return nil, pperrors.NewSyncError(
					fmt.Sprintf("validation failed for phase %s", phase.Name()),
					err,
				)
			}
		}

		// Execute phase with retries
		var err error
		for retry := 0; retry <= p.config.MaxRetries; retry++ {
			output, err = phase.Execute(ctx, input)
			if err == nil {
				break
			}

			if retry < p.config.MaxRetries {
				p.logger.Warn("Phase execution failed, retrying",
					zap.String("phase", phase.Name()),
					zap.Int("retry", retry+1),
					zap.Error(err),
				)
				time.Sleep(p.config.RetryDelay)
			}
		}

		if err != nil {
			return nil, pperrors.NewSyncError(
				fmt.Sprintf("phase %s failed after %d retries", phase.Name(), p.config.MaxRetries),
				err,
			)
		}

		// Update input for next phase
		if output != nil {
			input.Metadata = output.Metadata
			// Add processed files to file states
			for _, file := range output.ProcessedFiles {
				state := models.NewFileState(file.Path)
				state.UpdateLocalInfo(file.Hash, file.ModifiedTime, file.Size)
				input.FileStates[file.Path] = state
			}
		}
	}

	// Create sync result
	result := &interfaces.SyncResult{
		StartTime:        startTime.UnixNano(),
		EndTime:          time.Now().UnixNano(),
		FilesProcessed:   len(output.ProcessedFiles),
		FilesUploaded:    0, // Will be updated by execution phase
		FilesDownloaded:  0, // Will be updated by execution phase
		FilesDeleted:     0, // Will be updated by execution phase
		FilesSkipped:     len(output.FailedFiles),
		BytesTransferred: output.BytesTransferred,
		Conflicts:        output.Conflicts,
		Success:          len(output.FailedFiles) == 0 && len(output.Conflicts) == 0,
	}

	// Update transaction
	transaction.EndTime = time.Now()
	transaction.BytesTransferred = output.BytesTransferred
	transaction.Result = result

	p.logger.Info("Sync pipeline completed",
		zap.String("transaction_id", transaction.ID),
		zap.Int("files_processed", result.FilesProcessed),
		zap.Int64("bytes_transferred", result.BytesTransferred),
		zap.Bool("success", result.Success),
		zap.Duration("duration", time.Duration(result.EndTime-result.StartTime)),
	)

	return result, nil
}

// CollectionPhase collects files that need to be synced
type CollectionPhase struct {
	engine *PulsePointEngine
	logger *zap.Logger
}

// NewCollectionPhase creates a new collection phase
func NewCollectionPhase(engine *PulsePointEngine) *CollectionPhase {
	return &CollectionPhase{
		engine: engine,
		logger: engine.logger.With(zap.String("phase", "collection")),
	}
}

// Name returns the phase name
func (p *CollectionPhase) Name() string {
	return "collection"
}

// Execute collects files for synchronization
func (p *CollectionPhase) Execute(ctx context.Context, input *PipelineInput) (*PipelineOutput, error) {
	p.logger.Info("Starting file collection",
		zap.Int("changes", len(input.Changes)),
	)

	output := &PipelineOutput{
		ProcessedFiles: []*models.File{},
		FailedFiles:    []*models.File{},
		Metadata:       make(map[string]interface{}),
	}

	// Process each change event
	for _, change := range input.Changes {
		// Skip directories for now
		if change.IsDir {
			continue
		}

		// Create file model
		file := &models.File{
			Path:         change.Path,
			Size:         change.Size,
			Hash:         change.Hash,
			ModifiedTime: time.Unix(0, change.Timestamp),
			IsFolder:     change.IsDir,
		}

		// Check if file should be ignored
		if p.shouldIgnore(file.Path) {
			p.logger.Debug("Ignoring file", zap.String("path", file.Path))
			continue
		}

		// Check file size limit
		if p.engine.config.MaxFileSize > 0 && file.Size > p.engine.config.MaxFileSize {
			p.logger.Warn("File exceeds size limit",
				zap.String("path", file.Path),
				zap.Int64("size", file.Size),
				zap.Int64("limit", p.engine.config.MaxFileSize),
			)
			output.FailedFiles = append(output.FailedFiles, file)
			continue
		}

		output.ProcessedFiles = append(output.ProcessedFiles, file)
	}

	p.logger.Info("File collection completed",
		zap.Int("collected", len(output.ProcessedFiles)),
		zap.Int("failed", len(output.FailedFiles)),
	)

	output.Metadata["collection_count"] = len(output.ProcessedFiles)
	return output, nil
}

// Validate validates the input for collection phase
func (p *CollectionPhase) Validate(input *PipelineInput) error {
	if input.Transaction == nil {
		return fmt.Errorf("transaction is required")
	}
	return nil
}

// shouldIgnore checks if a file should be ignored
func (p *CollectionPhase) shouldIgnore(path string) bool {
	// Check ignore patterns
	for _, pattern := range p.engine.config.IgnorePatterns {
		// Simple pattern matching (can be enhanced with glob patterns)
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// AnalysisPhase analyzes files and detects conflicts
type AnalysisPhase struct {
	engine *PulsePointEngine
	logger *zap.Logger
}

// NewAnalysisPhase creates a new analysis phase
func NewAnalysisPhase(engine *PulsePointEngine) *AnalysisPhase {
	return &AnalysisPhase{
		engine: engine,
		logger: engine.logger.With(zap.String("phase", "analysis")),
	}
}

// Name returns the phase name
func (p *AnalysisPhase) Name() string {
	return "analysis"
}

// Execute analyzes files for synchronization
func (p *AnalysisPhase) Execute(ctx context.Context, input *PipelineInput) (*PipelineOutput, error) {
	p.logger.Info("Starting file analysis",
		zap.Int("files", len(input.FileStates)),
	)

	output := &PipelineOutput{
		ProcessedFiles: []*models.File{},
		FailedFiles:    []*models.File{},
		Conflicts:      []interfaces.Conflict{},
		Metadata:       input.Metadata,
	}

	// Analyze each file
	for path, state := range input.FileStates {
		// Get remote metadata
		remoteMeta, err := p.engine.provider.GetMetadata(ctx, path)
		if err != nil {
			// File doesn't exist remotely, mark for upload
			state.Status = models.FileSyncStatusPending
			continue
		}

		// Check for conflicts
		if state.LocalHash != "" && remoteMeta.Hash != "" && state.LocalHash != remoteMeta.Hash {
			// Both files modified - conflict detected
			conflict := interfaces.Conflict{
				Path: path,
				Type: interfaces.ConflictTypeModified,
				LocalFile: &interfaces.File{
					Path:         path,
					Hash:         state.LocalHash,
					ModifiedTime: state.LocalModTime,
					Size:         state.LocalSize,
				},
				RemoteFile: &interfaces.File{
					Path:         remoteMeta.Path,
					Hash:         remoteMeta.Hash,
					ModifiedTime: remoteMeta.ModifiedTime,
					Size:         remoteMeta.Size,
				},
				DetectedAt: time.Now().UnixNano(),
			}
			output.Conflicts = append(output.Conflicts, conflict)
			state.SetConflict(string(interfaces.ConflictTypeModified))
		}
	}

	p.logger.Info("File analysis completed",
		zap.Int("conflicts", len(output.Conflicts)),
	)

	output.Metadata["conflicts_detected"] = len(output.Conflicts)
	return output, nil
}

// Validate validates the input for analysis phase
func (p *AnalysisPhase) Validate(input *PipelineInput) error {
	return nil
}

// ExecutionPhase executes the actual sync operations
type ExecutionPhase struct {
	engine *PulsePointEngine
	logger *zap.Logger
}

// NewExecutionPhase creates a new execution phase
func NewExecutionPhase(engine *PulsePointEngine) *ExecutionPhase {
	return &ExecutionPhase{
		engine: engine,
		logger: engine.logger.With(zap.String("phase", "execution")),
	}
}

// Name returns the phase name
func (p *ExecutionPhase) Name() string {
	return "execution"
}

// Execute performs the sync operations
func (p *ExecutionPhase) Execute(ctx context.Context, input *PipelineInput) (*PipelineOutput, error) {
	p.logger.Info("Starting sync execution")

	// Use the sync strategy to perform the sync
	result, err := p.engine.strategy.Sync(ctx,
		p.engine.config.IgnorePatterns[0], // source path (simplified for now)
		"remote://",                       // destination (simplified)
		input.Changes,
	)

	if err != nil {
		return nil, pperrors.NewSyncError("sync execution failed", err)
	}

	output := &PipelineOutput{
		ProcessedFiles:   []*models.File{},
		FailedFiles:      []*models.File{},
		Conflicts:        result.Conflicts,
		BytesTransferred: result.BytesTransferred,
		Metadata:         input.Metadata,
	}

	output.Metadata["execution_result"] = result

	p.logger.Info("Sync execution completed",
		zap.Int("uploaded", result.FilesUploaded),
		zap.Int("downloaded", result.FilesDownloaded),
		zap.Int("deleted", result.FilesDeleted),
		zap.Int64("bytes", result.BytesTransferred),
	)

	return output, nil
}

// Validate validates the input for execution phase
func (p *ExecutionPhase) Validate(input *PipelineInput) error {
	return nil
}

// VerificationPhase verifies the sync results
type VerificationPhase struct {
	engine *PulsePointEngine
	logger *zap.Logger
}

// NewVerificationPhase creates a new verification phase
func NewVerificationPhase(engine *PulsePointEngine) *VerificationPhase {
	return &VerificationPhase{
		engine: engine,
		logger: engine.logger.With(zap.String("phase", "verification")),
	}
}

// Name returns the phase name
func (p *VerificationPhase) Name() string {
	return "verification"
}

// Execute verifies the sync results
func (p *VerificationPhase) Execute(ctx context.Context, input *PipelineInput) (*PipelineOutput, error) {
	p.logger.Info("Starting sync verification")

	output := &PipelineOutput{
		ProcessedFiles: []*models.File{},
		FailedFiles:    []*models.File{},
		Metadata:       input.Metadata,
	}

	// Verify each synced file
	var wg sync.WaitGroup
	var mu sync.Mutex
	verifyErrors := 0

	for path, state := range input.FileStates {
		if state.Status != models.FileSyncStatusSynced {
			continue
		}

		wg.Add(1)
		go func(filePath string, fileState *models.FileState) {
			defer wg.Done()

			// Get remote metadata to verify
			remoteMeta, err := p.engine.provider.GetMetadata(ctx, filePath)
			if err != nil {
				p.logger.Error("Failed to verify file",
					zap.String("path", filePath),
					zap.Error(err),
				)
				mu.Lock()
				verifyErrors++
				mu.Unlock()
				return
			}

			// Verify hash matches
			if fileState.LocalHash != remoteMeta.Hash {
				p.logger.Error("Hash mismatch after sync",
					zap.String("path", filePath),
					zap.String("local_hash", fileState.LocalHash),
					zap.String("remote_hash", remoteMeta.Hash),
				)
				mu.Lock()
				verifyErrors++
				mu.Unlock()
			}
		}(path, state)
	}

	wg.Wait()

	if verifyErrors > 0 {
		return nil, pperrors.NewSyncError(
			fmt.Sprintf("verification failed for %d files", verifyErrors),
			nil,
		)
	}

	p.logger.Info("Sync verification completed successfully")
	output.Metadata["verification_passed"] = true
	return output, nil
}

// Validate validates the input for verification phase
func (p *VerificationPhase) Validate(input *PipelineInput) error {
	return nil
}
