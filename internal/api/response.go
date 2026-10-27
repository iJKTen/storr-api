package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type ErrorResponse struct {
	Message    string            `json:"message"`
	Code       string            `json:"code"`
	StatusCode int               `json:"statusCode"`
	Details    map[string]string `json:"details,omitempty"`
}

type SuccessResponse struct {
	OK bool `json:"ok"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

// WriteJSON writes pre-marshaled JSON bytes to the response with the given status code.
// Logs a warning if the write to the client fails (e.g. connection closed).
func WriteJSON(w http.ResponseWriter, status int, data json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		slog.Warn("failed to write HTTP response", "error", err)
	}
}

// WriteError sends a structured error response to the client.
// Falls back to a plain-text error if JSON marshaling fails (should never happen for ErrorResponse).
func WriteError(w http.ResponseWriter, status int, message, code string) {
	data, err := json.Marshal(ErrorResponse{
		Message:    message,
		Code:       code,
		StatusCode: status,
	})
	if err != nil {
		slog.Error("failed to marshal error response", "error", err)
		http.Error(w, `{"message":"internal error","code":"MARSHAL_ERROR"}`, http.StatusInternalServerError)
		return
	}
	WriteJSON(w, status, data)
}

// WriteValidationError sends a 400 response with field-level validation details.
func WriteValidationError(w http.ResponseWriter, details map[string]string) {
	data, err := json.Marshal(ErrorResponse{
		Message:    "Validation failed",
		Code:       "VALIDATION_ERROR",
		StatusCode: http.StatusBadRequest,
		Details:    details,
	})
	if err != nil {
		slog.Error("failed to marshal validation error", "error", err)
		http.Error(w, `{"message":"validation failed","code":"MARSHAL_ERROR"}`, http.StatusInternalServerError)
		return
	}
	WriteJSON(w, http.StatusBadRequest, data)
}

// WriteInternalError logs the internal error details and sends a generic 500 response to the client.
func WriteInternalError(w http.ResponseWriter, logMsg string, err error) {
	slog.Error(logMsg, "error", err)
	WriteError(w, http.StatusInternalServerError, "An unexpected error occurred", "INTERNAL_ERROR")
}

// WriteMarshalError logs a marshaling failure and sends a 500 response to the client.
// Used by handlers when json.Marshal fails on a response payload.
func WriteMarshalError(w http.ResponseWriter, err error) {
	slog.Error("failed to marshal response", "error", err)
	WriteError(w, http.StatusInternalServerError, "An unexpected error occurred", "INTERNAL_ERROR")
}

// WriteStatusOK sends a {"status":"ok"} response with HTTP 200.
// Convenience function to eliminate repeated marshal-and-write of StatusResponse.
func WriteStatusOK(w http.ResponseWriter) {
	data, err := json.Marshal(StatusResponse{Status: "ok"})
	if err != nil {
		WriteMarshalError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, data)
}

// WriteSuccessOK sends a {"ok":true} response with HTTP 200.
// Convenience function to eliminate repeated marshal-and-write of SuccessResponse.
func WriteSuccessOK(w http.ResponseWriter) {
	data, err := json.Marshal(SuccessResponse{OK: true})
	if err != nil {
		WriteMarshalError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, data)
}
