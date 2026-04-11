package migrate

import (
	"cmp"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func convertAzure(cfg CIConfig, data []byte, opts Options) (*ConversionResult, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse Azure DevOps YAML: %w", err)
	}

	result := NewResult(cfg)
	jobs := convertAzurePipeline(doc, result, opts)

	result.Pipeline = &Pipeline{
		Comment: "# Converted from: " + cfg.File + " (azure-devops)\n\n",
		Jobs:    jobs,
	}
	return result, nil
}

func convertAzurePipeline(doc map[string]any, result *ConversionResult, opts Options) []Job {
	var jobs []Job

	if steps, ok := doc["steps"].([]any); ok {
		jobs = []Job{buildAzureJob("build", "Build", doc, steps, nil, result, opts)}
	} else if azJobs, ok := doc["jobs"].([]any); ok {
		jobs = buildAzureJobs(azJobs, doc, "", result, opts)
	} else if stages, ok := doc["stages"].([]any); ok {
		stageJobIDs := map[string][]string{}
		var prevStageID string
		for _, s := range stages {
			stage, ok := s.(map[string]any)
			if !ok {
				continue
			}
			stageID, _ := stage["stage"].(string)
			if stageID == "" {
				continue
			}
			stageJobs, ok := stage["jobs"].([]any)
			if !ok {
				continue
			}
			poolFallback := doc
			if _, ok := stage["pool"]; ok {
				poolFallback = stage
			}
			stageResults := buildAzureJobs(stageJobs, poolFallback, stageID, result, opts)

			var stageDeps []string
			if rawDeps := StringsOrScalar(stage["dependsOn"]); len(rawDeps) > 0 {
				for _, depStage := range rawDeps {
					stageDeps = append(stageDeps, stageJobIDs[depStage]...)
				}
			} else if _, explicit := stage["dependsOn"]; !explicit && prevStageID != "" {
				stageDeps = stageJobIDs[prevStageID]
			}
			for i := range stageResults {
				if len(stageResults[i].Dependencies) == 0 && len(stageDeps) > 0 {
					stageResults[i].Dependencies = append(stageResults[i].Dependencies, stageDeps...)
				}
			}

			var currentIDs []string
			for _, j := range stageResults {
				currentIDs = append(currentIDs, j.ID)
			}
			stageJobIDs[stageID] = currentIDs
			jobs = append(jobs, stageResults...)
			prevStageID = stageID
		}
	} else {
		result.NeedsReview = append(result.NeedsReview,
			"Full Azure DevOps pipeline needs manual or AI-assisted conversion")
		jobs = []Job{{
			ID: "main", Name: "Main", RunsOn: "Ubuntu-24.04-Large",
			Steps: []Step{{
				Name:          "Placeholder",
				ScriptContent: "echo 'TODO: Convert azure-pipelines.yml pipeline manually'\necho 'Use the migrate-to-teamcity skill with an AI agent for assisted conversion'",
			}},
		}}
	}

	if vars, ok := doc["variables"].(map[string]any); ok {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Azure variables (%d) → add as TeamCity parameters", len(vars)))
	} else if vars, ok := doc["variables"].([]any); ok {
		for _, v := range vars {
			if varMap, ok := v.(map[string]any); ok {
				if group, ok := varMap["group"].(string); ok {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Azure variable group %q → create corresponding parameter group in TeamCity", group))
				}
			}
		}
	}

	if trigger, ok := doc["trigger"].(map[string]any); ok {
		if branches, ok := trigger["branches"].(map[string]any); ok {
			if include, ok := branches["include"].([]any); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Azure trigger branches: %v → configure VCS trigger branch filter in TeamCity", StringsFromSlice(include)))
			}
		}
		if paths, ok := trigger["paths"].(map[string]any); ok {
			if include, ok := paths["include"].([]any); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Azure trigger paths: %v → configure VCS trigger rules in TeamCity", StringsFromSlice(include)))
			}
		}
	} else if _, ok := doc["trigger"]; ok {
		result.ManualSetup = append(result.ManualSetup, "Azure trigger → configure VCS trigger in TeamCity")
	}

	if _, ok := doc["pr"]; ok {
		result.ManualSetup = append(result.ManualSetup,
			"Azure PR trigger → configure pull request build trigger in TeamCity")
	}

	if schedules, ok := doc["schedules"].([]any); ok {
		for _, s := range schedules {
			if sched, ok := s.(map[string]any); ok {
				if cron, ok := sched["cron"].(string); ok {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Azure schedule cron: %q → configure scheduled trigger in TeamCity", cron))
				}
			}
		}
	}

	if resources, ok := doc["resources"].(map[string]any); ok {
		if containers, ok := resources["containers"].([]any); ok {
			for _, c := range containers {
				if container, ok := c.(map[string]any); ok {
					if image, ok := container["image"].(string); ok {
						result.ManualSetup = append(result.ManualSetup,
							fmt.Sprintf("Azure resource container %q → configure Docker image in TeamCity", image))
					}
				}
			}
		}
		if repos, ok := resources["repositories"].([]any); ok && len(repos) > 0 {
			result.ManualSetup = append(result.ManualSetup,
				"Azure resource repositories → configure additional VCS roots in TeamCity")
		}
	}

	return jobs
}

func buildAzureJob(id, name string, poolSource map[string]any, steps []any, deps []string, result *ConversionResult, opts Options) Job {
	j := Job{ID: id, Name: name, RunsOn: "Ubuntu-24.04-Large", Dependencies: deps}

	if pool, ok := poolSource["pool"].(map[string]any); ok {
		if image, ok := pool["vmImage"].(string); ok {
			j.RunsOn = opts.MapRunner(image)
		}
		if demands, ok := pool["demands"].([]any); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Azure pool demands %v → configure agent requirements in TeamCity", StringsFromSlice(demands)))
		}
	}

	hasCache := false
	for _, s := range steps {
		step, ok := s.(map[string]any)
		if !ok {
			continue
		}
		stepName, _ := step["displayName"].(string)

		switch {
		case step["script"] != nil:
			script := mapAzureVars(step["script"].(string))
			j.Steps = append(j.Steps, Step{Name: stepName, ScriptContent: script})
		case step["bash"] != nil:
			script := mapAzureVars(step["bash"].(string))
			j.Steps = append(j.Steps, Step{Name: stepName, ScriptContent: script})
		case step["pwsh"] != nil:
			script := mapAzureVars(step["pwsh"].(string))
			j.Steps = append(j.Steps, Step{Name: stepName, ScriptContent: script})
			result.NeedsReview = append(result.NeedsReview,
				fmt.Sprintf("Step %q uses pwsh — TC pipeline YAML has no shell selector; wrap script body with a pwsh/powershell invocation manually", stepName))
		case step["powershell"] != nil:
			script := mapAzureVars(step["powershell"].(string))
			j.Steps = append(j.Steps, Step{Name: stepName, ScriptContent: script})
			result.NeedsReview = append(result.NeedsReview,
				fmt.Sprintf("Step %q uses powershell — TC pipeline YAML has no shell selector; wrap script body with a pwsh/powershell invocation manually", stepName))
		case step["task"] != nil:
			taskRef, _ := step["task"].(string)
			taskSteps, taskCache := convertAzureTask(taskRef, step, stepName, result, opts)
			j.Steps = append(j.Steps, taskSteps...)
			if taskCache {
				hasCache = true
			}
		case step["template"] != nil:
			template, _ := step["template"].(string)
			result.NeedsReview = append(result.NeedsReview,
				fmt.Sprintf("Step uses template %q → inline template steps or convert separately", template))
			j.Steps = append(j.Steps, Step{
				Name:          cmp.Or(stepName, "Template: "+template),
				ScriptContent: fmt.Sprintf("# TODO: Inline template %q\necho 'TODO: implement template steps'", template),
			})
		case step["checkout"] != nil:
			result.Simplified = append(result.Simplified, "checkout (TeamCity VCS checkout is automatic)")
		case step["download"] != nil:
			result.ManualSetup = append(result.ManualSetup, "download step → configure artifact dependencies in TeamCity")
		case step["publish"] != nil:
			result.ManualSetup = append(result.ManualSetup, "publish step → add files-publication section")
		}
	}

	j.EnableDependencyCache = hasCache
	return j
}

func buildAzureJobs(azJobs []any, poolFallback map[string]any, stagePrefix string, result *ConversionResult, opts Options) []Job {
	var jobs []Job
	for _, j := range azJobs {
		job, ok := j.(map[string]any)
		if !ok {
			continue
		}
		id, _ := job["job"].(string)
		if id == "" {
			id = fmt.Sprintf("job_%d", len(jobs)+1)
		}
		name, _ := job["displayName"].(string)
		if name == "" {
			name = id
		}
		if stagePrefix != "" {
			id = stagePrefix + "_" + id
		}
		steps, _ := job["steps"].([]any)
		if len(steps) == 0 {
			if strategy, ok := job["strategy"].(map[string]any); ok {
				if runOnce, ok := strategy["runOnce"].(map[string]any); ok {
					if deploy, ok := runOnce["deploy"].(map[string]any); ok {
						if deploySteps, ok := deploy["steps"].([]any); ok {
							steps = deploySteps
							result.ManualSetup = append(result.ManualSetup,
								fmt.Sprintf("Azure deployment job %q → configure as TeamCity deployment configuration", id))
						}
					}
				}
			}
			if len(steps) == 0 {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Azure job %q has no steps (may be a deployment/template job) → add steps manually", id))
			}
		}

		if vars, ok := job["variables"].(map[string]any); ok && len(vars) > 0 {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Azure job %q has variables → add as TeamCity job parameters", id))
		}

		if condition, ok := job["condition"].(string); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Azure job %q condition: %s → configure execution policy in TeamCity", id, condition))
		}

		if continueOnError, ok := job["continueOnError"].(bool); ok && continueOnError {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Azure job %q has continueOnError → configure 'ignore build step error' in TeamCity", id))
		}

		poolSource := poolFallback
		if _, ok := job["pool"]; ok {
			poolSource = job
		}
		rawDeps := StringsOrScalar(job["dependsOn"])
		if stagePrefix != "" {
			for i, d := range rawDeps {
				rawDeps[i] = stagePrefix + "_" + d
			}
		}
		jobs = append(jobs, buildAzureJob(id, name, poolSource, steps, rawDeps, result, opts))
	}
	return jobs
}

// Azure DevOps task conversion.

var azureTaskMap = map[string]struct {
	script string
	note   string
}{
	"Npm":                        {"npm %s", ""},
	"NodeTool":                   {"", "NodeTool → ensure Node.js installed on agent"},
	"UseNode":                    {"", "UseNode → ensure Node.js installed on agent"},
	"UseDotNet":                  {"", "UseDotNet → ensure .NET SDK installed on agent"},
	"DotNetCoreCLI":              {"dotnet %s", ""},
	"Maven":                      {"mvn %s", ""},
	"Gradle":                     {"gradle %s", ""},
	"Docker":                     {"docker %s", ""},
	"DockerCompose":              {"docker-compose %s", ""},
	"PublishBuildArtifacts":      {"", "PublishBuildArtifacts → add files-publication section"},
	"DownloadBuildArtifacts":     {"", "DownloadBuildArtifacts → configure artifact dependencies in TeamCity"},
	"PublishPipelineArtifact":    {"", "PublishPipelineArtifact → add files-publication section"},
	"DownloadPipelineArtifact":   {"", "DownloadPipelineArtifact → configure artifact dependencies in TeamCity"},
	"PublishTestResults":         {"", "PublishTestResults → TeamCity auto-processes test reports"},
	"PublishCodeCoverageResults": {"", "PublishCodeCoverageResults → configure code coverage in TeamCity build features"},
	"CopyFiles":                  {"cp -r \"%s\" \"%s\"", ""},
	"AzureCLI":                   {"az %s", ""},
	"AzureWebApp":                {"az webapp deploy --name %s", ""},
	"AzureRmWebAppDeployment":    {"az webapp deploy --name %s", ""},
	"AzureKeyVault":              {"", "AzureKeyVault → create TeamCity secure parameters for vault secrets"},
	"SonarQubeAnalyze":           {"sonar-scanner", ""},
	"SonarQubePrepare":           {"", "SonarQubePrepare → configure SonarQube connection in TeamCity"},
	"SonarQubePublish":           {"", "SonarQubePublish → TeamCity SonarQube integration handles publishing"},
	"Cache":                      {"", "Cache → enable-dependency-cache: true"},
	"Checkout":                   {"", "Checkout → TeamCity VCS checkout is automatic"},
	"PowerShell":                 {"", ""},
	"CmdLine":                    {"", ""},
	"Bash":                       {"", ""},
}

func convertAzureTask(taskRef string, step map[string]any, stepName string, result *ConversionResult, opts Options) ([]Step, bool) {
	taskName := taskRef
	if idx := strings.Index(taskRef, "@"); idx >= 0 {
		taskName = taskRef[:idx]
	}

	inputs, _ := step["inputs"].(map[string]any)

	if mapping, ok := azureTaskMap[taskName]; ok {
		if mapping.note != "" {
			result.ManualSetup = append(result.ManualSetup, mapping.note)
		}
		switch {
		case taskName == "Cache":
			return nil, true
		case taskName == "Checkout" || taskName == "NodeTool" || taskName == "UseNode" || taskName == "UseDotNet":
			result.Simplified = append(result.Simplified, fmt.Sprintf("%s → handled by agent tooling", taskName))
			return nil, false
		case taskName == "PowerShell" || taskName == "CmdLine" || taskName == "Bash":
			script := ""
			if inputs != nil {
				if s, ok := inputs["script"].(string); ok {
					script = s
				} else if s, ok := inputs["inline"].(string); ok {
					script = s
				}
			}
			if script == "" {
				script = fmt.Sprintf("echo 'TODO: implement %s task'", taskName)
			}
			script = mapAzureVars(script)
			if taskName == "PowerShell" {
				result.NeedsReview = append(result.NeedsReview,
					fmt.Sprintf("Step %q uses PowerShell — wrap with powershell invocation manually", stepName))
			}
			return []Step{{Name: cmp.Or(stepName, taskName), ScriptContent: script}}, false
		case mapping.script != "":
			cmd := buildAzureTaskCommand(taskName, mapping.script, inputs)
			cmd = mapAzureVars(cmd)
			return []Step{{
				Name:          cmp.Or(stepName, "Azure task: "+taskName),
				ScriptContent: cmd,
			}}, false
		default:
			return nil, false
		}
	}

	if stepName == "" {
		stepName = "Azure task: " + taskName
	}
	result.NeedsReview = append(result.NeedsReview, fmt.Sprintf("Azure DevOps task %q in step %q", taskRef, stepName))
	stub := fmt.Sprintf("# TODO: Convert Azure DevOps task %q", taskRef)
	if inputs != nil {
		stub += "\n# Task inputs:"
		for k, v := range inputs {
			stub += fmt.Sprintf("\n#   %s: %v", k, v)
		}
	}
	stub += fmt.Sprintf("\necho 'TODO: implement %s'", taskName)
	return []Step{{Name: stepName, ScriptContent: stub}}, false
}

func buildAzureTaskCommand(taskName, template string, inputs map[string]any) string {
	if inputs == nil {
		return strings.TrimSpace(fmt.Sprintf(template, ""))
	}

	switch taskName {
	case "Npm":
		cmd, _ := inputs["command"].(string)
		return fmt.Sprintf(template, cmd)
	case "DotNetCoreCLI":
		cmd, _ := inputs["command"].(string)
		projects, _ := inputs["projects"].(string)
		args, _ := inputs["arguments"].(string)
		return strings.TrimSpace(fmt.Sprintf("dotnet %s %s %s", cmd, projects, args))
	case "Maven":
		goals, _ := inputs["goals"].(string)
		options, _ := inputs["options"].(string)
		return strings.TrimSpace(fmt.Sprintf("mvn %s %s", goals, options))
	case "Gradle":
		tasks, _ := inputs["tasks"].(string)
		return fmt.Sprintf("gradle %s", tasks)
	case "Docker":
		cmd, _ := inputs["command"].(string)
		return fmt.Sprintf("docker %s", cmd)
	case "DockerCompose":
		action, _ := inputs["action"].(string)
		return fmt.Sprintf("docker-compose %s", action)
	case "CopyFiles":
		src, _ := inputs["sourceFolder"].(string)
		dst, _ := inputs["targetFolder"].(string)
		return fmt.Sprintf("cp -r %q %q", src, dst)
	case "AzureCLI":
		script, _ := inputs["inlineScript"].(string)
		if script != "" {
			return script
		}
		return "az"
	case "AzureWebApp", "AzureRmWebAppDeployment":
		appName, _ := inputs["appName"].(string)
		return fmt.Sprintf("az webapp deploy --name %q", appName)
	}

	return fmt.Sprintf("# %s task — convert manually", taskName)
}
