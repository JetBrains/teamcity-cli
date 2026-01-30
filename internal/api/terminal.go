package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/acarl005/stripansi"
	"github.com/gorilla/websocket"
	"github.com/moby/term"
)

const (
	// pingInterval is the interval between WebSocket ping messages.
	// 60s is conservative; most proxies/load balancers have 60-120s idle timeouts.
	pingInterval = 60 * time.Second

	// writeTimeout is the maximum time to wait for a WebSocket write to complete.
	// 10s is generous for small control messages; prevents hanging on network issues.
	writeTimeout = 10 * time.Second

	// execMarker is used to delimit command output.
	// Using unique string without shell-interpreted chars ($, `, !, etc.)
	execMarker = "__TC_EXEC_7f3a9e2b__"
)

type TerminalClient struct {
	baseURL    string
	username   string
	token      string
	httpClient *http.Client
}

func NewTerminalClient(baseURL, username, token string) *TerminalClient {
	jar, _ := cookiejar.New(nil)
	return &TerminalClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		token:    token,
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}
}

func (c *TerminalClient) OpenSession(agentID int) (*TerminalSession, error) {
	endpoint := fmt.Sprintf("%s/httpAuth/plugins/teamcity-agent-terminal/agentTerminal.html?id=%d", c.baseURL, agentID)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(""))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	username := c.username
	if username == "" {
		username = "token"
	}
	req.SetBasicAuth(username, c.token)

	output.Debug("> POST %s", endpoint)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, tcerrors.NetworkError(c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	output.Debug("< %s", resp.Status)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, tcerrors.AuthenticationFailed()
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, tcerrors.PermissionDenied("open terminal session")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, tcerrors.WithSuggestion(
			fmt.Sprintf("Failed to open terminal session: %s", strings.TrimSpace(string(body))),
			"Check if the agent-terminal plugin is installed on the server",
		)
	}

	var session TerminalSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("invalid response from server: %w", err)
	}

	if session.NodeID == "" {
		session.NodeID = resp.Header.Get("Teamcity-Node-Id")
	}

	return &session, nil
}

func (c *TerminalClient) Connect(session *TerminalSession, cols, rows int) (*TerminalConn, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}

	scheme := "wss"
	if u.Scheme == "http" {
		scheme = "ws"
	}

	wsURL := fmt.Sprintf("%s://%s/app/agentTerminal/terminal/%s?cols=%d&rows=%d",
		scheme, u.Host, session.Token, cols, rows)

	header := http.Header{}
	header.Set("Origin", c.baseURL)

	var cookies []string
	for _, cookie := range c.httpClient.Jar.Cookies(u) {
		cookies = append(cookies, cookie.Name+"="+cookie.Value)
	}
	if session.NodeID != "" {
		cookies = append(cookies, "X-TeamCity-Node-Id-Cookie="+session.NodeID)
	}
	if len(cookies) > 0 {
		header.Set("Cookie", strings.Join(cookies, "; "))
	}

	output.Debug("WebSocket URL: %s", wsURL)

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("WebSocket connection failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return nil, fmt.Errorf("WebSocket connection failed: %w", err)
	}

	return &TerminalConn{conn: conn, done: make(chan struct{})}, nil
}

type TerminalConn struct {
	conn      *websocket.Conn
	closeOnce sync.Once
	done      chan struct{}
	mu        sync.Mutex
	writeMu   sync.Mutex // serializes writes to WebSocket
	err       error
}

func (tc *TerminalConn) RunInteractive(ctx context.Context) error {
	stdin, stdout, _ := term.StdStreams()
	fd, isTerminal := term.GetFdInfo(stdin)
	if !isTerminal {
		return tcerrors.New("terminal command requires an interactive terminal")
	}

	defer tc.Close()

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	defer func() { _ = term.RestoreTerminal(fd, oldState) }()

	errChan := make(chan error, 2)
	go tc.copyToWriter(stdout, errChan)
	go tc.copyFromReader(stdin, errChan)

	sigChan, stopSig := resizeSignal()
	defer stopSig()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tc.done:
			tc.mu.Lock()
			err := tc.err
			tc.mu.Unlock()
			return err
		case err := <-errChan:
			return err
		case <-sigChan:
			tc.sendResize()
		case <-ticker.C:
			tc.sendPing()
		}
	}
}

func (tc *TerminalConn) Exec(ctx context.Context, command string) (string, error) {
	defer tc.Close()

	type readResult struct {
		data string
		err  error
	}
	resultCh := make(chan readResult, 1)
	readyCh := make(chan struct{}, 1)

	go func() {
		var buf strings.Builder
		signalledReady := false

		for {
			_, msg, err := tc.conn.ReadMessage()
			if err != nil {
				content := buf.String()
				output.Debug("terminal exec: connection closed, buffer len=%d", len(content))
				if len(content) > 0 {
					output.Debug("terminal exec: raw output:\n%s", content)
				}
				if buf.Len() > 0 {
					resultCh <- readResult{data: extractExecOutput(content)}
				} else if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					resultCh <- readResult{err: fmt.Errorf("connection error: %w", err)}
				} else {
					resultCh <- readResult{data: ""}
				}
				return
			}

			buf.Write(msg)
			content := normalizeLineEndings(buf.String())

			if !signalledReady {
				signalledReady = true
				output.Debug("terminal exec: shell ready, initial output: %q", content)
				select {
				case readyCh <- struct{}{}:
				default:
				}
			}

			markerCount := strings.Count(content, "\n"+execMarker)
			if markerCount >= 2 {
				output.Debug("terminal exec: found %d markers, extracting output", markerCount)
				resultCh <- readResult{data: extractExecOutput(content)}
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		return "", tcerrors.New("command timed out")
	case <-readyCh: // Shell is ready
	case <-time.After(500 * time.Millisecond): // Timeout waiting for initial output, try anyway
	}

	// Brief delay for shell to fully initialize after prompt appears
	time.Sleep(100 * time.Millisecond)

	// Send everything in one message: start marker, command, end marker, exit
	fullCmd := fmt.Sprintf("echo %s; %s; echo; echo %s; exit\n", execMarker, command, execMarker)
	if err := tc.writeMessage(websocket.TextMessage, []byte(fullCmd)); err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	select {
	case <-ctx.Done():
		return "", tcerrors.New("command timed out")
	case res := <-resultCh:
		return res.data, res.err
	}
}

// normalizeLineEndings converts CRLF to LF and removes standalone CR.
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// extractExecOutput extracts the command output from between the markers.
func extractExecOutput(raw string) string {
	raw = normalizeLineEndings(raw)
	markerOnLine := "\n" + execMarker
	startIdx := strings.Index(raw, markerOnLine)
	if startIdx != -1 {
		raw = raw[startIdx+len(markerOnLine):]
		// Skip to next line
		if nlIdx := strings.Index(raw, "\n"); nlIdx != -1 {
			raw = raw[nlIdx+1:]
		}
	}
	endIdx := strings.Index(raw, markerOnLine)
	if endIdx != -1 {
		raw = raw[:endIdx]
	}

	raw = stripansi.Strip(raw)
	return strings.TrimSpace(raw)
}

func (tc *TerminalConn) Close() {
	tc.closeOnce.Do(func() {
		close(tc.done)
		_ = tc.conn.Close()
	})
}

// writeMessage writes a message to the WebSocket with proper serialization and deadline.
func (tc *TerminalConn) writeMessage(messageType int, data []byte) error {
	tc.writeMu.Lock()
	defer tc.writeMu.Unlock()
	_ = tc.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return tc.conn.WriteMessage(messageType, data)
}

func (tc *TerminalConn) copyToWriter(w io.Writer, errChan chan<- error) {
	defer tc.Close()
	for {
		_, r, err := tc.conn.NextReader()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				tc.mu.Lock()
				tc.err = err
				tc.mu.Unlock()
			}
			return
		}
		if _, err := io.Copy(w, r); err != nil {
			select {
			case errChan <- err:
			default:
			}
			return
		}
	}
}

func (tc *TerminalConn) copyFromReader(r io.Reader, errChan chan<- error) {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if err != nil {
			if err != io.EOF { // Propagate read errors (e.g., stdin closed unexpectedly)
				select {
				case errChan <- fmt.Errorf("stdin read error: %w", err):
				default:
				}
			}
			return
		}
		select {
		case <-tc.done:
			return
		default:
		}
		if err := tc.writeMessage(websocket.TextMessage, buf[:n]); err != nil {
			select {
			case errChan <- err:
			default:
			}
			return
		}
	}
}

func (tc *TerminalConn) sendResize() {
	cols, rows := output.TerminalSize()
	tc.sendJSON("resize", map[string]string{
		"cols": strconv.Itoa(cols),
		"rows": strconv.Itoa(rows),
	})
}

func (tc *TerminalConn) sendPing() {
	tc.sendJSON("ping", map[string]string{
		"ts": strconv.FormatInt(time.Now().UnixMilli(), 10),
	})
}

func (tc *TerminalConn) sendJSON(cmd string, details map[string]string) {
	data, err := json.Marshal(map[string]any{
		"agent-terminal-command": cmd,
		"details":                details,
	})
	if err != nil {
		output.Debug("terminal: failed to marshal %s command: %v", cmd, err)
		return
	}
	if err := tc.writeMessage(websocket.TextMessage, data); err != nil {
		output.Debug("terminal: failed to send %s command: %v", cmd, err)
	}
}
