package param

import (
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
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

// NewCmd creates a param subcommand for a resource (project or job)
func NewCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "param",
		Short: fmt.Sprintf("Manage %s parameters", resource),
		Long:  fmt.Sprintf("List, get, set, and delete %s parameters.", resource),
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newParamListCmd(f, resource, paramAPI))
	cmd.AddCommand(newParamGetCmd(f, resource, paramAPI))
	cmd.AddCommand(newParamSetCmd(f, resource, paramAPI))
	cmd.AddCommand(newParamDeleteCmd(f, resource, paramAPI))

	return cmd
}

type paramListOptions struct {
	json bool
}

func newParamListCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI) *cobra.Command {
	opts := &paramListOptions{}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("list <%s-id>", resource),
		Short: fmt.Sprintf("List %s parameters", resource),
		Long:  fmt.Sprintf("List all parameters for a %s.", resource),
		Args:  cobra.ExactArgs(1),
		Example: fmt.Sprintf(`  teamcity %s param list MyID
  teamcity %s param list MyID --json`, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParamList(f, args[0], opts, paramAPI)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

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
		return output.PrintJSON(params)
	}

	if params.Count == 0 {
		fmt.Println("No parameters found")
		return nil
	}

	headers := []string{"NAME", "VALUE"}
	var rows [][]string

	for _, p := range params.Property {
		value := p.Value
		if p.Type != nil && p.Type.RawValue == "password" {
			value = "********"
		}

		rows = append(rows, []string{
			p.Name,
			value,
		})
	}

	output.AutoSizeColumns(headers, rows, 2, 0, 1)
	output.PrintTable(headers, rows)
	return nil
}

func newParamGetCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("get <%s-id> <name>", resource),
		Short: fmt.Sprintf("Get a %s parameter value", resource),
		Long:  fmt.Sprintf("Get the value of a specific %s parameter.", resource),
		Args:  cobra.ExactArgs(2),
		Example: fmt.Sprintf(`  teamcity %s param get MyID MY_PARAM
  teamcity %s param get MyID VERSION`, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParamGet(f, args[0], args[1], paramAPI)
		},
	}

	return cmd
}

func runParamGet(f *cmdutil.Factory, id, name string, paramAPI ParamAPI) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	p, err := paramAPI.Get(client, id, name)
	if err != nil {
		return err
	}

	value := p.Value
	if p.Type != nil && p.Type.RawValue == "password" {
		value = "********"
	}

	fmt.Println(value)
	return nil
}

type paramSetOptions struct {
	secure bool
}

func newParamSetCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI) *cobra.Command {
	opts := &paramSetOptions{}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("set <%s-id> <name> <value>", resource),
		Short: fmt.Sprintf("Set a %s parameter value", resource),
		Long:  fmt.Sprintf("Set or update a %s parameter value.", resource),
		Args:  cobra.ExactArgs(3),
		Example: fmt.Sprintf(`  teamcity %s param set MyID MY_PARAM "my value"
  teamcity %s param set MyID SECRET_KEY "****" --secure`, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParamSet(f, args[0], args[1], args[2], opts, paramAPI)
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

	output.Success("Set parameter %s", name)
	return nil
}

func newParamDeleteCmd(f *cmdutil.Factory, resource string, paramAPI ParamAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("delete <%s-id> <name>", resource),
		Short:   fmt.Sprintf("Delete a %s parameter", resource),
		Long:    fmt.Sprintf("Delete a parameter from a %s.", resource),
		Args:    cobra.ExactArgs(2),
		Example: fmt.Sprintf(`  teamcity %s param delete MyID MY_PARAM`, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParamDelete(f, args[0], args[1], paramAPI)
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

	output.Success("Deleted parameter %s", name)
	return nil
}
