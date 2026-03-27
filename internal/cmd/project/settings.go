package project

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSettingsCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Manage versioned settings",
		Long: `View and manage versioned settings (Kotlin DSL) for a project.

Versioned settings allow you to store project configuration as code in a VCS repository.
This enables version control, code review, and automated deployment of CI/CD configuration.

See: https://www.jetbrains.com/help/teamcity/storing-project-settings-in-version-control.html`,
		Args: cobra.NoArgs,
		RunE: cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newProjectSettingsStatusCmd(f))
	cmd.AddCommand(newProjectSettingsExportCmd(f))
	cmd.AddCommand(newProjectSettingsValidateCmd(f))

	return cmd
}

type projectSettingsStatusOptions struct {
	json bool
}

func newProjectSettingsStatusCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &projectSettingsStatusOptions{}

	cmd := &cobra.Command{
		Use:   "status <project-id>",
		Short: "Show versioned settings sync status",
		Long: `Show the synchronization status of versioned settings for a project.

Displays:
- Whether versioned settings are enabled
- Current sync state (up-to-date, pending changes, errors)
- Last successful sync timestamp
- VCS root and format information
- Any warnings or errors from the last sync attempt`,
		Example: `  teamcity project settings status MyProject
  teamcity project settings status MyProject --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectSettingsStatus(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runProjectSettingsStatus(f *cmdutil.Factory, projectID string, opts *projectSettingsStatusOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	project, err := client.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	cfg, configErr := client.GetVersionedSettingsConfig(projectID)
	status, statusErr := client.GetVersionedSettingsStatus(projectID)

	if opts.json {
		result := map[string]any{
			"project": project,
		}
		if configErr == nil {
			result["config"] = cfg
		}
		if statusErr == nil {
			result["status"] = status
		}
		if configErr != nil {
			result["configError"] = configErr.Error()
		}
		if statusErr != nil {
			result["statusError"] = statusErr.Error()
		}
		return f.Printer.PrintJSON(result)
	}

	p := f.Printer
	if configErr != nil {
		_, _ = fmt.Fprintf(p.Out, "%s %s %s %s\n", output.Yellow("!"), output.Cyan(project.Name), output.Faint("·"), "not configured")
		_, _ = fmt.Fprintf(p.Out, "\n%s\n", output.Faint(configErr.Error()))
		return nil
	}

	statusIcon := output.Green("✓")
	statusLabel := "synchronized"
	if statusErr != nil {
		statusIcon = output.Red("✗")
		statusLabel = "unavailable"
	} else {
		if syncingStatus := getSyncingStatus(status.Message); syncingStatus != "" {
			statusIcon = output.Cyan("⟳")
			statusLabel = syncingStatus
		} else {
			switch status.Type {
			case "warning":
				statusIcon = output.Yellow("!")
				statusLabel = "warning"
			case "error":
				statusIcon = output.Red("✗")
				statusLabel = "error"
			}
		}
	}

	header := output.Cyan(project.Name)
	if project.ID != project.Name {
		header += " " + output.Faint("("+project.ID+")")
	}
	_, _ = fmt.Fprintf(p.Out, "%s %s %s %s\n", statusIcon, header, output.Faint("·"), statusLabel)

	_, _ = fmt.Fprintln(p.Out)
	_, _ = fmt.Fprintf(p.Out, "%-12s %s\n", output.Faint("Format"), formatSettingsFormat(cfg.Format))
	_, _ = fmt.Fprintf(p.Out, "%-12s %s\n", output.Faint("Sync"), cfg.SynchronizationMode)
	_, _ = fmt.Fprintf(p.Out, "%-12s %s\n", output.Faint("Build"), formatBuildMode(cfg.BuildSettingsMode))
	if cfg.VcsRootID != "" {
		vcsRoot := cfg.VcsRootID
		if cfg.SettingsPath != "" {
			vcsRoot += " @ " + cfg.SettingsPath
		}
		_, _ = fmt.Fprintf(p.Out, "%-12s %s\n", output.Faint("VCS Root"), vcsRoot)
	}

	if statusErr != nil {
		_, _ = fmt.Fprintf(p.Out, "\n%s\n", output.Faint(statusErr.Error()))
		return nil
	}

	if status.DslOutdated {
		_, _ = fmt.Fprintf(p.Out, "\n%s DSL scripts need to be regenerated\n", output.Yellow("!"))
	}

	if status.Timestamp != "" {
		_, _ = fmt.Fprintf(p.Out, "\n%-12s %s\n", output.Faint("Last sync"), formatRelativeTime(status.Timestamp))
	}

	if status.Message != "" && status.Type != "info" {
		_, _ = fmt.Fprintf(p.Out, "%-12s %s\n", output.Faint("Message"), output.Faint(status.Message))
	}

	_, _ = fmt.Fprintf(p.Out, "\n%-12s %s\n", output.Faint("View"), output.Faint(project.WebURL+"&tab=versionedSettings"))

	return nil
}

type projectSettingsExportOptions struct {
	kotlin         bool
	xml            bool
	output         string
	useRelativeIds bool
}

func newProjectSettingsExportCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &projectSettingsExportOptions{}

	cmd := &cobra.Command{
		Use:   "export <project-id>",
		Short: "Export project settings as Kotlin DSL or XML",
		Long: `Export project settings as a ZIP archive containing Kotlin DSL or XML configuration.

The exported archive can be used to:
- Version control your CI/CD configuration
- Migrate settings between TeamCity instances
- Review settings as code

By default, exports in Kotlin DSL format.`,
		Example: `  # Export as Kotlin DSL (default)
  teamcity project settings export MyProject

  # Export as Kotlin DSL explicitly
  teamcity project settings export MyProject --kotlin

  # Export as XML
  teamcity project settings export MyProject --xml

  # Save to specific file
  teamcity project settings export MyProject -o settings.zip`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectSettingsExport(f, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.kotlin, "kotlin", false, "Export as Kotlin DSL (default)")
	cmd.Flags().BoolVar(&opts.xml, "xml", false, "Export as XML")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output file path (default: projectSettings.zip)")
	cmd.Flags().BoolVar(&opts.useRelativeIds, "relative-ids", true, "Use relative IDs in exported settings")
	cmd.MarkFlagsMutuallyExclusive("kotlin", "xml")

	return cmd
}

func runProjectSettingsExport(f *cmdutil.Factory, projectID string, opts *projectSettingsExportOptions) error {
	client, err := f.Client()
	if err != nil {
		return err
	}

	format := "kotlin"
	if opts.xml {
		format = "xml"
	}

	outputFile := opts.output
	if outputFile == "" {
		outputFile = "projectSettings.zip"
	}

	data, err := client.ExportProjectSettings(projectID, format, opts.useRelativeIds)
	if err != nil {
		return fmt.Errorf("failed to export settings: %w", err)
	}

	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	_, _ = fmt.Fprintf(f.Printer.Out, "Exported %s settings to %s (%d bytes)\n", format, outputFile, len(data))
	return nil
}

func formatSettingsFormat(f string) string {
	switch strings.ToLower(f) {
	case "kotlin":
		return "Kotlin"
	case "xml":
		return "XML"
	default:
		return f
	}
}

func formatBuildMode(mode string) string {
	switch mode {
	case "useFromVCS":
		return "from VCS"
	case "useCurrentByDefault":
		return "prefer current"
	default:
		return mode
	}
}

func formatRelativeTime(ts string) string {
	t, err := time.Parse("Mon Jan 2 15:04:05 MST 2006", ts)
	if err != nil {
		return ts
	}
	local := t.Local()
	return fmt.Sprintf("%s (%s)", output.RelativeTime(local), local.Format("Jan 2 15:04"))
}

func getSyncingStatus(message string) string {
	lowerMsg := strings.ToLower(message)

	if strings.Contains(lowerMsg, "running dsl") {
		return "running DSL"
	}
	if strings.Contains(lowerMsg, "resolving maven dependencies") {
		return "resolving dependencies"
	}
	if strings.Contains(lowerMsg, "loading project settings from vcs") {
		return "loading from VCS"
	}
	if strings.Contains(lowerMsg, "generating settings") {
		return "generating settings"
	}
	if strings.Contains(lowerMsg, "waiting for update") {
		return "waiting for VCS"
	}

	return ""
}

type projectSettingsValidateOptions struct {
	verbose bool
	path    string
}

func newProjectSettingsValidateCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &projectSettingsValidateOptions{}

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate Kotlin DSL configuration locally",
		Long: `Validate Kotlin DSL configuration by running mvn teamcity-configs:generate.

Auto-detects .teamcity directory in the current directory or parents.
Requires Maven (mvn) or uses mvnw wrapper if present in the DSL directory.`,
		Example: `  teamcity project settings validate
  teamcity project settings validate ./path/to/.teamcity
  teamcity project settings validate --verbose`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.path = args[0]
			}
			return runProjectSettingsValidate(f, opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Show full Maven output")

	return cmd
}

func runProjectSettingsValidate(f *cmdutil.Factory, opts *projectSettingsValidateOptions) error {
	var dslDir string
	if opts.path != "" {
		abs, err := filepath.Abs(opts.path)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
		dslDir = abs
	} else {
		dslDir = config.DetectTeamCityDir()
	}

	if dslDir == "" {
		return fmt.Errorf("no TeamCity DSL directory found\n\nLooking for .teamcity in current directory and parents.\nSpecify path explicitly: teamcity project settings validate ./path/to/settings")
	}

	pomPath := filepath.Join(dslDir, "pom.xml")
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		return fmt.Errorf("pom.xml not found in %s", dslDir)
	}

	mvnCmd, err := findMaven()
	if err != nil {
		return err
	}

	p := f.Printer
	if !p.Quiet {
		_, _ = fmt.Fprintf(p.Out, "Validating %s\n", output.Faint(dslDir))
	}

	cmd := exec.Command(mvnCmd, "teamcity-configs:generate", "-f", pomPath)
	cmd.Dir = dslDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	combinedOutput := stdout.String() + stderr.String()

	if opts.verbose {
		_, _ = fmt.Fprintln(p.Out, combinedOutput)
	}

	if err != nil {
		_, _ = fmt.Fprintf(p.Out, "%s Configuration invalid\n", output.Red("✗"))

		errs := parseKotlinErrors(combinedOutput)
		if len(errs) > 0 {
			_, _ = fmt.Fprintln(p.Out)
			for _, e := range errs {
				_, _ = fmt.Fprintf(p.Out, "%s\n", e)
			}
		}

		if !opts.verbose {
			_, _ = fmt.Fprintf(p.Out, "\n%s\n", output.Faint("Hint: Run with --verbose for full compiler output"))
		}
		return fmt.Errorf("validation failed")
	}

	_, _ = fmt.Fprintf(p.Out, "%s Configuration valid\n", output.Green("✓"))

	if serverURL := config.DetectServerFromDSL(); serverURL != "" {
		_, _ = fmt.Fprintf(p.Out, "  %s %s\n", output.Faint("Server:"), serverURL)
	}
	if stats := parseValidationStats(dslDir); stats != "" {
		_, _ = fmt.Fprintf(p.Out, "  %s\n", output.Faint(stats))
	}

	return nil
}

func findMaven() (string, error) {
	mvn, err := exec.LookPath("mvn")
	if err != nil {
		return "", fmt.Errorf("maven not found\n\nInstall Maven to validate DSL locally.\nSee: https://maven.apache.org/install.html")
	}
	return mvn, nil
}

var kotlinErrorRegex = regexp.MustCompile(`(?m)^e:\s*(.+?):(\d+):(\d+):\s*(.+)$`)

func parseKotlinErrors(mavenOutput string) []string {
	var errs []string

	for _, m := range kotlinErrorRegex.FindAllStringSubmatch(mavenOutput, -1) {
		if len(m) >= 5 {
			errs = append(errs, fmt.Sprintf("%s %s\n  at %s:%s",
				output.Red("Error:"), m[4], filepath.Base(m[1]), m[2]))
		}
	}

	if len(errs) == 0 {
		scanner := bufio.NewScanner(strings.NewReader(mavenOutput))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "[ERROR]") && !strings.Contains(line, "BUILD FAILURE") {
				if msg, ok := strings.CutPrefix(line, "[ERROR] "); ok {
					errs = append(errs, output.Red("Error: ")+msg)
				}
			}
		}
		_ = scanner.Err()
	}

	return errs
}

func parseValidationStats(dslDir string) string {
	configsDir := filepath.Join(dslDir, "target", "generated-configs")
	entries, err := os.ReadDir(configsDir)
	if err != nil {
		return ""
	}

	var projects, builds, vcsRoots int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		projects++

		buildTypesDir := filepath.Join(configsDir, e.Name(), "buildTypes")
		if files, err := os.ReadDir(buildTypesDir); err == nil {
			builds += len(files)
		}

		vcsDir := filepath.Join(configsDir, e.Name(), "vcsRoots")
		if files, err := os.ReadDir(vcsDir); err == nil {
			vcsRoots += len(files)
		}
	}

	if projects == 0 {
		return ""
	}

	stats := fmt.Sprintf("Projects: %d, Build configurations: %d", projects, builds)
	if vcsRoots > 0 {
		stats += fmt.Sprintf(", VCS roots: %d", vcsRoots)
	}
	return stats
}
