// Package apierr defines the fixed set of error codes returned by the API.
package apierr

import (
	"fmt"
	"net/http"
)

type Code string

const (
	Validation        Code = "VALIDATION_ERROR"
	Unauthorized      Code = "UNAUTHORIZED"
	Forbidden         Code = "FORBIDDEN"
	NotFound          Code = "NOT_FOUND"
	Conflict          Code = "CONFLICT"
	QuotaExceeded     Code = "QUOTA_EXCEEDED"
	FileTooLarge      Code = "FILE_TOO_LARGE"
	UnsupportedFormat Code = "UNSUPPORTED_FORMAT"
	RateLimited       Code = "RATE_LIMITED"
	Unavailable       Code = "UNAVAILABLE"
	Internal          Code = "INTERNAL"
)

var statusByCode = map[Code]int{
	Validation:        http.StatusUnprocessableEntity,
	Unauthorized:      http.StatusUnauthorized,
	Forbidden:         http.StatusForbidden,
	NotFound:          http.StatusNotFound,
	Conflict:          http.StatusConflict,
	QuotaExceeded:     http.StatusTooManyRequests,
	FileTooLarge:      http.StatusRequestEntityTooLarge,
	UnsupportedFormat: http.StatusUnsupportedMediaType,
	RateLimited:       http.StatusTooManyRequests,
	Unavailable:       http.StatusServiceUnavailable,
	Internal:          http.StatusInternalServerError,
}

// Error is an error that is safe to show to the client.
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func (e *Error) Status() int {
	if s, ok := statusByCode[e.Code]; ok {
		return s
	}
	return http.StatusInternalServerError
}

func New(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
