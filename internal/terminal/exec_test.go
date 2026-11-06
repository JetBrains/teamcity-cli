package terminal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecExitCode(t *testing.T) {
	for _, tt := range []struct {
		status   string
		code     int
		complete bool
	}{
		{"0\n", 0, true}, {"42\n", 42, true}, {"True\n", 0, true}, {"False\n", 1, true},
		{"2", 0, false}, {"\n", 0, false}, {"$?\n", 0, false}, {"256\n", 256, false},
	} {
		t.Run(strings.TrimSpace(tt.status), func(t *testing.T) {
			code, complete := execExitCode(execMarker+"\noutput"+execMarker+":"+tt.status, execMarker)
			assert.Equal(t, tt.complete, complete)
			if complete {
				assert.Equal(t, tt.code, code)
			}
		})
	}
}

func TestExecCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// No prompt: Exec must stop waiting when its context ends.
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	for _, deadline := range []bool{false, true} {
		ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		require.NoError(t, err)
		conn := &Conn{conn: ws, done: make(chan struct{})}
		ctx, cancel := context.WithCancel(t.Context())
		want := context.Canceled
		if deadline {
			ctx, cancel = context.WithTimeout(t.Context(), time.Millisecond)
			want = context.DeadlineExceeded
		} else {
			cancel()
		}
		err = conn.Exec(ctx, "unused")
		cancel()
		assert.True(t, errors.Is(err, want), "got %v", err)
	}
}

func TestExecDoesNotDispatchAfterDeadline(t *testing.T) {
	messages := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("$ "))
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				close(messages)
				return
			}
			messages <- string(msg)
		}
	}))
	defer server.Close()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	conn := &Conn{conn: ws, done: make(chan struct{})}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, conn.Exec(ctx, "must-not-run"), context.DeadlineExceeded)
	for message := range messages {
		t.Errorf("dispatched after deadline: %q", message)
	}
}
