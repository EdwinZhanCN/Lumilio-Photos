package api

import (
	"encoding/json"
	"net/http"

	"server/internal/api/problem"

	"github.com/gin-gonic/gin"
)

const ProblemMediaType = problem.MediaType

// OpenAPI response models are aliases of the registry-owned wire structs.
// Runtime emission still goes exclusively through WriteProblem.
type ProblemResponse = problem.Details
type RateLimitedProblemResponse = problem.RateLimitedDetails
type RepositoryConflictProblemResponse = problem.RepositoryConflictDetails
type ProblemReference = problem.Reference

const (
	problemInstanceContextKey = "lumilio.problem.instance"
	problemTypeContextKey     = "lumilio.problem.type"
	problemCauseContextKey    = "lumilio.problem.cause"
)

// SuccessResponse represents a simple success response for endpoints that only return a message.
type SuccessResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// JSONOK sends a successful JSON response without an API envelope.
func JSONOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// WriteProblem is the only HTTP Problem emitter. Public fields come from the
// closed registry; the private cause is retained only for request logging.
func WriteProblem(c *gin.Context, failure problem.Failure) {
	instance := problem.NewInstance()
	body, err := json.Marshal(failure.Body(instance))
	if err != nil {
		failure = problem.About(http.StatusInternalServerError, err)
		body, _ = json.Marshal(failure.Body(instance))
	}
	c.Set(problemInstanceContextKey, instance)
	c.Set(problemTypeContextKey, failure.Type())
	if cause := failure.Cause(); cause != nil {
		c.Set(problemCauseContextKey, cause)
	}
	c.Data(failure.Status(), ProblemMediaType, body)
}

// Generic status constructors deliberately accept no display string.
func BadRequest(cause error) problem.Failure   { return problem.About(http.StatusBadRequest, cause) }
func Unauthorized(cause error) problem.Failure { return problem.About(http.StatusUnauthorized, cause) }
func Forbidden(cause error) problem.Failure    { return problem.About(http.StatusForbidden, cause) }
func NotFound(cause error) problem.Failure     { return problem.About(http.StatusNotFound, cause) }
func Internal(cause error) problem.Failure {
	return problem.About(http.StatusInternalServerError, cause)
}
func StatusProblem(status int, cause error) problem.Failure {
	return problem.About(status, cause)
}

// KnownProblem selects a registered type with no extension members.
func KnownProblem(descriptor problem.Descriptor, cause error) problem.Failure {
	return problem.New(descriptor, cause)
}
