// Package response provides a consistent JSON envelope for all API responses.
// Every handler in the application writes through this package so that clients
// always receive a predictable structure regardless of success or failure.
//
// Success envelope:
//
//	{
//	  "success": true,
//	  "message": "optional human-readable note",
//	  "data":    { ... }          // any JSON-serialisable value
//	}
//
// Error envelope:
//
//	{
//	  "success": false,
//	  "message": "human-readable error",
//	  "errors":  [ ... ]          // optional field-level validation errors
//	}
package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// envelope is the top-level JSON wrapper written for every response.
type envelope struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	// Data is omitted from the output when nil so error responses don't carry
	// a redundant null field.
	Data   any `json:"data,omitempty"`
	Errors any `json:"errors,omitempty"`
}

// JSON writes a JSON response with the given HTTP status code and body.
// Serialisation errors are logged and result in a plain 500 response so the
// caller never has to handle a write failure.
func JSON(w http.ResponseWriter, status int, body any, log *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Error("response: failed to encode JSON", "error", err)
	}
}

// Success writes a 2xx JSON success response.
//
//	response.Success(w, http.StatusOK, "vehicle retrieved", vehicle, log)
func Success(w http.ResponseWriter, status int, message string, data any, log *slog.Logger) {
	JSON(w, status, envelope{
		Success: true,
		Message: message,
		Data:    data,
	}, log)
}

// Error writes a 4xx/5xx JSON error response.
// errs is optional — pass nil when there are no field-level details.
//
//	response.Error(w, http.StatusBadRequest, "validation failed", fieldErrors, log)
func Error(w http.ResponseWriter, status int, message string, errs any, log *slog.Logger) {
	JSON(w, status, envelope{
		Success: false,
		Message: message,
		Errors:  errs,
	}, log)
}

// NoContent writes a 204 response with no body.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
