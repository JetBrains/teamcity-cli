package run

import (
	"errors"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/git"
)

// Indirection so tests can stub the git layer without touching internal/git.
var (
	isGitRepoFn       = git.IsRepo
	currentBranchFn   = currentBranch
	headRevisionFn    = headRevision
	resolveRevisionFn = resolveRevision
)

// resolveBranchFlag turns "@this" into the current git branch.
func resolveBranchFlag(branch string) (string, error) {
	if !strings.EqualFold(branch, "@this") {
		return branch, nil
	}
	if !isGitRepoFn() {
		return "", errors.New("--branch @this requires a git repository")
	}
	return currentBranchFn()
}

// resolveRevisionFlag resolves "@head" to HEAD SHA and expands short SHAs from the local repo.
func resolveRevisionFlag(revision string) (string, error) {
	if strings.EqualFold(revision, "@head") {
		if !isGitRepoFn() {
			return "", errors.New("--revision @head requires a git repository")
		}
		return headRevisionFn()
	}
	if revision != "" && len(revision) < 40 && isGitRepoFn() {
		return resolveRevisionFn(revision)
	}
	return revision, nil
}

// currentBranch wraps git.CurrentBranch with TC-flavored error messages.
func currentBranch() (string, error) {
	name, err := git.CurrentBranch()
	if errors.Is(err, git.ErrDetachedHEAD) {
		return "", api.Validation(
			"cannot determine branch: you are in detached HEAD state",
			"Check out a branch with 'git checkout <branch>' or specify --branch explicitly",
		)
	}
	if err != nil {
		return "", api.Validation(
			"failed to get current branch",
			"Ensure you are in a git repository and on a branch",
		)
	}
	return name, nil
}

// headRevision wraps git.HeadRevision with a TC-flavored error.
func headRevision() (string, error) {
	rev, err := git.HeadRevision()
	if err != nil {
		return "", api.Validation(
			"failed to resolve revision 'HEAD'",
			"Ensure you are in a git repository",
		)
	}
	return rev, nil
}

// resolveRevision wraps git.ResolveRevision with a TC-flavored error.
func resolveRevision(rev string) (string, error) {
	sha, err := git.ResolveRevision(rev)
	if err != nil {
		return "", api.Validation(
			"failed to resolve revision '"+rev+"'",
			"Ensure you are in a git repository and the revision exists",
		)
	}
	return sha, nil
}

// pushBranch wraps git.Push with a TC-flavored error.
func pushBranch(branch string) error {
	if err := git.Push(branch); err != nil {
		return api.Validation(
			"failed to push branch: "+err.Error(),
			"Ensure you have push access to the remote repository",
		)
	}
	return nil
}
