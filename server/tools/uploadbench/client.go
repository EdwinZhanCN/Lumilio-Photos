package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// client is a thin authenticated HTTP client for the Lumilio API.
type client struct {
	base  string
	token string
	http  *http.Client
}

func newClient(base string) *client {
	return &client{
		base: strings.TrimRight(base, "/"),
		http: &http.Client{
			// Uploads of large RAW files can take a while; keep a generous
			// per-request ceiling but rely on ctx for run-level cancellation.
			Timeout: 10 * time.Minute,
		},
	}
}

func (c *client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}

// getJSON performs a GET and decodes a JSON response into out.
func (c *client) getJSON(ctx context.Context, path string, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return err
	}
	return decodeInto(resp, out)
}

// sendJSON marshals in, sends it with the given method, and decodes into out
// (out may be nil to discard the body).
func (c *client) sendJSON(ctx context.Context, method, path string, in, out any) error {
	buf, err := json.Marshal(in)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, method, path, bytes.NewReader(buf), "application/json")
	if err != nil {
		return err
	}
	return decodeInto(resp, out)
}

func decodeInto(resp *http.Response, out any) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d: %s", resp.Request.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// --- Auth -----------------------------------------------------------------

type authResponse struct {
	Token       string `json:"token"`
	RequiresMFA bool   `json:"requires_mfa"`
}

func (c *client) login(ctx context.Context, user, pass string) error {
	var resp authResponse
	if err := c.sendJSON(ctx, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"username": user, "password": pass}, &resp); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if resp.RequiresMFA {
		return fmt.Errorf("login: benchmark user requires MFA; use a non-MFA benchmark account")
	}
	if resp.Token == "" {
		return fmt.Errorf("login: empty token")
	}
	c.token = resp.Token
	return nil
}

// --- Repositories ---------------------------------------------------------

type repositoryDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsPrimary bool   `json:"is_primary"`
}

type listRepositoriesResponse struct {
	Repositories []repositoryDTO `json:"repositories"`
}

// primaryRepository returns the primary repository (falling back to the first).
func (c *client) primaryRepository(ctx context.Context) (repositoryDTO, error) {
	var resp listRepositoriesResponse
	if err := c.getJSON(ctx, "/api/v1/repositories", &resp); err != nil {
		return repositoryDTO{}, fmt.Errorf("list repositories: %w", err)
	}
	if len(resp.Repositories) == 0 {
		return repositoryDTO{}, fmt.Errorf("no repositories exist; run first-time setup before benchmarking")
	}
	for _, r := range resp.Repositories {
		if r.IsPrimary {
			return r, nil
		}
	}
	return resp.Repositories[0], nil
}

// --- Settings (ML toggle) -------------------------------------------------

type mlSettings struct {
	SemanticEnabled bool `json:"semantic_enabled"`
	BioCLIPEnabled  bool `json:"bioclip_enabled"`
	OCREnabled      bool `json:"ocr_enabled"`
	FaceEnabled     bool `json:"face_enabled"`
}

type systemSettings struct {
	ML mlSettings `json:"ml"`
}

func (c *client) systemSettings(ctx context.Context) (systemSettings, error) {
	var s systemSettings
	err := c.getJSON(ctx, "/api/v1/settings/system", &s)
	return s, err
}

// disableML turns off all ML task toggles and returns the resulting state.
func (c *client) disableML(ctx context.Context) (systemSettings, error) {
	off := false
	patch := map[string]any{
		"ml": map[string]*bool{
			"semantic_enabled": &off,
			"bioclip_enabled":  &off,
			"ocr_enabled":      &off,
			"face_enabled":     &off,
		},
	}
	var s systemSettings
	if err := c.sendJSON(ctx, http.MethodPatch, "/api/v1/settings/system", patch, &s); err != nil {
		return s, fmt.Errorf("disable ML: %w", err)
	}
	return s, nil
}

// --- Queue observability --------------------------------------------------

type jobStats struct {
	Available int64 `json:"available"`
	Scheduled int64 `json:"scheduled"`
	Running   int64 `json:"running"`
	Retryable int64 `json:"retryable"`
	Completed int64 `json:"completed"`
	Cancelled int64 `json:"cancelled"`
	Discarded int64 `json:"discarded"`
}

// pending returns the number of not-yet-finalized jobs.
func (s jobStats) pending() int64 { return s.Available + s.Scheduled + s.Running + s.Retryable }

func (c *client) jobStats(ctx context.Context) (jobStats, error) {
	var s jobStats
	err := c.getJSON(ctx, "/api/v1/admin/river/stats", &s)
	return s, err
}

type queueSummary struct {
	Name             string `json:"name"`
	TotalJobs        int64  `json:"total_jobs"`
	ProcessedJobs    int64  `json:"processed_jobs"`
	RemainingJobs    int64  `json:"remaining_jobs"`
	RunningJobs      int64  `json:"running_jobs"`
	AttentionJobs    int64  `json:"attention_jobs"`
	AverageLatencyMs *int64 `json:"average_latency_ms,omitempty"`
	AverageRuntimeMs *int64 `json:"average_runtime_ms,omitempty"`
}

type queueSummaryResponse struct {
	Queues []queueSummary `json:"queues"`
}

func (c *client) queueSummary(ctx context.Context) ([]queueSummary, error) {
	var r queueSummaryResponse
	err := c.getJSON(ctx, "/api/v1/admin/river/queue-summary", &r)
	return r.Queues, err
}

// --- Asset listing (completion polling) -----------------------------------

type apiAsset struct {
	AssetID          string  `json:"asset_id"`
	OriginalFilename string  `json:"original_filename"`
	Hash             *string `json:"hash"`
	FileSize         int64   `json:"file_size"`
	Type             string  `json:"type"`
	// Status is JSONB; gin encodes []byte as base64, and Go's json decoder
	// transparently base64-decodes it back into the raw JSON bytes here.
	Status []byte `json:"status"`
}

type browseItem struct {
	Type  string    `json:"type"`
	Asset *apiAsset `json:"asset,omitempty"`
}

type queryAssetsResponse struct {
	Items       []browseItem `json:"items"`
	TotalAssets *int         `json:"total_assets,omitempty"`
}

// listAssets pages through every asset (stack_mode=expanded so stacks do not
// collapse RAW+JPEG pairs into a single representative) and returns them.
func (c *client) listAssets(ctx context.Context) ([]*apiAsset, error) {
	const pageSize = 100
	var out []*apiAsset
	for offset := 0; ; offset += pageSize {
		body := map[string]any{
			"stack_mode": "expanded",
			"sort_by":    "recently_added",
			"pagination": map[string]int{"limit": pageSize, "offset": offset},
		}
		var resp queryAssetsResponse
		if err := c.sendJSON(ctx, http.MethodPost, "/api/v1/assets/list", body, &resp); err != nil {
			return nil, fmt.Errorf("query assets (offset %d): %w", offset, err)
		}
		for _, it := range resp.Items {
			if it.Asset != nil {
				out = append(out, it.Asset)
			}
		}
		if len(resp.Items) < pageSize {
			break
		}
	}
	return out, nil
}

// --- Upload ---------------------------------------------------------------

type uploadResponse struct {
	TaskID      int64  `json:"task_id"`
	Status      string `json:"status"`
	FileName    string `json:"file_name"`
	ContentHash string `json:"content_hash"`
	StatusCode  int    `json:"-"` // HTTP status, carried for reporting
}

// uploadFile streams one file to POST /api/v1/assets. When repositoryID is set
// it is sent as a form field (which also triggers detect_stacks server-side, so
// the core profile leaves it empty). When clientHash is non-empty it is sent as
// X-Upload-Fingerprint as a non-authoritative precheck hint.
func (c *client) uploadFile(ctx context.Context, path, clientHash, contentType, repositoryID string) (uploadResponse, error) {
	f, err := os.Open(path)
	if err != nil {
		return uploadResponse{}, err
	}
	defer f.Close()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var werr error
		defer func() { _ = pw.CloseWithError(werr) }()
		if repositoryID != "" {
			if werr = mw.WriteField("repository_id", repositoryID); werr != nil {
				return
			}
		}
		part, err := createFilePart(mw, filepath.Base(path), contentType)
		if err != nil {
			werr = err
			return
		}
		if _, werr = io.Copy(part, f); werr != nil {
			return
		}
		werr = mw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/api/v1/assets", pr)
	if err != nil {
		return uploadResponse{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.token)
	if clientHash != "" {
		req.Header.Set("X-Upload-Fingerprint", clientHash)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return uploadResponse{}, err
	}
	var out uploadResponse
	out.StatusCode = resp.StatusCode
	if err := decodeInto(resp, &out); err != nil {
		return out, err
	}
	return out, nil
}

// createFilePart writes a form-data part for the "file" field with an explicit
// content type (multipart.CreateFormFile always uses octet-stream).
func createFilePart(mw *multipart.Writer, filename, contentType string) (io.Writer, error) {
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="file"; filename=%q`, escapeQuotes(filename))}
	if contentType != "" {
		h["Content-Type"] = []string{contentType}
	}
	return mw.CreatePart(h)
}

func escapeQuotes(s string) string { return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(s) }

// health probes the readiness endpoint and returns an error if not 2xx.
func (c *client) health(ctx context.Context) error {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/health", nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health: HTTP %d", resp.StatusCode)
	}
	return nil
}
