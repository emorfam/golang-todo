package apierror

import "net/http"

// APIError is a typed error that carries an HTTP status code and a user-safe message.
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string { return e.Message }

var (
	ErrNotFound     = &APIError{Code: http.StatusNotFound, Message: "resource not found"}
	ErrUnauthorized = &APIError{Code: http.StatusUnauthorized, Message: "unauthorized"}
	ErrForbidden    = &APIError{Code: http.StatusForbidden, Message: "forbidden"}
)

func ErrBadRequest(msg string) *APIError {
	return &APIError{Code: http.StatusBadRequest, Message: msg}
}

func ErrInternal(msg string) *APIError {
	return &APIError{Code: http.StatusInternalServerError, Message: msg}
}
