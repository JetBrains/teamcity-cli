package cmd

import (
	"errors"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// setupAnalytics computes opt-out, prints first-run notice, opens the session, and attaches a Client to f.
// f.Analytics stays nil when opted out — every Track* method is nil-safe.
func setupAnalytics(f *cmdutil.Factory) {
	enabled, reason := analytics.IsEnabled(config.IsAnalyticsEnabled())

	if enabled && !config.IsAnalyticsNoticeShown() {
		if analytics.PrintFirstRunNotice(f.Printer.ErrOut, false, false, f.Quiet, f.NoInput) {
			_ = config.MarkAnalyticsNoticeShown()
		}
	}

	if !enabled {
		f.Printer.Debug("analytics: disabled (%s)", reason)
		return
	}

	session, err := analytics.LoadOrCreateSession(time.Now())
	if err != nil {
		f.Printer.Debug("analytics: disabled (session error: %v)", err)
		return
	}

	env := analytics.DetectEnvironment()
	authSource := detectAuthSourceForAnalytics()
	serverVersion, serverType := analytics.LoadServerInfo(config.GetServerURL())
	f.Analytics = analytics.New(analytics.Config{
		CLIVersion:       version.String(),
		AuthSource:       authSource,
		Session:          session,
		Environment:      env,
		ServerVersion:    serverVersion,
		ServerType:       serverType,
		HasLinkedProject: false,
		Staging:          analytics.IsStaging(),
		Salt:             analytics.Salt(),
		Debug:            f.Printer.Debug,
	})
	f.Printer.Debug("analytics: enabled (session=%s new=%v source=%s ai_agent=%s ci=%s staging=%v)",
		session.ID, session.IsNew, analytics.ClassifySource(env), env.AIAgent, env.CISystem, analytics.IsStaging())
	f.Analytics.TrackSession()
	if session.IsNew && authSource != analytics.AuthSourceNone {
		f.Analytics.Track(analytics.GroupAuth, analytics.EventTokenLoaded, map[string]any{
			"source":     authSource,
			"is_expired": isTokenExpired(),
		})
	}
}

func isTokenExpired() bool {
	exp := config.GetTokenExpiry()
	if exp == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, exp)
	if err != nil {
		return false
	}
	return time.Now().After(t)
}

// trackAndFlushAnalytics emits the per-command counter event and flushes the FUS buffer.
func trackAndFlushAnalytics(f *cmdutil.Factory, executedCmd *cobra.Command, runErr error) {
	if f.Analytics == nil {
		return
	}
	defer func() { _ = f.Analytics.Close() }()

	if executedCmd == nil {
		return
	}

	f.Analytics.TrackCommand(analytics.CommandEvent{
		Command:        commandPathForAnalytics(executedCmd),
		HasJSON:        flagBool(executedCmd, "json"),
		HasGitContext:  detectGitContext(executedCmd),
		HasLinkContext: false,
		FlagCount:      countSetFlags(executedCmd),
		ExitCode:       exitCodeFromError(runErr),
		DurationMS:     time.Since(f.StartTime).Milliseconds(),
		ErrorType:      errorTypeFromError(runErr),
	})
}

// commandPathForAnalytics joins the cobra command path with dots, dropping the binary name.
func commandPathForAnalytics(cmd *cobra.Command) string {
	parts := strings.Fields(cmd.CommandPath())
	if len(parts) <= 1 {
		return "other"
	}
	return strings.Join(parts[1:], ".")
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := errors.AsType[*cmdutil.ExitError](err); ok {
		return ee.Code
	}
	return 1
}

// errorTypeFromError reuses output.ClassifyError's walk so the analytics enum always tracks the user-facing classification.
func errorTypeFromError(err error) string {
	if err == nil {
		return analytics.ErrorNone
	}
	code, _, _ := output.ClassifyError(err)
	switch code {
	case output.ErrCodeReadOnly:
		return analytics.ErrorReadOnly
	case output.ErrCodeAuth:
		return analytics.ErrorAuth
	case output.ErrCodePermission:
		return analytics.ErrorPermission
	case output.ErrCodeNotFound:
		return analytics.ErrorNotFound
	case output.ErrCodeNetwork:
		return analytics.ErrorNetwork
	case output.ErrCodeValidation:
		return analytics.ErrorValidation
	default:
		return analytics.ErrorInternal
	}
}

func flagBool(cmd *cobra.Command, name string) bool {
	f := cmd.Flags().Lookup(name)
	if f == nil || !f.Changed {
		return false
	}
	if f.Value.Type() == "bool" {
		return f.Value.String() == "true"
	}
	return f.Value.String() != ""
}

// detectGitContext reports whether the command was invoked with --local-changes, --branch=@this, or --revision=@head.
func detectGitContext(cmd *cobra.Command) bool {
	if flagBool(cmd, "local-changes") {
		return true
	}
	for _, name := range []string{"branch", "revision"} {
		f := cmd.Flags().Lookup(name)
		if f == nil || !f.Changed {
			continue
		}
		v := strings.ToLower(strings.TrimSpace(f.Value.String()))
		if v == "@this" || v == "@head" {
			return true
		}
	}
	return false
}

func countSetFlags(cmd *cobra.Command) int {
	n := 0
	cmd.Flags().Visit(func(_ *pflag.Flag) { n++ })
	return n
}

// detectAuthSourceForAnalytics maps the existing config token-source string to the wire enum.
func detectAuthSourceForAnalytics() string {
	if config.IsGuestAuth() {
		return analytics.AuthSourceGuest
	}
	_, source, _ := config.GetTokenWithSource()
	switch source {
	case "env":
		return analytics.AuthSourceEnv
	case "keyring", "config":
		return analytics.AuthSourceKeyring
	default:
		return analytics.AuthSourceNone
	}
}
