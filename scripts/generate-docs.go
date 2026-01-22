//go:build ignore
// +build ignore

// Script to generate CLI documentation in README.md from cobra commands.
// Extracts command structure, flags, and examples from the actual code.
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tiulpin/teamcity-cli/internal/cmd"
)

// Command group ordering and display names
var groupOrder = []string{"auth", "run", "job", "project", "queue", "api"}
var groupNames = map[string]string{
	"auth":    "Authentication",
	"run":     "Runs",
	"job":     "Jobs",
	"project": "Projects",
	"queue":   "Queue",
	"api":     "API",
}

func main() {
	rootCmd := cmd.GetRootCmd()

	var docs bytes.Buffer
	generateDocs(&docs, rootCmd)

	content, err := os.ReadFile("README.md")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading README.md: %v\n", err)
		os.Exit(1)
	}

	newContent := replaceBetweenMarkers(string(content), docs.String())

	if len(os.Args) > 1 && os.Args[1] == "--check" {
		if string(content) != newContent {
			fmt.Println("README.md is out of date. Run 'just docs' to update it.")
			os.Exit(1)
		}
		fmt.Println("README.md is up to date.")
		return
	}

	if err := os.WriteFile("README.md", []byte(newContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing README.md: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("README.md updated.")
}

func replaceBetweenMarkers(content, generated string) string {
	re := regexp.MustCompile(`(?s)<!-- COMMANDS_START -->.*<!-- COMMANDS_END -->`)
	replacement := "<!-- COMMANDS_START -->\n\n" + generated + "<!-- COMMANDS_END -->"
	return re.ReplaceAllString(content, replacement)
}

func generateDocs(buf *bytes.Buffer, rootCmd *cobra.Command) {
	cmdMap := make(map[string]*cobra.Command)
	for _, c := range rootCmd.Commands() {
		if c.Name() != "help" && c.Name() != "completion" {
			cmdMap[c.Name()] = c
		}
	}

	// Generate docs in the specified order
	for i, groupName := range groupOrder {
		c, exists := cmdMap[groupName]
		if !exists {
			continue
		}

		displayName := groupNames[groupName]
		buf.WriteString(fmt.Sprintf("## %s\n\n", displayName))

		generateGroupDocs(buf, c)

		// Add separator between groups, but not after the last one
		if i < len(groupOrder)-1 {
			buf.WriteString("---\n\n")
		}
	}
}

func generateGroupDocs(buf *bytes.Buffer, cmd *cobra.Command) {
	subCmds := getSortedCommands(cmd)

	if len(subCmds) == 0 {
		generateLeafCommandDocs(buf, cmd)
		return
	}

	for _, sub := range subCmds {
		subSubCmds := getSortedCommands(sub)
		if len(subSubCmds) > 0 {
			for _, subSub := range subSubCmds {
				generateSubcommandDocs(buf, cmd.Name(), sub.Name()+" "+subSub.Name(), subSub)
			}
		} else {
			generateSubcommandDocs(buf, cmd.Name(), sub.Name(), sub)
		}
	}
}

func generateLeafCommandDocs(buf *bytes.Buffer, cmd *cobra.Command) {
	if cmd.Long != "" {
		parts := strings.SplitN(cmd.Long, "\n\n", 2)
		buf.WriteString(fmt.Sprintf("%s\n\n", parts[0]))
	} else if cmd.Short != "" {
		buf.WriteString(fmt.Sprintf("%s\n\n", cmd.Short))
	}

	if cmd.Example != "" {
		buf.WriteString("```bash\n")
		buf.WriteString(cleanupExample(cmd.Example))
		buf.WriteString("\n```\n\n")
	}

	writeOptions(buf, cmd)
}

func generateSubcommandDocs(buf *bytes.Buffer, parentName, subName string, cmd *cobra.Command) {
	buf.WriteString(fmt.Sprintf("### %s %s\n\n", parentName, subName))

	if cmd.Long != "" {
		parts := strings.SplitN(cmd.Long, "\n\n", 2)
		buf.WriteString(fmt.Sprintf("%s\n\n", parts[0]))
	} else if cmd.Short != "" {
		buf.WriteString(fmt.Sprintf("%s\n\n", cmd.Short))
	}

	if parentName == "auth" && strings.HasPrefix(subName, "login") {
		buf.WriteString("This will:\n")
		buf.WriteString("1. Prompt for your TeamCity server URL\n")
		buf.WriteString("2. Open your browser to generate an access token\n")
		buf.WriteString("3. Validate and store the token securely\n\n")
	}

	if cmd.Example != "" {
		buf.WriteString("```bash\n")
		buf.WriteString(cleanupExample(cmd.Example))
		buf.WriteString("\n```\n\n")
	}

	writeOptions(buf, cmd)

	if parentName == "auth" && strings.HasPrefix(subName, "login") {
		buf.WriteString("**Environment variables** (for CI/CD):\n\n")
		buf.WriteString("```bash\n")
		buf.WriteString("export TEAMCITY_URL=\"https://teamcity.example.com\"\n")
		buf.WriteString("export TEAMCITY_TOKEN=\"your-access-token\"\n")
		buf.WriteString("```\n\n")
	}

	if parentName == "run" && subName == "log" {
		buf.WriteString("**Log viewer features:**\n")
		buf.WriteString("- **Mouse/Touchpad scrolling** – Scroll naturally with your trackpad\n")
		buf.WriteString("- **Search** – Press `/` to search forward, `?` to search backward\n")
		buf.WriteString("- **Navigation** – `n`/`N` for next/previous match, `g`/`G` for top/bottom\n")
		buf.WriteString("- **Filter** – `&pattern` to show only matching lines\n")
		buf.WriteString("- **Quit** – Press `q` to exit\n\n")
		buf.WriteString("Use `--raw` to bypass the pager.\n\n")
	}
}

func writeOptions(buf *bytes.Buffer, cmd *cobra.Command) {
	flags := getFlags(cmd)
	if len(flags) == 0 {
		return
	}

	buf.WriteString("**Options:**\n")
	for _, f := range flags {
		if f.Shorthand != "" {
			buf.WriteString(fmt.Sprintf("- `-%s, --%s` – %s\n", f.Shorthand, f.Name, f.Usage))
		} else {
			buf.WriteString(fmt.Sprintf("- `--%s` – %s\n", f.Name, f.Usage))
		}
	}
	buf.WriteString("\n")
}

func getSortedCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := cmd.Commands()
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name() < cmds[j].Name()
	})
	return cmds
}

func getFlags(cmd *cobra.Command) []*pflag.Flag {
	var flags []*pflag.Flag
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "help" {
			flags = append(flags, f)
		}
	})
	return flags
}

func cleanupExample(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var result []string
	for _, line := range lines {
		result = append(result, strings.TrimPrefix(line, "  "))
	}
	return strings.Join(result, "\n")
}
