package client

import "fmt"

// APIError represents an HTTP error returned by an AI provider API.
type APIError struct {
	StatusCode int
	Body       string
	Op         string // operation, e.g. "chat", "stream"
}

func (e *APIError) Error() string {
	return fmt.Sprintf("client: %s: status %d: %s", e.Op, e.StatusCode, e.Body)
}

// ParseError represents a response parsing error from a provider.
type ParseError struct {
	Provider string
	Err      error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s: parse response failed: %v", e.Provider, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// AuthError represents an authentication-related error.
type AuthError struct {
	Op  string
	Err error
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("auth: %s: %v", e.Op, e.Err)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}
