package queue

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type TaskType string

const (
	TaskTypeUpload  TaskType = "UPLOAD"
	TaskTypeScan    TaskType = "SCAN"
	TaskTypeProcess TaskType = "PROCESS"
	TaskTypeIndex   TaskType = "INDEX"
)

// Task represents a processing job that needs to be performed
type Task struct {
	TaskID      string    `json:"taskId"`
	Type        string    `json:"type"`
	ClientHash  string    `json:"clientHash"`
	StagedPath  string    `json:"stagedPath"`
	UserID      string    `json:"userId"`
	Timestamp   time.Time `json:"timestamp"`
	ContentType string    `json:"contentType,omitempty"`
	FileName    string    `json:"fileName,omitempty"`
}

// TaskQueue is a file-based persistent task queue
type TaskQueue struct {
	queueDir    string
	walFile     string
	doneFile    string
	tasks       chan Task
	mutex       sync.Mutex
	bufferSize  int
	initialized bool
}

// NewTaskQueue creates a new file-based task queue
func NewTaskQueue(queueDir string, bufferSize int) (*TaskQueue, error) {
	// Ensure queue directory exists
	if err := os.MkdirAll(queueDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create queue directory: %w", err)
	}

	walFile := filepath.Join(queueDir, "tasks.wal")
	doneFile := filepath.Join(queueDir, "tasks.done")

	return &TaskQueue{
		queueDir:   queueDir,
		walFile:    walFile,
		doneFile:   doneFile,
		tasks:      make(chan Task, bufferSize),
		bufferSize: bufferSize,
	}, nil
}

// Initialize loads existing tasks from WAL and starts watching for new tasks
func (q *TaskQueue) Initialize() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.initialized {
		return nil
	}

	// Create WAL file if it doesn't exist
	if _, err := os.Stat(q.walFile); os.IsNotExist(err) {
		file, err := os.Create(q.walFile)
		if err != nil {
			return fmt.Errorf("failed to create WAL file: %w", err)
		}
		file.Close()
	}

	// Create done file if it doesn't exist
	if _, err := os.Stat(q.doneFile); os.IsNotExist(err) {
		file, err := os.Create(q.doneFile)
		if err != nil {
			return fmt.Errorf("failed to create done file: %w", err)
		}
		file.Close()
	}

	// Load completed task IDs
	completedTasks, err := q.loadCompletedTaskIDs()
	if err != nil {
		return fmt.Errorf("failed to load completed tasks: %w", err)
	}

	// Load and enqueue existing tasks
	if err := q.loadExistingTasks(completedTasks); err != nil {
		return fmt.Errorf("failed to load existing tasks: %w", err)
	}

	// Start watching for new tasks
	go q.watchForNewTasks(completedTasks)

	q.initialized = true
	return nil
}

// EnqueueTask adds a new task to the queue
func (q *TaskQueue) EnqueueTask(task Task) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Set timestamp if not already set
	if task.Timestamp.IsZero() {
		task.Timestamp = time.Now()
	}

	// Marshal task to JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Append task to WAL
	file, err := os.OpenFile(q.walFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(string(taskJSON) + "\n"); err != nil {
		return fmt.Errorf("failed to write task to WAL: %w", err)
	}

	return nil
}

// GetTask retrieves the next task from the queue
func (q *TaskQueue) GetTask() (Task, bool) {
	task, ok := <-q.tasks
	return task, ok
}

// MarkTaskComplete marks a task as completed
func (q *TaskQueue) MarkTaskComplete(taskID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Append task ID to done file
	file, err := os.OpenFile(q.doneFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open done file: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(taskID + "\n"); err != nil {
		return fmt.Errorf("failed to write task ID to done file: %w", err)
	}

	return nil
}

// CleanupProcessedTasks removes completed tasks from the WAL
// This should be called periodically (e.g., daily)
func (q *TaskQueue) CleanupProcessedTasks() error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Load completed task IDs
	completedTasks, err := q.loadCompletedTaskIDs()
	if err != nil {
		return fmt.Errorf("failed to load completed tasks: %w", err)
	}

	// No completed tasks to clean up
	if len(completedTasks) == 0 {
		return nil
	}

	// Read all tasks from WAL
	walFile, err := os.Open(q.walFile)
	if err != nil {
		return fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer walFile.Close()

	// Create a temporary file for the new WAL
	tempWAL := q.walFile + ".new"
	tempFile, err := os.Create(tempWAL)
	if err != nil {
		return fmt.Errorf("failed to create temp WAL file: %w", err)
	}
	defer tempFile.Close()

	// Copy only incomplete tasks to the new WAL
	scanner := bufio.NewScanner(walFile)
	for scanner.Scan() {
		line := scanner.Text()
		var task Task
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			// Skip invalid lines
			continue
		}

		// If task is not completed, copy it to the new WAL
		if !completedTasks[task.TaskID] {
			if _, err := tempFile.WriteString(line + "\n"); err != nil {
				return fmt.Errorf("failed to write to temp WAL: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading WAL: %w", err)
	}

	// Close files before renaming
	walFile.Close()
	tempFile.Close()

	// Replace old WAL with new one
	if err := os.Rename(tempWAL, q.walFile); err != nil {
		return fmt.Errorf("failed to replace WAL: %w", err)
	}

	// Truncate the done file since we've processed all completed tasks
	if err := os.Truncate(q.doneFile, 0); err != nil {
		return fmt.Errorf("failed to truncate done file: %w", err)
	}

	return nil
}

// loadCompletedTaskIDs loads the set of completed task IDs
func (q *TaskQueue) loadCompletedTaskIDs() (map[string]bool, error) {
	completed := make(map[string]bool)

	file, err := os.Open(q.doneFile)
	if err != nil {
		if os.IsNotExist(err) {
			return completed, nil
		}
		return nil, fmt.Errorf("failed to open done file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		taskID := scanner.Text()
		completed[taskID] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading done file: %w", err)
	}

	return completed, nil
}

// loadExistingTasks loads tasks from the WAL that haven't been completed
func (q *TaskQueue) loadExistingTasks(completedTasks map[string]bool) error {
	file, err := os.Open(q.walFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open WAL file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		var task Task
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			// Log and skip invalid lines
			fmt.Printf("Error unmarshaling task: %v\n", err)
			continue
		}

		// Skip completed tasks
		if completedTasks[task.TaskID] {
			continue
		}

		// Try to enqueue the task, but don't block if channel is full
		select {
		case q.tasks <- task:
			// Task enqueued
		default:
			// Channel is full, log and continue
			fmt.Printf("Warning: Task queue channel is full, skipping task %s\n", task.TaskID)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading WAL file: %w", err)
	}

	return nil
}

// watchForNewTasks continuously watches the WAL file for new tasks
func (q *TaskQueue) watchForNewTasks(completedTasks map[string]bool) {
	lastSize, _ := q.getFileSize(q.walFile)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		currentSize, err := q.getFileSize(q.walFile)
		if err != nil {
			fmt.Printf("Error getting WAL file size: %v\n", err)
			continue
		}

		// If file has grown, check for new tasks
		if currentSize > lastSize {
			file, err := os.Open(q.walFile)
			if err != nil {
				fmt.Printf("Error opening WAL file: %v\n", err)
				continue
			}

			// Seek to the last read position
			_, err = file.Seek(lastSize, 0)
			if err != nil {
				fmt.Printf("Error seeking in WAL file: %v\n", err)
				file.Close()
				continue
			}

			// Read new lines
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				var task Task
				if err := json.Unmarshal([]byte(line), &task); err != nil {
					fmt.Printf("Error unmarshaling task: %v\n", err)
					continue
				}

				// Skip completed tasks
				if completedTasks[task.TaskID] {
					continue
				}

				// Try to enqueue the task
				select {
				case q.tasks <- task:
					// Task enqueued
				default:
					// Channel is full, log and continue
					fmt.Printf("Warning: Task queue channel is full, skipping task %s\n", task.TaskID)
				}
			}

			file.Close()
			lastSize = currentSize
		}

		// Periodically refresh the completed tasks list
		newCompletedTasks, err := q.loadCompletedTaskIDs()
		if err != nil {
			fmt.Printf("Error loading completed tasks: %v\n", err)
		} else {
			completedTasks = newCompletedTasks
		}
	}
}

// getFileSize returns the size of a file in bytes
func (q *TaskQueue) getFileSize(path string) (int64, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}

// Close closes the task queue
func (q *TaskQueue) Close() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.initialized {
		close(q.tasks)
		q.initialized = false
	}
}
