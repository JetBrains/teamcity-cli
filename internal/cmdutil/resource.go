package cmdutil

import (
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/spf13/cobra"
)

// IDResolver returns a resource ID, falling back from explicit → env → linked teamcity.toml.
type IDResolver func(explicit string) string

// MissingIDError is the standard error when an owner ID is neither given nor linked.
func MissingIDError(noun string) error {
	return api.Validation(
		noun+" id is required",
		"Pass <"+noun+"-id> or run 'teamcity link' to bind this repository",
	)
}

// ResolveOwnerID returns args[0] as the owner ID when present (with want trailing args), else the linked default.
func ResolveOwnerID(noun string, args []string, want int, resolve IDResolver) (string, []string, error) {
	if len(args) == want+1 {
		return args[0], args[1:], nil
	}
	id := resolve("")
	if id == "" {
		return "", nil, MissingIDError(noun)
	}
	return id, args, nil
}

// CompleteOwnerID completes only the leading owner-id slot (args[0]).
func CompleteOwnerID(idComplete completion.CompFunc) completion.CompFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return idComplete(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

// ListOptions are the standard list output flags; it embeds ViewOptions for JSON and EmitWebURL.
type ListOptions struct {
	ViewOptions
	Plain    bool
	NoHeader bool
}

// AddFlags registers --json/--plain/--no-header, plus --web when web is true.
func (o *ListOptions) AddFlags(cmd *cobra.Command, web bool) {
	cmd.Flags().BoolVar(&o.JSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&o.Plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&o.NoHeader, "no-header", false, "Omit header row (use with --plain)")
	cmd.MarkFlagsMutuallyExclusive("json", "plain")
	if web {
		AddWebFlags(cmd, &o.ViewOptions)
	}
}
