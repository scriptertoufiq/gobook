// Package apperror gives the service layer a way to express *what went wrong*
// without importing net/http or knowing anything about Gin. Controllers
// translate these into status codes at the edge.
package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	err     error  // wrapped cause, never exposed to clients
}

func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.err }

// Wrap attaches an underlying cause for logging.
func (e *Error) Wrap(err error) *Error {
	clone := *e
	clone.err = err
	return &clone
}

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func BadRequest(msg string) *Error {
	return New(http.StatusBadRequest, "bad_request", msg)
}

func Unauthorized(msg string) *Error {
	return New(http.StatusUnauthorized, "unauthorized", msg)
}

func Forbidden(msg string) *Error {
	return New(http.StatusForbidden, "forbidden", msg)
}

func NotFound(resource string) *Error {
	return New(http.StatusNotFound, "not_found", fmt.Sprintf("%s not found", resource))
}

func Conflict(msg string) *Error {
	return New(http.StatusConflict, "conflict", msg)
}

func Validation(msg string) *Error {
	return New(http.StatusUnprocessableEntity, "validation_failed", msg)
}

func TooManyRequests(msg string) *Error {
	return New(http.StatusTooManyRequests, "rate_limited", msg)
}

func Internal(err error) *Error {
	return New(http.StatusInternalServerError, "internal_error", "Something went wrong").Wrap(err)
}

// From normalises any error into an *Error so the response layer has
// exactly one shape to render. Unknown errors become 500s and the raw
// message is deliberately swallowed.
func From(err error) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal(err)
}
