package migrate

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const jenkinsMaxResponseBytes = 10 << 20 // 10 MB

// Jenkins converter.

func convertJenkins(cfg CIConfig, data []byte, opts Options) (*ConversionResult, error) {
	if opts.JenkinsURL == "" {
		result := NewResult(cfg)
		result.Pipeline = &Pipeline{
			Comment: "# Jenkinsfile detected but Jenkins API connection not configured.\n" +
				"# Jenkinsfile is Groovy — it requires Jenkins' own parser for correct conversion.\n" +
				"# Set environment variables and re-run:\n" +
				"#   export JENKINS_URL=https://your-jenkins.example.com\n" +
				"#   export JENKINS_USER=your-username\n" +
				"#   export JENKINS_TOKEN=your-api-token\n" +
				"#   teamcity migrate\n\n",
			Jobs: []Job{{
				ID: "build", Name: "Build", RunsOn: "Ubuntu-24.04-Large",
				Steps: []Step{{
					Name: "Placeholder",
					ScriptContent: "echo 'Jenkinsfile conversion requires Jenkins API connection.'\n" +
						"echo 'Set JENKINS_URL, JENKINS_USER, JENKINS_TOKEN and re-run teamcity migrate'",
				}},
			}},
		}
		result.NeedsReview = append(result.NeedsReview,
			"Jenkinsfile requires JENKINS_URL env var for conversion (Groovy needs Jenkins' own parser)")
		return result, nil
	}

	ctx := cmp.Or(opts.Ctx, context.Background())
	client := newJenkinsClient(opts.JenkinsURL, opts.JenkinsUser, opts.JenkinsToken)

	ast, err := client.jsonifyJenkinsfile(ctx, string(data))
	if err != nil {
		return nil, fmt.Errorf("jenkins API: %w", err)
	}

	result := NewResult(cfg)
	result.Pipeline = convertJenkinsAST(ast, cfg, result)
	return result, nil
}

// Jenkins API client.

type jenkinsClient struct {
	baseURL     string
	user, token string
	http        *http.Client
}

func newJenkinsClient(baseURL, user, token string) *jenkinsClient {
	return &jenkinsClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		user:    user,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *jenkinsClient) jsonifyJenkinsfile(ctx context.Context, jenkinsfile string) (*pipelineAST, error) {
	endpoint := c.baseURL + "/pipeline-model-converter/toJson"

	form := url.Values{"jenkinsfile": {jenkinsfile}}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.setAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jenkins API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, jenkinsMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jenkins API returned %d: %s", resp.StatusCode, truncateAt(string(body), 200))
	}

	var response jenkinsConverterResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("parse JSON response: %w", err)
	}

	if response.Data.Result != "success" {
		var errMsgs []string
		for _, e := range response.Data.Errors {
			errMsgs = append(errMsgs, e.Error)
		}
		if len(errMsgs) > 0 {
			return nil, fmt.Errorf("jenkins pipeline parser: %s", strings.Join(errMsgs, "; "))
		}
		return nil, fmt.Errorf("jenkins pipeline parser failed (status: %s)", response.Data.Result)
	}

	return &response.Data.JSON.Pipeline, nil
}

func (c *jenkinsClient) setAuth(req *http.Request) {
	if c.user != "" && c.token != "" {
		req.SetBasicAuth(c.user, c.token)
	}
}

func truncateAt(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Jenkins AST types.

type jenkinsConverterResponse struct {
	Data struct {
		Result string            `json:"result"`
		Errors []jenkinsAPIError `json:"errors"`
		JSON   struct {
			Pipeline pipelineAST `json:"pipeline"`
		} `json:"json"`
	} `json:"data"`
}

type jenkinsAPIError struct {
	Error string `json:"error"`
}

type pipelineAST struct {
	Agent       *agentAST       `json:"agent"`
	Stages      []stageAST      `json:"stages"`
	Environment []envVarAST     `json:"environment"`
	Post        *postAST        `json:"post"`
	Parameters  *parametersAST  `json:"parameters"`
	Tools       json.RawMessage `json:"tools"`
	Triggers    *triggersAST    `json:"triggers"`
	Options     *optionsAST     `json:"options"`
}

type stageAST struct {
	Name        string      `json:"name"`
	Steps       []stepAST   `json:"steps,omitempty"`
	Branches    []branchAST `json:"branches,omitempty"`
	Parallel    []stageAST  `json:"parallel,omitempty"`
	Stages      []stageAST  `json:"stages,omitempty"`
	Agent       *agentAST   `json:"agent,omitempty"`
	Environment []envVarAST `json:"environment,omitempty"`
	When        *whenAST    `json:"when,omitempty"`
	Post        *postAST    `json:"post,omitempty"`
}

type branchAST struct {
	Name  string    `json:"name"`
	Steps []stepAST `json:"steps"`
}

type stepAST struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Children  []stepAST       `json:"children,omitempty"`
}

type agentAST struct {
	Type      string          `json:"type"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Argument  json.RawMessage `json:"argument,omitempty"`
}

type envVarAST struct {
	Key   string `json:"key"`
	Value struct {
		IsLiteral bool   `json:"isLiteral"`
		Value     string `json:"value"`
	} `json:"value"`
}

type postAST struct {
	Conditions []postConditionAST `json:"conditions"`
}

type postConditionAST struct {
	Condition string `json:"condition"`
	Branch    struct {
		Steps []stepAST `json:"steps"`
	} `json:"branch"`
}

type whenAST struct {
	Conditions []conditionAST `json:"conditions"`
}

type conditionAST struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type parametersAST struct {
	Parameters []json.RawMessage `json:"parameters"`
}

type triggersAST struct {
	Triggers []json.RawMessage `json:"triggers"`
}

type optionsAST struct {
	Options []json.RawMessage `json:"options"`
}

// Jenkins AST → Pipeline conversion.

func convertJenkinsAST(ast *pipelineAST, cfg CIConfig, result *ConversionResult) *Pipeline {
	p := &Pipeline{
		Comment: "# Converted from: " + cfg.File + " (Jenkins Declarative Pipeline via API)\n\n",
	}

	if len(ast.Environment) > 0 {
		params := make(map[string]string)
		for _, env := range ast.Environment {
			if env.Value.IsLiteral {
				params[env.Key] = env.Value.Value
			} else {
				if strings.Contains(env.Value.Value, "credentials(") {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Jenkins credential binding %q (env %s) → create as TeamCity secure parameter", env.Value.Value, env.Key))
				} else {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Jenkins env %s = %s (Groovy expression) → set as TeamCity parameter manually", env.Key, env.Value.Value))
				}
			}
		}
		if len(params) > 0 {
			p.Parameters = params
		}
	}

	runner := resolveJenkinsAgent(ast.Agent, result)

	if ast.Parameters != nil {
		for _, raw := range ast.Parameters.Parameters {
			extractJenkinsParam(raw, result)
		}
	}

	if ast.Triggers != nil {
		for _, raw := range ast.Triggers.Triggers {
			extractJenkinsTrigger(raw, result)
		}
	}

	var prevJobIDs []string
	for _, stage := range ast.Stages {
		jobs := convertJenkinsStage(stage, runner, prevJobIDs, result)
		p.Jobs = append(p.Jobs, jobs...)
		prevJobIDs = jenkinsJobIDs(jobs)
	}

	if ast.Post != nil {
		postJobs := convertJenkinsPost(ast.Post, runner, prevJobIDs, result)
		p.Jobs = append(p.Jobs, postJobs...)
	}

	return p
}

func convertJenkinsStage(stage stageAST, runner string, prevJobIDs []string, result *ConversionResult) []Job {
	stageRunner := runner
	if stage.Agent != nil {
		stageRunner = resolveJenkinsAgent(stage.Agent, result)
	}

	if len(stage.Parallel) > 0 {
		var jobs []Job
		for _, child := range stage.Parallel {
			childJobs := convertJenkinsStage(child, stageRunner, prevJobIDs, result)
			jobs = append(jobs, childJobs...)
		}
		if stage.Post != nil {
			jobs = append(jobs, convertJenkinsPost(stage.Post, stageRunner, jenkinsJobIDs(jobs), result)...)
		}
		return jobs
	}

	if len(stage.Stages) > 0 {
		var jobs []Job
		deps := prevJobIDs
		for _, child := range stage.Stages {
			childJobs := convertJenkinsStage(child, stageRunner, deps, result)
			jobs = append(jobs, childJobs...)
			deps = jenkinsJobIDs(childJobs)
		}
		if stage.Post != nil {
			jobs = append(jobs, convertJenkinsPost(stage.Post, stageRunner, deps, result)...)
		}
		return jobs
	}

	j := Job{
		ID:     SanitizeJobID(stage.Name),
		Name:   stage.Name,
		RunsOn: stageRunner,
	}

	if len(prevJobIDs) > 0 {
		j.Dependencies = prevJobIDs
	}

	if len(stage.Environment) > 0 {
		params := make(map[string]string)
		for _, env := range stage.Environment {
			if env.Value.IsLiteral {
				params[env.Key] = env.Value.Value
			} else if strings.Contains(env.Value.Value, "credentials(") {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Stage %q credential %q → create as TeamCity secure parameter", stage.Name, env.Value.Value))
			}
		}
		if len(params) > 0 {
			j.Parameters = params
		}
	}

	if stage.When != nil {
		for _, cond := range stage.When.Conditions {
			desc := describeJenkinsWhen(cond)
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Stage %q when: %s → configure as branch filter or execution policy", stage.Name, desc))
		}
	}

	var astSteps []stepAST
	if len(stage.Steps) > 0 {
		astSteps = stage.Steps
	} else if len(stage.Branches) > 0 {
		for _, branch := range stage.Branches {
			astSteps = append(astSteps, branch.Steps...)
		}
	}

	for _, step := range astSteps {
		convertJenkinsStep(step, &j, result)
	}

	if len(j.Steps) == 0 {
		j.Steps = []Step{{
			Name:          stage.Name,
			ScriptContent: fmt.Sprintf("echo 'TODO: Convert stage %q steps'", stage.Name),
		}}
	}

	if stage.Post != nil {
		postJobs := convertJenkinsPost(stage.Post, stageRunner, []string{j.ID}, result)
		return append([]Job{j}, postJobs...)
	}

	return []Job{j}
}

func convertJenkinsStep(step stepAST, j *Job, result *ConversionResult) {
	args := parseJenkinsStepArgs(step.Arguments)

	switch step.Name {
	case "sh":
		j.Steps = append(j.Steps, Step{
			Name:          "Shell",
			ScriptContent: MapJenkinsVars(jenkinsScriptArg(args)),
		})

	case "bat":
		j.Steps = append(j.Steps, Step{
			Name:          "Batch",
			ScriptContent: MapJenkinsVars(jenkinsScriptArg(args)),
		})
		result.ManualSetup = append(result.ManualSetup,
			"bat step → ensure Windows agent or wrap in cmd invocation")

	case "powershell", "pwsh":
		j.Steps = append(j.Steps, Step{
			Name:          "PowerShell",
			ScriptContent: MapJenkinsVars(jenkinsScriptArg(args)),
		})
		result.ManualSetup = append(result.ManualSetup,
			"PowerShell step → TC pipeline YAML has no shell selector; wrap with pwsh invocation manually")

	case "echo":
		j.Steps = append(j.Steps, Step{
			Name:          "Echo",
			ScriptContent: fmt.Sprintf("echo %q", jenkinsScriptArg(args)),
		})

	case "checkout":
		result.Simplified = append(result.Simplified, "checkout (TeamCity VCS checkout is automatic)")

	case "dir":
		dir := jenkinsArgValue(args)
		for _, child := range step.Children {
			convertJenkinsStep(child, j, result)
			if len(j.Steps) > 0 {
				j.Steps[len(j.Steps)-1].WorkingDirectory = dir
			}
		}

	case "withCredentials":
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("withCredentials binding → create TeamCity secure parameters for credential: %s", truncateAt(jenkinsArgValue(args), 80)))
		for _, child := range step.Children {
			convertJenkinsStep(child, j, result)
		}

	case "withEnv":
		for _, child := range step.Children {
			convertJenkinsStep(child, j, result)
		}

	case "timeout":
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("timeout(%s) → configure build timeout in TeamCity failure conditions", jenkinsArgValue(args)))
		for _, child := range step.Children {
			convertJenkinsStep(child, j, result)
		}

	case "retry":
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("retry(%s) → configure auto-retry in TeamCity failure conditions", jenkinsArgValue(args)))
		for _, child := range step.Children {
			convertJenkinsStep(child, j, result)
		}

	case "script":
		result.NeedsReview = append(result.NeedsReview,
			"Groovy script {} block → requires manual conversion to shell commands")
		for _, child := range step.Children {
			convertJenkinsStep(child, j, result)
		}

	case "junit":
		pattern := jenkinsArgValue(args)
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("junit %q → TeamCity auto-detects JUnit XML; verify report path in build features", pattern))

	case "archiveArtifacts":
		pattern := jenkinsArgString(args, "artifacts")
		if pattern == "" {
			pattern = jenkinsArgValue(args)
		}
		if pattern != "" {
			j.FilesPublication = append(j.FilesPublication, FilePublication{
				Path:            pattern,
				PublishArtifact: true,
			})
			result.Simplified = append(result.Simplified,
				fmt.Sprintf("archiveArtifacts %q → files-publication", pattern))
		}

	case "stash":
		name := jenkinsArgString(args, "name")
		includes := jenkinsArgString(args, "includes")
		if includes == "" {
			includes = "**/*"
		}
		j.FilesPublication = append(j.FilesPublication, FilePublication{
			Path:          includes,
			ShareWithJobs: true,
		})
		result.Simplified = append(result.Simplified,
			fmt.Sprintf("stash %q → files-publication with share-with-jobs", name))

	case "unstash":
		name := jenkinsArgValue(args)
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("unstash %q → ensure upstream job publishes via files-publication with share-with-jobs: true", name))

	case "publishHTML":
		result.ManualSetup = append(result.ManualSetup,
			"publishHTML → add HTML report as artifact in files-publication")

	case "mail", "emailext":
		result.ManualSetup = append(result.ManualSetup,
			"email notification → configure TeamCity email notifier in build features")

	case "slackSend":
		result.ManualSetup = append(result.ManualSetup,
			"Slack notification → configure TeamCity Slack notifier in build features")

	case "input":
		result.ManualSetup = append(result.ManualSetup,
			"input step (manual approval) → configure TeamCity deployment confirmation or manual trigger")

	case "build":
		jobName := jenkinsArgString(args, "job")
		if jobName == "" {
			jobName = jenkinsArgValue(args)
		}
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("build job: %q → configure as snapshot dependency in TeamCity", jobName))

	case "cleanWs":
		result.Simplified = append(result.Simplified, "cleanWs (TeamCity handles workspace cleanup)")

	case "deleteDir":
		result.Simplified = append(result.Simplified, "deleteDir (TeamCity handles workspace cleanup)")

	case "git":
		result.Simplified = append(result.Simplified, "git checkout (TeamCity VCS checkout is automatic)")

	case "readFile", "writeFile", "fileExists":
		script := jenkinsArgValue(args)
		j.Steps = append(j.Steps, Step{
			Name:          step.Name,
			ScriptContent: fmt.Sprintf("# TODO: Convert Jenkins %s step\n# Arguments: %s", step.Name, truncateAt(script, 80)),
		})

	default:
		result.NeedsReview = append(result.NeedsReview,
			fmt.Sprintf("Jenkins step %q → convert manually", step.Name))
		j.Steps = append(j.Steps, Step{
			Name:          step.Name,
			ScriptContent: fmt.Sprintf("# TODO: Convert Jenkins step %q\necho 'TODO: implement %s'", step.Name, step.Name),
		})
	}
}

func convertJenkinsPost(post *postAST, runner string, dependOnIDs []string, result *ConversionResult) []Job {
	var jobs []Job
	for _, cond := range post.Conditions {
		j := Job{
			ID:           "post_" + cond.Condition,
			Name:         "Post: " + cond.Condition,
			RunsOn:       runner,
			Dependencies: dependOnIDs,
		}
		for _, step := range cond.Branch.Steps {
			convertJenkinsStep(step, &j, result)
		}
		if len(j.Steps) == 0 {
			continue
		}
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Post %s → configure as build notification or failure handler in TeamCity", cond.Condition))
		jobs = append(jobs, j)
	}
	return jobs
}

func resolveJenkinsAgent(agent *agentAST, result *ConversionResult) string {
	if agent == nil {
		return "Ubuntu-24.04-Large"
	}

	switch agent.Type {
	case "any", "none", "":
		return "Ubuntu-24.04-Large"
	case "docker":
		image := jenkinsAgentArgValue(agent, "image")
		if image != "" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Docker agent image %q → add as docker-image on steps or use Docker wrapper build feature", image))
		}
		return "Ubuntu-24.04-Large"
	case "dockerfile":
		result.ManualSetup = append(result.ManualSetup,
			"Dockerfile agent → build Docker image on agent; configure Docker wrapper build feature")
		return "Ubuntu-24.04-Large"
	case "kubernetes":
		result.ManualSetup = append(result.ManualSetup,
			"Kubernetes agent → configure TeamCity Kubernetes cloud profile")
		return "Ubuntu-24.04-Large"
	case "label", "node":
		label := jenkinsAgentArgValue(agent, "")
		if label == "" {
			label = jenkinsAgentArgValue(agent, "label")
		}
		if mapped, ok := RunnerMap[label]; ok {
			return mapped
		}
		if label != "" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Agent label %q → configure matching agent in TeamCity", label))
			return label
		}
		return "Ubuntu-24.04-Large"
	default:
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Agent type %q → configure matching agent in TeamCity", agent.Type))
		return "Ubuntu-24.04-Large"
	}
}

func jenkinsAgentArgValue(agent *agentAST, key string) string {
	raw := agent.Arguments
	if len(raw) == 0 {
		raw = agent.Argument
	}
	if len(raw) == 0 {
		return ""
	}

	var argList []struct {
		Key   *string `json:"key"`
		Value any     `json:"value"`
	}
	if err := json.Unmarshal(raw, &argList); err == nil {
		for _, a := range argList {
			matchKey := (key == "" && a.Key == nil) || (a.Key != nil && *a.Key == key)
			if matchKey {
				return extractJenkinsStringValue(a.Value)
			}
		}
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	return ""
}

type jenkinsStepArgs struct {
	positional string
	named      map[string]string
}

func parseJenkinsStepArgs(raw json.RawMessage) jenkinsStepArgs {
	result := jenkinsStepArgs{named: map[string]string{}}
	if len(raw) == 0 {
		return result
	}

	var argList []struct {
		Key   *string `json:"key"`
		Value any     `json:"value"`
	}
	if err := json.Unmarshal(raw, &argList); err == nil && len(argList) > 0 {
		for _, a := range argList {
			val := extractJenkinsStringValue(a.Value)
			if a.Key == nil || *a.Key == "" {
				result.positional = val
			} else {
				result.named[*a.Key] = val
			}
		}
		return result
	}

	var singleArg struct {
		Value any `json:"value"`
	}
	if err := json.Unmarshal(raw, &singleArg); err == nil && singleArg.Value != nil {
		result.positional = extractJenkinsStringValue(singleArg.Value)
		return result
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		result.positional = s
	}

	return result
}

func jenkinsArgValue(args jenkinsStepArgs) string {
	return args.positional
}

// jenkinsScriptArg returns the effective script body for shell-like steps
// (sh, bat, powershell, pwsh, echo). Jenkins supports both positional and
// named forms: sh 'make' and sh(script: 'make') / echo(message: 'hi').
func jenkinsScriptArg(args jenkinsStepArgs) string {
	if args.positional != "" {
		return args.positional
	}
	if s := args.named["script"]; s != "" {
		return s
	}
	return args.named["message"]
}

func jenkinsArgString(args jenkinsStepArgs, key string) string {
	return args.named[key]
}

func extractJenkinsStringValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case map[string]any:
		if inner, ok := val["value"]; ok {
			return extractJenkinsStringValue(inner)
		}
		b, _ := json.Marshal(val)
		return string(b)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func describeJenkinsWhen(cond conditionAST) string {
	args := parseJenkinsStepArgs(cond.Arguments)
	val := jenkinsArgValue(args)
	if val != "" {
		return fmt.Sprintf("%s %s", cond.Name, val)
	}
	return cond.Name
}

func extractJenkinsParam(raw json.RawMessage, result *ConversionResult) {
	var param struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &param); err != nil {
		return
	}
	args := parseJenkinsStepArgs(param.Arguments)
	name := jenkinsArgString(args, "name")
	if name == "" {
		name = jenkinsArgValue(args)
	}
	if name != "" {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Jenkins parameter %q (type: %s) → add as TeamCity configuration parameter", name, param.Name))
	}
}

func extractJenkinsTrigger(raw json.RawMessage, result *ConversionResult) {
	var trigger map[string]any
	if err := json.Unmarshal(raw, &trigger); err != nil {
		return
	}
	for name, val := range trigger {
		switch name {
		case "cron":
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Jenkins cron trigger %v → configure scheduled trigger in TeamCity", val))
		case "pollSCM":
			result.ManualSetup = append(result.ManualSetup,
				"Jenkins pollSCM trigger → configure VCS trigger polling interval in TeamCity")
		default:
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Jenkins trigger %q → configure equivalent trigger in TeamCity", name))
		}
	}
}

func jenkinsJobIDs(jobs []Job) []string {
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		ids[i] = j.ID
	}
	return ids
}
