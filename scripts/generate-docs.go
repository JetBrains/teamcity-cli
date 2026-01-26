//go:build ignore

// Script to generate CLI documentation in README.md from cobra commands.
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Preferred ordering (unlisted commands added alphabetically at end).
var preferredOrder = []string{"auth", "run", "job", "project", "queue", "agent", "pool", "api"}

// Custom display names for commands that need special treatment.
var displayNames = map[string]string{
	"api":  "API",
	"auth": "Authentication",
	"pool": "Agent Pools",
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
	return re.ReplaceAllString(content, "<!-- COMMANDS_START -->\n\n"+generated+"<!-- COMMANDS_END -->")
}

func generateDocs(buf *bytes.Buffer, rootCmd *cobra.Command) {
	// Collect commands
	cmds := make(map[string]*cobra.Command)
	for _, c := range rootCmd.Commands() {
		if c.Name() != "help" && c.Name() != "completion" {
			cmds[c.Name()] = c
		}
	}

	// Order: preferred first, then remaining alphabetically
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

	for i, name := range names {
		buf.WriteString(fmt.Sprintf("## %s\n\n", displayName(name)))
		generateGroupDocs(buf, cmds[name])
		if i < len(names)-1 {
			buf.WriteString("---\n\n")
		}
	}
}

func displayName(name string) string {
	if dn, ok := displayNames[name]; ok {
		return dn
	}
	return cases.Title(language.English).String(name) + "s"
}

func generateGroupDocs(buf *bytes.Buffer, c *cobra.Command) {
	subCmds := sortedCommands(c)
	if len(subCmds) == 0 {
		writeCommandDoc(buf, "", c)
		return
	}

	for _, sub := range subCmds {
		subSubs := sortedCommands(sub)
		if len(subSubs) > 0 {
			for _, subSub := range subSubs {
				writeCommandDoc(buf, c.Name()+" "+sub.Name(), subSub)
			}
		} else {
			writeCommandDoc(buf, c.Name(), sub)
		}
	}
}

func writeCommandDoc(buf *bytes.Buffer, prefix string, c *cobra.Command) {
	if prefix != "" {
		buf.WriteString(fmt.Sprintf("### %s %s\n\n", prefix, c.Name()))
	}

	if c.Long != "" {
		buf.WriteString(c.Long + "\n\n")
	} else if c.Short != "" {
		buf.WriteString(c.Short + "\n\n")
	}

	if c.Example != "" {
		buf.WriteString("```bash\n" + cleanExample(c.Example) + "\n```\n\n")
	}

	if flags := getFlags(c); len(flags) > 0 {
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
}

func sortedCommands(c *cobra.Command) []*cobra.Command {
	cmds := c.Commands()
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name() < cmds[j].Name() })
	return cmds
}

func getFlags(c *cobra.Command) (flags []*pflag.Flag) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name != "help" {
			flags = append(flags, f)
		}
	})
	return
}

func cleanExample(s string) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		lines = append(lines, strings.TrimPrefix(line, "  "))
	}
	return strings.Join(lines, "\n")
}
