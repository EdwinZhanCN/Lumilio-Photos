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

// ErrorDetail represents a single processing error
type ErrorDetail struct {
	Task  string `json:"task"`
	Error string `json:"error"`
	Time  string `json:"time,omitempty"`
}

// AssetStatus represents the complete status information for an asset
type AssetStatus struct {
	State     AssetState    `json:"state"`
	Message   string        `json:"message"`
	Errors    []ErrorDetail `json:"errors,omitempty"`
	UpdatedAt string        `json:"updated_at"`
}

// NewProcessingStatus creates a new processing status
func NewProcessingStatus(message string) AssetStatus {
	return AssetStatus{
		State:     StateProcessing,
		Message:   message,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

// NewCompleteStatus creates a new complete status
func NewCompleteStatus() AssetStatus {
	return AssetStatus{
		State:     StateComplete,
		Message:   "Asset processed successfully",
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

// NewWarningStatus creates a new warning status with errors
func NewWarningStatus(message string, errors []ErrorDetail) AssetStatus {
	return AssetStatus{
		State:     StateWarning,
		Message:   message,
		Errors:    errors,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

// NewFailedStatus creates a new failed status with errors
func NewFailedStatus(message string, errors []ErrorDetail) AssetStatus {
	return AssetStatus{
		State:     StateFailed,
		Message:   message,
		Errors:    errors,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

// AddError adds an error to the status
func (s *AssetStatus) AddError(task, errorMsg string) {
	s.Errors = append(s.Errors, ErrorDetail{
		Task:  task,
		Error: errorMsg,
		Time:  time.Now().Format(time.RFC3339),
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
func (s AssetStatus) IsRetryable() bool {
	return s.State == StateWarning || s.State == StateFailed
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
		"metadata":       {"extract_exif", "extract_metadata"},
		"thumbnails":     {"generate_thumbnails", "save_thumbnails"},
		"transcoding":    {"transcode_video", "transcode_audio", "generate_web_version"},
		"ai_processing":  {"clip_processing"},
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
