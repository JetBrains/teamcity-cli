package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/acarl005/stripansi"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

type tickMsg time.Time
type buildMsg *api.Build
type logMsg string
type errMsg error

type watchModel struct {
	ctx       context.Context
	client    api.ClientInterface
	runID     string
	interval  time.Duration
	build     *api.Build
	logLines  []string
	rawLogLen int
	err       error
	done      bool
	width     int
	height    int
	spinner   spinner.Model
}

func newWatchModel(ctx context.Context, client api.ClientInterface, runID string, interval int) watchModel {
	sp := spinner.New()
	sp.Spinner = spinner.Line
	return watchModel{
		ctx:      ctx,
		client:   client,
		runID:    runID,
		interval: time.Duration(interval) * time.Second,
		logLines: []string{},
		spinner:  sp,
	}
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(m.fetchBuild, m.fetchLog, m.spinner.Tick)
}

func (m watchModel) fetchBuild() tea.Msg {
	build, err := m.client.GetBuild(m.ctx, m.runID)
	if err != nil {
		return errMsg(err)
	}
	return buildMsg(build)
}

func (m watchModel) fetchLog() tea.Msg {
	log, err := m.client.GetBuildLog(m.ctx, m.runID)
	if err != nil {
		return logMsg("")
	}
	return logMsg(log)
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		if m.build != nil && m.build.State == "finished" {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		return m, tea.Batch(m.fetchBuild, m.fetchLog)

	case buildMsg:
		m.build = msg
		if m.build != nil && m.build.State == "finished" {
			return m, tea.Batch(m.fetchLog, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return tea.Quit()
			}))
		}
		return m, tickCmd(m.interval)

	case logMsg:
		rawLog := string(msg)
		if rawLog != "" && len(rawLog) != m.rawLogLen {
			m.rawLogLen = len(rawLog)
			m.logLines = parseWatchLogLines(rawLog)
		}

	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	return m, nil
}

func parseWatchLogLines(log string) []string {
	raw := strings.Split(log, "\n")
	result := make([]string, 0, len(raw))

	for _, line := range raw {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "export ") || strings.HasPrefix(line, "exec ") {
			continue
		}
		if strings.Contains(line, "Current time:") {
			continue
		}
		formatted := formatWatchLogLine(line)
		if formatted != "" {
			result = append(result, formatted)
		}
	}
	return result
}

func formatWatchLogLine(line string) string {
	if len(line) < 10 || line[0] != '[' {
		return ""
	}

	closeBracket := strings.Index(line, "]")
	if closeBracket < 8 {
		return ""
	}

	timestamp := line[1:closeBracket]
	rest := line[closeBracket+1:]

	if len(rest) >= 2 && rest[1] == ':' {
		rest = rest[2:]
	} else if len(rest) >= 3 && rest[0] == ' ' && rest[2] == ':' {
		rest = rest[3:]
	}

	if idx := strings.Index(rest, "[Step"); idx != -1 {
		rest = rest[idx:]
	} else {
		rest = strings.TrimSpace(rest)
	}

	if rest == "" {
		return ""
	}

	return fmt.Sprintf("[%s] %s", timestamp, output.RestoreAnsi(rest))
}

func (m watchModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	if m.width == 0 || m.height == 0 {
		return "Refreshing..."
	}

	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")

	logHeight := max(m.height-3, 3)
	b.WriteString(m.renderLogs(logHeight))

	spinnerView := ""
	if m.build == nil || m.build.State != "finished" {
		spinnerView = " " + m.spinner.View()
	}
	footer := output.Faint("q quit")
	if m.build != nil && m.build.Agent != nil {
		footer += output.Faint("  ·  ") + output.Cyan(fmt.Sprintf("teamcity agent term %d", m.build.Agent.ID))
	}
	b.WriteString(footer + output.Faint(spinnerView))

	return b.String()
}

func (m watchModel) renderHeader() string {
	if m.build == nil {
		return output.Yellow("*") + " Refreshing..."
	}

	jobName := m.build.BuildTypeID
	if m.build.BuildType != nil {
		jobName = m.build.BuildType.Name
	}

	icon := output.StatusIcon(m.build.Status, m.build.State, m.build.StatusText)
	status := output.StatusText(m.build.Status, m.build.State, m.build.StatusText)

	header := fmt.Sprintf("%s %s %d  #%s %s · %s", icon, output.Bold(jobName), m.build.ID, m.build.Number, output.Faint(m.build.WebURL), status)
	if m.build.PercentageComplete > 0 && m.build.State != "finished" {
		header += fmt.Sprintf(" (%d%%)", m.build.PercentageComplete)
	}
	return header
}

func (m watchModel) renderLogs(height int) string {
	if len(m.logLines) == 0 {
		var result strings.Builder
		result.WriteString(output.Faint("Waiting for logs..."))
		result.WriteString("\n")
		for i := 1; i < height; i++ {
			result.WriteString("\n")
		}
		return result.String()
	}

	start := max(len(m.logLines)-height, 0)
	visible := m.logLines[start:]

	maxWidth := max(m.width-1, 40)

	var result strings.Builder
	for _, line := range visible {
		if runewidth.StringWidth(stripansi.Strip(line)) > maxWidth {
			line = runewidth.Truncate(stripansi.Strip(line), maxWidth, "...")
		}
		result.WriteString(line)
		result.WriteString("\n")
	}

	for i := len(visible); i < height; i++ {
		result.WriteString("\n")
	}
	return result.String()
}

// RunWatchTUI launches the interactive TUI for watching a build.
func RunWatchTUI(ctx context.Context, client api.ClientInterface, runID string, interval int) error {
	m := newWatchModel(ctx, client, runID, interval)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	fm := finalModel.(watchModel)
	printer := output.DefaultPrinter()
	_, _ = fmt.Fprintln(printer.Out)

	if fm.build == nil || fm.build.State != "finished" {
		_, _ = fmt.Fprintln(printer.Out, output.Faint("Build still running in background"))
		_, _ = fmt.Fprintf(printer.Out, "Resume: teamcity run watch %s --logs\n", fm.runID)
		return nil
	}

	return cmdutil.BuildResultError(ctx, printer, client, fm.build, true)
}
