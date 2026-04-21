package run

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

type runDiffOptions struct {
	json    bool
	log     bool
	web     bool
	context int
}

func newRunDiffCmd(f *cmdutil.Factory) *cobra.Command {
	opts := &runDiffOptions{}

	cmd := &cobra.Command{
		Use:   "diff <id-1> [id-2]",
		Short: "Compare two runs and show differences",
		Long: `Compare two runs (builds) and highlight what changed between them.

Shows differences in status, duration, agent, parameters, test results,
and VCS changes. Useful for understanding why a build broke or what
changed between a passing and failing run.

Use --log to compare build logs with a colored unified diff, piped
through a pager. Timestamps, temp paths, and git progress lines are
normalized so the diff shows real content changes.

Pipe to external diff tools for advanced views:
  teamcity run diff 123 124 --log --no-color | delta
  teamcity run diff 123 124 --log --no-color | diff-so-fancy

If only one run ID is given, it is compared against the previous
finished run of the same job.`,
		Args: cobra.RangeArgs(1, 2),
		Example: `  teamcity run diff 123 124
  teamcity run diff 456                # compare with previous run
  teamcity run diff 123 124 --log      # compare build logs
  teamcity run diff 123 124 --log -U5  # 5 lines context
  teamcity run diff 123 124 --json
  teamcity run diff 123 124 --web      # open both in browser`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunDiff(f, args, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&opts.log, "log", false, "Compare logs with colored unified diff")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open in browser")
	cmd.Flags().IntVarP(&opts.context, "unified", "U", 3, "Number of context lines in log diff")

	cmd.MarkFlagsMutuallyExclusive("json", "log")
	cmd.MarkFlagsMutuallyExclusive("json", "web")
	cmd.MarkFlagsMutuallyExclusive("log", "web")

	cmdutil.MarkExperimental(f, cmd)
	return cmd
}

type buildData struct {
	build       *api.Build
	tests       *api.TestOccurrences
	testSummary *api.TestOccurrences
	changes     *api.ChangeList
	problems    *api.ProblemOccurrences
	params      *api.ParameterList
}

func runRunDiff(f *cmdutil.Factory, args []string, opts *runDiffOptions) error {
	p := f.Printer
	client, err := f.Client()
	if err != nil {
		return err
	}

	id1, id2, err := resolveDiffBuildIDs(client, args)
	if err != nil {
		return err
	}

	if opts.web {
		b1, err1 := client.GetBuild(context.Background(), id1)
		b2, err2 := client.GetBuild(context.Background(), id2)
		if err1 != nil {
			return fmt.Errorf("resolving #%s: %w", id1, err1)
		}
		if err2 != nil {
			return fmt.Errorf("resolving #%s: %w", id2, err2)
		}
		cmdutil.OpenURLOrWarn(p, b1.WebURL)
		cmdutil.OpenURLOrWarn(p, b2.WebURL)
		return nil
	}

	if opts.log {
		return runLogDiff(f, client, id1, id2, opts.context)
	}

	d1, d2, err := fetchBothBuilds(client, id1, id2, p)
	if err != nil {
		return err
	}

	if opts.json {
		return p.PrintJSON(buildDiffJSON(d1, d2))
	}

	renderDiff(p, d1, d2)
	return nil
}

func resolveDiffBuildIDs(client api.ClientInterface, args []string) (string, string, error) {
	if len(args) == 2 {
		return args[0], args[1], nil
	}

	build, err := client.GetBuild(context.Background(), args[0])
	if err != nil {
		return "", "", fmt.Errorf("could not resolve: %w", err)
	}

	builds, err := client.GetBuilds(context.Background(), api.BuildsOptions{
		BuildTypeID: build.BuildTypeID,
		Limit:       1,
		State:       "finished",
		UntilDate:   build.StartDate,
	})
	if err != nil {
		return "", "", fmt.Errorf("listing builds for %s: %w", build.BuildTypeID, err)
	}

	for _, b := range builds.Builds {
		if b.ID != build.ID {
			return fmt.Sprintf("%d", b.ID), args[0], nil
		}
	}

	return "", "", api.Validation(
		"no previous finished build found for "+build.BuildTypeID,
		"provide two run IDs explicitly: teamcity run diff <id1> <id2>",
	)
}

func fetchBuildData(client api.ClientInterface, id string, p *output.Printer) (buildData, error) {
	b, err := client.GetBuild(context.Background(), id)
	if err != nil {
		return buildData{}, fmt.Errorf("#%s: %w", id, err)
	}
	d := buildData{build: b}
	if d.tests, err = client.GetBuildTests(context.Background(), id, true, 0); err != nil {
		p.Warn("Could not fetch tests for #%s: %v", id, err)
	}
	if d.testSummary, err = client.GetBuildTestSummary(id); err != nil {
		p.Warn("Could not fetch test summary for #%s: %v", id, err)
	}
	if d.changes, err = client.GetBuildChanges(context.Background(), id); err != nil {
		p.Warn("Could not fetch changes for #%s: %v", id, err)
	}
	if d.problems, err = client.GetBuildProblems(id); err != nil {
		p.Warn("Could not fetch problems for #%s: %v", id, err)
	}
	if d.params, err = client.GetBuildResultingProperties(id); err != nil {
		p.Warn("Could not fetch parameters for #%s: %v", id, err)
	}
	return d, nil
}

func fetchBothBuilds(client api.ClientInterface, id1, id2 string, p *output.Printer) (buildData, buildData, error) {
	var d1, d2 buildData
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); d1, err1 = fetchBuildData(client, id1, p) }()
	go func() { defer wg.Done(); d2, err2 = fetchBuildData(client, id2, p) }()
	wg.Wait()
	if err1 != nil {
		return d1, d2, err1
	}
	return d1, d2, err2
}

func runLogDiff(f *cmdutil.Factory, client api.ClientInterface, id1, id2 string, contextLines int) error {
	p := f.Printer

	b1, err := client.GetBuild(context.Background(), id1)
	if err != nil {
		return err
	}
	b2, err := client.GetBuild(context.Background(), id2)
	if err != nil {
		return err
	}

	log1, err := client.GetBuildLog(context.Background(), id1)
	if err != nil {
		return fmt.Errorf("log for #%s: %w", id1, err)
	}
	log2, err := client.GetBuildLog(context.Background(), id2)
	if err != nil {
		return fmt.Errorf("log for #%s: %w", id2, err)
	}

	lines1 := output.NormalizeBuildLog(output.SplitLogLines(log1))
	lines2 := output.NormalizeBuildLog(output.SplitLogLines(log2))

	output.WithPager(p.Out, func(w io.Writer) {
		hasDiff, err := output.UnifiedDiff(w, lines1, lines2,
			fmt.Sprintf("Run #%s", b1.Number), fmt.Sprintf("Run #%s", b2.Number), contextLines)
		if err != nil {
			_, _ = fmt.Fprintf(w, "Error: %v\n", err)
			return
		}
		if !hasDiff {
			_, _ = fmt.Fprintln(w, "Build logs are identical")
		}
	})

	return nil
}

type paramDiff struct {
	name     string
	old, new string
	kind     string // "changed", "added", "removed"
}

func computeParamDiffs(p1, p2 *api.ParameterList) []paramDiff {
	if p1 == nil || p2 == nil {
		return nil
	}

	m1 := paramMap(p1)
	m2 := paramMap(p2)

	var diffs []paramDiff
	for name, v1 := range m1 {
		if isAutoParam(name) {
			continue
		}
		v2, ok := m2[name]
		if !ok {
			diffs = append(diffs, paramDiff{name, v1, "", "removed"})
		} else if v1 != v2 {
			diffs = append(diffs, paramDiff{name, v1, v2, "changed"})
		}
	}
	for name, v2 := range m2 {
		if isAutoParam(name) {
			continue
		}
		if _, ok := m1[name]; !ok {
			diffs = append(diffs, paramDiff{name, "", v2, "added"})
		}
	}

	slices.SortFunc(diffs, func(a, b paramDiff) int { return cmp.Compare(a.name, b.name) })
	return diffs
}

func computeTestDiffs(t1, t2, s1, s2 *api.TestOccurrences) (summaryChanged bool, newFailures []api.TestOccurrence, fixed []string) {
	if t1 == nil || t2 == nil {
		return false, nil, nil
	}

	if s1 != nil && s2 != nil {
		summaryChanged = s1.Passed != s2.Passed || s1.Failed != s2.Failed || s1.Ignored != s2.Ignored
	}

	failed1 := testNamesByStatus(t1, "FAILURE")

	for _, t := range t2.TestOccurrence {
		if t.Status == "FAILURE" && !failed1[t.Name] {
			newFailures = append(newFailures, t)
		}
	}
	for name := range failed1 {
		found := false
		for _, t := range t2.TestOccurrence {
			if t.Name == name && t.Status == "FAILURE" {
				found = true
				break
			}
		}
		if !found {
			fixed = append(fixed, name)
		}
	}
	slices.SortFunc(newFailures, func(a, b api.TestOccurrence) int { return cmp.Compare(a.Name, b.Name) })
	slices.Sort(fixed)
	return summaryChanged, newFailures, fixed
}

func computeChangeDiffs(c1, c2 *api.ChangeList) (onlyIn1, onlyIn2 []api.Change) {
	set1 := changeVersionSet(c1)
	set2 := changeVersionSet(c2)

	if c1 != nil {
		for _, c := range c1.Change {
			if _, ok := set2[c.Version]; !ok {
				onlyIn1 = append(onlyIn1, c)
			}
		}
	}
	if c2 != nil {
		for _, c := range c2.Change {
			if _, ok := set1[c.Version]; !ok {
				onlyIn2 = append(onlyIn2, c)
			}
		}
	}
	return onlyIn1, onlyIn2
}

func computeProblemDiffs(pr1, pr2 *api.ProblemOccurrences) (newProblems, resolved []api.ProblemOccurrence) {
	if pr1 == nil || pr2 == nil {
		return nil, nil
	}

	set1 := problemIdentitySet(pr1)
	set2 := problemIdentitySet(pr2)

	for _, prob := range pr1.ProblemOccurrence {
		if !set2[prob.Identity] {
			resolved = append(resolved, prob)
		}
	}
	for _, prob := range pr2.ProblemOccurrence {
		if !set1[prob.Identity] {
			newProblems = append(newProblems, prob)
		}
	}
	return newProblems, resolved
}

func renderDiff(p *output.Printer, d1, d2 buildData) {
	b1, b2 := d1.build, d2.build

	renderDiffHeader(p, b1, b2)

	var sections []string
	if renderStatusDiff(p, b1, b2) {
		sections = append(sections, "status")
	}
	if renderProblemsDiff(p, d1.problems, d2.problems) {
		sections = append(sections, "problems")
	}
	if renderTestsDiff(p, d1.tests, d2.tests, d1.testSummary, d2.testSummary) {
		sections = append(sections, "tests")
	}
	if renderChangesDiff(p, d1.changes, d2.changes) {
		sections = append(sections, "changes")
	}
	if renderParamsDiff(p, d1.params, d2.params) {
		sections = append(sections, "parameters")
	}
	if renderAgentDiff(p, b1, b2) {
		sections = append(sections, "agent")
	}
	if renderDurationDiff(p, b1, b2) {
		sections = append(sections, "duration")
	}

	if len(sections) == 0 {
		_, _ = fmt.Fprintf(p.Out, "\n%s\n", output.Faint("No differences found."))
	}

	renderDiffFooter(p, b1, b2, sections)
}

func renderDiffHeader(p *output.Printer, b1, b2 *api.Build) {
	icon1 := output.StatusIcon(b1.Status, b1.State, b1.StatusText)
	icon2 := output.StatusIcon(b2.Status, b2.State, b2.StatusText)

	jobName := b1.BuildTypeID
	if b1.BuildType != nil {
		jobName = b1.BuildType.Name
	}

	_, _ = fmt.Fprintf(p.Out, "COMPARING  %s %d  #%s  →  %s %d  #%s\n",
		icon1, b1.ID, b1.Number, icon2, b2.ID, b2.Number)

	meta := output.Faint("Job: ") + output.Cyan(jobName)
	if b1.BranchName != "" || b2.BranchName != "" {
		branch := b1.BranchName
		if b2.BranchName != "" && b2.BranchName != b1.BranchName {
			branch = b1.BranchName + " → " + b2.BranchName
		}
		if branch != "" {
			meta += output.Faint("  ·  Branch: ") + branch
		}
	}
	_, _ = fmt.Fprintln(p.Out, meta)
}

func renderDiffFooter(p *output.Printer, b1, b2 *api.Build, sections []string) {
	_, _ = fmt.Fprintln(p.Out)
	if len(sections) > 0 {
		_, _ = fmt.Fprintf(p.Out, "%s %s\n",
			output.Faint("Changed:"), strings.Join(sections, ", "))
	}
	if b1.WebURL != "" {
		_, _ = fmt.Fprintf(p.Out, "%s %s\n", output.Faint("View -"), b1.WebURL)
	}
	if b2.WebURL != "" {
		_, _ = fmt.Fprintf(p.Out, "%s %s\n", output.Faint("View +"), b2.WebURL)
	}
}

func sectionHeader(p *output.Printer, name string) {
	_, _ = fmt.Fprintf(p.Out, "\n%s\n", output.Bold(name))
}

func diffLine(p *output.Printer, prefix, color string, format string, args ...any) {
	output.DiffLine(p.Out, prefix, color, format, args...)
}

func renderStatusDiff(p *output.Printer, b1, b2 *api.Build) bool {
	if b1.Status == b2.Status && b1.State == b2.State {
		return false
	}

	sectionHeader(p, "STATUS")

	statusLine1 := output.StatusText(b1.Status, b1.State, b1.StatusText)
	if detail := statusTextDetail(b1); detail != "" {
		statusLine1 += output.Faint("  " + truncate(detail, 80))
	}
	statusLine2 := output.StatusText(b2.Status, b2.State, b2.StatusText)
	if detail := statusTextDetail(b2); detail != "" {
		statusLine2 += output.Faint("  " + truncate(detail, 80))
	}

	diffLine(p, "-", "red", "%s", statusLine1)
	diffLine(p, "+", "green", "%s", statusLine2)
	return true
}

func renderDurationDiff(p *output.Printer, b1, b2 *api.Build) bool {
	if b1.FinishDate == "" || b2.FinishDate == "" {
		return false
	}
	dur1 := buildDuration(b1)
	dur2 := buildDuration(b2)

	if dur1 == dur2 {
		return false
	}

	sectionHeader(p, "DURATION")

	diffLine(p, "-", "red", "%s", output.FormatDuration(dur1))

	delta := dur2 - dur1
	sign := "+"
	if delta < 0 {
		sign = "-"
		delta = -delta
	}
	diffLine(p, "+", "green", "%s  %s", output.FormatDuration(dur2),
		output.Faint(fmt.Sprintf("(%s%s)", sign, output.FormatDuration(delta))))
	return true
}

func renderAgentDiff(p *output.Printer, b1, b2 *api.Build) bool {
	a1, a2 := agentName(b1), agentName(b2)
	if a1 == a2 {
		return false
	}

	sectionHeader(p, "AGENT")
	diffLine(p, "-", "red", "%s", a1)
	diffLine(p, "+", "green", "%s", a2)
	return true
}

func renderParamsDiff(p *output.Printer, params1, params2 *api.ParameterList) bool {
	diffs := computeParamDiffs(params1, params2)
	if len(diffs) == 0 {
		return false
	}

	sectionHeader(p, "PARAMETERS")

	for _, c := range diffs {
		switch c.kind {
		case "changed":
			_, _ = fmt.Fprintf(p.Out, "  %s %s: %s → %s\n",
				output.Yellow("~"), c.name, output.Red(c.old), output.Green(c.new))
		case "added":
			_, _ = fmt.Fprintf(p.Out, "  %s %s: %s\n",
				output.Green("+"), c.name, output.Green(c.new))
		case "removed":
			_, _ = fmt.Fprintf(p.Out, "  %s %s: %s\n",
				output.Red("-"), c.name, output.Red(c.old))
		}
	}

	return true
}

func renderChangesDiff(p *output.Printer, c1, c2 *api.ChangeList) bool {
	onlyIn1, onlyIn2 := computeChangeDiffs(c1, c2)
	if len(onlyIn1) == 0 && len(onlyIn2) == 0 {
		return false
	}

	sectionHeader(p, "CHANGES")

	for _, c := range onlyIn1 {
		_, _ = fmt.Fprintf(p.Out, "  %s %s  %s  %s\n",
			output.Red("-"), output.Yellow(shortSHA(c.Version)), output.Faint(shortUsername(c.Username)), firstLine(c.Comment))
	}
	for _, c := range onlyIn2 {
		_, _ = fmt.Fprintf(p.Out, "  %s %s  %s  %s\n",
			output.Green("+"), output.Yellow(shortSHA(c.Version)), output.Faint(shortUsername(c.Username)), firstLine(c.Comment))
	}

	return true
}

func renderTestsDiff(p *output.Printer, t1, t2, s1, s2 *api.TestOccurrences) bool {
	summaryChanged, newFailures, fixed := computeTestDiffs(t1, t2, s1, s2)
	if !summaryChanged && len(newFailures) == 0 && len(fixed) == 0 {
		return false
	}

	sectionHeader(p, "TESTS")

	if summaryChanged {
		diffLine(p, "-", "red", "%s", testSummaryStr(s1))
		diffLine(p, "+", "green", "%s", testSummaryStr(s2))
	}

	if len(newFailures) > 0 {
		_, _ = fmt.Fprintf(p.Out, "  %s\n", output.Red("New failures:"))
		for _, t := range newFailures {
			_, _ = fmt.Fprintf(p.Out, "    %s %s\n", output.Red("✗"), t.Name)
			if t.Details != "" {
				_, _ = fmt.Fprintf(p.Out, "      %s\n", output.Faint(truncate(t.Details, 120)))
			}
		}
	}

	if len(fixed) > 0 {
		_, _ = fmt.Fprintf(p.Out, "  %s\n", output.Green("Fixed:"))
		for _, name := range fixed {
			_, _ = fmt.Fprintf(p.Out, "    %s %s\n", output.Green("✓"), name)
		}
	}

	return true
}

func renderProblemsDiff(p *output.Printer, pr1, pr2 *api.ProblemOccurrences) bool {
	newProblems, resolved := computeProblemDiffs(pr1, pr2)
	if len(newProblems) == 0 && len(resolved) == 0 {
		return false
	}

	sectionHeader(p, "PROBLEMS")

	for _, prob := range resolved {
		_, _ = fmt.Fprintf(p.Out, "  %s %s\n", output.Red("-"), firstLine(prob.Details))
	}
	for _, prob := range newProblems {
		_, _ = fmt.Fprintf(p.Out, "  %s %s\n", output.Green("+"), firstLine(prob.Details))
	}

	return true
}

func buildDiffJSON(d1, d2 buildData) map[string]any {
	b1, b2 := d1.build, d2.build

	jsonBuild := func(b *api.Build) map[string]any {
		m := map[string]any{"id": b.ID, "number": b.Number, "status": b.Status, "state": b.State}
		if a := agentName(b); a != "" {
			m["agent"] = a
		}
		return m
	}

	diff := map[string]any{}

	if b1.Status != b2.Status || b1.State != b2.State {
		diff["status"] = map[string]any{"from": statusString(b1), "to": statusString(b2)}
	}
	if b1.FinishDate != "" && b2.FinishDate != "" {
		if dur1, dur2 := buildDuration(b1), buildDuration(b2); dur1 != dur2 {
			diff["duration"] = map[string]any{"from": dur1.String(), "to": dur2.String()}
		}
	}
	if a1, a2 := agentName(b1), agentName(b2); a1 != a2 {
		diff["agent"] = map[string]any{"from": a1, "to": a2}
	}

	if diffs := computeParamDiffs(d1.params, d2.params); len(diffs) > 0 {
		params := make([]map[string]any, len(diffs))
		for i, d := range diffs {
			params[i] = map[string]any{"name": d.name, "from": d.old, "to": d.new, "type": d.kind}
		}
		diff["parameters"] = params
	}

	if summaryChanged, newFail, fixed := computeTestDiffs(d1.tests, d2.tests, d1.testSummary, d2.testSummary); summaryChanged || len(newFail) > 0 || len(fixed) > 0 {
		tests := map[string]any{
			"from": testSummaryJSON(d1.testSummary),
			"to":   testSummaryJSON(d2.testSummary),
		}
		if len(newFail) > 0 {
			failures := make([]map[string]string, len(newFail))
			for i, t := range newFail {
				failures[i] = map[string]string{"name": t.Name, "details": firstLine(t.Details)}
			}
			tests["newFailures"] = failures
		}
		if len(fixed) > 0 {
			tests["fixed"] = fixed
		}
		diff["tests"] = tests
	}

	if only1, only2 := computeChangeDiffs(d1.changes, d2.changes); len(only1) > 0 || len(only2) > 0 {
		jsonChange := func(c api.Change) map[string]string {
			return map[string]string{"sha": c.Version, "author": c.Username, "message": firstLine(c.Comment)}
		}
		changes := map[string]any{}
		if len(only1) > 0 {
			c := make([]map[string]string, len(only1))
			for i, v := range only1 {
				c[i] = jsonChange(v)
			}
			changes["onlyInRun1"] = c
		}
		if len(only2) > 0 {
			c := make([]map[string]string, len(only2))
			for i, v := range only2 {
				c[i] = jsonChange(v)
			}
			changes["onlyInRun2"] = c
		}
		diff["changes"] = changes
	}

	if newProbs, resolved := computeProblemDiffs(d1.problems, d2.problems); len(newProbs) > 0 || len(resolved) > 0 {
		problems := map[string]any{}
		if len(newProbs) > 0 {
			p := make([]string, len(newProbs))
			for i, v := range newProbs {
				p[i] = firstLine(v.Details)
			}
			problems["new"] = p
		}
		if len(resolved) > 0 {
			p := make([]string, len(resolved))
			for i, v := range resolved {
				p[i] = firstLine(v.Details)
			}
			problems["resolved"] = p
		}
		diff["problems"] = problems
	}

	return map[string]any{"run1": jsonBuild(b1), "run2": jsonBuild(b2), "diff": diff}
}

func buildDuration(b *api.Build) time.Duration {
	if b.StartDate == "" || b.FinishDate == "" {
		return 0
	}
	start, err1 := api.ParseTeamCityTime(b.StartDate)
	finish, err2 := api.ParseTeamCityTime(b.FinishDate)
	if err1 != nil || err2 != nil {
		return 0
	}
	return finish.Sub(start)
}

func agentName(b *api.Build) string {
	if b.Agent != nil {
		return b.Agent.Name
	}
	return ""
}

func statusTextDetail(b *api.Build) string {
	if b.StatusText == "" || b.StatusText == b.Status {
		return ""
	}
	if strings.EqualFold(b.StatusText, "Canceled") {
		return ""
	}
	return b.StatusText
}

func statusString(b *api.Build) string {
	if b.State == "running" {
		return "running"
	}
	if b.State == "queued" {
		return "queued"
	}
	return strings.ToLower(b.Status)
}

func paramMap(pl *api.ParameterList) map[string]string {
	m := make(map[string]string, len(pl.Property))
	for _, p := range pl.Property {
		if p.Type != nil && p.Type.RawValue == "password" {
			continue
		}
		m[p.Name] = p.Value
	}
	return m
}

var autoParamPrefixes = []string{
	"teamcity.",
	"system.teamcity.",
	"system.build.",
	"system.agent.",
	"system.ec2.",
	"system.cloud.",
	"build.number",
	"build.vcs.number",
	"build.counter",
	"env.BUILD_NUMBER",
	"env.BUILD_URL",
	"env.BUILD_VCS_NUMBER",
	"env.SSH_AUTH_SOCK",
	"env.INVOCATION_ID",
	"env.JOURNAL_STREAM",
	"env.SYSTEMD_EXEC_PID",
}

func isAutoParam(name string) bool {
	for _, prefix := range autoParamPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func changeVersionSet(cl *api.ChangeList) map[string]struct{} {
	m := make(map[string]struct{})
	if cl != nil {
		for _, c := range cl.Change {
			m[c.Version] = struct{}{}
		}
	}
	return m
}

func testNamesByStatus(tests *api.TestOccurrences, status string) map[string]bool {
	m := make(map[string]bool)
	for _, t := range tests.TestOccurrence {
		if t.Status == status {
			m[t.Name] = true
		}
	}
	return m
}

func testSummaryJSON(t *api.TestOccurrences) map[string]int {
	if t == nil {
		return map[string]int{"passed": 0, "failed": 0, "ignored": 0}
	}
	return map[string]int{"passed": t.Passed, "failed": t.Failed, "ignored": t.Ignored}
}

func testSummaryStr(t *api.TestOccurrences) string {
	if t == nil {
		return "no tests"
	}
	var parts []string
	if t.Passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", t.Passed))
	}
	if t.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", t.Failed))
	}
	if t.Ignored > 0 {
		parts = append(parts, fmt.Sprintf("%d ignored", t.Ignored))
	}
	if len(parts) == 0 {
		return "no tests"
	}
	return strings.Join(parts, ", ")
}

func problemIdentitySet(pr *api.ProblemOccurrences) map[string]bool {
	m := make(map[string]bool)
	if pr != nil {
		for _, p := range pr.ProblemOccurrence {
			m[p.Identity] = true
		}
	}
	return m
}

func truncate(s string, maxLen int) string {
	s = firstLine(s)
	return output.Truncate(s, maxLen)
}

func shortUsername(username string) string {
	if idx := strings.IndexByte(username, '@'); idx > 0 {
		return username[:idx]
	}
	return username
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx > 0 {
		return s[:idx]
	}
	return s
}
