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
		checkCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		checkOut, _ := checkCmd.Output()
		if strings.TrimSpace(string(checkOut)) == "HEAD" {
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

// getRemoteName returns the name of the remote for the current branch, or "origin" if no upstream is configured
func getRemoteName() string {
	cmd := exec.Command("git", "config", "--get", "branch."+getCurrentBranchSafe()+".remote")
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "origin"
	}
	return strings.TrimSpace(string(out))
}

// getCurrentBranchSafe returns the current branch or empty string on error
func getCurrentBranchSafe() string {
	branch, err := getCurrentBranch()
	if err != nil {
		return ""
	}
	return branch
}

// pushBranch pushes the given branch to its remote with -u flag
func pushBranch(branch string) error {
	remote := getRemoteName()
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
