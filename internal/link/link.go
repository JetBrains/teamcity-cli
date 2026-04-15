// Package link reads and writes teamcity.toml: a committed file binding a
// repository to one or more TeamCity servers, with optional per-path overrides
// inside each server entry.
package link

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the canonical committed-config filename.
const FileName = "teamcity.toml"

// PathScope is a per-subdirectory binding inside a server entry.
type PathScope struct {
	Project string   `toml:"project,omitempty"`
	Job     string   `toml:"job,omitempty"`
	Jobs    []string `toml:"jobs,omitempty"`
}

// Server is one [[server]] entry: a TC instance with default project/job and
// optional per-path overrides.
type Server struct {
	URL     string               `toml:"url"`
	Project string               `toml:"project,omitempty"`
	Job     string               `toml:"job,omitempty"`
	Jobs    []string             `toml:"jobs,omitempty"`
	Paths   map[string]PathScope `toml:"paths,omitempty"`
}

// Resolve returns the effective scope for rel — deepest path match overlaid on server-level defaults.
func (s *Server) Resolve(rel string) PathScope {
	rel = strings.Trim(rel, "/")
	var best PathScope
	bestLen := -1
	for path, scope := range s.Paths {
		if rel == path || strings.HasPrefix(rel, path+"/") {
			if len(path) > bestLen {
				best = scope
				bestLen = len(path)
			}
		}
	}
	out := PathScope{
		Project: cmp.Or(best.Project, s.Project),
		Job:     cmp.Or(best.Job, s.Job),
	}
	if len(best.Jobs) > 0 {
		out.Jobs = best.Jobs
	} else {
		out.Jobs = s.Jobs
	}
	return out
}

// Config is the parsed teamcity.toml.
type Config struct {
	Servers []Server `toml:"server,omitempty"`
}

// Match returns the entry for serverURL (compared with normalizeURL), or nil.
func (c *Config) Match(serverURL string) *Server {
	want := normalizeURL(serverURL)
	if want == "" {
		return nil
	}
	for i := range c.Servers {
		if normalizeURL(c.Servers[i].URL) == want {
			return &c.Servers[i]
		}
	}
	return nil
}

// UpsertScope sets the scope for serverURL at path (path="" = server top-level); siblings preserved.
func (c *Config) UpsertScope(serverURL, path string, scope PathScope) {
	s := c.findOrCreate(serverURL)
	if path == "" {
		s.Project = scope.Project
		s.Job = scope.Job
		s.Jobs = scope.Jobs
		return
	}
	if s.Paths == nil {
		s.Paths = map[string]PathScope{}
	}
	s.Paths[path] = scope
}

func (c *Config) findOrCreate(serverURL string) *Server {
	norm := normalizeURL(serverURL)
	for i := range c.Servers {
		if normalizeURL(c.Servers[i].URL) == norm {
			return &c.Servers[i]
		}
	}
	c.Servers = append(c.Servers, Server{URL: serverURL})
	return &c.Servers[len(c.Servers)-1]
}

// Find walks up from start looking for FileName; returns absolute path + true on hit.
// If start is inside a git working tree, the walk stops at the repo root so that
// an unrelated teamcity.toml in an ancestor directory (e.g. $HOME) cannot leak in.
func Find(start string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	stop := gitRootBoundary(dir)
	for {
		candidate := filepath.Join(dir, FileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		if stop != "" && dir == stop {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// gitRootBoundary returns the absolute path of the .git-containing ancestor of dir,
// or "" if dir is not inside a git working tree. Uses only filesystem stats so it
// has no dependency on the git binary.
func gitRootBoundary(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// RelPath returns target relative to root using forward slashes ("" at root); resolves symlinks first.
func RelPath(root, target string) string {
	root, _ = filepath.Abs(root)
	target, _ = filepath.Abs(target)
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if t, err := filepath.EvalSymlinks(target); err == nil {
		target = t
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// Load parses path into a Config.
func Load(path string) (*Config, error) {
	var c Config
	if _, err := toml.DecodeFile(path, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes c to path.
func Save(path string, c *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return toml.NewEncoder(f).Encode(c)
}

// normalizeURL trims trailing slashes and lowercases scheme+host for stable comparison.
func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimRight(u, "/")
	if i := strings.Index(u, "://"); i > 0 {
		head := strings.ToLower(u[:i+3])
		rest := u[i+3:]
		if j := strings.IndexAny(rest, "/?#"); j > 0 {
			return head + strings.ToLower(rest[:j]) + rest[j:]
		}
		return head + strings.ToLower(rest)
	}
	return strings.ToLower(u)
}
