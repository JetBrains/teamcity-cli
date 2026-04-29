package browserflow

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	openBrowser = func(string) error { return nil }
	os.Exit(m.Run())
}

func TestFindAvailableListener(t *testing.T) {
	t.Parallel()

	listener, err := FindAvailableListener()
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	addr := listener.Addr().(*net.TCPAddr)
	assert.Greater(t, addr.Port, 0)
	assert.Equal(t, "127.0.0.1", addr.IP.String())
}

func TestGenerateState(t *testing.T) {
	t.Parallel()

	urlSafe := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

	s1, err := GenerateState()
	require.NoError(t, err)
	require.NotEmpty(t, s1)
	assert.True(t, urlSafe.MatchString(s1))
	assert.NotContains(t, s1, "=")

	s2, err := GenerateState()
	require.NoError(t, err)
	assert.NotEqual(t, s1, s2)
}

// hitCallback simulates the browser bouncing back to the callback path.
func hitCallback(t *testing.T, port int, path, query string) {
	t.Helper()
	go func() {
		time.Sleep(50 * time.Millisecond)
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s?%s", port, path, query))
		if err != nil {
			t.Logf("hit callback: %v", err)
			return
		}
		_ = resp.Body.Close()
	}()
}

func TestRunCapturesCode(t *testing.T) {
	t.Parallel()

	listener, err := FindAvailableListener()
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port

	hitCallback(t, port, DefaultCallbackPath, "code=abc123&state=expected-state")

	result, err := Run(t.Context(), Options{
		Listener:     listener,
		State:        "expected-state",
		OpenURL:      "http://127.0.0.1:" + fmt.Sprint(port) + "/__noop", // browser open is best-effort; OK to "fail"
		CallbackPath: DefaultCallbackPath,
		Timeout:      2 * time.Second,
	})
	require.NoError(t, err)
	assert.Equal(t, "abc123", result.Code)
}

func TestRunRejectsStateMismatch(t *testing.T) {
	t.Parallel()

	listener, err := FindAvailableListener()
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port

	hitCallback(t, port, DefaultCallbackPath, "code=abc&state=wrong-state")

	_, err = Run(t.Context(), Options{
		Listener: listener,
		State:    "expected-state",
		OpenURL:  "http://127.0.0.1:" + fmt.Sprint(port) + "/__noop",
		Timeout:  2 * time.Second,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state mismatch")
}

func TestRunSurfacesUpstreamError(t *testing.T) {
	t.Parallel()

	listener, err := FindAvailableListener()
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port

	hitCallback(t, port, DefaultCallbackPath, "error=access_denied")

	_, err = Run(t.Context(), Options{
		Listener: listener,
		State:    "expected-state",
		OpenURL:  "http://127.0.0.1:" + fmt.Sprint(port) + "/__noop",
		Timeout:  2 * time.Second,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access_denied")
}

func TestRunMountsStartHandler(t *testing.T) {
	t.Parallel()

	listener, err := FindAvailableListener()
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port

	startHit := make(chan struct{}, 1)
	startHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case startHit <- struct{}{}:
		default:
		}
		_, _ = w.Write([]byte("started"))
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/start", port))
		if err == nil {
			_ = resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
		resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/cb?code=ok&state=s", port))
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	result, err := Run(t.Context(), Options{
		Listener:     listener,
		State:        "s",
		OpenURL:      "http://127.0.0.1:" + fmt.Sprint(port) + "/__noop",
		StartHandler: startHandler,
		CallbackPath: "/cb",
		Timeout:      2 * time.Second,
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", result.Code)
	select {
	case <-startHit:
	case <-time.After(time.Second):
		t.Fatal("StartHandler was never called")
	}
}

func TestRunTimesOut(t *testing.T) {
	t.Parallel()

	listener, err := FindAvailableListener()
	require.NoError(t, err)

	_, err = Run(t.Context(), Options{
		Listener: listener,
		State:    "s",
		OpenURL:  "http://127.0.0.1:1/__noop",
		Timeout:  100 * time.Millisecond,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestRunCancelsOnContext(t *testing.T) {
	t.Parallel()

	listener, err := FindAvailableListener()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()

	_, err = Run(ctx, Options{
		Listener: listener,
		State:    "s",
		OpenURL:  "http://127.0.0.1:1/__noop",
		Timeout:  5 * time.Second,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRunValidatesOptions(t *testing.T) {
	t.Parallel()

	_, err := Run(t.Context(), Options{State: "s", OpenURL: "x"})
	assert.ErrorContains(t, err, "Listener")

	for _, tc := range []struct {
		name string
		opts func(net.Listener) Options
		want string
	}{
		{"missing State", func(l net.Listener) Options { return Options{Listener: l, OpenURL: "x"} }, "State"},
		{"missing OpenURL", func(l net.Listener) Options { return Options{Listener: l, State: "s"} }, "OpenURL"},
	} {
		listener, lerr := FindAvailableListener()
		require.NoError(t, lerr)
		_, err := Run(t.Context(), tc.opts(listener))
		assert.ErrorContains(t, err, tc.want, tc.name)
	}
}
