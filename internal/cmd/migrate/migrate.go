package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/analytics"
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
	force      bool
}

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &migrateOptions{}

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convert CI configurations to TeamCity pipeline YAML",
		Long: `Detect CI/CD configurations in the current repository and convert them
to TeamCity pipeline YAML.

Supported sources: GitHub Actions, Bamboo (bamboo-specs/*.yml). Other systems
(GitLab, Jenkins, CircleCI, Azure DevOps, Travis, Bitbucket) will land in
follow-up releases.

This is a heuristic converter — always review generated files before shipping.

TeamCity Pipelines YAML is a subset of full TeamCity capability. Features
like matrix strategies, conditional step execution, service containers,
templates, and native trigger definitions have no YAML equivalent and are
surfaced under "Manual setup needed" for follow-up.`,
		Example: `  teamcity migrate
  teamcity migrate --dry-run
  teamcity migrate --file .github/workflows/ci.yml
  teamcity migrate --from github-actions --output-dir teamcity/
  teamcity migrate --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrate(f, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Preview without writing files")
	cmd.Flags().StringVarP(&opts.outputDir, "output-dir", "o", ".", "Output directory for generated files")
	cmd.Flags().StringVar(&opts.from, "from", "", "Source CI system (auto-detected if omitted)")
	cmd.Flags().StringVar(&opts.file, "file", "", "Convert a specific file only")
	cmd.Flags().BoolVar(&opts.noValidate, "no-validate", false, "Skip schema validation")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Overwrite existing output files")

	cmdutil.MarkExperimental(f, cmd)

	return cmd
}

func runMigrate(f *cmdutil.Factory, opts *migrateOptions) error {
	var configs []migrate.CIConfig
	var results []*migrate.ConversionResult
	defer func() {
		trackMigrate(f, opts, configs, results)
	}()

	var filterSource migrate.SourceCI
	if opts.from != "" {
		filterSource = migrate.SourceCI(opts.from)
		if !migrate.ValidSource(filterSource) {
			return fmt.Errorf("unknown CI source %q; supported: github-actions, bamboo", opts.from)
		}
	}

	if opts.file != "" {
		cfg, err := migrate.AnalyzeFile(opts.file, filterSource)
		if err != nil {
			return err
		}
		configs = []migrate.CIConfig{*cfg}
	} else {
		detected, err := migrate.Detect(".", filterSource)
		if err != nil {
			return fmt.Errorf("scanning for CI configurations: %w", err)
		}
		configs = detected
	}

	if len(configs) == 0 {
		if opts.jsonOutput {
			return f.Printer.PrintJSON(migrate.MigrateOutput{Sources: []migrate.CIConfig{}, Results: []*migrate.ConversionResult{}})
		}
		f.Printer.Info("No CI configurations detected")
		return nil
	}

	client, err := f.Client()
	if err != nil {
		client = nil
	}

	// The schema also drives runner-name mapping, so resolve it even with --no-validate — the flag must only skip validation, not change the YAML.
	schemaData := resolveSchema(client)

	convertOpts := migrate.Options{RunnerMap: resolveRunnerMap(client, schemaData)}

	results = []*migrate.ConversionResult{}
	var conversionErrors int
	for _, cfg := range configs {
		data, err := os.ReadFile(cfg.File)
		if err != nil {
			f.Printer.Warn("Failed to read %s: %v", cfg.File, err)
			conversionErrors++
			continue
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

	// Write before branching on output format so --json changes only the report, not the behavior.
	written := []writtenFile{}
	var skippedExisting int
	if !opts.dryRun {
		for _, result := range results {
			outPath := filepath.Join(opts.outputDir, result.OutputFile)
			// Refuse to clobber user edits: identical content keeps reruns idempotent, anything else needs --force.
			if existing, err := os.ReadFile(outPath); err == nil && !opts.force && string(existing) != result.YAML {
				f.Printer.Warn("Skipping %s: file exists with different content (use --force to overwrite)", outPath)
				skippedExisting++
				continue
			}
			if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
			if err := os.WriteFile(outPath, []byte(result.YAML), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", outPath, err)
			}
			written = append(written, writtenFile{path: outPath, result: result})
		}
	}

	if opts.jsonOutput {
		if err := f.Printer.PrintJSON(migrate.MigrateOutput{Sources: configs, Results: results}); err != nil {
			return err
		}
	} else {
		printMigrateReport(f, opts, configs, results, written)
	}

	if conversionErrors > 0 || skippedExisting > 0 {
		return &cmdutil.ExitError{Code: 1}
	}
	return validationExitError(results, opts.noValidate)
}

// writtenFile pairs an output path with its conversion so the report shows per-file validation state.
type writtenFile struct {
	path   string
	result *migrate.ConversionResult
}

func printMigrateReport(f *cmdutil.Factory, opts *migrateOptions, configs []migrate.CIConfig, results []*migrate.ConversionResult, written []writtenFile) {
	if !opts.dryRun {
		_, _ = fmt.Fprintf(f.Printer.Out, "Detected %d CI configuration(s):\n\n", len(configs))
	}

	cfgByFile := make(map[string]migrate.CIConfig, len(configs))
	for _, c := range configs {
		cfgByFile[c.File] = c
	}
	for _, result := range results {
		printConversionResult(f, cfgByFile[result.SourceFile], result, opts.dryRun)
	}

	if len(written) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "Written:\n")
		for _, w := range written {
			if w.result.ValidationError != "" {
				_, _ = fmt.Fprintf(f.Printer.Out, "  %s %s\n", output.Green(w.path), output.Yellow("(schema validation failed — review before deploying)"))
				continue
			}
			_, _ = fmt.Fprintf(f.Printer.Out, "  %s\n", output.Green(w.path))
		}
	}

	printItemList(f, "Needs review:", migrate.CollectNeedsReview(results))
	printItemList(f, "Manual setup needed:", migrate.CollectManualSetup(results))

	if len(written) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "\nNext:\n")
		_, _ = fmt.Fprintf(f.Printer.Out, "  teamcity pipeline validate %s\n", written[0].path)
		_, _ = fmt.Fprintf(f.Printer.Out, "  teamcity pipeline create <name> -p <project-id> -f %s\n", written[0].path)
	}

	printMigrateTips(f)
}

func printItemList(f *cmdutil.Factory, header string, items []string) {
	if len(items) == 0 {
		return
	}
	_, _ = fmt.Fprintf(f.Printer.Out, "\n%s\n", header)
	for _, item := range items {
		_, _ = fmt.Fprintf(f.Printer.Out, "  %s %s\n", output.Yellow("•"), item)
	}
}

func trackMigrate(f *cmdutil.Factory, opts *migrateOptions, configs []migrate.CIConfig, results []*migrate.ConversionResult) {
	f.Analytics.Track(analytics.GroupMigrate, analytics.EventCompleted, map[string]any{
		"source":            migrateSourceField(configs),
		"outcome":           migrateOutcomeField(configs, results),
		"validation_status": migrateValidationField(opts, results),
		"is_dry_run":        opts.dryRun,
	})
}

// migrateSourceField collapses the detected configs to a single telemetry source value.
func migrateSourceField(configs []migrate.CIConfig) string {
	if len(configs) == 0 {
		return analytics.MigrateSourceNone
	}
	first := mapSourceCI(configs[0].Source)
	if slices.ContainsFunc(configs[1:], func(c migrate.CIConfig) bool {
		return mapSourceCI(c.Source) != first
	}) {
		return analytics.MigrateSourceMixed
	}
	return first
}

func mapSourceCI(s migrate.SourceCI) string {
	switch s {
	case migrate.GitHubActions:
		return analytics.MigrateSourceGitHubActions
	case migrate.Bamboo:
		return analytics.MigrateSourceBamboo
	}
	return analytics.MigrateSourceOther
}

func migrateOutcomeField(configs []migrate.CIConfig, results []*migrate.ConversionResult) string {
	switch {
	case len(configs) == 0:
		return analytics.MigrateOutcomeNothingFound
	case len(results) == 0:
		return analytics.MigrateOutcomeFailed
	}
	// Fewer results than configs means some files failed to read or convert.
	if len(results) < len(configs) || slices.ContainsFunc(results, func(r *migrate.ConversionResult) bool {
		return len(r.NeedsReview) > 0 || len(r.ManualSetup) > 0
	}) {
		return analytics.MigrateOutcomePartial
	}
	return analytics.MigrateOutcomeClean
}

func migrateValidationField(opts *migrateOptions, results []*migrate.ConversionResult) string {
	if opts.noValidate {
		return analytics.MigrateValidationSkipped
	}
	if migrate.HasValidationErrors(results) {
		return analytics.MigrateValidationInvalid
	}
	return analytics.MigrateValidationValid
}

func printConversionStatus(f *cmdutil.Factory, result *migrate.ConversionResult) {
	if result.ValidationError != "" {
		_, _ = fmt.Fprintf(f.Printer.Out, "    %s Schema validation failed (use --no-validate to skip)\n",
			output.Red("✗"))
		return
	}
	reviews, manuals := len(result.NeedsReview), len(result.ManualSetup)
	if reviews == 0 && manuals == 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "    %s Fully converted\n", output.Green("✓"))
		return
	}
	var parts []string
	if reviews > 0 {
		parts = append(parts, fmt.Sprintf("%d to review", reviews))
	}
	if manuals > 0 {
		parts = append(parts, fmt.Sprintf("%d manual step(s)", manuals))
	}
	_, _ = fmt.Fprintf(f.Printer.Out, "    %s Partial conversion — %s\n",
		output.Yellow("⚠"), strings.Join(parts, ", "))
}

func printMigrateTips(f *cmdutil.Factory) {
	f.Printer.Info("")
	f.Printer.Tip("run the migrate-to-teamcity skill with an AI agent for better conversions; report issues at https://jb.gg/tc/migrate/issues")
}

func validationExitError(results []*migrate.ConversionResult, noValidate bool) error {
	if noValidate {
		return nil
	}
	if migrate.HasValidationErrors(results) {
		return &cmdutil.ExitError{Code: 1}
	}
	return nil
}

func printConversionResult(f *cmdutil.Factory, cfg migrate.CIConfig, result *migrate.ConversionResult, dryRun bool) {
	_, _ = fmt.Fprintf(f.Printer.Out, "  %s (%s)\n", result.SourceFile, result.Source)

	srcJobs, srcSteps := cfg.Jobs, cfg.Steps
	if srcJobs == 0 && srcSteps == 0 {
		srcJobs = result.JobsConverted
		srcSteps = result.StepsConverted + len(result.Simplified)
	}
	_, _ = fmt.Fprintf(f.Printer.Out, "    %d jobs, %d steps → %d jobs, %d steps\n",
		srcJobs, srcSteps, result.JobsConverted, result.StepsConverted)

	if len(cfg.Features) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "    Features: %s\n",
			output.Faint(strings.Join(cfg.Features, ", ")))
	}

	if len(result.Simplified) > 0 {
		_, _ = fmt.Fprintf(f.Printer.Out, "    Simplified: %s\n",
			output.Faint(summarizeSimplifications(result.Simplified)))
	}

	printConversionStatus(f, result)

	if dryRun {
		_, _ = fmt.Fprintf(f.Printer.Out, "\n--- %s ---\n%s--- end ---\n\n", result.OutputFile, result.YAML)
	} else {
		_, _ = fmt.Fprintln(f.Printer.Out)
	}
}

func summarizeSimplifications(items []string) string {
	if len(items) <= 3 {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(items[:3], ", "), len(items)-3)
}

// resolveSchema fetches the cached server schema, falling back to the embedded one when offline.
func resolveSchema(client api.ClientInterface) []byte {
	c, ok := client.(*api.Client)
	if !ok {
		return pipelineschema.Bytes
	}
	schema, _, _, err := cmdutil.FetchOrCachePipelineSchema(c, false)
	if err != nil {
		return pipelineschema.Bytes
	}
	return schema
}

// resolveRunnerMap uses server-reported hosted images when connected, falling back to the schema when offline.
func resolveRunnerMap(client api.ClientInterface, schemaData []byte) map[string]string {
	if m, known := resolveCloudRunners(client); known {
		return m
	}
	if names := pipelineschema.HostedAgentNames(schemaData); len(names) > 0 {
		if m := migrate.BuildRunnerMap(names); m != nil {
			return m
		}
	}
	return nil
}

func resolveCloudRunners(client api.ClientInterface) (map[string]string, bool) {
	if client == nil {
		return nil, false
	}
	list, _, err := client.GetCloudImages(api.CloudImagesOptions{})
	if err != nil {
		return nil, false
	}
	if len(list.Images) == 0 {
		return map[string]string{}, true
	}
	names := make([]string, 0, len(list.Images))
	for _, img := range list.Images {
		names = append(names, img.Name)
	}
	return migrate.BuildRunnerMap(names), true
}
