package job

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newJobStepCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "step",
		Short: "Manage job build steps",
		Long: `List, view, add, and delete a job's build steps.

A build step is a single action in a job (build configuration) - run a
script, invoke Gradle or Maven, build a Docker image, and so on. Steps
run in order and each has a runner type and a set of typed parameters.

The <job-id> positional is optional when teamcity.toml binds this repo
via 'teamcity link' - the linked job is used automatically.

See: https://www.jetbrains.com/help/teamcity/configuring-build-steps.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newJobStepListCmd(f))
	cmd.AddCommand(newJobStepViewCmd(f))
	cmd.AddCommand(newJobStepAddCmd(f))
	cmd.AddCommand(newJobStepDeleteCmd(f))

	return cmd
}

// resolveStepArgs picks the job ID from an optional leading positional (falling back to the
// linked default) and returns the remaining positionals. want is the number of trailing args
// the caller expects after the job ID.
func resolveStepArgs(f *cmdutil.Factory, args []string, want int) (string, []string, error) {
	if len(args) == want+1 {
		return f.ResolveDefaultJob(args[0]), args[1:], nil
	}
	id := f.ResolveDefaultJob("")
	if id == "" {
		return "", nil, api.Validation(
			"job id is required",
			"Pass <job-id> or run 'teamcity link' to bind a default job to this repository",
		)
	}
	return id, args, nil
}

// stepJobComplete completes the job-id slot (args[0]) only.
func stepJobComplete() completion.CompFunc {
	jobs := completion.LinkedJobs()
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return jobs(cmd, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func stepStatus(disabled bool) string {
	if disabled {
		return output.Faint("disabled")
	}
	return output.Green("enabled")
}

type jobStepListOptions struct {
	json     bool
	plain    bool
	noHeader bool
}

func newJobStepListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &jobStepListOptions{}

	cmd := &cobra.Command{
		Use:               "list [job-id]",
		Short:             "List job build steps",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: stepJobComplete(),
		Example: `  teamcity job step list MyBuild
  teamcity job step list                 # uses linked job (see 'teamcity link')
  teamcity job step list MyBuild --json
  teamcity job step list MyBuild --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, _, err := resolveStepArgs(f, args, 0)
			if err != nil {
				return err
			}
			return runJobStepList(f, jobID, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "Output in plain text format for scripting")
	cmd.Flags().BoolVar(&opts.noHeader, "no-header", false, "Omit header row (use with --plain)")
	cmd.MarkFlagsMutuallyExclusive("json", "plain")

	return cmd
}

func runJobStepList(f *cmdutil.Factory, jobID string, opts *jobStepListOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	steps, err := client.GetBuildSteps(jobID)
	if err != nil {
		return err
	}

	if opts.json {
		return f.Printer.PrintJSON(steps)
	}

	p := f.Printer
	if steps.Count == 0 {
		p.Empty("No build steps found", "Add one with 'teamcity job step add "+jobID+" --type <runner-id>'")
		return nil
	}

	headers := []string{"#", "ID", "NAME", "TYPE", "STATUS"}
	var rows [][]string
	for i, s := range steps.Step {
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			s.ID,
			s.Name,
			s.Type,
			stepStatus(s.Disabled),
		})
	}

	if opts.plain {
		p.PrintPlainTable(headers, rows, opts.noHeader)
	} else {
		output.AutoSizeColumns(headers, rows, 2, 2, 3)
		p.PrintTable(headers, rows)
	}
	return nil
}

func newJobStepViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cmdutil.ViewOptions{}

	cmd := &cobra.Command{
		Use:               "view [job-id] <step-id>",
		Short:             "View build step details",
		Aliases:           []string{"show"},
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: stepJobComplete(),
		Example: `  teamcity job step view MyBuild RUNNER_1
  teamcity job step view RUNNER_1        # uses linked job
  teamcity job step view MyBuild RUNNER_1 --json
  teamcity job step view MyBuild RUNNER_1 --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, rest, err := resolveStepArgs(f, args, 1)
			if err != nil {
				return err
			}
			return runJobStepView(f, jobID, rest[0], opts)
		},
	}

	cmdutil.AddViewFlags(cmd, opts)
	return cmd
}

func runJobStepView(f *cmdutil.Factory, jobID, stepID string, opts *cmdutil.ViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	step, err := client.GetBuildStep(jobID, stepID)
	if err != nil {
		return err
	}

	url := client.ServerURL() + "/admin/editBuildRunners.html?id=buildType:" + jobID
	if done, err := opts.EmitWebURL(f.Printer, url); done {
		return err
	}

	if opts.JSON {
		return f.Printer.PrintJSON(step)
	}

	p := f.Printer
	p.PrintField("ID", step.ID)
	p.PrintField("Name", step.Name)
	p.PrintField("Type", step.Type)
	p.PrintField("Status", stepStatus(step.Disabled))

	if len(step.Properties.Property) > 0 {
		_, _ = fmt.Fprintln(p.Out)
		headers := []string{"PARAMETER", "VALUE"}
		var rows [][]string
		for _, prop := range step.Properties.Property {
			rows = append(rows, []string{prop.Name, prop.Value})
		}
		output.AutoSizeColumns(headers, rows, 2, 0, 1)
		p.PrintTable(headers, rows)
	}
	return nil
}

type jobStepAddOptions struct {
	stepType string
	name     string
	params   []string
	json     bool
}

func newJobStepAddCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &jobStepAddOptions{}

	cmd := &cobra.Command{
		Use:   "add [job-id] --type <runner-id>",
		Short: "Add a build step to a job",
		Long: `Add a build step to a job (build configuration).

--type is the runner type ID used by TeamCity's REST API, which differs
from the display name in the UI: Command Line is "simpleRunner", Gradle
is "gradle-runner", Maven is "Maven2", and so on. Find a runner's ID by
inspecting an existing step with 'teamcity job step view', or from the
TeamCity documentation. Repeat --param key=value for each step setting;
the available keys depend on the runner type.`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: stepJobComplete(),
		Example: `  teamcity job step add MyBuild --type simpleRunner --name "Run Tests" --param use.custom.script=true --param script.content="./gradlew test"
  teamcity job step add MyBuild --type gradle-runner --name Build --param gradle.tasks=build
  teamcity job step add --type simpleRunner --param use.custom.script=true --param script.content="make"   # uses linked job`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, _, err := resolveStepArgs(f, args, 0)
			if err != nil {
				return err
			}
			return runJobStepAdd(f, jobID, opts)
		},
	}

	cmd.Flags().StringVar(&opts.stepType, "type", "", "Runner type ID (e.g. simpleRunner, gradle-runner, Maven2; see 'teamcity job step view')")
	cmd.Flags().StringVar(&opts.name, "name", "", "Step name")
	cmd.Flags().StringArrayVar(&opts.params, "param", nil, "Step parameter as key=value (repeatable)")
	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

func runJobStepAdd(f *cmdutil.Factory, jobID string, opts *jobStepAddOptions) error {
	props, err := parseStepParams(opts.params)
	if err != nil {
		return err
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	step, err := client.CreateBuildStep(jobID, api.BuildStep{
		Name:       opts.name,
		Type:       opts.stepType,
		Properties: api.PropertyList{Property: props},
	})
	if err != nil {
		return fmt.Errorf("failed to add build step: %w", err)
	}

	if opts.json {
		return f.Printer.PrintJSON(step)
	}

	name := step.Name
	if name == "" {
		name = step.Type
	}
	f.Printer.Success("Added step %q (id: %s) to job %s", name, step.ID, jobID)
	return nil
}

func parseStepParams(params []string) ([]api.Property, error) {
	var props []api.Property
	for _, p := range params {
		key, value, ok := strings.Cut(p, "=")
		if !ok || key == "" {
			return nil, api.Validation(
				fmt.Sprintf("invalid --param %q", p),
				"Use key=value, for example --param script.content=\"./gradlew test\"",
			)
		}
		props = append(props, api.Property{Name: key, Value: value})
	}
	return props, nil
}

func newJobStepDeleteCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "delete [job-id] <step-id>",
		Short:             "Delete a build step",
		Aliases:           []string{"remove", "rm"},
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: stepJobComplete(),
		Example: `  teamcity job step delete MyBuild RUNNER_1
  teamcity job step delete RUNNER_1      # uses linked job`,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, rest, err := resolveStepArgs(f, args, 1)
			if err != nil {
				return err
			}
			return runJobStepDelete(f, jobID, rest[0])
		},
	}

	return cmd
}

func runJobStepDelete(f *cmdutil.Factory, jobID, stepID string) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := client.DeleteBuildStep(jobID, stepID); err != nil {
		return fmt.Errorf("failed to delete build step: %w", err)
	}

	f.Printer.Success("Deleted step %s", stepID)
	return nil
}
