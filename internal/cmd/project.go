package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	tcerrors "github.com/JetBrains/teamcity-cli/internal/errors"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
		Long:  `List and view TeamCity projects.`,
		Args:  cobra.NoArgs,
		RunE:  subcommandRequired,
	}

	cmd.AddCommand(newProjectListCmd())
	cmd.AddCommand(newProjectViewCmd())
	cmd.AddCommand(newProjectTokenCmd())
	cmd.AddCommand(newProjectSettingsCmd())
	cmd.AddCommand(newParamCmd("project", projectParamAPI))

	return cmd
}

type projectListOptions struct {
	parent     string
	limit      int
	jsonFields string
}

func newProjectListCmd() *cobra.Command {
	opts := &projectListOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Long:  `List all TeamCity projects.`,
		Example: `  tc project list
  tc project list --parent Falcon
  tc project list --json
  tc project list --json=id,name,webUrl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectList(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.parent, "parent", "p", "", "Filter by parent project ID")
	cmd.Flags().IntVarP(&opts.limit, "limit", "n", 100, "Maximum number of projects")
	AddJSONFieldsFlag(cmd, &opts.jsonFields)

	return cmd
}

func runProjectList(cmd *cobra.Command, opts *projectListOptions) error {
	if err := validateLimit(opts.limit); err != nil {
		return err
	}
	jsonResult, showHelp, err := ParseJSONFields(cmd, opts.jsonFields, &api.ProjectFields)
	if err != nil {
		return err
	}
	if showHelp {
		return nil
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	projects, err := client.GetProjects(api.ProjectsOptions{
		Parent: opts.parent,
		Limit:  opts.limit,
		Fields: jsonResult.Fields,
	})
	if err != nil {
		return err
	}

	if jsonResult.Enabled {
		return output.PrintJSON(projects)
	}

	if projects.Count == 0 {
		fmt.Println("No projects found")
		return nil
	}

	headers := []string{"ID", "NAME", "PARENT"}
	var rows [][]string

	for _, p := range projects.Projects {
		parent := "-"
		if p.ParentProjectID != "" {
			parent = p.ParentProjectID
		}

		rows = append(rows, []string{
			p.ID,
			p.Name,
			parent,
		})
	}

	output.AutoSizeColumns(headers, rows, 2, 0, 1, 2)
	output.PrintTable(headers, rows)
	return nil
}

func newProjectViewCmd() *cobra.Command {
	opts := &viewOptions{}
	cmd := &cobra.Command{
		Use:   "view <project-id>",
		Short: "View project details",
		Long:  `View details of a TeamCity project.`,
		Args:  cobra.ExactArgs(1),
		Example: `  tc project view Falcon
  tc project view Falcon --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectView(args[0], opts)
		},
	}
	addViewFlags(cmd, opts)
	return cmd
}

func runProjectView(projectID string, opts *viewOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	project, err := client.GetProject(projectID)
	if err != nil {
		return err
	}

	if opts.web {
		return browser.OpenURL(project.WebURL)
	}

	if opts.json {
		return output.PrintJSON(project)
	}

	fmt.Printf("%s\n", output.Cyan(project.Name))
	fmt.Printf("ID: %s\n", project.ID)

	if project.ParentProjectID != "" {
		fmt.Printf("Parent: %s\n", project.ParentProjectID)
	}

	if project.Description != "" {
		fmt.Printf("Description: %s\n", project.Description)
	}

	fmt.Printf("\n%s %s\n", output.Faint("View in browser:"), output.Green(project.WebURL))

	return nil
}

// Token command - manage secure tokens for versioned settings
func newProjectTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage secure tokens",
		Long: `Manage secure tokens for versioned settings.

Secure tokens allow you to store sensitive values (passwords, API keys, etc.)
in TeamCity's credentials storage. The scrambled token can be safely committed
to version control and used in configuration files as credentialsJSON:<token>.

See: https://www.jetbrains.com/help/teamcity/storing-project-settings-in-version-control.html#Managing+Tokens`,
		Args: cobra.NoArgs,
		RunE: subcommandRequired,
	}

	cmd.AddCommand(newProjectTokenPutCmd())
	cmd.AddCommand(newProjectTokenGetCmd())

	return cmd
}

type projectTokenPutOptions struct {
	stdin bool
}

func newProjectTokenPutCmd() *cobra.Command {
	opts := &projectTokenPutOptions{}

	cmd := &cobra.Command{
		Use:   "put <project-id> [value]",
		Short: "Store a secret and get a secure token",
		Long: `Store a sensitive value and get a secure token reference.

The returned token can be used in versioned settings configuration files
as credentialsJSON:<token>. The actual value is stored securely in TeamCity
and is not committed to version control.

Requires EDIT_PROJECT permission (Project Administrator role).`,
		Example: `  # Store a secret interactively (prompts for value)
  tc project token put Falcon

  # Store a secret from a value
  tc project token put Falcon "my-secret-password"

  # Store a secret from stdin (useful for piping)
  echo -n "my-secret" | tc project token put Falcon --stdin

  # Use the token in versioned settings
  # password: credentialsJSON:<returned-token>`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var value string
			if len(args) > 1 {
				value = args[1]
			}
			return runProjectTokenPut(args[0], value, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.stdin, "stdin", false, "Read value from stdin")

	return cmd
}

func runProjectTokenPut(projectID, value string, opts *projectTokenPutOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if opts.stdin {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		value = strings.TrimSuffix(string(data), "\n")
	}

	if value == "" {
		prompt := &survey.Password{
			Message: "Enter secure value to scramble:",
		}
		if err := survey.AskOne(prompt, &value); err != nil {
			return fmt.Errorf("failed to read value: %w", err)
		}
	}

	if value == "" {
		return fmt.Errorf("value cannot be empty")
	}

	token, err := client.CreateSecureToken(projectID, value)
	if err != nil {
		return fmt.Errorf("failed to create secure token: %w", err)
	}

	fmt.Println(token)

	if strings.HasPrefix(token, "credentialsJSON:") {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, output.Faint("Use in versioned settings as: "+token))
	}

	return nil
}

func newProjectTokenGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <project-id> <token>",
		Short: "Get the value of a secure token",
		Long: `Retrieve the original value for a secure token.

This operation requires CHANGE_SERVER_SETTINGS permission,
which is only available to System Administrators.`,
		Example: `  tc project token get Falcon "credentialsJSON:abc123..."
  tc project token get Falcon "abc123..."`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectTokenGet(args[0], args[1])
		},
	}

	return cmd
}

func runProjectTokenGet(projectID, token string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Strip credentialsJSON: prefix if present
	token = strings.TrimPrefix(token, "credentialsJSON:")

	value, err := client.GetSecureValue(projectID, token)
	if err != nil {
		return fmt.Errorf("failed to get secure value: %w", err)
	}

	fmt.Println(value)
	return nil
}

func newProjectSettingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Manage versioned settings",
		Long: `View and manage versioned settings (Kotlin DSL) for a project.

Versioned settings allow you to store project configuration as code in a VCS repository.
This enables version control, code review, and automated deployment of CI/CD configuration.

See: https://www.jetbrains.com/help/teamcity/storing-project-settings-in-version-control.html`,
		Args: cobra.NoArgs,
		RunE: subcommandRequired,
	}

	cmd.AddCommand(newProjectSettingsStatusCmd())
	cmd.AddCommand(newProjectSettingsExportCmd())
	cmd.AddCommand(newProjectSettingsValidateCmd())

	return cmd
}

type projectSettingsStatusOptions struct {
	json bool
}

func newProjectSettingsStatusCmd() *cobra.Command {
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
		Example: `  tc project settings status MyProject
  tc project settings status MyProject --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectSettingsStatus(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")

	return cmd
}

func runProjectSettingsStatus(projectID string, opts *projectSettingsStatusOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	project, err := client.GetProject(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	config, configErr := client.GetVersionedSettingsConfig(projectID)
	status, statusErr := client.GetVersionedSettingsStatus(projectID)

	if opts.json {
		result := map[string]interface{}{
			"project": project,
		}
		if configErr == nil {
			result["config"] = config
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
		return output.PrintJSON(result)
	}

	if configErr != nil {
		fmt.Printf("%s %s %s %s\n", output.Yellow("!"), output.Cyan(project.Name), output.Faint("·"), "not configured")
		fmt.Printf("\n%s\n", output.Faint(configErr.Error()))
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
	fmt.Printf("%s %s %s %s\n", statusIcon, header, output.Faint("·"), statusLabel)

	fmt.Println()
	fmt.Printf("%-12s %s\n", output.Faint("Format"), formatSettingsFormat(config.Format))
	fmt.Printf("%-12s %s\n", output.Faint("Sync"), config.SynchronizationMode)
	fmt.Printf("%-12s %s\n", output.Faint("Build"), formatBuildMode(config.BuildSettingsMode))
	if config.VcsRootID != "" {
		vcsRoot := config.VcsRootID
		if config.SettingsPath != "" {
			vcsRoot += " @ " + config.SettingsPath
		}
		fmt.Printf("%-12s %s\n", output.Faint("VCS Root"), vcsRoot)
	}

	if statusErr != nil {
		fmt.Printf("\n%s\n", output.Faint(statusErr.Error()))
		return nil
	}

	if status.DslOutdated {
		fmt.Printf("\n%s DSL scripts need to be regenerated\n", output.Yellow("!"))
	}

	if status.Timestamp != "" {
		fmt.Printf("\n%-12s %s\n", output.Faint("Last sync"), formatRelativeTime(status.Timestamp))
	}

	if status.Message != "" && status.Type != "info" {
		fmt.Printf("%-12s %s\n", output.Faint("Message"), output.Faint(status.Message))
	}

	fmt.Printf("\n%-12s %s\n", output.Faint("View"), output.Faint(project.WebURL+"&tab=versionedSettings"))

	return nil
}

type projectSettingsExportOptions struct {
	kotlin         bool
	xml            bool
	output         string
	useRelativeIds bool
}

func newProjectSettingsExportCmd() *cobra.Command {
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
  tc project settings export MyProject

  # Export as Kotlin DSL explicitly
  tc project settings export MyProject --kotlin

  # Export as XML
  tc project settings export MyProject --xml

  # Save to specific file
  tc project settings export MyProject -o settings.zip`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectSettingsExport(args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.kotlin, "kotlin", false, "Export as Kotlin DSL (default)")
	cmd.Flags().BoolVar(&opts.xml, "xml", false, "Export as XML")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output file path (default: projectSettings.zip)")
	cmd.Flags().BoolVar(&opts.useRelativeIds, "relative-ids", true, "Use relative IDs in exported settings")
	cmd.MarkFlagsMutuallyExclusive("kotlin", "xml")

	return cmd
}

func runProjectSettingsExport(projectID string, opts *projectSettingsExportOptions) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	// Determine format (default to kotlin)
	format := "kotlin"
	if opts.xml {
		format = "xml"
	}

	// Determine output filename
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

	fmt.Printf("Exported %s settings to %s (%d bytes)\n", format, outputFile, len(data))
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

// getSyncingStatus returns a status if the message indicates DSL is currently running, or empty string if not
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

func newProjectSettingsValidateCmd() *cobra.Command {
	opts := &projectSettingsValidateOptions{}

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate Kotlin DSL configuration locally",
		Long: `Validate Kotlin DSL configuration by running mvn teamcity-configs:generate.

Auto-detects .teamcity directory in the current directory or parents.
Requires Maven (mvn) or uses mvnw wrapper if present in the DSL directory.`,
		Example: `  tc project settings validate
  tc project settings validate ./path/to/.teamcity
  tc project settings validate --verbose`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.path = args[0]
			}
			return runProjectSettingsValidate(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Show full Maven output")

	return cmd
}

func runProjectSettingsValidate(opts *projectSettingsValidateOptions) error {
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
		return fmt.Errorf("no TeamCity DSL directory found\n\nLooking for .teamcity in current directory and parents.\nSpecify path explicitly: tc project settings validate ./path/to/settings")
	}

	pomPath := filepath.Join(dslDir, "pom.xml")
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		return fmt.Errorf("pom.xml not found in %s", dslDir)
	}

	mvnCmd, err := findMaven()
	if err != nil {
		return err
	}

	if !Quiet {
		fmt.Printf("Validating %s\n", output.Faint(dslDir))
	}

	cmd := exec.Command(mvnCmd, "teamcity-configs:generate", "-f", pomPath)
	cmd.Dir = dslDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	combinedOutput := stdout.String() + stderr.String()

	if opts.verbose {
		fmt.Println(combinedOutput)
	}

	if err != nil {
		fmt.Printf("%s Configuration invalid\n", output.Red("✗"))

		errs := parseKotlinErrors(combinedOutput)
		if len(errs) > 0 {
			fmt.Println()
			for _, e := range errs {
				fmt.Printf("%s\n", e)
			}
		}

		if !opts.verbose {
			fmt.Printf("\n%s\n", output.Faint("Hint: Run with --verbose for full compiler output"))
		}
		return fmt.Errorf("validation failed")
	}

	fmt.Printf("%s Configuration valid\n", output.Green("✓"))

	if serverURL := config.DetectServerFromDSL(); serverURL != "" {
		fmt.Printf("  %s %s\n", output.Faint("Server:"), serverURL)
	}
	if stats := parseValidationStats(dslDir); stats != "" {
		fmt.Printf("  %s\n", output.Faint(stats))
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
				if msg := strings.TrimPrefix(line, "[ERROR] "); msg != line {
					errs = append(errs, output.Red("Error: ")+msg)
				}
			}
		}
		_ = scanner.Err() // string reader won't error, but be explicit
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

// GetClientFunc is the function used to create API clients.
// It can be overridden in tests to inject mock clients.
var GetClientFunc = defaultGetClient

// getClient returns an API client using the current GetClientFunc.
func getClient() (api.ClientInterface, error) {
	return GetClientFunc()
}

func defaultGetClient() (api.ClientInterface, error) {
	serverURL := config.GetServerURL()
	token := config.GetToken()

	if serverURL != "" && token != "" {
		return api.NewClient(serverURL, token), nil
	}

	if buildAuth, ok := config.GetBuildAuth(); ok {
		if serverURL == "" {
			serverURL = buildAuth.ServerURL
		}
		output.Debug("Using build-level authentication")
		return api.NewClientWithBasicAuth(serverURL, buildAuth.Username, buildAuth.Password), nil
	}

	return nil, tcerrors.NotAuthenticated()
}
