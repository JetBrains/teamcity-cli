package migrate

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var travisLanguageDefaults = map[string]struct{ install, script string }{
	"c":           {"./configure && make", "make test"},
	"cpp":         {"./configure && make", "make test"},
	"clojure":     {"lein deps", "lein test"},
	"go":          {"go get -t -v ./...", "go test -v ./..."},
	"java":        {"", "mvn install -DskipTests=true -B -V && mvn test -B"},
	"groovy":      {"", "gradle assemble && gradle check"},
	"node_js":     {"npm install", "npm test"},
	"php":         {"", "phpunit"},
	"python":      {"pip install -r requirements.txt", ""},
	"ruby":        {"bundle install --jobs=3 --retry=3", "rake"},
	"rust":        {"", "cargo build --verbose && cargo test --verbose"},
	"scala":       {"", "sbt test"},
	"elixir":      {"mix deps.get", "mix test"},
	"swift":       {"", "swift build && swift test"},
	"objective-c": {"", "xcodebuild -scheme default build test | xcpretty"},
	"dart":        {"pub get", "pub run test"},
	"haskell":     {"cabal install --only-dependencies", "cabal test"},
	"perl":        {"cpanm --installdeps .", "make test"},
	"r":           {"", "Rscript -e 'devtools::test()'"},
}

var travisPhases = []string{"before_install", "install", "before_script", "script", "after_script"}

func convertTravis(cfg CIConfig, data []byte) (*ConversionResult, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse Travis CI config: %w", err)
	}

	result := NewResult(cfg)
	jobs := convertTravisCI(doc, result)

	p := &Pipeline{
		Comment: "# Converted from: " + cfg.File + " (travis)\n\n",
		Jobs:    jobs,
	}
	result.Pipeline = p
	return result, nil
}

func convertTravisCI(doc map[string]any, result *ConversionResult) []Job {
	if jobsMap, ok := doc["jobs"].(map[string]any); ok {
		if includes, ok := jobsMap["include"].([]any); ok && len(includes) > 0 {
			return convertTravisJobMatrix(doc, includes, result)
		}
	}
	if matrix, ok := doc["matrix"].(map[string]any); ok {
		if includes, ok := matrix["include"].([]any); ok && len(includes) > 0 {
			return convertTravisJobMatrix(doc, includes, result)
		}
	}

	lang, _ := doc["language"].(string)
	j := buildTravisJob("build", "Build", doc, lang, result)
	addTravisMetadata(doc, result)
	return []Job{j}
}

func convertTravisJobMatrix(doc map[string]any, includes []any, result *ConversionResult) []Job {
	lang, _ := doc["language"].(string)
	var jobs []Job

	for i, entry := range includes {
		jobDoc, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		merged := make(map[string]any, len(doc)+len(jobDoc))
		for k, v := range doc {
			merged[k] = v
		}
		for k, v := range jobDoc {
			merged[k] = v
		}

		jobLang := lang
		if l, ok := jobDoc["language"].(string); ok {
			jobLang = l
		}

		name, _ := jobDoc["name"].(string)
		stage, _ := jobDoc["stage"].(string)
		if name == "" {
			if stage != "" {
				name = stage
			} else {
				name = fmt.Sprintf("job_%d", i+1)
			}
		}

		id := SanitizeJobID(name)
		j := buildTravisJob(id, name, merged, jobLang, result)

		if stage != "" && len(jobs) > 0 {
			prevStage, _ := includes[max(0, i-1)].(map[string]any)["stage"].(string)
			if prevStage != "" && prevStage != stage {
				j.Dependencies = []string{jobs[len(jobs)-1].ID}
			}
		}

		jobs = append(jobs, j)
	}

	addTravisMetadata(doc, result)
	return jobs
}

func buildTravisJob(id, name string, doc map[string]any, lang string, result *ConversionResult) Job {
	j := Job{ID: id, Name: name, RunsOn: resolveTravisOS(doc)}

	hasInstall := doc["install"] != nil
	hasScript := doc["script"] != nil
	defaults, hasDefaults := travisLanguageDefaults[lang]

	for _, phase := range travisPhases {
		switch v := doc[phase].(type) {
		case string:
			j.Steps = append(j.Steps, Step{Name: phase, ScriptContent: mapTravisVars(v)})
		case []any:
			if script := StringsFromSlice(v); len(script) > 0 {
				script = MapScriptVars(script, mapTravisVars)
				j.Steps = append(j.Steps, Step{Name: phase, ScriptContent: strings.Join(script, "\n")})
			}
		default:
			if hasDefaults {
				if phase == "install" && !hasInstall && defaults.install != "" {
					j.Steps = append(j.Steps, Step{Name: "install (" + lang + " default)", ScriptContent: defaults.install})
				}
				if phase == "script" && !hasScript && defaults.script != "" {
					j.Steps = append(j.Steps, Step{Name: "script (" + lang + " default)", ScriptContent: defaults.script})
				}
			}
		}
	}

	if len(j.Steps) == 0 {
		j.Steps = []Step{{Name: "Placeholder", ScriptContent: "echo 'TODO: add build steps'"}}
	}

	if lang != "" {
		if version := travisLanguageVersion(doc, lang); version != "" {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Travis %s version %s → pin via docker-image or install script", lang, version))
		}
	}

	if env, ok := doc["env"].(map[string]any); ok {
		if global, ok := env["global"].([]any); ok {
			params := make(map[string]string)
			for _, e := range global {
				if s, ok := e.(string); ok {
					if k, v, found := strings.Cut(s, "="); found {
						params[k] = v
					}
				} else if m, ok := e.(map[string]any); ok {
					if secure, ok := m["secure"].(string); ok {
						result.ManualSetup = append(result.ManualSetup,
							fmt.Sprintf("Travis encrypted env var (%s...) → create as TeamCity secure parameter", secure[:min(20, len(secure))]))
					}
				}
			}
			if len(params) > 0 {
				j.Parameters = params
			}
		}
	}

	if _, ok := doc["cache"]; ok {
		j.EnableDependencyCache = true
		result.Simplified = append(result.Simplified, "cache → enable-dependency-cache: true")
	}

	if services, ok := doc["services"].([]any); ok {
		svcNames := StringsFromSlice(services)
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Travis services %v → configure as Docker Compose or agent-level services", svcNames))
	}

	if deploy := doc["deploy"]; deploy != nil {
		switch d := deploy.(type) {
		case map[string]any:
			convertTravisDeployProvider(d, result)
		case []any:
			for _, item := range d {
				if dm, ok := item.(map[string]any); ok {
					convertTravisDeployProvider(dm, result)
				}
			}
		}
	}

	if addons, ok := doc["addons"].(map[string]any); ok {
		if apt, ok := addons["apt"].(map[string]any); ok {
			if packages, ok := apt["packages"].([]any); ok {
				pkgNames := StringsFromSlice(packages)
				if len(pkgNames) > 0 {
					installStep := Step{
						Name:          "Install apt packages",
						ScriptContent: "sudo apt-get update && sudo apt-get install -y " + strings.Join(pkgNames, " "),
					}
					j.Steps = append([]Step{installStep}, j.Steps...)
				}
			}
		}
		if _, ok := addons["homebrew"]; ok {
			result.ManualSetup = append(result.ManualSetup,
				"Travis addon homebrew → install packages via brew in a script step")
		}
		if _, ok := addons["sonarcloud"]; ok {
			result.ManualSetup = append(result.ManualSetup,
				"Travis addon sonarcloud → configure SonarCloud in TeamCity build features")
		}
	}

	return j
}

func addTravisMetadata(doc map[string]any, result *ConversionResult) {
	for _, phase := range []string{"after_success", "after_failure", "after_deploy"} {
		if doc[phase] != nil {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Travis %s → add as conditional step or TeamCity notification", phase))
		}
	}

	if branches, ok := doc["branches"].(map[string]any); ok {
		if only, ok := branches["only"].([]any); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Travis branches.only %v → configure VCS trigger branch filter", StringsFromSlice(only)))
		}
		if except, ok := branches["except"].([]any); ok {
			result.ManualSetup = append(result.ManualSetup,
				fmt.Sprintf("Travis branches.except %v → configure VCS trigger exclusion filter", StringsFromSlice(except)))
		}
	}
	if _, ok := doc["env"]; ok {
		result.ManualSetup = append(result.ManualSetup, "Travis env matrix → expand to separate jobs or parameterize")
	}
	if _, ok := doc["notifications"]; ok {
		result.ManualSetup = append(result.ManualSetup, "Notifications → configure TeamCity notifiers")
	}
}

func convertTravisDeployProvider(deploy map[string]any, result *ConversionResult) {
	provider, _ := deploy["provider"].(string)
	switch provider {
	case "pages", "github-pages":
		result.ManualSetup = append(result.ManualSetup,
			"Travis deploy pages → add git push to gh-pages branch as deployment step")
	case "npm":
		result.ManualSetup = append(result.ManualSetup,
			"Travis deploy npm → add 'npm publish' step with NPM_TOKEN as TeamCity secure parameter")
	case "pypi":
		result.ManualSetup = append(result.ManualSetup,
			"Travis deploy pypi → add 'twine upload' step with PyPI credentials as TeamCity secure parameters")
	case "heroku":
		result.ManualSetup = append(result.ManualSetup,
			"Travis deploy heroku → add 'git push heroku' or Heroku CLI deploy step")
	case "s3":
		bucket, _ := deploy["bucket"].(string)
		result.ManualSetup = append(result.ManualSetup,
			fmt.Sprintf("Travis deploy s3 (bucket: %q) → add 'aws s3 sync' step with AWS credentials", bucket))
	case "releases":
		result.ManualSetup = append(result.ManualSetup,
			"Travis deploy releases → add 'gh release create' step")
	default:
		result.NeedsReview = append(result.NeedsReview,
			fmt.Sprintf("Travis deploy provider %q → convert to TeamCity deployment steps", provider))
	}
}

func resolveTravisOS(doc map[string]any) string {
	os, _ := doc["os"].(string)
	switch os {
	case "osx":
		return "macOS-15-Sequoia-Large-Arm64"
	case "windows":
		return "Windows-Server-2022-Large"
	default:
		return "Ubuntu-24.04-Large"
	}
}

func travisLanguageVersion(doc map[string]any, lang string) string {
	versionKeys := map[string]string{
		"node_js": "node_js", "go": "go", "python": "python", "ruby": "rvm",
		"java": "jdk", "php": "php", "rust": "rust", "scala": "scala",
		"elixir": "elixir", "swift": "swift", "dart": "dart",
	}
	key, ok := versionKeys[lang]
	if !ok {
		return ""
	}
	switch v := doc[key].(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			return fmt.Sprint(v[0])
		}
	}
	return ""
}
