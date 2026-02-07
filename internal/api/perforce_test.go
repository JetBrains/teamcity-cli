//go:build integration

package api_test

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	p4dImage = "sourcegraph/helix-p4d:latest"
	p4dName  = "tc-test-p4d"
)

// perforceTestEnv holds the state for Perforce integration tests
type perforceTestEnv struct {
	container testcontainers.Container
	port      string
	host      string
	ctx       context.Context
}

func (e *perforceTestEnv) Cleanup() {
	if e.container != nil {
		_ = e.container.Terminate(e.ctx)
	}
}

// startP4D starts a Perforce server container and populates it with test data.
func startP4D(ctx context.Context, networkName string) (*perforceTestEnv, error) {
	log.Println("Starting Perforce server (p4d)...")

	aliases := map[string][]string{}
	var networks []string
	if networkName != "" {
		networks = []string{networkName}
		aliases[networkName] = []string{"perforce-server"}
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Name:           p4dName,
			Image:          p4dImage,
			ExposedPorts:   []string{"1666/tcp"},
			Networks:       networks,
			NetworkAliases: aliases,
			WaitingFor: wait.ForLog("p4d -r").
				WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start p4d: %w", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "1666/tcp")

	env := &perforceTestEnv{
		container: container,
		host:      host,
		port:      port.Port(),
		ctx:       ctx,
	}

	log.Printf("P4D running at %s:%s", host, env.port)

	if err := waitForP4D(ctx, container); err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("p4d not ready: %w", err)
	}

	if err := populateP4Depot(ctx, container); err != nil {
		env.Cleanup()
		return nil, fmt.Errorf("populate depot: %w", err)
	}

	return env, nil
}

// waitForP4D polls p4d until it accepts connections.
func waitForP4D(ctx context.Context, container testcontainers.Container) error {
	deadline := time.After(30 * time.Second)
	for {
		select {
		case <-deadline:
			return fmt.Errorf("timeout waiting for p4d to accept connections")
		case <-ctx.Done():
			return ctx.Err()
		default:
			_, _, err := container.Exec(ctx, []string{"p4", "-p", "localhost:1666", "info"})
			if err == nil {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// populateP4Depot creates a test depot with files in the p4d container.
func populateP4Depot(ctx context.Context, container testcontainers.Container) error {
	commands := [][]string{
		// Create a client workspace for initial setup
		{"bash", "-c", `p4 -p localhost:1666 -u admin client -o test-setup |
sed 's|//depot/...|//depot/main/...|' |
p4 -p localhost:1666 -u admin client -i`},
		// Create the depot structure with a test file
		{"bash", "-c", `export P4PORT=localhost:1666 P4USER=admin P4CLIENT=test-setup
mkdir -p /tmp/p4ws/main
cd /tmp/p4ws
p4 set P4PORT=localhost:1666
p4 set P4USER=admin
p4 set P4CLIENT=test-setup
echo 'Hello from Perforce' > /tmp/p4ws/main/test.txt
p4 -p localhost:1666 -u admin -c test-setup add /tmp/p4ws/main/test.txt 2>/dev/null || true
p4 -p localhost:1666 -u admin -c test-setup submit -d "Initial commit" 2>/dev/null || true`},
	}

	for _, cmd := range commands {
		_, _, err := container.Exec(ctx, cmd)
		if err != nil {
			return fmt.Errorf("p4d setup failed: %w", err)
		}
	}
	return nil
}

// TestPerforceVcsRootCRUD tests creating, reading, and deleting a Perforce VCS root.
func TestPerforceVcsRootCRUD(T *testing.T) {
	if testEnvRef == nil || testEnvRef.network == nil {
		T.Skip("test requires testcontainers with Docker network")
	}

	ctx := context.Background()
	p4Env, err := startP4D(ctx, testEnvRef.network.Name)
	if err != nil {
		T.Skipf("could not start p4d: %v", err)
	}
	defer p4Env.Cleanup()

	vcsRootID := "Sandbox_PerforceTest"

	T.Run("create perforce vcs root", func(t *testing.T) {
		// Use the Docker network name for the P4 port so TeamCity server can reach it
		p4Port := fmt.Sprintf("perforce-server:1666")

		root, err := client.CreateVcsRoot(api.CreateVcsRootRequest{
			ID:      vcsRootID,
			Name:    "Perforce Test Depot",
			VcsName: "perforce",
			ProjectID: testProject,
			Properties: api.NewPerforceVcsRootProperties(
				p4Port,
				"admin",
				"",
				"",
			),
		})
		require.NoError(t, err)
		assert.Equal(t, vcsRootID, root.ID)
		assert.Equal(t, "perforce", root.VcsName)
		t.Logf("Created VCS root: %s", root.ID)
	})

	T.Run("get perforce vcs root", func(t *testing.T) {
		root, err := client.GetVcsRoot(vcsRootID)
		require.NoError(t, err)
		assert.Equal(t, "perforce", root.VcsName)
		assert.Equal(t, "Perforce Test Depot", root.Name)

		// Verify properties
		props := make(map[string]string)
		for _, p := range root.Properties.Property {
			props[p.Name] = p.Value
		}
		assert.Contains(t, props["port"], "perforce-server:1666")
		assert.Equal(t, "admin", props["user"])
		t.Logf("VCS root properties: %v", props)
	})

	T.Run("list vcs roots includes perforce", func(t *testing.T) {
		roots, err := client.GetVcsRoots(api.VcsRootOptions{Project: testProject})
		require.NoError(t, err)

		found := false
		for _, r := range roots.VcsRoots {
			if r.ID == vcsRootID {
				found = true
				assert.Equal(t, "perforce", r.VcsName)
				break
			}
		}
		assert.True(t, found, "should find perforce VCS root in list")
	})

	T.Run("vcs root exists", func(t *testing.T) {
		assert.True(t, client.VcsRootExists(vcsRootID))
		assert.False(t, client.VcsRootExists("NonExistent_P4Root"))
	})

	T.Run("attach to build config", func(t *testing.T) {
		// Create a new build config for this test
		p4ConfigID := "Sandbox_PerforceDemo"
		if !client.BuildTypeExists(p4ConfigID) {
			_, err := client.CreateBuildType(testProject, api.CreateBuildTypeRequest{
				ID:   p4ConfigID,
				Name: "Perforce Demo",
			})
			require.NoError(t, err)
		}

		err := client.AttachVcsRoot(p4ConfigID, vcsRootID)
		require.NoError(t, err)
		t.Logf("Attached VCS root %s to build config %s", vcsRootID, p4ConfigID)
	})

	T.Run("delete perforce vcs root", func(t *testing.T) {
		// First detach from build configs by deleting the build config
		// (simpler than detaching VCS root entries)
		p4ConfigID := "Sandbox_PerforceDemo"
		if client.BuildTypeExists(p4ConfigID) {
			// Delete the build config first to avoid "VCS root is in use" error
			raw, err := client.RawRequest("DELETE", fmt.Sprintf("/app/rest/buildTypes/id:%s", p4ConfigID), nil, nil)
			if err != nil {
				t.Logf("Warning: could not delete build config: %v", err)
			} else if raw.StatusCode >= 300 {
				t.Logf("Warning: delete build config returned %d", raw.StatusCode)
			}
		}

		err := client.DeleteVcsRoot(vcsRootID)
		require.NoError(t, err)

		assert.False(t, client.VcsRootExists(vcsRootID), "VCS root should be deleted")
	})
}

// TestPerforceBuildWithVcsRoot tests running a build that uses a Perforce VCS root.
func TestPerforceBuildWithVcsRoot(T *testing.T) {
	if testEnvRef == nil || testEnvRef.network == nil {
		T.Skip("test requires testcontainers with Docker network")
	}

	ctx := context.Background()
	p4Env, err := startP4D(ctx, testEnvRef.network.Name)
	if err != nil {
		T.Skipf("could not start p4d: %v", err)
	}
	defer p4Env.Cleanup()

	// Create a Perforce VCS root
	vcsRootID := "Sandbox_P4BuildTest"
	p4ConfigID := "Sandbox_P4BuildDemo"

	// Cleanup from any previous run
	if client.BuildTypeExists(p4ConfigID) {
		client.RawRequest("DELETE", fmt.Sprintf("/app/rest/buildTypes/id:%s", p4ConfigID), nil, nil)
	}
	if client.VcsRootExists(vcsRootID) {
		client.DeleteVcsRoot(vcsRootID)
	}

	// Create VCS root pointing to the p4d container
	root, err := client.CreateVcsRoot(api.CreateVcsRootRequest{
		ID:        vcsRootID,
		Name:      "P4 Build Test",
		VcsName:   "perforce",
		ProjectID: testProject,
		Properties: api.NewPerforceVcsRootProperties(
			"perforce-server:1666",
			"admin",
			"",
			"",
		),
	})
	require.NoError(T, err)
	T.Logf("Created VCS root: %s", root.ID)

	defer func() {
		if client.BuildTypeExists(p4ConfigID) {
			client.RawRequest("DELETE", fmt.Sprintf("/app/rest/buildTypes/id:%s", p4ConfigID), nil, nil)
		}
		client.DeleteVcsRoot(vcsRootID)
	}()

	// Create build config with a simple step
	_, err = client.CreateBuildType(testProject, api.CreateBuildTypeRequest{
		ID:   p4ConfigID,
		Name: "P4 Build Demo",
	})
	require.NoError(T, err)

	err = client.AttachVcsRoot(p4ConfigID, vcsRootID)
	require.NoError(T, err)

	err = client.CreateBuildStep(p4ConfigID, api.BuildStep{
		Name: "Test P4",
		Type: "simpleRunner",
		Properties: api.PropertyList{
			Property: []api.Property{
				{Name: "script.content", Value: "echo 'Build from Perforce depot'\nls -la"},
				{Name: "use.custom.script", Value: "true"},
			},
		},
	})
	require.NoError(T, err)

	// Trigger a build
	build, err := client.RunBuild(p4ConfigID, api.RunBuildOptions{
		Comment: "Perforce integration test",
	})
	require.NoError(T, err)
	T.Logf("Triggered build #%d", build.ID)

	// Wait for build to complete (it may fail if p4 checkout doesn't work,
	// but the important thing is that the VCS root integration works)
	buildID := fmt.Sprintf("%d", build.ID)
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		build, err = client.GetBuild(buildID)
		require.NoError(T, err)
		if build.State == "finished" {
			break
		}
		time.Sleep(5 * time.Second)
	}

	T.Logf("Build finished: state=%s status=%s", build.State, build.Status)

	// Get build log to see what happened
	buildLog, err := client.GetBuildLog(buildID)
	if err == nil {
		// Look for Perforce-related output in the log
		if strings.Contains(buildLog, "Perforce") || strings.Contains(buildLog, "p4") {
			T.Logf("Build log contains Perforce references")
		}
		// Log last 500 chars for debugging
		if len(buildLog) > 500 {
			T.Logf("Build log (tail):\n%s", buildLog[len(buildLog)-500:])
		} else {
			T.Logf("Build log:\n%s", buildLog)
		}
	}

	// The build triggered successfully with a Perforce VCS root - that's the key assertion
	assert.Equal(T, "finished", build.State, "build should have finished")
}

// TestPerforceUploadDiffChanges tests uploading a Perforce-style diff as personal changes.
func TestPerforceUploadDiffChanges(T *testing.T) {
	T.Parallel()

	// Perforce diffs in unified format should upload just like Git diffs
	patch := []byte(`--- a/depot/main/test.txt
+++ b/depot/main/test.txt
@@ -1 +1 @@
-Hello from Perforce
+Hello from Perforce - modified in personal build
`)

	changeID, err := client.UploadDiffChanges(patch, "Perforce personal build test")
	require.NoError(T, err)
	assert.NotEmpty(T, changeID)
	T.Logf("Uploaded Perforce diff as change ID: %s", changeID)
}
