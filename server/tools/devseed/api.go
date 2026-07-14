package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type apiClient struct {
	baseURL string
	http    *http.Client
}

type setupStatus struct {
	Initialized                  bool `json:"initialized"`
	DatabaseInitialized          bool `json:"database_initialized"`
	AdminInitialized             bool `json:"admin_initialized"`
	PrimaryRepositoryInitialized bool `json:"primary_repository_initialized"`
}

type authResponse struct {
	Token       string `json:"token"`
	RequiresMFA bool   `json:"requires_mfa"`
}

type repository struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsPrimary bool   `json:"is_primary"`
}

type createRepositoryResponse struct {
	Repository repository `json:"repository"`
}

type repositoryListResponse struct {
	Repositories []repository `json:"repositories"`
}

type queuedScan struct {
	JobID        int64  `json:"job_id"`
	RepositoryID string `json:"repository_id"`
	Status       string `json:"status"`
}

type scanRun struct {
	ScanID      string     `json:"scan_id"`
	Mode        string     `json:"mode"`
	RequestedBy *string    `json:"requested_by"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	Discovered  int64      `json:"discovered_count"`
	Updated     int64      `json:"updated_count"`
	Skipped     int64      `json:"skipped_count"`
	Error       *string    `json:"error"`
}

type scanRunList struct {
	Scans []scanRun `json:"scans"`
}

type assetQueryResponse struct {
	TotalAssets *int `json:"total_assets"`
}

type folderSummary struct {
	AssetCount int `json:"asset_count"`
}

func (api *apiClient) waitUntilReachable(ctx context.Context) (setupStatus, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		var status setupStatus
		code, err := api.json(ctx, http.MethodGet, "/api/v1/setup/status", "", nil, &status)
		if err == nil && code == http.StatusOK {
			return status, nil
		}
		if err != nil {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return setupStatus{}, fmt.Errorf("wait for Lumilio API: %w (last error: %v)", ctx.Err(), lastErr)
			}
			return setupStatus{}, fmt.Errorf("wait for Lumilio API: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (api *apiClient) bootstrap(ctx context.Context, status setupStatus, username, password string) (string, repository, error) {
	if !status.DatabaseInitialized {
		code, err := api.json(ctx, http.MethodPost, "/api/v1/setup", "", struct{}{}, nil)
		if err != nil {
			return "", repository{}, fmt.Errorf("initialize first-run database setup: %w", err)
		}
		if code != http.StatusOK {
			return "", repository{}, fmt.Errorf("initialize first-run database setup returned HTTP %d", code)
		}
		var refreshErr error
		status, refreshErr = api.setupStatus(ctx)
		if refreshErr != nil {
			return "", repository{}, refreshErr
		}
		if !status.DatabaseInitialized {
			return "", repository{}, errors.New("database setup did not reach initialized state")
		}
		fmt.Println("first-run database setup completed")
	}

	var token string
	var err error
	if status.AdminInitialized {
		token, err = api.login(ctx, username, password)
	} else {
		token, err = api.registerFirstAdmin(ctx, username, password)
	}
	if err != nil {
		return "", repository{}, err
	}

	var primary repository
	if status.PrimaryRepositoryInitialized {
		primary, err = api.primaryRepository(ctx, token)
	} else {
		primary, err = api.createPrimaryRepository(ctx, token)
	}
	if err != nil {
		return "", repository{}, err
	}
	if strings.TrimSpace(primary.ID) == "" || strings.TrimSpace(primary.Path) == "" || !primary.IsPrimary {
		return "", repository{}, errors.New("API returned an invalid Primary repository")
	}
	return token, primary, nil
}

func (api *apiClient) setupStatus(ctx context.Context) (setupStatus, error) {
	var status setupStatus
	code, err := api.json(ctx, http.MethodGet, "/api/v1/setup/status", "", nil, &status)
	if err != nil {
		return setupStatus{}, err
	}
	if code != http.StatusOK {
		return setupStatus{}, fmt.Errorf("setup status returned HTTP %d", code)
	}
	return status, nil
}

func (api *apiClient) registerFirstAdmin(ctx context.Context, username, password string) (string, error) {
	payload := map[string]string{"username": username, "password": password}
	var auth authResponse
	code, err := api.json(ctx, http.MethodPost, "/api/v1/auth/register/start", "", payload, &auth)
	if err != nil {
		return "", fmt.Errorf("register first admin: %w", err)
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("register first admin returned HTTP %d", code)
	}
	if auth.RequiresMFA || strings.TrimSpace(auth.Token) == "" {
		return "", errors.New("first admin registration did not return a usable token")
	}
	fmt.Printf("created development administrator %q\n", username)
	return auth.Token, nil
}

func (api *apiClient) login(ctx context.Context, username, password string) (string, error) {
	payload := map[string]string{"username": username, "password": password}
	var auth authResponse
	code, err := api.json(ctx, http.MethodPost, "/api/v1/auth/login", "", payload, &auth)
	if err != nil {
		return "", fmt.Errorf("login as %q: %w", username, err)
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("login returned HTTP %d", code)
	}
	if auth.RequiresMFA {
		return "", errors.New("development seed cannot complete an MFA challenge; use the fresh non-MFA admin or dev-reset")
	}
	if strings.TrimSpace(auth.Token) == "" {
		return "", errors.New("login did not return a usable token")
	}
	return auth.Token, nil
}

func (api *apiClient) createPrimaryRepository(ctx context.Context, token string) (repository, error) {
	payload := map[string]string{
		"name":               "Primary Storage",
		"role":               "primary",
		"storage_strategy":   "date",
		"duplicate_handling": "rename",
	}
	var response createRepositoryResponse
	code, err := api.json(ctx, http.MethodPost, "/api/v1/repositories", token, payload, &response)
	if err != nil {
		return repository{}, fmt.Errorf("create Primary repository: %w", err)
	}
	if code != http.StatusOK {
		return repository{}, fmt.Errorf("create Primary repository returned HTTP %d", code)
	}
	fmt.Printf("created Primary repository at %s\n", response.Repository.Path)
	return response.Repository, nil
}

func (api *apiClient) primaryRepository(ctx context.Context, token string) (repository, error) {
	var response repositoryListResponse
	code, err := api.json(ctx, http.MethodGet, "/api/v1/repositories", token, nil, &response)
	if err != nil {
		return repository{}, fmt.Errorf("list repositories: %w", err)
	}
	if code != http.StatusOK {
		return repository{}, fmt.Errorf("list repositories returned HTTP %d", code)
	}
	for _, item := range response.Repositories {
		if item.IsPrimary {
			return item, nil
		}
	}
	return repository{}, errors.New("initialized instance has no Primary repository")
}

func (api *apiClient) queueManualScan(ctx context.Context, token, repositoryID string) (queuedScan, time.Time, error) {
	queuedAt := time.Now().UTC()
	var response queuedScan
	endpoint := "/api/v1/repositories/" + url.PathEscape(repositoryID) + "/scan"
	code, err := api.json(ctx, http.MethodPost, endpoint, token, map[string]bool{"force": false}, &response)
	if err != nil {
		return queuedScan{}, queuedAt, fmt.Errorf("queue manual scan: %w", err)
	}
	if code != http.StatusOK {
		return queuedScan{}, queuedAt, fmt.Errorf("queue manual scan returned HTTP %d", code)
	}
	return response, queuedAt, nil
}

func (api *apiClient) waitForManualScan(ctx context.Context, token, repositoryID, username string, queuedAt time.Time) (scanRun, error) {
	endpoint := "/api/v1/repositories/" + url.PathEscape(repositoryID) + "/scans?limit=20&offset=0"
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	lastReport := time.Time{}
	for {
		var response scanRunList
		code, err := api.json(ctx, http.MethodGet, endpoint, token, nil, &response)
		if err == nil && code == http.StatusOK {
			sort.Slice(response.Scans, func(i, j int) bool { return response.Scans[i].StartedAt.After(response.Scans[j].StartedAt) })
			for _, run := range response.Scans {
				if run.Mode != "manual" || run.StartedAt.Before(queuedAt.Add(-2*time.Second)) {
					continue
				}
				if run.RequestedBy == nil || *run.RequestedBy != username {
					continue
				}
				switch run.Status {
				case "completed":
					return run, nil
				case "failed", "cancelled":
					message := run.Status
					if run.Error != nil {
						message += ": " + *run.Error
					}
					return run, fmt.Errorf("manual repository scan %s", message)
				}
				if time.Since(lastReport) >= 5*time.Second {
					fmt.Printf("manual scan status: %s\n", run.Status)
					lastReport = time.Now()
				}
				break
			}
		}
		select {
		case <-ctx.Done():
			return scanRun{}, fmt.Errorf("wait for manual scan: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (api *apiClient) totalAssets(ctx context.Context, token, repositoryID string) (int, error) {
	payload := map[string]any{
		"filter": map[string]any{
			"repository_id": repositoryID,
		},
		"stack_mode": "expanded",
		"pagination": map[string]int{"limit": 1, "offset": 0},
	}
	var response assetQueryResponse
	code, err := api.json(ctx, http.MethodPost, "/api/v1/assets/list", token, payload, &response)
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK || response.TotalAssets == nil {
		return 0, fmt.Errorf("asset count returned HTTP %d without total_assets", code)
	}
	return *response.TotalAssets, nil
}

func (api *apiClient) folderAssetCount(ctx context.Context, token, repositoryID, folder string) (int, error) {
	query := url.Values{"repository_id": {repositoryID}, "path": {folder}}
	endpoint := "/api/v1/assets/folders/summary?" + query.Encode()
	var response folderSummary
	code, err := api.json(ctx, http.MethodGet, endpoint, token, nil, &response)
	if err != nil {
		return 0, err
	}
	if code != http.StatusOK {
		return 0, fmt.Errorf("folder summary %q returned HTTP %d", folder, code)
	}
	return response.AssetCount, nil
}

func (api *apiClient) waitForLibrary(ctx context.Context, token, repositoryID string) error {
	folders := []string{"landing-demo", "旅行档案", "城市漫游", "生活记录", "创作练习"}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastReport := time.Time{}
	for {
		total, totalErr := api.totalAssets(ctx, token, repositoryID)
		scanTotal := 0
		var folderErr error
		if totalErr == nil {
			for _, folder := range folders {
				count, err := api.folderAssetCount(ctx, token, repositoryID, folder)
				if err != nil {
					folderErr = err
					break
				}
				scanTotal += count
			}
		}
		if totalErr == nil && folderErr == nil {
			if total > expectedScanImages+expectedUploadImages || scanTotal > expectedScanImages {
				return fmt.Errorf("seed count exceeded: total=%d scan-folders=%d", total, scanTotal)
			}
			if total == expectedScanImages+expectedUploadImages && scanTotal == expectedScanImages {
				return nil
			}
			if time.Since(lastReport) >= 5*time.Second {
				fmt.Printf("base assets materialized: total=%d/%d scan-folders=%d/%d\n", total, expectedScanImages+expectedUploadImages, scanTotal, expectedScanImages)
				lastReport = time.Now()
			}
		}
		select {
		case <-ctx.Done():
			if totalErr != nil {
				return fmt.Errorf("wait for library materialization: %w (last API error: %v)", ctx.Err(), totalErr)
			}
			if folderErr != nil {
				return fmt.Errorf("wait for library materialization: %w (last folder error: %v)", ctx.Err(), folderErr)
			}
			return fmt.Errorf("wait for library materialization: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (api *apiClient) json(ctx context.Context, method, endpoint, token string, input, output any) (int, error) {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, api.baseURL+endpoint, body)
	if err != nil {
		return 0, err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := api.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, fmt.Errorf("%s %s returned %s: %s", method, endpoint, response.Status, strings.TrimSpace(string(content)))
	}
	if output != nil && len(bytes.TrimSpace(content)) > 0 {
		if err := json.Unmarshal(content, output); err != nil {
			return response.StatusCode, fmt.Errorf("decode %s %s: %w", method, endpoint, err)
		}
	}
	return response.StatusCode, nil
}
