// Package problem defines Lumilio's language-neutral RFC 9457 vocabulary.
//
// It is intentionally a leaf package: handlers and transports may depend on
// these descriptors, while domain packages remain unaware of HTTP problem URIs.
package problem

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
)

const (
	MediaType = "application/problem+json"
	Namespace = "https://lumilio.org/problems/"
)

// Descriptor is the canonical definition of one public Problem type.
type Descriptor struct {
	Type       string
	Status     int
	Title      string
	Definition string
	Recovery   string
	Extensions []Extension
}

// Extension documents one bounded, type-specific public member.
type Extension struct {
	Name        string
	Schema      string
	Description string
}

var (
	InvalidCredentials = descriptor("auth/invalid-credentials", http.StatusUnauthorized,
		"Invalid credentials", "The supplied sign-in credentials were not accepted.")
	AuthenticationRequired = descriptor("auth/authentication-required", http.StatusUnauthorized,
		"Authentication required", "A valid authenticated session is required for this operation.")
	SessionExpired = descriptor("auth/session-expired", http.StatusUnauthorized,
		"Session expired", "The browser session can no longer be refreshed.")
	PasskeyUnavailable = descriptor("auth/passkey-unavailable", http.StatusBadRequest,
		"Passkey unavailable", "Passkey authentication is unavailable for this request.")
	MFAInvalid = descriptor("auth/mfa-invalid", http.StatusUnauthorized,
		"Invalid multi-factor authentication", "The supplied multi-factor authentication proof was not accepted.")
	PermissionDenied = descriptor("auth/permission-denied", http.StatusForbidden,
		"Permission denied", "The authenticated user cannot perform this operation.")
	UntrustedOrigin = descriptor("auth/untrusted-origin", http.StatusForbidden,
		"Untrusted request origin", "The browser request origin is not trusted for a session-changing operation.")
	RateLimited = Descriptor{
		Type: Namespace + "auth/rate-limited", Status: http.StatusTooManyRequests,
		Title: "Too many requests", Definition: "Authentication requests are temporarily rate limited.",
		Recovery:   "Wait for Retry-After, then retry the authentication request.",
		Extensions: []Extension{{Name: "retry_after_seconds", Schema: "integer", Description: "Whole seconds until the request may be retried."}},
	}
	AppNotInitialized = descriptor("bootstrap/app-not-initialized", http.StatusConflict,
		"Application not initialized", "First-run setup must finish before this operation is available.")
	RepositoryUnavailable = descriptor("repository/unavailable", http.StatusServiceUnavailable,
		"Repository unavailable", "The Repository required by the operation is unavailable.")
	RepositoryConflict = Descriptor{
		Type: Namespace + "repository/conflict", Status: http.StatusConflict,
		Title: "Repository conflict", Definition: "Repository state requires an explicit recovery choice.",
		Recovery: "Choose one of the response's allowed recovery actions, then retry.",
		Extensions: []Extension{
			{Name: "conflict_type", Schema: "string", Description: "Stable recovery classification."},
			{Name: "repository_id", Schema: "string", Description: "Safe Repository identity, when known."},
			{Name: "actions", Schema: "array[string]", Description: "Stable recovery actions allowed for this conflict."},
		},
	}
	StorageConfirmationRequired = descriptor("storage/confirmation-required", http.StatusConflict,
		"Storage confirmation required", "The storage operation requires explicit administrator risk confirmation.")
	ServiceUnavailable = descriptor("service/unavailable", http.StatusServiceUnavailable,
		"Service unavailable", "A required optional service is currently unavailable.")
	InvalidMediaRequest = descriptor("media/invalid-request", http.StatusUnprocessableEntity,
		"Invalid media request", "The requested media operation cannot process the supplied media.")
	ImageEmbeddingMissing = descriptor("media/image-embedding-missing", http.StatusConflict,
		"Image embedding missing", "The selected asset does not have an Image Semantic Analysis embedding.")
	SemanticAnalysisUnavailable = descriptor("lumen/image-semantic-analysis-unavailable", http.StatusServiceUnavailable,
		"Image Semantic Analysis unavailable", "Image Semantic Analysis is not currently available.")
	AgentFailed = descriptor("agent/operation-failed", http.StatusInternalServerError,
		"Agent operation failed", "The Agent operation could not be completed.")
	UploadProcessingFailed = descriptor("upload/processing-failed", http.StatusInternalServerError,
		"Upload processing failed", "An accepted upload could not be processed.")
	RepositoryScanFailed = descriptor("repository/scan-failed", http.StatusInternalServerError,
		"Repository scan failed", "An accepted Repository scan could not be completed.")
	RepositoryScanIncomplete = descriptor("repository/scan-incomplete", http.StatusInternalServerError,
		"Repository scan incomplete", "A Repository scan completed with partial results.")
	BackupRestoreFailed = descriptor("backup/restore-failed", http.StatusInternalServerError,
		"Backup restore failed", "An accepted database restore could not be completed.")
	HostActionFailed = descriptor("storage/host-action-failed", http.StatusInternalServerError,
		"Native host action failed", "An accepted native host action could not be completed.")
	HostActionExpired = descriptor("storage/host-action-expired", http.StatusInternalServerError,
		"Native host action expired", "A native host action expired before it was approved.")
	CloudImportFailed = descriptor("cloud/import-failed", http.StatusInternalServerError,
		"Cloud import failed", "An accepted cloud import could not be completed.")
)

var registry = mustRegistry(
	InvalidCredentials,
	AuthenticationRequired,
	SessionExpired,
	PasskeyUnavailable,
	MFAInvalid,
	PermissionDenied,
	UntrustedOrigin,
	RateLimited,
	AppNotInitialized,
	RepositoryUnavailable,
	RepositoryConflict,
	StorageConfirmationRequired,
	ServiceUnavailable,
	InvalidMediaRequest,
	ImageEmbeddingMissing,
	SemanticAnalysisUnavailable,
	AgentFailed,
	UploadProcessingFailed,
	RepositoryScanFailed,
	RepositoryScanIncomplete,
	BackupRestoreFailed,
	HostActionFailed,
	HostActionExpired,
	CloudImportFailed,
)

func descriptor(path string, status int, title, definition string) Descriptor {
	return Descriptor{
		Type: Namespace + path, Status: status, Title: title, Definition: definition,
		Recovery: "Correct the documented condition, then retry the operation.",
	}
}

func mustRegistry(descriptors ...Descriptor) map[string]Descriptor {
	result := make(map[string]Descriptor, len(descriptors))
	for _, item := range descriptors {
		if item.Type == "" || item.Status < 400 || item.Status > 599 || item.Title == "" || item.Definition == "" || item.Recovery == "" {
			panic(fmt.Sprintf("invalid Problem descriptor: %#v", item))
		}
		if _, exists := result[item.Type]; exists {
			panic("duplicate Problem descriptor: " + item.Type)
		}
		result[item.Type] = item
	}
	return result
}

// Lookup returns the registered descriptor for a Lumilio Problem type.
func Lookup(problemType string) (Descriptor, bool) {
	descriptor, ok := registry[problemType]
	return descriptor, ok
}

// Registered returns the canonical catalog in stable URI order.
func Registered() []Descriptor {
	result := make([]Descriptor, 0, len(registry))
	for _, descriptor := range registry {
		result = append(result, descriptor)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Type < result[j].Type })
	return result
}

// Details is Lumilio's deliberately narrow RFC 9457 base response.
type Details struct {
	Type     string `json:"type" binding:"required" example:"https://lumilio.org/problems/auth/invalid-credentials"`
	Status   int    `json:"status" binding:"required" example:"401"`
	Instance string `json:"instance" binding:"required" example:"urn:lumilio:problem:0123456789abcdef0123456789abcdef"`
}

// RateLimitedDetails mirrors Retry-After for localized countdown copy.
type RateLimitedDetails struct {
	Details
	RetryAfterSeconds int64 `json:"retry_after_seconds" binding:"required" minimum:"1" example:"60"`
}

// RepositoryConflictDetails carries only bounded recovery facts. Host paths
// and display prose are intentionally absent.
type RepositoryConflictDetails struct {
	Details
	ConflictType string   `json:"conflict_type" binding:"required" example:"repository_identity"`
	RepositoryID string   `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Actions      []string `json:"actions,omitempty" example:"relocate,copy"`
}

// Reference is the transport-neutral equivalent used by streams and
// persisted asynchronous operation DTOs. It intentionally has no HTTP status.
type Reference struct {
	Type      string `json:"type" binding:"required" example:"https://lumilio.org/problems/upload/processing-failed"`
	Instance  string `json:"instance" binding:"required" example:"urn:lumilio:problem:0123456789abcdef0123456789abcdef"`
	Retryable *bool  `json:"retryable,omitempty"`
}

// NewInstance creates an opaque occurrence URI for one request or ephemeral
// stream failure. It contains no caller or domain identifier.
func NewInstance() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("generate Problem occurrence ID: " + err.Error())
	}
	return instanceFromBytes(value[:])
}

// StableInstance derives an opaque occurrence URI from a durable operation
// identity. The identity is not embedded in the URI; the same persisted
// operation therefore retains its support reference across restarts.
func StableInstance(descriptor Descriptor, operationIdentity string) string {
	digest := sha256.Sum256([]byte("lumilio.problem.instance.v1\x00" + descriptor.Type + "\x00" + operationIdentity))
	return instanceFromBytes(digest[:16])
}

// NewReference creates a Problem Reference for an ephemeral stream or item
// failure. References are limited to descriptors in the closed registry.
func NewReference(descriptor Descriptor, retryable bool) Reference {
	return reference(descriptor, NewInstance(), retryable)
}

// ReferenceFor creates a stable Problem Reference for a persisted operation.
func ReferenceFor(descriptor Descriptor, operationIdentity string, retryable bool) Reference {
	return reference(descriptor, StableInstance(descriptor, operationIdentity), retryable)
}

func reference(descriptor Descriptor, instance string, retryable bool) Reference {
	registered, ok := Lookup(descriptor.Type)
	if !ok || registered.Status != descriptor.Status {
		registered = ServiceUnavailable
	}
	return Reference{Type: registered.Type, Instance: instance, Retryable: &retryable}
}

func instanceFromBytes(value []byte) string {
	return "urn:lumilio:problem:" + hex.EncodeToString(value)
}

type bodyKind uint8

const (
	baseBody bodyKind = iota
	rateLimitedBody
	repositoryConflictBody
)

// Failure pairs one safe public descriptor with its private diagnostic cause.
// Its fields are private so callers cannot add arbitrary response members.
type Failure struct {
	descriptor        Descriptor
	status            int
	cause             error
	kind              bodyKind
	retryAfterSeconds int64
	conflictType      string
	repositoryID      string
	actions           []string
}

// About creates a generic about:blank failure for a status whose meaning and
// operation-local UI fallback are sufficient.
func About(status int, cause error) Failure {
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	return Failure{descriptor: Descriptor{Type: "about:blank", Status: status}, status: status, cause: cause}
}

// New creates a registered Problem failure without extension members.
func New(descriptor Descriptor, cause error) Failure {
	registered, ok := Lookup(descriptor.Type)
	if !ok || registered.Status != descriptor.Status || len(registered.Extensions) != 0 {
		return About(http.StatusInternalServerError, fmt.Errorf("invalid Problem descriptor %q: %w", descriptor.Type, cause))
	}
	return Failure{descriptor: registered, status: registered.Status, cause: cause}
}

// NewRateLimited creates the exact typed rate-limit subtype.
func NewRateLimited(cause error, retryAfterSeconds int64) Failure {
	if retryAfterSeconds < 1 {
		retryAfterSeconds = 1
	}
	return Failure{descriptor: RateLimited, status: RateLimited.Status, cause: cause, kind: rateLimitedBody, retryAfterSeconds: retryAfterSeconds}
}

// NewRepositoryConflict creates the exact typed Repository conflict subtype.
func NewRepositoryConflict(cause error, conflictType, repositoryID string, actions []string) Failure {
	return Failure{
		descriptor: RepositoryConflict, status: RepositoryConflict.Status, cause: cause,
		kind: repositoryConflictBody, conflictType: conflictType, repositoryID: repositoryID,
		actions: append([]string(nil), actions...),
	}
}

func (f Failure) Status() int            { return f.status }
func (f Failure) Type() string           { return f.descriptor.Type }
func (f Failure) Cause() error           { return f.cause }
func (f Failure) Descriptor() Descriptor { return f.descriptor }

// Body materializes the public typed payload after the transport assigns an
// opaque occurrence instance.
func (f Failure) Body(instance string) any {
	base := Details{Type: f.Type(), Status: f.Status(), Instance: instance}
	switch f.kind {
	case rateLimitedBody:
		return RateLimitedDetails{Details: base, RetryAfterSeconds: f.retryAfterSeconds}
	case repositoryConflictBody:
		return RepositoryConflictDetails{
			Details: base, ConflictType: f.conflictType, RepositoryID: f.repositoryID,
			Actions: append([]string(nil), f.actions...),
		}
	default:
		return base
	}
}
