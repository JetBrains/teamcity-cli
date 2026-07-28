//go:build ignore

// Script to vendor the teamcity-cli agent skill from the single source of
// truth — JetBrains/TeamCitySkills — into skills/teamcity-cli/, so that
// //go:embed all:skills bundles the source-of-truth content into the shipped
// binary.
//
// Only skills/teamcity-cli/ is synced. skills/migrate-to-teamcity/ is a
// CLI-local skill and is intentionally left untouched.
//
// Usage:
//
//	go run scripts/fetch-skills.go [--ref <branch|tag>]
//
// --ref defaults to "main". Override with a tag to pin a released bundle
// (see TW-101972 for the versioned nightly bundle).
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	externalRepo = "JetBrains/TeamCitySkills"
	// The single skill sourced from TeamCitySkills. migrate-to-teamcity is
	// deliberately NOT listed — it is maintained in this repo.
	skillPath = "skills/teamcity-cli"
)

func main() {
	ref := parseRefFlag()
	if ref == "" {
		ref = "main"
	}

	tmp, err := os.MkdirTemp("", "teamcity-skills-*")
	if err != nil {
		fatal("mktemp: %v", err)
	}
	defer os.RemoveAll(tmp)

	fmt.Printf("Fetching %s@%s %s ...\n", externalRepo, ref, skillPath)
	run("git", "clone", "--depth", "1", "--branch", ref,
		"https://github.com/"+externalRepo+".git", tmp)

	src := filepath.Join(tmp, skillPath)
	if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err != nil {
		fatal("fetched bundle is missing %s/SKILL.md — refusing to vendor a wrong/empty skill: %v", skillPath, err)
	}

	// Replace the vendored copy: remove the old tree, then copy the fetched one.
	if err := os.RemoveAll(skillPath); err != nil {
		fatal("removing stale %s: %v", skillPath, err)
	}
	if err := copyDir(src, skillPath); err != nil {
		fatal("copying %s: %v", skillPath, err)
	}

	n := countFiles(skillPath)
	fmt.Printf("Vendored %d file(s) into %s from %s@%s\n", n, skillPath, externalRepo, ref)
}

func parseRefFlag() string {
	for i, arg := range os.Args {
		if arg == "--ref" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}
	return ""
}

// copyDir recursively copies src into dst, preserving the tree layout.
// Implemented in pure Go so the script runs identically on Linux, macOS, and
// Windows CI agents (no `cp`/`xcopy` dependency).
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func countFiles(root string) int {
	n := 0
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func run(bin string, args ...string) {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("%s %v: %v", bin, args, err)
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "fetch-skills: "+format+"\n", a...)
	os.Exit(1)
}
