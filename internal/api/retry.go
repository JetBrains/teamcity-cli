package api

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// RetryConfig defines retry behavior for API operations.
type RetryConfig struct {
	MaxRetries uint64
	Interval   time.Duration
}

// Predefined retry configurations for different operation types.
var (
	// ReadRetry is the default for idempotent read operations (GET).
	// Retries on network errors and 5xx responses.
	ReadRetry = RetryConfig{MaxRetries: 3, Interval: 200 * time.Millisecond}

	// NoRetry disables retries. Use for non-idempotent operations (POST to queue, etc.).
	NoRetry = RetryConfig{MaxRetries: 0}

	// LongRetry is for operations that may need more time to succeed
	// (e.g., waiting for resources to propagate).
	LongRetry = RetryConfig{MaxRetries: 3, Interval: 1 * time.Second}
)

// retryableError wraps an error to indicate it should be retried.
type retryableError struct {
	err error
}

func (e retryableError) Error() string {
	return e.err.Error()
}

func (e retryableError) Unwrap() error {
	return e.err
}

// withRetry executes an HTTP operation with retry logic based on the config.
// The operation function should return the response and any error.
// Retries occur on:
// - Network errors (timeouts, connection refused, DNS failures)
// - 5xx server errors (502, 503, 504, etc.)
// Does NOT retry on:
// - 4xx client errors (these indicate a problem with the request itself)
// - Successful responses (2xx)
func withRetry(cfg RetryConfig, op func() (*http.Response, error)) (*http.Response, error) {
	if cfg.MaxRetries == 0 {
		return op()
	}

	bo := backoff.NewConstantBackOff(cfg.Interval)

	var lastResp *http.Response
	_, err := backoff.RetryWithData(func() (struct{}, error) {
		resp, err := op()
		lastResp = resp

		if err != nil {
			if isRetryableNetworkError(err) {
				return struct{}{}, retryableError{err}
			}
			// Non-retryable error (e.g., request creation failed)
			return struct{}{}, backoff.Permanent(err)
		}

		if isRetryableStatusCode(resp.StatusCode) {
			return struct{}{}, retryableError{
				fmt.Errorf("server returned %d", resp.StatusCode),
			}
		}

		return struct{}{}, nil
	}, backoff.WithMaxRetries(bo, cfg.MaxRetries))

	if err != nil {
		// Unwrap a retryable error to return the original
		var re retryableError
		if errors.As(err, &re) {
			return lastResp, re.err
		}
		return lastResp, err
	}

	return lastResp, nil
}

// isRetryableNetworkError checks if an error is a transient network issue.
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for connection errors (refused, reset, DNS)
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

// isRetryableStatusCode returns true for server errors that may be transient.
func isRetryableStatusCode(code int) bool {
	switch code {
	case http.StatusInternalServerError, // 500
		http.StatusBadGateway,                    // 502
		http.StatusServiceUnavailable,            // 503
		http.StatusGatewayTimeout,                // 504
		http.StatusInsufficientStorage,           // 507
		http.StatusLoopDetected,                  // 508
		509,                                      // Bandwidth Limit Exceeded (non-standard)
		http.StatusNotExtended,                   // 510
		http.StatusNetworkAuthenticationRequired: // 511
		return true
	}
	return false
}
