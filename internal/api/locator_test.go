package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocator(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name  string
		build func() *Locator
		want  string
	}{
		{
			name: "empty locator",
			build: func() *Locator {
				return NewLocator()
			},
			want: "",
		},
		{
			name: "single value",
			build: func() *Locator {
				return NewLocator().Add("project", "MyProject")
			},
			want: "project:MyProject",
		},
		{
			name: "multiple values",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					Add("branch", "main")
			},
			want: "project:MyProject,branch:main",
		},
		{
			name: "skip empty values",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					Add("branch", "").
					Add("status", "success")
			},
			want: "project:MyProject,status:success",
		},
		{
			name: "int values",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					AddInt("count", 10)
			},
			want: "project:MyProject,count:10",
		},
		{
			name: "skip zero int values",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					AddInt("count", 0)
			},
			want: "project:MyProject",
		},
		{
			name: "int with default",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					AddIntDefault("count", 0, 30)
			},
			want: "project:MyProject,count:30",
		},
		{
			name: "int overrides default",
			build: func() *Locator {
				return NewLocator().
					Add("project", "MyProject").
					AddIntDefault("count", 50, 30)
			},
			want: "project:MyProject,count:50",
		},
		{
			name: "uppercase value",
			build: func() *Locator {
				return NewLocator().
					AddUpper("status", "success")
			},
			want: "status:SUCCESS",
		},
		{
			name: "escape colon in value",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "feature:test")
			},
			want: "branch:(feature:test)",
		},
		{
			name: "escape comma in value",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "a,b")
			},
			want: "branch:(a,b)",
		},
		// Edge cases for special characters
		{
			name: "escape parentheses in value",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "feature(test)")
			},
			want: "branch:(feature(test$))",
		},
		{
			name: "multiple special chars",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "a:b,c(d)")
			},
			want: "branch:(a:b,c(d$))",
		},
		{
			name: "unicode characters",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "feature/日本語")
			},
			want: "branch:feature/日本語",
		},
		{
			name: "emoji in value",
			build: func() *Locator {
				return NewLocator().
					Add("branch", "feature/🚀-release")
			},
			want: "branch:feature/🚀-release",
		},
		{
			name: "value with only special chars",
			build: func() *Locator {
				return NewLocator().
					Add("branch", ":,:()")
			},
			want: "branch:(:,:($))",
		},
		{
			name: "injection attempt via closing paren",
			build: func() *Locator {
				return NewLocator().
					Add("project", "Foo),status:FAILURE,tag:(bar")
			},
			want: "project:(Foo$),status:FAILURE,tag:(bar)",
		},
		{
			name: "negative int value is skipped",
			build: func() *Locator {
				return NewLocator().
					AddInt("count", -1)
			},
			want: "", // AddInt skips values <= 0
		},
		{
			name: "zero int value is skipped",
			build: func() *Locator {
				return NewLocator().
					AddInt("count", 0)
			},
			want: "", // AddInt skips values <= 0
		},
		{
			name: "int default with negative value uses default",
			build: func() *Locator {
				return NewLocator().
					AddIntDefault("count", -5, 30)
			},
			want: "count:30", // AddIntDefault uses default when value <= 0
		},
		{
			name: "int default with zero value uses default",
			build: func() *Locator {
				return NewLocator().
					AddIntDefault("count", 0, 30)
			},
			want: "count:30", // AddIntDefault uses default when value <= 0
		},
		{
			name: "whitespace in value",
			build: func() *Locator {
				return NewLocator().
					Add("name", "my project")
			},
			want: "name:my project",
		},
		{
			name: "value starting with special char",
			build: func() *Locator {
				return NewLocator().
					Add("branch", ":main")
			},
			want: "branch:(:main)",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.build().String()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLocatorEncode(T *testing.T) {
	T.Parallel()
	locator := NewLocator().
		Add("buildType", "Project_Build").
		Add("branch", "feature/test")

	got := locator.Encode()
	want := "buildType%3AProject_Build%2Cbranch%3Afeature%2Ftest"

	assert.Equal(T, want, got)
}

func TestLocatorIsEmpty(T *testing.T) {
	T.Parallel()
	tests := []struct {
		name  string
		build func() *Locator
		want  bool
	}{
		{
			name:  "new locator is empty",
			build: func() *Locator { return NewLocator() },
			want:  true,
		},
		{
			name:  "locator with value is not empty",
			build: func() *Locator { return NewLocator().Add("key", "value") },
			want:  false,
		},
		{
			name:  "locator with empty value is empty",
			build: func() *Locator { return NewLocator().Add("key", "") },
			want:  true,
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.build().IsEmpty()
			assert.Equal(t, tc.want, got)
		})
	}
}
