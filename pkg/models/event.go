package models

import (
	"fmt"
	"time"
)

// ChangeEvent represents a file system change event
type ChangeEvent struct {
	// Event identification
	ID     string     `json:"id" bolt:"id"`
	Type   ChangeType `json:"type" bolt:"type"`
	Source string     `json:"source" bolt:"source"` // local, remote, manual

	// File information
	Path    string `json:"path" bolt:"path"`
	OldPath string `json:"old_path,omitempty" bolt:"old_path"` // For rename/move operations

	// Event details
	Timestamp time.Time `json:"timestamp" bolt:"timestamp"`
	Size      int64     `json:"size,omitempty" bolt:"size"`
	OldSize   int64     `json:"old_size,omitempty" bolt:"old_size"`
	Hash      string    `json:"hash,omitempty" bolt:"hash"`
	OldHash   string    `json:"old_hash,omitempty" bolt:"old_hash"`

	// File properties
	IsDir       bool   `json:"is_dir" bolt:"is_dir"`
	MimeType    string `json:"mime_type,omitempty" bolt:"mime_type"`
	Permissions string `json:"permissions,omitempty" bolt:"permissions"`

	// Processing status
	Status    EventStatus `json:"status" bolt:"status"`
	Processed bool        `json:"processed" bolt:"processed"`
	Retries   int         `json:"retries" bolt:"retries"`
	Error     string      `json:"error,omitempty" bolt:"error"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty" bolt:"metadata"`
}

// NewChangeEvent creates a new change event
func NewChangeEvent(eventType ChangeType, path string) *ChangeEvent {
	return &ChangeEvent{
		ID:        GenerateEventID(),
		Type:      eventType,
		Path:      path,
		Source:    "local",
		Timestamp: time.Now(),
		Status:    EventStatusPending,
		Metadata:  make(map[string]interface{}),
	}
}

// GenerateEventID generates a unique event ID
func GenerateEventID() string {
	// Implementation will use UUID or timestamp-based ID
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}

// ChangeType defines the type of file system change
type ChangeType string

const (
	// ChangeTypeCreate indicates a file or directory was created
	ChangeTypeCreate ChangeType = "create"

	// ChangeTypeModify indicates a file was modified
	ChangeTypeModify ChangeType = "modify"

	// ChangeTypeDelete indicates a file or directory was deleted
	ChangeTypeDelete ChangeType = "delete"

	// ChangeTypeRename indicates a file or directory was renamed
	ChangeTypeRename ChangeType = "rename"

	// ChangeTypeMove indicates a file or directory was moved
	ChangeTypeMove ChangeType = "move"

	// ChangeTypeChmod indicates file permissions changed
	ChangeTypeChmod ChangeType = "chmod"

	// ChangeTypeTouch indicates file was touched (timestamp changed)
	ChangeTypeTouch ChangeType = "touch"

	// ChangeTypeTruncate indicates file was truncated
	ChangeTypeTruncate ChangeType = "truncate"
)

// EventStatus represents the processing status of an event
type EventStatus string

const (
	// EventStatusPending event is pending processing
	EventStatusPending EventStatus = "pending"

	// EventStatusProcessing event is being processed
	EventStatusProcessing EventStatus = "processing"

	// EventStatusCompleted event processing completed
	EventStatusCompleted EventStatus = "completed"

	// EventStatusFailed event processing failed
	EventStatusFailed EventStatus = "failed"

	// EventStatusSkipped event was skipped
	EventStatusSkipped EventStatus = "skipped"

	// EventStatusDeferred event processing was deferred
	EventStatusDeferred EventStatus = "deferred"
)

// String returns the string representation of the change type
func (ct ChangeType) String() string {
	return string(ct)
}

// IsCreateOrModify checks if the event is a create or modify operation
func (e *ChangeEvent) IsCreateOrModify() bool {
	return e.Type == ChangeTypeCreate || e.Type == ChangeTypeModify
}

// IsDelete checks if the event is a delete operation
func (e *ChangeEvent) IsDelete() bool {
	return e.Type == ChangeTypeDelete
}

// IsRenameOrMove checks if the event is a rename or move operation
func (e *ChangeEvent) IsRenameOrMove() bool {
	return e.Type == ChangeTypeRename || e.Type == ChangeTypeMove
}

// SetError sets an error on the event
func (e *ChangeEvent) SetError(err error) {
	e.Status = EventStatusFailed
	e.Error = err.Error()
	e.Retries++
}

// MarkProcessed marks the event as processed
func (e *ChangeEvent) MarkProcessed() {
	e.Status = EventStatusCompleted
	e.Processed = true
}

// CanRetry checks if the event can be retried
func (e *ChangeEvent) CanRetry(maxRetries int) bool {
	return e.Retries < maxRetries && e.Status == EventStatusFailed
}

// EventBatch represents a batch of events for processing
type EventBatch struct {
	ID        string         `json:"id"`
	Events    []*ChangeEvent `json:"events"`
	CreatedAt time.Time      `json:"created_at"`
	Size      int            `json:"size"`
	Priority  int            `json:"priority"`
}

// NewEventBatch creates a new event batch
func NewEventBatch(events []*ChangeEvent) *EventBatch {
	return &EventBatch{
		ID:        GenerateBatchID(),
		Events:    events,
		CreatedAt: time.Now(),
		Size:      len(events),
		Priority:  0,
	}
}

// GenerateBatchID generates a unique batch ID
func GenerateBatchID() string {
	return fmt.Sprintf("batch_%d", time.Now().UnixNano())
}

// AddEvent adds an event to the batch
func (b *EventBatch) AddEvent(event *ChangeEvent) {
	b.Events = append(b.Events, event)
	b.Size = len(b.Events)
}

// EventQueue represents a queue of events for processing
type EventQueue struct {
	Events     []*ChangeEvent          `json:"events"`
	Processing map[string]*ChangeEvent `json:"processing"`
	Completed  []*ChangeEvent          `json:"completed"`
	Failed     []*ChangeEvent          `json:"failed"`
	MaxSize    int                     `json:"max_size"`
	CreatedAt  time.Time               `json:"created_at"`
}

// NewEventQueue creates a new event queue
func NewEventQueue(maxSize int) *EventQueue {
	return &EventQueue{
		Events:     make([]*ChangeEvent, 0),
		Processing: make(map[string]*ChangeEvent),
		Completed:  make([]*ChangeEvent, 0),
		Failed:     make([]*ChangeEvent, 0),
		MaxSize:    maxSize,
		CreatedAt:  time.Now(),
	}
}

// Push adds an event to the queue
func (q *EventQueue) Push(event *ChangeEvent) error {
	if len(q.Events) >= q.MaxSize {
		return fmt.Errorf("queue is full (max size: %d)", q.MaxSize)
	}
	q.Events = append(q.Events, event)
	return nil
}

// Pop removes and returns the first event from the queue
func (q *EventQueue) Pop() *ChangeEvent {
	if len(q.Events) == 0 {
		return nil
	}
	event := q.Events[0]
	q.Events = q.Events[1:]
	q.Processing[event.ID] = event
	return event
}

// MarkCompleted marks an event as completed
func (q *EventQueue) MarkCompleted(eventID string) {
	if event, ok := q.Processing[eventID]; ok {
		delete(q.Processing, eventID)
		event.MarkProcessed()
		q.Completed = append(q.Completed, event)
	}
}

// MarkFailed marks an event as failed
func (q *EventQueue) MarkFailed(eventID string, err error) {
	if event, ok := q.Processing[eventID]; ok {
		delete(q.Processing, eventID)
		event.SetError(err)
		q.Failed = append(q.Failed, event)
	}
}

// Size returns the number of pending events
func (q *EventQueue) Size() int {
	return len(q.Events)
}

// IsEmpty checks if the queue is empty
func (q *EventQueue) IsEmpty() bool {
	return len(q.Events) == 0
}
