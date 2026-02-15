package errors

import (
	"errors"
	"fmt"
)

// Error provides structured error information.
// Users can extract this with errors.As to get detailed context.
type Error struct {
	Op      string         // Operation that failed (e.g., "Client.Put", "Schema.New")
	Err     error          // Underlying error. Check with errors.Is or errors.As
	Field   string         // Field name (for validation errors)
	Details map[string]any // Additional details
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: field %s: %v", e.Op, e.Field, e.Err)
	}
	if len(e.Details) > 0 {
		return fmt.Sprintf("%s: %v (details %+v)", e.Op, e.Err, e.Details)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

type ValidationErrors struct {
	Op     string
	Errors []*FieldError
}

type FieldError struct {
	Field   string
	Message string
	Value   any
}

func (ve *ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return fmt.Sprintf("%s: validation failed", ve.Op)
	}

	msg := fmt.Sprintf("%s: %d validation error(s):\n", ve.Op, len(ve.Errors))
	for _, fe := range ve.Errors {
		msg += fmt.Sprintf("\t- %s: %s\n", fe.Field, fe.Message)
	}
	return msg
}

func (ve *ValidationErrors) Add(field, message string, value any) {
	ve.Errors = append(ve.Errors, &FieldError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.Errors) > 0
}

func Errorf(op string, err error, format string, args ...any) *Error {
	return &Error{
		Op:  op,
		Err: fmt.Errorf(format+": %v", append(args, err)...),
	}
}

func ValidationError(op, field string, err error) *Error {
	return &Error{
		Op:    op,
		Field: field,
		Err:   fmt.Errorf("%w: %v", ErrValidation, err),
	}
}

func ConfigError(op string, message string) *Error {
	return &Error{
		Op:  op,
		Err: fmt.Errorf("%w: %s", ErrInvalidConfig, message),
	}
}

var (
	ErrInvalidConfig       = errors.New("invalid configuration")
	ErrInvalidSchema       = errors.New("invalid schema")
	ErrValidation          = errors.New("validation error")
	ErrSession             = errors.New("session error")
	ErrEncryption          = errors.New("encryption error")
	ErrNetwork             = errors.New("network error")
	ErrNotFound            = errors.New("not found")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrRateLimited         = errors.New("rate limited")
	ErrCancelledOrTimedOut = errors.New("operation cancelled or timed out")
	ErrVersionConflict     = errors.New("version conflict")
	ErrMaxRetriesExceeded  = errors.New("max retries exceeded")
)
