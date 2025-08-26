package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Result represents the standard API response format
// @Description Standard API response wrapper
type Result struct {
	Code    int         `json:"code" example:"0"`                        // Business status code (0 for success, non-zero for errors)
	Message string      `json:"message" example:"success"`               // User readable message
	Data    interface{} `json:"data,omitempty" swaggertype:"object"`     // Business data, ignore empty values
	Error   string      `json:"error,omitempty" example:"error details"` // Debug error message, ignore empty values
}

// Success standardized success response constructor
func Success(w http.ResponseWriter, data interface{}) {
	result := &Result{
		Code:    0,
		Message: "success",
		Data:    data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// Error standardized error response constructor
func Error(w http.ResponseWriter, code int, err error, statusCode int, messages ...string) {
	msg := "operation failed"
	if len(messages) > 0 {
		msg = messages[0]
	}

	result := &Result{
		Code:    code,
		Message: msg,
		Error:   err.Error(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(result)
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"Bad request"`
	Error   string `json:"error,omitempty" example:"validation failed"`
}

// SuccessResponse represents a simple success response
type SuccessResponse struct {
	Message string `json:"message" example:"Operation completed successfully"`
}

// Gin-specific helper functions

// GinSuccess sends a standardized success response using gin.Context
func GinSuccess(c *gin.Context, data interface{}) {
	result := &Result{
		Code:    0,
		Message: "success",
		Data:    data,
	}
	c.JSON(http.StatusOK, result)
}

// GinError sends a standardized error response using gin.Context
func GinError(c *gin.Context, code int, err error, statusCode int, messages ...string) {
	msg := "operation failed"
	if len(messages) > 0 {
		msg = messages[0]
	}

	result := &Result{
		Code:    code,
		Message: msg,
		Error:   err.Error(),
	}
	c.JSON(statusCode, result)
}

// HandleError is a helper function for consistent error handling with gin.Context
func HandleError(c *gin.Context, statusCode int, message string, err error) {
	result := &Result{
		Code:    statusCode,
		Message: message,
	}
	if err != nil {
		result.Error = err.Error()
	}
	c.JSON(statusCode, result)
}

// Common response helpers

// GinBadRequest sends a 400 Bad Request response
func GinBadRequest(c *gin.Context, err error, message ...string) {
	msg := "Bad request"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusBadRequest, err, http.StatusBadRequest, msg)
}

// GinUnauthorized sends a 401 Unauthorized response
func GinUnauthorized(c *gin.Context, err error, message ...string) {
	msg := "Unauthorized"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusUnauthorized, err, http.StatusUnauthorized, msg)
}

// GinForbidden sends a 403 Forbidden response
func GinForbidden(c *gin.Context, err error, message ...string) {
	msg := "Access denied"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusForbidden, err, http.StatusForbidden, msg)
}

// GinNotFound sends a 404 Not Found response
func GinNotFound(c *gin.Context, err error, message ...string) {
	msg := "Resource not found"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusNotFound, err, http.StatusNotFound, msg)
}

// GinInternalError sends a 500 Internal Server Error response
func GinInternalError(c *gin.Context, err error, message ...string) {
	msg := "Internal server error"
	if len(message) > 0 {
		msg = message[0]
	}
	GinError(c, http.StatusInternalServerError, err, http.StatusInternalServerError, msg)
}
