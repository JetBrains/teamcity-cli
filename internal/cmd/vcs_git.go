package cmd

import "fmt"

type GitProvider struct{}

func (g *GitProvider) Name() string                        { return "git" }
func (g *GitProvider) IsAvailable() bool                   { return isGitRepo() }
func (g *GitProvider) GetCurrentBranch() (string, error)   { return getCurrentBranch() }
func (g *GitProvider) GetHeadRevision() (string, error)    { return getHeadCommit() }
func (g *GitProvider) GetLocalDiff() ([]byte, error)       { return getGitDiff() }
func (g *GitProvider) BranchExistsOnRemote(b string) bool  { return branchExistsOnRemote(b) }
func (g *GitProvider) PushBranch(b string) error           { return pushBranch(b) }

func (g *GitProvider) FormatRevision(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}

func (g *GitProvider) DiffHint(firstRev, lastRev string) string {
	first, last := firstRev, lastRev
	if len(first) > 7 {
		first = first[:7]
	}
	if len(last) > 7 {
		last = last[:7]
	}
	return fmt.Sprintf("git diff %s^..%s", first, last)
}
