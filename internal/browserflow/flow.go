// Package browserflow runs a browser-driven authorization flow over a local HTTP listener.
package browserflow

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"

	"github.com/pkg/browser"
)

//go:embed templates/callback.html
var callbackHTML string
var callbackTmpl = template.Must(template.New("callback").Parse(callbackHTML))

// openBrowser is overridable in tests to avoid spawning real browser tabs.
var openBrowser = browser.OpenURL

// DefaultCallbackPath is the conventional path for the OAuth-style callback handler.
const DefaultCallbackPath = "/callback"

// DefaultTimeout is the wall-clock limit on a flow.
const DefaultTimeout = 5 * time.Minute

// Logger is a small interface for status updates during a flow; *output.Printer satisfies it.
type Logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
}

// Options configure a single browser-driven flow run.
type Options struct {
	Listener     net.Listener  // ownership transfers to Run; closed on return
	State        string        // expected state value; required
	OpenURL      string        // URL the browser is sent to; required
	StartHandler http.Handler  // optional; mounted at /start so OpenURL can be the local listener
	CallbackPath string        // path that captures code+state; defaults to DefaultCallbackPath
	Timeout      time.Duration // 0 → DefaultTimeout
	Logger       Logger        // optional
	SuccessTitle string        // shown on the callback page; defaults to "Authentication successful"
	SuccessBody  string        // shown on the callback page; defaults to a generic close-tab message
	ErrorTitle   string        // shown on the callback page when GitHub/upstream returns an error
}

// Result holds the validated callback code; state is checked inside Run and not exposed.
type Result struct {
	Code string
}

// FindAvailableListener binds to a free port on 127.0.0.1 only.
func FindAvailableListener() (net.Listener, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen on loopback: %w", err)
	}
	return l, nil
}

// GenerateState returns a 16-byte random URL-safe string suitable for OAuth state / CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Run hosts opts.Listener, opens opts.OpenURL, waits for opts.CallbackPath, validates state in constant time, and closes the listener on return.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Listener == nil {
		return nil, errors.New("browserflow: Listener is required")
	}
	defer func() { _ = opts.Listener.Close() }() // safe even after srv.Shutdown closes it
	if opts.State == "" {
		return nil, errors.New("browserflow: State is required")
	}
	if opts.OpenURL == "" {
		return nil, errors.New("browserflow: OpenURL is required")
	}

	cbPath := opts.CallbackPath
	if cbPath == "" {
		cbPath = DefaultCallbackPath
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	successTitle := opts.SuccessTitle
	if successTitle == "" {
		successTitle = "Authentication successful"
	}
	successBody := opts.SuccessBody
	if successBody == "" {
		successBody = "You can close this window and return to the terminal."
	}
	errorTitle := opts.ErrorTitle
	if errorTitle == "" {
		errorTitle = "Authentication failed"
	}

	type rawResult struct {
		code, state, errMsg string
	}
	resultCh := make(chan rawResult, 1)

	mux := http.NewServeMux()
	if opts.StartHandler != nil {
		mux.Handle("/start", opts.StartHandler)
	}
	mux.HandleFunc(cbPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		raw := rawResult{code: q.Get("code"), state: q.Get("state"), errMsg: q.Get("error")}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'")

		data := struct {
			Error, ErrorTitle, SuccessTitle, SuccessBody string
		}{ErrorTitle: errorTitle, SuccessTitle: successTitle, SuccessBody: successBody}
		if raw.errMsg != "" {
			data.Error = raw.errMsg
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = callbackTmpl.Execute(w, data)

		select {
		case resultCh <- raw:
		default:
		}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(opts.Listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := openBrowser(opts.OpenURL); err != nil {
		if opts.Logger != nil {
			opts.Logger.Warn("Could not open browser automatically: %v", err)
			opts.Logger.Info("Open this URL in your browser:\n  %s", opts.OpenURL)
		}
	}

	select {
	case raw := <-resultCh:
		if raw.errMsg != "" {
			return nil, fmt.Errorf("authorization denied: %s", raw.errMsg)
		}
		if subtle.ConstantTimeCompare([]byte(raw.state), []byte(opts.State)) != 1 {
			return nil, errors.New("state mismatch: possible CSRF attack")
		}
		if raw.code == "" {
			return nil, errors.New("callback received without code")
		}
		return &Result{Code: raw.code}, nil
	case err := <-serverErr:
		return nil, fmt.Errorf("local callback server failed: %w", err)
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for callback (exceeded %v)", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
