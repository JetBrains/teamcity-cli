package cmd

// VCSProvider abstracts version control operations used by the CLI.
// Implementations exist for Git and Perforce.
type VCSProvider interface {
	// Name returns the VCS type name (e.g., "git", "perforce").
	Name() string

	// IsAvailable returns true if this VCS is detected in the current working directory.
	IsAvailable() bool

	// GetCurrentBranch returns the current branch or stream name.
	// For Git, this is the branch name. For Perforce, this is the stream depot path.
	GetCurrentBranch() (string, error)

	// GetHeadRevision returns the current revision identifier.
	// For Git, this is a commit SHA. For Perforce, this is a changelist number.
	GetHeadRevision() (string, error)

	// GetLocalDiff returns a unified diff of uncommitted local changes.
	// For Git, this is `git diff HEAD` including untracked files.
	// For Perforce, this is `p4 diff -du` of pending changelists.
	GetLocalDiff() ([]byte, error)

	// BranchExistsOnRemote checks whether the branch/stream exists on the remote server.
	BranchExistsOnRemote(branch string) bool

	// PushBranch pushes the given branch to the remote.
	// For Perforce, this is a no-op (changes are submitted directly).
	PushBranch(branch string) error

	// FormatRevision returns a short display form of a revision identifier.
	// For Git, truncates SHA to 7 chars. For Perforce, returns the changelist number as-is.
	FormatRevision(rev string) string

	// FormatVCSBranch converts a user-facing branch name to the VCS-specific ref format
	// used in TeamCity API calls.
	// For Git: "main" -> "refs/heads/main". For Perforce: returns depot path as-is.
	FormatVCSBranch(branch string) string

	// DiffHint returns a human-readable hint for viewing the full diff between two revisions.
	// For Git: "git diff abc1234^..def5678". For Perforce: "p4 changes -l @123,@456".
	DiffHint(firstRev, lastRev string) string
}

// DetectVCS returns the VCS provider for the current working directory,
// or nil if no supported VCS is detected.
func DetectVCS() VCSProvider {
	providers := []VCSProvider{
		&GitProvider{},
		&PerforceProvider{},
	}
	for _, p := range providers {
		if p.IsAvailable() {
			return p
		}
	}
	return nil
}

// DetectVCSByName returns the VCS provider matching the given name, or nil.
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
