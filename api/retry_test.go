package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fastRetry = RetryConfig{MaxRetries: 3, Interval: 10 * time.Millisecond}

// Unit tests for our decision logic (no servers needed)
func TestIsRetryableStatusCode(T *testing.T) {
	T.Parallel()

	retryable := []int{429, 500, 502, 503, 504, 507, 508, 509, 510, 511}
	notRetryable := []int{200, 201, 204, 400, 401, 403, 404, 409, 422, 501, 505}

	for _, code := range retryable {
		assert.True(T, isRetryableStatusCode(code), "%d should be retryable", code)
	}
	for _, code := range notRetryable {
		assert.False(T, isRetryableStatusCode(code), "%d should NOT be retryable", code)
	}
}

func TestIsRetryableNetworkError(T *testing.T) {
	T.Parallel()

	assert.False(T, isRetryableNetworkError(nil))
	assert.True(T, isRetryableNetworkError(&net.OpError{Op: "dial"}))
	assert.True(T, isRetryableNetworkError(&net.DNSError{Err: "no such host"}))
	assert.True(T, isRetryableNetworkError(&net.OpError{Op: "read", Err: timeoutErr{}}))
}

type timeoutErr struct{}

func (e timeoutErr) Error() string   { return "timeout" }
func (e timeoutErr) Timeout() bool   { return true }
func (e timeoutErr) Temporary() bool { return true }

// TestIsRetryableNetworkError_ContextCancellation pins that ctx deadline/cancel is not retried.
func TestIsRetryableNetworkError_ContextCancellation(T *testing.T) {
	T.Parallel()
	assert.False(T, isRetryableNetworkError(context.DeadlineExceeded))
	assert.False(T, isRetryableNetworkError(&net.OpError{Op: "read", Err: context.DeadlineExceeded}))
}

// Integration test: verify retry actually happens
func TestWithRetry_RetriesOnServerError(T *testing.T) {
	T.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	T.Cleanup(server.Close)
	client := server.Client()

	resp, err := withRetry(T.Context(), fastRetry, func() (*http.Response, error) {
		return client.Get(server.URL)
	})

	require.NoError(T, err)
	assert.Equal(T, http.StatusOK, resp.StatusCode)
	assert.Equal(T, int32(3), atomic.LoadInt32(&attempts))
	resp.Body.Close()
}

func TestRetryAfterSeconds(T *testing.T) {
	T.Parallel()
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "5")
	assert.Equal(T, 5*time.Second, retryAfter(resp))
}

func TestRetryAfterMissing(T *testing.T) {
	T.Parallel()
	resp := &http.Response{Header: http.Header{}}
	assert.Zero(T, retryAfter(resp))
	assert.Zero(T, retryAfter(nil))
}

func TestWithRetry_RetriesOn429(T *testing.T) {
	T.Parallel()

	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	T.Cleanup(server.Close)
	client := server.Client()

	resp, err := withRetry(T.Context(), fastRetry, func() (*http.Response, error) {
		return client.Get(server.URL)
	})

	require.NoError(T, err)
	assert.Equal(T, http.StatusOK, resp.StatusCode)
	assert.Equal(T, int32(2), atomic.LoadInt32(&attempts))
	resp.Body.Close()
}

// TestWithRetry_HonorsRetryAfter verifies Retry-After wins over the exponential schedule.
func TestWithRetry_HonorsRetryAfter(T *testing.T) {
	T.Parallel()

	var attempts int32
	var firstAt, secondAt time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			firstAt = time.Now()
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		secondAt = time.Now()
		w.WriteHeader(http.StatusOK)
	}))
	T.Cleanup(server.Close)
	client := server.Client()

	// InitialInterval 10ms — far less than the server's 1s hint, so any sleep
	// between 1.0s and 1.1s proves the Retry-After header won over exponential.
	resp, err := withRetry(T.Context(), RetryConfig{MaxRetries: 2, Interval: 10 * time.Millisecond}, func() (*http.Response, error) {
		return client.Get(server.URL)
	})

	require.NoError(T, err)
	assert.Equal(T, http.StatusOK, resp.StatusCode)
	elapsed := secondAt.Sub(firstAt)
	assert.GreaterOrEqual(T, elapsed, 900*time.Millisecond,
		"second attempt should wait for Retry-After hint; waited %v", elapsed)
	assert.Less(T, elapsed, 2*time.Second, "shouldn't wait excessively; waited %v", elapsed)
	resp.Body.Close()
}

// TestGetWithRetry_ContextCancelUnwraps pins that caller-canceled ctx surfaces bare, not as NetworkError.
func TestGetWithRetry_ContextCancelUnwraps(T *testing.T) {
	T.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	T.Cleanup(server.Close)
	c := NewClient(server.URL, "test-token")

	ctx, cancel := context.WithCancel(T.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	err := c.getWithRetry(ctx, "/app/rest/server", nil, NoRetry)

	require.Error(T, err)
	assert.True(T, errors.Is(err, context.Canceled), "expected context.Canceled, got %v", err)
	_, ok := errors.AsType[*NetworkError](err)
	assert.False(T, ok, "should not be wrapped as NetworkError")
}

// TestGetWithRetry_ExhaustionPreservesHTTPError pins that exhausted 429/5xx keeps the typed HTTP error.
func TestGetWithRetry_ExhaustionPreservesHTTPError(T *testing.T) {
	T.Parallel()

	client := setupTestServer(T, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"errors":[{"message":"maintenance window"}]}`))
	})

	var result Server
	err := client.getWithRetry(T.Context(), "/app/rest/server", &result, fastRetry)

	require.Error(T, err)
	he, ok := errors.AsType[*HTTPError](err)
	require.True(T, ok, "expected *HTTPError, got %T: %v", err, err)
	assert.Equal(T, http.StatusServiceUnavailable, he.Status)
	assert.Contains(T, err.Error(), "maintenance window")
}

func TestWithRetry_RetriesOnNetworkError(T *testing.T) {
	T.Parallel()

	var attempts int32
	cfg := RetryConfig{MaxRetries: 2, Interval: 10 * time.Millisecond}

	//nolint:errcheck // exercising retry behavior, not the return
	withRetry(T.Context(), cfg, func() (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return http.Get("http://127.0.0.1:1") // connection refused
	})

	assert.Equal(T, int32(3), atomic.LoadInt32(&attempts))
}

// Client integration: verify get- / post-behavior
func TestClientRetryBehavior(T *testing.T) {
	original := ReadRetry
	T.Cleanup(func() { ReadRetry = original })
	ReadRetry = fastRetry

	T.Run("get retries on 503", func(t *testing.T) {
		t.Parallel()

		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt32(&attempts, 1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Server{Version: "2024.1"})
		}))
		t.Cleanup(server.Close)

		client := NewClient(server.URL, "test-token")
		var result Server
		err := client.get(t.Context(), "/app/rest/server", &result)

		require.NoError(t, err)
		assert.Equal(t, "2024.1", result.Version)
		assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
	})

	T.Run("post never retries", func(t *testing.T) {
		t.Parallel()

		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		t.Cleanup(server.Close)

		client := NewClient(server.URL, "test-token")
		client.post(t.Context(), "/app/rest/buildQueue", nil, nil)

		assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
	})
}
