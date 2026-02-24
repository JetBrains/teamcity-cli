//go:build ignore

// Script to sync CLI documentation between this repo and JetBrains/teamcity-documentation.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const externalRepo = "JetBrains/teamcity-documentation"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: go run scripts/sync-docs.go <pull|push> [--branch <branch>]\n")
		os.Exit(1)
	}

	branch := parseBranchFlag()

	switch os.Args[1] {
	case "pull":
		pull(branch)
	case "push":
		push(branch)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s (expected pull or push)\n", os.Args[1])
		os.Exit(1)
	}
}

func parseBranchFlag() string {
	for i, arg := range os.Args {
		if arg == "--branch" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

func resolveBranch(override string) string {
	if override != "" {
		fmt.Printf("Using branch: %s (override)\n", override)
		return override
	}
	out := git("", "api", "repos/"+externalRepo, "-q", ".default_branch")
	branch := strings.TrimSpace(out)
	fmt.Printf("Using branch: %s (default)\n", branch)
	return branch
}

// git runs a git (or gh) command in dir, returning stdout. Empty dir means cwd.
// On failure it prints the error and exits.
func git(dir string, args ...string) string {
	bin := "git"
	if len(args) > 0 && args[0] == "api" {
		bin = "gh"
	}
	cmd := exec.Command(bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Command failed: %s %s\n  %v\n", bin, strings.Join(args, " "), err)
		os.Exit(1)
	}
	return string(out)
}

// gitExec runs a git command with stdout/stderr connected to the terminal.
func gitExec(dir string, args ...string) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Command failed: git %s\n  %v\n", strings.Join(args, " "), err)
		os.Exit(1)
	}
}

func isTopicFile(name string) bool {
	return strings.HasPrefix(name, "teamcity-cli-") && strings.HasSuffix(name, ".md")
}

func localGIFNames() map[string]bool {
	entries, err := os.ReadDir("docs/images")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading docs/images: %v\n", err)
		os.Exit(1)
	}
	names := make(map[string]bool)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".gif") {
			names[e.Name()] = true
		}
	}
	return names
}

func pull(branchOverride string) {
	branch := resolveBranch(branchOverride)

	tmpDir, err := os.MkdirTemp("", "tc-docs-pull-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("Cloning external repo (sparse)...")
	gitExec("", "clone", "--depth", "1", "--branch", branch,
		"--filter=blob:none", "--sparse",
		"https://github.com/"+externalRepo+".git", tmpDir)
	gitExec(tmpDir, "sparse-checkout", "set", "topics", "images")

	gifNames := localGIFNames()
	var changed, added, unchanged int

	syncDir := func(srcDir, dstDir string, match func(string) bool) {
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", srcDir, err)
			os.Exit(1)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !match(name) {
				continue
			}
			switch copyFile(filepath.Join(srcDir, name), filepath.Join(dstDir, name)) {
			case "changed":
				changed++
				fmt.Printf("  updated: %s\n", filepath.Join(dstDir, name))
			case "added":
				added++
				fmt.Printf("  added:   %s\n", filepath.Join(dstDir, name))
			default:
				unchanged++
			}
		}
	}

	syncDir(filepath.Join(tmpDir, "topics"), "docs/topics", isTopicFile)
	syncDir(filepath.Join(tmpDir, "images"), "docs/images", func(n string) bool { return gifNames[n] })

	fmt.Printf("\nDone: %d updated, %d added, %d unchanged\n", changed, added, unchanged)
}

func push(branchOverride string) {
	branch := resolveBranch(branchOverride)
	syncBranch := "cli-docs-sync-" + time.Now().Format("20060102")

	tmpDir, err := os.MkdirTemp("", "tc-docs-push-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("Cloning external repo...")
	gitExec("", "clone", "--depth", "1", "--branch", branch,
		"https://github.com/"+externalRepo+".git", tmpDir)

	fmt.Printf("Creating branch %s...\n", syncBranch)
	gitExec(tmpDir, "checkout", "-b", syncBranch)

	gifNames := localGIFNames()
	var copied int

	entries, err := os.ReadDir("docs/topics")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading docs/topics: %v\n", err)
		os.Exit(1)
	}
	for _, e := range entries {
		if !e.IsDir() && isTopicFile(e.Name()) {
			copyFile(filepath.Join("docs/topics", e.Name()), filepath.Join(tmpDir, "topics", e.Name()))
			copied++
		}
	}
	for name := range gifNames {
		copyFile(filepath.Join("docs/images", name), filepath.Join(tmpDir, "images", name))
		copied++
	}

	fmt.Printf("Copied %d files\n", copied)

	if strings.TrimSpace(git(tmpDir, "status", "--porcelain")) == "" {
		fmt.Println("No changes to push.")
		return
	}

	gitExec(tmpDir, "add", "-A")
	gitExec(tmpDir, "commit", "-m", "Update TeamCity CLI documentation")
	gitExec(tmpDir, "push", "-u", "origin", syncBranch)

	fmt.Println("Creating pull request...")
	cmd := exec.Command("gh", "pr", "create",
		"--repo", externalRepo,
		"--base", branch,
		"--head", syncBranch,
		"--title", "Update TeamCity CLI documentation",
		"--body", "Automated sync of CLI documentation from teamcity-cli repo.",
	)
	cmd.Dir = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating PR: %v\n", err)
		os.Exit(1)
	}
}

func copyFile(src, dst string) string {
	srcData, err := os.ReadFile(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", src, err)
		os.Exit(1)
	}

	status := "added"
	if dstData, err := os.ReadFile(dst); err == nil {
		if bytes.Equal(dstData, srcData) {
			return "unchanged"
		}
		status = "changed"
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory for %s: %v\n", dst, err)
		os.Exit(1)
	}
	if err := os.WriteFile(dst, srcData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", dst, err)
		os.Exit(1)
	}
	return status
}
