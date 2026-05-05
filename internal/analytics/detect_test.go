package analytics

import "testing"

func TestNormalizeOSArch(t *testing.T) {
	cases := map[string]string{
		"darwin":  "darwin",
		"linux":   "linux",
		"windows": "windows",
		"freebsd": "freebsd",
		"plan9":   "other",
	}
	for in, want := range cases {
		if got := NormalizeOS(in); got != want {
			t.Errorf("NormalizeOS(%q) = %q, want %q", in, got, want)
		}
	}
	archCases := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
		"386":   "386",
		"riscv": "other",
	}
	for in, want := range archCases {
		if got := NormalizeArch(in); got != want {
			t.Errorf("NormalizeArch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectCI(t *testing.T) {
	allEnv := []string{
		"TEAMCITY_BUILD_PROPERTIES_FILE", "TEAMCITY_VERSION",
		"GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "CIRCLECI",
		"BUILDKITE", "TF_BUILD", "TRAVIS", "CI",
	}
	clear := func(t *testing.T) {
		for _, k := range allEnv {
			t.Setenv(k, "")
		}
	}

	t.Run("none", func(t *testing.T) {
		clear(t)
		if got := DetectCI(); got != CINone {
			t.Errorf("DetectCI = %q, want %q", got, CINone)
		}
	})
	t.Run("teamcity wins over generic CI", func(t *testing.T) {
		clear(t)
		t.Setenv("TEAMCITY_VERSION", "2024.12")
		t.Setenv("CI", "true")
		if got := DetectCI(); got != CITeamCity {
			t.Errorf("DetectCI = %q, want %q", got, CITeamCity)
		}
	})
	t.Run("github actions", func(t *testing.T) {
		clear(t)
		t.Setenv("GITHUB_ACTIONS", "true")
		if got := DetectCI(); got != CIGitHubActions {
			t.Errorf("DetectCI = %q, want %q", got, CIGitHubActions)
		}
	})
	t.Run("generic CI falls through to other", func(t *testing.T) {
		clear(t)
		t.Setenv("CI", "true")
		if got := DetectCI(); got != CIOther {
			t.Errorf("DetectCI = %q, want %q", got, CIOther)
		}
	})
}

func TestClassifySource(t *testing.T) {
	cases := []struct {
		name string
		env  Environment
		want string
	}{
		{"human", Environment{AIAgent: "none", CISystem: CINone}, SourceHuman},
		{"agent always wins", Environment{AIAgent: "claude_code", CISystem: CITeamCity}, SourceAgent},
		{"build_step from teamcity", Environment{AIAgent: "none", CISystem: CITeamCity}, SourceBuildStep},
		{"ci from github", Environment{AIAgent: "none", CISystem: CIGitHubActions}, SourceCI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifySource(tc.env); got != tc.want {
				t.Errorf("ClassifySource = %q, want %q", got, tc.want)
			}
		})
	}
}
