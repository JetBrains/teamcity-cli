//go:build ignore

// Setup local TeamCity for integration testing.
// Usage: go run scripts/setup-local-teamcity.go

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	baseURL           = "http://localhost:8111"
	testProjectID     = "Sandbox"
	testBuildConfigID = "Sandbox_Demo"
)

// main initializes and configures the TeamCity server, including setting up projects, build configurations, and API tokens.
func main() {
	fmt.Println("Starting TeamCity containers...")
	run("docker", "compose", "up", "-d")

	// Wait for server
	fmt.Println("Waiting for server...")
	waitFor(func() bool {
		resp, err := http.Get(baseURL + "/app/rest/server")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == 200 || resp.StatusCode == 401
	}, 10*time.Minute)

	token := getSuperuserToken()
	fmt.Printf("Got superuser token: %s...\n", token[:8])

	if !exists("/app/rest/projects/id:"+testProjectID, token) {
		fmt.Println("Creating test project...")
		post("/app/rest/projects", token, `{"id":"Sandbox","name":"Sandbox"}`)
	}

	if !exists("/app/rest/buildTypes/id:"+testBuildConfigID, token) {
		fmt.Println("Creating test build config...")
		post("/app/rest/projects/id:Sandbox/buildTypes", token, `{"id":"Sandbox_Demo","name":"Demo"}`)
		post("/app/rest/buildTypes/id:Sandbox_Demo/steps", token, `{
			"name":"Test","type":"simpleRunner",
			"properties":{"property":[
				{"name":"script.content","value":"echo Hello"},
				{"name":"use.custom.script","value":"true"}
			]}
		}`)
	}

	fmt.Println("Creating API token...")
	apiToken := createAPIToken(token)

	env := fmt.Sprintf("TEAMCITY_URL=%s\nTEAMCITY_TOKEN=%s\nTEAMCITY_TEST_CONFIG=%s\nTEAMCITY_TEST_PROJECT=%s\n",
		baseURL, apiToken, testBuildConfigID, testProjectID)
	os.WriteFile(".env", []byte(env), 0644)
	fmt.Println("Wrote .env")

	fmt.Println("Checking for build agent...")
	if agentID := getConnectedAgent(token); agentID > 0 {
		authorizeAgent(agentID, token)
		fmt.Println("Agent authorized")
		fmt.Println("Triggering initial build...")
		if buildID := triggerBuild(apiToken); buildID > 0 {
			fmt.Printf("Triggered build #%d, waiting for completion...\n", buildID)
			if waitForBuild(buildID, apiToken) {
				fmt.Println("Initial build completed")
			} else {
				fmt.Println("Build did not complete in time (tests will trigger their own)")
			}
		}
	} else {
		fmt.Println("No agent connected yet (tests will trigger builds when agent is ready)")
	}

	fmt.Println("\nDone! Run 'just test' to test.")
}

func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)
		os.Exit(1)
	}
}

func waitFor(check func() bool, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(3 * time.Second)
	}
	fmt.Fprintln(os.Stderr, "Timeout waiting for server")
	os.Exit(1)
}

func getSuperuserToken() string {
	out, _ := exec.Command("docker", "compose", "logs", "teamcity-server").Output()
	re := regexp.MustCompile(`Super user authentication token: (\d+)`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	if len(matches) == 0 {
		fmt.Fprintln(os.Stderr, "Could not find superuser token in logs")
		os.Exit(1)
	}
	return matches[len(matches)-1][1]
}

func exists(path, token string) bool {
	req, _ := http.NewRequest("GET", baseURL+path, nil)
	req.SetBasicAuth("", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func post(path, token, body string) {
	req, _ := http.NewRequest("POST", baseURL+path, strings.NewReader(body))
	req.SetBasicAuth("", token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "POST %s failed: %v\n", path, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "POST %s returned %d: %s\n", path, resp.StatusCode, body)
	}
}

func getConnectedAgent(token string) int {
	req, _ := http.NewRequest("GET", baseURL+"/app/rest/agents?locator=authorized:any", nil)
	req.SetBasicAuth("", token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var result struct {
		Agent []struct {
			ID int `json:"id"`
		} `json:"agent"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Agent) > 0 {
		return result.Agent[0].ID
	}
	return 0
}

func authorizeAgent(id int, token string) {
	req, _ := http.NewRequest("PUT", fmt.Sprintf("%s/app/rest/agents/id:%d/authorized", baseURL, id), strings.NewReader("true"))
	req.SetBasicAuth("", token)
	req.Header.Set("Content-Type", "text/plain")
	resp, _ := http.DefaultClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}
}

func triggerBuild(apiToken string) int {
	req, _ := http.NewRequest("POST", baseURL+"/app/rest/buildQueue", strings.NewReader(
		fmt.Sprintf(`{"buildType":{"id":"%s"},"comment":{"text":"Setup script - initial build"}}`, testBuildConfigID)))
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to trigger build: %v\n", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Failed to trigger build (status %d): %s\n", resp.StatusCode, body)
		return 0
	}

	var result struct {
		ID int `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.ID
}

func waitForBuild(buildID int, apiToken string) bool {
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/app/rest/builds/id:%d", baseURL, buildID), nil)
		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Accept", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		var result struct {
			State  string `json:"state"`
			Status string `json:"status"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if result.State == "finished" {
			fmt.Printf("Build finished with status: %s\n", result.Status)
			return true
		}

		fmt.Printf("Build state: %s...\n", result.State)
		time.Sleep(5 * time.Second)
	}
	return false
}

func createAPIToken(superuserToken string) string {
	if !exists("/app/rest/users/username:admin", superuserToken) {
		post("/app/rest/users", superuserToken, `{"username":"admin","password":"admin123","roles":{"role":[{"roleId":"SYSTEM_ADMIN","scope":"g"}]}}`)
	}

	req, _ := http.NewRequest("DELETE", baseURL+"/app/rest/users/current/tokens/tc-cli-test", nil)
	req.SetBasicAuth("admin", "admin123")
	if resp, _ := http.DefaultClient.Do(req); resp != nil {
		resp.Body.Close()
	}

	req, _ = http.NewRequest("POST", baseURL+"/app/rest/users/current/tokens/tc-cli-test", nil)
	req.SetBasicAuth("admin", "admin123")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create API token: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Value string `json:"value"`
	}
	json.Unmarshal(body, &result)
	if result.Value == "" {
		fmt.Fprintf(os.Stderr, "Failed to get API token (status %d): %s\n", resp.StatusCode, body)
		os.Exit(1)
	}
	return result.Value
}
