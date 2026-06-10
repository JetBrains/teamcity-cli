package migrate

import (
	"cmp"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

func convertBamboo(cfg CIConfig, data []byte, opts Options) (*ConversionResult, error) {
	var spec map[string]any
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	result := NewResult(cfg)

	if _, ok := spec["plan"]; !ok {
		result.NeedsReview = append(result.NeedsReview,
			cfg.File+" has no top-level `plan:` block — likely a deployment or permissions spec; convert manually")
		result.Pipeline = fallbackPipeline(cfg, result)
		return result, nil
	}

	p := &Pipeline{Comment: fmt.Sprintf("# Converted from: %s (bamboo-specs)\n\n", cfg.File)}

	if vars, ok := spec["variables"].(map[string]any); ok && len(vars) > 0 {
		params := map[string]string{}
		for _, k := range SortedKeys(vars) {
			if bambooLooksSecret(k) {
				params[k] = bambooSecretPlaceholder
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Variable %q looks like a secret → store with `teamcity project token put` and reference as %%%s%%", k, k))
				continue
			}
			params[k] = MapBambooExpressions(fmt.Sprint(vars[k]))
		}
		// Bamboo plan variables are config params (referenced as %name%), not env vars — MapBambooExpressions emits %X% so keys must be unprefixed.
		p.ConfigParameters = params
	}

	stages, _ := spec["stages"].([]any)
	if len(stages) == 0 {
		result.NeedsReview = append(result.NeedsReview,
			"Bamboo spec has no `stages` block — nothing to convert")
		result.Pipeline = fallbackPipeline(cfg, result)
		return result, nil
	}

	var prevJobIDs []string
	for _, stageEntry := range stages {
		stageName, stageInfo, ok := bambooStageEntry(stageEntry)
		if !ok {
			continue
		}
		if boolFromAny(stageInfo["manual"]) {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Stage %q is manual → configure as approval / manual trigger in TeamCity", stageName))
		}
		if boolFromAny(stageInfo["final"]) {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Stage %q is final → set 'Even if some build steps have failed' on its job's steps in TeamCity", stageName))
		}

		jobNames := stringSliceFromAny(stageInfo["jobs"])
		var stageJobIDs []string
		for _, jobName := range jobNames {
			jobDef, _ := spec[jobName].(map[string]any)
			j := convertBambooJob(stageName, jobName, jobDef, prevJobIDs, opts, result)
			p.Jobs = append(p.Jobs, j)
			stageJobIDs = append(stageJobIDs, j.ID)
		}
		prevJobIDs = stageJobIDs
	}

	surfaceBambooMeta(spec, result)

	result.Pipeline = p
	return result, nil
}

func convertBambooJob(stageName, jobName string, def map[string]any, deps []string, opts Options, result *ConversionResult) Job {
	j := Job{
		// Keep the raw id; Pipeline.String() sanitizes and de-duplicates job keys and
		// resolves dependencies against them, so two names that collide after sanitization
		// (e.g. "Build-A" and "Build_A") stay distinct instead of merging.
		ID:           stageName + "_" + jobName,
		Name:         jobName,
		Dependencies: append([]string{}, deps...),
	}

	if def == nil {
		result.NeedsReview = append(result.NeedsReview,
			fmt.Sprintf("Job %q in stage %q has no top-level definition", jobName, stageName))
		j.Steps = []Step{{
			Name:          jobName,
			ScriptContent: fmt.Sprintf("echo 'TODO: missing job definition for %s'", jobName),
		}}
		return j
	}

	j.RunsOn = bambooRunsOn(def, opts, result, jobName)

	if docker, ok := def["docker"].(map[string]any); ok {
		if img, _ := docker["image"].(string); img != "" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q runs in docker image %q → wrap steps with TeamCity Docker container settings", jobName, img))
		}
	}

	var stepResults []StepResult

	for _, sub := range anySlice(def["artifact-subscriptions"]) {
		if subMap, ok := sub.(map[string]any); ok {
			stepResults = append(stepResults, transformBambooArtifactSubscription(subMap))
		}
	}

	for _, t := range bambooTaskList(def["tasks"]) {
		stepResults = append(stepResults, transformBambooTask(t, result, jobName, false))
	}

	for _, t := range bambooTaskList(def["final-tasks"]) {
		stepResults = append(stepResults, transformBambooTask(t, result, jobName, true))
	}

	steps, artifacts, cache := applyResults(stepResults, result)
	if len(steps) == 0 && len(stepResults) > 0 {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q has no convertible steps (all simplified or unsupported) → delete the job or replace with manual TC configuration", jobName))
		steps = []Step{{
			Name:          "No-op",
			ScriptContent: fmt.Sprintf("# TODO: All tasks in Bamboo job %q were simplified or unsupported (see manual-setup notes)\necho 'Job %s has no executable steps; configure manually or delete'", jobName, jobName),
		}}
	}
	j.Steps = steps
	j.EnableDependencyCache = cache

	for _, a := range anySlice(def["artifacts"]) {
		if amap, ok := a.(map[string]any); ok {
			artifacts = append(artifacts, bambooArtifact(amap))
		}
	}
	j.FilesPublication = artifacts

	if env := def["environment"]; env != nil {
		if params := bambooEnvParams(env, result, fmt.Sprintf("Job %q", jobName)); len(params) > 0 {
			j.Parameters = params
		}
	}

	return j
}

// bambooStageEntry unpacks `- 'StageName': {jobs: [...]}` into name + info.
func bambooStageEntry(entry any) (string, map[string]any, bool) {
	m, ok := entry.(map[string]any)
	if !ok || len(m) == 0 {
		return "", nil, false
	}
	keys := SortedKeys(m)
	if len(keys) == 0 {
		return "", nil, false
	}
	info, _ := m[keys[0]].(map[string]any)
	if info == nil {
		info = map[string]any{}
	}
	return keys[0], info, true
}

type bambooTask struct {
	identifier string
	body       map[string]any
}

func bambooTaskList(v any) []bambooTask {
	items := anySlice(v)
	out := make([]bambooTask, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, bambooTask{identifier: "script", body: map[string]any{"scripts": []any{s}}})
			continue
		}
		m, ok := item.(map[string]any)
		if !ok || len(m) == 0 {
			continue
		}
		for _, k := range SortedKeys(m) {
			out = append(out, bambooTask{identifier: k, body: bambooTaskBody(m[k])})
			break
		}
	}
	return out
}

// bambooTaskBody folds shorthand string/list bodies into the canonical {scripts: [...]} map.
func bambooTaskBody(v any) map[string]any {
	switch body := v.(type) {
	case map[string]any:
		return body
	case string:
		return map[string]any{"scripts": []any{body}}
	case []any:
		return map[string]any{"scripts": body}
	}
	return map[string]any{}
}

func transformBambooTask(t bambooTask, result *ConversionResult, jobName string, final bool) StepResult {
	transformer, ok := bambooTaskRegistry[t.identifier]
	if !ok {
		for prefix, fn := range bambooPluginKeyAliases {
			if strings.HasPrefix(t.identifier, prefix) {
				transformer = fn
				ok = true
				break
			}
		}
	}

	var sr StepResult
	if ok {
		sr = transformer(t.body, result, jobName)
	} else {
		sr = bambooUnknownTask(t.identifier, t.body)
	}

	if final {
		for i := range sr.Steps {
			sr.Steps[i].Name = "[final] " + sr.Steps[i].Name
		}
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q has Bamboo final-task %q → set step execution policy to 'Even if some build steps have failed' in TeamCity", jobName, t.identifier))
	}

	if env := t.body["environment"]; env != nil {
		params := bambooEnvParams(env, result, fmt.Sprintf("Task %q in job %q", t.identifier, jobName))
		if len(params) > 0 {
			for i := range sr.Steps {
				if sr.Steps[i].Parameters == nil {
					sr.Steps[i].Parameters = map[string]string{}
				}
				maps.Copy(sr.Steps[i].Parameters, params)
			}
		}
	}

	return sr
}

type bambooTransformer func(body map[string]any, result *ConversionResult, jobName string) StepResult

var bambooTaskRegistry = map[string]bambooTransformer{
	"script":            bambooScript,
	"command":           bambooCommand,
	"checkout":          bambooCheckout,
	"clean":             bambooClean,
	"maven":             bambooMaven,
	"mvn2":              bambooMaven,
	"mvn3":              bambooMaven,
	"ant":               bambooAnt,
	"gradle":            bambooGradle,
	"npm":               bambooNpm,
	"node":              bambooNode,
	"node_unit":         bambooNode,
	"docker":            bambooDocker,
	"docker-cli":        bambooDocker,
	"inject-variables":  bambooInjectVariables,
	"dump-variables":    bambooDumpVariables,
	"artifact-download": bambooArtifactDownload,
	"test-parser":       bambooTestParser,
	"j_unit":            bambooTestParser,
	"nunit-parser":      bambooTestParser,
	"mocha":             bambooTestParser,
	"ssh":               bambooSSH,
	"scp":               bambooSCP,
	"ms-build":          bambooMSBuild,
	"ms-test":           bambooMSTest,
	"visual-studio":     bambooVisualStudio,
	"nunit-runner":      bambooNUnitRunner,
	"fastlane":          bambooFastlane,
	"unlock-keychain":   bambooUnlockKeychain,
	"stop-job":          bambooStopJob,
	"repository-tag":    bambooRepoTag,
	"repository-branch": bambooRepoBranch,
	"repository-commit": bambooRepoCommit,
	"repository-push":   bambooRepoPush,
	"aws-code-deploy":   bambooAWSCodeDeploy,
	"grails":            bambooGrails,
	"gulp":              bambooNpmRunner("gulp"),
	"grunt":             bambooNpmRunner("grunt"),
	"bower":             bambooBower,
}

var bambooPluginKeyAliases = map[string]bambooTransformer{
	"any-task/plugin-key/com.atlassian.bamboo.plugins.maven":  bambooMaven,
	"any-task/plugin-key/com.atlassian.bamboo.plugins.ant":    bambooAnt,
	"any-task/plugin-key/com.atlassian.bamboo.plugins.script": bambooScript,
}

func bambooScript(body map[string]any, result *ConversionResult, jobName string) StepResult {
	scripts := stringSliceFromAny(body["scripts"])
	if len(scripts) == 0 {
		if file, _ := body["file"].(string); file != "" {
			arg, _ := body["argument"].(string)
			scripts = []string{strings.TrimSpace(file + " " + arg)}
		}
	}
	script := strings.Join(scripts, "\n")
	script = MapBambooExpressions(script)

	step := Step{
		Name:             stringField(body, "description", ""),
		ScriptContent:    script,
		WorkingDirectory: bambooWorkingDir(body),
	}
	if interpreter, _ := body["interpreter"].(string); interpreter == "WINDOWS_POWER_SHELL" {
		// TC `type: script` runs cmd.exe on Windows. A single line can be dispatched via
		// `powershell -Command "..."`, but a multi-line body can't be one cmd.exe argument —
		// leave it as raw PowerShell and flag it for the PowerShell runner.
		if strings.Contains(script, "\n") {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q has a multi-line Windows PowerShell task → run it under TeamCity's PowerShell runner; the step body is emitted as raw PowerShell", jobName))
		} else {
			step.ScriptContent = fmt.Sprintf("powershell -Command %q", script)
		}
	}
	return Converted([]Step{step})
}

func bambooCommand(body map[string]any, result *ConversionResult, jobName string) StepResult {
	exe, _ := body["executable"].(string)
	args, _ := body["argument"].(string)
	cmd := strings.TrimSpace(exe + " " + args)
	if cmd == "" {
		return Unknown("command", flattenStringMap(body))
	}
	return Converted([]Step{{
		Name:             stringField(body, "description", "Run "+exe),
		ScriptContent:    MapBambooExpressions(cmd),
		WorkingDirectory: bambooWorkingDir(body),
	}})
}

func bambooCheckout(body map[string]any, result *ConversionResult, jobName string) StepResult {
	repo, _ := body["repository"].(string)
	if repo != "" {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q checkout used Bamboo repo %q → ensure a TC VCS root for that repo is attached to the pipeline", jobName, repo))
	}
	return StepResult{Status: StatusSimplified, Note: "checkout (TeamCity VCS checkout is automatic)"}
}

func bambooClean(body map[string]any, result *ConversionResult, jobName string) StepResult {
	return StepResult{
		Status: StatusSimplified,
		Note:   "clean (set 'Clean checkout' on the VCS root in TeamCity)",
	}
}

func bambooMaven(body map[string]any, result *ConversionResult, jobName string) StepResult {
	goal := cmp.Or(stringFromAny(body["goal"]), stringFromAny(dig(body, "configuration", "goal")))
	projectFile := cmp.Or(stringFromAny(body["project-file"]), stringFromAny(dig(body, "configuration", "projectFile")))

	parts := []string{"mvn"}
	if projectFile != "" {
		parts = append(parts, "-f "+projectFile)
	}
	if goal != "" {
		parts = append(parts, goal)
	} else {
		parts = append(parts, "package")
	}
	cmd := strings.Join(parts, " ")

	steps := []Step{{
		Name:             stringField(body, "description", "Run Maven"),
		ScriptContent:    MapBambooExpressions(cmd),
		WorkingDirectory: bambooWorkingDir(body),
	}}

	jdk := cmp.Or(stringFromAny(body["jdk"]), stringFromAny(dig(body, "configuration", "buildJdk")))
	if jdk != "" {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Maven task uses JDK %q → ensure the build agent has the matching Java installed (or set JAVA_HOME)", jdk))
	}
	if bambooMavenPublishesTests(body) {
		result.ManualSetup = append(result.ManualSetup,
			"Maven `tests` flag was set → TeamCity auto-imports surefire reports; verify the report path matches")
	}
	return Converted(steps)
}

func bambooMavenPublishesTests(body map[string]any) bool {
	if v, ok := body["tests"]; ok && v != nil && fmt.Sprint(v) != "false" {
		return true
	}
	return stringFromAny(dig(body, "configuration", "testChecked")) == "true"
}

func bambooAnt(body map[string]any, result *ConversionResult, jobName string) StepResult {
	target := cmp.Or(stringFromAny(body["target"]), stringFromAny(body["targets"]))
	build := cmp.Or(stringFromAny(body["buildfile"]), stringFromAny(body["build-file"]))
	parts := []string{"ant"}
	if build != "" {
		parts = append(parts, "-f "+build)
	}
	if target != "" {
		parts = append(parts, target)
	}
	return Converted([]Step{{
		Name:             stringField(body, "description", "Run Ant"),
		ScriptContent:    MapBambooExpressions(strings.Join(parts, " ")),
		WorkingDirectory: bambooWorkingDir(body),
	}})
}

func bambooGradle(body map[string]any, result *ConversionResult, jobName string) StepResult {
	tasks := cmp.Or(stringFromAny(body["task"]), stringFromAny(body["tasks"]))
	if tasks == "" {
		tasks = "build"
	}
	wrapper := "./gradlew"
	if useSystem, _ := body["use-wrapper"].(bool); useSystem {
		wrapper = "./gradlew"
	} else if v, ok := body["executable"].(string); ok && v != "" {
		wrapper = v
	}
	return Converted([]Step{{
		Name:             stringField(body, "description", "Run Gradle"),
		ScriptContent:    MapBambooExpressions(wrapper + " " + tasks),
		WorkingDirectory: bambooWorkingDir(body),
	}})
}

func bambooNpm(body map[string]any, result *ConversionResult, jobName string) StepResult {
	cmd := stringFromAny(body["command"])
	if cmd == "" {
		cmd = "install"
	}
	return Converted([]Step{{
		Name:             stringField(body, "description", "npm "+cmd),
		ScriptContent:    MapBambooExpressions("npm " + cmd),
		WorkingDirectory: bambooWorkingDir(body),
	}})
}

func bambooNpmRunner(tool string) bambooTransformer {
	return func(body map[string]any, result *ConversionResult, jobName string) StepResult {
		task := cmp.Or(stringFromAny(body["task"]), stringFromAny(body["tasks"]))
		if task == "" {
			task = "default"
		}
		return Converted([]Step{{
			Name:             stringField(body, "description", "Run "+tool),
			ScriptContent:    MapBambooExpressions(fmt.Sprintf("npx %s %s", tool, task)),
			WorkingDirectory: bambooWorkingDir(body),
		}})
	}
}

func bambooBower(body map[string]any, result *ConversionResult, jobName string) StepResult {
	cmd := cmp.Or(stringFromAny(body["command"]), "install")
	return Converted([]Step{{
		Name:          stringField(body, "description", "Run Bower"),
		ScriptContent: MapBambooExpressions("bower " + cmd),
	}})
}

func bambooNode(body map[string]any, result *ConversionResult, jobName string) StepResult {
	script := stringFromAny(body["script"])
	args := stringFromAny(body["argument"])
	if script == "" {
		return Unknown("node", flattenStringMap(body))
	}
	cmd := strings.TrimSpace("node " + script + " " + args)
	return Converted([]Step{{
		Name:             stringField(body, "description", "Run Node"),
		ScriptContent:    MapBambooExpressions(cmd),
		WorkingDirectory: bambooWorkingDir(body),
	}})
}

func bambooDocker(body map[string]any, result *ConversionResult, jobName string) StepResult {
	cmdType := cmp.Or(stringFromAny(body["cmd"]), stringFromAny(body["command"]))
	image := stringFromAny(body["image"])
	switch cmdType {
	case "build":
		dockerfile := cmp.Or(stringFromAny(body["dockerfile"]), "Dockerfile")
		return Converted([]Step{{
			Name:          stringField(body, "description", "Docker build"),
			ScriptContent: MapBambooExpressions(fmt.Sprintf("docker build -f %s -t %s .", dockerfile, cmp.Or(image, "build:latest"))),
		}})
	case "push":
		return Converted([]Step{{
			Name:          stringField(body, "description", "Docker push"),
			ScriptContent: MapBambooExpressions("docker push " + cmp.Or(image, "build:latest")),
		}})
	case "run":
		args, _ := body["arguments"].(string)
		cmd := "docker run"
		if args != "" {
			cmd += " " + args
		}
		cmd += " " + cmp.Or(image, "build:latest")
		return Converted([]Step{{
			Name:          stringField(body, "description", "Docker run"),
			ScriptContent: MapBambooExpressions(cmd),
		}})
	}
	return Unknown("docker", flattenStringMap(body))
}

func bambooInjectVariables(body map[string]any, result *ConversionResult, jobName string) StepResult {
	file := stringFromAny(body["file"])
	if file == "" {
		return Unknown("inject-variables", flattenStringMap(body))
	}
	result.ManualSetup = append(result.ManualSetup,
		fmt.Sprintf("Job %q injects variables from %q → load via `set -a; . %s; set +a` or convert to TC parameters", jobName, file, file))
	return Converted([]Step{{
		Name:          stringField(body, "description", "Inject variables"),
		ScriptContent: MapBambooExpressions(fmt.Sprintf("set -a\n. %s\nset +a", file)),
	}})
}

func bambooDumpVariables(body map[string]any, result *ConversionResult, jobName string) StepResult {
	return Converted([]Step{{
		Name:          stringField(body, "description", "Dump variables"),
		ScriptContent: "env | sort",
	}})
}

func bambooArtifactDownload(body map[string]any, result *ConversionResult, jobName string) StepResult {
	src := cmp.Or(stringFromAny(body["source-plan"]), stringFromAny(body["sourcePlan"]))
	artifact := stringFromAny(body["artifact"])
	dest := cmp.Or(stringFromAny(body["destination"]), ".")
	if artifact == "" {
		result.NeedsReview = append(result.NeedsReview,
			fmt.Sprintf("Job %q has artifact-download with no artifact name → review manually", jobName))
		return StepResult{Status: StatusUnsupported, Note: "artifact-download without name"}
	}
	result.ManualSetup = append(result.ManualSetup,
		fmt.Sprintf("artifact-download for %q from %q → declare an artifact-dependency in TC pipeline `dependencies:`; downloaded to %q", artifact, src, dest))
	return StepResult{Status: StatusSimplified, Note: "artifact-download (configure as TC artifact-dependency)"}
}

func bambooTestParser(body map[string]any, result *ConversionResult, jobName string) StepResult {
	pattern := cmp.Or(
		stringFromAny(body["test-results"]),
		stringFromAny(body["resultsDirectory"]),
		stringFromAny(body["pattern"]),
		"**/test-reports/*.xml",
	)
	result.ManualSetup = append(result.ManualSetup,
		fmt.Sprintf("Job %q expects test reports at %q → TeamCity auto-imports JUnit/NUnit reports; ensure agent step runs the test command", jobName, pattern))
	return StepResult{Status: StatusSimplified, Note: "test-parser (TeamCity has built-in test-report import)"}
}

func bambooSSH(body map[string]any, result *ConversionResult, jobName string) StepResult {
	host := stringFromAny(body["host"])
	cmds := stringSliceFromAny(body["command"])
	if len(cmds) == 0 {
		cmds = stringSliceFromAny(body["commands"])
	}
	if host == "" || len(cmds) == 0 {
		return Unknown("ssh", flattenStringMap(body))
	}
	user := cmp.Or(stringFromAny(body["username"]), "$SSH_USER")
	target := user + "@" + host
	script := fmt.Sprintf("ssh %s <<'EOF'\n%s\nEOF", target, strings.Join(cmds, "\n"))
	result.ManualSetup = append(result.ManualSetup,
		fmt.Sprintf("Job %q SSH task → upload an SSH key with `teamcity project ssh-key upload` and reference it in the agent runner", jobName))
	return Converted([]Step{{
		Name:          stringField(body, "description", "SSH "+host),
		ScriptContent: MapBambooExpressions(script),
	}})
}

func bambooSCP(body map[string]any, result *ConversionResult, jobName string) StepResult {
	host := stringFromAny(body["host"])
	src := stringFromAny(body["local-path"])
	dst := stringFromAny(body["remote-path"])
	if host == "" || src == "" || dst == "" {
		return Unknown("scp", flattenStringMap(body))
	}
	user := cmp.Or(stringFromAny(body["username"]), "$SSH_USER")
	cmd := fmt.Sprintf("scp -r %s %s@%s:%s", src, user, host, dst)
	result.ManualSetup = append(result.ManualSetup,
		fmt.Sprintf("Job %q SCP task → upload an SSH key with `teamcity project ssh-key upload` and configure on the agent", jobName))
	return Converted([]Step{{
		Name:          stringField(body, "description", "SCP to "+host),
		ScriptContent: MapBambooExpressions(cmd),
	}})
}

func bambooMSBuild(body map[string]any, result *ConversionResult, jobName string) StepResult {
	sln := cmp.Or(stringFromAny(body["solution"]), stringFromAny(body["projectFile"]))
	if sln == "" {
		sln = "*.sln"
	}
	return Converted([]Step{{
		Name:          stringField(body, "description", "MSBuild"),
		ScriptContent: MapBambooExpressions("msbuild " + sln + " " + stringFromAny(body["arguments"])),
	}})
}

func bambooMSTest(body map[string]any, result *ConversionResult, jobName string) StepResult {
	asm := stringFromAny(body["test-files"])
	return Converted([]Step{{
		Name:          stringField(body, "description", "MSTest"),
		ScriptContent: MapBambooExpressions("mstest /testcontainer:" + asm),
	}})
}

func bambooVisualStudio(body map[string]any, result *ConversionResult, jobName string) StepResult {
	sln := cmp.Or(stringFromAny(body["solution"]), stringFromAny(body["projectFile"]))
	return Converted([]Step{{
		Name:          stringField(body, "description", "Visual Studio build"),
		ScriptContent: MapBambooExpressions("devenv " + sln + " /Build " + cmp.Or(stringFromAny(body["configuration"]), "Release")),
	}})
}

func bambooNUnitRunner(body map[string]any, result *ConversionResult, jobName string) StepResult {
	return Converted([]Step{{
		Name:          stringField(body, "description", "NUnit runner"),
		ScriptContent: MapBambooExpressions("nunit3-console " + stringFromAny(body["assembly"])),
	}})
}

func bambooFastlane(body map[string]any, result *ConversionResult, jobName string) StepResult {
	lane := cmp.Or(stringFromAny(body["lane"]), "release")
	return Converted([]Step{{
		Name:          stringField(body, "description", "Fastlane "+lane),
		ScriptContent: MapBambooExpressions("fastlane " + lane),
	}})
}

func bambooUnlockKeychain(body map[string]any, result *ConversionResult, jobName string) StepResult {
	keychain := cmp.Or(stringFromAny(body["keychain"]), "$HOME/Library/Keychains/login.keychain-db")
	result.ManualSetup = append(result.ManualSetup,
		fmt.Sprintf("Job %q unlocks keychain %q → store the password as a TC token (`teamcity project token put`) and reference it as %%KEYCHAIN_PASSWORD%%", jobName, keychain))
	return Converted([]Step{{
		Name:          "Unlock keychain",
		ScriptContent: "security unlock-keychain -p %KEYCHAIN_PASSWORD% " + keychain,
	}})
}

func bambooStopJob(body map[string]any, result *ConversionResult, jobName string) StepResult {
	return StepResult{Status: StatusUnsupported, Note: "stop-job (Bamboo-specific control flow; configure step `Execute step` policy or remove)"}
}

func bambooRepoTag(body map[string]any, result *ConversionResult, jobName string) StepResult {
	tag := cmp.Or(stringFromAny(body["name"]), "v%build.number%")
	return Converted([]Step{{
		Name:          "Tag repository",
		ScriptContent: MapBambooExpressions("git tag " + tag + "\ngit push origin " + tag),
	}})
}

func bambooRepoBranch(body map[string]any, result *ConversionResult, jobName string) StepResult {
	branch := stringFromAny(body["name"])
	return Converted([]Step{{
		Name:          "Create branch",
		ScriptContent: MapBambooExpressions("git checkout -b " + branch + "\ngit push -u origin " + branch),
	}})
}

func bambooRepoCommit(body map[string]any, result *ConversionResult, jobName string) StepResult {
	msg := cmp.Or(stringFromAny(body["message"]), "Automated commit %build.number%")
	return Converted([]Step{{
		Name:          "Commit",
		ScriptContent: MapBambooExpressions(fmt.Sprintf("git add -A\ngit diff --cached --quiet || git commit -m %q", msg)),
	}})
}

func bambooRepoPush(body map[string]any, result *ConversionResult, jobName string) StepResult {
	branch := cmp.Or(stringFromAny(body["branch"]), "%teamcity.build.branch%")
	return Converted([]Step{{
		Name:          "Push",
		ScriptContent: MapBambooExpressions("git push origin " + branch),
	}})
}

func bambooAWSCodeDeploy(body map[string]any, result *ConversionResult, jobName string) StepResult {
	app := stringFromAny(body["application-name"])
	group := stringFromAny(body["deployment-group"])
	bucket := stringFromAny(body["s3-bucket"])
	if app == "" || group == "" {
		return Unknown("aws-code-deploy", flattenStringMap(body))
	}
	result.ManualSetup = append(result.ManualSetup,
		fmt.Sprintf("Job %q triggers AWS CodeDeploy for %q → store AWS credentials with `teamcity project token put` and reference as env vars", jobName, app))
	return Converted([]Step{{
		Name: "AWS CodeDeploy",
		ScriptContent: MapBambooExpressions(fmt.Sprintf(
			"aws deploy create-deployment --application-name %s --deployment-group-name %s --s3-location bucket=%s,key=%%build.number%%.zip,bundleType=zip",
			app, group, cmp.Or(bucket, "$S3_BUCKET"))),
	}})
}

func bambooGrails(body map[string]any, result *ConversionResult, jobName string) StepResult {
	cmd := cmp.Or(stringFromAny(body["command"]), "test-app")
	return Converted([]Step{{
		Name:          "Grails " + cmd,
		ScriptContent: MapBambooExpressions("grails " + cmd),
	}})
}

func bambooUnknownTask(identifier string, body map[string]any) StepResult {
	var stub strings.Builder
	fmt.Fprintf(&stub, "# TODO: Replace Bamboo task %q with equivalent commands", identifier)
	if len(body) > 0 {
		stub.WriteString("\n# Task fields:")
		for _, k := range SortedKeys(body) {
			fmt.Fprintf(&stub, "\n%s", commentBlock(fmt.Sprintf("  %s: %v", k, body[k])))
		}
	}
	stub.WriteString("\necho 'TODO: implement equivalent of " + identifier + "'")
	return StepResult{
		Status:     StatusUnknown,
		Identifier: identifier,
		Note:       "Bamboo task: " + identifier,
		Steps: []Step{{
			Name:          identifier,
			ScriptContent: stub.String(),
		}},
	}
}

func transformBambooArtifactSubscription(sub map[string]any) StepResult {
	artifact := stringFromAny(sub["artifact"])
	if artifact == "" {
		return StepResult{Status: StatusUnsupported, Note: "artifact-subscription without name"}
	}
	return StepResult{
		Status: StatusSimplified,
		Note:   fmt.Sprintf("artifact-subscription %q (configure as TC artifact-dependency in pipeline `dependencies:`)", artifact),
	}
}

func bambooArtifact(a map[string]any) FilePublication {
	pattern, _ := a["pattern"].(string)
	location, _ := a["location"].(string)
	path := pattern
	if location != "" && pattern != "" {
		path = strings.TrimRight(location, "/") + "/" + pattern
	} else if location != "" {
		path = location
	}
	shared := boolFromAny(a["shared"])
	return FilePublication{
		Path:            path,
		ShareWithJobs:   shared,
		PublishArtifact: !shared,
	}
}

// bambooRequirementOSHints maps OS-shaped Bamboo capability labels to a GHA runner key (non-OS reqs surface as manual tasks).
var bambooRequirementOSHints = map[string]string{
	"linux":        "ubuntu-latest",
	"ubuntu":       "ubuntu-latest",
	"debian":       "ubuntu-latest",
	"centos":       "ubuntu-latest",
	"redhat":       "ubuntu-latest",
	"unix":         "ubuntu-latest",
	"macos":        "macos-latest",
	"mac":          "macos-latest",
	"darwin":       "macos-latest",
	"osx":          "macos-latest",
	"windows":      "windows-latest",
	"win":          "windows-latest",
	"win32":        "windows-latest",
	"windows-2022": "windows-2022",
	"ubuntu-22.04": "ubuntu-22.04",
	"ubuntu-24.04": "ubuntu-24.04",
}

func bambooRunsOn(def map[string]any, opts Options, result *ConversionResult, jobName string) string {
	reqs := stringSliceFromAny(def["requirements"])
	var osHint string
	var nonOS []string
	for _, r := range reqs {
		if hint, ok := bambooRequirementOSHints[strings.ToLower(r)]; ok && osHint == "" {
			osHint = hint
			continue
		}
		nonOS = append(nonOS, r)
	}
	if len(nonOS) > 0 {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Job %q has agent requirements %v → set agent capabilities/requirements in TeamCity", jobName, nonOS))
	}
	if osHint == "" {
		osHint = "ubuntu-latest"
	}
	return opts.MapRunner(osHint)
}

func surfaceBambooMeta(spec map[string]any, result *ConversionResult) {
	if triggers, ok := spec["triggers"].([]any); ok && len(triggers) > 0 {
		var names []string
		for _, t := range triggers {
			if m, ok := t.(map[string]any); ok {
				names = append(names, SortedKeys(m)...)
			} else if s, ok := t.(string); ok {
				names = append(names, s)
			}
		}
		if len(names) > 0 {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Triggers (%s) → configure as VCS / schedule trigger in TeamCity project settings", strings.Join(names, ", ")))
		}
	}
	if _, ok := spec["branches"]; ok {
		result.ManualSetup = append(result.ManualSetup,
			"Bamboo branch policy (`branches:`) → configure branch filters and feature branches in TeamCity VCS root settings")
	}
	if _, ok := spec["dependencies"]; ok {
		result.ManualSetup = append(result.ManualSetup,
			"Bamboo plan dependencies (`dependencies:`) → declare cross-pipeline `dependencies:` in TC YAML or use snapshot dependencies in the UI")
	}
	if perms, ok := spec["plan-permissions"]; ok && perms != nil {
		result.ManualSetup = append(result.ManualSetup,
			"plan-permissions block → configure project roles in TeamCity (Administration → Roles)")
	}
	if _, ok := spec["notifications"]; ok {
		result.ManualSetup = append(result.ManualSetup,
			"Bamboo notifications → configure as TeamCity notification rules per user/project")
	}
}

func analyzeBamboo(relPath string, data []byte) (*CIConfig, error) {
	cfg := &CIConfig{Source: Bamboo, File: relPath, Features: []string{}}

	var spec map[string]any
	if err := yaml.Unmarshal(data, &spec); err != nil {
		// Don't fail detection on malformed YAML; convertBamboo surfaces the parse error.
		return cfg, nil
	}

	if _, ok := spec["plan"]; !ok {
		cfg.Features = append(cfg.Features, "non-plan")
		return cfg, nil
	}

	stages, _ := spec["stages"].([]any)
	features := map[string]bool{}
	for _, stageEntry := range stages {
		_, info, ok := bambooStageEntry(stageEntry)
		if !ok {
			continue
		}
		if boolFromAny(info["manual"]) {
			features["manual-stage"] = true
		}
		if boolFromAny(info["final"]) {
			features["final-stage"] = true
		}
		for _, jobName := range stringSliceFromAny(info["jobs"]) {
			cfg.Jobs++
			jobDef, _ := spec[jobName].(map[string]any)
			if jobDef == nil {
				continue
			}
			tasks := bambooTaskList(jobDef["tasks"])
			finalTasks := bambooTaskList(jobDef["final-tasks"])
			cfg.Steps += len(tasks) + len(finalTasks)
			if _, ok := jobDef["docker"]; ok {
				features["docker"] = true
			}
			if a, ok := jobDef["artifacts"]; ok && len(anySlice(a)) > 0 {
				features["artifacts"] = true
			}
			for _, t := range append(tasks, finalTasks...) {
				switch t.identifier {
				case "ssh", "scp":
					features["ssh"] = true
				case "aws-code-deploy":
					features["aws-deploy"] = true
				case "test-parser", "j_unit", "nunit-parser", "mocha":
					features["tests"] = true
				}
			}
		}
	}
	if _, ok := spec["triggers"]; ok {
		features["triggers"] = true
	}
	if _, ok := spec["variables"]; ok {
		features["variables"] = true
	}
	for f := range features {
		cfg.Features = append(cfg.Features, f)
	}
	slices.Sort(cfg.Features)
	return cfg, nil
}

// bambooBraceVarRe matches `${name}` references; bare `$VAR` shell expansions pass through.
var bambooBraceVarRe = regexp.MustCompile(`\$\{([a-zA-Z][a-zA-Z0-9._]*)\}`)

// MapBambooExpressions rewrites Bamboo `${bamboo.*}` references to TeamCity %param% syntax.
func MapBambooExpressions(s string) string {
	return bambooBraceVarRe.ReplaceAllStringFunc(s, func(match string) string {
		name := bambooBraceVarRe.FindStringSubmatch(match)[1]
		if mapped, ok := bambooKnownVars[name]; ok {
			return mapped
		}
		if after, ok := strings.CutPrefix(name, "bamboo."); ok {
			return "%" + after + "%"
		}
		return match
	})
}

var bambooKnownVars = map[string]string{
	"bamboo.build.number":                      "%build.number%",
	"bamboo.repository.revision.number":        "%build.vcs.number%",
	"bamboo.repository.branch.name":            "%teamcity.build.branch%",
	"bamboo.repository.git.branch":             "%teamcity.build.branch%",
	"bamboo.repository.git.repositoryUrl":      "%vcsroot.url%",
	"bamboo.repository.name":                   "%vcsroot.name%",
	"bamboo.working.directory":                 "%teamcity.build.checkoutDir%",
	"bamboo.tmp.directory":                     "%system.teamcity.build.tempDir%",
	"bamboo.buildPlanName":                     "%teamcity.buildConfName%",
	"bamboo.planKey":                           "%system.teamcity.buildType.id%",
	"bamboo.shortPlanKey":                      "%system.teamcity.buildType.id%",
	"bamboo.planName":                          "%teamcity.projectName%",
	"bamboo.agentId":                           "%teamcity.agent.id%",
	"bamboo.agentWorkingDirectory":             "%teamcity.agent.work.dir%",
	"bamboo.build.timeStamp":                   "%build.start.date.timestamp%",
	"bamboo.buildTimeStamp":                    "%build.start.date.timestamp%",
	"bamboo.buildKey":                          "%system.teamcity.buildType.id%",
	"bamboo.buildResultKey":                    "%teamcity.build.id%",
	"bamboo.ManualBuildTriggerReason.userName": "%teamcity.build.triggeredBy.username%",
}

func bambooEnvParams(v any, result *ConversionResult, scope string) map[string]string {
	out := map[string]string{}
	set := func(name, value string) {
		if bambooLooksSecret(name) {
			out[name] = bambooSecretPlaceholder
			if result != nil {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("%s env %q looks like a secret → store with `teamcity project token put` and reference as %%env.%s%%", scope, name, name))
			}
			return
		}
		out[name] = MapBambooExpressions(value)
	}
	switch env := v.(type) {
	case map[string]any:
		for _, k := range SortedKeys(env) {
			set(k, fmt.Sprint(env[k]))
		}
	case string:
		// `environment: 'KEY=val OTHER=val2'` shorthand.
		for kv := range strings.FieldsSeq(env) {
			if i := strings.Index(kv, "="); i > 0 {
				set(kv[:i], kv[i+1:])
			}
		}
	}
	return out
}

func anySlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func stringSliceFromAny(v any) []string {
	if s, ok := v.(string); ok {
		return []string{s}
	}
	items := anySlice(v)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func boolFromAny(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(b, "true")
	}
	return false
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func stringField(m map[string]any, key, fallback string) string {
	if v := stringFromAny(m[key]); v != "" {
		return v
	}
	return fallback
}

// bambooWorkingDir resolves `working-dir` and rewrites any ${bamboo.X} so the step runs from the correct path.
func bambooWorkingDir(body map[string]any) string {
	wd := stringFromAny(body["working-dir"])
	if wd == "" {
		return ""
	}
	return MapBambooExpressions(wd)
}

func dig(m map[string]any, keys ...string) any {
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

// bambooSecretPlaceholder replaces the value of a secret-looking plan variable so it never lands in the generated YAML.
const bambooSecretPlaceholder = "TODO: set via `teamcity project token put`"

// bambooSecretMarkers are case-insensitive substrings that imply a variable holds a credential.
var bambooSecretMarkers = []string{"password", "sshkey", "passphrase", "secret", "token"}

func bambooLooksSecret(name string) bool {
	low := strings.ToLower(name)
	return slices.ContainsFunc(bambooSecretMarkers, func(m string) bool {
		return strings.Contains(low, m)
	})
}

func flattenStringMap(m map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = stringFromAny(v)
	}
	return out
}
