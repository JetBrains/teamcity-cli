package api

import (
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
)

// FieldSpec defines available and default fields for a resource type
type FieldSpec struct {
	Available []string
	Default   []string
}

// ParseFields parses comma-separated fields, validates them, returns defaults if empty
func (fs *FieldSpec) ParseFields(input string) ([]string, error) {
	if input = strings.TrimSpace(input); input == "" {
		return fs.Default, nil
	}
	var fields, invalid []string
	for _, f := range strings.Split(input, ",") {
		if f = strings.TrimSpace(f); f != "" {
			fields = append(fields, f)
			if !slices.Contains(fs.Available, f) {
				invalid = append(invalid, f)
			}
		}
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("unknown fields: %s", strings.Join(invalid, ", "))
	}
	return fields, nil
}

// Help returns help text
func (fs *FieldSpec) Help() string {
	return fmt.Sprintf("Available: %s\n\nDefault: %s",
		strings.Join(fs.Available, ", "), strings.Join(fs.Default, ", "))
}

// ToAPIFields converts dot-notation fields to TeamCity API fields parameter.
// Example: ["id", "buildType.name", "buildType.projectId"] -> "id,buildType(name,projectId)"
func ToAPIFields(fields []string) string {
	groups := make(map[string][]string)
	var topLevel []string

	for _, f := range fields {
		if idx := strings.Index(f, "."); idx > 0 {
			groups[f[:idx]] = append(groups[f[:idx]], f[idx+1:])
		} else {
			topLevel = append(topLevel, f)
		}
	}

	var result []string
	result = append(result, topLevel...)

	var parents []string
	for p := range groups {
		parents = append(parents, p)
	}
	sort.Strings(parents)

	for _, p := range parents {
		result = append(result, fmt.Sprintf("%s(%s)", p, ToAPIFields(groups[p])))
	}

	return strings.Join(result, ",")
}

// ToAPIFieldsEncoded returns URL-encoded TeamCity API fields parameter
func ToAPIFieldsEncoded(fields []string) string {
	return url.QueryEscape(ToAPIFields(fields))
}

// BuildFields field specifications
var BuildFields = FieldSpec{
	Available: []string{
		"id", "number", "status", "state", "href", "webUrl", "branchName", "defaultBranch",
		"buildTypeId", "statusText", "queuedDate", "startDate", "finishDate", "percentageComplete",
		"buildType.id", "buildType.name", "buildType.projectName", "buildType.projectId", "buildType.href", "buildType.webUrl",
		"triggered.type", "triggered.date", "triggered.user.name", "triggered.user.username",
		"agent.id", "agent.name", "agent.href", "agent.webUrl",
	},
	Default: []string{
		"id", "number", "status", "state", "branchName", "buildTypeId",
		"buildType.id", "buildType.name", "buildType.projectName",
		"triggered.type", "triggered.user.name", "triggered.user.username",
		"startDate", "finishDate", "queuedDate", "agent.name",
	},
}

var BuildTypeFields = FieldSpec{
	Available: []string{"id", "name", "projectName", "projectId", "href", "webUrl", "paused"},
	Default:   []string{"id", "name", "projectName", "projectId", "href", "webUrl", "paused"},
}

var ProjectFields = FieldSpec{
	Available: []string{"id", "name", "description", "parentProjectId", "href", "webUrl"},
	Default:   []string{"id", "name", "description", "parentProjectId", "href", "webUrl"},
}

var QueuedBuildFields = FieldSpec{
	Available: []string{
		"id", "buildTypeId", "state", "branchName", "href", "webUrl", "queuedDate",
		"buildType.id", "buildType.name", "buildType.projectName",
		"triggered.type", "triggered.date", "triggered.user.name", "triggered.user.username",
	},
	Default: []string{
		"id", "buildTypeId", "state", "branchName", "href", "webUrl", "queuedDate",
		"buildType.id", "buildType.name", "buildType.projectName",
		"triggered.type", "triggered.user.name", "triggered.user.username",
	},
}

var AgentFields = FieldSpec{
	Available: []string{"id", "name", "typeId", "connected", "enabled", "authorized", "href", "webUrl", "pool.id", "pool.name"},
	Default:   []string{"id", "name", "connected", "enabled", "authorized", "href", "webUrl", "pool.id", "pool.name"},
}
