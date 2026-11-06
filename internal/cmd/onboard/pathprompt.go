package onboard

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/charmbracelet/huh"
)

// promptDir asks for a directory path, completing subdirectories with ctrl+e, and echoes the answer like cmdutil.PromptString.
func promptDir(f *cmdutil.Factory, title, description string, validate func(string) error) (string, error) {
	var input string
	field := huh.NewInput().
		Title(title).
		Description(description).
		Validate(validate).
		Value(&input).
		SuggestionsFunc(func() []string { return dirSuggestions(input) }, &input)
	if err := cmdutil.Prompt(f.Printer, field); err != nil {
		return "", err
	}
	_, _ = fmt.Fprintf(f.Printer.Out, "%s: %s\n", title, output.Cyan(input))
	return input, nil
}

// dirSuggestions returns the subdirectories completing typed, keeping its prefix as written (e.g. ~); dot-dirs only when typed asks for them.
func dirSuggestions(typed string) []string {
	if typed == "" || strings.ContainsAny(typed, `"'`) {
		return nil
	}
	dir, base := filepath.Split(typed)
	parent, err := normalizePath(cmp.Or(dir, "."))
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		// Case-sensitive: accepting a suggestion keeps the typed prefix, so a case mismatch would produce a wrong path.
		if !strings.HasPrefix(name, base) || (strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".")) {
			continue
		}
		if !isDir(parent, e) {
			continue
		}
		out = append(out, dir+name+string(filepath.Separator))
	}
	return out
}

// isDir reports whether e is a directory or a symlink to one.
func isDir(parent string, e fs.DirEntry) bool {
	if e.Type()&fs.ModeSymlink == 0 {
		return e.IsDir()
	}
	info, err := os.Stat(filepath.Join(parent, e.Name()))
	return err == nil && info.IsDir()
}
