package project

import (
	"cmp"
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/completion"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCloudCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cloud",
		Short: "Manage cloud profiles, images, and instances",
		Long: `List and manage cloud profiles, images, and instances for a project.

A cloud profile binds a project to a cloud provider (AWS, GCP, Azure,
Kubernetes, ...) and defines one or more images that TeamCity uses to
start ephemeral build agents on demand.

See: https://www.jetbrains.com/help/teamcity/agent-cloud-profile.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newCloudProfileCmd(f))
	cmd.AddCommand(newCloudImageCmd(f))
	cmd.AddCommand(newCloudInstanceCmd(f))

	return cmd
}

func newCloudProfileCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage cloud profiles",
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newCloudProfileListCmd(f))
	cmd.AddCommand(newCloudProfileViewCmd(f))

	return cmd
}

type cloudProfileListOptions struct {
	project string
	cmdutil.ListFlags
}

func newCloudProfileListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cloudProfileListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List cloud profiles",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Example: `  teamcity project cloud profile list
  teamcity project cloud profile list --project MyProject
  teamcity project cloud profile list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.CloudProfileFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func (opts *cloudProfileListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	profiles, err := client.GetCloudProfiles(api.CloudProfilesOptions{
		ProjectID: opts.project,
		Limit:     opts.Limit,
		Fields:    fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"ID", "NAME", "TYPE"}
	var rows [][]string

	for _, p := range profiles.Profiles {
		rows = append(rows, []string{
			p.ID,
			p.Name,
			p.CloudProviderID,
		})
	}

	return &cmdutil.ListResult{
		JSON:     profiles,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{1}},
		EmptyMsg: "No cloud profiles found",
	}, nil
}

type cloudProfileViewOptions struct {
	json bool
}

func newCloudProfileViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cloudProfileViewOptions{}

	cmd := &cobra.Command{
		Use:     "view <profile>",
		Short:   "View cloud profile details",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project cloud profile view aws-prod
  teamcity project cloud profile view aws-prod --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCloudProfileView(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runCloudProfileView(f *cmdutil.Factory, locator string, opts *cloudProfileViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	profile, err := client.GetCloudProfile(locator)
	if err != nil {
		return err
	}

	if opts.json {
		return f.Printer.PrintJSON(profile)
	}

	p := f.Printer
	_, _ = fmt.Fprintf(p.Out, "%s\n", output.Cyan(profile.Name))
	_, _ = fmt.Fprintf(p.Out, "ID: %s\n", profile.ID)
	if profile.CloudProviderID != "" {
		_, _ = fmt.Fprintf(p.Out, "Type: %s\n", profile.CloudProviderID)
	}
	if profile.Project != nil && profile.Project.ID != "" {
		_, _ = fmt.Fprintf(p.Out, "Project: %s\n", profile.Project.ID)
	}

	return nil
}

func newCloudImageCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage cloud images",
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newCloudImageListCmd(f))
	cmd.AddCommand(newCloudImageViewCmd(f))
	cmd.AddCommand(newCloudImageStartCmd(f))

	return cmd
}

type cloudImageListOptions struct {
	profile string
	project string
	cmdutil.ListFlags
}

func newCloudImageListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cloudImageListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List cloud images",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Example: `  teamcity project cloud image list
  teamcity project cloud image list --project MyProject --profile aws-prod
  teamcity project cloud image list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.CloudImageFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmd.Flags().StringVar(&opts.profile, "profile", "", "Filter by cloud profile")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func (opts *cloudImageListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	images, err := client.GetCloudImages(api.CloudImagesOptions{
		ProjectID: opts.project,
		Profile:   opts.profile,
		Limit:     opts.Limit,
		Fields:    fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"NAME", "PROFILE", "ID"}
	var rows [][]string

	for _, img := range images.Images {
		profileName := ""
		if img.Profile != nil {
			profileName = img.Profile.Name
		}

		rows = append(rows, []string{
			img.Name,
			profileName,
			img.ID,
		})
	}

	return &cmdutil.ListResult{
		JSON:     images,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 2}},
		EmptyMsg: "No cloud images found",
	}, nil
}

type cloudImageViewOptions struct {
	json bool
}

func newCloudImageViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cloudImageViewOptions{}

	cmd := &cobra.Command{
		Use:     "view <image>",
		Short:   "View cloud image details",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project cloud image view ubuntu-22-large
  teamcity project cloud image view ubuntu-22-large --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCloudImageView(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runCloudImageView(f *cmdutil.Factory, locator string, opts *cloudImageViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	image, err := client.GetCloudImage(locator)
	if err != nil {
		return err
	}

	if opts.json {
		return f.Printer.PrintJSON(image)
	}

	p := f.Printer
	_, _ = fmt.Fprintf(p.Out, "%s\n", output.Cyan(image.Name))
	_, _ = fmt.Fprintf(p.Out, "ID: %s\n", image.ID)
	if image.Profile != nil {
		_, _ = fmt.Fprintf(p.Out, "Profile: %s\n", image.Profile.Name)
	}

	return nil
}

type cloudImageStartOptions struct {
	json bool
}

func newCloudImageStartCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cloudImageStartOptions{}

	cmd := &cobra.Command{
		Use:   "start <image>",
		Short: "Start a cloud instance from an image",
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity project cloud image start ubuntu-22-large
  teamcity project cloud image start ubuntu-22-large --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCloudImageStart(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runCloudImageStart(f *cmdutil.Factory, selector string, opts *cloudImageStartOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	image, err := client.GetCloudImage(selector)
	if err != nil {
		return err
	}

	instance, err := client.StartCloudInstance(image.ID)
	if err != nil {
		return err
	}

	if opts.json {
		return f.Printer.PrintJSON(instance)
	}

	f.Printer.Success("Started instance %s from image %s", instance.ID, cmp.Or(image.Name, selector))
	return nil
}

func newCloudInstanceCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Manage cloud instances",
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newCloudInstanceListCmd(f))
	cmd.AddCommand(newCloudInstanceViewCmd(f))
	cmd.AddCommand(newCloudInstanceStopCmd(f))

	return cmd
}

type cloudInstanceListOptions struct {
	image   string
	project string
	cmdutil.ListFlags
}

func newCloudInstanceListCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cloudInstanceListOptions{}

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List cloud instances",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Example: `  teamcity project cloud instance list
  teamcity project cloud instance list --project MyProject --image ubuntu-22-large
  teamcity project cloud instance list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdutil.RunList(f, cmd, &opts.ListFlags, &api.CloudInstanceFields, opts.fetch)
		},
	}

	cmd.Flags().StringVarP(&opts.project, "project", "p", "", "Filter by project ID")
	cmd.Flags().StringVar(&opts.image, "image", "", "Filter by cloud image")
	cmdutil.AddListFlags(cmd, &opts.ListFlags, 100)

	_ = cmd.RegisterFlagCompletionFunc("project", completion.LinkedProjects())

	return cmd
}

func (opts *cloudInstanceListOptions) fetch(client api.ClientInterface, fields []string) (*cmdutil.ListResult, error) {
	instances, err := client.GetCloudInstances(api.CloudInstancesOptions{
		ProjectID: opts.project,
		Image:     opts.image,
		Limit:     opts.Limit,
		Fields:    fields,
	})
	if err != nil {
		return nil, err
	}

	headers := []string{"NAME", "STATE", "IMAGE", "AGENT", "STARTED", "ID"}
	var rows [][]string

	for _, inst := range instances.Instances {
		imageName := ""
		if inst.Image != nil {
			imageName = inst.Image.Name
		}
		agentName := ""
		if inst.Agent != nil {
			agentName = inst.Agent.Name
		}
		started := ""
		if inst.StartDate != "" {
			if t, err := api.ParseTeamCityTime(inst.StartDate); err == nil {
				started = output.RelativeTime(t.Local())
			}
		}

		rows = append(rows, []string{
			inst.Name,
			inst.State,
			imageName,
			agentName,
			started,
			inst.ID,
		})
	}

	return &cmdutil.ListResult{
		JSON:     instances,
		Table:    cmdutil.ListTable{Headers: headers, Rows: rows, FlexCols: []int{0, 2, 3, 5}},
		EmptyMsg: "No cloud instances found",
	}, nil
}

type cloudInstanceViewOptions struct {
	json bool
}

func newCloudInstanceViewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cloudInstanceViewOptions{}

	cmd := &cobra.Command{
		Use:     "view <instance>",
		Short:   "View cloud instance details",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: `  teamcity project cloud instance view i-0245b46070c443201
  teamcity project cloud instance view i-0245b46070c443201 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCloudInstanceView(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runCloudInstanceView(f *cmdutil.Factory, locator string, opts *cloudInstanceViewOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	instance, err := client.GetCloudInstance(locator)
	if err != nil {
		return err
	}

	if opts.json {
		return f.Printer.PrintJSON(instance)
	}

	p := f.Printer
	_, _ = fmt.Fprintf(p.Out, "%s\n", output.Cyan(cmp.Or(instance.Name, instance.ID)))
	_, _ = fmt.Fprintf(p.Out, "ID: %s\n", instance.ID)
	if instance.State != "" {
		_, _ = fmt.Fprintf(p.Out, "State: %s\n", instance.State)
	}
	if instance.Image != nil {
		_, _ = fmt.Fprintf(p.Out, "Image: %s\n", instance.Image.Name)
	}
	if instance.Agent != nil {
		_, _ = fmt.Fprintf(p.Out, "Agent: %s\n", instance.Agent.Name)
	}
	if instance.StartDate != "" {
		if t, err := api.ParseTeamCityTime(instance.StartDate); err == nil {
			_, _ = fmt.Fprintf(p.Out, "Started: %s\n", output.RelativeTime(t.Local()))
		}
	}

	return nil
}

type cloudInstanceStopOptions struct {
	force bool
}

func newCloudInstanceStopCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &cloudInstanceStopOptions{}

	cmd := &cobra.Command{
		Use:   "stop <instance>",
		Short: "Stop a cloud instance",
		Long:  `Stop a running cloud instance. Use --force to terminate immediately.`,
		Args:  cobra.ExactArgs(1),
		Example: `  teamcity project cloud instance stop i-0245b46070c443201
  teamcity project cloud instance stop i-0245b46070c443201 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCloudInstanceStop(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.force, "force", false, "Force-stop the instance immediately")

	return cmd
}

func runCloudInstanceStop(f *cmdutil.Factory, locator string, opts *cloudInstanceStopOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	if err := client.StopCloudInstance(locator, opts.force); err != nil {
		return err
	}

	if opts.force {
		f.Printer.Success("Force-stopped instance %s", locator)
	} else {
		f.Printer.Success("Stopped instance %s", locator)
	}
	return nil
}
