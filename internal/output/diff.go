package output

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	udiff "github.com/aymanbagabas/go-udiff"
)

// UnifiedDiff writes a colored unified diff between two line slices, returns true if they differ.
func UnifiedDiff(w io.Writer, a, b []string, fromLabel, toLabel string, context int) (bool, error) {
	oldStr := strings.Join(a, "")
	newStr := strings.Join(b, "")
	if oldStr == newStr {
		return false, nil
	}

	edits := udiff.Strings(oldStr, newStr)
	text, err := udiff.ToUnified(fromLabel, toLabel, oldStr, edits, context)
	if err != nil {
		return false, fmt.Errorf("computing diff: %w", err)
	}
	if text == "" {
		return false, nil
	}

	for line := range strings.SplitSeq(text, "\n") {
		_, _ = fmt.Fprintln(w, ColorDiffLine(line))
	}
	return true, nil
}

var (
	tcTimestampRe = regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}\]`)
	tcTempPathRe  = regexp.MustCompile(`/(opt|mnt)/buildAgent/temp/\S+`)
	tcAgentNameRe = regexp.MustCompile(`i-[0-9a-f]{10,}-VM-\d+`)
	gitProgressRe = regexp.MustCompile(`(remote: )?(Counting|Compressing|Enumerating|Receiving|Resolving) (objects|deltas):\s+\d+%`)
)

// NormalizeBuildLog strips per-run noise (header, ANSI, timestamps, temp paths, agent IDs, git progress).
func NormalizeBuildLog(lines []string) []string {
	start := 0
	for i := range min(15, len(lines)) {
		if tcTimestampRe.MatchString(lines[i]) {
			start = i
			break
		}
	}

	out := make([]string, 0, len(lines)-start)
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if gitProgressRe.MatchString(line) {
			continue
		}
		line = TCAnsiRe.ReplaceAllString(line, "")
		line = tcTimestampRe.ReplaceAllString(line, "[]")
		line = tcTempPathRe.ReplaceAllString(line, "<tmp>")
		line = tcAgentNameRe.ReplaceAllString(line, "<agent>")
		out = append(out, line)
	}
	return out
}

// SplitLogLines splits a log string into \n-terminated lines for difflib.
func SplitLogLines(log string) []string {
	lines := strings.Split(log, "\n")
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = strings.TrimSuffix(line, "\r") + "\n"
	}
	return result
}

// ColorDiffLine applies git-style coloring to a single diff line.
func ColorDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "---"), strings.HasPrefix(line, "+++"):
		return Bold(line)
	case strings.HasPrefix(line, "@@"):
		return Cyan(line)
	case strings.HasPrefix(line, "-"):
		return Red(line)
	case strings.HasPrefix(line, "+"):
		return Green(line)
	default:
		return line
	}
}

// DiffLine prints a single line with a colored prefix (used for structured diffs).
func DiffLine(w io.Writer, prefix, color string, format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	var colorFn func(...any) string
	switch color {
	case "red":
		colorFn = Red
	case "green":
		colorFn = Green
	case "yellow":
		colorFn = Yellow
	default:
		_, _ = fmt.Fprintf(w, "    %s\n", text)
		return
	}
	_, _ = fmt.Fprintf(w, "  %s %s\n", colorFn(prefix), text)
}
