package run

import (
	"fmt"
	"io"
	"os"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/git"
)

type localChangesValue struct {
	val *string
}

func (v *localChangesValue) String() string {
	if v.val == nil {
		return ""
	}
	return *v.val
}

func (v *localChangesValue) Set(s string) error {
	*v.val = s
	return nil
}

func (v *localChangesValue) Type() string {
	return "string"
}

func loadLocalChanges(source string, stdin io.Reader) ([]byte, error) {
	switch source {
	case "git":
		if !isGitRepoFn() {
			return nil, api.Validation(
				"not a git repository",
				"Run this command from within a git repository, or use --local-changes <path> to specify a diff file",
			)
		}
		patch, err := git.UncommittedDiff()
		if err != nil {
			return nil, api.Validation(
				"failed to generate git diff",
				"Ensure you have at least one commit in your repository",
			)
		}
		if len(patch) == 0 {
			return nil, api.Validation(
				"no uncommitted changes found",
				"Make some changes to your files before running a personal build, or use --local-changes <path> to specify a diff file",
			)
		}
		return patch, nil
	case "-":
		patch, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
		if len(patch) == 0 {
			return nil, api.Validation(
				"no changes provided via stdin",
				"Pipe a diff file to stdin, e.g.: git diff | teamcity run start Job --local-changes -",
			)
		}
		return patch, nil
	default:
		patch, err := os.ReadFile(source)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, api.Validation(
					"diff file not found: "+source,
					"Check the file path and try again",
				)
			}
			return nil, fmt.Errorf("failed to read diff file: %w", err)
		}
		if len(patch) == 0 {
			return nil, api.Validation(
				"diff file is empty: "+source,
				"Provide a non-empty diff file",
			)
		}
		return patch, nil
	}
}
