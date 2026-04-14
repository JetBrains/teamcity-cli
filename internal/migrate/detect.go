package migrate

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rhysd/actionlint"
)

var ciPatterns = map[SourceCI][]string{
	GitHubActions: {".github/workflows/*.yml", ".github/workflows/*.yaml"},
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

	if source == GitHubActions {
		return analyzeGitHubActions(relPath, data)
	}
	return &CIConfig{Source: source, File: relPath, Features: []string{}}, nil
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

type parseError struct {
	msg string
}

func (e *parseError) Error() string {
	return e.msg
}
