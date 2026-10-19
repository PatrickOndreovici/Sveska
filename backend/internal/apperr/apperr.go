// Package apperr defines the errors services return to callers.
//
// Services describe what went wrong (a Kind plus a stable Code); the HTTP
// layer decides how that maps to a status code. Anything that is not an
// *Error is treated as an unexpected internal failure.
package apperr

import "fmt"

// Kind classifies an error for the transport layer.
type Kind string

const (
	// Invalid means the input failed validation. Fields says which inputs and why.
	Invalid Kind = "invalid"
	// Unauthenticated means the caller is not logged in or used bad credentials.
	Unauthenticated Kind = "unauthenticated"
	// Conflict means the request clashes with existing state, such as a taken email.
	Conflict Kind = "conflict"
)

// Error is an expected, user-facing failure.
type Error struct {
	Kind    Kind
	Code    string            // stable, machine-readable, e.g. "email_taken"
	Message string            // human-readable, safe to show to end users
	Fields  map[string]string // per-field problems, only for Kind Invalid
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// New creates an Error without field details.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// Validation creates an Invalid error from per-field messages.
func Validation(fields map[string]string) *Error {
	return &Error{
		Kind:    Invalid,
		Code:    "validation_failed",
		Message: "Some fields are missing or invalid.",
		Fields:  fields,
	}
}
