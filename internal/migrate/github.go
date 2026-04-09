package migrate

import (
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/rhysd/actionlint"
)

// Step result types — used only by GitHub Actions converter.

type ResultStatus int

const (
	StatusConverted ResultStatus = iota
	StatusSimplified
	StatusUnsupported
	StatusUnknown
)

type StepResult struct {
	Status      ResultStatus
	Steps       []Step
	Artifacts   []FilePublication
	Features    []string
	Note        string
	ManualTasks []string
	Identifier  string
}

func Converted(steps []Step) StepResult {
	return StepResult{Status: StatusConverted, Steps: steps}
}

func Unknown(identifier string, inputs map[string]string) StepResult {
	stub := fmt.Sprintf("# TODO: Replace %s with equivalent commands", identifier)
	if len(inputs) > 0 {
		stub += "\n# Action inputs:"
		for _, k := range SortedKeys(inputs) {
			stub += fmt.Sprintf("\n#   %s: %s", k, inputs[k])
		}
	}
	stub += "\necho 'TODO: implement equivalent of " + shortActionID(identifier) + "'"
	return StepResult{
		Status:     StatusUnknown,
		Identifier: identifier,
		Note:       identifier,
		Steps:      []Step{{Name: shortActionID(identifier), ScriptContent: stub}},
	}
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
			for _, f := range r.Features {
				if f == "enable-dependency-cache" {
					cache = true
				}
			}
		case StatusUnsupported:
			cr.NeedsReview = append(cr.NeedsReview, r.Note)
		case StatusUnknown:
			cr.NeedsReview = append(cr.NeedsReview, r.Note)
			steps = append(steps, r.Steps...)
		}
		artifacts = append(artifacts, r.Artifacts...)
		cr.ManualSetup = append(cr.ManualSetup, r.ManualTasks...)
	}
	return
}

// Action transformer registry.

type actionTransformer func(name, uses string, inputs map[string]string) StepResult

var actionRegistry = initActionRegistry()

func LookupActionTransformer(uses string) (actionTransformer, bool) {
	name := uses
	if idx := strings.Index(uses, "@"); idx >= 0 {
		name = uses[:idx]
	}
	if t, ok := actionRegistry[name]; ok {
		return t, true
	}
	// Fall back to progressively shorter path prefixes so that a registered
	// owner/repo entry matches owner/repo/subpath usages (e.g. snyk/actions/node
	// → snyk/actions), and owner/* wildcards match any subpath under owner.
	for i := strings.LastIndex(name, "/"); i > 0; i = strings.LastIndex(name[:i], "/") {
		prefix := name[:i]
		if t, ok := actionRegistry[prefix]; ok {
			return t, true
		}
		if t, ok := actionRegistry[prefix+"/*"]; ok {
			return t, true
		}
	}
	return nil, false
}

// GitHub Actions converter.

var (
	secretsRe = regexp.MustCompile(`\$\{\{\s*secrets\.(\w+)\s*}}`)
	ghExprRe  = regexp.MustCompile(`\$\{\{.*?}}`)
)

func convertGitHub(cfg CIConfig, data []byte, opts Options) (*ConversionResult, error) {
	workflow, errs := actionlint.Parse(data)
	if workflow == nil {
		msg := "failed to parse workflow"
		if len(errs) > 0 {
			msg = errs[0].Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	result := NewResult(cfg)
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

	result.Pipeline = p
	return result, nil
}

type ghaRunDefaults struct {
	workDir string
	shell   string
}

type ghaJobAccumulator struct {
	result   *ConversionResult
	defaults ghaRunDefaults
	opts     Options
}

func convertGHAJob(id string, job *actionlint.Job, result *ConversionResult, opts Options, wfDefaults ghaRunDefaults) Job {
	j := Job{ID: id, Name: id}
	if job.Name != nil {
		j.Name = job.Name.Value
	}

	for _, need := range job.Needs {
		j.Dependencies = append(j.Dependencies, need.Value)
	}

	if job.WorkflowCall != nil {
		uses := ""
		if job.WorkflowCall.Uses != nil {
			uses = job.WorkflowCall.Uses.Value
		}
		result.NeedsReview = append(result.NeedsReview,
			fmt.Sprintf("Job %q calls reusable workflow %q → inline or convert the called workflow separately", id, uses))
		j.Steps = []Step{{
			Name:          "Reusable workflow call",
			ScriptContent: fmt.Sprintf("# TODO: Job %q calls reusable workflow: %s\n# Inline the workflow steps or convert separately\necho 'TODO: implement reusable workflow call'", id, uses),
		}}
		return j
	}

	if job.RunsOn != nil && len(job.RunsOn.Labels) > 0 {
		j.RunsOn = opts.MapRunner(job.RunsOn.Labels[0].Value)
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
			fmt.Sprintf("Job %q condition: %s → configure as branch filter or execution policy", id, job.If.Value))
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

	acc := &ghaJobAccumulator{result: result, defaults: jobDefaults, opts: opts}

	var stepResults []StepResult
	for _, step := range job.Steps {
		if step.If != nil && step.If.Value != "" {
			stepName := ""
			if step.Name != nil {
				stepName = step.Name.Value
			}
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Step %q has if: %s → add execution condition or branch filter in TeamCity", stepName, step.If.Value))
		}
		stepResults = append(stepResults, transformGHAStep(step, acc)...)
	}

	steps, artifacts, cache := applyResults(stepResults, result)
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
		shell := acc.defaults.shell
		if exec.Shell != nil {
			shell = exec.Shell.Value
		}
		if shell != "" && shell != "bash" && shell != "sh" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Step %q uses shell %q → prepend shebang or configure agent accordingly", stepName, shell))
		}
		detectGHASecrets(script, result)
		script = MapGHAExpressions(script)
		return []StepResult{Converted([]Step{{
			Name:             stepName,
			ScriptContent:    script,
			WorkingDirectory: workDir,
			Parameters:       extractGHAEnvParams(step.Env, result),
		}})}

	case *actionlint.ExecAction:
		if exec.Uses == nil {
			return nil
		}
		uses := exec.Uses.Value
		inputs := collectActionInputs(exec)

		var r StepResult
		if transformer, ok := LookupActionTransformer(uses); ok {
			r = transformer(stepName, uses, inputs)
		} else {
			r = Unknown(uses, inputs)
		}
		r.Identifier = uses
		// Propagate step-level env to emitted steps so action-scoped
		// variables (e.g. credentials, region) reach the runtime.
		if env := extractGHAEnvParams(step.Env, result); len(env) > 0 {
			for i := range r.Steps {
				if r.Steps[i].Parameters == nil {
					r.Steps[i].Parameters = map[string]string{}
				}
				maps.Copy(r.Steps[i].Parameters, env)
			}
		}
		return []StepResult{r}
	}

	return nil
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
		// Always emit the env mapping so secret-backed and expression-backed
		// vars still reach the runtime as TeamCity parameter references.
		params[v.Name.Value] = mapped
		// Surface secret-creation hints regardless — users still need to
		// provision the corresponding TC secure parameter.
		for _, m := range secretsRe.FindAllStringSubmatch(val, -1) {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Secret %s → create with: teamcity project token put <project-id> <value>", m[1]))
		}
		// Flag expressions we could not translate so users can finish them by hand.
		if mapped == val && ghExprRe.MatchString(val) {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Env %s=%s → GitHub expression; set equivalent value in TC parameters", v.Name.Value, val))
		}
	}
	return params
}

func detectGHASecrets(script string, result *ConversionResult) {
	for _, match := range secretsRe.FindAllStringSubmatch(script, -1) {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Secret %s → create with: teamcity project token put <project-id> <value>", match[1]))
	}
}

func describeGHATriggers(events []actionlint.Event) string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.EventName()
	}
	return strings.Join(names, ", ")
}
