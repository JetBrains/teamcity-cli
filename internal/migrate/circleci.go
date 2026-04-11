package migrate

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type circleInvocation struct {
	id       string
	template string
	deps     []string
	params   map[string]any
	context  []string
}

type circleExecutorInfo struct {
	image   string
	runner  string
	envVars map[string]string
}

func convertCircleCI(cfg CIConfig, data []byte) (*ConversionResult, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse CircleCI config: %w", err)
	}

	result := NewResult(cfg)
	jobs := convertCircleCIJobs(doc, result)

	p := &Pipeline{
		Comment: "# Converted from: " + cfg.File + " (circleci)\n\n",
		Jobs:    jobs,
	}
	result.Pipeline = p
	return result, nil
}

func convertCircleCIJobs(doc map[string]any, result *ConversionResult) []Job {
	topJobs, ok := doc["jobs"].(map[string]any)
	if !ok {
		result.Warnings = append(result.Warnings, "no jobs found in CircleCI config")
		return nil
	}

	if orbs, ok := doc["orbs"].(map[string]any); ok {
		for orbName, orbDef := range orbs {
			switch v := orbDef.(type) {
			case string:
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("CircleCI orb %q (%s) → convert orb commands to equivalent shell scripts", orbName, v))
			case map[string]any:
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Inline orb %q → inline orb commands into job steps", orbName))
			}
		}
	}

	executors := map[string]circleExecutorInfo{}
	if execs, ok := doc["executors"].(map[string]any); ok {
		for name, exec := range execs {
			executors[name] = resolveCircleExecutor(exec)
		}
	}

	commands := map[string][]any{}
	if cmds, ok := doc["commands"].(map[string]any); ok {
		for name, cmd := range cmds {
			if cmdMap, ok := cmd.(map[string]any); ok {
				if steps, ok := cmdMap["steps"].([]any); ok {
					commands[name] = steps
				}
			}
		}
	}

	invocations := collectCircleInvocations(doc)
	if len(invocations) == 0 {
		for _, id := range SortedKeys(topJobs) {
			if _, ok := topJobs[id].(map[string]any); ok {
				invocations = append(invocations, circleInvocation{id: id, template: id})
			}
		}
	}

	var jobs []Job
	for _, inv := range invocations {
		job, ok := topJobs[inv.template].(map[string]any)
		if !ok {
			result.NeedsReview = append(result.NeedsReview,
				fmt.Sprintf("CircleCI job %q (template %q) not found in top-level jobs — likely an orb command", inv.id, inv.template))
			jobs = append(jobs, Job{
				ID: inv.id, Name: inv.id, RunsOn: "Ubuntu-24.04-Large",
				Dependencies: inv.deps,
				Steps: []Step{{
					Name:          inv.template,
					ScriptContent: fmt.Sprintf("# TODO: Convert orb/external job %q\necho 'TODO: implement %s'", inv.template, inv.template),
				}},
			})
			continue
		}

		j := Job{ID: inv.id, Name: inv.id, RunsOn: "Ubuntu-24.04-Large"}
		j.Dependencies = append(j.Dependencies, inv.deps...)

		exec := resolveCircleExecutor(job)
		if exec.image == "" {
			if execName, ok := job["executor"].(string); ok {
				if resolved, ok := executors[execName]; ok {
					exec = resolved
				}
			} else if execMap, ok := job["executor"].(map[string]any); ok {
				if name, ok := execMap["name"].(string); ok {
					if resolved, ok := executors[name]; ok {
						exec = resolved
					}
				}
			}
		}
		if exec.image != "" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q uses Docker image %q → add as docker-image on steps", inv.id, exec.image))
		}
		if exec.runner != "" {
			j.RunsOn = exec.runner
		}

		if len(inv.context) > 0 {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q uses contexts %v → create equivalent TeamCity parameter groups", inv.id, inv.context))
		}

		if env, ok := job["environment"].(map[string]any); ok {
			params := make(map[string]string, len(env))
			for k, v := range env {
				params[k] = fmt.Sprint(v)
			}
			j.Parameters = params
		}

		steps, ok := job["steps"].([]any)
		if !ok {
			continue
		}

		hasCache := false
		for _, s := range steps {
			switch v := s.(type) {
			case string:
				if v == "checkout" {
					result.Simplified = append(result.Simplified, "checkout (TeamCity VCS checkout is automatic)")
				} else {
					converted := convertCircleStepString(v, inv.id, commands, result)
					if converted != nil {
						j.Steps = append(j.Steps, *converted)
					}
				}
			case map[string]any:
				convertedSteps, cache := convertCircleStepMap(v, inv.id, commands, result)
				j.Steps = append(j.Steps, convertedSteps...)
				if cache {
					hasCache = true
				}
			}
		}

		j.EnableDependencyCache = hasCache
		jobs = append(jobs, j)
	}
	return jobs
}

func convertCircleStepString(step, jobID string, commands map[string][]any, result *ConversionResult) *Step {
	if cmdSteps, ok := commands[step]; ok {
		var scripts []string
		for _, cs := range cmdSteps {
			if csMap, ok := cs.(map[string]any); ok {
				if run, ok := csMap["run"].(map[string]any); ok {
					if cmd, ok := run["command"].(string); ok {
						scripts = append(scripts, cmd)
					}
				} else if run, ok := csMap["run"].(string); ok {
					scripts = append(scripts, run)
				}
			}
		}
		if len(scripts) > 0 {
			script := strings.Join(MapScriptVars(scripts, mapCircleCIVars), "\n")
			return &Step{Name: step, ScriptContent: script}
		}
	}

	if expanded := expandCircleOrbStep(step, nil); expanded != nil {
		result.Simplified = append(result.Simplified, fmt.Sprintf("orb command %q → expanded to shell", step))
		return expanded
	}

	result.NeedsReview = append(result.NeedsReview, fmt.Sprintf("CircleCI step %q in job %q", step, jobID))
	return &Step{
		Name:          step,
		ScriptContent: fmt.Sprintf("# TODO: Convert CircleCI step %q (possibly an orb command)\necho 'TODO: implement %s'", step, step),
	}
}

func convertCircleStepMap(v map[string]any, jobID string, commands map[string][]any, result *ConversionResult) ([]Step, bool) {
	hasCache := false

	if run, ok := v["run"].(map[string]any); ok {
		cmd, _ := run["command"].(string)
		name, _ := run["name"].(string)
		workDir, _ := run["working_directory"].(string)
		if cmd != "" {
			cmd = mapCircleCIVars(substituteCircleParams(cmd, nil))
			step := Step{Name: name, ScriptContent: cmd, WorkingDirectory: workDir}
			if env, ok := run["environment"].(map[string]any); ok {
				step.Parameters = make(map[string]string, len(env))
				for k, val := range env {
					step.Parameters[k] = fmt.Sprint(val)
				}
			}
			return []Step{step}, false
		}
	}
	if run, ok := v["run"].(string); ok {
		return []Step{{ScriptContent: mapCircleCIVars(run)}}, false
	}
	if _, ok := v["restore_cache"]; ok {
		hasCache = true
		result.Simplified = append(result.Simplified, "restore_cache → enable-dependency-cache: true")
		return nil, hasCache
	}
	if _, ok := v["save_cache"]; ok {
		hasCache = true
		result.Simplified = append(result.Simplified, "save_cache → enable-dependency-cache: true")
		return nil, hasCache
	}
	if persist, ok := v["persist_to_workspace"].(map[string]any); ok {
		root, _ := persist["root"].(string)
		if paths, ok := persist["paths"].([]any); ok {
			for _, p := range paths {
				if _, ok := p.(string); ok {
					result.Simplified = append(result.Simplified, "persist_to_workspace → files-publication")
					return nil, false
				}
			}
		}
		_ = root
		result.ManualSetup = append(result.ManualSetup, "persist_to_workspace → add files-publication with share-with-jobs: true")
		return nil, false
	}
	if attach, ok := v["attach_workspace"].(map[string]any); ok {
		at, _ := attach["at"].(string)
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("attach_workspace at: %q → ensure upstream job publishes via files-publication with share-with-jobs: true", at))
		return nil, false
	}
	if store, ok := v["store_artifacts"].(map[string]any); ok {
		path, _ := store["path"].(string)
		if path != "" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("store_artifacts path: %q → add files-publication with publish-artifact: true", path))
		}
		return nil, false
	}
	if store, ok := v["store_test_results"].(map[string]any); ok {
		path, _ := store["path"].(string)
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("store_test_results path: %q → TeamCity auto-processes test reports; ensure path is in build features", path))
		return nil, false
	}

	for key := range v {
		stepParams, _ := v[key].(map[string]any)

		if cmdSteps, ok := commands[key]; ok {
			var scripts []string
			for _, cs := range cmdSteps {
				if csMap, ok := cs.(map[string]any); ok {
					if run, ok := csMap["run"].(map[string]any); ok {
						if cmd, ok := run["command"].(string); ok {
							cmd = substituteCircleParams(cmd, stepParams)
							scripts = append(scripts, cmd)
						}
					} else if run, ok := csMap["run"].(string); ok {
						run = substituteCircleParams(run, stepParams)
						scripts = append(scripts, run)
					}
				}
			}
			if len(scripts) > 0 {
				script := strings.Join(MapScriptVars(scripts, mapCircleCIVars), "\n")
				return []Step{{Name: key, ScriptContent: script}}, false
			}
		}

		if expanded := expandCircleOrbStep(key, stepParams); expanded != nil {
			result.Simplified = append(result.Simplified, fmt.Sprintf("orb command %q → expanded to shell", key))
			return []Step{*expanded}, false
		}

		result.NeedsReview = append(result.NeedsReview, fmt.Sprintf("CircleCI step %q in job %q", key, jobID))
		return []Step{{
			Name:          key,
			ScriptContent: fmt.Sprintf("# TODO: Convert CircleCI step %q\necho 'TODO: implement %s'", key, key),
		}}, false
	}

	return nil, false
}

func resolveCircleExecutor(job any) circleExecutorInfo {
	info := circleExecutorInfo{}
	jobMap, ok := job.(map[string]any)
	if !ok {
		return info
	}

	if docker, ok := jobMap["docker"].([]any); ok && len(docker) > 0 {
		if img, ok := docker[0].(map[string]any); ok {
			if image, ok := img["image"].(string); ok {
				info.image = image
			}
			if env, ok := img["environment"].(map[string]any); ok {
				info.envVars = make(map[string]string, len(env))
				for k, v := range env {
					info.envVars[k] = fmt.Sprint(v)
				}
			}
		}
	}

	if machine, ok := jobMap["machine"].(map[string]any); ok {
		if image, ok := machine["image"].(string); ok {
			info.image = image
		}
		info.runner = "Ubuntu-24.04-Large"
	} else if machine, ok := jobMap["machine"].(bool); ok && machine {
		info.runner = "Ubuntu-24.04-Large"
	}

	if macos, ok := jobMap["macos"].(map[string]any); ok {
		_ = macos
		info.runner = "macOS-15-Sequoia-Large-Arm64"
	}

	if _, ok := jobMap["windows"]; ok {
		info.runner = "Windows-Server-2022-Large"
	}

	if rc, ok := jobMap["resource_class"].(string); ok {
		if strings.Contains(rc, "arm") {
			if info.runner == "" {
				info.runner = "Ubuntu-24.04-Large"
			}
		}
		if strings.Contains(rc, "windows") {
			info.runner = "Windows-Server-2022-Large"
		}
	}

	return info
}

func collectCircleInvocations(doc map[string]any) []circleInvocation {
	workflows, ok := doc["workflows"].(map[string]any)
	if !ok {
		return nil
	}

	var allInvocations []circleInvocation
	for wfName, wf := range workflows {
		wfMap, ok := wf.(map[string]any)
		if !ok {
			continue
		}
		wfJobs, ok := wfMap["jobs"].([]any)
		if !ok {
			continue
		}

		multiWf := len(workflows) > 1

		nameToID := map[string]string{}
		var wfInvocations []circleInvocation
		for _, j := range wfJobs {
			switch v := j.(type) {
			case string:
				id := v
				if multiWf {
					id = wfName + "_" + v
				}
				nameToID[v] = id
				wfInvocations = append(wfInvocations, circleInvocation{id: id, template: v})
			case map[string]any:
				for templateName, jobCfg := range v {
					id := templateName
					if multiWf {
						id = wfName + "_" + templateName
					}
					inv := circleInvocation{template: templateName}
					if cfg, ok := jobCfg.(map[string]any); ok {
						if name, ok := cfg["name"].(string); ok {
							if multiWf {
								id = wfName + "_" + name
							} else {
								id = name
							}
						}
						if requires, ok := cfg["requires"].([]any); ok {
							inv.deps = StringsFromSlice(requires)
						}
						if contexts, ok := cfg["context"].([]any); ok {
							inv.context = StringsFromSlice(contexts)
						} else if ctx, ok := cfg["context"].(string); ok {
							inv.context = []string{ctx}
						}
						if filters, ok := cfg["filters"].(map[string]any); ok {
							if branches, ok := filters["branches"].(map[string]any); ok {
								if only, ok := branches["only"].([]any); ok {
									_ = only
								}
							}
						}
					}
					inv.id = id
					if _, exists := nameToID[templateName]; !exists {
						nameToID[templateName] = id
					}
					if cfg, ok := jobCfg.(map[string]any); ok {
						if name, ok := cfg["name"].(string); ok {
							nameToID[name] = id
						}
					}
					wfInvocations = append(wfInvocations, inv)
				}
			}
		}

		for i, inv := range wfInvocations {
			resolved := make([]string, 0, len(inv.deps))
			for _, dep := range inv.deps {
				if resolvedID, ok := nameToID[dep]; ok {
					resolved = append(resolved, resolvedID)
				}
			}
			wfInvocations[i].deps = resolved
		}

		allInvocations = append(allInvocations, wfInvocations...)
	}
	return allInvocations
}

// CircleCI parameter substitution and orb expansion.

var circleParamRe = regexp.MustCompile(`<<\s*parameters\.([a-zA-Z0-9_-]+)\s*>>`)

func substituteCircleParams(s string, params map[string]any) string {
	return circleParamRe.ReplaceAllStringFunc(s, func(match string) string {
		m := circleParamRe.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		if val, ok := params[m[1]]; ok {
			return fmt.Sprint(val)
		}
		return match
	})
}

var orbCommandExpansions = map[string]func(inputs map[string]any) (string, string){
	"node/install-packages": func(inputs map[string]any) (string, string) {
		pm, _ := inputs["pkg-manager"].(string)
		if pm == "yarn" {
			return "Install packages (yarn)", "yarn install --frozen-lockfile"
		}
		return "Install packages (npm)", "npm ci"
	},
	"node/install": func(inputs map[string]any) (string, string) {
		version, _ := inputs["node-version"].(string)
		if version != "" {
			return "Install Node.js", fmt.Sprintf("# Install Node.js %s\nnvm install %s && nvm use %s", version, version, version)
		}
		return "Install Node.js", "# Node.js installed on agent"
	},
	"node/test": func(_ map[string]any) (string, string) {
		return "Run tests", "npm test"
	},
	"docker/build": func(inputs map[string]any) (string, string) {
		image, _ := inputs["image"].(string)
		tag, _ := inputs["tag"].(string)
		dockerfile, _ := inputs["dockerfile"].(string)
		cmd := "docker build"
		if dockerfile != "" && dockerfile != "Dockerfile" {
			cmd += fmt.Sprintf(" -f %q", dockerfile)
		}
		if image != "" {
			if tag != "" {
				cmd += fmt.Sprintf(" -t %s:%s", image, tag)
			} else {
				cmd += fmt.Sprintf(" -t %s", image)
			}
		}
		cmd += " ."
		return "Docker build", cmd
	},
	"docker/push": func(inputs map[string]any) (string, string) {
		image, _ := inputs["image"].(string)
		tag, _ := inputs["tag"].(string)
		if image != "" && tag != "" {
			return "Docker push", fmt.Sprintf("docker push %s:%s", image, tag)
		}
		if image != "" {
			return "Docker push", fmt.Sprintf("docker push %s", image)
		}
		return "Docker push", "docker push \"$IMAGE\""
	},
	"docker/check": func(inputs map[string]any) (string, string) {
		registry, _ := inputs["registry"].(string)
		if registry != "" {
			return "Docker login", fmt.Sprintf("echo \"$DOCKER_PASSWORD\" | docker login %s -u \"$DOCKER_LOGIN\" --password-stdin", registry)
		}
		return "Docker login", "echo \"$DOCKER_PASSWORD\" | docker login -u \"$DOCKER_LOGIN\" --password-stdin"
	},
	"aws-cli/install": func(_ map[string]any) (string, string) {
		return "Install AWS CLI", "curl \"https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip\" -o awscliv2.zip && unzip -q awscliv2.zip && sudo ./aws/install"
	},
	"aws-cli/setup": func(_ map[string]any) (string, string) {
		return "Configure AWS CLI", "aws configure set aws_access_key_id \"$AWS_ACCESS_KEY_ID\" && aws configure set aws_secret_access_key \"$AWS_SECRET_ACCESS_KEY\" && aws configure set region \"${AWS_DEFAULT_REGION:-us-east-1}\""
	},
	"aws-ecr/build-and-push-image": func(inputs map[string]any) (string, string) {
		repo, _ := inputs["repo"].(string)
		tag, _ := inputs["tag"].(string)
		if tag == "" {
			tag = "latest"
		}
		return "ECR build and push", fmt.Sprintf("aws ecr get-login-password | docker login --username AWS --password-stdin \"$AWS_ECR_REGISTRY\"\ndocker build -t \"%s:%s\" .\ndocker tag \"%s:%s\" \"$AWS_ECR_REGISTRY/%s:%s\"\ndocker push \"$AWS_ECR_REGISTRY/%s:%s\"", repo, tag, repo, tag, repo, tag, repo, tag)
	},
	"aws-ecs/update-service": func(inputs map[string]any) (string, string) {
		family, _ := inputs["family"].(string)
		cluster, _ := inputs["cluster"].(string)
		service, _ := inputs["service-name"].(string)
		return "ECS update service", fmt.Sprintf("aws ecs update-service --cluster %q --service %q --task-definition %q --force-new-deployment", cluster, service, family)
	},
	"aws-s3/sync": func(inputs map[string]any) (string, string) {
		from, _ := inputs["from"].(string)
		to, _ := inputs["to"].(string)
		return "S3 sync", fmt.Sprintf("aws s3 sync %s %s", from, to)
	},
	"aws-s3/copy": func(inputs map[string]any) (string, string) {
		from, _ := inputs["from"].(string)
		to, _ := inputs["to"].(string)
		return "S3 copy", fmt.Sprintf("aws s3 cp %s %s", from, to)
	},
	"heroku/deploy-via-git": func(inputs map[string]any) (string, string) {
		appName, _ := inputs["app-name"].(string)
		return "Heroku deploy", fmt.Sprintf("git push https://heroku:%s@git.heroku.com/%s.git HEAD:main", "$HEROKU_API_KEY", appName)
	},
	"slack/notify": func(_ map[string]any) (string, string) {
		return "Slack notification", "# TeamCity has built-in Slack integration\n# Configure in: Project Settings → Build Features → Slack Notifier"
	},
	"python/install-packages": func(inputs map[string]any) (string, string) {
		pm, _ := inputs["pkg-manager"].(string)
		if pm == "pipenv" {
			return "Install packages (pipenv)", "pipenv install --dev"
		}
		if pm == "poetry" {
			return "Install packages (poetry)", "poetry install"
		}
		return "Install packages (pip)", "pip install -r requirements.txt"
	},
	"ruby/install-deps": func(_ map[string]any) (string, string) {
		return "Install Ruby deps", "bundle install --jobs=4 --retry=3"
	},
	"go/install": func(inputs map[string]any) (string, string) {
		version, _ := inputs["version"].(string)
		if version != "" {
			return "Install Go", fmt.Sprintf("# Go %s — ensure installed on agent", version)
		}
		return "Install Go", "# Go installed on agent"
	},
	"go/test": func(_ map[string]any) (string, string) {
		return "Go tests", "go test -v ./..."
	},
	"go/mod-download": func(_ map[string]any) (string, string) {
		return "Go mod download", "go mod download"
	},
	"browser-tools/install-chrome": func(_ map[string]any) (string, string) {
		return "Install Chrome", "wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | sudo apt-key add - && sudo sh -c 'echo \"deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main\" > /etc/apt/sources.list.d/google-chrome.list' && sudo apt-get update && sudo apt-get install -y google-chrome-stable"
	},
	"browser-tools/install-firefox": func(_ map[string]any) (string, string) {
		return "Install Firefox", "sudo apt-get update && sudo apt-get install -y firefox"
	},
}

func expandCircleOrbStep(orbCommand string, inputs map[string]any) *Step {
	if expand, ok := orbCommandExpansions[orbCommand]; ok {
		name, script := expand(inputs)
		return &Step{Name: name, ScriptContent: script}
	}
	return nil
}
