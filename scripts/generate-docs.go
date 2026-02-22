//go:build ignore

// Script to generate CLI documentation in docs/topics/teamcity-cli-commands.md from cobra commands.
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Preferred ordering (unlisted commands added alphabetically at end).
var preferredOrder = []string{"auth", "run", "job", "project", "queue", "agent", "pool", "api"}

// Custom display names for commands that need special treatment.
var displayNames = map[string]string{
	"alias": "Aliases",
	"api":   "API",
	"auth":  "Authentication",
	"pool":  "Agent Pools",
}

// sectionDescriptions maps command groups to their description and detail page link for Writerside docs.
var sectionDescriptions = map[string]struct {
	desc string
	page string
}{
	"auth":       {"Manage server authentication.", "teamcity-cli-authentication.md"},
	"run":        {"Start, monitor, and manage builds.", "teamcity-cli-managing-runs.md"},
	"job":        {"View and configure build configurations.", "teamcity-cli-managing-jobs.md"},
	"project":    {"Browse projects and manage parameters and settings.", "teamcity-cli-managing-projects.md"},
	"queue":      {"Manage the build queue.", "teamcity-cli-managing-build-queue.md"},
	"agent":      {"Monitor and control build agents.", "teamcity-cli-managing-agents.md"},
	"pool":       {"Manage agent pool assignments.", "teamcity-cli-managing-agent-pools.md"},
	"api":        {"Make raw REST API requests.", "teamcity-cli-rest-api-access.md"},
	"alias":      {"Create custom command shortcuts.", "teamcity-cli-aliases.md"},
	"completion": {"Generate shell completion scripts.", "teamcity-cli-configuration.md#shell-completion"},
	"skill":      {"Manage AI agent integration.", "teamcity-cli-ai-agent-integration.md"},
}

const commandsDocPath = "docs/topics/teamcity-cli-commands.md"

func main() {
	rootCmd := cmd.GetRootCmd()

	check := len(os.Args) > 1 && os.Args[1] == "--check"
	ok := true

	ok = updateFile(commandsDocPath, func(rootCmd *cobra.Command) string {
		var buf bytes.Buffer
		generateWritersideDocs(&buf, rootCmd)
		return buf.String()
	}, rootCmd, check) && ok

	if check && !ok {
		os.Exit(1)
	}
}

func updateFile(path string, generate func(*cobra.Command) string, rootCmd *cobra.Command, check bool) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	generated := generate(rootCmd)
	newContent := replaceBetweenMarkers(string(content), generated)

	if check {
		if string(content) != newContent {
			fmt.Printf("%s is out of date. Run 'just docs' to update it.\n", path)
			return false
		}
		fmt.Printf("%s is up to date.\n", path)
		return true
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("%s updated.\n", path)
	return true
}

func replaceBetweenMarkers(content, generated string) string {
	re := regexp.MustCompile(`(?s)<!-- COMMANDS_START -->.*<!-- COMMANDS_END -->`)
	return re.ReplaceAllLiteralString(content, "<!-- COMMANDS_START -->\n\n"+generated+"<!-- COMMANDS_END -->")
}

// orderedCommands returns command groups in preferred order, including completion.
func orderedCommands(rootCmd *cobra.Command, includeCompletion bool) ([]string, map[string]*cobra.Command) {
	cmds := make(map[string]*cobra.Command)
	for _, c := range rootCmd.Commands() {
		if c.Name() == "help" {
			continue
		}
		if c.Name() == "completion" && !includeCompletion {
			continue
		}
		cmds[c.Name()] = c
	}

	seen := make(map[string]bool)
	var names []string
	for _, name := range preferredOrder {
		if _, ok := cmds[name]; ok {
			names = append(names, name)
			seen[name] = true
		}
	}
	var rest []string
	for name := range cmds {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	names = append(names, rest...)
	return names, cmds
}

func displayName(name string) string {
	if dn, ok := displayNames[name]; ok {
		return dn
	}
	return cases.Title(language.English).String(name) + "s"
}

// --- Writerside generation ---

func generateWritersideDocs(buf *bytes.Buffer, rootCmd *cobra.Command) {
	names, cmds := orderedCommands(rootCmd, true)

	for _, name := range names {
		c := cmds[name]
		sectionName := displayName(name)
		// completion is special: not pluralized by displayName
		if name == "completion" {
			sectionName = "Completion"
		}

		buf.WriteString(fmt.Sprintf("## %s\n\n", sectionName))

		if sec, ok := sectionDescriptions[name]; ok {
			buf.WriteString(fmt.Sprintf("%s See [%s](%s) for details.\n\n", sec.desc, pageLinkText(sec.page), sec.page))
		}

		buf.WriteString("<table>\n")
		writeWritersideRow(buf, "Command", "Description")

		subCmds := sortedCommands(c)
		if len(subCmds) == 0 {
			// Top-level command (e.g., api, completion)
			cmdStr := fmt.Sprintf("`teamcity %s`", c.Name())
			if name == "api" {
				cmdStr = "`teamcity api <endpoint>`"
			}
			writeWritersideRow(buf, cmdStr, c.Short)
		} else {
			for _, sub := range subCmds {
				subSubs := sortedCommands(sub)
				if len(subSubs) > 0 {
					for _, subSub := range subSubs {
						cmdStr := fmt.Sprintf("`teamcity %s %s %s`", c.Name(), sub.Name(), subSub.Name())
						writeWritersideRow(buf, cmdStr, subSub.Short)
					}
				} else {
					cmdStr := fmt.Sprintf("`teamcity %s %s`", c.Name(), sub.Name())
					writeWritersideRow(buf, cmdStr, sub.Short)
				}
			}
		}

		buf.WriteString("</table>\n\n")
	}
}

func writeWritersideRow(buf *bytes.Buffer, col1, col2 string) {
	buf.WriteString("<tr>\n<td>\n\n")
	buf.WriteString(col1)
	buf.WriteString("\n\n</td>\n<td>\n\n")
	buf.WriteString(col2)
	buf.WriteString("\n\n</td>\n</tr>\n")
}

func pageLinkText(page string) string {
	// Map page filenames to human-readable link text
	links := map[string]string{
		"teamcity-cli-authentication.md":            "Authentication",
		"teamcity-cli-managing-runs.md":             "Managing runs",
		"teamcity-cli-managing-jobs.md":             "Managing jobs",
		"teamcity-cli-managing-projects.md":         "Managing projects",
		"teamcity-cli-managing-build-queue.md":      "Managing the build queue",
		"teamcity-cli-managing-agents.md":           "Managing agents",
		"teamcity-cli-managing-agent-pools.md":      "Managing agent pools",
		"teamcity-cli-rest-api-access.md":           "REST API access",
		"teamcity-cli-aliases.md":                   "Aliases",
		"teamcity-cli-configuration.md#shell-completion": "Configuration",
		"teamcity-cli-ai-agent-integration.md":      "AI agent integration",
	}
	if text, ok := links[page]; ok {
		return text
	}
	return page
}

// --- Shared helpers ---

func sortedCommands(c *cobra.Command) []*cobra.Command {
	cmds := c.Commands()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name() < cmds[j].Name() })
	return cmds
}

