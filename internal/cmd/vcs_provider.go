package cmd

// VCSProvider abstracts version control operations used by the CLI.
type VCSProvider interface {
	Name() string
	IsAvailable() bool
	GetCurrentBranch() (string, error)
	GetHeadRevision() (string, error)
	GetLocalDiff() ([]byte, error)
	BranchExistsOnRemote(branch string) bool
	PushBranch(branch string) error
	FormatRevision(rev string) string
	DiffHint(firstRev, lastRev string) string
}

func DetectVCS() VCSProvider {
	for _, p := range []VCSProvider{&GitProvider{}, &PerforceProvider{}} {
		if p.IsAvailable() {
			return p
		}
	}
	return nil
}

func DetectVCSByName(name string) VCSProvider {
	switch name {
	case "git":
		return &GitProvider{}
	case "p4", "perforce":
		return &PerforceProvider{}
	default:
		return nil
	}
}
