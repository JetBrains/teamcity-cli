package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v5"
)

// RetryConfig defines retry behavior for API operations.
type RetryConfig struct {
	MaxRetries uint
	Interval   time.Duration
}

// Predefined retry configurations for different operation types.
var (
	// ReadRetry is the default for idempotent read operations (GET).
	// Retries on network errors, 429, and 5xx responses.
	ReadRetry = RetryConfig{MaxRetries: 3, Interval: 200 * time.Millisecond}

	// NoRetry disables retries. Use for non-idempotent operations (POST to queue, etc.).
	NoRetry = RetryConfig{MaxRetries: 0}

	// LongRetry is for operations that may need more time to succeed
	// (e.g., waiting for resources to propagate).
	LongRetry = RetryConfig{MaxRetries: 3, Interval: 1 * time.Second}
)

// withRetry retries op on network errors, 429, and 5xx (except 501/505), honoring Retry-After and ctx cancellation.
func withRetry(ctx context.Context, cfg RetryConfig, op func() (*http.Response, error)) (*http.Response, error) {
	if cfg.MaxRetries == 0 {
		return op()
	}

	expo := backoff.NewExponentialBackOff()
	expo.InitialInterval = cfg.Interval
	expo.MaxInterval = 30 * time.Second

	return backoff.Retry(ctx, func() (*http.Response, error) {
		resp, err := op()
		if err != nil {
			if isRetryableNetworkError(err) {
				return resp, err
			}
			return resp, backoff.Permanent(err)
		}
		if !isRetryableStatusCode(resp.StatusCode) {
			return resp, nil
		}
		if d := retryAfter(resp); d > 0 {
			return resp, &backoff.RetryAfterError{Duration: d}
		}
		return resp, fmt.Errorf("server returned %d", resp.StatusCode)
	}, backoff.WithBackOff(expo), backoff.WithMaxTries(cfg.MaxRetries+1))
}

// isRetryableNetworkError reports whether err is a transient network issue (not ctx cancellation).
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
		return true
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}
	_, ok := errors.AsType[*net.DNSError](err)
	return ok
}

// isRetryableStatusCode returns true for server errors that may be transient.
func isRetryableStatusCode(code int) bool {
	if code == http.StatusTooManyRequests {
		return true
	}
	if code < 500 {
		return false
	}
	// 501 Not Implemented and 505 HTTP Version Not Supported are permanent.
	return code != http.StatusNotImplemented && code != http.StatusHTTPVersionNotSupported
}

// retryAfter returns the delay requested by the Retry-After header (seconds or HTTP-date).
func retryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}
