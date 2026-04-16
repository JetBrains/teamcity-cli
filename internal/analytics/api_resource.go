package analytics

import "strings"

// APIResource maps a `/app/rest/...` endpoint to a coarse resource bucket; unknown → "other".
func APIResource(endpoint string) string {
	p := strings.TrimSpace(endpoint)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "app/rest/")
	if p == "" {
		return "other"
	}
	if i := strings.IndexAny(p, "/?"); i >= 0 {
		p = p[:i]
	}
	switch p {
	case "builds", "buildQueue":
		return "builds"
	case "buildTypes":
		return "build_types"
	case "projects":
		return "projects"
	case "agents", "agentPools", "agentTypes":
		return "agents"
	case "users", "userGroups", "roles", "audit":
		return "users"
	case "vcs-roots", "vcs-root-instances":
		return "vcs"
	case "queue":
		return "queue"
	case "tests", "testOccurrences", "testScopes":
		return "tests"
	case "problems", "problemOccurrences", "investigations", "mutes":
		return "problems"
	case "changes":
		return "changes"
	case "pipelines":
		return "pipelines"
	case "cloud":
		return "cloud"
	case "server":
		return "server"
	default:
		return "other"
	}
}

// NormalizeHTTPMethod maps an HTTP method to the wire enum; unknown → GET.
func NormalizeHTTPMethod(method string) string {
	upper := strings.ToUpper(strings.TrimSpace(method))
	switch upper {
	case "GET", "POST", "PUT", "DELETE", "PATCH":
		return upper
	default:
		return "GET"
	}
}
