package migrate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/migrate"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
	"github.com/spf13/cobra"
)

type migrateOptions struct {
	dryRun     bool
	outputDir  string
	from       string
	file       string
	noValidate bool
	jsonOutput bool
}

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &migrateOptions{}

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convert CI configurations to TeamCity pipeline YAML",
		Long: `Detect CI/CD configurations in the current repository and convert them
to TeamCity pipeline YAML.

Supported sources: GitHub Actions, GitLab CI, Jenkins, CircleCI,
Azure DevOps, Travis CI, Bitbucket Pipelines.

The generated YAML files can be deployed with:
  teamcity pipeline create <name> --project <id> --file <generated>.tc.yml`,
		Example: `  teamcity migrate
  teamcity migrate --dry-run
  teamcity migrate --file .github/workflows/ci.yml
  teamcity migrate --from github-actions --output-dir teamcity/
  JENKINS_URL=https://jenkins.example.com JENKINS_USER=admin JENKINS_TOKEN=xxx teamcity migrate
  teamcity migrate --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate(cmd.Context(), f, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Preview without writing files")
	cmd.Flags().StringVarP(&opts.outputDir, "output-dir", "o", ".", "Output directory for generated files")
	cmd.Flags().StringVar(&opts.from, "from", "", "Source CI system (auto-detected if omitted)")
	cmd.Flags().StringVar(&opts.file, "file", "", "Convert a specific file only")
	cmd.Flags().BoolVar(&opts.noValidate, "no-validate", false, "Skip schema validation")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func runMigrate(ctx context.Context, f *cmdutil.Factory, opts *migrateOptions) error {
	sourceDir := "."

	var filterSource migrate.SourceCI
	if opts.from != "" {
		filterSource = migrate.SourceCI(opts.from)
		if !migrate.ValidSource(filterSource) {
			return fmt.Errorf("unknown CI source %q; supported: github-actions, gitlab, jenkins, circleci, azure-devops, travis, bitbucket", opts.from)
		}
	}

	// Detect CI configs
	configs, err := migrate.Detect(sourceDir, filterSource)
	if err != nil {
		return fmt.Errorf("scanning for CI configurations: %w", err)
	}

	// If --file specified, filter to that file
	if opts.file != "" {
		want := filepath.ToSlash(filepath.Clean(opts.file))
		filtered := []migrate.CIConfig{}
		for _, c := range configs {
			if filepath.ToSlash(filepath.Clean(c.File)) == want {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("file %q not found in detected CI configurations", opts.file)
		}
		configs = filtered
	}

	if len(configs) == 0 {
		if opts.jsonOutput {
			enc := json.NewEncoder(f.Printer.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(migrate.MigrateOutput{Sources: []migrate.CIConfig{}, Results: []*migrate.ConversionResult{}})
		}
		f.Printer.Info("No CI configurations detected")
		return nil
	}

	convertOpts := migrate.Options{
		Ctx:          ctx,
		RunnerMap:    resolveCloudRunners(f),
		WorkDir:      sourceDir,
		JenkinsURL:   os.Getenv("JENKINS_URL"),
		JenkinsUser:  os.Getenv("JENKINS_USER"),
		JenkinsToken: os.Getenv("JENKINS_TOKEN"),
	}

	var schemaData []byte
	if !opts.noValidate {
		schemaData = resolveSchema(f)
	}

	// Convert each config
	results := []*migrate.ConversionResult{}
	var conversionErrors int
	for _, cfg := range configs {
		data, err := os.ReadFile(filepath.Join(sourceDir, cfg.File))
		if err != nil {
			return fmt.Errorf("reading %s: %w", cfg.File, err)
		}

		result, err := migrate.Convert(cfg, data, convertOpts)
		if err != nil {
			f.Printer.Warn("Failed to convert %s: %v", cfg.File, err)
			conversionErrors++
			continue
		}

		if !opts.noValidate {
			if valErr := pipelineschema.ValidateWithSchema(result.YAML, schemaData); valErr != "" {
				result.ValidationError = valErr
			}
		}

		results = append(results, result)
	}

	if len(results) == 0 {
		return fmt.Errorf("all %d detected CI configuration(s) failed to convert", len(configs))
	}

	migrate.DeduplicateOutputNames(results)

	// JSON output
	if opts.jsonOutput {
		out := migrate.MigrateOutput{
			Sources: configs,
			Results: results,
		}
		enc := json.NewEncoder(f.Printer.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return err
		}
		if conversionErrors > 0 {
			return &cmdutil.ExitError{Code: 1}
		}
		return hasValidationErrors(results, opts.noValidate)
	}

	// Human-readable output
	if !opts.dryRun {
		_, _ = fmt.Fprintf(f.Printer.Out, "Scanning for CI configurations...\n\n")
	}

	writtenFiles := []string{}
	for _, result := range results {
		printConversionResult(f, result, opts.dryRun)

		if !opts.dryRun {
			outPath := filepath.Join(opts.outputDir, result.OutputFile)
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
			if err := os.WriteFile(outPath, []byte(result.YAML), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}
			writtenFiles = append(writtenFiles, outPath)
		}
	}

	if opts.dryRun {
		if conversionErrors > 0 {
			return &cmdutil.ExitError{Code: 1}
		}
		return hasValidationErrors(results, opts.noValidate)
	}

	// Summary
	if len(writtenFiles) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "Written:\n")
		for _, path := range writtenFiles {
			_, _ = fmt.Fprintf(f.Printer.Out, "  %s\n", output.Green(path))
		}
	}

	// Aggregate manual setup
	manualItems := collectManualSetup(results)
	if len(manualItems) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "\nManual setup needed:\n")
		for _, item := range manualItems {
			_, _ = fmt.Fprintf(f.Printer.Out, "  %s %s\n", output.Yellow("•"), item)
		}
	}

	// Next steps
	if len(writtenFiles) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "\nNext:\n")
		_, _ = fmt.Fprintf(f.Printer.Out, "  teamcity pipeline validate %s\n", writtenFiles[0])
		_, _ = fmt.Fprintf(f.Printer.Out, "  teamcity pipeline create <name> -p <project-id> -f %s\n", writtenFiles[0])
	}

	if conversionErrors > 0 {
		return &cmdutil.ExitError{Code: 1}
	}
	return hasValidationErrors(results, opts.noValidate)
}

func hasValidationErrors(results []*migrate.ConversionResult, noValidate bool) error {
	if noValidate {
		return nil
	}
	for _, r := range results {
		if r.ValidationError != "" {
			return &cmdutil.ExitError{Code: 1}
		}
	}
	return nil
}

func printConversionResult(f *cmdutil.Factory, result *migrate.ConversionResult, dryRun bool) {
	_, _ = fmt.Fprintf(f.Printer.Out, "  %s (%s)\n", result.SourceFile, result.Source)

	stepsIn := result.StepsConverted + len(result.Simplified)
	_, _ = fmt.Fprintf(f.Printer.Out, "    %d jobs, %d steps → %d jobs, %d steps\n",
		result.JobsConverted, stepsIn, result.JobsConverted, result.StepsConverted)

	if len(result.Simplified) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "    Simplified: %s\n",
			output.Faint(summarizeSimplifications(result.Simplified)))
	}

	if result.ValidationError != "" {
		_, _ = fmt.Fprintf(f.Printer.Out, "    %s Schema validation failed (use --no-validate to skip)\n",
			output.Red("✗"))
	} else if !dryRun {
		_, _ = fmt.Fprintf(f.Printer.Out, "    %s Valid TeamCity pipeline YAML\n",
			output.Green("✓"))
	}

	if len(result.NeedsReview) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "    Needs review: %s\n",
			output.Yellow(fmt.Sprintf("%d action(s)", len(result.NeedsReview))))
	}

	if dryRun {
		_, _ = fmt.Fprintf(f.Printer.Out, "\n--- %s ---\n%s--- end ---\n\n", result.OutputFile, result.YAML)
	} else {
		_, _ = fmt.Fprintln(f.Printer.Out)
	}
}

func summarizeSimplifications(items []string) string {
	if len(items) <= 3 {
		return joinItems(items)
	}
	return fmt.Sprintf("%s, +%d more", joinItems(items[:3]), len(items)-3)
}

func joinItems(items []string) string {
	return strings.Join(items, ", ")
}

// resolveSchema fetches the pipeline JSON schema from the server (cached 24h),
// falling back to the embedded schema if not connected.
func resolveSchema(f *cmdutil.Factory) []byte {
	client, err := f.Client()
	if err != nil {
		return pipelineschema.Bytes
	}
	c, ok := client.(*api.Client)
	if !ok {
		return pipelineschema.Bytes
	}
	schema, err := cmdutil.FetchOrCachePipelineSchema(c, false)
	if err != nil {
		return pipelineschema.Bytes
	}
	return schema
}

// resolveCloudRunners queries the TC server for cloud image names and returns
// a runner map that maps generic OS labels to the first matching image per platform.
// Returns nil if not connected or no images found, in which case the built-in defaults apply.
func resolveCloudRunners(f *cmdutil.Factory) map[string]string {
	client, err := f.Client()
	if err != nil {
		return nil
	}
	list, err := client.GetCloudImages(api.CloudImagesOptions{})
	if err != nil || len(list.Images) == 0 {
		return nil
	}

	// Pick the first image whose name matches each platform keyword.
	// Keys are the canonical source-CI runner labels that callers emit.
	byOS := map[string]string{}
	for _, img := range list.Images {
		n := strings.ToLower(img.Name)
		switch {
		case strings.Contains(n, "ubuntu") || strings.Contains(n, "linux"):
			if _, ok := byOS["linux"]; !ok {
				byOS["linux"] = img.Name
			}
		case strings.Contains(n, "macos") || strings.Contains(n, "mac"):
			if _, ok := byOS["mac"]; !ok {
				byOS["mac"] = img.Name
			}
		case strings.Contains(n, "windows"):
			if _, ok := byOS["windows"]; !ok {
				byOS["windows"] = img.Name
			}
		}
	}
	if len(byOS) == 0 {
		return nil
	}

	m := map[string]string{}
	linuxSrc := []string{"ubuntu-latest", "ubuntu-24.04", "ubuntu-22.04", "ubuntu-20.04"}
	macSrc := []string{"macos-latest", "macos-15", "macos-14", "macos-13"}
	winSrc := []string{"windows-latest", "windows-2022", "windows-2019"}
	if img, ok := byOS["linux"]; ok {
		for _, k := range linuxSrc {
			m[k] = img
		}
	}
	if img, ok := byOS["mac"]; ok {
		for _, k := range macSrc {
			m[k] = img
		}
	}
	if img, ok := byOS["windows"]; ok {
		for _, k := range winSrc {
			m[k] = img
		}
	}
	return m
}

func collectManualSetup(results []*migrate.ConversionResult) []string {
	seen := map[string]bool{}
	items := []string{}
	for _, r := range results {
		for _, item := range r.ManualSetup {
			if !seen[item] {
				seen[item] = true
				items = append(items, item)
			}
		}
	}
	return items
}
