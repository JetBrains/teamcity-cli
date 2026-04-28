package cmdutil

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/link"
)

type linkResolver struct {
	once  sync.Once
	scope link.PathScope
	found bool
}

// SkipLinkLookup pre-resolves this Factory's link to empty, isolating Resolve* from the host's teamcity.toml.
func (f *Factory) SkipLinkLookup() {
	if f.link == nil {
		f.link = &linkResolver{}
	}
	// Consume the Once with a no-op so the real load in linkScope never runs.
	f.link.once.Do(func() {})
}

func (f *Factory) linkScope() (link.PathScope, bool) {
	if f.link == nil {
		f.link = &linkResolver{}
	}
	f.link.once.Do(func() {
		cwd, err := os.Getwd()
		if err != nil {
			return
		}
		path, ok := link.Find(cwd)
		if !ok {
			return
		}
		cfg, err := link.Load(path)
		if err != nil {
			if f.Printer != nil {
				f.Printer.Warn("ignoring %s: %v", path, err)
			}
			return
		}
		srv := cfg.Match(config.GetServerURL())
		if srv == nil {
			return
		}
		f.link.scope = srv.Resolve(link.RelPath(filepath.Dir(path), cwd))
		f.link.found = true
	})
	return f.link.scope, f.link.found
}

// HasLinkContext reports whether a teamcity.toml in the cwd hierarchy matched the active server.
func (f *Factory) HasLinkContext() bool {
	_, ok := f.linkScope()
	return ok
}

// ResolveProject returns explicit, then TEAMCITY_PROJECT, then the linked scope's project.
func (f *Factory) ResolveProject(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv(config.EnvProject); v != "" {
		return v
	}
	if s, ok := f.linkScope(); ok {
		return s.Project
	}
	return ""
}

// ResolveDefaultJob returns explicit, then TEAMCITY_JOB, then scope.Job, then a single Jobs entry.
func (f *Factory) ResolveDefaultJob(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv(config.EnvJob); v != "" {
		return v
	}
	s, ok := f.linkScope()
	if !ok {
		return ""
	}
	if s.Job != "" {
		return s.Job
	}
	if len(s.Jobs) == 1 {
		return s.Jobs[0]
	}
	return ""
}
