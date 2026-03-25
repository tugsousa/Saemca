package common

import "net/http"

// AppError is a domain error that carries an HTTP status code and a machine-readable
// error code. Services return these; handlers translate them to HTTP responses.
//
// Example:
//
//	return nil, common.NotFound("PROFILE_NOT_FOUND", "profile does not exist")
type AppError struct {
	Code    string
	Message string
	Status  int
}

func (e *AppError) Error() string {
	return e.Message
}

// — Constructors — one per HTTP status family used in this app —

func NotFound(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Status: http.StatusNotFound}
}

func Unauthorized(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Status: http.StatusUnauthorized}
}

func Forbidden(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Status: http.StatusForbidden}
}

func BadRequest(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Status: http.StatusBadRequest}
}

func Conflict(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Status: http.StatusConflict}
}

func UnprocessableEntity(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Status: http.StatusUnprocessableEntity}
}

func Internal(code, message string) *AppError {
	return &AppError{Code: code, Message: message, Status: http.StatusInternalServerError}
}
