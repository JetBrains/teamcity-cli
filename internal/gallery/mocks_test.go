//go:build gallery

package gallery_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func setupGalleryMocks(t *testing.T) *cmdtest.TestServer {
	ts := cmdtest.SetupMockClient(t)
	now := time.Now().UTC()
	tcTime := func(d time.Duration) string { return now.Add(d).Format("20060102T150405+0000") }

	ts.Handle("GET /app/rest/builds", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "snapshotDependency") {
			q := r.URL.RawQuery
			match := func(id string) bool {
				return strings.Contains(q, "id%3A"+id) || strings.Contains(q, "id:"+id)
			}
			switch {
			case match("45231"): // Build → Lint + Compile
				cmdtest.JSON(w, api.BuildList{Count: 2, Builds: []api.Build{
					{ID: 45225, Number: "118", Status: "SUCCESS", State: "finished", BuildTypeID: "MyApp_Lint",
						BuildType: &api.BuildType{ID: "MyApp_Lint", Name: "Lint"}},
					{ID: 45240, Number: "501", Status: "SUCCESS", State: "finished", BuildTypeID: "MyApp_Compile",
						BuildType: &api.BuildType{ID: "MyApp_Compile", Name: "Compile"}},
				}})
			case match("45230"): // Run Tests → Build
				cmdtest.JSON(w, api.BuildList{Count: 1, Builds: []api.Build{
					{ID: 45231, Number: "831", Status: "SUCCESS", State: "finished", BuildTypeID: "MyApp_Build",
						BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build"}},
				}})
			case match("45229"): // Deploy Staging → Build + Run Tests
				cmdtest.JSON(w, api.BuildList{Count: 2, Builds: []api.Build{
					{ID: 45231, Number: "831", Status: "SUCCESS", State: "finished", BuildTypeID: "MyApp_Build",
						BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build"}},
					{ID: 45230, Number: "442", Status: "FAILURE", State: "finished", BuildTypeID: "MyApp_Test",
						BuildType: &api.BuildType{ID: "MyApp_Test", Name: "Run Tests"}},
				}})
			default:
				cmdtest.JSON(w, api.BuildList{Count: 0, Builds: []api.Build{}})
			}
			return
		}
		cmdtest.JSON(w, api.BuildList{Count: 7, Builds: []api.Build{
			{ID: 45231, Number: "831", Status: "SUCCESS", State: "finished", BuildTypeID: "MyApp_Build", BranchName: "main", DefaultBranch: true,
				StartDate: tcTime(-2*time.Hour - 2*time.Minute - 13*time.Second), FinishDate: tcTime(-2 * time.Hour),
				BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build"}, Triggered: &api.Triggered{Type: "user", User: &api.User{Name: "Viktor Tiulpin"}},
				WebURL: ts.URL + "/viewLog.html?buildId=45231"},
			{ID: 45230, Number: "442", Status: "FAILURE", State: "finished", BuildTypeID: "MyApp_Test", BranchName: "feature/auth",
				StartDate: tcTime(-3*time.Hour - 5*time.Minute - 1*time.Second), FinishDate: tcTime(-3 * time.Hour),
				BuildType: &api.BuildType{ID: "MyApp_Test", Name: "Run Tests"}, Triggered: &api.Triggered{Type: "vcs", User: &api.User{Name: "CI Bot"}},
				WebURL: ts.URL + "/viewLog.html?buildId=45230"},
			{ID: 45229, Number: "830", Status: "", State: "running", BuildTypeID: "MyApp_Deploy", BranchName: "main", DefaultBranch: true, PercentageComplete: 67,
				StartDate: tcTime(-1*time.Minute - 22*time.Second),
				BuildType: &api.BuildType{ID: "MyApp_Deploy", Name: "Deploy Staging"}, Triggered: &api.Triggered{Type: "user", User: &api.User{Name: "Viktor Tiulpin"}},
				Agent: &api.Agent{ID: 1, Name: "linux-agent-01"}, WebURL: ts.URL + "/viewLog.html?buildId=45229"},
			{ID: 45228, Number: "829", Status: "SUCCESS", State: "finished", BuildTypeID: "MyApp_Build", BranchName: "fix/memory-leak",
				StartDate: tcTime(-5*time.Hour - 45*time.Second), FinishDate: tcTime(-5 * time.Hour),
				BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build"}, Triggered: &api.Triggered{Type: "user", User: &api.User{Name: "Anna Kowalski"}},
				WebURL: ts.URL + "/viewLog.html?buildId=45228"},
			{ID: 45227, Number: "126", Status: "FAILURE", State: "finished", BuildTypeID: "MyApp_IntTest", BranchName: "main", DefaultBranch: true,
				StartDate: tcTime(-8*time.Hour - 12*time.Minute - 45*time.Second), FinishDate: tcTime(-8 * time.Hour),
				BuildType: &api.BuildType{ID: "MyApp_IntTest", Name: "Integration Tests"}, Triggered: &api.Triggered{Type: "schedule"},
				WebURL: ts.URL + "/viewLog.html?buildId=45227"},
			{ID: 45226, Number: "", Status: "", State: "queued", BuildTypeID: "MyApp_Build", BranchName: "feature/onboard",
				QueuedDate: tcTime(-25 * time.Hour), BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build"},
				Triggered: &api.Triggered{Type: "vcs", User: &api.User{Name: "GitHub Hook"}}, WaitReason: "All compatible agents are busy",
				WebURL: ts.URL + "/viewLog.html?buildId=45226"},
			{ID: 45225, Number: "118", Status: "SUCCESS", State: "finished", BuildTypeID: "MyApp_Lint", BranchName: "main", DefaultBranch: true,
				StartDate: tcTime(-25*time.Hour - 12*time.Second), FinishDate: tcTime(-25 * time.Hour),
				BuildType: &api.BuildType{ID: "MyApp_Lint", Name: "Lint"}, Triggered: &api.Triggered{Type: "vcs", User: &api.User{Name: "CI Bot"}},
				WebURL: ts.URL + "/viewLog.html?buildId=45225"},
		}})
	})

	dependents := map[string][]api.BuildType{
		"MyApp_Lint":    {{ID: "MyApp_Build", Name: "Build", ProjectID: "MyApp"}},
		"MyApp_Build":   {{ID: "MyApp_Test", Name: "Run Tests", ProjectID: "MyApp"}, {ID: "MyApp_IntTest", Name: "Integration Tests", ProjectID: "MyApp"}},
		"MyApp_Test":    {{ID: "MyApp_Deploy", Name: "Deploy Staging", ProjectID: "MyApp"}},
		"MyApp_IntTest": {{ID: "MyApp_Deploy", Name: "Deploy Staging", ProjectID: "MyApp"}},
		"MyApp_Deploy":  nil,
	}
	ts.Handle("GET /app/rest/buildTypes", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		if strings.Contains(q, "snapshotDependency") && strings.Contains(q, "from") {
			for id, list := range dependents {
				if strings.Contains(q, "id:"+id) || strings.Contains(q, "id%3A"+id) {
					cmdtest.JSON(w, api.BuildTypeList{Count: len(list), BuildTypes: list})
					return
				}
			}
			cmdtest.JSON(w, api.BuildTypeList{Count: 0, BuildTypes: []api.BuildType{}})
			return
		}
		if strings.Contains(r.URL.Path, "id:") {
			id := cmdtest.ExtractID(r.URL.Path, "id:")
			cmdtest.JSON(w, api.BuildType{ID: id, Name: "Build", ProjectID: "MyApp", ProjectName: "My Application", WebURL: ts.URL + "/viewType.html?buildTypeId=" + id})
			return
		}
		cmdtest.JSON(w, api.BuildTypeList{Count: 5, BuildTypes: []api.BuildType{
			{ID: "MyApp_Build", Name: "Build", ProjectID: "MyApp", ProjectName: "My Application"},
			{ID: "MyApp_Test", Name: "Run Tests", ProjectID: "MyApp", ProjectName: "My Application"},
			{ID: "MyApp_Deploy", Name: "Deploy Staging", ProjectID: "MyApp", ProjectName: "My Application", Paused: true},
			{ID: "MyApp_IntTest", Name: "Integration Tests", ProjectID: "MyApp", ProjectName: "My Application"},
			{ID: "MyApp_Lint", Name: "Lint", ProjectID: "MyApp", ProjectName: "My Application"},
		}})
	})

	ts.Handle("GET /app/rest/agents", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.AgentList{Count: 5, Agents: []api.Agent{
			{ID: 1, Name: "linux-agent-01", Connected: true, Enabled: true, Authorized: true, Pool: &api.Pool{ID: 0, Name: "Default"}},
			{ID: 2, Name: "linux-agent-02", Connected: true, Enabled: true, Authorized: true, Pool: &api.Pool{ID: 0, Name: "Default"}},
			{ID: 3, Name: "windows-agent-01", Connected: true, Enabled: true, Authorized: true, Pool: &api.Pool{ID: 1, Name: "Windows"}},
			{ID: 4, Name: "mac-agent-01", Connected: false, Enabled: true, Authorized: true, Pool: &api.Pool{ID: 2, Name: "macOS"}},
			{ID: 5, Name: "cloud-agent-01", Connected: true, Enabled: false, Authorized: true, Pool: &api.Pool{ID: 3, Name: "Cloud"}},
		}})
	})

	ts.Handle("GET /app/rest/projects", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.ProjectList{Count: 5, Projects: []api.Project{
			{ID: "_Root", Name: "Root project"},
			{ID: "MyApp", Name: "My Application", ParentProjectID: "_Root"},
			{ID: "MyApp_Frontend", Name: "Frontend", ParentProjectID: "MyApp"},
			{ID: "MyApp_Backend", Name: "Backend", ParentProjectID: "MyApp"},
			{ID: "Infrastructure", Name: "Infrastructure", ParentProjectID: "_Root"},
		}})
	})

	ts.Handle("GET /app/rest/buildQueue", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildQueue{Count: 5, Builds: []api.QueuedBuild{
			{ID: 45233, State: "queued", BuildTypeID: "MyApp_Build", BranchName: "feature/onboard",
				BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build"}, Triggered: &api.Triggered{Type: "vcs"}, WaitReason: "All compatible agents are busy"},
			{ID: 45234, State: "queued", BuildTypeID: "MyApp_Test", BranchName: "main",
				BuildType: &api.BuildType{ID: "MyApp_Test", Name: "Run Tests"}, Triggered: &api.Triggered{Type: "snapshotDependency"}, WaitReason: "Waiting for build #45233 in queue"},
			{ID: 45235, State: "queued", BuildTypeID: "MyApp_IntTest", BranchName: "main",
				BuildType: &api.BuildType{ID: "MyApp_IntTest", Name: "Integration Tests"}, Triggered: &api.Triggered{Type: "snapshotDependency"}, WaitReason: "Waiting for build #45234 in queue"},
			{ID: 45236, State: "queued", BuildTypeID: "MyApp_Build", BranchName: "develop",
				BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build"}, Triggered: &api.Triggered{Type: "user"}, WaitReason: "Build queue is paused"},
			{ID: 45237, State: "queued", BuildTypeID: "MyApp_Deploy", BranchName: "main",
				BuildType: &api.BuildType{ID: "MyApp_Deploy", Name: "Deploy Staging"}, Triggered: &api.Triggered{Type: "user"}, WaitReason: "Waiting for approval"},
		}})
	})

	ts.Handle("GET /app/rest/pipelines", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.PipelineList{Count: 5, Pipelines: []api.Pipeline{
			{ID: "MyApp_CI", Name: "CI", ParentProject: &api.ProjectRef{ID: "MyApp", Name: "My Application"},
				HeadBuildType: &api.BuildTypeRef{ID: "MyApp_CI"},
				Jobs:          &api.PipelineJobs{Count: 2, Job: []api.PipelineJob{{ID: "build", Name: "Build"}, {ID: "test", Name: "Test"}}}},
			{ID: "MyApp_Release", Name: "Release", ParentProject: &api.ProjectRef{ID: "MyApp", Name: "My Application"},
				HeadBuildType: &api.BuildTypeRef{ID: "MyApp_Release"},
				Jobs:          &api.PipelineJobs{Count: 4, Job: []api.PipelineJob{{ID: "build", Name: "Build"}, {ID: "test", Name: "Test"}, {ID: "stage", Name: "Stage"}, {ID: "prod", Name: "Production"}}}},
			{ID: "MyApp_Nightly", Name: "Nightly", ParentProject: &api.ProjectRef{ID: "MyApp", Name: "My Application"},
				HeadBuildType: &api.BuildTypeRef{ID: "MyApp_Nightly"},
				Jobs:          &api.PipelineJobs{Count: 3, Job: []api.PipelineJob{{ID: "build", Name: "Build"}, {ID: "test", Name: "Test"}, {ID: "perf", Name: "Performance"}}}},
			{ID: "Infra_Deploy", Name: "Infrastructure Deploy", ParentProject: &api.ProjectRef{ID: "Infrastructure", Name: "Infrastructure"},
				HeadBuildType: &api.BuildTypeRef{ID: "Infra_Deploy"},
				Jobs:          &api.PipelineJobs{Count: 2, Job: []api.PipelineJob{{ID: "plan", Name: "Plan"}, {ID: "apply", Name: "Apply"}}}},
			{ID: "Frontend_CI", Name: "Frontend CI", ParentProject: &api.ProjectRef{ID: "MyApp_Frontend", Name: "Frontend"},
				HeadBuildType: &api.BuildTypeRef{ID: "Frontend_CI"},
				Jobs:          &api.PipelineJobs{Count: 3, Job: []api.PipelineJob{{ID: "lint", Name: "Lint"}, {ID: "build", Name: "Build"}, {ID: "test", Name: "Test"}}}},
		}})
	})

	ts.Handle("GET /app/rest/agentPools", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.PoolList{Count: 5, Pools: []api.Pool{
			{ID: 0, Name: "Default", MaxAgents: 0},
			{ID: 1, Name: "Linux Agents", MaxAgents: 10},
			{ID: 2, Name: "Windows Agents", MaxAgents: 5},
			{ID: 3, Name: "macOS Agents", MaxAgents: 3},
			{ID: 4, Name: "Cloud (Auto-scale)", MaxAgents: 50},
		}})
	})

	galleryBuild := api.Build{
		ID: 45231, Number: "831", Status: "SUCCESS", State: "finished",
		BuildTypeID: "MyApp_Build", BranchName: "main", DefaultBranch: true,
		StartDate: tcTime(-2*time.Hour - 2*time.Minute - 13*time.Second), FinishDate: tcTime(-2 * time.Hour),
		BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build", ProjectID: "MyApp", ProjectName: "My Application"},
		Triggered: &api.Triggered{Type: "user", User: &api.User{Name: "Viktor Tiulpin"}},
		Agent:     &api.Agent{ID: 1, Name: "linux-agent-01"},
		Tags:      &api.TagList{Tag: []api.Tag{{Name: "release"}, {Name: "v1.0.0"}}},
		WebURL:    ts.URL + "/viewLog.html?buildId=45231",
	}
	galleryBuilds := map[string]api.Build{
		"45231": galleryBuild,
		"45230": {ID: 45230, Number: "442", Status: "FAILURE", State: "finished", BuildTypeID: "MyApp_Test", BranchName: "feature/auth",
			BuildType: &api.BuildType{ID: "MyApp_Test", Name: "Run Tests", ProjectID: "MyApp", ProjectName: "My Application"},
			Triggered: &api.Triggered{Type: "vcs", User: &api.User{Name: "CI Bot"}},
			StartDate: tcTime(-3*time.Hour - 5*time.Minute), FinishDate: tcTime(-3 * time.Hour),
			WebURL: ts.URL + "/viewLog.html?buildId=45230"},
		"45229": {ID: 45229, Number: "830", Status: "", State: "running", BuildTypeID: "MyApp_Deploy", BranchName: "main",
			BuildType: &api.BuildType{ID: "MyApp_Deploy", Name: "Deploy Staging", ProjectID: "MyApp", ProjectName: "My Application"},
			Triggered: &api.Triggered{Type: "user", User: &api.User{Name: "Viktor Tiulpin"}},
			Agent:     &api.Agent{ID: 1, Name: "linux-agent-01"}, PercentageComplete: 67,
			StartDate: tcTime(-1*time.Minute - 22*time.Second),
			WebURL:    ts.URL + "/viewLog.html?buildId=45229"},
		"45233": {ID: 45233, Number: "", Status: "", State: "queued", BuildTypeID: "MyApp_Build",
			BuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build", ProjectID: "MyApp", ProjectName: "My Application"},
			WebURL:    ts.URL + "/viewLog.html?buildId=45233"},
	}
	ts.Handle("GET /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		id := cmdtest.ExtractID(path, "id:")
		switch {
		case strings.Contains(path, "/snapshot-dependencies"):
			cmdtest.JSON(w, api.BuildList{Count: 0, Builds: []api.Build{}})
		case strings.Contains(path, "/tags"):
			cmdtest.JSON(w, api.TagList{Tag: []api.Tag{{Name: "release"}, {Name: "v1.0.0"}}})
		case strings.Contains(path, "/comment"):
			cmdtest.Text(w, "Verified in staging — ready for production")
		case strings.Contains(path, "/pin"):
			w.WriteHeader(http.StatusOK)
		case strings.Contains(path, "/artifacts/content/"):
			_, _ = w.Write([]byte("file content"))
		case strings.Contains(path, "/artifacts"):
			href := &api.Content{Href: "/download"}
			cmdtest.JSON(w, api.Artifacts{Count: 5, File: []api.Artifact{
				{Name: "app.jar", Size: 13002342, Content: href},
				{Name: "test-report.html", Size: 239616, Content: href},
				{Name: "coverage.xml", Size: 45230, Content: href},
				{Name: "checksums.sha256", Size: 512, Content: href},
				{Name: "logs", Children: &api.Artifacts{Count: 2, File: []api.Artifact{
					{Name: "build.log", Size: 45678, Content: href},
					{Name: "test.log", Size: 12345, Content: href},
				}}},
			}})
		case strings.Contains(path, "/resulting-properties"):
			if id == "45230" {
				cmdtest.JSON(w, api.ParameterList{Count: 3, Property: []api.Parameter{
					{Name: "version", Value: "1.0.1"},
					{Name: "env.JAVA_HOME", Value: "/usr/lib/jvm/java-17"},
					{Name: "new.flag", Value: "true"},
				}})
			} else {
				cmdtest.JSON(w, api.ParameterList{Count: 2, Property: []api.Parameter{
					{Name: "version", Value: "1.0.0"},
					{Name: "env.JAVA_HOME", Value: "/usr/lib/jvm/java-17"},
				}})
			}
		default:
			if b, ok := galleryBuilds[id]; ok {
				cmdtest.JSON(w, b)
			} else {
				cmdtest.JSON(w, galleryBuild)
			}
		}
	})
	ts.Handle("POST /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts.Handle("PUT /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts.Handle("DELETE /app/rest/builds/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	ts.Handle("GET /app/rest/changes", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		if strings.Contains(q, "45230") || strings.Contains(q, "id%3A2") {
			cmdtest.JSON(w, api.ChangeList{Count: 1, Change: []api.Change{
				{ID: 201, Version: "fed9876543ba", Username: "lchen", Comment: "Add OAuth2 handler",
					Date: tcTime(-3 * time.Hour), Files: &api.Files{File: []api.FileChange{
						{File: "src/auth/oauth.go", ChangeType: "added"}, {File: "src/auth/oauth_test.go", ChangeType: "added"},
					}}}}})
			return
		}
		cmdtest.JSON(w, api.ChangeList{Count: 3, Change: []api.Change{
			{ID: 101, Version: "abc1234def5", Username: "vtiulpin", Comment: "Fix memory leak in connection pool",
				Date: tcTime(-2 * time.Hour), Files: &api.Files{File: []api.FileChange{
					{File: "src/pool/connection.go", ChangeType: "edited"}, {File: "src/pool/connection_test.go", ChangeType: "edited"},
				}}},
			{ID: 100, Version: "def5678abc9", Username: "akowalski", Comment: "Add graceful shutdown handler",
				Date: tcTime(-5 * time.Hour), Files: &api.Files{File: []api.FileChange{
					{File: "src/shutdown/handler.go", ChangeType: "added"}, {File: "src/main.go", ChangeType: "edited"},
				}}},
			{ID: 99, Version: "789abcdef01", Username: "lchen", Comment: "Update CI pipeline configuration",
				Date: tcTime(-8 * time.Hour), Files: &api.Files{File: []api.FileChange{
					{File: ".teamcity.yml", ChangeType: "edited"},
				}}},
		}})
	})

	ts.Handle("GET /app/rest/testOccurrences", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		if strings.Contains(q, "45230") || strings.Contains(q, "id%3A2") {
			cmdtest.JSON(w, api.TestOccurrences{Count: 145, Passed: 140, Failed: 2, Ignored: 3,
				TestOccurrence: []api.TestOccurrence{
					{ID: "1", Name: "TestAuthHandler/invalid_token", Status: "FAILURE", Details: "Expected status 401, got 200"},
					{ID: "2", Name: "TestAuthHandler/expired_session", Status: "FAILURE", Details: "context deadline exceeded", NewFailure: true},
					{ID: "3", Name: "TestAuthHandler/valid_token", Status: "SUCCESS"},
					{ID: "4", Name: "TestConnectionPool/acquire", Status: "SUCCESS"},
					{ID: "5", Name: "TestConnectionPool/release", Status: "SUCCESS"},
					{ID: "6", Name: "TestMigration/rollback", Status: "UNKNOWN", Ignored: true},
				}})
			return
		}
		cmdtest.JSON(w, api.TestOccurrences{Count: 145, Passed: 145,
			TestOccurrence: []api.TestOccurrence{
				{ID: "1", Name: "TestAuthHandler/valid_token", Status: "SUCCESS"},
				{ID: "2", Name: "TestConnectionPool/acquire", Status: "SUCCESS"},
				{ID: "3", Name: "TestConnectionPool/release", Status: "SUCCESS"},
				{ID: "4", Name: "TestShutdown/graceful", Status: "SUCCESS"},
				{ID: "5", Name: "TestMigration/forward", Status: "SUCCESS"},
			}})
	})

	ts.Handle("GET /app/rest/problemOccurrences", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.RawQuery
		if strings.Contains(q, "45230") || strings.Contains(q, "id%3A2") {
			cmdtest.JSON(w, api.ProblemOccurrences{Count: 1, ProblemOccurrence: []api.ProblemOccurrence{
				{ID: "1", Type: "TC_COMPILATION_ERROR", Identity: "compilationError", Details: "Compilation failed with 3 errors"},
			}})
			return
		}
		cmdtest.JSON(w, api.ProblemOccurrences{Count: 0, ProblemOccurrence: []api.ProblemOccurrence{}})
	})

	ts.Handle("GET /app/messages", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildMessagesResponse{
			Messages: []api.BuildMessage{
				{ID: 1, Text: "Build started", Level: 1, Status: 1, Timestamp: tcTime(-2*time.Hour - 2*time.Minute)},
				{ID: 2, Text: "Step 1/3: Compile (go build)", Level: 1, Status: 1, Timestamp: tcTime(-2*time.Hour - 2*time.Minute + time.Second)},
				{ID: 3, Text: "Starting: go build -o bin/app ./cmd/app", Level: 2, Status: 1, Timestamp: tcTime(-2*time.Hour - 2*time.Minute + time.Second)},
				{ID: 4, Text: "Process exited with code 0", Level: 2, Status: 1, Timestamp: tcTime(-2*time.Hour - 2*time.Minute + 5*time.Second)},
				{ID: 5, Text: "Step 2/3: Test (go test)", Level: 1, Status: 1, Timestamp: tcTime(-2*time.Hour - 1*time.Minute)},
				{ID: 6, Text: "Starting: go test -race ./...", Level: 2, Status: 1, Timestamp: tcTime(-2*time.Hour - 1*time.Minute)},
				{ID: 7, Text: "ok  myapp/internal/auth  0.152s", Level: 2, Status: 1, Timestamp: tcTime(-2*time.Hour - 50*time.Second)},
				{ID: 8, Text: "ok  myapp/internal/pool  0.089s", Level: 2, Status: 1, Timestamp: tcTime(-2*time.Hour - 45*time.Second)},
				{ID: 9, Text: "ok  myapp/internal/handler  0.523s", Level: 2, Status: 1, Timestamp: tcTime(-2*time.Hour - 40*time.Second)},
				{ID: 10, Text: "Step 3/3: Package", Level: 1, Status: 1, Timestamp: tcTime(-2*time.Hour - 30*time.Second)},
				{ID: 11, Text: "Build finished", Level: 1, Status: 1, Timestamp: tcTime(-2 * time.Hour)},
			},
			LastMessageIndex:    11,
			FocusIndex:          11,
			LastMessageIncluded: true,
		})
	})

	ts.Handle("GET /app/rest/agents/id:", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/compatibleBuildTypes"):
			cmdtest.JSON(w, api.BuildTypeList{Count: 5, BuildTypes: []api.BuildType{
				{ID: "MyApp_Build", Name: "Build", ProjectName: "My Application", ProjectID: "MyApp"},
				{ID: "MyApp_Test", Name: "Run Tests", ProjectName: "My Application", ProjectID: "MyApp"},
				{ID: "MyApp_Lint", Name: "Lint", ProjectName: "My Application", ProjectID: "MyApp"},
				{ID: "MyApp_IntTest", Name: "Integration Tests", ProjectName: "My Application", ProjectID: "MyApp"},
				{ID: "Infra_Validate", Name: "Validate", ProjectName: "Infrastructure", ProjectID: "Infrastructure"},
			}})
		case strings.Contains(path, "/incompatibleBuildTypes"):
			cmdtest.JSON(w, api.CompatibilityList{Count: 2, Compatibility: []api.Compatibility{
				{Compatible: false, BuildType: &api.BuildType{ID: "MyApp_Deploy", Name: "Deploy Staging", ProjectName: "My Application"},
					Reasons: &api.IncompatibleReasons{Reasons: []string{"Missing requirement: docker.server.version >= 20.0"}}},
				{Compatible: false, BuildType: &api.BuildType{ID: "Infra_Deploy", Name: "Infra Deploy", ProjectName: "Infrastructure"},
					Reasons: &api.IncompatibleReasons{Reasons: []string{"Missing requirement: terraform >= 1.5", "Missing requirement: aws-cli"}}},
			}})
		default:
			cmdtest.JSON(w, api.Agent{
				ID: 1, Name: "linux-agent-01", Connected: true, Enabled: true, Authorized: true,
				WebURL: ts.URL + "/agentDetails.html?id=1",
				Pool:   &api.Pool{ID: 0, Name: "Default"},
				Build: &api.Build{ID: 45229, Number: "830", Status: "", State: "running",
					BuildType: &api.BuildType{ID: "MyApp_Deploy", Name: "Deploy Staging"}},
			})
		}
	})
	ts.Handle("PUT /app/rest/agents/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	ts.Handle("GET /app/rest/agents/id:1/compatibleBuildTypes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.BuildTypeList{Count: 5, BuildTypes: []api.BuildType{
			{ID: "MyApp_Build", Name: "Build", ProjectName: "My Application", ProjectID: "MyApp"},
			{ID: "MyApp_Test", Name: "Run Tests", ProjectName: "My Application", ProjectID: "MyApp"},
			{ID: "MyApp_Lint", Name: "Lint", ProjectName: "My Application", ProjectID: "MyApp"},
			{ID: "MyApp_IntTest", Name: "Integration Tests", ProjectName: "My Application", ProjectID: "MyApp"},
			{ID: "Infra_Validate", Name: "Validate", ProjectName: "Infrastructure", ProjectID: "Infrastructure"},
		}})
	})
	ts.Handle("GET /app/rest/agents/id:1/incompatibleBuildTypes", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.CompatibilityList{Count: 2, Compatibility: []api.Compatibility{
			{Compatible: false, BuildType: &api.BuildType{ID: "MyApp_Deploy", Name: "Deploy Staging", ProjectName: "My Application"},
				Reasons: &api.IncompatibleReasons{Reasons: []string{"Missing requirement: docker.server.version >= 20.0"}}},
			{Compatible: false, BuildType: &api.BuildType{ID: "Infra_Deploy", Name: "Infra Deploy", ProjectName: "Infrastructure"},
				Reasons: &api.IncompatibleReasons{Reasons: []string{"Missing requirement: terraform >= 1.5", "Missing requirement: aws-cli"}}},
		}})
	})

	ts.Handle("GET /app/rest/agentPools/id:", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Pool{
			ID: 0, Name: "Default", MaxAgents: 0,
			Agents: &api.AgentList{Count: 3, Agents: []api.Agent{
				{ID: 1, Name: "linux-agent-01", Connected: true, Enabled: true, Authorized: true},
				{ID: 2, Name: "linux-agent-02", Connected: true, Enabled: true, Authorized: true},
				{ID: 6, Name: "linux-agent-03", Connected: false, Enabled: true, Authorized: true},
			}},
			Projects: &api.ProjectList{Count: 2, Projects: []api.Project{
				{ID: "_Root", Name: "Root project"},
				{ID: "MyApp", Name: "My Application"},
			}},
		})
	})

	projectDetailHandler := func(w http.ResponseWriter, r *http.Request) {
		id := cmdtest.ExtractID(r.URL.Path, "id:")
		if id == "" {
			trimmed := strings.TrimPrefix(r.URL.Path, "/app/rest/projects/")
			id, _, _ = strings.Cut(trimmed, "/")
		}
		if strings.Contains(r.URL.Path, "/parameters/") {
			cmdtest.JSON(w, api.Parameter{Name: "param1", Value: "value1"})
			return
		}
		if strings.Contains(r.URL.Path, "/parameters") {
			cmdtest.JSON(w, api.ParameterList{Count: 5, Property: []api.Parameter{
				{Name: "env.DEPLOY_ENV", Value: "staging"},
				{Name: "env.JAVA_HOME", Value: "/usr/lib/jvm/java-17"},
				{Name: "version.prefix", Value: "1.0"},
				{Name: "docker.registry", Value: "ghcr.io/myorg"},
				{Name: "system.teamcity.build.branch", Value: "main"},
			}})
			return
		}
		if strings.Contains(r.URL.Path, "/sshKeys") {
			cmdtest.JSON(w, api.SSHKeyList{SSHKey: []api.SSHKey{
				{Name: "deploy-key", Encrypted: false, PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."},
				{Name: "ci-key", Encrypted: false, PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5BBBB..."},
				{Name: "backup-key", Encrypted: true, PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQAB..."},
				{Name: "github-app", Encrypted: false, PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5CCCC..."},
				{Name: "staging-access", Encrypted: true, PublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQCC..."},
			}})
			return
		}
		if strings.Contains(r.URL.Path, "/projectFeatures") {
			cmdtest.JSON(w, api.ProjectFeatureList{Count: 5, ProjectFeature: []api.ProjectFeature{
				{ID: "PROJECT_EXT_1", Type: "OAuthProvider", Properties: &api.PropertyList{Property: []api.Property{{Name: "displayName", Value: "GitHub App"}, {Name: "providerType", Value: "GitHubApp"}}}},
				{ID: "PROJECT_EXT_2", Type: "OAuthProvider", Properties: &api.PropertyList{Property: []api.Property{{Name: "displayName", Value: "Docker Registry"}, {Name: "providerType", Value: "DockerRegistry"}}}},
				{ID: "PROJECT_EXT_3", Type: "OAuthProvider", Properties: &api.PropertyList{Property: []api.Property{{Name: "displayName", Value: "AWS Connection"}, {Name: "providerType", Value: "AWSConnection"}}}},
				{ID: "PROJECT_EXT_4", Type: "OAuthProvider", Properties: &api.PropertyList{Property: []api.Property{{Name: "displayName", Value: "Slack Notifier"}, {Name: "providerType", Value: "SlackConnection"}}}},
				{ID: "PROJECT_EXT_5", Type: "OAuthProvider", Properties: &api.PropertyList{Property: []api.Property{{Name: "displayName", Value: "Jira Cloud"}, {Name: "providerType", Value: "JiraCloud"}}}},
			}})
			return
		}
		if strings.Contains(r.URL.Path, "/secure/values") {
			cmdtest.Text(w, "secret-value")
			return
		}
		if strings.Contains(r.URL.Path, "/versionedSettings/config") {
			cmdtest.JSON(w, api.VersionedSettingsConfig{
				SynchronizationMode: "enabled", Format: "kotlin", BuildSettingsMode: "useFromVCS",
				VcsRootID: "MyApp_HttpsGithubComOrgRepoGit", SettingsPath: ".teamcity",
				AllowUIEditing: true, ShowSettingsChanges: true,
			})
			return
		}
		if strings.Contains(r.URL.Path, "/versionedSettings/status") {
			cmdtest.JSON(w, api.VersionedSettingsStatus{
				Type: "info", Message: "Settings are up to date",
				Timestamp: "Mon Jan 27 10:30:00 UTC 2025", DslOutdated: false,
			})
			return
		}
		cmdtest.JSON(w, api.Project{
			ID: id, Name: "My Application", ParentProjectID: "_Root", Description: "Main product monorepo",
			WebURL: ts.URL + "/project.html?projectId=" + id,
		})
	}
	ts.Handle("GET /app/rest/projects/id:", projectDetailHandler)
	ts.Handle("GET /app/rest/projects/", projectDetailHandler)

	ts.Handle("POST /app/rest/projects/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/secure/tokens") {
			cmdtest.Text(w, "credentialsJSON:abc123")
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	ts.Handle("GET /app/rest/vcs-roots", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.VcsRootList{Count: 5, VcsRoot: []api.VcsRoot{
			{ID: "MyApp_MainRepo", Name: "Main Repo", VcsName: "jetbrains.git", Project: &api.Project{ID: "MyApp"}},
			{ID: "MyApp_ConfigRepo", Name: "Config Repo", VcsName: "jetbrains.git", Project: &api.Project{ID: "MyApp"}},
			{ID: "MyApp_DocsRepo", Name: "Documentation", VcsName: "jetbrains.git", Project: &api.Project{ID: "MyApp"}},
			{ID: "Shared_Libraries", Name: "Shared Libraries", VcsName: "jetbrains.git", Project: &api.Project{ID: "_Root"}},
			{ID: "Legacy_SVN", Name: "Legacy Codebase", VcsName: "svn", Project: &api.Project{ID: "MyApp"}},
		}})
	})
	ts.Handle("GET /app/rest/vcs-roots/id:", func(w http.ResponseWriter, r *http.Request) {
		id := cmdtest.ExtractID(r.URL.Path, "id:")
		cmdtest.JSON(w, api.VcsRoot{
			ID: id, Name: "Main Repo", VcsName: "jetbrains.git",
			Project: &api.Project{ID: "MyApp", Name: "My Application"},
			Href:    "/app/rest/vcs-roots/id:" + id,
			Properties: &api.PropertyList{Property: []api.Property{
				{Name: "url", Value: "https://github.com/jetbrains/teamcity-cli.git"},
				{Name: "branch", Value: "refs/heads/main"},
				{Name: "authMethod", Value: "TEAMCITY_SSH_KEY"},
				{Name: "teamcitySshKey", Value: "deploy-key"},
			}},
		})
	})

	ts.Handle("GET /app/rest/cloud/profiles", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.CloudProfileList{Count: 5, Profiles: []api.CloudProfile{
			{ID: "aws-prod", Name: "AWS Production", CloudProviderID: "amazon", Project: &api.Project{ID: "MyApp"}},
			{ID: "aws-staging", Name: "AWS Staging", CloudProviderID: "amazon", Project: &api.Project{ID: "MyApp"}},
			{ID: "azure-eu", Name: "Azure EU", CloudProviderID: "azure", Project: &api.Project{ID: "MyApp"}},
			{ID: "gcp-us", Name: "GCP US Central", CloudProviderID: "google", Project: &api.Project{ID: "MyApp"}},
			{ID: "k8s-local", Name: "Kubernetes On-Prem", CloudProviderID: "kubernetes", Project: &api.Project{ID: "MyApp"}},
		}})
	})
	ts.Handle("GET /app/rest/cloud/profiles/", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.CloudProfile{
			ID: "aws-prod", Name: "AWS Production", CloudProviderID: "amazon",
			Project: &api.Project{ID: "MyApp", Name: "My Application"},
		})
	})
	ts.Handle("GET /app/rest/cloud/images", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.CloudImageList{Count: 5, Images: []api.CloudImage{
			{ID: "img-1", Name: "ubuntu-22-large", Profile: &api.CloudProfile{ID: "aws-prod", Name: "AWS Production"}},
			{ID: "img-2", Name: "ubuntu-22-xlarge", Profile: &api.CloudProfile{ID: "aws-prod", Name: "AWS Production"}},
			{ID: "img-3", Name: "windows-2022", Profile: &api.CloudProfile{ID: "azure-eu", Name: "Azure EU"}},
			{ID: "img-4", Name: "debian-12-small", Profile: &api.CloudProfile{ID: "aws-staging", Name: "AWS Staging"}},
			{ID: "img-5", Name: "ubuntu-22-gpu", Profile: &api.CloudProfile{ID: "gcp-us", Name: "GCP US Central"}},
		}})
	})
	ts.Handle("GET /app/rest/cloud/instances", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.CloudInstanceList{Count: 5, Instances: []api.CloudInstance{
			{ID: "i-0a24f...", Name: "agent-cloud-1", State: "running", Image: &api.CloudImage{Name: "ubuntu-22-large"}, Agent: &api.Agent{ID: 10, Name: "agent-cloud-1"}},
			{ID: "i-0b35e...", Name: "agent-cloud-2", State: "running", Image: &api.CloudImage{Name: "ubuntu-22-large"}, Agent: &api.Agent{ID: 11, Name: "agent-cloud-2"}},
			{ID: "i-0c46d...", Name: "agent-cloud-3", State: "starting", Image: &api.CloudImage{Name: "ubuntu-22-xlarge"}},
			{ID: "vm-eu-01", Name: "agent-azure-1", State: "running", Image: &api.CloudImage{Name: "windows-2022"}, Agent: &api.Agent{ID: 12, Name: "agent-azure-1"}},
			{ID: "gke-node-5", Name: "agent-k8s-5", State: "running", Image: &api.CloudImage{Name: "ubuntu-22-gpu"}, Agent: &api.Agent{ID: 13, Name: "agent-k8s-5"}},
		}})
	})

	ts.Handle("POST /app/rest/projects/id:MyApp/sshKeys/generated", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("keyName")
		cmdtest.JSON(w, api.SSHKey{
			Name:      name,
			PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5_generated_key",
		})
	})
	ts.Handle("DELETE /app/rest/projects/id:MyApp/sshKeys/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	ts.Handle("POST /app/pipeline", func(w http.ResponseWriter, r *http.Request) {
		cmdtest.JSON(w, api.Pipeline{ID: "MyApp_Onboarding", Name: "Onboarding",
			ParentProject: &api.ProjectRef{ID: "MyApp", Name: "My Application"},
			HeadBuildType: &api.BuildTypeRef{ID: "MyApp_Onboarding"},
			WebURL:        ts.URL + "/pipeline/MyApp_Onboarding",
		})
	})

	ts.Handle("GET /app/rest/pipelines/id:", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/yaml") {
			cmdtest.Text(w, "version: v1.0\njobs:\n  build:\n    steps:\n      - script: go build ./...\n  test:\n    needs: [build]\n    steps:\n      - script: go test -race ./...\n")
			return
		}
		cmdtest.JSON(w, api.Pipeline{ID: "MyApp_CI", Name: "CI",
			ParentProject: &api.ProjectRef{ID: "MyApp", Name: "My Application"},
			HeadBuildType: &api.BuildTypeRef{ID: "MyApp_CI"},
			Jobs:          &api.PipelineJobs{Count: 2, Job: []api.PipelineJob{{ID: "build", Name: "Build"}, {ID: "test", Name: "Test"}}}})
	})
	ts.Handle("PUT /app/rest/pipelines/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	ts.Handle("DELETE /app/rest/pipelines/id:", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	ts.Handle("GET /app/rest/buildTypes/id:", func(w http.ResponseWriter, r *http.Request) {
		id := cmdtest.ExtractID(r.URL.Path, "id:")
		if strings.Contains(r.URL.Path, "/snapshot-dependencies") {
			var deps []api.SnapshotDependency
			switch id {
			case "MyApp_Build":
				deps = []api.SnapshotDependency{{ID: "snap-build-lint", SourceBuildType: &api.BuildType{ID: "MyApp_Lint", Name: "Lint"}}}
			case "MyApp_Test", "MyApp_IntTest":
				deps = []api.SnapshotDependency{{ID: "snap-dep", SourceBuildType: &api.BuildType{ID: "MyApp_Build", Name: "Build"}}}
			case "MyApp_Deploy":
				deps = []api.SnapshotDependency{
					{ID: "snap-dep-test", SourceBuildType: &api.BuildType{ID: "MyApp_Test", Name: "Run Tests"}},
					{ID: "snap-dep-int", SourceBuildType: &api.BuildType{ID: "MyApp_IntTest", Name: "Integration Tests"}},
				}
			}
			cmdtest.JSON(w, api.SnapshotDependencyList{Count: len(deps), SnapshotDependency: deps})
			return
		}
		if strings.Contains(r.URL.Path, "/parameters/") {
			cmdtest.JSON(w, api.Parameter{Name: "param1", Value: "value1"})
			return
		}
		if strings.Contains(r.URL.Path, "/parameters") {
			cmdtest.JSON(w, api.ParameterList{Count: 5, Property: []api.Parameter{
				{Name: "env.JAVA_HOME", Value: "/usr/lib/jvm/java-17"},
				{Name: "env.GOVERSION", Value: "1.26"},
				{Name: "version", Value: "1.0.0"},
				{Name: "deploy.target", Value: "staging"},
				{Name: "build.parallel", Value: "4"},
			}})
			return
		}
		names := map[string]string{"MyApp_Build": "Build", "MyApp_Test": "Run Tests", "MyApp_Deploy": "Deploy Staging", "MyApp_IntTest": "Integration Tests", "MyApp_Lint": "Lint"}
		name := names[id]
		if name == "" {
			name = id
		}
		cmdtest.JSON(w, api.BuildType{
			ID: id, Name: name, ProjectID: "MyApp", ProjectName: "My Application",
			WebURL: ts.URL + "/viewType.html?buildTypeId=" + id,
		})
	})

	return ts
}
