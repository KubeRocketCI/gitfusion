package errors

import (
	"errors"
	"fmt"
)

// Error represents a structured error with a code and message.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Predefined errors
var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")

	ErrGitServerNotFound = &Error{
		Code:    "git_server_not_found",
		Message: "The Git server was not found.",
	}

	ErrSecretNotFound = &Error{
		Code:    "secret_not_found",
		Message: "The secret was not found.",
	}

	ErrBadRequest = &Error{
		Code:    "bad_request",
		Message: "The request could not be processed.",
	}
)
