package api

import (
	"strings"
	"testing"
)

func TestToAPIFields(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
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
		t.Run(tc.name, func(t *testing.T) {
			if got := ToAPIFields(tc.input); got != tc.expected {
				t.Errorf("ToAPIFields(%v) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestToAPIFieldsEncoded(t *testing.T) {
	got := ToAPIFieldsEncoded([]string{"id", "buildType.name"})
	want := "id%2CbuildType%28name%29"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFieldSpec_ParseFields(t *testing.T) {
	spec := FieldSpec{Available: []string{"id", "name", "status"}, Default: []string{"id", "name"}}

	tests := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{"", []string{"id", "name"}, false},
		{"   ", []string{"id", "name"}, false},
		{"status", []string{"status"}, false},
		{"id,status", []string{"id", "status"}, false},
		{" id , status ", []string{"id", "status"}, false},
		{"invalid", nil, true},
		{"id,invalid", nil, true},
	}

	for _, tc := range tests {
		got, err := spec.ParseFields(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseFields(%q) expected error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseFields(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("ParseFields(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestFieldSpec_Help(t *testing.T) {
	spec := FieldSpec{Available: []string{"id", "name", "status"}, Default: []string{"id", "name"}}
	help := spec.Help()
	if !strings.Contains(help, "id, name, status") || !strings.Contains(help, "Default") {
		t.Errorf("Help() = %q, missing expected content", help)
	}
}

func TestPredefinedFieldSpecs(t *testing.T) {
	specs := map[string]FieldSpec{
		"BuildFields":       BuildFields,
		"BuildTypeFields":   BuildTypeFields,
		"ProjectFields":     ProjectFields,
		"QueuedBuildFields": QueuedBuildFields,
		"AgentFields":       AgentFields,
	}

	for name, spec := range specs {
		if len(spec.Available) == 0 || len(spec.Default) == 0 {
			t.Errorf("%s: empty fields", name)
		}
		// Verify defaults are subset of available
		for _, d := range spec.Default {
			found := false
			for _, a := range spec.Available {
				if a == d {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: default %q not in available", name, d)
			}
		}
	}
}
