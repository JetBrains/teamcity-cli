package migrate

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

type SourceCI string

const (
	GitHubActions SourceCI = "github-actions"
)

var validSources = map[SourceCI]bool{
	GitHubActions: true,
}

func ValidSource(s SourceCI) bool { return validSources[s] }

type CIConfig struct {
	Source   SourceCI `json:"source"`
	File     string   `json:"file"`
	Jobs     int      `json:"jobs"`
	Steps    int      `json:"steps"`
	Features []string `json:"features"`
}

type ConversionResult struct {
	SourceFile      string   `json:"sourceFile"`
	OutputFile      string   `json:"outputFile"`
	Source          SourceCI `json:"source"`
	YAML            string   `json:"yaml"`
	JobsConverted   int      `json:"jobsConverted"`
	StepsConverted  int      `json:"stepsConverted"`
	Simplified      []string `json:"simplified"`
	NeedsReview     []string `json:"needsReview"`
	ManualSetup     []string `json:"manualSetup"`
	Warnings        []string `json:"warnings"`
	ValidationError string   `json:"validationError,omitempty"`

	Pipeline *Pipeline `json:"-"`
}

type MigrateOutput struct {
	Sources []CIConfig          `json:"sources"`
	Results []*ConversionResult `json:"results"`
}

type Options struct {
	Ctx       context.Context
	RunnerMap map[string]string
	WorkDir   string // Source directory for resolving local includes.
}

func NewResult(cfg CIConfig) *ConversionResult {
	return &ConversionResult{
		SourceFile:  cfg.File,
		OutputFile:  OutputFileName(cfg.File),
		Source:      cfg.Source,
		Simplified:  []string{},
		NeedsReview: []string{},
		ManualSetup: []string{},
		Warnings:    []string{},
	}
}

func Convert(cfg CIConfig, data []byte, opts Options) (*ConversionResult, error) {
	var result *ConversionResult
	var err error

	switch cfg.Source {
	case GitHubActions:
		result, err = convertGitHub(cfg, data, opts)
	default:
		result = NewResult(cfg)
		result.Pipeline = fallbackPipeline(cfg, result)
	}

	if err != nil {
		return nil, err
	}

	result.YAML = result.Pipeline.String()
	result.JobsConverted = len(result.Pipeline.Jobs)
	for _, j := range result.Pipeline.Jobs {
		result.StepsConverted += len(j.Steps)
	}
	return result, nil
}

func fallbackPipeline(cfg CIConfig, result *ConversionResult) *Pipeline {
	result.NeedsReview = append(result.NeedsReview,
		fmt.Sprintf("Full %s pipeline needs manual or AI-assisted conversion", cfg.Source))
	return &Pipeline{
		Comment: "# Converted from: " + cfg.File + " (" + string(cfg.Source) + ")\n\n",
		Jobs: []Job{{
			ID: "main", Name: "Main", RunsOn: "Ubuntu-24.04-Large",
			Steps: []Step{{
				Name:          "Placeholder",
				ScriptContent: fmt.Sprintf("echo 'TODO: Convert %s (%s) pipeline manually'\necho 'Use the migrate-to-teamcity skill with an AI agent for assisted conversion'", cfg.File, cfg.Source),
			}},
		}},
	}
}

func OutputFileName(sourcePath string) string {
	normalized := filepath.ToSlash(sourcePath)
	base := filepath.Base(normalized)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if ext == "" {
		return name + ".tc.yml"
	}
	return name + ".tc" + ext
}

func DeduplicateOutputNames(results []*ConversionResult) {
	seen := map[string]int{}
	for _, r := range results {
		seen[r.OutputFile]++
	}
	for _, r := range results {
		if seen[r.OutputFile] <= 1 {
			continue
		}
		dir := filepath.Dir(filepath.ToSlash(r.SourceFile))
		prefix := strings.NewReplacer(".", "", "/", "-").Replace(dir)
		if prefix == "" {
			continue
		}
		ext := filepath.Ext(r.OutputFile)
		base := strings.TrimSuffix(r.OutputFile, ext)
		candidate := prefix + "-" + base + ext
		for i := 2; seen[candidate] > 0; i++ {
			candidate = fmt.Sprintf("%s-%s_%d%s", prefix, base, i, ext)
		}
		r.OutputFile = candidate
		seen[candidate]++
	}
}

func SortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

func CollectScripts(job map[string]any, key string) []string {
	return StringsOrScalar(job[key])
}

func StringsOrScalar(v any) []string {
	if s, ok := v.(string); ok {
		return []string{s}
	}
	return StringsFromSlice(v)
}

func StringsFromSlice(v any) []string {
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range slice {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func MapScriptVars(scripts []string, mapper func(string) string) []string {
	out := make([]string, len(scripts))
	for i, s := range scripts {
		out[i] = mapper(s)
	}
	return out
}

func NewVarMapper(varMap map[string]string) func(string) string {
	keys := make([]string, 0, len(varMap))
	for k := range varMap {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return len(b) - len(a) // longer keys first to avoid partial matches
	})
	pairs := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		pairs = append(pairs, k, varMap[k])
	}
	return strings.NewReplacer(pairs...).Replace
}

func (o Options) MapRunner(label string) string {
	if o.RunnerMap != nil {
		if mapped, ok := o.RunnerMap[label]; ok {
			return mapped
		}
	}
	if mapped, ok := RunnerMap[label]; ok {
		return mapped
	}
	return label
}

var RunnerMap = map[string]string{
	"ubuntu-latest":  "Ubuntu-24.04-Large",
	"ubuntu-24.04":   "Ubuntu-24.04-Large",
	"ubuntu-22.04":   "Ubuntu-22.04-Large",
	"macos-latest":   "macOS-15-Sequoia-Large-Arm64",
	"macos-15":       "macOS-15-Sequoia-Large-Arm64",
	"macos-14":       "macOS-14-Sonoma-Large-Arm64",
	"windows-latest": "Windows-Server-2022-Large",
	"windows-2022":   "Windows-Server-2022-Large",
}
