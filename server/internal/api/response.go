package api

import (
	"encoding/json"
	"net/http"
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

// HandleError is a helper function for consistent error handling
func HandleError(c interface{}, statusCode int, message string, err error) {
	// This is a placeholder - in a real implementation, you'd use the gin.Context
	// For now, we'll just use the existing Error function approach
	response := ErrorResponse{
		Code:    statusCode,
		Message: message,
	}
	if err != nil {
		response.Error = err.Error()
	}

	// Note: This would need to be implemented properly with gin.Context
	// c.JSON(statusCode, response)
}
