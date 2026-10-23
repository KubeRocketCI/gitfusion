package errors

import (
	"errors"
	"fmt"
)

// InvalidParamError reports a request parameter the server cannot use.
// Param is the parameter name from the OpenAPI spec; Err is the cause and must not be nil.
type InvalidParamError struct {
	Param string
	Err   error
}

// Error implements the error interface.
func (e *InvalidParamError) Error() string {
	return fmt.Sprintf("invalid parameter %s: %v", e.Param, e.Err)
}

// Unwrap returns the cause.
func (e *InvalidParamError) Unwrap() error {
	return e.Err
}

// UnsupportedOperation returns an error matching ErrUnsupportedOperation that names the git provider and the
// OpenAPI operationId.
func UnsupportedOperation(gitProvider, operation string) error {
	return fmt.Errorf("provider %s does not support %s: %w", gitProvider, operation, ErrUnsupportedOperation)
}

// Predefined errors
var (
	ErrNotFound = errors.New("not found")

	// ErrUnauthorized marks a git provider rejecting the token stored for the GitServer (401/403).
	ErrUnauthorized = errors.New("unauthorized")

	// ErrUnsupportedOperation marks an operation the git server's provider does not implement.
	ErrUnsupportedOperation = errors.New("unsupported operation")
)
