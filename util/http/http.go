// Package http provides shared HTTP transport and error types.
package http

import (
	"fmt"
	"net/http"
	"time"
)

// SharedTransport is the process-wide shared *http.Transport for connection
// pooling across LLM clients and tool HTTP requests. Treat as read-only —
// do not mutate its fields; use NewSharedTransport() for a custom instance.
var SharedTransport = NewSharedTransport()

// NewSharedTransport returns a fresh *http.Transport with sensible pooling
// defaults. Use it when you need custom configuration (e.g. timeouts).
func NewSharedTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}

// HTTPError is a typed error carrying the HTTP status code and response body.
// Use errors.As to extract the status code in retry logic instead of parsing
// formatted error strings.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("API error (HTTP %d): %s", e.StatusCode, e.Body)
}

// NewHTTPError creates an HTTPError.
func NewHTTPError(statusCode int, body string) *HTTPError {
	return &HTTPError{StatusCode: statusCode, Body: body}
}
