package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	hashutil "server/internal/utils/hash"
)

type uploadResponse struct {
	TaskID      int64  `json:"task_id"`
	Status      string `json:"status"`
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	ContentHash string `json:"content_hash"`
}

type uploadResult struct {
	TaskID int64
	Status string
	File   string
	Error  error
}

type jobStatus struct {
	TaskID   int64   `json:"task_id"`
	Status   string  `json:"status"`
	Terminal bool    `json:"terminal"`
	Success  bool    `json:"success"`
	Error    *string `json:"error"`
}

type jobStatusResponse struct {
	Jobs []jobStatus `json:"jobs"`
}

func (api *apiClient) uploadDataset(ctx context.Context, token string, files []string, workers int) error {
	if workers <= 0 {
		workers = 4
	}
	results := make([]uploadResult, len(files))
	work := make(chan int)
	var completed atomic.Int64
	var group sync.WaitGroup
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range work {
				results[index] = api.uploadFile(ctx, token, files[index])
				done := completed.Add(1)
				if done%10 == 0 || int(done) == len(files) {
					fmt.Printf("upload requests completed: %d/%d\n", done, len(files))
				}
			}
		}()
	}
	for index := range files {
		select {
		case work <- index:
		case <-ctx.Done():
			close(work)
			group.Wait()
			return ctx.Err()
		}
	}
	close(work)
	group.Wait()

	taskIDs := make([]int64, 0, len(results))
	duplicates := 0
	for _, result := range results {
		if result.Error != nil {
			return fmt.Errorf("upload %s: %w", result.File, result.Error)
		}
		if result.Status == "duplicate" {
			duplicates++
		}
		if result.TaskID > 0 {
			taskIDs = append(taskIDs, result.TaskID)
		}
	}
	if err := api.waitForUploadJobs(ctx, token, taskIDs); err != nil {
		return err
	}
	fmt.Printf("upload route complete: new=%d duplicate=%d\n", len(taskIDs), duplicates)
	return nil
}

func (api *apiClient) uploadFile(ctx context.Context, token, filePath string) uploadResult {
	result := uploadResult{File: filepath.Base(filePath)}
	contentHash, err := hashutil.CalculateBLAKE3(filePath)
	if err != nil {
		result.Error = err
		return result
	}

	for attempt := 1; attempt <= 4; attempt++ {
		payload, status, uploadErr := api.sendUpload(ctx, token, filePath, contentHash)
		if uploadErr == nil && status >= 200 && status < 300 {
			result.TaskID = payload.TaskID
			result.Status = payload.Status
			return result
		}
		if status > 0 && status < 500 {
			result.Error = uploadErr
			return result
		}
		if attempt == 4 {
			result.Error = uploadErr
			return result
		}
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result
		case <-time.After(time.Duration(attempt*attempt) * 500 * time.Millisecond):
		}
	}
	return result
}

func (api *apiClient) sendUpload(ctx context.Context, token, filePath, contentHash string) (uploadResponse, int, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return uploadResponse{}, 0, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return uploadResponse{}, 0, err
	}
	_, copyErr := io.Copy(part, file)
	closeErr := file.Close()
	if copyErr != nil {
		return uploadResponse{}, 0, copyErr
	}
	if closeErr != nil {
		return uploadResponse{}, 0, closeErr
	}
	if err := writer.Close(); err != nil {
		return uploadResponse{}, 0, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, api.baseURL+"/api/v1/assets", &body)
	if err != nil {
		return uploadResponse{}, 0, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-Upload-Fingerprint", contentHash)
	response, err := api.http.Do(request)
	if err != nil {
		return uploadResponse{}, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		var payload uploadResponse
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			return uploadResponse{}, response.StatusCode, err
		}
		return payload, response.StatusCode, nil
	}
	message, _ := io.ReadAll(io.LimitReader(response.Body, 8<<10))
	return uploadResponse{}, response.StatusCode, fmt.Errorf("upload returned %s: %s", response.Status, strings.TrimSpace(string(message)))
}

func (api *apiClient) waitForUploadJobs(ctx context.Context, token string, taskIDs []int64) error {
	if len(taskIDs) == 0 {
		return nil
	}
	sort.Slice(taskIDs, func(i, j int) bool { return taskIDs[i] < taskIDs[j] })
	parts := make([]string, len(taskIDs))
	for index, id := range taskIDs {
		parts[index] = strconv.FormatInt(id, 10)
	}
	query := url.Values{"task_ids": {strings.Join(parts, ",")}}
	endpoint := "/api/v1/assets/batch/jobs?" + query.Encode()
	lastReport := time.Time{}
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, api.baseURL+endpoint, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := api.http.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		var payload jobStatusResponse
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&payload)
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("upload job status returned %s", response.Status)
		}
		if decodeErr != nil {
			return decodeErr
		}

		terminal := 0
		for _, status := range payload.Jobs {
			if status.Terminal {
				terminal++
				if !status.Success {
					return fmt.Errorf("upload ingest task %d failed (%s): %v", status.TaskID, status.Status, status.Error)
				}
			}
		}
		if terminal == len(taskIDs) && len(payload.Jobs) == len(taskIDs) {
			return nil
		}
		if time.Since(lastReport) >= 5*time.Second {
			fmt.Printf("upload ingest jobs terminal: %d/%d\n", terminal, len(taskIDs))
			lastReport = time.Now()
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for upload ingest jobs: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
