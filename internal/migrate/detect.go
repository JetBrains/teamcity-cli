package migrate

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rhysd/actionlint"
	"gopkg.in/yaml.v3"
)

var ciPatterns = map[SourceCI][]string{
	GitHubActions: {".github/workflows/*.yml", ".github/workflows/*.yaml"},
	GitLabCI:      {".gitlab-ci.yml", ".gitlab-ci.yaml"},
	Jenkins:       {"Jenkinsfile"},
	CircleCI:      {".circleci/config.yml", ".circleci/config.yaml"},
	AzureDevOps:   {"azure-pipelines.yml", "azure-pipelines.yaml"},
	TravisCI:      {".travis.yml"},
	Bitbucket:     {"bitbucket-pipelines.yml"},
}

func Detect(dir string, filterSource SourceCI) ([]CIConfig, error) {
	configs := []CIConfig{}

	for source, patterns := range ciPatterns {
		if filterSource != "" && source != filterSource {
			continue
		}
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				rel, _ := filepath.Rel(dir, match)
				rel = filepath.ToSlash(rel)
				cfg, err := analyzeFile(source, rel, match)
				if err != nil {
					cfg = &CIConfig{Source: source, File: rel, Features: []string{}}
				}
				configs = append(configs, *cfg)
			}
		}
	}

	slices.SortFunc(configs, func(a, b CIConfig) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		return strings.Compare(string(a.Source), string(b.Source))
	})
	return configs, nil
}

func analyzeFile(source SourceCI, relPath, absPath string) (*CIConfig, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	switch source {
	case GitHubActions:
		return analyzeGitHubActions(relPath, data)
	case Jenkins:
		return analyzeJenkins(relPath, data)
	default:
		return analyzeGenericYAML(source, relPath, data)
	}
}

func analyzeGitHubActions(relPath string, data []byte) (*CIConfig, error) {
	workflow, errs := actionlint.Parse(data)
	if workflow == nil {
		errMsg := "parse error"
		if len(errs) > 0 {
			errMsg = errs[0].Error()
		}
		return nil, &parseError{msg: errMsg}
	}

	cfg := &CIConfig{
		Source:   GitHubActions,
		File:     relPath,
		Jobs:     len(workflow.Jobs),
		Features: []string{},
	}

	features := map[string]bool{}
	for _, job := range workflow.Jobs {
		cfg.Steps += len(job.Steps)
		if job.Strategy != nil && job.Strategy.Matrix != nil {
			features["matrix"] = true
		}
		if job.Container != nil {
			features["docker"] = true
		}
		if job.Services != nil {
			features["services"] = true
		}
		for _, step := range job.Steps {
			analyzeGHAStep(step, features)
		}
	}

	for f := range features {
		cfg.Features = append(cfg.Features, f)
	}
	return cfg, nil
}

func analyzeGHAStep(step *actionlint.Step, features map[string]bool) {
	switch exec := step.Exec.(type) {
	case *actionlint.ExecRun:
		if exec.Run != nil && strings.Contains(exec.Run.Value, "docker") {
			features["docker"] = true
		}
	case *actionlint.ExecAction:
		if exec.Uses == nil {
			return
		}
		uses := exec.Uses.Value
		switch {
		case strings.Contains(uses, "actions/cache"):
			features["cache"] = true
		case strings.Contains(uses, "upload-artifact"), strings.Contains(uses, "download-artifact"):
			features["artifacts"] = true
		case strings.Contains(uses, "docker/"):
			features["docker"] = true
		}
		if strings.Contains(uses, "${{") && strings.Contains(uses, "secrets.") {
			features["secrets"] = true
		}
	}
}

var stageRe = regexp.MustCompile(`(?m)^\s*stage\s*\(`)
var shRe = regexp.MustCompile(`(?m)^\s*sh\s+['"]`)

func analyzeJenkins(relPath string, data []byte) (*CIConfig, error) {
	content := string(data)
	stages := stageRe.FindAllStringIndex(content, -1)
	steps := shRe.FindAllStringIndex(content, -1)

	features := []string{}
	if strings.Contains(content, "credentials(") || strings.Contains(content, "withCredentials") {
		features = append(features, "secrets")
	}
	if strings.Contains(content, "docker") || strings.Contains(content, "dockerfile") {
		features = append(features, "docker")
	}
	if strings.Contains(content, "parallel") {
		features = append(features, "parallel")
	}

	return &CIConfig{
		Source:   Jenkins,
		File:     relPath,
		Jobs:     len(stages),
		Steps:    len(steps),
		Features: features,
	}, nil
}

func analyzeGenericYAML(source SourceCI, relPath string, data []byte) (*CIConfig, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	cfg := &CIConfig{
		Source:   source,
		File:     relPath,
		Features: []string{},
	}

	features := map[string]bool{}

	switch source {
	case GitLabCI:
		analyzeGitLab(doc, cfg, features)
	case CircleCI:
		analyzeCircleCI(doc, cfg, features)
	case AzureDevOps:
		analyzeAzureDevOps(doc, cfg, features)
	case TravisCI:
		analyzeTravis(doc, cfg, features)
	case Bitbucket:
		analyzeBitbucket(doc, cfg, features)
	}

	for f := range features {
		cfg.Features = append(cfg.Features, f)
	}
	return cfg, nil
}

var GitLabReservedKeys = map[string]bool{
	"stages": true, "variables": true, "include": true, "default": true,
	"image": true, "services": true, "cache": true, "before_script": true,
	"after_script": true, "workflow": true,
}

func analyzeGitLab(doc map[string]any, cfg *CIConfig, features map[string]bool) {
	for key, val := range doc {
		if GitLabReservedKeys[key] {
			switch key {
			case "variables":
				features["variables"] = true
			case "cache":
				features["cache"] = true
			case "services":
				features["services"] = true
			case "image":
				features["docker"] = true
			}
			continue
		}
		if job, ok := val.(map[string]any); ok {
			cfg.Jobs++
			if scripts, ok := job["script"].([]any); ok {
				cfg.Steps += len(scripts)
			}
			if _, ok := job["image"]; ok {
				features["docker"] = true
			}
			if _, ok := job["artifacts"]; ok {
				features["artifacts"] = true
			}
			if _, ok := job["cache"]; ok {
				features["cache"] = true
			}
		}
	}
}

func analyzeCircleCI(doc map[string]any, cfg *CIConfig, features map[string]bool) {
	if jobs, ok := doc["jobs"].(map[string]any); ok {
		cfg.Jobs = len(jobs)
		for _, v := range jobs {
			if job, ok := v.(map[string]any); ok {
				if steps, ok := job["steps"].([]any); ok {
					cfg.Steps += len(steps)
				}
				if _, ok := job["docker"]; ok {
					features["docker"] = true
				}
			}
		}
	}
	if _, ok := doc["orbs"]; ok {
		features["orbs"] = true
	}
}

func analyzeAzureDevOps(doc map[string]any, cfg *CIConfig, features map[string]bool) {
	if stages, ok := doc["stages"].([]any); ok {
		for _, s := range stages {
			if stage, ok := s.(map[string]any); ok {
				if jobs, ok := stage["jobs"].([]any); ok {
					cfg.Jobs += len(jobs)
					for _, j := range jobs {
						if job, ok := j.(map[string]any); ok {
							if steps, ok := job["steps"].([]any); ok {
								cfg.Steps += len(steps)
							}
						}
					}
				}
			}
		}
	}
	if jobs, ok := doc["jobs"].([]any); ok {
		cfg.Jobs += len(jobs)
		for _, j := range jobs {
			if job, ok := j.(map[string]any); ok {
				if steps, ok := job["steps"].([]any); ok {
					cfg.Steps += len(steps)
				}
			}
		}
	}
	if _, ok := doc["variables"]; ok {
		features["variables"] = true
	}
}

func analyzeTravis(doc map[string]any, cfg *CIConfig, features map[string]bool) {
	cfg.Jobs = 1
	if scripts, ok := doc["script"].([]any); ok {
		cfg.Steps = len(scripts)
	}
	if _, ok := doc["matrix"]; ok {
		features["matrix"] = true
	}
	if _, ok := doc["env"]; ok {
		features["variables"] = true
	}
	if _, ok := doc["services"]; ok {
		features["services"] = true
	}
	if _, ok := doc["cache"]; ok {
		features["cache"] = true
	}
}

func analyzeBitbucket(doc map[string]any, cfg *CIConfig, features map[string]bool) {
	if pipelines, ok := doc["pipelines"].(map[string]any); ok {
		for _, branch := range pipelines {
			if steps, ok := branch.([]any); ok {
				for _, s := range steps {
					if step, ok := s.(map[string]any); ok {
						if _, ok := step["step"]; ok {
							cfg.Jobs++
							cfg.Steps++
						}
					}
				}
			}
		}
	}
	if _, ok := doc["image"]; ok {
		features["docker"] = true
	}
}

type parseError struct {
	msg string
}

func (e *parseError) Error() string {
	return e.msg
}
