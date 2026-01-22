package api

import "testing"

func TestLocator(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *Locator
		expected string
	}{
		{
			name: "empty locator",
			build: func() *Locator {
				return NewLocator()
			},
			expected: "",
		},
		{
			name: "single value",
			build: func() *Locator {
				return NewLocator().Add("project", "MyProject")
			},
			expected: "project:MyProject",
		},
		{
			name: "multiple values",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					Add("branch", "main")
			},
			expected: "project:MyProject,branch:main",
		},
		{
			name: "skip empty values",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					Add("branch", "").
					Add("status", "success")
			},
			expected: "project:MyProject,status:success",
		},
		{
			name: "int values",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					AddInt("count", 10)
			},
			expected: "project:MyProject,count:10",
		},
		{
			name: "skip zero int values",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					AddInt("count", 0)
			},
			expected: "project:MyProject",
		},
		{
			name: "int with default",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					AddIntDefault("count", 0, 30)
			},
			expected: "project:MyProject,count:30",
		},
		{
			name: "int overrides default",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					AddIntDefault("count", 50, 30)
			},
			expected: "project:MyProject,count:50",
		},
		{
			name: "uppercase value",
			build: func() *Locator {
				return NewLocator().
					AddUpper("status", "success")
			},
			expected: "status:SUCCESS",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.build().String()
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestLocatorEncode(t *testing.T) {
	locator := NewLocator().
		Add("buildType", "Project_Build").
		Add("branch", "feature/test")

	encoded := locator.Encode()
	expected := "buildType%3AProject_Build%2Cbranch%3Afeature%2Ftest"

	if encoded != expected {
		t.Errorf("got %q, want %q", encoded, expected)
	}
}

func TestLocatorIsEmpty(t *testing.T) {
	empty := NewLocator()
	if !empty.IsEmpty() {
		t.Error("new locator should be empty")
	}

	notEmpty := NewLocator().Add("key", "value")
	if notEmpty.IsEmpty() {
		t.Error("locator with values should not be empty")
	}

	stillEmpty := NewLocator().Add("key", "")
	if !stillEmpty.IsEmpty() {
		t.Error("locator with only empty values should be empty")
	}
}
