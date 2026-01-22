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
		{
			name: "escape colon in value",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "feature:test")
			},
			expected: "branch:(feature:test)",
		},
		{
			name: "escape comma in value",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "a,b")
			},
			expected: "branch:(a,b)",
		},
		// Edge cases for special characters
		{
			name: "escape parentheses in value",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "feature(test)")
			},
			expected: "branch:(feature(test))",
		},
		{
			name: "multiple special chars",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "a:b,c(d)")
			},
			expected: "branch:(a:b,c(d))",
		},
		{
			name: "unicode characters",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "feature/日本語")
			},
			expected: "branch:feature/日本語",
		},
		{
			name: "emoji in value",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "feature/🚀-release")
			},
			expected: "branch:feature/🚀-release",
		},
		{
			name: "value with only special chars",
			build: func() *Locator {
				return NewLocator().
					Add("branch", ":,:()")
			},
			expected: "branch:(:,:())",
		},
		{
			name: "negative int value is skipped",
			build: func() *Locator {
				return NewLocator().
					AddInt("count", -1)
			},
			expected: "", // AddInt skips values <= 0
		},
		{
			name: "zero int value is skipped",
			build: func() *Locator {
				return NewLocator().
					AddInt("count", 0)
			},
			expected: "", // AddInt skips values <= 0
		},
		{
			name: "int default with negative value uses default",
			build: func() *Locator {
				return NewLocator().
					AddIntDefault("count", -5, 30)
			},
			expected: "count:30", // AddIntDefault uses default when value <= 0
		},
		{
			name: "int default with zero value uses default",
			build: func() *Locator {
				return NewLocator().
					AddIntDefault("count", 0, 30)
			},
			expected: "count:30", // AddIntDefault uses default when value <= 0
		},
		{
			name: "whitespace in value",
			build: func() *Locator {
				return NewLocator().
					Add("name", "my project")
			},
			expected: "name:my project",
		},
		{
			name: "value starting with special char",
			build: func() *Locator {
				return NewLocator().
					Add("branch", ":main")
			},
			expected: "branch:(:main)",
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
