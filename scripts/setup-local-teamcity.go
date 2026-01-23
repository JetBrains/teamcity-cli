//go:build ignore

// Setup local TeamCity for integration testing.
// Usage: go run scripts/setup-local-teamcity.go

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/tiulpin/teamcity-cli/internal/api"
)

const (
	baseURL           = "http://localhost:8111"
	testProjectID     = "Sandbox"
	testBuildConfigID = "Sandbox_Demo"
)

func main() {
	os.Setenv("TC_INSECURE_SKIP_WARN", "1")

	fmt.Println("Starting TeamCity containers...")
	run("docker", "compose", "up", "-d")

	fmt.Println("Waiting for server...")
	waitFor(func() bool {
		resp, err := http.Get(baseURL + "/app/rest/server")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == 200 || resp.StatusCode == 401
	}, 10*time.Minute)

	// Get superuser token from docker logs and create client with Basic Auth
	superuserToken := getSuperuserToken()
	fmt.Printf("Got superuser token: %s...\n", superuserToken[:8])

	superClient := api.NewClientWithBasicAuth(baseURL, "", superuserToken)

	// Create a test project if it doesn't exist
	if !superClient.ProjectExists(testProjectID) {
		fmt.Println("Creating test project...")
		_, err := superClient.CreateProject(api.CreateProjectRequest{
			ID:   testProjectID,
			Name: "Sandbox",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create project: %v\n", err)
			os.Exit(1)
		}
	}

	// Create test build config if it doesn't exist
	if !superClient.BuildTypeExists(testBuildConfigID) {
		fmt.Println("Creating test build config...")
		_, err := superClient.CreateBuildType(testProjectID, api.CreateBuildTypeRequest{
			ID:   testBuildConfigID,
			Name: "Demo",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create build config: %v\n", err)
			os.Exit(1)
		}

		err = superClient.CreateBuildStep(testBuildConfigID, api.BuildStep{
			Name: "Test",
			Type: "simpleRunner",
			Properties: api.PropertyList{
				Property: []api.Property{
					{Name: "script.content", Value: "echo Hello\nmkdir -p output\necho 'test content' > output/result.txt\necho '{\"status\":\"ok\"}' > output/report.json"},
					{Name: "use.custom.script", Value: "true"},
				},
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create build step: %v\n", err)
			os.Exit(1)
		}

		// Set artifact rules
		err = superClient.SetBuildTypeSetting(testBuildConfigID, "artifactRules", "output/** => artifacts")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set artifact rules: %v\n", err)
			os.Exit(1)
		}
	}

	// Create admin user if it doesn't exist
	fmt.Println("Setting up admin user...")
	if !superClient.UserExists("admin") {
		_, err := superClient.CreateUser(api.CreateUserRequest{
			Username: "admin",
			Password: "admin123",
			Roles: api.RoleList{
				Role: []api.Role{{RoleID: "SYSTEM_ADMIN", Scope: "g"}},
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create admin user: %v\n", err)
			os.Exit(1)
		}
	}

	// Create API token using admin credentials
	fmt.Println("Creating API token...")
	adminClient := api.NewClientWithBasicAuth(baseURL, "admin", "admin123")

	// Delete existing token if any (ignore errors)
	_ = adminClient.DeleteAPIToken("tc-cli-test")

	token, err := adminClient.CreateAPIToken("tc-cli-test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create API token: %v\n", err)
		os.Exit(1)
	}

	// Write .env file
	env := fmt.Sprintf("TEAMCITY_URL=%s\nTEAMCITY_TOKEN=%s\nTEAMCITY_TEST_CONFIG=%s\nTEAMCITY_TEST_PROJECT=%s\n",
		baseURL, token.Value, testBuildConfigID, testProjectID)
	if err := os.WriteFile(".env", []byte(env), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write .env: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Wrote .env")

	// Now use bearer token client for remaining operations
	client := api.NewClient(baseURL, token.Value)

	fmt.Println("Waiting for build agent...")
	agentID := waitForAgent(client, 3*time.Minute)
	if agentID > 0 {
		if err := client.AuthorizeAgent(agentID, true); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authorize agent: %v\n", err)
		} else {
			fmt.Println("Agent authorized")
		}

		fmt.Println("Triggering initial build...")
		build, err := client.RunBuild(testBuildConfigID, api.RunBuildOptions{
			Comment: "Setup script - initial build",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to trigger build: %v\n", err)
		} else {
			fmt.Printf("Triggered build #%d, waiting for completion...\n", build.ID)
			if waitForBuild(client, build.ID) {
				fmt.Println("Initial build completed")
			} else {
				fmt.Println("Build did not complete in time (tests will trigger their own)")
			}
		}
	} else {
		fmt.Println("WARNING: No agent connected after timeout (tests may skip)")
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

func waitForAgent(client *api.Client, timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		if err == nil && len(agents.Agents) > 0 {
			return agents.Agents[0].ID
		}
		fmt.Println("Waiting for agent to connect...")
		time.Sleep(5 * time.Second)
	}
	return 0
}

func waitForBuild(client *api.Client, buildID int) bool {
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		build, err := client.GetBuild(fmt.Sprintf("%d", buildID))
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		if build.State == "finished" {
			fmt.Printf("Build finished with status: %s\n", build.Status)
			return true
		}

		fmt.Printf("Build state: %s...\n", build.State)
		time.Sleep(5 * time.Second)
	}
	return false
}
