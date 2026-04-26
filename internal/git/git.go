// Package git provides filesystem and URL helpers for interacting with git repositories.
// Functions here are intentionally small and dependency-light so any command can use them
// without pulling in the cobra/Factory stack.
package git

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

// RemoteURLs returns the configured URL of every remote in cwd (origin first, then others, deduped); nil if git isn't available or cwd isn't a repo.
func RemoteURLs(cwd string) []string {
	names, err := exec.Command("git", "-C", cwd, "remote").Output()
	if err != nil {
		return nil
	}

	var urls []string
	seen := map[string]bool{}
	addFor := func(name string) {
		out, err := exec.Command("git", "-C", cwd, "remote", "get-url", name).Output()
		if err != nil {
			return
		}
		raw := strings.TrimSpace(string(out))
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
		urls = append(urls, raw)
	}

	if out, err := exec.Command("git", "-C", cwd, "remote", "get-url", "origin").Output(); err == nil {
		raw := strings.TrimSpace(string(out))
		if raw != "" {
			seen[raw] = true
			urls = append(urls, raw)
		}
	}
	for name := range strings.FieldsSeq(string(names)) {
		if name == "origin" {
			continue
		}
		addFor(name)
	}
	return urls
}

// CanonicalURL reduces any supported git remote URL form (SSH short, ssh://, http(s)://) to "host/org/repo", or "" if unparseable.
func CanonicalURL(rawURL string) string {
	raw := strings.TrimSpace(rawURL)
	raw = strings.TrimSuffix(raw, "/")
	raw = strings.TrimSuffix(raw, ".git")
	if raw == "" {
		return ""
	}

	// SSH long form with scheme: ssh://user@host[:port]/path
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

	// SSH short form: user@host:path (no scheme; colon separates host from path)
	if !strings.Contains(raw, "://") {
		if at := strings.Index(raw, "@"); at > 0 {
			rest := raw[at+1:]
			if colon := strings.Index(rest, ":"); colon > 0 {
				return strings.ToLower(rest[:colon]) + "/" + rest[colon+1:]
			}
		}
	}

	// http(s)://[user[:pass]@]host[:port]/path
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
