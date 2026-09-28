package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"server/config"
)

// blackboxServer is one complete in-process Server generation driven only
// through its public HTTP API and the repository directory on disk, the way
// the Desktop host embeds it. The *_regression_test.go files in this package
// are the Phase 0 tests of
// docs/exec-plans/active/repository-index-and-asset-lifecycle.md (#222,
// #223); going through this seam keeps their assertions valid across
// rewrites of the scan and catalog internals.
type blackboxServer struct {
	t       testing.TB
	baseURL string
	token   string
	client  *http.Client
	primary blackboxRepository
}

type blackboxRepository struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type blackboxAsset struct {
	AssetID          string `json:"asset_id"`
	ContentID        string `json:"content_id"`
	OriginalFilename string `json:"original_filename"`
}

// Every status either scan implementation reports while a run is still
// active. Anything else is terminal.
var blackboxActiveScanStatuses = map[string]bool{
	"queued": true, "crawling": true, "catching_up": true, "finalizing": true,
	"walking": true, "sweeping": true,
}

const blackboxPollInterval = 100 * time.Millisecond

func startBlackboxServer(tb testing.TB) *blackboxServer {
	tb.Helper()
	root := tb.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		tb.Fatal(err)
	}
	// SQLite refuses a catalog directory readable by group or others.
	if err := os.Mkdir(filepath.Join(root, "state"), 0o700); err != nil {
		tb.Fatal(err)
	}
	listen := blackboxFreeListen(tb)
	profile, ok := config.ProfileByName(config.ProfileDevVite)
	if !ok {
		tb.Fatal("dev-vite profile is missing")
	}
	manifest, err := config.EncodeProfile(profile, config.ProfileInputs{
		StateDir:   filepath.ToSlash(filepath.Join(root, "state")),
		StorageDir: filepath.ToSlash(filepath.Join(root, "storage")),
	}, "")
	if err != nil {
		tb.Fatal(err)
	}
	// The regression tests need no inference, and a one-second settle window
	// keeps a freshly written file from being deferred for long.
	manifest = blackboxSetKey(tb, manifest, "listen", fmt.Sprintf("%q", listen))
	manifest = blackboxSetKey(tb, manifest, "discovery_enabled", "false")
	manifest = blackboxSetKey(tb, manifest, "discovery_mdns_enabled", "false")
	manifest = blackboxSetKey(tb, manifest, "settle_seconds", "1")
	manifest = blackboxSetKey(tb, manifest, "level", `"error"`)
	manifestPath := filepath.Join(root, "config", "server.toml")
	appConfig, err := config.LoadAppConfigBytes(manifestPath, manifest)
	if err != nil {
		tb.Fatal(err)
	}

	gin.SetMode(gin.ReleaseMode)
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan RuntimeInfo, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, appConfig, OperatorControls{
			RuntimeReady: func(info RuntimeInfo) { ready <- info },
		})
	}()
	tb.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				tb.Errorf("server shutdown: %v", err)
			}
		case <-time.After(90 * time.Second):
			tb.Error("server did not shut down within 90s")
		}
	})
	select {
	case <-ready:
	case err := <-done:
		done <- err // let the cleanup observe the exit too
		tb.Fatalf("server exited before it was ready: %v", err)
	case <-time.After(90 * time.Second):
		tb.Fatal("server was not ready within 90s")
	}

	server := &blackboxServer{
		t: tb, baseURL: "http://" + listen,
		client: &http.Client{Timeout: 60 * time.Second},
	}
	var registered struct {
		Token string `json:"token"`
	}
	server.mustJSON(http.MethodPost, "/api/v1/auth/register/start", map[string]any{
		"username": "blackbox-admin", "password": "Lumilio-Blackbox-2026!",
	}, &registered)
	if registered.Token == "" {
		tb.Fatal("first-admin registration returned no session token")
	}
	server.token = registered.Token
	var created struct {
		Repository blackboxRepository `json:"repository"`
	}
	server.mustJSON(http.MethodPost, "/api/v1/setup/primary-repository", map[string]any{
		// The fixture's temporary directory may sit on a volume the storage
		// path policy flags as risky; confirming it is the operator's choice.
		"name": "Blackbox Primary", "storage_strategy": "flat", "risk_confirmation": true,
	}, &created)
	if created.Repository.ID == "" || created.Repository.Path == "" {
		tb.Fatalf("primary repository = %+v", created.Repository)
	}
	server.primary = created.Repository
	server.waitIdle(created.Repository)
	return server
}

// waitNoActiveAssets waits until the Storage page counts no active Asset in a
// repository, which is how an operator sees that its files are missing.
func (s *blackboxServer) waitNoActiveAssets(repository blackboxRepository) {
	s.t.Helper()
	count := int64(-1)
	gone := waitFor(time.Minute, func() bool {
		var view struct {
			Repositories []struct {
				ID         string `json:"id"`
				AssetCount int64  `json:"asset_count"`
			} `json:"repositories"`
		}
		s.mustJSON(http.MethodGet, "/api/v1/storage/view", nil, &view)
		for _, candidate := range view.Repositories {
			if candidate.ID == repository.ID {
				count = candidate.AssetCount
			}
		}
		return count == 0
	})
	if !gone {
		s.t.Fatalf("repository %s still counts %d active Assets after its files were deleted", repository.Name, count)
	}
}

// waitIdle waits until a repository's lifecycle activity is idle, which
// upload admission requires.
func (s *blackboxServer) waitIdle(repository blackboxRepository) {
	s.t.Helper()
	activity := ""
	idle := waitFor(90*time.Second, func() bool {
		var view struct {
			Repositories []struct {
				ID       string `json:"id"`
				Activity string `json:"activity"`
			} `json:"repositories"`
		}
		s.mustJSON(http.MethodGet, "/api/v1/storage/view", nil, &view)
		for _, candidate := range view.Repositories {
			if candidate.ID == repository.ID {
				activity = candidate.Activity
			}
		}
		return activity == "idle"
	})
	if !idle {
		s.t.Fatalf("repository %s activity is %q, want idle", repository.Name, activity)
	}
}

func blackboxFreeListen(tb testing.TB) string {
	tb.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		tb.Fatal(err)
	}
	return address
}

// blackboxSetKey rewrites exactly one `key = value` line of a generated
// manifest, failing if the key is absent or ambiguous.
func blackboxSetKey(tb testing.TB, manifest []byte, key, value string) []byte {
	tb.Helper()
	lines := strings.Split(string(manifest), "\n")
	matched := 0
	for index, line := range lines {
		if strings.HasPrefix(line, key+" = ") {
			lines[index] = key + " = " + value
			matched++
		}
	}
	if matched != 1 {
		tb.Fatalf("manifest key %q matched %d lines, want 1", key, matched)
	}
	return []byte(strings.Join(lines, "\n"))
}

// do sends one request and returns the status and body. Callers decide which
// statuses are acceptable.
func (s *blackboxServer) do(method, pathname string, body io.Reader, contentType string) (int, []byte) {
	s.t.Helper()
	request, err := http.NewRequest(method, s.baseURL+pathname, body)
	if err != nil {
		s.t.Fatal(err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if s.token != "" {
		request.Header.Set("Authorization", "Bearer "+s.token)
	}
	response, err := s.client.Do(request)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, pathname, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		s.t.Fatal(err)
	}
	return response.StatusCode, payload
}

func (s *blackboxServer) mustJSON(method, pathname string, requestBody, responseBody any) {
	s.t.Helper()
	var body io.Reader
	contentType := ""
	if requestBody != nil {
		encoded, err := json.Marshal(requestBody)
		if err != nil {
			s.t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
		contentType = "application/json"
	}
	status, payload := s.do(method, pathname, body, contentType)
	if status != http.StatusOK {
		s.t.Fatalf("%s %s: %d %s", method, pathname, status, payload)
	}
	if responseBody != nil {
		if err := json.Unmarshal(payload, responseBody); err != nil {
			s.t.Fatalf("%s %s: decode %s: %v", method, pathname, payload, err)
		}
	}
}

// createRepository creates a regular repository beside the primary one in the
// default Storage Location.
func (s *blackboxServer) createRepository(name, directory string) blackboxRepository {
	s.t.Helper()
	var created struct {
		Repository blackboxRepository `json:"repository"`
	}
	s.mustJSON(http.MethodPost, "/api/v1/storage/repositories", map[string]any{
		"name": name, "directory_name": directory, "storage_strategy": "flat", "risk_confirmation": true,
	}, &created)
	if created.Repository.ID == "" || created.Repository.Path == "" {
		s.t.Fatalf("created repository = %+v", created.Repository)
	}
	s.waitIdle(created.Repository)
	return created.Repository
}

func (s *blackboxServer) removeRepository(repository blackboxRepository) {
	s.t.Helper()
	s.mustJSON(http.MethodPost, "/api/v1/storage/repositories/"+repository.ID+"/detach",
		map[string]any{"confirmation_name": repository.Name}, nil)
}

// writeFile writes an original into a repository with an mtime outside the
// settle window, the way a file copied in earlier looks to a scan.
func (s *blackboxServer) writeFile(repository blackboxRepository, relative string, contents []byte) string {
	s.t.Helper()
	target := filepath.Join(repository.Path, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(target, contents, 0o644); err != nil {
		s.t.Fatal(err)
	}
	settled := time.Now().Add(-time.Hour)
	if err := os.Chtimes(target, settled, settled); err != nil {
		s.t.Fatal(err)
	}
	return target
}

// scan requests a full verification and waits until a run that started after
// the request is terminal. A request that coalesces onto an already running
// verification may predate the caller's disk change, so it is repeated once
// that run ends.
func (s *blackboxServer) scan(repository blackboxRepository, timeout time.Duration) map[string]any {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	run := s.scanOnce(repository, deadline)
	if coalesced, _ := run["__coalesced"].(bool); coalesced {
		run = s.scanOnce(repository, deadline)
	}
	delete(run, "__coalesced")
	return run
}

func (s *blackboxServer) scanOnce(repository blackboxRepository, deadline time.Time) map[string]any {
	s.t.Helper()
	var queued struct {
		OperationID string `json:"operation_id"`
		Coalesced   bool   `json:"coalesced"`
	}
	s.mustJSON(http.MethodPost, "/api/v1/storage/repositories/"+repository.ID+"/verifications",
		map[string]any{"force": true}, &queued)
	if queued.OperationID == "" {
		s.t.Fatal("verification request returned no operation id")
	}
	for {
		var run map[string]any
		s.mustJSON(http.MethodGet, "/api/v1/storage/repositories/"+repository.ID+
			"/verifications/"+queued.OperationID, nil, &run)
		status, _ := run["status"].(string)
		if !blackboxActiveScanStatuses[status] {
			run["__coalesced"] = queued.Coalesced
			return run
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("scan of %s still %q at the deadline: %v", repository.Name, status, run)
		}
		time.Sleep(blackboxPollInterval)
	}
}

// listByFilename returns the default library browse for one filename, the
// query the Web library issues. Missing and trashed Assets must not appear.
func (s *blackboxServer) listByFilename(filename string) []blackboxAsset {
	s.t.Helper()
	var response struct {
		Items []struct {
			Asset     *blackboxAsset `json:"asset"`
			MediaItem *struct {
				PrimaryAsset *blackboxAsset `json:"primary_asset"`
			} `json:"media_item"`
		} `json:"items"`
	}
	s.mustJSON(http.MethodPost, "/api/v1/assets/list", map[string]any{
		"query": filename, "search_type": "filename",
		"pagination": map[string]any{"limit": 50, "offset": 0},
		"stack_mode": "expanded",
	}, &response)
	matches := make([]blackboxAsset, 0, len(response.Items))
	for _, item := range response.Items {
		asset := item.Asset
		if asset == nil && item.MediaItem != nil {
			asset = item.MediaItem.PrimaryAsset
		}
		if asset != nil && asset.OriginalFilename == filename {
			matches = append(matches, *asset)
		}
	}
	return matches
}

// assetStatus returns the HTTP status of the Asset detail endpoint: 200 while
// the Asset exists in any lifecycle state, 404 once it is purged.
func (s *blackboxServer) assetStatus(assetID string) int {
	s.t.Helper()
	status, _ := s.do(http.MethodGet, "/api/v1/assets/"+url.PathEscape(assetID), nil, "")
	return status
}

// upload sends one file through the single-file upload endpoint and waits
// until its catalog ingest receipt is terminal and successful.
func (s *blackboxServer) upload(repository blackboxRepository, filename string, contents []byte) {
	s.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("repository_id", repository.ID); err != nil {
		s.t.Fatal(err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
	header.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(header)
	if err != nil {
		s.t.Fatal(err)
	}
	if _, err := part.Write(contents); err != nil {
		s.t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		s.t.Fatal(err)
	}
	status, payload := s.do(http.MethodPost, "/api/v1/assets", &body, writer.FormDataContentType())
	if status != http.StatusOK {
		s.t.Fatalf("upload %s: %d %s", filename, status, payload)
	}
	var accepted struct {
		ReceiptID string `json:"receipt_id"`
	}
	if err := json.Unmarshal(payload, &accepted); err != nil || accepted.ReceiptID == "" {
		s.t.Fatalf("upload %s: no receipt in %s (%v)", filename, payload, err)
	}
	type operation struct {
		Terminal bool   `json:"terminal"`
		Success  bool   `json:"success"`
		Status   string `json:"status"`
	}
	var last operation
	terminal := waitFor(2*time.Minute, func() bool {
		var response struct {
			Operations []operation `json:"operations"`
		}
		s.mustJSON(http.MethodGet, "/api/v1/assets/batch/operations?receipt_ids="+accepted.ReceiptID, nil, &response)
		if len(response.Operations) != 1 {
			return false
		}
		last = response.Operations[0]
		return last.Terminal
	})
	if !terminal || !last.Success {
		s.t.Fatalf("upload %s receipt terminal=%t: %+v", filename, terminal, last)
	}
}

func (s *blackboxServer) createAlbum(name string) int64 {
	s.t.Helper()
	var album struct {
		AlbumID int64 `json:"album_id"`
	}
	s.mustJSON(http.MethodPost, "/api/v1/albums", map[string]any{"album_name": name}, &album)
	if album.AlbumID == 0 {
		s.t.Fatal("album creation returned no id")
	}
	return album.AlbumID
}

func (s *blackboxServer) addToAlbum(albumID int64, assetID string) {
	s.t.Helper()
	s.mustJSON(http.MethodPost, fmt.Sprintf("/api/v1/albums/%d/assets/%s", albumID, url.PathEscape(assetID)),
		map[string]any{}, nil)
}

func (s *blackboxServer) albumAssets(albumID int64) []blackboxAsset {
	s.t.Helper()
	var response struct {
		Assets []blackboxAsset `json:"assets"`
	}
	s.mustJSON(http.MethodGet, fmt.Sprintf("/api/v1/albums/%d/assets", albumID), nil, &response)
	return response.Assets
}

// waitFor polls condition until it holds or the timeout expires, returning
// whether it held. The condition reads only public state.
func waitFor(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if condition() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(blackboxPollInterval)
	}
}

// blackboxJPEG returns a small valid JPEG whose pixels are shade and whose
// bytes are unique to marker, so every call with a new marker is new content.
func blackboxJPEG(tb testing.TB, shade uint8, marker string) []byte {
	tb.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			picture.Set(x, y, color.RGBA{R: shade, G: uint8(x * 16), B: uint8(y * 16), A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, picture, &jpeg.Options{Quality: 90}); err != nil {
		tb.Fatal(err)
	}
	return blackboxWithComment(encoded.Bytes(), marker)
}

// blackboxWithComment inserts a JPEG COM segment after SOI. Pixels are
// unchanged; only content identity changes, like a metadata-only write.
func blackboxWithComment(source []byte, marker string) []byte {
	comment := []byte(marker)
	length := len(comment) + 2
	out := make([]byte, 0, len(source)+length+2)
	out = append(out, source[:2]...)
	out = append(out, 0xff, 0xfe, byte(length>>8), byte(length))
	out = append(out, comment...)
	return append(out, source[2:]...)
}
