package analytics

import "testing"

func TestAPIResource(t *testing.T) {
	cases := map[string]string{
		"":                       "other",
		"/":                      "other",
		"/app/rest/builds/12345": "builds",
		"/app/rest/builds":       "builds",
		"/app/rest/builds?locator=defaultFilter:false": "builds",
		"/app/rest/buildTypes/MyBuild":                 "build_types",
		"/app/rest/projects":                           "projects",
		"/app/rest/agentPools/1":                       "agents",
		"/app/rest/agents":                             "agents",
		"/app/rest/vcs-roots/id:Foo":                   "vcs",
		"/app/rest/vcs-root-instances":                 "vcs",
		"/app/rest/queue":                              "queue",
		"/app/rest/buildQueue":                         "builds",
		"/app/rest/testOccurrences":                    "tests",
		"/app/rest/problemOccurrences":                 "problems",
		"/app/rest/investigations":                     "problems",
		"/app/rest/changes":                            "changes",
		"/app/rest/pipelines":                          "pipelines",
		"/app/rest/cloud":                              "cloud",
		"/app/rest/server":                             "server",
		"/app/rest/users":                              "users",
		"/app/rest/userGroups":                         "users",
		"app/rest/builds":                              "builds",
		"/app/rest/some-future-thing":                  "other",
	}
	for in, want := range cases {
		if got := APIResource(in); got != want {
			t.Errorf("APIResource(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeHTTPMethod(t *testing.T) {
	cases := map[string]string{
		"GET":     "GET",
		"get":     "GET",
		"Post":    "POST",
		"DELETE":  "DELETE",
		"PATCH":   "PATCH",
		"OPTIONS": "GET",
		"":        "GET",
	}
	for in, want := range cases {
		if got := NormalizeHTTPMethod(in); got != want {
			t.Errorf("NormalizeHTTPMethod(%q) = %q, want %q", in, got, want)
		}
	}
}
