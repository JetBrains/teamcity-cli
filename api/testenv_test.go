//go:build integration || guest

package api_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/docker/docker/api/types/container"
	"github.com/joho/godotenv"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	serverImage = "jetbrains/teamcity-server:latest"
	agentImage  = "jetbrains/teamcity-agent:latest"
	serverName  = "tc-test-server"
	agentName   = "tc-test-agent"
)

type testEnv struct {
	Client    *api.Client
	URL       string
	Token     string
	ProjectID string
	ConfigID  string
	Build     *api.Build

	guestAuth      bool
	ownsContainers bool
	network        *testcontainers.DockerNetwork
	server         testcontainers.Container
	agent          testcontainers.Container
	ctx            context.Context
}

func (e *testEnv) Cleanup() {
	if !e.ownsContainers {
		return
	}
	if e.agent != nil {
		_ = e.agent.Terminate(e.ctx)
	}
	if e.server != nil {
		_ = e.server.Terminate(e.ctx)
	}
	if e.network != nil {
		_ = e.network.Remove(e.ctx)
	}
}

func setupTestEnv() (*testEnv, error) {
	_ = godotenv.Load("../../.env")

	url := os.Getenv("TEAMCITY_URL")
	token := os.Getenv("TEAMCITY_TOKEN")

	if guest := os.Getenv("TEAMCITY_GUEST"); guest == "1" || guest == "true" || guest == "yes" {
		if url == "" {
			url = "https://cli.teamcity.com"
		}
		client := api.NewGuestClient(url)
		if _, err := client.GetServer(); err != nil {
			return nil, fmt.Errorf("guest auth failed for %s: %w", url, err)
		}
		log.Printf("Using guest auth against %s", url)
		env := &testEnv{
			Client:    client,
			URL:       url,
			guestAuth: true,
		}
		if err := env.discoverTestData(); err != nil {
			log.Println("Warning: could not discover test data:", err.Error())
		}
		return env, nil
	}

	if url != "" && token != "" {
		client := api.NewClient(url, token)
		if _, err := client.GetCurrentUser(); err == nil {
			env := &testEnv{
				Client:    client,
				URL:       url,
				Token:     token,
				ProjectID: os.Getenv("TEAMCITY_TEST_PROJECT"),
				ConfigID:  os.Getenv("TEAMCITY_TEST_CONFIG"),
			}
			if err := env.ensureBuild(); err != nil {
				log.Println("Warning: could not ensure test build:", err.Error())
			}
			return env, nil
		}
		log.Println("Configured credentials invalid, falling back to testcontainers")
	}

	return startContainers()
}

func (e *testEnv) discoverTestData() error {
	projects, err := e.Client.GetProjects(api.ProjectsOptions{Parent: "_Root", Limit: 5})
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	for _, p := range projects.Projects {
		if p.ID != "_Root" {
			e.ProjectID = p.ID
			break
		}
	}

	if e.ProjectID != "" {
		configs, err := e.Client.GetBuildTypes(api.BuildTypesOptions{Project: e.ProjectID, Limit: 5})
		if err == nil && len(configs.BuildTypes) > 0 {
			e.ConfigID = configs.BuildTypes[0].ID
		}
	}

	if e.ConfigID != "" {
		if err := e.ensureBuild(); err != nil {
			log.Println("Warning: could not ensure test build:", err.Error())
		}
	}

	log.Printf("Discovered: project=%s config=%s build=%v", e.ProjectID, e.ConfigID, e.Build != nil)
	return nil
}

func startContainers() (*testEnv, error) {
	ctx := context.Background()

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return nil, fmt.Errorf("docker not available: %w", err)
	}
	defer provider.Close()

	env := &testEnv{
		ctx:       ctx,
		ProjectID: "Sandbox",
		ConfigID:  "Sandbox_Demo",
	}

	existing := findExistingServer(ctx)
	if existing != nil {
		log.Println("Reusing existing testcontainers...")
		env.server = existing
		host, _ := existing.Host(ctx)
		port, _ := existing.MappedPort(ctx, "8111/tcp")
		env.URL = fmt.Sprintf("http://%s:%s", host, port.Port())
		env.Token = os.Getenv("TEAMCITY_TOKEN")
		if env.Token == "" {
			return nil, fmt.Errorf("existing container found but TEAMCITY_TOKEN not set")
		}
		env.Client = api.NewClient(env.URL, env.Token)
		if err := env.ensureBuild(); err != nil {
			log.Println("Warning: could not ensure test build:", err.Error())
		}
		return env, nil
	}

	env.ownsContainers = true
	log.Println("Starting testcontainers...")

	env.network, err = network.New(ctx,
		network.WithCheckDuplicate(),
		network.WithDriver("bridge"),
	)
	if err != nil {
		return nil, fmt.Errorf("create network: %w", err)
	}

	log.Println("Starting TeamCity server...")
	env.server, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:         serverName,
			Image:        serverImage,
			ExposedPorts: []string{"8111/tcp"},
			Networks:     []string{env.network.Name},
			NetworkAliases: map[string][]string{
				env.network.Name: {"teamcity-server"},
			},
			Env: map[string]string{
				"TEAMCITY_SERVER_OPTS": "-Dteamcity.installation.completed=true -Dteamcity.startup.maintenance=false -Dteamcity.licenseAgreement.accepted=true",
			},
			WaitingFor: wait.ForHTTP("/app/rest/server/version").
				WithPort("8111/tcp").
				WithStatusCodeMatcher(func(status int) bool { return status == 200 || status == 401 }).
				WithStartupTimeout(5 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("start server: %w", err)
	}

	host, _ := env.server.Host(ctx)
	port, _ := env.server.MappedPort(ctx, "8111/tcp")
	env.URL = fmt.Sprintf("http://%s:%s", host, port.Port())
	log.Println("Server running at:", env.URL)

	superToken, err := getSuperuserToken(ctx, env.server)
	if err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("get superuser token: %w", err)
	}

	if err := acceptLicense(env.URL, superToken); err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("accept license: %w", err)
	}

	env.Token, err = setupServer(env.URL, superToken, env.ProjectID, env.ConfigID)
	if err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("setup server: %w", err)
	}

	os.Setenv("TEAMCITY_URL", env.URL)
	os.Setenv("TEAMCITY_TOKEN", env.Token)
	os.Setenv("TEAMCITY_TEST_PROJECT", env.ProjectID)
	os.Setenv("TEAMCITY_TEST_CONFIG", env.ConfigID)

	env.Client = api.NewClient(env.URL, env.Token)

	log.Println("Starting TeamCity agent...")
	env.agent, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:     agentName,
			Image:    agentImage,
			Networks: []string{env.network.Name},
			NetworkAliases: map[string][]string{
				env.network.Name: {"teamcity-agent"},
			},
			Env:        map[string]string{"SERVER_URL": "http://teamcity-server:8111"},
			Privileged: true,
			// Block EC2 IMDS so the agent's amazonEC2 plugin doesn't override SERVER_URL
			// with a TeamCity Cloud placeholder when running on EC2 instances.
			ExtraHosts: []string{"169.254.169.254:127.0.0.1"},
			ConfigModifier: func(c *container.Config) {
				c.Tty = true
				c.OpenStdin = true
			},
		},
		Started: true,
	})
	if err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("start agent: %w", err)
	}

	if err := waitForAgent(env.Client); err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("authorize agent: %w", err)
	}

	if err := env.ensureBuild(); err != nil {
		log.Println("Warning: could not ensure test build:", err.Error())
	}

	return env, nil
}

func findExistingServer(ctx context.Context) testcontainers.Container {
	containers, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{Name: serverName},
		Reuse:            true,
	})
	if err != nil || containers == nil {
		return nil
	}
	state, err := containers.State(ctx)
	if err != nil || !state.Running {
		return nil
	}
	return containers
}

func (e *testEnv) ensureBuild() error {
	if e.ConfigID == "" {
		return fmt.Errorf("config ID not set")
	}

	builds, err := e.Client.GetBuilds(api.BuildsOptions{BuildTypeID: e.ConfigID, State: "finished", Limit: 1})
	if err != nil {
		return err
	}
	if builds.Count > 0 {
		e.Build = &builds.Builds[0]
		log.Println("Using existing build:", e.Build.ID)
		return nil
	}

	log.Println("Triggering new build...")
	build, err := e.Client.RunBuild(e.ConfigID, api.RunBuildOptions{Comment: "Test setup"})
	if err != nil {
		return err
	}

	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		build, err = e.Client.GetBuild(fmt.Sprintf("%d", build.ID))
		if err != nil {
			return err
		}
		if build.State == "finished" {
			e.Build = build
			log.Println("Build finished:", build.Status)
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("build timeout")
}

func getSuperuserToken(ctx context.Context, container testcontainers.Container) (string, error) {
	time.Sleep(2 * time.Second)
	reader, err := container.Logs(ctx)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`Super user authentication token: (\d+)`)
	if m := re.FindStringSubmatch(string(logs)); len(m) >= 2 {
		return m[1], nil
	}
	return "", fmt.Errorf("token not found in logs")
}

func acceptLicense(serverURL, superToken string) error {
	req, _ := http.NewRequest("POST", serverURL+"/showAgreement.html?agree=true&super="+superToken, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	time.Sleep(2 * time.Second)
	return nil
}

func setupServer(serverURL, superToken, projectID, configID string) (string, error) {
	client := api.NewClientWithBasicAuth(serverURL, "", superToken)

	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := client.GetServer(); err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}

	// Set internal server URL so build properties use the Docker network name
	setServerURL(serverURL, superToken, "http://teamcity-server:8111")

	if !client.ProjectExists(projectID) {
		if _, err := client.CreateProject(api.CreateProjectRequest{ID: projectID, Name: "Sandbox"}); err != nil {
			return "", err
		}
	}

	if !client.BuildTypeExists(configID) {
		if _, err := client.CreateBuildType(projectID, api.CreateBuildTypeRequest{ID: configID, Name: "Demo"}); err != nil {
			return "", err
		}
		client.CreateBuildStep(configID, api.BuildStep{
			Name: "Test",
			Type: "simpleRunner",
			Properties: api.PropertyList{
				Property: []api.Property{
					{Name: "script.content", Value: "echo Hello\necho 'test artifact content' > result.txt\nmkdir -p reports\necho 'report data' > reports/summary.txt"},
					{Name: "use.custom.script", Value: "true"},
				},
			},
		})
		client.SetBuildTypeSetting(configID, "artifactRules", "result.txt\nreports => reports")
	}

	if !client.UserExists("admin") {
		if _, err := client.CreateUser(api.CreateUserRequest{
			Username: "admin",
			Password: "admin123",
			Roles:    api.RoleList{Role: []api.Role{{RoleID: "SYSTEM_ADMIN", Scope: "g"}}},
		}); err != nil {
			return "", err
		}
	}

	adminClient := api.NewClientWithBasicAuth(serverURL, "admin", "admin123")
	_ = adminClient.DeleteAPIToken("tc-cli-test")
	token, err := adminClient.CreateAPIToken("tc-cli-test")
	if err != nil {
		return "", err
	}
	return token.Value, nil
}

func waitForAgent(client *api.Client) error {
	log.Println("Waiting for agent...")
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		agents, err := client.GetAgents(api.AgentsOptions{})
		if err == nil && len(agents.Agents) > 0 {
			log.Println("Authorizing agent...")
			return client.AuthorizeAgent(agents.Agents[0].ID, true)
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("agent timeout")
}

func setServerURL(serverURL, superToken, internalURL string) {
	req, err := http.NewRequest("PUT", serverURL+"/app/rest/server/rootUrl", strings.NewReader(internalURL))
	if err != nil {
		log.Printf("Warning: could not create request to set server URL: %v", err)
		return
	}
	req.SetBasicAuth("", superToken)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Warning: could not set server URL: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("Warning: set server URL returned %d", resp.StatusCode)
		return
	}
	log.Printf("Set server root URL to %s", internalURL)
}

func copyBinaryToAgent(env *testEnv) error {
	log.Println("Building CLI binary for agent...")

	tmpDir, err := os.MkdirTemp("", "tc-cli-build")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := tmpDir + "/tc"
	cmd := exec.Command("go", "build", "-o", binaryPath, "../tc")
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build binary: %w", err)
	}

	log.Println("Copying binary to agent container...")
	err = env.agent.CopyFileToContainer(env.ctx, binaryPath, "/usr/local/bin/teamcity", 0755)
	if err != nil {
		return fmt.Errorf("copy to container: %w", err)
	}

	log.Println("CLI binary installed on agent at /usr/local/bin/teamcity")
	return nil
}
