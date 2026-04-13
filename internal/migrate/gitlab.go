package migrate

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var defaultGitLabStages = []string{".pre", "build", "test", "deploy", ".post"}

func convertGitLab(cfg CIConfig, data []byte, opts Options) (*ConversionResult, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", cfg.File, err)
	}

	result := NewResult(cfg)

	resolveGitLabLocalIncludes(doc, opts.WorkDir, result)

	p := &Pipeline{
		Comment: "# Converted from: " + cfg.File + " (" + string(cfg.Source) + ")\n# Review and adjust for your TeamCity setup.\n\n",
	}
	p.Jobs = convertGitLabJobs(doc, result)
	result.Pipeline = p
	return result, nil
}

func resolveGitLabLocalIncludes(doc map[string]any, workDir string, result *ConversionResult) {
	includes := doc["include"]
	if includes == nil {
		return
	}

	var includeList []any
	switch v := includes.(type) {
	case string:
		includeList = []any{v}
	case []any:
		includeList = v
	case map[string]any:
		includeList = []any{v}
	default:
		return
	}

	for _, inc := range includeList {
		switch v := inc.(type) {
		case string:
			mergeGitLabLocalInclude(doc, v, workDir, result)
		case map[string]any:
			if local, ok := v["local"].(string); ok {
				mergeGitLabLocalInclude(doc, local, workDir, result)
			} else if remote, ok := v["remote"].(string); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("GitLab include remote: %q → fetch and merge manually before migration", remote))
			} else if template, ok := v["template"].(string); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("GitLab include template: %q → fetch GitLab template and merge manually", template))
			} else if project, ok := v["project"].(string); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("GitLab include project: %q → fetch from external project and merge manually", project))
			}
		}
	}

	delete(doc, "include")
}

func mergeGitLabLocalInclude(doc map[string]any, path, workDir string, result *ConversionResult) {
	if workDir == "" {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("GitLab include local: %q → merge this file manually (working directory not available)", path))
		return
	}

	// include:local paths beginning with "/" are relative to the repo root,
	// not absolute filesystem paths — strip the leading slash before joining.
	absPath := filepath.Join(workDir, strings.TrimPrefix(path, "/"))
	data, err := os.ReadFile(absPath)
	if err != nil {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("GitLab include local: %q → file not found at %s; merge manually", path, absPath))
		return
	}

	var included map[string]any
	if err := yaml.Unmarshal(data, &included); err != nil {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("GitLab include local: %q → failed to parse: %v", path, err))
		return
	}

	// Main file wins on conflicts. For map-valued keys (e.g. variables,
	// default), deep-merge so non-conflicting inner entries from the include
	// are preserved rather than dropped wholesale.
	for key, val := range included {
		existing, exists := doc[key]
		if !exists {
			doc[key] = val
			continue
		}
		existingMap, ok1 := existing.(map[string]any)
		incomingMap, ok2 := val.(map[string]any)
		if ok1 && ok2 {
			for k, v := range incomingMap {
				if _, ok := existingMap[k]; !ok {
					existingMap[k] = v
				}
			}
		}
	}

	result.Simplified = append(result.Simplified,
		fmt.Sprintf("include local: %q → merged into pipeline", path))
}

func convertGitLabJobs(doc map[string]any, result *ConversionResult) []Job {
	var globalBeforeScript []string
	globalBeforeScript = append(globalBeforeScript, CollectScripts(doc, "before_script")...)
	if defaults, ok := doc["default"].(map[string]any); ok {
		globalBeforeScript = append(globalBeforeScript, CollectScripts(defaults, "before_script")...)
	}

	var globalAfterScript []string
	globalAfterScript = append(globalAfterScript, CollectScripts(doc, "after_script")...)
	if defaults, ok := doc["default"].(map[string]any); ok {
		globalAfterScript = append(globalAfterScript, CollectScripts(defaults, "after_script")...)
	}

	globalVars := collectGitLabVars(doc)
	if globalVars == nil {
		globalVars = make(map[string]string)
	}
	if defMap, ok := doc["default"].(map[string]any); ok {
		maps.Copy(globalVars, collectGitLabVars(defMap))
	}

	if services, ok := doc["services"].([]any); ok && len(services) > 0 {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Global services %v → configure as Docker Compose or agent-level services", extractGitLabServiceNames(services)))
	}

	globalImage := ""
	if image, ok := doc["image"].(string); ok {
		globalImage = image
	} else if imgMap, ok := doc["image"].(map[string]any); ok {
		if name, ok := imgMap["name"].(string); ok {
			globalImage = name
		}
	}
	if globalImage != "" {
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Global Docker image %q → add as docker-image on steps or use Docker wrapper build feature", globalImage))
	}

	if includes := doc["include"]; includes != nil {
		result.ManualSetup = append(result.ManualSetup,
			"GitLab include: directives detected → included configs must be merged manually or fetched before migration")
	}

	if cache, ok := doc["cache"].(map[string]any); ok {
		if paths, ok := cache["paths"].([]any); ok && len(paths) > 0 {
			result.Simplified = append(result.Simplified, "global cache → enable-dependency-cache: true")
		}
	}

	stageOrder := defaultGitLabStages
	if rawStages, ok := doc["stages"].([]any); ok {
		stageOrder = make([]string, 0, len(rawStages))
		for _, s := range rawStages {
			if name, ok := s.(string); ok {
				stageOrder = append(stageOrder, name)
			}
		}
	}
	stageIdx := make(map[string]int, len(stageOrder))
	for i, s := range stageOrder {
		stageIdx[s] = i
	}

	resolvedJobs := map[string]map[string]any{}
	var jobKeys []string
	for _, key := range SortedKeys(doc) {
		if GitLabReservedKeys[key] || strings.HasPrefix(key, ".") {
			continue
		}
		raw, ok := doc[key].(map[string]any)
		if !ok {
			continue
		}
		resolvedJobs[key] = resolveGitLabExtends(raw, doc)
		jobKeys = append(jobKeys, key)
	}

	jobsByStage := map[string][]string{}
	for _, key := range jobKeys {
		stage, _ := resolvedJobs[key]["stage"].(string)
		if stage == "" {
			stage = "test"
		}
		jobsByStage[stage] = append(jobsByStage[stage], key)
	}

	var jobs []Job
	for _, key := range jobKeys {
		job := resolvedJobs[key]

		j := Job{ID: key, Name: key, RunsOn: "Ubuntu-24.04-Large"}

		if image, ok := job["image"].(string); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q uses image %q → add as docker-image on steps", key, image))
		} else if imgMap, ok := job["image"].(map[string]any); ok {
			if name, ok := imgMap["name"].(string); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Job %q uses image %q → add as docker-image on steps", key, name))
			}
		}

		if services, ok := job["services"].([]any); ok && len(services) > 0 {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q uses services %v → configure as Docker Compose or agent-level services", key, extractGitLabServiceNames(services)))
		}

		jobVars := collectGitLabVars(job)
		allVars := make(map[string]string, len(globalVars)+len(jobVars))
		maps.Copy(allVars, globalVars)
		maps.Copy(allVars, jobVars)
		if len(allVars) > 0 {
			j.Parameters = allVars
		}

		if needs, ok := job["needs"].([]any); ok {
			for _, n := range needs {
				switch v := n.(type) {
				case string:
					j.Dependencies = append(j.Dependencies, v)
				case map[string]any:
					if jobName, ok := v["job"].(string); ok {
						j.Dependencies = append(j.Dependencies, jobName)
					}
				}
			}
		} else {
			stage, _ := job["stage"].(string)
			if stage == "" {
				stage = "test"
			}
			if idx, ok := stageIdx[stage]; ok {
				for prev := idx - 1; prev >= 0; prev-- {
					if prevJobs := jobsByStage[stageOrder[prev]]; len(prevJobs) > 0 {
						j.Dependencies = append(j.Dependencies, prevJobs...)
						break
					}
				}
			}
		}

		if rules, ok := job["rules"].([]any); ok {
			for _, r := range rules {
				if rule, ok := r.(map[string]any); ok {
					if ifCond, ok := rule["if"].(string); ok {
						result.ManualSetup = append(result.ManualSetup,
							fmt.Sprintf("Job %q rule if: %s → configure as branch filter or VCS trigger condition", key, ifCond))
					}
					if changes, ok := rule["changes"].([]any); ok && len(changes) > 0 {
						result.ManualSetup = append(result.ManualSetup,
							fmt.Sprintf("Job %q rule changes: %v → configure VCS trigger rules with path patterns", key, StringsFromSlice(changes)))
					}
				}
			}
		}
		if only, ok := job["only"].([]any); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q only: %v → configure VCS trigger branch filter", key, StringsFromSlice(only)))
		}
		if except, ok := job["except"].([]any); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q except: %v → configure VCS trigger exclusion filter", key, StringsFromSlice(except)))
		}

		if when, ok := job["when"].(string); ok && when != "on_success" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q when: %s → configure execution policy in TeamCity", key, when))
		}

		if allowFailure, ok := job["allow_failure"].(bool); ok && allowFailure {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q has allow_failure → configure 'ignore build step error' in TeamCity", key))
		}

		var scripts []string
		if _, hasLocalBefore := job["before_script"]; hasLocalBefore {
			scripts = append(scripts, CollectScripts(job, "before_script")...)
		} else {
			scripts = append(scripts, globalBeforeScript...)
		}
		scripts = append(scripts, CollectScripts(job, "script")...)
		if _, hasLocalAfter := job["after_script"]; hasLocalAfter {
			scripts = append(scripts, CollectScripts(job, "after_script")...)
		} else {
			scripts = append(scripts, globalAfterScript...)
		}

		scripts = MapScriptVars(scripts, mapGitLabVars)

		if len(scripts) > 0 {
			j.Steps = []Step{{Name: key, ScriptContent: strings.Join(scripts, "\n")}}
		}

		if artifacts, ok := job["artifacts"].(map[string]any); ok {
			if paths, ok := artifacts["paths"].([]any); ok {
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
			if reports, ok := artifacts["reports"].(map[string]any); ok {
				for reportType := range reports {
					result.ManualSetup = append(result.ManualSetup,
						fmt.Sprintf("Job %q artifact report %q → configure in TeamCity build features (JUnit, code coverage, etc.)", key, reportType))
				}
			}
			if expireIn, ok := artifacts["expire_in"].(string); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Job %q artifact expire_in: %s → configure artifact cleanup rules in TeamCity", key, expireIn))
			}
		}

		if cache, ok := job["cache"].(map[string]any); ok {
			if paths, ok := cache["paths"].([]any); ok && len(paths) > 0 {
				j.EnableDependencyCache = true
				result.Simplified = append(result.Simplified,
					fmt.Sprintf("Job %q cache → enable-dependency-cache: true", key))
			}
		}

		if environment, ok := job["environment"].(map[string]any); ok {
			envName, _ := environment["name"].(string)
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q deploys to environment %q → configure TeamCity deployment configuration", key, envName))
		} else if envName, ok := job["environment"].(string); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q deploys to environment %q → configure TeamCity deployment configuration", key, envName))
		}

		if parallel, ok := job["parallel"].(map[string]any); ok {
			if matrix, ok := parallel["matrix"].([]any); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Job %q uses parallel matrix (%d combinations) → expand to separate jobs or parameterize", key, len(matrix)))
			}
		}

		if trigger, ok := job["trigger"].(map[string]any); ok {
			project, _ := trigger["project"].(string)
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q triggers downstream pipeline %q → configure as snapshot dependency in TeamCity", key, project))
		} else if trigger, ok := job["trigger"].(string); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Job %q triggers downstream pipeline %q → configure as snapshot dependency in TeamCity", key, trigger))
		}

		if retry, ok := job["retry"].(map[string]any); ok {
			if maxRetries, ok := retry["max"].(int); ok {
				result.ManualSetup = append(result.ManualSetup,
					fmt.Sprintf("Job %q retry max: %d → configure auto-retry in TeamCity build failure conditions", key, maxRetries))
			}
		}

		jobs = append(jobs, j)
	}
	return jobs
}

func collectGitLabVars(m map[string]any) map[string]string {
	vars, ok := m["variables"].(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(vars))
	for k, v := range vars {
		switch val := v.(type) {
		case string:
			result[k] = val
		case map[string]any:
			if v, ok := val["value"]; ok {
				result[k] = fmt.Sprint(v)
			}
		default:
			if v != nil {
				result[k] = fmt.Sprint(val)
			}
		}
	}
	return result
}

func resolveGitLabExtends(job map[string]any, doc map[string]any) map[string]any {
	return resolveGitLabExtendsWithVisited(job, doc, map[string]bool{})
}

func resolveGitLabExtendsWithVisited(job map[string]any, doc map[string]any, visited map[string]bool) map[string]any {
	ext, ok := job["extends"]
	if !ok {
		return job
	}

	var templateNames []string
	switch v := ext.(type) {
	case string:
		templateNames = []string{v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				templateNames = append(templateNames, s)
			}
		}
	}

	merged := make(map[string]any)
	for _, name := range templateNames {
		if visited[name] {
			continue
		}
		tmpl, ok := doc[name].(map[string]any)
		if !ok {
			continue
		}
		visited[name] = true
		tmpl = resolveGitLabExtendsWithVisited(tmpl, doc, visited)
		for k, v := range tmpl {
			if k != "extends" {
				merged[k] = v
			}
		}
	}
	for k, v := range job {
		if k != "extends" {
			merged[k] = v
		}
	}
	return merged
}

func extractGitLabServiceNames(services []any) []string {
	var names []string
	for _, s := range services {
		switch v := s.(type) {
		case string:
			names = append(names, v)
		case map[string]any:
			if name, ok := v["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}
