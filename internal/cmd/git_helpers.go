package cmd

import (
	"os/exec"
	"strings"

	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
)

// isGitRepo checks if the current directory is inside a git repository
func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	err := cmd.Run()
	return err == nil
}

// getCurrentBranch returns the current branch name.
func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		// Check if we're in detached HEAD state
		checkCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		checkOut, checkErr := checkCmd.Output()
		if checkErr == nil && strings.TrimSpace(string(checkOut)) == "HEAD" {
			return "", tcerrors.WithSuggestion(
				"cannot determine branch: you are in detached HEAD state",
				"Check out a branch with 'git checkout <branch>' or specify --branch explicitly",
			)
		}
		return "", tcerrors.WithSuggestion(
			"failed to get current branch",
			"Ensure you are in a git repository and on a branch",
		)
	}
	return strings.TrimSpace(string(out)), nil
}

// branchExistsOnRemote checks if the branch exists on the remote
func branchExistsOnRemote(branch string) bool {
	remote := getRemoteForBranch(branch)
	cmd := exec.Command("git", "ls-remote", "--heads", remote, branch)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// getRemoteForBranch returns the name of the remote for the given branch, or "origin" if no upstream is configured
func getRemoteForBranch(branch string) string {
	cmd := exec.Command("git", "config", "--get", "branch."+branch+".remote")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "origin"
	}
	return strings.TrimSpace(string(out))
}

// pushBranch pushes the given branch to its remote with -u flag
func pushBranch(branch string) error {
	remote := getRemoteForBranch(branch)
	cmd := exec.Command("git", "push", "-u", remote, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr != "" {
			return tcerrors.WithSuggestion(
				"failed to push branch: "+outStr,
				"Ensure you have push access to the remote repository",
			)
		}
		return tcerrors.WithSuggestion(
			"failed to push branch",
			"Ensure you have push access to the remote repository",
		)
	}
	return nil
}

// getUntrackedFiles returns a list of untracked files in the repository
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
