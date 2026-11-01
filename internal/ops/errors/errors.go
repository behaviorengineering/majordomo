package errors

import "fmt"

// Code identifies a stable operator-facing failure category.
type Code string

const (
	CodeCanceled        Code = "canceled"
	CodeConflict        Code = "conflict"
	CodeInternal        Code = "internal"
	CodeInvalidArgument Code = "invalid_argument"
	CodeNotFound        Code = "not_found"
	CodeUnavailable     Code = "unavailable"
)

// Error carries a stable code and operation while preserving the underlying cause.
type Error struct {
	Code    Code
	Op      string
	Message string
	Cause   error
}

// Error returns the operator error message.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Cause)
}

// Unwrap returns the underlying cause for errors.Is and errors.As.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// New creates a categorized operator error without an underlying cause.
func New(code Code, op, message string) *Error {
	return &Error{Code: code, Op: op, Message: message}
}

// Wrap creates a categorized operator error that preserves its cause.
func Wrap(cause error, code Code, op, message string) *Error {
	return &Error{Code: code, Op: op, Message: message, Cause: cause}
}
