package api

import (
	"encoding/json"
	"net/http"
)

type Result struct {
	Code    int         `json:"code"`            // Business status code
	Message string      `json:"message"`         // User readable message
	Data    interface{} `json:"data,omitempty"`  // Business data, ignore empty values
	Error   string      `json:"error,omitempty"` // Debug error message, ignore empty values
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
