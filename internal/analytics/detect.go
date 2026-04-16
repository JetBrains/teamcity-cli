package analytics

import (
	"os"
	"runtime"

	"github.com/tiulpin/instill"
)

type Environment struct {
	OS       string
	Arch     string
	CISystem string
	AIAgent  string
}

func DetectEnvironment() Environment {
	return Environment{
		OS:       NormalizeOS(runtime.GOOS),
		Arch:     NormalizeArch(runtime.GOARCH),
		CISystem: DetectCI(),
		AIAgent:  DetectAIAgent(),
	}
}

func NormalizeOS(goos string) string {
	switch goos {
	case "darwin", "linux", "windows", "freebsd":
		return goos
	default:
		return "other"
	}
}

func NormalizeArch(goarch string) string {
	switch goarch {
	case "amd64", "arm64", "386":
		return goarch
	default:
		return "other"
	}
}

func DetectAIAgent() string {
	r := instill.DetectRuntime()
	if r == nil {
		return CINone
	}
	return NormalizeAIAgent(r.Name)
}

// DetectCI returns the wire-enum value for the surrounding CI system; teamcity wins over generic CI.
func DetectCI() string {
	if os.Getenv("TEAMCITY_BUILD_PROPERTIES_FILE") != "" || os.Getenv("TEAMCITY_VERSION") != "" {
		return CITeamCity
	}
	if os.Getenv("GITHUB_ACTIONS") != "" {
		return CIGitHubActions
	}
	if os.Getenv("GITLAB_CI") != "" {
		return CIGitLab
	}
	if os.Getenv("JENKINS_URL") != "" {
		return CIJenkins
	}
	if os.Getenv("CIRCLECI") != "" {
		return CICircleCI
	}
	if os.Getenv("BUILDKITE") != "" {
		return CIBuildkite
	}
	if os.Getenv("TF_BUILD") != "" {
		return CIAzure
	}
	if os.Getenv("TRAVIS") != "" {
		return CITravis
	}
	if isTruthy(os.Getenv("CI")) {
		return CIOther
	}
	return CINone
}

// ClassifySource collapses environment context into the four-bucket source taxonomy.
func ClassifySource(env Environment) string {
	if env.AIAgent != "none" {
		return SourceAgent
	}
	if env.CISystem == CITeamCity {
		return SourceBuildStep
	}
	if env.CISystem != CINone {
		return SourceCI
	}
	return SourceHuman
}
