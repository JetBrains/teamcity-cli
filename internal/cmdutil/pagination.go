package cmdutil

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

const continueTokenVersion = 1
const continueConflictsAnnotation = "teamcity-cli/continue-conflicts"

type continueToken struct {
	Version int    `json:"version"`
	Command string `json:"command"`
	Path    string `json:"path"`
	Offset  int    `json:"offset,omitzero"`
}

// ListPageInfo contains pagination metadata for paginated list commands.
type ListPageInfo struct {
	Count          int
	ContinuePath   string
	ContinueOffset int
}

// EncodeContinueToken converts internal pagination state into an opaque CLI token.
func EncodeContinueToken(commandPath, path string, offset int) (string, error) {
	if path == "" {
		return "", nil
	}

	payload, err := json.Marshal(continueToken{
		Version: continueTokenVersion,
		Command: commandPath,
		Path:    path,
		Offset:  offset,
	})
	if err != nil {
		return "", fmt.Errorf("encode continuation token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodeContinueToken validates and decodes a continuation token for a command.
func DecodeContinueToken(commandPath, token string) (string, int, error) {
	payload, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", 0, fmt.Errorf("invalid continuation token")
	}

	var decoded continueToken
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", 0, fmt.Errorf("invalid continuation token")
	}

	switch {
	case decoded.Version != continueTokenVersion:
		return "", 0, fmt.Errorf("unsupported continuation token version")
	case decoded.Command != commandPath:
		return "", 0, fmt.Errorf("continuation token does not belong to %q", commandPath)
	case decoded.Path == "":
		return "", 0, fmt.Errorf("invalid continuation token")
	case decoded.Offset < 0:
		return "", 0, fmt.Errorf("invalid continuation token")
	default:
		return decoded.Path, decoded.Offset, nil
	}
}

// SetContinueConflicts records flags that are incompatible with --continue for a command.
func SetContinueConflicts(cmd *cobra.Command, flagNames ...string) {
	if len(flagNames) == 0 {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[continueConflictsAnnotation] = strings.Join(flagNames, ",")
}

// ValidateContinueConflicts checks whether --continue is combined with incompatible flags.
func ValidateContinueConflicts(cmd *cobra.Command) error {
	if cmd.Flags().Lookup("continue") == nil || !cmd.Flags().Changed("continue") {
		return nil
	}

	conflictList := cmd.Annotations[continueConflictsAnnotation]
	if conflictList == "" {
		return nil
	}

	conflicts := make([]string, 0, 4)
	for flagName := range strings.SplitSeq(conflictList, ",") {
		if flagName != "" && cmd.Flags().Changed(flagName) && !slices.Contains(conflicts, flagName) {
			conflicts = append(conflicts, flagName)
		}
	}
	if len(conflicts) == 0 {
		return nil
	}
	if len(conflicts) == 1 {
		return fmt.Errorf("--continue cannot be used with --%s", conflicts[0])
	}
	return fmt.Errorf("--continue cannot be used with flags: --%s", strings.Join(conflicts, ", --"))
}
