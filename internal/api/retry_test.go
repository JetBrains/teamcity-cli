package api

import (
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

	retryable := []int{500, 502, 503, 504, 507, 508, 509, 510, 511}
	notRetryable := []int{200, 201, 204, 400, 401, 403, 404, 409, 422, 501}

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

func TestRetryableError(T *testing.T) {
	T.Parallel()

	inner := errors.New("connection refused")
	re := retryableError{err: inner}

	T.Run("Error delegates to wrapped error", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "connection refused", re.Error())
	})

	T.Run("Unwrap returns inner error", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, inner, re.Unwrap())
		assert.True(t, errors.Is(re, inner))
	})
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

	resp, err := withRetry(fastRetry, func() (*http.Response, error) {
		return http.Get(server.URL)
	})

	require.NoError(T, err)
	assert.Equal(T, http.StatusOK, resp.StatusCode)
	assert.Equal(T, int32(3), atomic.LoadInt32(&attempts))
	resp.Body.Close()
}

func TestWithRetry_RetriesOnNetworkError(T *testing.T) {
	T.Parallel()

	var attempts int32
	cfg := RetryConfig{MaxRetries: 2, Interval: 10 * time.Millisecond}

	withRetry(cfg, func() (*http.Response, error) {
		atomic.AddInt32(&attempts, 1)
		return http.Get("http://127.0.0.1:1") // connection refused
	})

	assert.Equal(T, int32(3), atomic.LoadInt32(&attempts))
}

// Client integration: verify get- / post-behavior
func TestClientRetryBehavior(T *testing.T) {
	T.Parallel()

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
		err := client.get("/app/rest/server", &result)

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
		client.post("/app/rest/buildQueue", nil, nil)

		assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
	})
}
