//go:build gallery

package gallery_test

import (
	"bytes"
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

	book := termbook.New("TeamCity CLI — Screen Gallery",
		termbook.WithGitHub("https://github.com/jetbrains/teamcity-cli/"),
		termbook.WithAccent("#07C3F2"),
		termbook.WithIntro("Auto-generated from real command output. Click terminals to expand. Regenerate with just gallery"),
	)

	book.Category("Style Guide", "style-guide", styleGuideScreens()...)
	book.Category("Runs", "runs", runScreens(t, ts)...)
	book.Category("Jobs", "jobs", jobScreens(t, ts)...)
	book.Category("Agents", "agents", agentScreens(t, ts)...)
	book.Category("Queue", "queue", queueScreens(t, ts)...)
	book.Category("Pools", "pools", poolScreens(t, ts)...)
	book.Category("Projects", "projects", projectScreens(t, ts)...)
	book.Category("Pipelines", "pipelines", pipelineScreens(t, ts, yamlFile.Name())...)
	book.Category("Auth", "auth", authScreens(t, ts)...)
	book.Category("Config", "config", configScreens(t, ts)...)
	book.Category("Aliases", "aliases", aliasScreens(t, ts)...)
	book.Category("Skills", "skills", skillScreens(t, ts)...)
	book.Category("API", "api", apiScreens(t, ts)...)
	book.Category("Update", "update", updateScreens(t, ts)...)
	book.Category("Errors", "errors", errorScreens()...)
	book.Category("Help Screens", "help", helpScreens(t, ts)...)

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
