package stromboli

import "fmt"

// Error represents a Stromboli API error.
type Error struct {
	Code    string
	Message string
	Status  int
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("stromboli: %s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("stromboli: %s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// Sentinel errors.
var (
	ErrNotFound     = &Error{Code: "NOT_FOUND", Message: "resource not found", Status: 404}
	ErrTimeout      = &Error{Code: "TIMEOUT", Message: "request timed out", Status: 408}
	ErrUnauthorized = &Error{Code: "UNAUTHORIZED", Message: "invalid credentials", Status: 401}
	ErrBadRequest   = &Error{Code: "BAD_REQUEST", Message: "invalid request", Status: 400}
	ErrInternal     = &Error{Code: "INTERNAL", Message: "internal server error", Status: 500}
)
