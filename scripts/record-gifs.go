//go:build ignore

// Script to generate GIF demos from VHS tape files in docs/tapes/.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	darkTheme  = "Dark+"
	lightTheme = "Catppuccin Latte"
	tapesDir   = "docs/tapes"
	imagesDir  = "docs/images"
)

func main() {
	genDark := true
	genLight := true
	var tapeNames []string

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--dark-only":
			genLight = false
		case "--light-only":
			genDark = false
		case "--list":
			listTapes()
			return
		case "-h", "--help":
			usage()
			return
		default:
			tapeNames = append(tapeNames, os.Args[i])
		}
	}

	if _, err := exec.LookPath("vhs"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: vhs is not installed. Install it with: brew install vhs\n")
		os.Exit(1)
	}

	ensureTeamcityCommand()

	tapeFiles := resolveTapes(tapeNames)
	if len(tapeFiles) == 0 {
		fmt.Fprintf(os.Stderr, "No tape files found in %s/\n", tapesDir)
		os.Exit(1)
	}

	os.MkdirAll(imagesDir, 0755)

	fmt.Printf("Generating GIFs from %d tape(s)...\n\n", len(tapeFiles))

	failed := 0
	for _, tape := range tapeFiles {
		if genDark {
			if err := recordTape(tape, darkTheme, "dark"); err != nil {
				fmt.Fprintf(os.Stderr, "  !! %v\n", err)
				failed++
			}
		}
		if genLight {
			if err := recordTape(tape, lightTheme, "light"); err != nil {
				fmt.Fprintf(os.Stderr, "  !! %v\n", err)
				failed++
			}
		}
		fmt.Println()
	}

	fmt.Printf("Done. Generated GIFs in %s/\n", imagesDir)
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %d recording(s) failed.\n", failed)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`Usage: go run scripts/record-gifs.go [OPTIONS] [tape_name...]

Generate GIF demos from VHS tape files in docs/tapes/

Options:
  --dark-only     Generate only dark theme variants
  --light-only    Generate only light theme variants
  --list          List available tape files
  -h, --help      Show this help message

Examples:
  go run scripts/record-gifs.go                    # All tapes, both themes
  go run scripts/record-gifs.go run-list            # Single tape
  go run scripts/record-gifs.go --dark-only         # All tapes, dark only
  go run scripts/record-gifs.go --light-only auth-login run-list
`)
}

func listTapes() {
	matches, err := filepath.Glob(filepath.Join(tapesDir, "*.tape"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing tapes: %v\n", err)
		os.Exit(1)
	}
	for _, m := range matches {
		name := strings.TrimSuffix(filepath.Base(m), ".tape")
		fmt.Println(name)
	}
}

func ensureTeamcityCommand() {
	if _, err := exec.LookPath("teamcity"); err == nil {
		return
	}

	tcPath, err := exec.LookPath("tc")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: neither 'teamcity' nor 'tc' found in PATH\n")
		os.Exit(1)
	}

	linkDir := filepath.Dir(tcPath)
	linkPath := filepath.Join(linkDir, "teamcity")
	os.Remove(linkPath)
	if err := os.Symlink(tcPath, linkPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating symlink %s -> %s: %v\n", linkPath, tcPath, err)
		os.Exit(1)
	}
	fmt.Printf("Creating symlink: %s -> %s\n", linkPath, tcPath)
}

func resolveTapes(names []string) []string {
	if len(names) == 0 {
		matches, err := filepath.Glob(filepath.Join(tapesDir, "*.tape"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error globbing tapes: %v\n", err)
			os.Exit(1)
		}
		return matches
	}

	var files []string
	for _, name := range names {
		path := filepath.Join(tapesDir, name+".tape")
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: tape not found: %s\n", path)
			continue
		}
		files = append(files, path)
	}
	return files
}

func recordTape(tapeFile, theme, suffix string) error {
	name := strings.TrimSuffix(filepath.Base(tapeFile), ".tape")

	var output string
	if suffix == "dark" {
		output = filepath.Join(imagesDir, name+"_dark.gif")
	} else {
		output = filepath.Join(imagesDir, name+".gif")
	}

	fmt.Printf("Recording: %s (%s)...\n", name, suffix)

	content, err := os.ReadFile(tapeFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", tapeFile, err)
	}

	tape := strings.ReplaceAll(string(content), "{{THEME}}", theme)
	tape = strings.ReplaceAll(tape, "{{OUTPUT}}", fmt.Sprintf("%q", output))

	if strings.Contains(tape, "{{RUN_ID}}") {
		runID, err := triggerBuild()
		if err != nil {
			return fmt.Errorf("triggering build for {{RUN_ID}}: %w", err)
		}
		fmt.Printf("  Triggered build %s for live recording\n", runID)
		tape = strings.ReplaceAll(tape, "{{RUN_ID}}", runID)
	}

	cmd := exec.Command("vhs", "-")
	cmd.Stdin = strings.NewReader(tape)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vhs failed for %s (%s): %w", name, suffix, err)
	}

	info, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("failed to generate %s", output)
	}
	fmt.Printf("  -> %s (%s)\n", output, formatSize(info.Size()))
	return nil
}

func triggerBuild() (string, error) {
	cmd := exec.Command("teamcity", "run", "start", "Sandbox_Build", "--branch", "main", "--json")
	cmd.Env = append(os.Environ(), "TEAMCITY_URL=https://cli.teamcity.com")

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	var result struct {
		ID json.Number `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return "", fmt.Errorf("parsing build response: %w", err)
	}
	id := result.ID.String()
	if id == "" {
		return "", fmt.Errorf("empty build ID in response")
	}
	return id, nil
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
	)
	switch {
	case bytes >= mb:
		return fmt.Sprintf("%.1fM", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.0fK", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
