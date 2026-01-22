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
