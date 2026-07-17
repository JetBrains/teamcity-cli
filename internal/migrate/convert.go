package migrate

import (
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

type SourceCI string

const (
	GitHubActions SourceCI = "github-actions"
	Bamboo        SourceCI = "bamboo"
)

var validSources = map[SourceCI]bool{
	GitHubActions: true,
	Bamboo:        true,
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
	ValidationError string   `json:"validationError,omitempty"`

	Pipeline *Pipeline `json:"-"`
}

type MigrateOutput struct {
	Sources []CIConfig          `json:"sources"`
	Results []*ConversionResult `json:"results"`
}

type Options struct {
	RunnerMap map[string]string
}

func NewResult(cfg CIConfig) *ConversionResult {
	return &ConversionResult{
		SourceFile:  cfg.File,
		OutputFile:  OutputFileName(cfg.File),
		Source:      cfg.Source,
		Simplified:  []string{},
		NeedsReview: []string{},
		ManualSetup: []string{},
	}
}

func Convert(cfg CIConfig, data []byte, opts Options) (*ConversionResult, error) {
	var result *ConversionResult
	var err error

	switch cfg.Source {
	case GitHubActions:
		result, err = convertGitHub(cfg, data, opts)
	case Bamboo:
		result, err = convertBamboo(cfg, data, opts)
	default:
		return nil, fmt.Errorf("unsupported CI source %q", cfg.Source)
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

func fallbackPipeline(cfg CIConfig, result *ConversionResult, opts Options) *Pipeline {
	result.NeedsReview = append(result.NeedsReview,
		fmt.Sprintf("Full %s pipeline needs manual or AI-assisted conversion", cfg.Source))
	return &Pipeline{
		Comment: "# Converted from: " + cfg.File + " (" + string(cfg.Source) + ")\n\n",
		Jobs: []Job{{
			ID: "main", Name: "Main", RunsOn: opts.MapRunner("ubuntu-latest"),
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
		// path.Dir, not filepath.Dir — on Windows filepath.Clean rewrites the slash-normalized path with backslashes.
		dir := path.Dir(filepath.ToSlash(r.SourceFile))
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

// condense collapses whitespace runs (incl. newlines) so multiline YAML values stay on one report line.
func condense(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func (o Options) MapRunner(label string) string {
	mapped, _ := o.ResolveRunner(label)
	return mapped
}

// ResolveRunner maps a CI runner label to a TC agent name; ok=false flags labels needing manual setup — "" for hosted labels the server has no image for, "self-hosted" for custom labels.
func (o Options) ResolveRunner(label string) (mapped string, ok bool) {
	if mapped, ok := o.RunnerMap[label]; ok {
		return mapped, true
	}
	if mapped, ok := RunnerMap[label]; ok {
		// A non-nil RunnerMap is server truth: a hosted label missing there has no matching agent, so omit runs-on rather than emit a name the server can't run.
		if o.RunnerMap != nil {
			return "", false
		}
		return mapped, true
	}
	if label == "self-hosted" {
		return label, true
	}
	switch classifyOS(label) {
	case "linux":
		return o.ResolveRunner("ubuntu-latest")
	case "mac":
		return o.ResolveRunner("macos-latest")
	case "windows":
		return o.ResolveRunner("windows-latest")
	}
	return "self-hosted", false
}

// classifyOS sniffs the OS out of a runner label or agent image name; "" when none matches.
func classifyOS(label string) string {
	low := strings.ToLower(label)
	switch {
	case strings.Contains(low, "ubuntu"), strings.Contains(low, "linux"):
		return "linux"
	case strings.Contains(low, "mac"), strings.Contains(low, "osx"), strings.Contains(low, "darwin"):
		return "mac"
	case strings.Contains(low, "win"):
		return "windows"
	}
	return ""
}

// RunnerMap holds the default GHA-label → JetBrains-hosted-agent mapping per the 2026.2 schema enum; overridden by schema- or cloud-derived names when connected.
var RunnerMap = map[string]string{
	"ubuntu-latest":  "Linux-Large",
	"ubuntu-24.04":   "Linux-Large",
	"ubuntu-22.04":   "Linux-Large",
	"ubuntu-20.04":   "Linux-Large",
	"macos-latest":   "Mac-Medium",
	"macos-15":       "Mac-Medium",
	"macos-14":       "Mac-Medium",
	"macos-13":       "Mac-Medium",
	"windows-latest": "Windows-Medium",
	"windows-2022":   "Windows-Medium",
	"windows-2019":   "Windows-Medium",
}
