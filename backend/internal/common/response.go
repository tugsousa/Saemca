package common

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// Response is the standard JSON envelope for every API response.
// Matches the contract defined in docs/technical_design.md Section 2.
type Response struct {
	TraceID string    `json:"trace_id"`
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Meta    *Meta     `json:"meta,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// Meta is included on paginated responses only.
type Meta struct {
	Page  int `json:"page"`
	Total int `json:"total"`
}

// APIError is the error object returned when Success is false.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	TraceID string `json:"trace_id"`
}

// WriteSuccess writes a 2xx response with data payload.
func WriteSuccess(w http.ResponseWriter, r *http.Request, status int, data any) {
	write(w, status, Response{
		TraceID: traceID(r),
		Success: true,
		Data:    data,
	})
}

// WritePaginated writes a 200 response with data and pagination metadata.
func WritePaginated(w http.ResponseWriter, r *http.Request, data any, page, total int) {
	write(w, http.StatusOK, Response{
		TraceID: traceID(r),
		Success: true,
		Data:    data,
		Meta:    &Meta{Page: page, Total: total},
	})
}

// WriteError writes an error response. Use the AppError helpers below for
// standard cases, or call this directly with a custom status code.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	id := traceID(r)
	write(w, status, Response{
		TraceID: id,
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			TraceID: id,
		},
	})
}

// WriteAppError writes an AppError — the standard way to propagate domain errors
// from the service layer up to the handler layer.
func WriteAppError(w http.ResponseWriter, r *http.Request, err *AppError) {
	WriteError(w, r, err.Status, err.Code, err.Message)
}

func traceID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}

func write(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Ignore the encode error — if we can't write the response, there's nothing
	// more we can do at this point.
	_ = json.NewEncoder(w).Encode(resp)
}
