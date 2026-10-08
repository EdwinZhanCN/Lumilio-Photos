package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	pb "github.com/edwinzhancn/lumen-sdk/proto"
	"google.golang.org/protobuf/encoding/protojson"
)

// Fixture layout (committed under server/tools/fakelumen/fixtures/):
//
//	manifest.json                 schema version, recording provenance, and the
//	                              upstream capability of every recorded service
//	records/<task>/<sha256>.json  one recorded inference: request identity by
//	                              payload hash, response bytes verbatim
//
// Replay looks a request up by (task, sha256(payload)). The payload bytes are
// produced by the server's own preprocessing (libvips decode + SDK tensor
// path), so fixtures recorded through the containerized E2E stack replay
// byte-identically in CI. A miss falls back to the deterministic builtin
// response and increments the per-task miss counter; recording again is the
// re-sync procedure when seeds, specs, or the preprocessing pipeline change.
const fixtureSchemaVersion = 1

const (
	manifestFilename = "manifest.json"
	recordsDirname   = "records"
)

type fixtureManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	RecordedFrom  string            `json:"recordedFrom,omitempty"`
	RecordedAt    string            `json:"recordedAt,omitempty"`
	Note          string            `json:"note,omitempty"`
	Capabilities  []json.RawMessage `json:"capabilities,omitempty"`
}

type fixtureRecord struct {
	Task          string `json:"task"`
	PayloadSHA256 string `json:"payloadSha256"`
	PayloadBytes  int    `json:"payloadBytes"`
	PayloadMime   string `json:"payloadMime,omitempty"`
	// PayloadText keeps small text payloads readable in review; binary
	// payloads (images, tensors) are identified by hash only.
	PayloadText  string            `json:"payloadText,omitempty"`
	RequestMeta  map[string]string `json:"requestMeta,omitempty"`
	ResultMime   string            `json:"resultMime"`
	ResultJSON   json.RawMessage   `json:"resultJson,omitempty"`
	ResultBase64 string            `json:"resultBase64,omitempty"`
	ResponseMeta map[string]string `json:"responseMeta,omitempty"`
}

func (r *fixtureRecord) resultBytes() ([]byte, error) {
	if len(r.ResultJSON) > 0 {
		return r.ResultJSON, nil
	}
	if r.ResultBase64 != "" {
		return base64.StdEncoding.DecodeString(r.ResultBase64)
	}
	return nil, fmt.Errorf("fixture %s/%s has neither resultJson nor resultBase64", r.Task, r.PayloadSHA256)
}

func payloadDigest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func recordKey(task, digest string) string { return task + "\x00" + digest }

// fixtureStore indexes recorded inferences and the recorded capability set.
// It is read-only in replay mode; record mode writes through to dir.
//
// The manifest persists an upstream capability only once at least one of its
// tasks has a recorded fixture. Replay overrides the builtin capability of
// exactly those services, so recording one service (say, face) never swaps
// the model identity or tensor contract of services whose requests still
// fall back to builtin responses.
type fixtureStore struct {
	mu       sync.Mutex
	manifest fixtureManifest
	caps     []*pb.Capability
	records  map[string]*fixtureRecord
	dir      string // non-empty only when opened writable for recording
	// upstream is the latest capability set streamed by the recording hub,
	// including services that have no recorded fixture yet.
	upstream     []*pb.Capability
	upstreamAddr string
}

// loadFixtureStore reads a fixture tree from any fs.FS (the embedded default
// or an override directory). A missing manifest yields an empty store: replay
// then serves only the deterministic builtin capability.
func loadFixtureStore(fsys fs.FS) (*fixtureStore, error) {
	store := &fixtureStore{records: map[string]*fixtureRecord{}}

	manifestBytes, err := fs.ReadFile(fsys, manifestFilename)
	switch {
	case err == nil:
		if err := json.Unmarshal(manifestBytes, &store.manifest); err != nil {
			return nil, fmt.Errorf("parse %s: %w", manifestFilename, err)
		}
		if store.manifest.SchemaVersion != fixtureSchemaVersion {
			return nil, fmt.Errorf("%s schemaVersion %d is not the supported %d",
				manifestFilename, store.manifest.SchemaVersion, fixtureSchemaVersion)
		}
		for i, raw := range store.manifest.Capabilities {
			capability := &pb.Capability{}
			if err := protojson.Unmarshal(raw, capability); err != nil {
				return nil, fmt.Errorf("parse manifest capability %d: %w", i, err)
			}
			store.caps = append(store.caps, capability)
		}
	case os.IsNotExist(err):
		store.manifest = fixtureManifest{SchemaVersion: fixtureSchemaVersion}
	default:
		return nil, fmt.Errorf("read %s: %w", manifestFilename, err)
	}

	err = fs.WalkDir(fsys, recordsDirname, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		raw, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read fixture %s: %w", path, err)
		}
		record := &fixtureRecord{}
		if err := json.Unmarshal(raw, record); err != nil {
			return fmt.Errorf("parse fixture %s: %w", path, err)
		}
		if record.Task == "" || record.PayloadSHA256 == "" {
			return fmt.Errorf("fixture %s is missing task or payloadSha256", path)
		}
		store.records[recordKey(record.Task, record.PayloadSHA256)] = record
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return store, nil
}

// openFixtureDir loads a fixture tree from disk and keeps it writable so
// record mode can append fixtures and persist the upstream capability set.
func openFixtureDir(dir string) (*fixtureStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create fixtures dir: %w", err)
	}
	store, err := loadFixtureStore(os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	store.dir = dir
	return store, nil
}

func (s *fixtureStore) lookup(task string, digest string) (*fixtureRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[recordKey(task, digest)]
	return record, ok
}

func (s *fixtureStore) size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

func (s *fixtureStore) capabilities() []*pb.Capability {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*pb.Capability(nil), s.caps...)
}

// put indexes a recorded fixture and writes it to the store directory.
// Re-recording an identical (task, payload) pair overwrites in place, so a
// repeated recording run converges instead of accumulating duplicates.
func (s *fixtureStore) put(record *fixtureRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return fmt.Errorf("fixture store is read-only; recording requires -fixtures <dir>")
	}
	s.records[recordKey(record.Task, record.PayloadSHA256)] = record

	taskDir := filepath.Join(s.dir, recordsDirname, record.Task)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		return fmt.Errorf("create record dir: %w", err)
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode fixture: %w", err)
	}
	target := filepath.Join(taskDir, record.PayloadSHA256+".json")
	if err := os.WriteFile(target, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write fixture: %w", err)
	}
	// The first fixture of a service persists that service's capability.
	if capabilityForTask(s.caps, record.Task) == nil && capabilityForTask(s.upstream, record.Task) != nil {
		return s.writeManifestLocked()
	}
	return nil
}

// setCapabilities remembers the upstream capability set and persists the
// capabilities of services that have recorded fixtures, so a later replay
// advertises exactly what the recording hub advertised for those services.
func (s *fixtureStore) setCapabilities(caps []*pb.Capability, upstream string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return fmt.Errorf("fixture store is read-only; recording requires -fixtures <dir>")
	}
	s.upstream = append([]*pb.Capability(nil), caps...)
	s.upstreamAddr = upstream
	return s.writeManifestLocked()
}

// writeManifestLocked recomputes the recorded capability set — the upstream
// capability of every service with a recorded task, plus previously persisted
// capabilities of recorded services this upstream does not advertise — and
// writes the manifest.
func (s *fixtureStore) writeManifestLocked() error {
	recorded := make([]*pb.Capability, 0, len(s.upstream))
	for _, capability := range s.upstream {
		if s.hasRecordLocked(capability) {
			recorded = append(recorded, capability)
		}
	}
	for _, capability := range s.caps {
		upstreamHasService := slices.ContainsFunc(s.upstream, func(candidate *pb.Capability) bool {
			return candidate.GetServiceName() == capability.GetServiceName()
		})
		if !upstreamHasService && s.hasRecordLocked(capability) {
			recorded = append(recorded, capability)
		}
	}

	s.caps = recorded
	s.manifest.SchemaVersion = fixtureSchemaVersion
	s.manifest.RecordedFrom = s.upstreamAddr
	s.manifest.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	s.manifest.Capabilities = s.manifest.Capabilities[:0]
	marshal := protojson.MarshalOptions{UseProtoNames: true}
	for _, capability := range recorded {
		raw, err := marshal.Marshal(capability)
		if err != nil {
			return fmt.Errorf("encode capability: %w", err)
		}
		s.manifest.Capabilities = append(s.manifest.Capabilities, json.RawMessage(raw))
	}
	raw, err := json.MarshalIndent(s.manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, manifestFilename), append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func (s *fixtureStore) hasRecordLocked(capability *pb.Capability) bool {
	for _, record := range s.records {
		if capabilityForTask([]*pb.Capability{capability}, record.Task) != nil {
			return true
		}
	}
	return false
}

// capabilityForTask returns the capability in caps that serves task, if any.
func capabilityForTask(caps []*pb.Capability, task string) *pb.Capability {
	for _, capability := range caps {
		for _, candidate := range capability.GetTasks() {
			if candidate.GetName() == task {
				return capability
			}
		}
	}
	return nil
}

// newFixtureRecord captures one completed upstream exchange. Text payloads are
// kept inline for review; anything non-UTF-8 or large stays hash-only.
func newFixtureRecord(request *pb.InferRequest, resultMime string, result []byte, responseMeta map[string]string) *fixtureRecord {
	record := &fixtureRecord{
		Task:          request.GetTask(),
		PayloadSHA256: payloadDigest(request.GetPayload()),
		PayloadBytes:  len(request.GetPayload()),
		PayloadMime:   request.GetPayloadMime(),
		RequestMeta:   request.GetMeta(),
		ResultMime:    resultMime,
		ResponseMeta:  responseMeta,
	}
	payload := request.GetPayload()
	if len(payload) <= 4096 && utf8.Valid(payload) &&
		strings.HasPrefix(request.GetPayloadMime(), "text/") {
		record.PayloadText = string(payload)
	}
	if json.Valid(result) && strings.Contains(resultMime, "application/json") {
		record.ResultJSON = json.RawMessage(result)
	} else {
		record.ResultBase64 = base64.StdEncoding.EncodeToString(result)
	}
	return record
}
