package errors

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserError(T *testing.T) {
	T.Parallel()
	T.Run("with suggestion", func(t *testing.T) {
		t.Parallel()
		err := &UserError{
			Message:    "Test error",
			Suggestion: "Try this fix",
		}
		got := err.Error()
		assert.Contains(t, got, "Test error")
		assert.Contains(t, got, "Try this fix")
	})
	T.Run("without suggestion", func(t *testing.T) {
		t.Parallel()
		err := &UserError{Message: "Test error"}
		assert.Equal(t, "Test error", err.Error())
	})
	T.Run("implements error interface", func(t *testing.T) {
		t.Parallel()

		var err error = New("test")
		assert.NotNil(t, err, "UserError should implement error interface")
	})
}

func TestNew(T *testing.T) {
	T.Parallel()
	err := New("custom error")
	assert.Equal(T, "custom error", err.Message)
	assert.Empty(T, err.Suggestion)
}

func TestWithSuggestion(T *testing.T) {
	T.Parallel()
	err := WithSuggestion("error msg", "suggestion text")
	assert.Equal(T, "error msg", err.Message)
	assert.Equal(T, "suggestion text", err.Suggestion)
}

func TestNotAuthenticated(T *testing.T) {
	T.Parallel()
	err := NotAuthenticated()
	assert.Contains(T, err.Message, "Not authenticated")
	assert.Contains(T, err.Suggestion, "teamcity auth login")
	assert.NotContains(T, err.Suggestion, "TEAMCITY_GUEST")
}

func TestNotFound(T *testing.T) {
	T.Parallel()
	err := NotFound("build", "123")
	assert.Contains(T, err.Message, "build")
	assert.Contains(T, err.Message, "123")
	assert.Contains(T, err.Message, "not found")
}

func TestAuthenticationFailed(T *testing.T) {
	T.Parallel()
	err := AuthenticationFailed()
	assert.Contains(T, err.Message, "Authentication failed")
	assert.Contains(T, err.Suggestion, "teamcity auth login")
}

func TestPermissionDenied(T *testing.T) {
	T.Parallel()
	err := PermissionDenied("delete build")
	assert.Contains(T, err.Message, "Permission denied")
	assert.Contains(T, err.Message, "delete build")
}

func TestNetworkError(T *testing.T) {
	T.Parallel()

	T.Run("without cause", func(t *testing.T) {
		t.Parallel()

		err := NetworkError("https://tc.example.com", nil)
		assert.Contains(t, err.Message, "Cannot connect")
		assert.Contains(t, err.Message, "tc.example.com")
	})

	T.Run("with cause", func(t *testing.T) {
		t.Parallel()

		cause := fmt.Errorf("connection refused")
		err := NetworkError("https://tc.example.com", cause)
		assert.Contains(t, err.Message, "connection refused")
	})
}

func TestRequiredFlag(T *testing.T) {
	T.Parallel()

	err := RequiredFlag("project")
	assert.Contains(T, err.Message, "--project")
	assert.Contains(T, err.Message, "required")
}

func TestMutuallyExclusive(T *testing.T) {
	T.Parallel()

	T.Run("basic", func(t *testing.T) {
		t.Parallel()

		err := MutuallyExclusive("BUILD_ID", "latest")
		assert.Contains(t, err.Message, "BUILD_ID")
		assert.Contains(t, err.Message, "latest")
		assert.Contains(t, err.Message, "cannot specify both")
		assert.True(t, strings.Contains(err.Suggestion, "BUILD_ID") || strings.Contains(err.Suggestion, "latest"),
			"Suggestion should contain alternatives")
	})

	T.Run("empty strings", func(t *testing.T) {
		t.Parallel()

		err := MutuallyExclusive("", "")
		assert.NotEmpty(t, err.Message)
	})
}

func TestNotFoundEdgeCases(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name     string
		resource string
		id       string
	}{
		{"empty resource", "", "123"},
		{"empty id", "build", ""},
		{"both empty", "", ""},
		{"special chars in id", "build", "#123"},
		{"unicode in id", "build", "日本語"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := NotFound(tc.resource, tc.id)
			require.NotNil(t, err)
			// Should not panic and should produce valid error
			_ = err.Error()
		})
	}
}

func TestNetworkErrorEdgeCases(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name      string
		serverURL string
		cause     error
	}{
		{"nil cause", "https://tc.example.com", nil},
		{"with cause", "https://tc.example.com", fmt.Errorf("connection refused")},
		{"empty URL", "", fmt.Errorf("invalid URL")},
		{"URL with port", "http://localhost:8111", nil},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := NetworkError(tc.serverURL, tc.cause)
			require.NotNil(t, err)
			got := err.Error()
			if tc.cause != nil {
				assert.Contains(t, got, tc.cause.Error())
			}
		})
	}
}

func TestRequiredFlagEdgeCases(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name string
		flag string
	}{
		{"normal flag", "project"},
		{"empty flag", ""},
		{"flag with hyphen", "build-type"},
		{"flag with special chars", "flag:name"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := RequiredFlag(tc.flag)
			require.NotNil(t, err)
			// Should not panic
			_ = err.Error()
		})
	}
}

func TestPermissionDeniedEdgeCases(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name   string
		action string
	}{
		{"normal action", "delete build"},
		{"empty action", ""},
		{"long action", "perform administrative operations on the server"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := PermissionDenied(tc.action)
			require.NotNil(t, err)
			if tc.action != "" {
				assert.Contains(t, err.Message, tc.action)
			}
		})
	}
}
