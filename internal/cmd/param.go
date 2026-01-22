package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/output"
)

// paramAPI defines the interface for parameter operations
type paramAPI struct {
	list   func(client *api.Client, id string) (*api.ParameterList, error)
	get    func(client *api.Client, id, name string) (*api.Parameter, error)
	set    func(client *api.Client, id, name, value string, secure bool) error
	delete func(client *api.Client, id, name string) error
}

var projectParamAPI = paramAPI{
	list:   func(c *api.Client, id string) (*api.ParameterList, error) { return c.GetProjectParameters(id) },
	get:    func(c *api.Client, id, name string) (*api.Parameter, error) { return c.GetProjectParameter(id, name) },
	set:    func(c *api.Client, id, name, value string, secure bool) error { return c.SetProjectParameter(id, name, value, secure) },
	delete: func(c *api.Client, id, name string) error { return c.DeleteProjectParameter(id, name) },
}

var jobParamAPI = paramAPI{
	list:   func(c *api.Client, id string) (*api.ParameterList, error) { return c.GetBuildTypeParameters(id) },
	get:    func(c *api.Client, id, name string) (*api.Parameter, error) { return c.GetBuildTypeParameter(id, name) },
	set:    func(c *api.Client, id, name, value string, secure bool) error { return c.SetBuildTypeParameter(id, name, value, secure) },
	delete: func(c *api.Client, id, name string) error { return c.DeleteBuildTypeParameter(id, name) },
}

// newParamCmd creates a param subcommand for a resource (project or job)
func newParamCmd(resource string, api paramAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "param",
		Short: fmt.Sprintf("Manage %s parameters", resource),
		Long:  fmt.Sprintf("List, get, set, and delete %s parameters.", resource),
	}

	cmd.AddCommand(newParamListCmd(resource, api))
	cmd.AddCommand(newParamGetCmd(resource, api))
	cmd.AddCommand(newParamSetCmd(resource, api))
	cmd.AddCommand(newParamDeleteCmd(resource, api))

	return cmd
}

type paramListOptions struct {
	json bool
}

func newParamListCmd(resource string, api paramAPI) *cobra.Command {
	opts := &paramListOptions{}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("list <%s-id>", resource),
		Short: fmt.Sprintf("List %s parameters", resource),
		Long:  fmt.Sprintf("List all parameters for a %s.", resource),
		Args:  cobra.ExactArgs(1),
		Example: fmt.Sprintf(`  tc %s param list MyID
  tc %s param list MyID --json`, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParamList(args[0], opts, api)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runParamList(id string, opts *paramListOptions, api paramAPI) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	params, err := api.list(client, id)
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

	widths := output.ColumnWidths(10, 40, 40, 60)

	for _, p := range params.Property {
		value := p.Value
		if p.Type != nil && p.Type.RawValue == "password" {
			value = "********"
		}

		rows = append(rows, []string{
			output.Truncate(p.Name, widths[0]),
			output.Truncate(value, widths[1]),
		})
	}

	output.PrintTable(headers, rows)
	return nil
}

func newParamGetCmd(resource string, api paramAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("get <%s-id> <name>", resource),
		Short: fmt.Sprintf("Get a %s parameter value", resource),
		Long:  fmt.Sprintf("Get the value of a specific %s parameter.", resource),
		Args:  cobra.ExactArgs(2),
		Example: fmt.Sprintf(`  tc %s param get MyID MY_PARAM
  tc %s param get MyID VERSION`, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParamGet(args[0], args[1], api)
		},
	}

	return cmd
}

func runParamGet(id, name string, api paramAPI) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	param, err := api.get(client, id, name)
	if err != nil {
		return err
	}

	value := param.Value
	if param.Type != nil && param.Type.RawValue == "password" {
		value = "********"
	}

	fmt.Println(value)
	return nil
}

type paramSetOptions struct {
	secure bool
}

func newParamSetCmd(resource string, api paramAPI) *cobra.Command {
	opts := &paramSetOptions{}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("set <%s-id> <name> <value>", resource),
		Short: fmt.Sprintf("Set a %s parameter value", resource),
		Long:  fmt.Sprintf("Set or update a %s parameter value.", resource),
		Args:  cobra.ExactArgs(3),
		Example: fmt.Sprintf(`  tc %s param set MyID MY_PARAM "my value"
  tc %s param set MyID SECRET_KEY "****" --secure`, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParamSet(args[0], args[1], args[2], opts, api)
		},
	}

	cmd.Flags().BoolVar(&opts.secure, "secure", false, "Mark as secure/password parameter")

	return cmd
}

func runParamSet(id, name, value string, opts *paramSetOptions, api paramAPI) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := api.set(client, id, name, value, opts.secure); err != nil {
		return fmt.Errorf("failed to set parameter: %w", err)
	}

	output.Success("Set parameter %s", name)
	return nil
}

func newParamDeleteCmd(resource string, api paramAPI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("delete <%s-id> <name>", resource),
		Short:   fmt.Sprintf("Delete a %s parameter", resource),
		Long:    fmt.Sprintf("Delete a parameter from a %s.", resource),
		Args:    cobra.ExactArgs(2),
		Example: fmt.Sprintf(`  tc %s param delete MyID MY_PARAM`, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runParamDelete(args[0], args[1], api)
		},
	}

	return cmd
}

func runParamDelete(id, name string, api paramAPI) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if err := api.delete(client, id, name); err != nil {
		return fmt.Errorf("failed to delete parameter: %w", err)
	}

	output.Success("Deleted parameter %s", name)
	return nil
}
