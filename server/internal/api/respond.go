package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse represents a standardized error response.
type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"Bad request"`
	Error   string `json:"error,omitempty" example:"validation failed"`
}

// SuccessResponse represents a simple success response for endpoints that only return a message.
type SuccessResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// JSONOK sends a successful JSON response without an API envelope.
func JSONOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// GinError sends a standardized error response using gin.Context.
func GinError(c *gin.Context, code int, err error, statusCode int, messages ...string) {
	msg := "operation failed"
	if len(messages) > 0 {
		msg = messages[0]
	}

	result := ErrorResponse{
		Code:    code,
		Message: msg,
	}
	if err != nil {
		result.Error = err.Error()
	}
	c.JSON(statusCode, result)
}

// HandleError is a helper function for consistent error handling with gin.Context.
func HandleError(c *gin.Context, statusCode int, message string, err error) {
	GinError(c, statusCode, err, statusCode, message)
}

// GinBadRequest sends a 400 Bad Request response.
func GinBadRequest(c *gin.Context, err error, message ...string) {
	msg := "Bad request"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusBadRequest, err, http.StatusBadRequest, msg)
}

// GinUnauthorized sends a 401 Unauthorized response.
func GinUnauthorized(c *gin.Context, err error, message ...string) {
	msg := "Unauthorized"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusUnauthorized, err, http.StatusUnauthorized, msg)
}

// GinForbidden sends a 403 Forbidden response.
func GinForbidden(c *gin.Context, err error, message ...string) {
	msg := "Access denied"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusForbidden, err, http.StatusForbidden, msg)
}

// GinNotFound sends a 404 Not Found response.
func GinNotFound(c *gin.Context, err error, message ...string) {
	msg := "Resource not found"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusNotFound, err, http.StatusNotFound, msg)
}

// GinInternalError sends a 500 Internal Server Error response.
func GinInternalError(c *gin.Context, err error, message ...string) {
	msg := "Internal server error"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, msg)
}
