//go:build gallery

package gallery_test

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
	"github.com/tiulpin/termbook"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

func TestGenerateGallery(t *testing.T) {
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = true })

	ts := setupGalleryMocks(t)

	yamlFile, err := os.CreateTemp(t.TempDir(), "*.teamcity.yml")
	require.NoError(t, err)
	_, _ = yamlFile.WriteString("version: v1.0\njobs:\n  build:\n    steps:\n      - script: go build ./...\n  test:\n    needs: [build]\n    steps:\n      - script: go test -race ./...\n")
	yamlFile.Close()

	book := termbook.New("teamcity cli screen gallery",
		termbook.WithGitHub("https://github.com/jetbrains/teamcity-cli/"),
		termbook.WithCSS(jetBrainsCSS),
		termbook.WithDecor(termbook.Decor{
			BrandName:    "teamcity cli",
			BrandVersion: "v0.9.0",
			Crumbs:       []string{"jetbrains", "teamcity-cli", "gallery"},
			Facts: []termbook.Fact{
				{Value: "134", Label: "screens"},
				{Value: "16", Label: "command groups"},
			},
			Notes: termbook.Notes{
				Title: "Reading this gallery",
				Body: template.HTML(`
			<p>Every block is real CLI output, captured on each release — not screenshots or mockups. Skim top-to-bottom, jump by section from the sidebar, or filter by name.</p>
			<p class="mono-hint">press <span class="kbd">/</span> to filter ·
			<span class="kbd">T</span> to toggle theme ·
			<span class="kbd">Esc</span> to clear</p>
		`), //nolint:gosec // trusted
			},
			Footer: "teamcity cli · screen gallery",
			Attribution: template.HTML(`
			<span class="jb-wordmark">JetBrains</span>
			<span class="dot">·</span>
			<span>Developer Tools</span>
			<span class="dot">·</span>
			<span>The Drive to Develop</span>
			<span style="margin-left:auto">© 2026 JetBrains s.r.o.</span>
		`), //nolint:gosec // trusted
			BloomDark:  "radial-gradient(closest-side, oklch(0.74 0.20 20 / 0.28), oklch(0.78 0.18 45 / 0.16) 45%, transparent 72%)",
			BloomLight: "radial-gradient(closest-side, oklch(0.78 0.18 340 / 0.30), oklch(0.82 0.14 60 / 0.18) 45%, transparent 72%)",
		}),
	)

	book.CategoryWithBlurb("Style Guide", "style-guide",
		"The atoms the CLI uses to communicate. Treat these as non-negotiable until we decide to rework them together.",
		styleGuideScreens()...)
	book.CategoryWithBlurb("Runs", "runs",
		"List, start, watch, diff, and manage builds.",
		runScreens(t, ts)...)
	book.CategoryWithBlurb("Jobs", "jobs",
		"Build configurations: list, view, pause, parameters.",
		jobScreens(t, ts)...)
	book.CategoryWithBlurb("Agents", "agents",
		"List agents, view status, open terminals, execute commands.",
		agentScreens(t, ts)...)
	book.CategoryWithBlurb("Queue", "queue",
		"Inspect and reorder the build queue.",
		queueScreens(t, ts)...)
	book.CategoryWithBlurb("Pools", "pools",
		"Agent pools and project assignments.",
		poolScreens(t, ts)...)
	book.CategoryWithBlurb("Projects", "projects",
		"Project tree, VCS, SSH, cloud, parameters, tokens.",
		projectScreens(t, ts)...)
	book.CategoryWithBlurb("Pipelines", "pipelines",
		"YAML pipeline lifecycle: validate, push, pull.",
		pipelineScreens(t, ts, yamlFile.Name())...)
	book.CategoryWithBlurb("Auth", "auth",
		"Login, logout, multi-server status.",
		authScreens(t, ts)...)
	book.CategoryWithBlurb("Config", "config",
		"CLI configuration values.",
		configScreens(t, ts)...)
	book.CategoryWithBlurb("Aliases", "aliases",
		"Command shortcuts.",
		aliasScreens(t, ts)...)
	book.CategoryWithBlurb("Skills", "skills",
		"AI coding agent skills: install, update, remove.",
		skillScreens(t, ts)...)
	book.CategoryWithBlurb("API", "api",
		"Raw authenticated REST calls.",
		apiScreens(t, ts)...)
	book.CategoryWithBlurb("Update", "update",
		"Check for new CLI releases.",
		updateScreens(t, ts)...)
	book.CategoryWithBlurb("Errors", "errors",
		"Error message shapes and hints.",
		errorScreens()...)
	book.CategoryWithBlurb("Help Screens", "help",
		"Built-in --help output for every command group.",
		helpScreens(t, ts)...)

	outPath := filepath.Join(repoRoot(), "docs", "index.html")
	require.NoError(t, book.Generate(outPath))

	t.Logf("Gallery written to %s", outPath)
}

var mockURLReplacer *strings.Replacer

func capture(t *testing.T, ts *cmdtest.TestServer, args ...string) string {
	t.Helper()
	if mockURLReplacer == nil {
		mockURLReplacer = strings.NewReplacer(
			ts.URL, "https://tc.example.com",
			"https://cli.teamcity.com", "https://tc.example.com",
			"https://buildserver.labs.intellij.net", "https://staging.tc.example.com",
			"https://jetbrains-ai.internal.teamcity.cloud", "https://ai.tc.example.com",
			"https://teamcity-nightly.labs.intellij.net", "https://nightly.tc.example.com",
			os.Getenv("HOME")+"/.config/tc/config.yml", "~/.config/tc/config.yml",
			os.Getenv("HOME"), "/home/user",
		)
	}
	f := ts.CloneFactory()
	var buf bytes.Buffer
	f.Printer = &output.Printer{Out: &buf, ErrOut: &buf}
	rootCmd := cmd.NewRootCmdWithFactory(f)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	require.NoError(t, rootCmd.Execute(), "teamcity %s", strings.Join(args, " "))
	result := mockURLReplacer.Replace(buf.String())
	result = regexp.MustCompile(`http://127\.0\.0\.1:\d+`).ReplaceAllString(result, "https://tc.example.com")
	return result
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

const jetBrainsCSS = `
.mast h1 .slash {
  background: linear-gradient(135deg, #FE2857 0%, #FF9E2C 45%, #E1309B 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  -webkit-text-fill-color: transparent;
}
.attrib .jb-wordmark {
  font-family: var(--mono);
  font-weight: 700;
  letter-spacing: -0.01em;
  color: var(--fg);
  font-size: 12px;
}
`

