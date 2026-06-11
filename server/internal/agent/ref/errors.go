package ref

import "fmt"

// Code enumerates the recoverable tool error codes (INV-6). Tools never
// return these as Go errors to the agent loop — they embed an *Error in the
// tool output so the model can read the code and follow the hint.
type Code string

const (
	CodeRefNotFound        Code = "RefNotFound"
	CodeEmptySet           Code = "EmptySet"
	CodeLimitExceeded      Code = "LimitExceeded"
	CodeInvalidArgument    Code = "InvalidArgument"
	CodeFeatureUnavailable Code = "FeatureUnavailable"
	CodeInternal           Code = "Internal"
)

// Error is the typed, agent-readable error envelope. Message states what
// happened; Hint states how the agent can recover.
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NotFound is returned for missing, expired and cross-scope refs alike, so
// existence does not leak across scopes (INV-4).
func NotFound(id string) *Error {
	return &Error{
		Code:    CodeRefNotFound,
		Message: fmt.Sprintf("ref %q does not exist or has expired", id),
		Hint:    "re-run the query that produced this result to get a fresh ref",
	}
}

// EmptySet flags an operation that received a ref with zero members.
func EmptySet(id string) *Error {
	return &Error{
		Code:    CodeEmptySet,
		Message: fmt.Sprintf("ref %q is an empty set", id),
		Hint:    "broaden the filter or try a different search dimension",
	}
}

// LimitExceeded flags a mutation whose target set is larger than allowed.
func LimitExceeded(count, limit int) *Error {
	return &Error{
		Code:    CodeLimitExceeded,
		Message: fmt.Sprintf("operation targets %d assets, maximum is %d", count, limit),
		Hint:    "narrow the set first, e.g. with a stricter filter or top/sample",
	}
}

// InvalidArgument flags an out-of-range or unsupported parameter value.
func InvalidArgument(message string) *Error {
	return &Error{
		Code:    CodeInvalidArgument,
		Message: message,
	}
}

// FeatureUnavailable flags a capability that is not configured or offline.
func FeatureUnavailable(message string) *Error {
	return &Error{
		Code:    CodeFeatureUnavailable,
		Message: message,
		Hint:    "this capability is currently unavailable; try a different approach",
	}
}

// Internal wraps an unexpected backend failure (e.g. a database error) so the
// agent loop keeps running (INV-6). The underlying error is not exposed.
func Internal(operation string) *Error {
	return &Error{
		Code:    CodeInternal,
		Message: fmt.Sprintf("%s failed unexpectedly", operation),
		Hint:    "retry once; if it fails again, tell the user and continue without it",
	}
}
