package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToAPIFields(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"id"}, "id"},
		{"multiple", []string{"id", "name", "status"}, "id,name,status"},
		{"nested single", []string{"buildType.name"}, "buildType(name)"},
		{"nested same parent", []string{"buildType.name", "buildType.projectId"}, "buildType(name,projectId)"},
		{"mixed", []string{"id", "status", "buildType.name"}, "id,status,buildType(name)"},
		{"deeply nested", []string{"triggered.user.name", "triggered.user.username"}, "triggered(user(name,username))"},
		{"three levels", []string{"a.b.c"}, "a(b(c))"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ToAPIFields(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestToAPIFieldsEncoded(T *testing.T) {
	T.Parallel()
	got := ToAPIFieldsEncoded([]string{"id", "buildType.name"})
	want := "id%2CbuildType%28name%29"

	assert.Equal(T, want, got)
}

func TestFieldSpec_ParseFields(T *testing.T) {
	T.Parallel()
	spec := FieldSpec{Available: []string{"id", "name", "status"}, Default: []string{"id", "name"}}

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"empty returns default", "", []string{"id", "name"}, false},
		{"whitespace returns default", "   ", []string{"id", "name"}, false},
		{"single field", "status", []string{"status"}, false},
		{"multiple fields", "id,status", []string{"id", "status"}, false},
		{"fields with whitespace", " id , status ", []string{"id", "status"}, false},
		{"invalid field", "invalid", nil, true},
		{"valid and invalid mixed", "id,invalid", nil, true},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := spec.ParseFields(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, len(tc.want), len(got))
		})
	}
}

func TestFieldSpec_Help(T *testing.T) {
	T.Parallel()
	spec := FieldSpec{Available: []string{"id", "name", "status"}, Default: []string{"id", "name"}}
	got := spec.Help()

	assert.True(T, strings.Contains(got, "id, name, status"), "Help() should contain available fields")
	assert.True(T, strings.Contains(got, "Default"), "Help() should contain 'Default' section")
}

func TestPredefinedFieldSpecs(T *testing.T) {
	T.Parallel()
	specs := map[string]FieldSpec{
		"BuildFields":       BuildFields,
		"BuildTypeFields":   BuildTypeFields,
		"ProjectFields":     ProjectFields,
		"QueuedBuildFields": QueuedBuildFields,
		"AgentFields":       AgentFields,
	}

	for name, spec := range specs {
		T.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, spec.Available, "%s.Available should not be empty", name)
			assert.NotEmpty(t, spec.Default, "%s.Default should not be empty", name)

			availableSet := make(map[string]bool)
			for _, a := range spec.Available {
				availableSet[a] = true
			}

			for _, d := range spec.Default {
				assert.True(t, availableSet[d], "%s: default field %q not in Available", name, d)
			}
		})
	}
}
