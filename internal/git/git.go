// Package git provides filesystem and shell helpers for interacting with git repositories.
// All git command-line invocations in the CLI live here; callers wrap the typed errors with
// CLI-flavored hints at the boundary.
package git

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrDetachedHEAD is returned by CurrentBranch when the working tree is in detached-HEAD state.
var ErrDetachedHEAD = errors.New("detached HEAD: not on a branch")

// RepoRoot returns the absolute path of the .git-containing ancestor of start, or ("", false) outside a working tree.
func RepoRoot(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// IsRepo reports whether the current working directory is inside a git working tree.
func IsRepo() bool {
	return exec.Command("git", "rev-parse", "--is-inside-work-tree").Run() == nil
}

// CurrentBranch returns the current branch's short name; ErrDetachedHEAD when on no branch.
func CurrentBranch() (string, error) {
	out, err := exec.Command("git", "symbolic-ref", "--short", "HEAD").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	check, checkErr := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if checkErr == nil && strings.TrimSpace(string(check)) == "HEAD" {
		return "", ErrDetachedHEAD
	}
	return "", fmt.Errorf("git symbolic-ref --short HEAD: %w", err)
}

// HeadRevision returns the SHA of HEAD.
func HeadRevision() (string, error) { return ResolveRevision("HEAD") }

// ResolveRevision returns the SHA for rev (full or short).
func ResolveRevision(rev string) (string, error) {
	out, err := exec.Command("git", "rev-parse", rev).Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", rev, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RemoteForBranch returns the configured remote for branch, defaulting to "origin".
func RemoteForBranch(branch string) string {
	out, err := exec.Command("git", "config", "--get", "branch."+branch+".remote").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "origin"
	}
	return strings.TrimSpace(string(out))
}

// BranchExistsOnRemote reports whether branch exists on its configured remote.
func BranchExistsOnRemote(branch string) bool {
	remote := RemoteForBranch(branch)
	out, err := exec.Command("git", "ls-remote", "--heads", remote, branch).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// Push runs `git push -u <remote> <branch>` for the branch's configured remote.
func Push(branch string) error {
	remote := RemoteForBranch(branch)
	out, err := exec.Command("git", "push", "-u", remote, branch).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return errors.New(msg)
		}
		return err
	}
	return nil
}

// UntrackedFiles returns files reported by `git ls-files --others --exclude-standard`.
func UntrackedFiles() ([]string, error) {
	out, err := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "\n"), nil
}

// UncommittedDiff returns `git diff HEAD` output, including untracked files via a transient intent-to-add.
func UncommittedDiff() ([]byte, error) {
	untracked, err := UntrackedFiles()
	if err != nil {
		return nil, err
	}
	if len(untracked) > 0 {
		addArgs := append([]string{"add", "-N", "--"}, untracked...)
		if exec.Command("git", addArgs...).Run() == nil {
			defer func() {
				resetArgs := append([]string{"reset", "HEAD", "--"}, untracked...)
				_ = exec.Command("git", resetArgs...).Run()
			}()
		}
	}
	out, err := exec.Command("git", "diff", "HEAD").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff HEAD: %w", err)
	}
	return out, nil
}

// CanonicalURL reduces any supported git remote URL form (SSH short, ssh://, http(s)://) to "host/org/repo", or "" if unparseable.
func CanonicalURL(rawURL string) string {
	raw := strings.TrimSpace(rawURL)
	raw = strings.TrimSuffix(raw, "/")
	raw = strings.TrimSuffix(raw, ".git")
	if raw == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(raw, "ssh://"); ok {
		if i := strings.Index(rest, "@"); i >= 0 {
			rest = rest[i+1:]
		}
		if slash := strings.Index(rest, "/"); slash > 0 {
			host, path := rest[:slash], rest[slash:]
			if colon := strings.Index(host, ":"); colon > 0 {
				host = host[:colon]
			}
			return strings.ToLower(host) + path
		}
		return strings.ToLower(rest)
	}
	if !strings.Contains(raw, "://") {
		if at := strings.Index(raw, "@"); at > 0 {
			rest := raw[at+1:]
			if colon := strings.Index(rest, ":"); colon > 0 {
				return strings.ToLower(rest[:colon]) + "/" + rest[colon+1:]
			}
		}
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		host := strings.ToLower(u.Host)
		if colon := strings.Index(host, ":"); colon > 0 {
			host = host[:colon]
		}
		return host + u.Path
	}
	return ""
}

// RepoPath extracts the "org/repo" path component from any supported git URL form, or "".
func RepoPath(rawURL string) string {
	canonical := CanonicalURL(rawURL)
	if canonical == "" {
		return ""
	}
	if i := strings.Index(canonical, "/"); i >= 0 {
		return strings.TrimPrefix(canonical[i:], "/")
	}
	return canonical
}
