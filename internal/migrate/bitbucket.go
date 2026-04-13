package migrate

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func convertBitbucket(cfg CIConfig, data []byte) (*ConversionResult, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse bitbucket-pipelines.yml: %w", err)
	}

	result := NewResult(cfg)
	p := &Pipeline{
		Comment: "# Converted from: " + cfg.File + " (bitbucket)\n\n",
	}
	p.Jobs = convertBitbucketPipeline(doc, result)
	result.Pipeline = p
	return result, nil
}

func convertBitbucketPipeline(doc map[string]any, result *ConversionResult) []Job {
	pipelines, ok := doc["pipelines"].(map[string]any)
	if !ok {
		result.NeedsReview = append(result.NeedsReview,
			fmt.Sprintf("Full %s pipeline needs manual or AI-assisted conversion", Bitbucket))
		return []Job{{
			ID: "main", Name: "Main", RunsOn: "Ubuntu-24.04-Large",
			Steps: []Step{{
				Name:          "Placeholder",
				ScriptContent: fmt.Sprintf("echo 'TODO: Convert %s (%s) pipeline manually'\necho 'Use the migrate-to-teamcity skill with an AI agent for assisted conversion'", "bitbucket-pipelines.yml", Bitbucket),
			}},
		}}
	}

	if image, ok := doc["image"].(string); ok {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Global image %q → add as docker-image on steps or use Docker wrapper build feature", image))
	} else if imgMap, ok := doc["image"].(map[string]any); ok {
		if name, ok := imgMap["name"].(string); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Global image %q → add as docker-image on steps or use Docker wrapper build feature", name))
		}
	}

	if definitions, ok := doc["definitions"].(map[string]any); ok {
		if services, ok := definitions["services"].(map[string]any); ok {
			for svcName, svc := range services {
				if svcMap, ok := svc.(map[string]any); ok {
					if image, ok := svcMap["image"].(string); ok {
						result.ManualSetup = append(result.ManualSetup,
							fmt.Sprintf("Bitbucket service %q (image: %q) → configure as Docker Compose or agent-level service", svcName, image))
					}
				}
			}
		}
		if caches, ok := definitions["caches"].(map[string]any); ok && len(caches) > 0 {
			result.Simplified = append(result.Simplified, "Bitbucket caches → enable-dependency-cache: true")
		}
	}

	type stepList struct {
		branch string
		steps  []any
	}
	var allStepLists []stepList
	for branch, val := range pipelines {
		switch v := val.(type) {
		case []any:
			allStepLists = append(allStepLists, stepList{branch, v})
		case map[string]any:
			for pattern, pv := range v {
				if nestedSteps, ok := pv.([]any); ok {
					allStepLists = append(allStepLists, stepList{branch + "/" + pattern, nestedSteps})
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Bitbucket %s/%s pipeline → add branch filter or VCS trigger condition in TeamCity", branch, pattern))
				}
			}
		}
	}

	var jobs []Job
	jobIndex := 0
	for _, sl := range allStepLists {
		var prevIDs []string
		for _, s := range sl.steps {
			stepMap, ok := s.(map[string]any)
			if !ok {
				continue
			}

			var stepDefs []map[string]any
			// sequential is true for stage blocks (nested steps run in order);
			// false for a single step or parallel block (siblings fan out from prevIDs).
			sequential := false
			switch {
			case stepMap["step"] != nil:
				if stepDef, ok := stepMap["step"].(map[string]any); ok {
					stepDefs = []map[string]any{stepDef}
				}
			case stepMap["parallel"] != nil:
				if parallelItems, ok := stepMap["parallel"].([]any); ok {
					for _, p := range parallelItems {
						if pm, ok := p.(map[string]any); ok {
							if sd, ok := pm["step"].(map[string]any); ok {
								stepDefs = append(stepDefs, sd)
							}
						}
					}
				}
			case stepMap["stage"] != nil:
				if stage, ok := stepMap["stage"].(map[string]any); ok {
					sequential = true
					stageName, _ := stage["name"].(string)
					if stageSteps, ok := stage["steps"].([]any); ok {
						for _, ss := range stageSteps {
							if ssMap, ok := ss.(map[string]any); ok {
								if sd, ok := ssMap["step"].(map[string]any); ok {
									stepDefs = append(stepDefs, sd)
								}
							}
						}
					}
					if stageName != "" {
						result.ManualSetup = append(result.ManualSetup,
							fmt.Sprintf("Bitbucket stage %q → emitted as sequential job chain", stageName))
					}
				}
			}
			if len(stepDefs) == 0 {
				continue
			}

			var groupIDs []string
			deps := prevIDs
			for _, stepDef := range stepDefs {
				jobIndex++
				name, _ := stepDef["name"].(string)
				if name == "" {
					name = fmt.Sprintf("%s step %d", sl.branch, jobIndex)
				}
				id := fmt.Sprintf("%s_%d", sl.branch, jobIndex)
				j := Job{ID: id, Name: name, RunsOn: "Ubuntu-24.04-Large"}
				j.Dependencies = append(j.Dependencies, deps...)
				if sequential {
					deps = []string{id}
				}

				if image, ok := stepDef["image"].(string); ok {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Step %q uses image %q → add as docker-image on step", name, image))
				} else if imgMap, ok := stepDef["image"].(map[string]any); ok {
					if imgName, ok := imgMap["name"].(string); ok {
						result.ManualSetup = append(result.ManualSetup,
							fmt.Sprintf("Step %q uses image %q → add as docker-image on step", name, imgName))
					}
				}

				if services, ok := stepDef["services"].([]any); ok {
					svcNames := StringsFromSlice(services)
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Step %q uses services %v → configure as Docker Compose or agent-level services", name, svcNames))
				}

				if caches, ok := stepDef["caches"].([]any); ok && len(caches) > 0 {
					j.EnableDependencyCache = true
					result.Simplified = append(result.Simplified, fmt.Sprintf("Step %q caches → enable-dependency-cache: true", name))
				}

				if artifacts, ok := stepDef["artifacts"].([]any); ok {
					for _, a := range artifacts {
						if path, ok := a.(string); ok {
							j.FilesPublication = append(j.FilesPublication, FilePublication{
								Path:            path,
								ShareWithJobs:   true,
								PublishArtifact: true,
							})
						}
					}
				} else if artifactsMap, ok := stepDef["artifacts"].(map[string]any); ok {
					if paths, ok := artifactsMap["paths"].([]any); ok {
						for _, p := range paths {
							if path, ok := p.(string); ok {
								j.FilesPublication = append(j.FilesPublication, FilePublication{
									Path:            path,
									ShareWithJobs:   true,
									PublishArtifact: true,
								})
							}
						}
					}
					if download, ok := artifactsMap["download"].(bool); ok && download {
						result.ManualSetup = append(result.ManualSetup,
							fmt.Sprintf("Step %q artifact download → ensure upstream step publishes via files-publication", name))
					}
				}

				if trigger, ok := stepDef["trigger"].(string); ok && trigger == "manual" {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Step %q has manual trigger → configure TeamCity deployment confirmation or manual approval", name))
				}

				if afterScript := collectBitbucketScripts(stepDef["after-script"]); len(afterScript) > 0 {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Step %q has after-script → add as 'always execute' step in TeamCity", name))
				}

				if maxTime, ok := stepDef["max-time"].(int); ok {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Step %q max-time: %d minutes → configure build timeout in TeamCity", name, maxTime))
				}

				if size, ok := stepDef["size"].(string); ok && size != "" {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Step %q size: %q → select appropriate agent pool in TeamCity", name, size))
				}

				if scripts := collectBitbucketScripts(stepDef["script"]); len(scripts) > 0 {
					scripts = MapScriptVars(scripts, mapBitbucketVars)
					j.Steps = []Step{{Name: name, ScriptContent: strings.Join(scripts, "\n")}}
				}

				jobs = append(jobs, j)
				groupIDs = append(groupIDs, id)
			}
			// A sequential stage hands only its final job forward; a parallel
			// block / single step hands all its jobs forward so the next
			// pipeline item fans in from them.
			if sequential && len(groupIDs) > 0 {
				prevIDs = []string{groupIDs[len(groupIDs)-1]}
			} else {
				prevIDs = groupIDs
			}
		}
	}
	return jobs
}

func collectBitbucketScripts(v any) []string {
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range slice {
		switch s := item.(type) {
		case string:
			result = append(result, s)
		case map[string]any:
			if pipe, ok := s["pipe"].(string); ok {
				pipeScript := convertBitbucketPipe(pipe, s)
				result = append(result, pipeScript)
			}
		}
	}
	return result
}

// Bitbucket pipe conversion.

var bitbucketPipeMap = map[string]struct {
	script string
	note   string
}{
	"atlassian/aws-s3-deploy":             {"aws s3 sync %s s3://%s", "AWS credentials → create TeamCity secure parameters"},
	"atlassian/aws-cloudformation-deploy": {"aws cloudformation deploy --template-file %s --stack-name %s", "AWS credentials → create TeamCity secure parameters"},
	"atlassian/aws-ecs-deploy":            {"aws ecs update-service --cluster %s --service %s --force-new-deployment", "AWS credentials → create TeamCity secure parameters"},
	"atlassian/aws-ecr-push-image":        {"docker push %s", "AWS ECR credentials → configure Docker registry in TeamCity"},
	"atlassian/ssh-run":                   {"ssh %s %s", "SSH key → create TeamCity secure parameter"},
	"atlassian/scp-deploy":                {"scp -r %s %s", "SSH key → create TeamCity secure parameter"},
	"atlassian/slack-notify":              {"", "Slack notification → configure TeamCity Slack notifier"},
	"atlassian/trigger-pipeline":          {"", "Trigger pipeline → configure snapshot dependency in TeamCity"},
	"sonarsource/sonarcloud-scan":         {"sonar-scanner", "SonarCloud credentials → configure SonarCloud in TeamCity"},
	"sonarsource/sonarcloud-quality-gate": {"", "SonarCloud quality gate → configure in TeamCity build features"},
	"snyk/snyk-scan":                      {"snyk test", "Snyk token → create TeamCity secure parameter"},
	"atlassian/docker-publish":            {"docker build -t %s . && docker push %s", "Docker registry credentials → configure in TeamCity"},
}

func convertBitbucketPipe(pipe string, step map[string]any) string {
	pipeName := pipe
	if idx := strings.Index(pipe, ":"); idx >= 0 {
		pipeName = pipe[:idx]
	}

	if mapping, ok := bitbucketPipeMap[pipeName]; ok && mapping.script != "" {
		return fmt.Sprintf("# Converted from Bitbucket pipe: %s\n%s", pipe, mapping.script)
	}

	return fmt.Sprintf("# TODO: Replace Bitbucket pipe %q with equivalent commands\necho 'TODO: implement %s'", pipe, pipeName)
}
