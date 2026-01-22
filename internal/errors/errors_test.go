package errors

import (
	"fmt"
	"strings"
	"testing"
)

func TestUserErrorWithSuggestion(t *testing.T) {
	err := &UserError{
		Message:    "Test error",
		Suggestion: "Try this fix",
	}

	result := err.Error()
	if !strings.Contains(result, "Test error") {
		t.Errorf("Error() should contain message, got %q", result)
	}
	if !strings.Contains(result, "Try this fix") {
		t.Errorf("Error() should contain suggestion, got %q", result)
	}
}

func TestUserErrorWithoutSuggestion(t *testing.T) {
	err := &UserError{
		Message: "Test error",
	}

	result := err.Error()
	if result != "Test error" {
		t.Errorf("Error() = %q, want %q", result, "Test error")
	}
}

func TestNew(t *testing.T) {
	err := New("custom error")
	if err.Message != "custom error" {
		t.Errorf("New().Message = %q, want %q", err.Message, "custom error")
	}
	if err.Suggestion != "" {
		t.Errorf("New().Suggestion should be empty, got %q", err.Suggestion)
	}
}

func TestWithSuggestion(t *testing.T) {
	err := WithSuggestion("error msg", "suggestion text")
	if err.Message != "error msg" {
		t.Errorf("WithSuggestion().Message = %q, want %q", err.Message, "error msg")
	}
	if err.Suggestion != "suggestion text" {
		t.Errorf("WithSuggestion().Suggestion = %q, want %q", err.Suggestion, "suggestion text")
	}
}

func TestNotAuthenticated(t *testing.T) {
	err := NotAuthenticated()
	if !strings.Contains(err.Message, "Not authenticated") {
		t.Errorf("NotAuthenticated().Message should contain 'Not authenticated', got %q", err.Message)
	}
	if !strings.Contains(err.Suggestion, "tc auth login") {
		t.Errorf("NotAuthenticated().Suggestion should mention login, got %q", err.Suggestion)
	}
}

func TestNotFound(t *testing.T) {
	err := NotFound("build", "123")
	if !strings.Contains(err.Message, "build") {
		t.Errorf("NotFound().Message should contain resource type, got %q", err.Message)
	}
	if !strings.Contains(err.Message, "123") {
		t.Errorf("NotFound().Message should contain id, got %q", err.Message)
	}
	if !strings.Contains(err.Message, "not found") {
		t.Errorf("NotFound().Message should contain 'not found', got %q", err.Message)
	}
}

func TestAuthenticationFailed(t *testing.T) {
	err := AuthenticationFailed()
	if !strings.Contains(err.Message, "Authentication failed") {
		t.Errorf("AuthenticationFailed().Message should contain 'Authentication failed', got %q", err.Message)
	}
	if !strings.Contains(err.Suggestion, "tc auth login") {
		t.Errorf("AuthenticationFailed().Suggestion should mention login, got %q", err.Suggestion)
	}
}

func TestPermissionDenied(t *testing.T) {
	err := PermissionDenied("delete build")
	if !strings.Contains(err.Message, "Permission denied") {
		t.Errorf("PermissionDenied().Message should contain 'Permission denied', got %q", err.Message)
	}
	if !strings.Contains(err.Message, "delete build") {
		t.Errorf("PermissionDenied().Message should contain action, got %q", err.Message)
	}
}

func TestNetworkError(t *testing.T) {
	err := NetworkError("https://tc.example.com", nil)
	if !strings.Contains(err.Message, "Cannot connect") {
		t.Errorf("NetworkError().Message should contain 'Cannot connect', got %q", err.Message)
	}
	if !strings.Contains(err.Message, "tc.example.com") {
		t.Errorf("NetworkError().Message should contain server URL, got %q", err.Message)
	}

	// Test with cause
	cause := fmt.Errorf("connection refused")
	errWithCause := NetworkError("https://tc.example.com", cause)
	if !strings.Contains(errWithCause.Message, "connection refused") {
		t.Errorf("NetworkError().Message should contain cause, got %q", errWithCause.Message)
	}
}

func TestRequiredFlag(t *testing.T) {
	err := RequiredFlag("project")
	if !strings.Contains(err.Message, "--project") {
		t.Errorf("RequiredFlag().Message should contain flag name, got %q", err.Message)
	}
	if !strings.Contains(err.Message, "required") {
		t.Errorf("RequiredFlag().Message should contain 'required', got %q", err.Message)
	}
}

func TestUserErrorImplementsError(t *testing.T) {
	var err error = New("test")
	if err == nil {
		t.Error("UserError should implement error interface")
	}
}

func TestMutuallyExclusive(t *testing.T) {
	err := MutuallyExclusive("BUILD_ID", "latest")
	if !strings.Contains(err.Message, "BUILD_ID") {
		t.Errorf("MutuallyExclusive().Message should contain arg name, got %q", err.Message)
	}
	if !strings.Contains(err.Message, "latest") {
		t.Errorf("MutuallyExclusive().Message should contain flag name, got %q", err.Message)
	}
	if !strings.Contains(err.Message, "cannot specify both") {
		t.Errorf("MutuallyExclusive().Message should explain the conflict, got %q", err.Message)
	}
	if !strings.Contains(err.Suggestion, "BUILD_ID") && !strings.Contains(err.Suggestion, "latest") {
		t.Errorf("MutuallyExclusive().Suggestion should mention alternatives, got %q", err.Suggestion)
	}
}

func TestMutuallyExclusiveEmptyStrings(t *testing.T) {
	// Edge case: empty strings should still produce valid error
	err := MutuallyExclusive("", "")
	if err.Message == "" {
		t.Error("MutuallyExclusive with empty strings should still produce a message")
	}
}

func TestNotFoundEdgeCases(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			err := NotFound(tc.resource, tc.id)
			if err == nil {
				t.Fatal("NotFound should return an error")
			}
			// Should not panic and should produce valid error
			_ = err.Error()
		})
	}
}

func TestNetworkErrorWithVariousCauses(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			err := NetworkError(tc.serverURL, tc.cause)
			if err == nil {
				t.Fatal("NetworkError should return an error")
			}
			errorStr := err.Error()
			if tc.cause != nil && !strings.Contains(errorStr, tc.cause.Error()) {
				t.Errorf("Error message should contain cause: %s", errorStr)
			}
		})
	}
}

func TestRequiredFlagEdgeCases(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			err := RequiredFlag(tc.flag)
			if err == nil {
				t.Fatal("RequiredFlag should return an error")
			}
			_ = err.Error() // Should not panic
		})
	}
}

func TestPermissionDeniedEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		action string
	}{
		{"normal action", "delete build"},
		{"empty action", ""},
		{"long action", "perform administrative operations on the server"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := PermissionDenied(tc.action)
			if err == nil {
				t.Fatal("PermissionDenied should return an error")
			}
			if tc.action != "" && !strings.Contains(err.Message, tc.action) {
				t.Errorf("Message should contain action: %s", err.Message)
			}
		})
	}
}
