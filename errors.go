package stromboli

import (
	"errors"
	"fmt"
)

// Error represents an error returned by the Stromboli API.
//
// Error implements the standard error interface and supports
// error wrapping via the Unwrap method. Use [errors.As] to
// check for specific error types:
//
//	result, err := client.Run(ctx, req)
//	if err != nil {
//	    var apiErr *stromboli.Error
//	    if errors.As(err, &apiErr) {
//	        fmt.Printf("API error %s: %s\n", apiErr.Code, apiErr.Message)
//	    }
//	}
//
// Common error codes include:
//   - NOT_FOUND: The requested resource does not exist
//   - TIMEOUT: The request timed out
//   - UNAUTHORIZED: Invalid or missing authentication
//   - BAD_REQUEST: Invalid request parameters
//   - INTERNAL: Internal server error
type Error struct {
	// Code is a machine-readable error code.
	// Common values: NOT_FOUND, TIMEOUT, UNAUTHORIZED, BAD_REQUEST, INTERNAL.
	Code string

	// Message is a human-readable error description.
	Message string

	// Status is the HTTP status code returned by the API.
	// Zero if the error occurred before receiving a response.
	Status int

	// Cause is the underlying error, if any.
	// Use errors.Unwrap or errors.Is to inspect the cause chain.
	Cause error
}

// Error returns a string representation of the error.
//
// The format is "stromboli: CODE: message" or
// "stromboli: CODE: message: cause" if there is an underlying error.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("stromboli: %s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("stromboli: %s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error cause.
//
// This allows using [errors.Is] and [errors.As] to inspect
// the error chain.
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is reports whether the target error matches this error.
//
// Two errors match if they have the same Code. This allows
// using [errors.Is] with sentinel errors:
//
//	if errors.Is(err, stromboli.ErrNotFound) {
//	    // Handle not found
//	}
func (e *Error) Is(target error) bool {
	var t *Error
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// Sentinel errors for common error conditions.
//
// Use [errors.Is] to check for these errors:
//
//	if errors.Is(err, stromboli.ErrNotFound) {
//	    fmt.Println("Resource not found")
//	}
var (
	// ErrNotFound indicates the requested resource does not exist.
	// HTTP status: 404.
	ErrNotFound = &Error{
		Code:    "NOT_FOUND",
		Message: "resource not found",
		Status:  404,
	}

	// ErrTimeout indicates the request timed out.
	// This can occur for long-running operations or network issues.
	// HTTP status: 408.
	ErrTimeout = &Error{
		Code:    "TIMEOUT",
		Message: "request timed out",
		Status:  408,
	}

	// ErrUnauthorized indicates invalid or missing authentication.
	// HTTP status: 401.
	ErrUnauthorized = &Error{
		Code:    "UNAUTHORIZED",
		Message: "invalid credentials",
		Status:  401,
	}

	// ErrBadRequest indicates invalid request parameters.
	// Check the error message for details about what was invalid.
	// HTTP status: 400.
	ErrBadRequest = &Error{
		Code:    "BAD_REQUEST",
		Message: "invalid request",
		Status:  400,
	}

	// ErrInternal indicates an internal server error.
	// This usually indicates a bug in the Stromboli server.
	// HTTP status: 500.
	ErrInternal = &Error{
		Code:    "INTERNAL",
		Message: "internal server error",
		Status:  500,
	}

	// ErrUnavailable indicates the service is temporarily unavailable.
	// Retry the request after a short delay.
	// HTTP status: 503.
	ErrUnavailable = &Error{
		Code:    "UNAVAILABLE",
		Message: "service temporarily unavailable",
		Status:  503,
	}
)

// newError creates a new Error with the given parameters.
// This is an internal helper for creating errors from API responses.
func newError(code, message string, status int, cause error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Status:  status,
		Cause:   cause,
	}
}

// wrapError wraps an error with additional context.
// If err is already an *Error, it returns a new Error with the original as cause.
// Otherwise, it creates a new Error with the provided code and message.
func wrapError(err error, code, message string, status int) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Status:  status,
		Cause:   err,
	}
}
