package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rhysd/actionlint"
)

// Bamboo patterns cover the canonical bamboo-specs/ root plus one level up (multi-CI repos keep specs under e.g. Bamboo/bamboo-specs/) and one level down (!include layouts).
var ciPatterns = map[SourceCI][]string{
	GitHubActions: {".github/workflows/*.yml", ".github/workflows/*.yaml"},
	Bamboo: {
		"bamboo-specs/*.yml", "bamboo-specs/*.yaml",
		"bamboo-specs/*/*.yml", "bamboo-specs/*/*.yaml",
		"*/bamboo-specs/*.yml", "*/bamboo-specs/*.yaml",
		"bamboo.yml", "bamboo.yaml",
	},
}

var unsupportedCIFiles = []string{"Jenkinsfile", ".gitlab-ci.yml", ".circleci/config.yml"}

// DetectUnsupported returns CI configurations present in the repository that migrate does not convert.
func DetectUnsupported(dir string) []string {
	found := []string{}
	for _, name := range unsupportedCIFiles {
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err == nil && !info.IsDir() {
			found = append(found, name)
		}
	}
	return found
}

func Detect(dir string, filterSource SourceCI) ([]CIConfig, error) {
	configs := []CIConfig{}

	for source, patterns := range ciPatterns {
		if filterSource != "" && source != filterSource {
			continue
		}
		// Overlapping patterns (e.g. bamboo-specs/*/* and */bamboo-specs/*) can match the same file.
		seen := map[string]bool{}
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				return nil, err
			}
			for _, match := range matches {
				if seen[match] {
					continue
				}
				seen[match] = true
				rel, _ := filepath.Rel(dir, match)
				rel = filepath.ToSlash(rel)
				configs = append(configs, *analyzeFile(source, rel, match))
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

func analyzeFile(source SourceCI, relPath, absPath string) *CIConfig {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return &CIConfig{Source: source, File: relPath, Features: []string{}}
	}
	return analyzeContents(source, relPath, data)
}

func analyzeContents(source SourceCI, relPath string, data []byte) *CIConfig {
	switch source {
	case GitHubActions:
		return analyzeGitHubActions(relPath, data)
	case Bamboo:
		return analyzeBamboo(relPath, data)
	}
	return &CIConfig{Source: source, File: relPath, Features: []string{}}
}

// AnalyzeFile reads a single path and returns its CIConfig, inferring the source from the path when not provided.
func AnalyzeFile(path string, source SourceCI) (*CIConfig, error) {
	if source == "" {
		source = InferSource(path)
		if source == "" {
			return nil, fmt.Errorf("cannot infer CI source from %q; pass --from", path)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return analyzeContents(source, filepath.ToSlash(path), data), nil
}

// InferSource guesses a CI source from a path; returns "" when no heuristic matches.
func InferSource(path string) SourceCI {
	p := filepath.ToSlash(path)
	base := filepath.Base(p)
	switch {
	case strings.Contains(p, ".github/workflows/"):
		return GitHubActions
	case strings.Contains(p, "bamboo-specs/"), base == "bamboo.yml", base == "bamboo.yaml":
		return Bamboo
	}
	return ""
}

func analyzeGitHubActions(relPath string, data []byte) *CIConfig {
	cfg := &CIConfig{Source: GitHubActions, File: relPath, Features: []string{}}

	workflow, _ := actionlint.Parse(data)
	if workflow == nil {
		// Don't fail detection on a malformed workflow; conversion surfaces the parse error.
		return cfg
	}
	cfg.Jobs = len(workflow.Jobs)

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
	slices.Sort(cfg.Features)
	return cfg
}

func analyzeGHAStep(step *actionlint.Step, features map[string]bool) {
	if step.Env != nil {
		for _, v := range step.Env.Vars {
			if v != nil && v.Value != nil && secretsRe.MatchString(v.Value.Value) {
				features["secrets"] = true
			}
		}
	}
	switch exec := step.Exec.(type) {
	case *actionlint.ExecRun:
		if exec.Run == nil {
			return
		}
		if strings.Contains(exec.Run.Value, "docker") {
			features["docker"] = true
		}
		if secretsRe.MatchString(exec.Run.Value) {
			features["secrets"] = true
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
		for _, input := range exec.Inputs {
			if input != nil && input.Value != nil && secretsRe.MatchString(input.Value.Value) {
				features["secrets"] = true
				break
			}
		}
	}
}
