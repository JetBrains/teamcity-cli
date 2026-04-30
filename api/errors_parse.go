package api

import (
	"cmp"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const maxErrorBody = 1 << 20 // 1 MiB

var (
	// permissionQuotedRE matches `You do not have "X" permission` (both quote styles).
	permissionQuotedRE = regexp.MustCompile(`["']([^"']+)["']\s+permission\b`)
	// permissionUnquotedRE matches `… does not have permission X …` from server-core.
	permissionUnquotedRE = regexp.MustCompile(`does not have permission\s+(.+?)(?:\s+in\s+project\b|\.\s*$|$)`)
	// projectRE captures a trailing `in project [with internal id]: '<id>'`.
	projectRE = regexp.MustCompile(`in\s+project(?:\s+with\s+internal\s+id)?:\s*['"]?([^'"\s.]+)['"]?`)

	// notFoundRE matches `No <kind> found by locator '…id:X…'`.
	notFoundRE = regexp.MustCompile(`^No (build types?|build|project|user|agent) found by locator '(?:[^']*?id:)?([^,')]+)`)
	// nothingFoundRE matches `Nothing is found by locator 'count:1,<kind>:(id:X)…'`.
	nothingFoundRE = regexp.MustCompile(`Nothing is found by locator '[^']*?(buildType|project|user|agent):?\(?[^']*?id:([^,')]+)`)

	resourceAliases = map[string]string{
		"build types": "job",
		"build type":  "job",
		"buildType":   "job",
		"build":       "run",
	}
)

// ErrorFromResponse reads a non-2xx body (capped at maxErrorBody) and returns the typed error.
func ErrorFromResponse(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	return errorFromBody(resp.StatusCode, body)
}

// errorFromBody is a pure function for ease of testing.
func errorFromBody(status int, body []byte) error {
	w := parseWire(body)
	base := HTTPError{Status: status, Wire: w}

	switch status {
	case http.StatusUnauthorized:
		base.cat = CatAuth
		return &base
	case http.StatusForbidden:
		base.cat = CatPermission
		perm, project := parsePermission(w.Message)
		return &PermissionError{HTTPError: base, Permission: perm, Project: project}
	case http.StatusNotFound:
		base.cat = CatNotFound
		resource, id := parseNotFound(w.Message)
		return &NotFoundError{HTTPError: base, Resource: resource, ID: id}
	default:
		base.cat = CatInternal
		return &base
	}
}

// parseWire tries JSON, then XML, then plain text (<= 1 KiB, non-HTML).
func parseWire(body []byte) Wire {
	var j struct {
		Errors []struct {
			Message           string `json:"message"`
			AdditionalMessage string `json:"additionalMessage"`
			StatusText        string `json:"statusText"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &j); err == nil {
		if len(j.Errors) == 0 {
			return Wire{}
		}
		e := j.Errors[0]
		return Wire{Message: e.Message, Additional: e.AdditionalMessage, StatusText: e.StatusText}
	}

	var x struct {
		XMLName xml.Name `xml:"errors"`
		Errors  []struct {
			Message           string `xml:"message"`
			AdditionalMessage string `xml:"additionalMessage"`
			StatusText        string `xml:"statusText"`
		} `xml:"error"`
	}
	if err := xml.Unmarshal(body, &x); err == nil && len(x.Errors) > 0 {
		e := x.Errors[0]
		return Wire{Message: e.Message, Additional: e.AdditionalMessage, StatusText: e.StatusText}
	}

	t := strings.TrimSpace(string(body))
	if t != "" && len(t) <= 1024 && !strings.HasPrefix(t, "<") {
		return Wire{Message: t}
	}
	return Wire{}
}

// parsePermission extracts the permission description and project id from a 403 message.
func parsePermission(msg string) (permission, project string) {
	if m := permissionQuotedRE.FindStringSubmatch(msg); m != nil {
		permission = m[1]
	} else if m := permissionUnquotedRE.FindStringSubmatch(msg); m != nil {
		permission = strings.TrimSpace(m[1])
	}
	if m := projectRE.FindStringSubmatch(msg); m != nil {
		project = m[1]
	}
	return
}

// parseNotFound extracts the resource kind and id from a 404 locator message.
func parseNotFound(msg string) (resource, id string) {
	if m := notFoundRE.FindStringSubmatch(msg); m != nil {
		return cmp.Or(resourceAliases[m[1]], m[1]), m[2]
	}
	if m := nothingFoundRE.FindStringSubmatch(msg); m != nil {
		return cmp.Or(resourceAliases[m[1]], m[1]), m[2]
	}
	return "", ""
}
