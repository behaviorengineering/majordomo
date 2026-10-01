// Package errors defines stable failures for operation entrypoints.
package errors

import "fmt"

// Code identifies the class of an operation failure.
type Code string

const (
	// CodeInvalidArgument identifies invalid operator input.
	CodeInvalidArgument Code = "invalid_argument"
	// CodeConfiguration identifies missing or invalid repository configuration.
	CodeConfiguration Code = "configuration"
	// CodeExecution identifies a failed external operation.
	CodeExecution Code = "execution"
	// CodeInternal identifies an unexpected operation failure.
	CodeInternal Code = "internal"
)

// Error carries a stable code and operation while preserving the cause.
type Error struct {
	Code    Code
	Op      string
	Message string
	Cause   error
}

// Error returns the operator-facing error message.
func (e *Error) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Cause)
}

// Unwrap returns the underlying cause for errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a typed operation error without an underlying cause.
func New(code Code, op, message string) error {
	return &Error{Code: code, Op: op, Message: message}
}

// Wrap creates a typed operation error around a cause.
func Wrap(err error, code Code, op, message string) error {
	if err == nil {
		return nil
	}
	return &Error{
		Code:    code,
		Op:      op,
		Message: message,
		Cause:   err,
	}
}
