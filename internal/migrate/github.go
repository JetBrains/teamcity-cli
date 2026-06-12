package migrate

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strconv"
	"strings"

	"github.com/rhysd/actionlint"
)

type ResultStatus int

const (
	StatusConverted ResultStatus = iota
	StatusSimplified
	StatusUnsupported
	StatusUnknown
)

type StepResult struct {
	Status                ResultStatus
	Steps                 []Step
	Artifacts             []FilePublication
	EnableDependencyCache bool
	Note                  string
	ManualTasks           []string
}

func Converted(steps []Step) StepResult {
	return StepResult{Status: StatusConverted, Steps: steps}
}

func Unknown(identifier string, inputs map[string]string) StepResult {
	r := unknownStub(identifier, shortActionID(identifier), "Action inputs", inputs)
	r.Note = identifier
	return r
}

// unknownStub renders a TODO script step that preserves the original fields as comments.
func unknownStub(identifier, name, fieldsLabel string, fields map[string]string) StepResult {
	var stub strings.Builder
	fmt.Fprintf(&stub, "# TODO: Replace %s with equivalent commands", identifier)
	if len(fields) > 0 {
		stub.WriteString("\n# " + fieldsLabel + ":")
		for _, k := range SortedKeys(fields) {
			v := fields[k]
			if bambooLooksSecret(k) {
				v = "REDACTED"
			}
			fmt.Fprintf(&stub, "\n%s", commentBlock(fmt.Sprintf("  %s: %s", k, v)))
		}
	}
	stub.WriteString("\necho 'TODO: implement equivalent of " + name + "'")
	return StepResult{
		Status: StatusUnknown,
		Steps:  []Step{{Name: name, ScriptContent: stub.String()}},
	}
}

// commentBlock prefixes every line of s with "# " so multiline values can't escape into runnable shell.
func commentBlock(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "# " + l
	}
	return strings.Join(lines, "\n")
}

func shortActionID(id string) string {
	if name, _, ok := strings.Cut(id, "@"); ok {
		id = name
	}
	if idx := strings.LastIndex(id, "/"); idx >= 0 {
		return id[idx+1:]
	}
	return id
}

func applyResults(results []StepResult, cr *ConversionResult) (steps []Step, artifacts []FilePublication, cache bool) {
	for _, r := range results {
		switch r.Status {
		case StatusConverted:
			steps = append(steps, r.Steps...)
		case StatusSimplified:
			cr.Simplified = append(cr.Simplified, r.Note)
		case StatusUnsupported:
			cr.NeedsReview = append(cr.NeedsReview, r.Note)
		case StatusUnknown:
			cr.NeedsReview = append(cr.NeedsReview, r.Note)
			steps = append(steps, r.Steps...)
		}
		cache = cache || r.EnableDependencyCache
		artifacts = append(artifacts, r.Artifacts...)
		cr.ManualSetup = append(cr.ManualSetup, r.ManualTasks...)
	}
	return
}

// noOpFallback replaces a job whose steps were all simplified or unsupported, so it still emits schema-valid YAML.
func noOpFallback(jobName string, result *ConversionResult) []Step {
	result.ManualSetup = append(result.ManualSetup,
		fmt.Sprintf("Job %q has no convertible steps (all simplified or unsupported) → delete the job or replace with manual TC configuration", jobName))
	return []Step{{
		Name:          "No-op",
		ScriptContent: fmt.Sprintf("# TODO: All steps in job %q were simplified or unsupported (see manual-setup notes)\necho 'Job %s has no executable steps; configure manually or delete'", jobName, jobName),
	}}
}

// mergeStepParams folds env params into every step's parameters.
func mergeStepParams(steps []Step, params map[string]string) {
	if len(params) == 0 {
		return
	}
	for i := range steps {
		if steps[i].Parameters == nil {
			steps[i].Parameters = map[string]string{}
		}
		maps.Copy(steps[i].Parameters, params)
	}
}

type actionTransformer func(name string, inputs map[string]string) StepResult

var actionRegistry = initActionRegistry()

func LookupActionTransformer(uses string) (actionTransformer, bool) {
	name := uses
	if before, _, ok := strings.Cut(uses, "@"); ok {
		name = before
	}
	if t, ok := actionRegistry[name]; ok {
		return t, true
	}
	// Walk shorter prefixes so an owner/repo entry matches owner/repo/subpath.
	for i := strings.LastIndex(name, "/"); i > 0; i = strings.LastIndex(name[:i], "/") {
		if t, ok := actionRegistry[name[:i]]; ok {
			return t, true
		}
	}
	return nil, false
}

var (
	secretsRe = regexp.MustCompile(`\$\{\{\s*secrets\.(\w+)\s*}}`)
	ghExprRe  = regexp.MustCompile(`\$\{\{.*?}}`)
)

func convertGitHub(cfg CIConfig, data []byte, opts Options) (*ConversionResult, error) {
	workflow, errs := actionlint.Parse(data)
	if workflow == nil {
		if len(errs) > 0 {
			return nil, errs[0]
		}
		return nil, errors.New("failed to parse workflow")
	}

	result := NewResult(cfg)
	flagGHAParseErrors(errs, result)
	p := &Pipeline{}

	var wfDefaults ghaRunDefaults
	if workflow.Defaults != nil && workflow.Defaults.Run != nil {
		if workflow.Defaults.Run.WorkingDirectory != nil {
			wfDefaults.workDir = workflow.Defaults.Run.WorkingDirectory.Value
		}
		if workflow.Defaults.Run.Shell != nil {
			wfDefaults.shell = workflow.Defaults.Run.Shell.Value
		}
	}

	for _, id := range SortedKeys(workflow.Jobs) {
		p.Jobs = append(p.Jobs, convertGHAJob(id, workflow.Jobs[id], result, opts, wfDefaults))
	}

	if workflow.Env != nil {
		if params := extractGHAEnvParams(workflow.Env, result); len(params) > 0 {
			p.Parameters = params
		}
	}

	if len(workflow.On) > 0 {
		if triggers := describeGHATriggers(workflow.On); triggers != "" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("VCS trigger (%s) → configure in TeamCity project settings", triggers))
		}
	}
	if workflow.Concurrency != nil {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Workflow sets concurrency%s → recreate with \"Limit the number of simultaneously running builds\" in TeamCity", ghaConcurrencyGroup(workflow.Concurrency)))
	}

	result.Pipeline = p
	return result, nil
}

// flagGHAParseErrors surfaces non-fatal parse errors as NeedsReview entries, capped so a pathological file doesn't flood the report.
func flagGHAParseErrors(errs []*actionlint.Error, result *ConversionResult) {
	const maxParseErrors = 3
	for i, e := range errs {
		if i == maxParseErrors {
			result.NeedsReview = append(result.NeedsReview,
				fmt.Sprintf("...and %d more parse errors → review the source workflow", len(errs)-maxParseErrors))
			break
		}
		result.NeedsReview = append(result.NeedsReview,
			fmt.Sprintf("Workflow parse error at line %d: %s → fix the source or convert that section manually", e.Line, condense(e.Message)))
	}
}

type ghaRunDefaults struct {
	workDir string
	shell   string
}

type ghaJobAccumulator struct {
	result        *ConversionResult
	defaults      ghaRunDefaults
	runsOnWindows bool
}

func convertGHAJob(id string, job *actionlint.Job, result *ConversionResult, opts Options, wfDefaults ghaRunDefaults) Job {
	j := Job{ID: id, Name: id}
	if job.Name != nil {
		j.Name = job.Name.Value
	}
	if strings.Contains(j.Name, "${{") {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q name uses GHA expression %q → set static name or use TC parameter", id, j.Name))
		j.Name = id
	}

	for _, need := range job.Needs {
		j.Dependencies = append(j.Dependencies, need.Value)
	}

	if job.WorkflowCall != nil {
		j.Steps = []Step{workflowCallStub(id, job.WorkflowCall, result)}
		return j
	}

	var runsOnWindows bool
	switch {
	case job.RunsOn == nil:
	// A bare `runs-on: ${{ matrix.os }}` parses into LabelsExpr, not Labels — without this branch it would be dropped silently.
	case job.RunsOn.LabelsExpr != nil:
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q runs-on uses expression %q → emitted the default runner; expand the matrix into separate jobs with explicit runners", id, condense(job.RunsOn.LabelsExpr.Value)))
		j.RunsOn = opts.MapRunner("ubuntu-latest")
	case len(job.RunsOn.Labels) > 0:
		raw := job.RunsOn.Labels[0].Value
		if strings.Contains(raw, "${{") {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q runs-on uses matrix expression %q → expand the matrix into separate jobs with explicit runners", id, condense(raw)))
			j.RunsOn = opts.MapRunner("ubuntu-latest")
		} else {
			mapped, known := opts.ResolveRunner(raw)
			j.RunsOn = mapped
			if !known {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Job %q runs-on %q is not a GitHub-hosted runner → emitted `self-hosted`; configure matching agent requirements in TeamCity", id, raw))
			}
			for _, l := range job.RunsOn.Labels {
				runsOnWindows = runsOnWindows || strings.Contains(strings.ToLower(l.Value), "windows")
			}
		}
		if len(job.RunsOn.Labels) > 1 {
			all := make([]string, len(job.RunsOn.Labels))
			for i, l := range job.RunsOn.Labels {
				all[i] = l.Value
			}
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q uses multi-label runner %v → configure agent requirements in TC", id, all))
		}
	}

	if job.Container != nil {
		img := ""
		if job.Container.Image != nil {
			img = job.Container.Image.Value
		}
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q uses container %q → add docker-image to steps or use Docker wrapper build feature", id, img))
	}
	if job.Services != nil && len(job.Services.Value) > 0 {
		var svcNames []string
		for svcID := range job.Services.Value {
			svcNames = append(svcNames, svcID)
		}
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q uses service containers %v → configure as Docker Compose or agent-level services", id, svcNames))
	}

	if job.If != nil && job.If.Value != "" {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q condition: %s → configure as branch filter or execution policy", id, condense(job.If.Value)))
	}
	if job.TimeoutMinutes != nil {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q sets timeout-minutes: %s → configure an execution timeout in TeamCity failure conditions", id, ghaFloatString(job.TimeoutMinutes)))
	}
	if job.Concurrency != nil {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q sets concurrency%s → recreate with \"Limit the number of simultaneously running builds\" in TeamCity", id, ghaConcurrencyGroup(job.Concurrency)))
	}
	if ghaBoolSet(job.ContinueOnError) {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q has continue-on-error: %s → its failure must not fail the pipeline; relax dependency failure conditions in TeamCity", id, ghaBoolString(job.ContinueOnError)))
	}
	if len(job.Outputs) > 0 {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q defines outputs (%s) → expose them as TeamCity output parameters and rewire consumers of needs.%s.outputs.<name> to %%dep.%s.<param>%%", id, strings.Join(SortedKeys(job.Outputs), ", "), id, id))
	}
	if job.Strategy != nil && job.Strategy.Matrix != nil {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q uses strategy.matrix → expand to separate jobs or use parallelism in TeamCity", id))
	}

	jobDefaults := wfDefaults
	if job.Defaults != nil && job.Defaults.Run != nil {
		if job.Defaults.Run.WorkingDirectory != nil {
			jobDefaults.workDir = job.Defaults.Run.WorkingDirectory.Value
		}
		if job.Defaults.Run.Shell != nil {
			jobDefaults.shell = job.Defaults.Run.Shell.Value
		}
	}

	acc := &ghaJobAccumulator{result: result, defaults: jobDefaults, runsOnWindows: runsOnWindows}

	var stepResults []StepResult
	for _, step := range job.Steps {
		stepName := ""
		if step.Name != nil {
			stepName = step.Name.Value
		}
		if step.If != nil && step.If.Value != "" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Step %q has if: %s → add execution condition or branch filter in TeamCity", stepName, condense(step.If.Value)))
		}
		if ghaBoolSet(step.ContinueOnError) {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Step %q has continue-on-error: %s → wrap the command so its exit code is ignored (e.g. `cmd || true`) or override the failure condition; TC fails the build on nonzero exit by default", stepName, ghaBoolString(step.ContinueOnError)))
		}
		if step.TimeoutMinutes != nil {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Step %q sets timeout-minutes: %s → no per-step timeout in TeamCity; configure an execution timeout in the job's failure conditions", stepName, ghaFloatString(step.TimeoutMinutes)))
		}
		stepResults = append(stepResults, transformGHAStep(step, acc)...)
	}

	steps, artifacts, cache := applyResults(stepResults, result)
	if len(steps) == 0 && len(stepResults) > 0 {
		steps = noOpFallback(id, result)
	}
	j.Steps = steps
	j.FilesPublication = artifacts
	j.EnableDependencyCache = cache

	if job.Env != nil {
		if params := extractGHAEnvParams(job.Env, result); len(params) > 0 {
			j.Parameters = params
		}
	}

	return j
}

// workflowCallStub renders a TODO step for a reusable-workflow job, preserving with: inputs and secrets: names in comments so nothing is silently dropped.
func workflowCallStub(id string, call *actionlint.WorkflowCall, result *ConversionResult) Step {
	uses := ""
	if call.Uses != nil {
		uses = call.Uses.Value
	}
	result.NeedsReview = append(result.NeedsReview,
		fmt.Sprintf("Job %q calls reusable workflow %q → inline or convert the called workflow separately", id, uses))

	var stub strings.Builder
	fmt.Fprintf(&stub, "# TODO: Job %q calls reusable workflow: %s\n# Inline the workflow steps or convert separately", id, uses)
	if len(call.Inputs) > 0 {
		stub.WriteString("\n# Workflow inputs (with:):")
		for _, k := range SortedKeys(call.Inputs) {
			val := ""
			if in := call.Inputs[k]; in != nil && in.Value != nil {
				val = in.Value.Value
			}
			fmt.Fprintf(&stub, "\n%s", commentBlock(fmt.Sprintf("  %s: %s", k, val)))
		}
	}
	if len(call.Secrets) > 0 {
		stub.WriteString("\n# Workflow secrets:")
		for _, k := range SortedKeys(call.Secrets) {
			val := ""
			if sec := call.Secrets[k]; sec != nil && sec.Value != nil {
				val = sec.Value.Value
			}
			detectGHASecrets(val, result)
			fmt.Fprintf(&stub, "\n%s", commentBlock(fmt.Sprintf("  %s: %s", k, val)))
		}
	}
	if call.InheritSecrets {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q passes `secrets: inherit` to a reusable workflow → recreate the secrets it needs with: teamcity project token put <project-id> <value>", id))
	}
	stub.WriteString("\necho 'TODO: implement reusable workflow call'")
	return Step{Name: "Reusable workflow call", ScriptContent: stub.String()}
}

func transformGHAStep(step *actionlint.Step, acc *ghaJobAccumulator) []StepResult {
	result := acc.result
	stepName := ""
	if step.Name != nil {
		stepName = step.Name.Value
	}

	switch exec := step.Exec.(type) {
	case *actionlint.ExecRun:
		script := ""
		if exec.Run != nil {
			script = exec.Run.Value
		}
		workDir := acc.defaults.workDir
		if exec.WorkingDirectory != nil {
			workDir = exec.WorkingDirectory.Value
		}
		if workDir != "" {
			workDir = MapGHAExpressions(workDir)
			flagUnmappedGHAExpressions(workDir, fmt.Sprintf("Step %q working-directory", stepName), result)
		}
		shell := acc.defaults.shell
		if exec.Shell != nil {
			shell = exec.Shell.Value
		}
		switch {
		case (shell == "bash" || shell == "sh") && acc.runsOnWindows:
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Step %q sets shell %q on a Windows runner → TeamCity script steps run cmd.exe; provide Git Bash/WSL on the agent or rewrite for cmd", stepName, shell))
		case shell != "" && shell != "bash" && shell != "sh":
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Step %q uses shell %q → prepend shebang or configure agent accordingly", stepName, shell))
		case shell == "" && acc.runsOnWindows:
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Step %q runs on a Windows runner with no explicit shell → GitHub defaults to PowerShell but TeamCity script steps run cmd.exe; set the step shell or wrap commands in `powershell -Command`", stepName))
		}
		detectGHASecrets(script, result)
		script = MapGHAExpressions(script)
		flagUnmappedGHAExpressions(script, fmt.Sprintf("Step %q script", stepName), result)
		return []StepResult{Converted([]Step{{
			Name:             stepName,
			ScriptContent:    script,
			WorkingDirectory: workDir,
			Parameters:       extractGHAEnvParams(step.Env, result),
		}})}

	case *actionlint.ExecAction:
		if exec.Uses == nil {
			return []StepResult{{Status: StatusUnsupported,
				Note: fmt.Sprintf("Step %q has uses: with no action reference → dropped from output; fix the source step", stepName)}}
		}
		uses := exec.Uses.Value
		inputs := collectActionInputs(exec)
		for k, v := range inputs {
			detectGHASecrets(v, result)
			inputs[k] = MapGHAExpressions(v)
			flagUnmappedGHAExpressions(inputs[k], fmt.Sprintf("Action %q input %q", uses, k), result)
		}

		var r StepResult
		transformer, known := LookupActionTransformer(uses)
		if known {
			r = transformer(stepName, inputs)
		} else {
			r = Unknown(uses, inputs)
		}
		if known && acc.runsOnWindows {
			for _, s := range r.Steps {
				r.ManualTasks = append(r.ManualTasks,
					fmt.Sprintf("Step %q converted from %q emits a POSIX shell script on a Windows runner → provide Git Bash/WSL on the agent or rewrite for cmd/PowerShell", s.Name, uses))
			}
		}
		mergeStepParams(r.Steps, extractGHAEnvParams(step.Env, result))
		return []StepResult{r}
	}

	return []StepResult{{Status: StatusUnsupported,
		Note: fmt.Sprintf("Step %q has neither run: nor uses: → dropped from output; rewrite it as a script step manually", stepName)}}
}

func collectActionInputs(exec *actionlint.ExecAction) map[string]string {
	inputs := map[string]string{}
	if exec.Inputs != nil {
		for key, input := range exec.Inputs {
			if input.Value != nil {
				inputs[key] = input.Value.Value
			}
		}
	}
	return inputs
}

func extractGHAEnvParams(env *actionlint.Env, result *ConversionResult) map[string]string {
	params := map[string]string{}
	if env == nil || env.Vars == nil {
		return params
	}
	for _, v := range env.Vars {
		if v == nil || v.Name == nil || v.Value == nil {
			continue
		}
		val := v.Value.Value
		mapped := MapGHAExpressions(val)
		params[v.Name.Value] = mapped
		detectGHASecrets(val, result)
		flagUnmappedGHAExpressions(mapped, "Env "+v.Name.Value, result)
	}
	return params
}

// flagUnmappedGHAExpressions records a manual-setup item for each distinct ${{ ... }} left after mapping.
func flagUnmappedGHAExpressions(s, label string, result *ConversionResult) {
	matches := ghExprRe.FindAllString(s, -1)
	if len(matches) == 0 {
		return
	}
	seen := map[string]bool{}
	for _, m := range matches {
		if seen[m] {
			continue
		}
		seen[m] = true
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("%s contains unmapped GitHub expression %s → set equivalent TeamCity parameter", label, m))
	}
}

func detectGHASecrets(script string, result *ConversionResult) {
	for _, match := range secretsRe.FindAllStringSubmatch(script, -1) {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Secret %s → create with: teamcity project token put <project-id> <value>", match[1]))
	}
}

// ghaFloatString renders a numeric GHA field, falling back to its raw ${{ }} expression form.
// ghaConcurrencyGroup renders the concurrency group suffix when one is set.
func ghaConcurrencyGroup(c *actionlint.Concurrency) string {
	if c.Group != nil && c.Group.Value != "" {
		return fmt.Sprintf(" (group %s)", c.Group.Value)
	}
	return ""
}

// ghaBoolSet reports a bool field that is literally true or driven by a runtime expression.
func ghaBoolSet(b *actionlint.Bool) bool {
	return b != nil && (b.Value || b.Expression != nil)
}

// ghaBoolString renders a literal bool field or its raw expression.
func ghaBoolString(b *actionlint.Bool) string {
	if b.Expression != nil {
		return b.Expression.Value
	}
	return strconv.FormatBool(b.Value)
}

func ghaFloatString(f *actionlint.Float) string {
	if f.Expression != nil {
		return f.Expression.Value
	}
	return strconv.FormatFloat(f.Value, 'f', -1, 64)
}

func describeGHATriggers(events []actionlint.Event) string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.EventName()
	}
	return strings.Join(names, ", ")
}
