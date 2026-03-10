package status

import (
	"encoding/json"
	"fmt"
	"time"
)

// AssetState represents the processing state of an asset
type AssetState string

const (
	// StateProcessing indicates the asset is currently being processed
	StateProcessing AssetState = "processing"
	// StateComplete indicates the asset has been successfully processed
	StateComplete AssetState = "complete"
	// StateWarning indicates the asset was processed with some non-fatal errors
	StateWarning AssetState = "warning"
	// StateFailed indicates the asset processing failed completely
	StateFailed AssetState = "failed"
)

// TaskState represents the processing state of an individual pipeline task.
type TaskState string

const (
	TaskPending    TaskState = "pending"
	TaskProcessing TaskState = "processing"
	TaskComplete   TaskState = "complete"
	TaskFailed     TaskState = "failed"
)

// ErrorDetail represents a single processing error
type ErrorDetail struct {
	Task  string `json:"task"`
	Error string `json:"error"`
	Time  string `json:"time,omitempty"`
}

// TaskStatus represents status for a single queued processing task.
type TaskStatus struct {
	State     TaskState `json:"state"`
	Message   string    `json:"message,omitempty"`
	UpdatedAt string    `json:"updated_at"`
}

// AssetStatus represents the complete status information for an asset
type AssetStatus struct {
	State     AssetState            `json:"state"`
	Message   string                `json:"message"`
	Errors    []ErrorDetail         `json:"errors,omitempty"`
	UpdatedAt string                `json:"updated_at"`
	Tasks     map[string]TaskStatus `json:"tasks,omitempty"`
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

// NewProcessingStatus creates a new processing status
func NewProcessingStatus(message string) AssetStatus {
	return AssetStatus{
		State:     StateProcessing,
		Message:   message,
		UpdatedAt: nowRFC3339(),
	}
}

// NewCompleteStatus creates a new complete status
func NewCompleteStatus() AssetStatus {
	return AssetStatus{
		State:     StateComplete,
		Message:   "Asset processed successfully",
		UpdatedAt: nowRFC3339(),
	}
}

// NewWarningStatus creates a new warning status with errors
func NewWarningStatus(message string, errors []ErrorDetail) AssetStatus {
	return AssetStatus{
		State:     StateWarning,
		Message:   message,
		Errors:    errors,
		UpdatedAt: nowRFC3339(),
	}
}

// NewFailedStatus creates a new failed status with errors
func NewFailedStatus(message string, errors []ErrorDetail) AssetStatus {
	return AssetStatus{
		State:     StateFailed,
		Message:   message,
		Errors:    errors,
		UpdatedAt: nowRFC3339(),
	}
}

// NewTrackedProcessingStatus initializes a processing status with known pipeline tasks.
func NewTrackedProcessingStatus(message string, tasks []string) AssetStatus {
	status := NewProcessingStatus(message)
	status.EnsureTasks(tasks)
	return status
}

// AddError adds an error to the status
func (s *AssetStatus) AddError(task, errorMsg string) {
	s.upsertError(ErrorDetail{
		Task:  task,
		Error: errorMsg,
		Time:  nowRFC3339(),
	})
}

// ToJSONB converts AssetStatus to JSONB format for database storage
func (s AssetStatus) ToJSONB() ([]byte, error) {
	return json.Marshal(s)
}

// FromJSONB parses JSONB data into AssetStatus
func FromJSONB(data []byte) (AssetStatus, error) {
	var status AssetStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return AssetStatus{}, fmt.Errorf("failed to unmarshal asset status: %w", err)
	}
	return status, nil
}

// IsRetryable returns true if the asset can be retried
// Allows retry for warning, failed, and complete states (not processing)
func (s AssetStatus) IsRetryable() bool {
	return s.State == StateWarning || s.State == StateFailed || s.State == StateComplete
}

// HasFatalErrors returns true if there are fatal errors that prevent retry
func (s AssetStatus) HasFatalErrors() bool {
	// Define tasks that are considered fatal
	fatalTasks := map[string]bool{
		"initial_validation": true,
		"file_read":          true,
		"file_corrupted":     true,
	}

	for _, err := range s.Errors {
		if fatalTasks[err.Task] {
			return true
		}
	}
	return false
}

// GetFailedTasks returns a list of failed task names
func (s AssetStatus) GetFailedTasks() []string {
	tasks := make([]string, 0, len(s.Errors))
	for _, err := range s.Errors {
		tasks = append(tasks, err.Task)
	}
	return tasks
}

// GetFailedTasksByCategory returns failed tasks grouped by category
func (s AssetStatus) GetFailedTasksByCategory() map[string][]string {
	categories := map[string][]string{
		"metadata":       {"metadata_asset", "extract_exif", "extract_metadata"},
		"thumbnails":     {"thumbnail_asset", "generate_thumbnails", "save_thumbnails"},
		"transcoding":    {"transcode_asset", "transcode_video", "transcode_audio", "generate_web_version"},
		"ai_processing":  {"process_clip", "process_ocr", "process_caption", "process_face", "clip_processing"},
		"raw_processing": {"raw_processing"},
	}

	result := make(map[string][]string)
	for _, err := range s.Errors {
		for category, taskPatterns := range categories {
			for _, pattern := range taskPatterns {
				if err.Task == pattern {
					result[category] = append(result[category], err.Task)
					break
				}
			}
		}
	}
	return result
}

// ShouldRetryTask determines if a specific task should be retried
func (s AssetStatus) ShouldRetryTask(taskName string, requestedTasks []string) bool {
	// If no specific tasks requested, retry all failed tasks
	if len(requestedTasks) == 0 {
		return true
	}

	// Check if this specific task was requested
	for _, requestedTask := range requestedTasks {
		if requestedTask == taskName {
			return true
		}
	}
	return false
}

// FilterErrorsByTasks returns only the errors for the specified tasks
func (s AssetStatus) FilterErrorsByTasks(tasks []string) []ErrorDetail {
	if len(tasks) == 0 {
		return s.Errors
	}

	filtered := make([]ErrorDetail, 0)
	for _, err := range s.Errors {
		for _, task := range tasks {
			if err.Task == task {
				filtered = append(filtered, err)
				break
			}
		}
	}
	return filtered
}

// EnsureTasks initializes the tracked task map for the provided task names.
func (s *AssetStatus) EnsureTasks(tasks []string) {
	if len(tasks) == 0 {
		return
	}
	if s.Tasks == nil {
		s.Tasks = make(map[string]TaskStatus, len(tasks))
	}
	for _, task := range tasks {
		if task == "" {
			continue
		}
		if _, ok := s.Tasks[task]; ok {
			continue
		}
		s.Tasks[task] = TaskStatus{
			State:     TaskPending,
			Message:   "Queued",
			UpdatedAt: nowRFC3339(),
		}
	}
}

// MarkTaskPending marks a task as queued/pending.
func (s *AssetStatus) MarkTaskPending(taskName string, message string) {
	s.markTask(taskName, TaskPending, message)
}

// MarkTaskProcessing marks a task as currently running.
func (s *AssetStatus) MarkTaskProcessing(taskName string, message string) {
	s.markTask(taskName, TaskProcessing, message)
}

// MarkTaskComplete marks a task as complete and clears any stale error for it.
func (s *AssetStatus) MarkTaskComplete(taskName string, message string) {
	s.removeError(taskName)
	s.markTask(taskName, TaskComplete, message)
}

// MarkTaskFailed marks a task as failed and records the error detail.
func (s *AssetStatus) MarkTaskFailed(taskName string, message string, errorMsg string) {
	s.upsertError(ErrorDetail{
		Task:  taskName,
		Error: errorMsg,
		Time:  nowRFC3339(),
	})
	s.markTask(taskName, TaskFailed, message)
}

// RefreshSummary recomputes top-level state/message from tracked task statuses.
func (s *AssetStatus) RefreshSummary() {
	if len(s.Tasks) == 0 {
		if s.State == "" {
			s.State = StateProcessing
		}
		if s.Message == "" {
			s.Message = "Asset processing in progress"
		}
		s.UpdatedAt = nowRFC3339()
		return
	}

	var (
		total           = len(s.Tasks)
		pendingCount    int
		processingCount int
		completeCount   int
		failedCount     int
		processingTask  string
		processingMsg   string
	)

	for taskName, taskStatus := range s.Tasks {
		switch taskStatus.State {
		case TaskComplete:
			completeCount++
		case TaskFailed:
			failedCount++
		case TaskProcessing:
			processingCount++
			processingTask = taskName
			processingMsg = taskStatus.Message
		default:
			pendingCount++
		}
	}

	switch {
	case processingCount > 0:
		s.State = StateProcessing
		if processingCount == 1 && processingMsg != "" {
			s.Message = processingMsg
		} else if processingCount == 1 {
			s.Message = fmt.Sprintf("%s is in progress", processingTask)
		} else {
			s.Message = fmt.Sprintf("Processing %d of %d tasks", completeCount+failedCount+processingCount, total)
		}
	case pendingCount > 0:
		s.State = StateProcessing
		if failedCount > 0 {
			s.Message = fmt.Sprintf("Processing %d of %d tasks (%d failed)", completeCount+failedCount, total, failedCount)
		} else {
			s.Message = fmt.Sprintf("Processing %d of %d tasks", completeCount, total)
		}
	case failedCount == total:
		s.State = StateFailed
		s.Message = "Asset processing failed"
	case failedCount > 0:
		s.State = StateWarning
		s.Message = fmt.Sprintf("Asset processed with %d failed task(s)", failedCount)
	default:
		s.State = StateComplete
		s.Message = "Asset processed successfully"
	}

	s.UpdatedAt = nowRFC3339()
}

func (s *AssetStatus) markTask(taskName string, state TaskState, message string) {
	if taskName == "" {
		return
	}
	s.EnsureTasks([]string{taskName})
	taskStatus := s.Tasks[taskName]
	taskStatus.State = state
	if message != "" {
		taskStatus.Message = message
	}
	taskStatus.UpdatedAt = nowRFC3339()
	s.Tasks[taskName] = taskStatus
	s.RefreshSummary()
}

func (s *AssetStatus) upsertError(detail ErrorDetail) {
	for i := range s.Errors {
		if s.Errors[i].Task == detail.Task {
			s.Errors[i] = detail
			return
		}
	}
	s.Errors = append(s.Errors, detail)
}

func (s *AssetStatus) removeError(taskName string) {
	if len(s.Errors) == 0 {
		return
	}

	filtered := s.Errors[:0]
	for _, detail := range s.Errors {
		if detail.Task != taskName {
			filtered = append(filtered, detail)
		}
	}
	if len(filtered) == 0 {
		s.Errors = nil
		return
	}
	s.Errors = filtered
}
