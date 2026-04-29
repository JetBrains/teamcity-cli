package param

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

// ParamAPI defines the interface for parameter operations
type ParamAPI struct {
	List   func(client api.ClientInterface, id string) (*api.ParameterList, error)
	Get    func(client api.ClientInterface, id, name string) (*api.Parameter, error)
	Set    func(client api.ClientInterface, id, name, value string, secure bool) error
	Delete func(client api.ClientInterface, id, name string) error
}

// IDResolver returns a resource ID, falling back from explicit → env → linked teamcity.toml.
type IDResolver func(explicit string) string

var ProjectParamAPI = ParamAPI{
	List: func(c api.ClientInterface, id string) (*api.ParameterList, error) { return c.GetProjectParameters(id) },
	Get: func(c api.ClientInterface, id, name string) (*api.Parameter, error) {
		return c.GetProjectParameter(id, name)
	},
	Set: func(c api.ClientInterface, id, name, value string, secure bool) error {
		return c.SetProjectParameter(id, name, value, secure)
	},
	Delete: func(c api.ClientInterface, id, name string) error { return c.DeleteProjectParameter(id, name) },
}

var JobParamAPI = ParamAPI{
	List: func(c api.ClientInterface, id string) (*api.ParameterList, error) {
		return c.GetBuildTypeParameters(id)
	},
	Get: func(c api.ClientInterface, id, name string) (*api.Parameter, error) {
		return c.GetBuildTypeParameter(id, name)
	},
	Set: func(c api.ClientInterface, id, name, value string, secure bool) error {
		return c.SetBuildTypeParameter(id, name, value, secure)
	},
	Delete: func(c api.ClientInterface, id, name string) error { return c.DeleteBuildTypeParameter(id, name) },
}

// NewCmd creates a param subcommand for a resource (project or job). resolveID supplies the
// linked default when the user omits the resource-id positional.
func NewCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI, resolveID IDResolver) *cobra.Command {
	idComplete := completion.LinkedProjects()
	if resource == "job" {
		idComplete = completion.LinkedJobs()
	}
	cmd := &cobra.Command{
		Use:   "param",
		Short: fmt.Sprintf("Manage %s parameters", resource),
		Long: fmt.Sprintf(`List, get, set, and delete %s parameters.

Parameters are typed key-value pairs attached to a %s. They drive
build behavior, can reference other parameters, and may be marked
as password (secure) so their values never appear in logs.

The <%s-id> positional is optional when teamcity.toml binds this
repo via 'teamcity link' — the linked %s is used automatically.

See: https://www.jetbrains.com/help/teamcity/configuring-build-parameters.html`, resource, resource, resource, resource),
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newParamListCmd(f, resource, paramAPI, resolveID, idComplete))
	cmd.AddCommand(newParamGetCmd(f, resource, paramAPI, resolveID, idComplete))
	cmd.AddCommand(newParamSetCmd(f, resource, paramAPI, resolveID, idComplete))
	cmd.AddCommand(newParamDeleteCmd(f, resource, paramAPI, resolveID, idComplete))

	return cmd
}

// resolveResourceID picks args[0] when present, else falls back to resolveID. Returns the chosen
// ID and the trimmed args (with the id consumed if it came from a positional).
func resolveResourceID(resource string, args []string, want int, resolveID IDResolver) (string, []string, error) {
	if len(args) == want+1 {
		return args[0], args[1:], nil
	}
	id := resolveID("")
	if id == "" {
		return "", nil, api.Validation(
			resource+" id is required",
			"Pass <"+resource+"-id> or run 'teamcity link' to bind this repository",
		)
	}
	return id, args, nil
}

type paramListOptions struct {
	json     bool
	plain    bool
	noHeader bool
}

func newParamListCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI, resolveID IDResolver, idComplete completion.CompFunc) *cobra.Command {
	opts := &paramListOptions{}

	cmd := &cobra.Command{
		Use:               fmt.Sprintf("list [%s-id]", resource),
		Short:             fmt.Sprintf("List %s parameters", resource),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: idComplete,
		Example: fmt.Sprintf(`  teamcity %s param list MyID
  teamcity %s param list                # uses linked %s (see 'teamcity link')
  teamcity %s param list MyID --json
  teamcity %s param list MyID --plain`, resource, resource, resource, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _, err := resolveResourceID(resource, args, 0, resolveID)
			if err != nil {
				return err
			}
			return runParamList(f, id, opts, paramAPI)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&opts.noHeader, "no-header", false, "Omit header row (use with --plain)")
	cmd.MarkFlagsMutuallyExclusive("json", "plain")

	return cmd
}

func runParamList(f *cmdutil.Factory, id string, opts *paramListOptions, paramAPI ParamAPI) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	params, err := paramAPI.List(client, id)
	if err != nil {
		return err
	}

	if opts.json {
		return f.Printer.PrintJSON(params)
	}

	p := f.Printer
	if params.Count == 0 {
		p.Empty("No parameters found", output.TipNoParametersFor(id))
		return nil
	}

	headers := []string{"NAME", "VALUE"}
	var rows [][]string

	for _, param := range params.Property {
		value := param.Value
		if param.Type != nil && param.Type.RawValue == "password" {
			value = "********"
		}

		rows = append(rows, []string{
			param.Name,
			value,
		})
	}

	if opts.plain {
		p.PrintPlainTable(headers, rows, opts.noHeader)
	} else {
		output.AutoSizeColumns(headers, rows, 2, 0, 1)
		p.PrintTable(headers, rows)
	}
	return nil
}

func newParamGetCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI, resolveID IDResolver, idComplete completion.CompFunc) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:               fmt.Sprintf("get [%s-id] <name>", resource),
		Short:             fmt.Sprintf("Get a %s parameter value", resource),
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: paramFirstArgComplete(idComplete),
		Example: fmt.Sprintf(`  teamcity %s param get MyID MY_PARAM
  teamcity %s param get MY_PARAM         # uses linked %s
  teamcity %s param get MyID VERSION`, resource, resource, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, rest, err := resolveResourceID(resource, args, 1, resolveID)
			if err != nil {
				return err
			}
			return runParamGet(f, id, rest[0], jsonOutput, paramAPI)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func runParamGet(f *cmdutil.Factory, id, name string, jsonOutput bool, paramAPI ParamAPI) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	param, err := paramAPI.Get(client, id, name)
	if err != nil {
		return err
	}

	if jsonOutput {
		return f.Printer.PrintJSON(param)
	}

	value := param.Value
	if param.Type != nil && param.Type.RawValue == "password" {
		value = "********"
	}

	_, _ = fmt.Fprintln(f.Printer.Out, value)
	return nil
}

type paramSetOptions struct {
	secure bool
}

func newParamSetCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI, resolveID IDResolver, idComplete completion.CompFunc) *cobra.Command {
	opts := &paramSetOptions{}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("set [%s-id] <name> <value>", resource),
		Short: fmt.Sprintf("Set a %s parameter value", resource),
		Long: fmt.Sprintf(`Set or update a %s parameter.

Use --secure to mark the parameter as a password. Secure values are
stored encrypted server-side and masked in logs and UI output.

Omit the <%s-id> when this repo is linked via 'teamcity link'.`, resource, resource),
		Args:              cobra.RangeArgs(2, 3),
		ValidArgsFunction: paramFirstArgComplete(idComplete),
		Example: fmt.Sprintf(`  teamcity %s param set MyID MY_PARAM "my value"
  teamcity %s param set MY_PARAM "my value"           # uses linked %s
  teamcity %s param set MyID SECRET_KEY "****" --secure`, resource, resource, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, rest, err := resolveResourceID(resource, args, 2, resolveID)
			if err != nil {
				return err
			}
			return runParamSet(f, id, rest[0], rest[1], opts, paramAPI)
		},
	}

	cmd.Flags().BoolVar(&opts.secure, "secure", false, "Mark as secure/password parameter")

	return cmd
}

func runParamSet(f *cmdutil.Factory, id, name, value string, opts *paramSetOptions, paramAPI ParamAPI) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := paramAPI.Set(client, id, name, value, opts.secure); err != nil {
		return fmt.Errorf("failed to set parameter: %w", err)
	}

	f.Printer.Success("Set parameter %s", name)
	return nil
}

func newParamDeleteCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI, resolveID IDResolver, idComplete completion.CompFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:               fmt.Sprintf("delete [%s-id] <name>", resource),
		Short:             fmt.Sprintf("Delete a %s parameter", resource),
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: paramFirstArgComplete(idComplete),
		Example: fmt.Sprintf(`  teamcity %s param delete MyID MY_PARAM
  teamcity %s param delete MY_PARAM      # uses linked %s`, resource, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, rest, err := resolveResourceID(resource, args, 1, resolveID)
			if err != nil {
				return err
			}
			return runParamDelete(f, id, rest[0], paramAPI)
		},
	}

	return cmd
}

func runParamDelete(f *cmdutil.Factory, id, name string, paramAPI ParamAPI) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := paramAPI.Delete(client, id, name); err != nil {
		return fmt.Errorf("failed to delete parameter: %w", err)
	}

	f.Printer.Success("Deleted parameter %s", name)
	return nil
}

// paramFirstArgComplete applies idComplete only to the resource-id slot (args[0]).
func paramFirstArgComplete(idComplete completion.CompFunc) completion.CompFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return idComplete(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}
