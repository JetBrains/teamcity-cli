package run

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
)

var (
	isGitRepoFn       = isGitRepo
	currentBranchFn   = getCurrentBranch
	headRevisionFn    = getHeadRevision
	resolveRevisionFn = resolveRevision
)

// resolveBranchFlag turns "@this" into the current git branch. Other values pass through.
func resolveBranchFlag(branch string) (string, error) {
	if !strings.EqualFold(branch, "@this") {
		return branch, nil
	}
	if !isGitRepoFn() {
		return "", errors.New("--branch @this requires a git repository")
	}
	return currentBranchFn()
}

// resolveRevisionFlag turns "@head" into the current HEAD SHA and expands short SHAs to full ones.
// Values of 40+ chars pass through unchanged, as do empty strings or values when not in a git repo.
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

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	err := cmd.Run()
	return err == nil
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		checkCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		checkOut, checkErr := checkCmd.Output()
		if checkErr == nil && strings.TrimSpace(string(checkOut)) == "HEAD" {
			return "", api.Validation(
				"cannot determine branch: you are in detached HEAD state",
				"Check out a branch with 'git checkout <branch>' or specify --branch explicitly",
			)
		}
		return "", api.Validation(
			"failed to get current branch",
			"Ensure you are in a git repository and on a branch",
		)
	}
	return strings.TrimSpace(string(out)), nil
}

func getHeadRevision() (string, error) {
	return resolveRevision("HEAD")
}

func resolveRevision(rev string) (string, error) {
	cmd := exec.Command("git", "rev-parse", rev)
	out, err := cmd.Output()
	if err != nil {
		return "", api.Validation(
			"failed to resolve revision '"+rev+"'",
			"Ensure you are in a git repository and the revision exists",
		)
	}
	return strings.TrimSpace(string(out)), nil
}

func branchExistsOnRemote(branch string) bool {
	remote := getRemoteForBranch(branch)
	cmd := exec.Command("git", "ls-remote", "--heads", remote, branch)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func getRemoteForBranch(branch string) string {
	cmd := exec.Command("git", "config", "--get", "branch."+branch+".remote")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "origin"
	}
	return strings.TrimSpace(string(out))
}

func pushBranch(branch string) error {
	remote := getRemoteForBranch(branch)
	cmd := exec.Command("git", "push", "-u", remote, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr != "" {
			return api.Validation(
				"failed to push branch: "+outStr,
				"Ensure you have push access to the remote repository",
			)
		}
		return api.Validation(
			"failed to push branch",
			"Ensure you have push access to the remote repository",
		)
	}
	return nil
}

func getUntrackedFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	outStr := strings.TrimSpace(string(out))
	if outStr == "" {
		return nil, nil
	}

	return strings.Split(outStr, "\n"), nil
}
