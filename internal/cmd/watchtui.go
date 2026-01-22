package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/output"
)

var spinnerFrames = []string{"/", "-", "\\", "|"}

type tickMsg time.Time
type buildMsg *api.Build
type logMsg string
type errMsg error

type watchModel struct {
	client       *api.Client
	runID        string
	interval     time.Duration
	build        *api.Build
	logLines     []string
	rawLogLen    int
	err          error
	done         bool
	width        int
	height       int
	spinnerIndex int
}

func newWatchModel(client *api.Client, runID string, interval int) watchModel {
	return watchModel{
		client:   client,
		runID:    runID,
		interval: time.Duration(interval) * time.Second,
		logLines: []string{},
	}
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(m.fetchBuild, m.fetchLog, m.spinnerTick())
}

func (m watchModel) fetchBuild() tea.Msg {
	build, err := m.client.GetBuild(m.runID)
	if err != nil {
		return errMsg(err)
	}
	return buildMsg(build)
}

func (m watchModel) fetchLog() tea.Msg {
	log, err := m.client.GetBuildLog(m.runID)
	if err != nil {
		return logMsg("")
	}
	return logMsg(log)
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type spinnerMsg struct{}

func (m watchModel) spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return spinnerMsg{} })
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

	case spinnerMsg:
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
		if m.build != nil && m.build.State == "finished" {
			return m, nil
		}
		return m, m.spinnerTick()

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

	return fmt.Sprintf("[%s] %s", timestamp, rest)
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
	b.WriteString("\n")
	b.WriteString(output.Faint(strings.Repeat("─", min(m.width, 80))))
	b.WriteString("\n")

	logHeight := m.height - 3
	if logHeight < 3 {
		logHeight = 3
	}
	b.WriteString(m.renderLogs(logHeight))

	spinner := ""
	if m.build == nil || m.build.State != "finished" {
		spinner = " " + spinnerFrames[m.spinnerIndex]
	}
	b.WriteString(output.Faint("q quit" + spinner))

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

	icon := output.StatusIcon(m.build.Status, m.build.State)
	status := output.StatusText(m.build.Status, m.build.State)

	header := fmt.Sprintf("%s %s #%s · %s", icon, output.Bold(jobName), m.build.Number, status)
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

	start := len(m.logLines) - height
	if start < 0 {
		start = 0
	}
	visible := m.logLines[start:]

	maxWidth := m.width - 1
	if maxWidth < 40 {
		maxWidth = 40
	}

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

func runWatchTUI(client *api.Client, runID string, interval int) error {
	m := newWatchModel(client, runID, interval)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	fm := finalModel.(watchModel)
	if fm.build != nil {
		fmt.Println()
		jobName := fm.build.BuildTypeID
		if fm.build.BuildType != nil {
			jobName = fm.build.BuildType.Name
		}

		icon := output.StatusIcon(fm.build.Status, fm.build.State)
		if fm.build.State == "finished" {
			if fm.build.Status == "SUCCESS" {
				fmt.Printf("%s %s #%s completed\n", icon, output.Cyan(jobName), fm.build.Number)
			} else {
				fmt.Printf("%s %s #%s failed: %s\n", icon, output.Cyan(jobName), fm.build.Number, fm.build.StatusText)
			}
		} else {
			fmt.Println(output.Faint("Build still running in background"))
			fmt.Printf("Resume: tc run watch %s --logs\n", fm.runID)
		}
		fmt.Printf("View details: %s\n", fm.build.WebURL)
	}

	return nil
}
