//go:build acceptance

// Package acceptance contains end-to-end blackbox tests for the teamcity CLI binary.
//
// Tests are written as txtar scripts and executed via the testscript framework,
// following the same approach used by the GitHub CLI (cli/cli).
//
// Two execution modes:
//
//  1. Source mode (default): the CLI is compiled from source and run in-process
//     via testscript.RunMain.
//  2. Binary mode (TC_ACCEPTANCE_BINARY set): a pre-built binary (e.g. goreleaser
//     snapshot) is placed on PATH and used directly.
//
// Required environment variables: none (defaults to guest auth on cli.teamcity.com).
//
// Optional environment variables:
//
//	TC_ACCEPTANCE_HOST   — TeamCity server URL (default: https://cli.teamcity.com)
//	TC_ACCEPTANCE_TOKEN  — API token for authenticated tests (skipped if unset)
//	TC_ACCEPTANCE_BINARY — path to pre-built binary (e.g. goreleaser snapshot)
//	TC_ACCEPTANCE_SCRIPT — run only scripts matching this substring
package acceptance

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/rogpeppe/go-internal/testscript"
)

const defaultHost = "https://cli.teamcity.com"

func TestMain(m *testing.M) {
	cmds := map[string]func() int{}

	// When TC_ACCEPTANCE_BINARY is set, don't register the in-process command.
	// testscript will find the external binary on PATH instead.
	if os.Getenv("TC_ACCEPTANCE_BINARY") == "" {
		cmds["teamcity"] = teamcityMain
	}

	os.Exit(testscript.RunMain(m, cmds))
}

// teamcityMain is the in-process entry point for the teamcity CLI.
// It mirrors tc/main.go so that testscript can run the CLI without
// an external binary.
func teamcityMain() int {
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		return 1
	}
	if err := cmd.Execute(); err != nil {
		var exitErr *cmd.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.Code
		}
		return 1
	}
	return 0
}

func TestAcceptance(t *testing.T) {
	host := envOr("TC_ACCEPTANCE_HOST", defaultHost)
	token := os.Getenv("TC_ACCEPTANCE_TOKEN")
	scriptFilter := os.Getenv("TC_ACCEPTANCE_SCRIPT")

	dirs := []string{
		"help",
		"auth",
		"config",
		"project",
		"run",
		"job",
		"agent",
		"pool",
		"api",
		"queue",
		"alias",
		"skill",
	}

	for _, dir := range dirs {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			runScripts(t, dir, host, token, scriptFilter)
		})
	}
}

func runScripts(t *testing.T, dir, host, token, scriptFilter string) {
	t.Helper()

	dataDir := filepath.Join("testdata", dir)
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Skipf("no test data in %s: %v", dataDir, err)
		return
	}

	hasScripts := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".txtar") {
			hasScripts = true
			break
		}
	}
	if !hasScripts {
		t.Skipf("no .txtar files in %s", dataDir)
		return
	}

	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		t.Fatal(err)
	}
	randomString := hex.EncodeToString(randomBytes)

	params := testscript.Params{
		Dir: dataDir,
		Setup: func(env *testscript.Env) error {
			scriptName := filepath.Base(env.WorkDir)

			env.Setenv("HOME", env.WorkDir)
			env.Setenv("USERPROFILE", env.WorkDir) // Windows: os.UserHomeDir uses USERPROFILE, not HOME
			env.Setenv("TC_HOST", host)
			env.Setenv("TC_INSECURE_SKIP_WARN", "1")
			env.Setenv("TEAMCITY_URL", host)
			env.Setenv("NO_COLOR", "1")

			if token != "" {
				env.Setenv("TEAMCITY_TOKEN", token)
				env.Setenv("TC_HAS_TOKEN", "1")
			} else {
				env.Setenv("TEAMCITY_GUEST", "1")
				env.Setenv("TC_HAS_TOKEN", "0")
			}

			env.Setenv("RANDOM_STRING", randomString)
			env.Setenv("SCRIPT_NAME", scriptName)

			// If a pre-built binary is specified, put its directory on PATH
			if bin := os.Getenv("TC_ACCEPTANCE_BINARY"); bin != "" {
				absBin, err := filepath.Abs(bin)
				if err != nil {
					return fmt.Errorf("resolve TC_ACCEPTANCE_BINARY: %w", err)
				}
				binDir := filepath.Dir(absBin)
				env.Setenv("PATH", binDir+string(os.PathListSeparator)+env.Getenv("PATH"))
			}

			return nil
		},
		Cmds: customCommands(),
		Condition: func(cond string) (bool, error) {
			switch cond {
			case "has_token":
				return token != "", nil
			case "guest":
				return token == "", nil
			default:
				return false, fmt.Errorf("unknown condition %q", cond)
			}
		},
	}

	if scriptFilter != "" {
		origDir := params.Dir
		params.Dir = t.TempDir()

		entries, err := os.ReadDir(origDir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".txtar") {
				continue
			}
			if !strings.Contains(e.Name(), scriptFilter) {
				continue
			}
			src := filepath.Join(origDir, e.Name())
			dst := filepath.Join(params.Dir, e.Name())
			data, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	testscript.Run(t, params)
}

// customCommands returns the custom testscript commands available in test scripts.
func customCommands() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	return map[string]func(ts *testscript.TestScript, neg bool, args []string){
		"stdout2env":     cmdStdout2Env,
		"env2upper":      cmdEnv2Upper,
		"sleep":          cmdSleep,
		"extract":        cmdExtract,
		"wait_for_agent": cmdWaitForAgent,
	}
}

// cmdExtract applies a regex to the previous command's stdout and stores the
// first capture group in an environment variable.
// Usage: extract <regex> <VAR_NAME>
// Example: extract '"id":(\d+)' BUILD_ID
// Example: extract '"id":"([^"]+)"' JOB_ID
func cmdExtract(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("extract does not support negation")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: extract <regex> <VAR_NAME>")
	}
	pattern, varName := args[0], args[1]
	re, err := regexp.Compile(pattern)
	if err != nil {
		ts.Fatalf("invalid regex %q: %v", pattern, err)
	}
	stdout := ts.ReadFile("stdout")
	m := re.FindStringSubmatch(stdout)
	if m == nil || len(m) < 2 {
		ts.Fatalf("pattern %q did not match stdout:\n%s", pattern, stdout)
	}
	ts.Setenv(varName, strings.TrimSpace(m[1]))
}

// cmdStdout2Env captures the previous command's stdout into an environment variable.
// Usage: stdout2env <VAR_NAME>
func cmdStdout2Env(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("stdout2env does not support negation")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: stdout2env <VAR_NAME>")
	}
	stdout := ts.ReadFile("stdout")
	ts.Setenv(args[0], strings.TrimSpace(stdout))
}

// cmdEnv2Upper converts an environment variable value to uppercase.
// Usage: env2upper <VAR_NAME>
func cmdEnv2Upper(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("env2upper does not support negation")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: env2upper <VAR_NAME>")
	}
	ts.Setenv(args[0], strings.ToUpper(ts.Getenv(args[0])))
}

// cmdSleep pauses execution for the given duration.
// Usage: sleep <duration>
// Example: sleep 2s
func cmdSleep(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("sleep does not support negation")
	}
	if len(args) != 1 {
		ts.Fatalf("usage: sleep <duration>")
	}
	d, err := time.ParseDuration(args[0])
	if err != nil {
		ts.Fatalf("invalid duration %q: %v", args[0], err)
	}
	time.Sleep(d)
}

// cmdWaitForAgent polls a build until an agent is assigned and stores the agent ID.
// Usage: wait_for_agent <BUILD_ID> <AGENT_VAR>
// Polls every 5s for up to 120s.
func cmdWaitForAgent(ts *testscript.TestScript, neg bool, args []string) {
	if neg {
		ts.Fatalf("wait_for_agent does not support negation")
	}
	if len(args) != 2 {
		ts.Fatalf("usage: wait_for_agent <BUILD_ID> <AGENT_VAR>")
	}
	buildID, varName := args[0], args[1]

	host := ts.Getenv("TEAMCITY_URL")
	token := ts.Getenv("TEAMCITY_TOKEN")
	guest := ts.Getenv("TEAMCITY_GUEST")

	url := fmt.Sprintf("%s/app/rest/builds/id:%s?fields=agent(id,name)", host, buildID)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			ts.Fatalf("create request: %v", err)
		}
		req.Header.Set("Accept", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		} else if guest == "1" {
			q := req.URL.Query()
			q.Set("guest", "1")
			req.URL.RawQuery = q.Encode()
		}

		resp, err := client.Do(req)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Agent *struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"agent"`
		}
		if err := json.Unmarshal(body, &result); err == nil && result.Agent != nil && result.Agent.ID > 0 {
			ts.Setenv(varName, fmt.Sprintf("%d", result.Agent.ID))
			ts.Logf("agent found: id=%d name=%s", result.Agent.ID, result.Agent.Name)
			return
		}

		time.Sleep(5 * time.Second)
	}
	ts.Fatalf("timed out waiting for agent on build %s", buildID)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
