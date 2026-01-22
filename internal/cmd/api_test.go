package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/tiulpin/teamcity-cli/internal/config"
)

// createTestRootCmd creates a fresh root command with the api subcommand for testing
func createTestRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use: "tc",
	}
	rootCmd.PersistentFlags().Bool("no-color", false, "")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "")
	rootCmd.PersistentFlags().Bool("verbose", false, "")
	rootCmd.PersistentFlags().Bool("no-input", false, "")
	rootCmd.AddCommand(newAPICmd())
	return rootCmd
}

func setupMockServerForAPI(handler http.HandlerFunc) (*httptest.Server, func()) {
	server := httptest.NewServer(handler)

	// Save and override config
	originalURL := os.Getenv("TEAMCITY_URL")
	originalToken := os.Getenv("TEAMCITY_TOKEN")

	os.Setenv("TEAMCITY_URL", server.URL)
	os.Setenv("TEAMCITY_TOKEN", "test-token")
	config.Init()

	cleanup := func() {
		server.Close()
		os.Setenv("TEAMCITY_URL", originalURL)
		os.Setenv("TEAMCITY_TOKEN", originalToken)
		config.Init()
	}

	return server, cleanup
}

func TestAPICommandBasicGET(t *testing.T) {
	requestReceived := false
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/app/rest/server" {
			t.Errorf("Expected /app/rest/server, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("Expected Bearer auth header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"version": "2024.03"})
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !requestReceived {
		t.Error("Expected request to be sent to server")
	}
}

func TestAPICommandPOSTWithFields(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["buildType"] != "MyBuild" {
			t.Errorf("Expected buildType MyBuild, got %v", body["buildType"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int{"id": 123})
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/buildQueue", "-X", "POST", "-f", "buildType=MyBuild"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandWithCustomHeaders(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/xml" {
			t.Errorf("Expected Accept application/xml, got %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("X-Custom") != "custom-value" {
			t.Errorf("Expected X-Custom header, got %s", r.Header.Get("X-Custom"))
		}
		w.Write([]byte("<server/>"))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "-H", "Accept: application/xml", "-H", "X-Custom: custom-value"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandIncludeHeaders(t *testing.T) {
	requestReceived := false
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.Header().Set("X-Response-Header", "test-value")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--include"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !requestReceived {
		t.Error("Expected request to be sent to server")
	}
	// Note: output includes headers is printed to stdout, not captured in buffer
}

func TestAPICommandSilentMode(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--silent"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Silent mode should produce no output on success
	if out.String() != "" {
		t.Errorf("Expected no output in silent mode, got: %s", out.String())
	}
}

func TestAPICommandRawOutput(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"compact":true}`))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "--raw"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Raw mode should not pretty-print (no indentation)
	output := out.String()
	if strings.Contains(output, "  \"compact\"") {
		t.Errorf("Expected compact output in raw mode, got: %s", output)
	}
}

func TestAPICommandErrorResponse(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Resource not found"))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds/id:999"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected error to mention 404, got: %v", err)
	}
}

func TestAPICommandDELETE(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds/id:123", "-X", "DELETE"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandPUT(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated":true}`))
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/projects/id:Test", "-X", "PUT", "-f", "name=Updated"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandInvalidHeaderFormat(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/server", "-H", "InvalidHeader"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error for invalid header format")
	}
	if !strings.Contains(err.Error(), "invalid header format") {
		t.Errorf("Expected 'invalid header format' error, got: %v", err)
	}
}

func TestAPICommandInvalidFieldFormat(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "-X", "POST", "-f", "invalid"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("Expected error for invalid field format")
	}
	if !strings.Contains(err.Error(), "invalid field format") {
		t.Errorf("Expected 'invalid field format' error, got: %v", err)
	}
}

func TestAPICommandWithJSONField(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// Check that nested JSON was parsed correctly
		buildType, ok := body["buildType"].(map[string]interface{})
		if !ok {
			t.Errorf("Expected buildType to be object, got %T", body["buildType"])
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if buildType["id"] != "MyBuild" {
			t.Errorf("Expected buildType.id MyBuild, got %v", buildType["id"])
		}

		w.WriteHeader(http.StatusCreated)
	})
	defer cleanup()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/buildQueue", "-X", "POST", "-f", `buildType={"id":"MyBuild"}`})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}

func TestAPICommandFromStdin(t *testing.T) {
	_, cleanup := setupMockServerForAPI(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"test":"stdin"}` {
			t.Errorf("Unexpected body: %s", string(body))
		}
		w.WriteHeader(http.StatusCreated)
	})
	defer cleanup()

	// Save original stdin
	oldStdin := os.Stdin

	// Create a pipe for stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.Write([]byte(`{"test":"stdin"}`))
	w.Close()

	var out bytes.Buffer
	rootCmd := createTestRootCmd()
	rootCmd.SetArgs([]string{"api", "/app/rest/builds", "-X", "POST", "--input", "-"})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)

	err := rootCmd.Execute()

	// Restore stdin
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}
}
