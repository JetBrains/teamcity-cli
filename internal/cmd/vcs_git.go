package cmd

import (
	"fmt"
	"strings"
)

// GitProvider implements VCSProvider for Git repositories.
type GitProvider struct{}

func (g *GitProvider) Name() string {
	return "git"
}

func (g *GitProvider) IsAvailable() bool {
	return isGitRepo()
}

func (g *GitProvider) GetCurrentBranch() (string, error) {
	return getCurrentBranch()
}

func (g *GitProvider) GetHeadRevision() (string, error) {
	return getHeadCommit()
}

func (g *GitProvider) GetLocalDiff() ([]byte, error) {
	return getGitDiff()
}

func (g *GitProvider) BranchExistsOnRemote(branch string) bool {
	return branchExistsOnRemote(branch)
}

func (g *GitProvider) PushBranch(branch string) error {
	return pushBranch(branch)
}

func (g *GitProvider) FormatRevision(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}

func (g *GitProvider) FormatVCSBranch(branch string) string {
	if branch != "" && !strings.HasPrefix(branch, "refs/") {
		return "refs/heads/" + branch
	}
	return branch
}

func (g *GitProvider) DiffHint(firstRev, lastRev string) string {
	first := firstRev
	last := lastRev
	if len(first) > 7 {
		first = first[:7]
	}
	if len(last) > 7 {
		last = last[:7]
	}
	return fmt.Sprintf("git diff %s^..%s", first, last)
}
