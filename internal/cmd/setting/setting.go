package setting

import (
	"fmt"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/param"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

// NewCmd builds the settings command group for a resource, using resolveID as the linked default.
func NewCmd(f *cmdutil.Factory, resource string, resolveID param.IDResolver) *cobra.Command {
	idComplete := completion.LinkedJobs()
	cmd := &cobra.Command{
		Use:   "settings",
		Short: fmt.Sprintf("Manage %s settings", resource),
		Long: fmt.Sprintf(`List, get, and set %s settings.

Settings are the general build-configuration options (build number
format, execution timeout, artifact rules, checkout rules, etc.).
Unlike parameters they always have a server default and cannot be
deleted, only changed.

The <%s-id> positional is optional when teamcity.toml binds this
repo via 'teamcity link' - the linked %s is used automatically.

See: https://www.jetbrains.com/help/teamcity/configuring-general-settings.html`, resource, resource, resource),
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newSettingListCmd(f, resource, resolveID, idComplete))
	cmd.AddCommand(newSettingGetCmd(f, resource, resolveID, idComplete))
	cmd.AddCommand(newSettingSetCmd(f, resource, resolveID, idComplete))

	return cmd
}

// resolveResourceID returns args[0] when the positional id is present, else the linked default.
func resolveResourceID(resource string, args []string, want int, resolveID param.IDResolver) (string, []string, error) {
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

type settingListOptions struct {
	json     bool
	plain    bool
	noHeader bool
	cmdutil.ViewOptions
}

// newSettingListCmd builds the `settings list` subcommand.
func newSettingListCmd(f *cmdutil.Factory, resource string, resolveID param.IDResolver, idComplete completion.CompFunc) *cobra.Command {
	opts := &settingListOptions{}

	cmd := &cobra.Command{
		Use:               fmt.Sprintf("list [%s-id]", resource),
		Short:             fmt.Sprintf("List %s settings", resource),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: idComplete,
		Example: fmt.Sprintf(`  teamcity %s settings list MyID
  teamcity %s settings list                # uses linked %s (see 'teamcity link')
  teamcity %s settings list MyID --json
  teamcity %s settings list MyID --plain
  teamcity %s settings list MyID --web`, resource, resource, resource, resource, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _, err := resolveResourceID(resource, args, 0, resolveID)
			if err != nil {
				return err
			}
			return runSettingList(f, id, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&opts.noHeader, "no-header", false, "Omit header row (use with --plain)")
	cmdutil.AddWebFlags(cmd, &opts.ViewOptions)
	cmd.MarkFlagsMutuallyExclusive("json", "plain")

	return cmd
}

// settingsListURL returns the TeamCity admin URL for a job's general settings page.
func settingsListURL(serverURL, id string) string {
	return serverURL + "/admin/editBuildTypeGeneralSettings.html?id=buildType:" + id
}

// runSettingList fetches and renders a job's settings as a table, JSON, or plain text.
func runSettingList(f *cmdutil.Factory, id string, opts *settingListOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	// Fetch first so a mistyped id is reported, then --web navigates to the settings page.
	settings, err := client.GetBuildTypeSettings(id)
	if err != nil {
		return err
	}

	if done, err := opts.EmitWebURL(f.Printer, settingsListURL(client.ServerURL(), id)); done {
		return err
	}

	if opts.json {
		return f.Printer.PrintJSON(settings)
	}

	p := f.Printer
	if settings.Count == 0 {
		p.Empty("No settings found", output.TipNoSettingsFor(id))
		return nil
	}

	headers := []string{"SETTING", "VALUE"}
	var rows [][]string
	for _, s := range settings.Property {
		rows = append(rows, []string{s.Name, s.Value})
	}

	if opts.plain {
		p.PrintPlainTable(headers, rows, opts.noHeader)
	} else {
		output.AutoSizeColumns(headers, rows, 2, 0, 1)
		p.PrintTable(headers, rows)
	}
	return nil
}

// newSettingGetCmd builds the `settings get` subcommand.
func newSettingGetCmd(f *cmdutil.Factory, resource string, resolveID param.IDResolver, idComplete completion.CompFunc) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:               fmt.Sprintf("get [%s-id] <setting>", resource),
		Short:             fmt.Sprintf("Get a %s setting value", resource),
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: settingFirstArgComplete(idComplete),
		Example: fmt.Sprintf(`  teamcity %s settings get MyID buildNumberPattern
  teamcity %s settings get executionTimeoutMin   # uses linked %s
  teamcity %s settings get MyID artifactRules`, resource, resource, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, rest, err := resolveResourceID(resource, args, 1, resolveID)
			if err != nil {
				return err
			}
			return runSettingGet(f, id, rest[0], jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

// runSettingGet prints a single setting's value as plain text or JSON.
func runSettingGet(f *cmdutil.Factory, id, name string, jsonOutput bool) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	value, err := client.GetBuildTypeSetting(id, name)
	if err != nil {
		return err
	}

	if jsonOutput {
		return f.Printer.PrintJSON(api.Setting{Name: name, Value: value})
	}

	_, _ = fmt.Fprintln(f.Printer.Out, strings.TrimRight(value, "\n"))
	return nil
}

// newSettingSetCmd builds the `settings set` subcommand.
func newSettingSetCmd(f *cmdutil.Factory, resource string, resolveID param.IDResolver, idComplete completion.CompFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("set [%s-id] <setting> <value>", resource),
		Short: fmt.Sprintf("Set a %s setting value", resource),
		Long: fmt.Sprintf(`Set or update a %s setting.

Omit the <%s-id> when this repo is linked via 'teamcity link'.`, resource, resource),
		Args:              cobra.RangeArgs(2, 3),
		ValidArgsFunction: settingFirstArgComplete(idComplete),
		Example: fmt.Sprintf(`  teamcity %s settings set MyID buildNumberPattern "2.0.%%build.counter%%"
  teamcity %s settings set executionTimeoutMin 30        # uses linked %s
  teamcity %s settings set MyID artifactRules "build/** => artifacts"`, resource, resource, resource, resource),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, rest, err := resolveResourceID(resource, args, 2, resolveID)
			if err != nil {
				return err
			}
			return runSettingSet(f, id, rest[0], rest[1])
		},
	}

	return cmd
}

// runSettingSet writes a single setting value and confirms the change.
func runSettingSet(f *cmdutil.Factory, id, name, value string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := client.SetBuildTypeSetting(id, name, value); err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}

	f.Printer.Success("Set setting %s", name)
	return nil
}

// settingFirstArgComplete applies idComplete only to the resource-id slot (args[0]).
func settingFirstArgComplete(idComplete completion.CompFunc) completion.CompFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return idComplete(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}
