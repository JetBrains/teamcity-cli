package analytics

import (
	"cmp"
	"context"
	"errors"
	"sync"
	"time"

	fus "github.com/JetBrains/fus-reporting-api-go"
)

// Client is nil-safe. Pass nil for "disabled" — every Track* method becomes a no-op.
type Client struct {
	mu sync.Mutex

	staging       bool
	salt          string
	logger        *fus.Logger
	session       *Session
	env           Environment
	source        string
	cliVersion    string
	serverVersion string
	serverType    string
	authSource    string
	hasLinkedPrj  bool
	debug         func(string, ...any)
	bootOnce      sync.Once
}

type Config struct {
	Staging          bool
	Salt             string
	CLIVersion       string
	ServerVersion    string // YYYY.MM[.x]; omitted from session event when empty
	ServerType       string // "cloud" | "on_prem"; omitted when empty
	AuthSource       string
	HasLinkedProject bool
	Session          *Session
	Environment      Environment

	// Debug, when non-nil, receives one line per lifecycle event (boot, track, flush).
	// Pass f.Printer.Debug to surface only with --verbose / --debug.
	Debug func(string, ...any)
}

// New builds a Client; FUS logger boots lazily on first Track and noops on failure.
func New(cfg Config) *Client {
	return &Client{
		session:       cfg.Session,
		env:           cfg.Environment,
		source:        ClassifySource(cfg.Environment),
		cliVersion:    cfg.CLIVersion,
		serverVersion: cfg.ServerVersion,
		serverType:    cfg.ServerType,
		authSource:    cfg.AuthSource,
		hasLinkedPrj:  cfg.HasLinkedProject,
		staging:       cfg.Staging,
		salt:          cfg.Salt,
		debug:         cfg.Debug,
	}
}

func (c *Client) logf(format string, args ...any) {
	if c == nil || c.debug == nil {
		return
	}
	c.debug(format, args...)
}

func (c *Client) SessionID() string {
	if c == nil || c.session == nil {
		return ""
	}
	return c.session.ID
}

func (c *Client) boot() {
	c.bootOnce.Do(func() {
		dir, err := DataDir()
		if err != nil {
			c.logf("analytics: boot failed (data dir): %v", err)
			return
		}
		validator, err := fus.NewValidator(Scheme)
		if err != nil {
			c.logf("analytics: boot failed (validator): %v", err)
			return
		}
		opts := []fus.LoggerOption{fus.WithValidator(validator)}
		if c.staging {
			stagingCfg, err := fus.FetchTestConfig(RecorderID, ProductCode)
			if err != nil {
				c.logf("analytics: boot failed (staging config): %v", err)
				return
			}
			opts = append(opts, fus.WithFUSConfig(stagingCfg))
		}
		logger, err := fus.NewLogger(
			fus.RecorderConfig{
				RecorderID:        RecorderID,
				RecorderVersion:   RecorderVersion,
				ProductCode:       ProductCode,
				BuildVersion:      c.cliVersion,
				DataDir:           dir,
				AnonymizationSalt: c.salt,
			},
			opts...,
		)
		if err != nil {
			c.logf("analytics: boot failed (logger): %v", err)
			return
		}
		c.logger = logger
		c.logf("analytics: ready (recorder=%s product=%s staging=%v salt=%t buffer=%s)",
			RecorderID, ProductCode, c.staging, c.salt != "", dir)
	})
}

func (c *Client) track(group string, state bool, eventID string, data map[string]any) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.boot()
	if c.logger == nil {
		return
	}
	c.logger.Track(fus.EventGroup{ID: group, Version: groupVersion, State: state}, eventID, data)
	c.logf("analytics: emit %s/%s (%d fields)", group, eventID, len(data))
}

// Track emits a counter event. Field names and values must match the scheme;
// TestLintScheme catches drift at test time.
func (c *Client) Track(group, eventID string, data map[string]any) {
	c.track(group, false, eventID, data)
}

// TrackSession emits the session.invoked state event once per new session.
func (c *Client) TrackSession() {
	if c == nil || c.session == nil || !c.session.IsNew {
		return
	}
	data := map[string]any{
		"session_id":         c.session.ID,
		"cli_version":        c.cliVersion,
		"os":                 c.env.OS,
		"arch":               c.env.Arch,
		"ci_system":          c.env.CISystem,
		"auth_source":        cmp.Or(c.authSource, AuthSourceNone),
		"ai_agent":           c.env.AIAgent,
		"has_linked_project": c.hasLinkedPrj,
	}
	if c.serverVersion != "" {
		data["server_version"] = c.serverVersion
	}
	if c.serverType != "" {
		data["server_type"] = c.serverType
	}
	c.track(GroupSession, true, EventInvoked, data)
}

// CommandEvent carries the command-executed counter event; typed because exit code and error type need normalization.
type CommandEvent struct {
	Command        string
	HasJSON        bool
	HasGitContext  bool
	HasLinkContext bool
	FlagCount      int
	ExitCode       int
	DurationMS     int64
	ErrorType      string
}

func (c *Client) TrackCommand(e CommandEvent) {
	if c == nil {
		return
	}
	exit := "0"
	switch {
	case e.ExitCode == 2:
		exit = "2"
	case e.ExitCode != 0:
		exit = "1"
	}
	c.track(GroupCommand, false, EventExecuted, map[string]any{
		"session_id":       c.SessionID(),
		"command":          NormalizeCommand(e.Command),
		"source":           c.source,
		"has_json":         e.HasJSON,
		"has_git_context":  e.HasGitContext,
		"has_link_context": e.HasLinkContext,
		"flag_count":       e.FlagCount,
		"exit_code":        exit,
		"duration_ms":      e.DurationMS,
		"error_type":       cmp.Or(e.ErrorType, ErrorNone),
	})
}

// APIEvent carries the api-invoked counter event; typed because raw endpoints need sanitization.
type APIEvent struct {
	Method     string
	Endpoint   string
	StatusCode int
	Paginated  bool
	Slurp      bool
	HadFields  bool
	HadInput   bool
}

func (c *Client) TrackAPI(e APIEvent) {
	if c == nil {
		return
	}
	c.track(GroupAPI, false, EventInvoked, map[string]any{
		"method":       NormalizeHTTPMethod(e.Method),
		"resource":     APIResource(e.Endpoint),
		"status_code":  e.StatusCode,
		"is_paginated": e.Paginated,
		"is_slurp":     e.Slurp,
		"had_fields":   e.HadFields,
		"had_input":    e.HadInput,
	})
}

func (c *Client) Flush(ctx context.Context) error {
	if c == nil || c.logger == nil {
		return nil
	}
	return c.logger.Flush(ctx)
}

// Close flushes with a 2s timeout; idempotent.
func (c *Client) Close() error {
	if c == nil || c.logger == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := c.logger.Flush(ctx)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		c.logf("analytics: flush timed out after 2s")
		return nil
	case err != nil:
		c.logf("analytics: flush failed: %v", err)
	default:
		c.logf("analytics: flushed")
	}
	return err
}
