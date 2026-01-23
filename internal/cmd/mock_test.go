package cmd_test

import (
	"io"
	"testing"

	"github.com/tiulpin/teamcity-cli/internal/api"
	"github.com/tiulpin/teamcity-cli/internal/api/mocks"
	"github.com/tiulpin/teamcity-cli/internal/cmd"
	"github.com/tiulpin/teamcity-cli/internal/config"
	tcerrors "github.com/tiulpin/teamcity-cli/internal/errors"
)

// mockClient is a reusable mock client with sensible defaults for testing
func mockClient() *mocks.ClientInterfaceMock {
	return &mocks.ClientInterfaceMock{
		// Server
		GetServerFunc: func() (*api.Server, error) {
			return &api.Server{
				Version:      " (build 197398)",
				VersionMajor: 2025,
				VersionMinor: 7,
				BuildNumber:  "197398",
				WebURL:       "http://mock.teamcity.test",
			}, nil
		},
		ServerVersionFunc: func() (*api.Server, error) {
			return &api.Server{
				Version:      " (build 197398)",
				VersionMajor: 2025,
				VersionMinor: 7,
				BuildNumber:  "197398",
				WebURL:       "http://mock.teamcity.test",
			}, nil
		},
		CheckVersionFunc: func() error { return nil },
		SupportsFeatureFunc: func(feature string) bool {
			return true
		},

		// Users
		GetCurrentUserFunc: func() (*api.User, error) {
			return &api.User{
				ID:       1,
				Username: "admin",
				Name:     "Administrator",
			}, nil
		},
		GetUserFunc: func(username string) (*api.User, error) {
			return &api.User{
				ID:       1,
				Username: username,
				Name:     "Test User",
			}, nil
		},
		UserExistsFunc: func(username string) bool {
			return true
		},
		CreateUserFunc: func(req api.CreateUserRequest) (*api.User, error) {
			return &api.User{ID: 1, Username: req.Username}, nil
		},
		CreateAPITokenFunc: func(name string) (*api.Token, error) {
			return &api.Token{Name: name, Value: "test-token"}, nil
		},
		DeleteAPITokenFunc: func(name string) error { return nil },

		// Projects
		GetProjectsFunc: func(opts api.ProjectsOptions) (*api.ProjectList, error) {
			return &api.ProjectList{
				Count: 2,
				Projects: []api.Project{
					{ID: "_Root", Name: "Root project", ParentProjectID: ""},
					{ID: "TestProject", Name: "Test Project", ParentProjectID: "_Root"},
				},
			}, nil
		},
		GetProjectFunc: func(id string) (*api.Project, error) {
			if id == "NonExistentProject123456" {
				return nil, tcerrors.NotFound("project", id)
			}
			return &api.Project{
				ID:              id,
				Name:            "Test Project",
				ParentProjectID: "_Root",
				WebURL:          "http://localhost/project.html?projectId=" + id,
			}, nil
		},
		CreateProjectFunc: func(req api.CreateProjectRequest) (*api.Project, error) {
			return &api.Project{ID: req.ID, Name: req.Name}, nil
		},
		ProjectExistsFunc: func(id string) bool {
			return id != "NonExistentProject123456"
		},
		CreateSecureTokenFunc: func(projectID, value string) (string, error) {
			return "credentialsJSON:abc123", nil
		},
		GetSecureValueFunc: func(projectID, token string) (string, error) {
			return "secret-value", nil
		},

		// Build Types (Jobs)
		GetBuildTypesFunc: func(opts api.BuildTypesOptions) (*api.BuildTypeList, error) {
			return &api.BuildTypeList{
				Count: 1,
				BuildTypes: []api.BuildType{
					{ID: "TestProject_Build", Name: "Build", ProjectID: "TestProject"},
				},
			}, nil
		},
		GetBuildTypeFunc: func(id string) (*api.BuildType, error) {
			if id == "NonExistentJob123456" {
				return nil, tcerrors.NotFound("job", id)
			}
			return &api.BuildType{
				ID:        id,
				Name:      "Build",
				ProjectID: "TestProject",
				WebURL:    "http://localhost/viewType.html?buildTypeId=" + id,
			}, nil
		},
		SetBuildTypePausedFunc: func(id string, paused bool) error { return nil },
		CreateBuildTypeFunc: func(projectID string, req api.CreateBuildTypeRequest) (*api.BuildType, error) {
			return &api.BuildType{ID: req.ID, Name: req.Name, ProjectID: projectID}, nil
		},
		BuildTypeExistsFunc: func(id string) bool {
			return id != "NonExistentJob123456"
		},
		CreateBuildStepFunc:     func(buildTypeID string, step api.BuildStep) error { return nil },
		SetBuildTypeSettingFunc: func(buildTypeID, setting, value string) error { return nil },

		// Builds (Runs)
		GetBuildsFunc: func(opts api.BuildsOptions) (*api.BuildList, error) {
			return &api.BuildList{
				Count: 1,
				Builds: []api.Build{
					{
						ID:          1,
						Number:      "1",
						Status:      "SUCCESS",
						State:       "finished",
						BuildTypeID: "TestProject_Build",
						StartDate:   "20240101T120000+0000",
						FinishDate:  "20240101T120100+0000",
						WebURL:      "http://localhost/viewLog.html?buildId=1",
					},
				},
			}, nil
		},
		GetBuildFunc: func(ref string) (*api.Build, error) {
			if ref == "999999999" {
				return nil, tcerrors.NotFound("run", ref)
			}
			return &api.Build{
				ID:          1,
				Number:      "1",
				Status:      "SUCCESS",
				State:       "finished",
				BuildTypeID: "TestProject_Build",
				StartDate:   "20240101T120000+0000",
				FinishDate:  "20240101T120100+0000",
				WebURL:      "http://localhost/viewLog.html?buildId=1",
			}, nil
		},
		ResolveBuildIDFunc: func(ref string) (string, error) {
			return ref, nil
		},
		RunBuildFunc: func(buildTypeID string, opts api.RunBuildOptions) (*api.Build, error) {
			return &api.Build{
				ID:          100,
				Number:      "100",
				Status:      "SUCCESS",
				State:       "queued",
				BuildTypeID: buildTypeID,
				WebURL:      "http://localhost/viewLog.html?buildId=100",
			}, nil
		},
		CancelBuildFunc: func(buildID string, comment string) error { return nil },
		GetBuildLogFunc: func(buildID string) (string, error) {
			return "Build log content\nStep 1: Success\nStep 2: Success\n", nil
		},
		PinBuildFunc:     func(buildID string, comment string) error { return nil },
		UnpinBuildFunc:   func(buildID string) error { return nil },
		AddBuildTagsFunc: func(buildID string, tags []string) error { return nil },
		GetBuildTagsFunc: func(buildID string) (*api.TagList, error) {
			return &api.TagList{Tag: []api.Tag{
				{Name: "test-tag"},
				{Name: "cli-test-tag"},
				{Name: "another-tag"},
			}}, nil
		},
		RemoveBuildTagFunc:  func(buildID string, tag string) error { return nil },
		SetBuildCommentFunc: func(buildID string, comment string) error { return nil },
		GetBuildCommentFunc: func(buildID string) (string, error) {
			return "Test comment", nil
		},
		DeleteBuildCommentFunc: func(buildID string) error { return nil },
		GetBuildChangesFunc: func(buildID string) (*api.ChangeList, error) {
			return &api.ChangeList{
				Count: 1,
				Change: []api.Change{
					{
						ID:       1,
						Version:  "abc123",
						Username: "developer",
						Date:     "20240101T120000+0000",
						Comment:  "Test commit",
					},
				},
			}, nil
		},
		GetBuildTestsFunc: func(buildID string, failedOnly bool, limit int) (*api.TestOccurrences, error) {
			return &api.TestOccurrences{
				Count:  1,
				Passed: 1,
				Failed: 0,
				TestOccurrence: []api.TestOccurrence{
					{
						ID:     "1",
						Name:   "TestExample",
						Status: "SUCCESS",
					},
				},
			}, nil
		},

		// Artifacts
		GetArtifactsFunc: func(buildID string) (*api.Artifacts, error) {
			return &api.Artifacts{
				Count: 1,
				File:  []api.Artifact{{Name: "test.txt", Size: 100}},
			}, nil
		},
		DownloadArtifactFunc: func(buildID, artifactPath string) ([]byte, error) {
			return []byte("artifact content"), nil
		},

		// Build Queue
		GetBuildQueueFunc: func(opts api.QueueOptions) (*api.BuildQueue, error) {
			return &api.BuildQueue{
				Count:  0,
				Builds: []api.QueuedBuild{},
			}, nil
		},
		RemoveFromQueueFunc:        func(id string) error { return nil },
		SetQueuedBuildPositionFunc: func(buildID string, position int) error { return nil },
		MoveQueuedBuildToTopFunc:   func(buildID string) error { return nil },
		ApproveQueuedBuildFunc:     func(buildID string) error { return nil },
		GetQueuedBuildApprovalInfoFunc: func(buildID string) (*api.ApprovalInfo, error) {
			return &api.ApprovalInfo{
				Status:        "waitingForApproval",
				CanBeApproved: true,
			}, nil
		},

		// Parameters
		GetProjectParametersFunc: func(projectID string) (*api.ParameterList, error) {
			return &api.ParameterList{
				Count:    1,
				Property: []api.Parameter{{Name: "param1", Value: "value1"}},
			}, nil
		},
		GetProjectParameterFunc: func(projectID, name string) (*api.Parameter, error) {
			return &api.Parameter{Name: name, Value: "value1"}, nil
		},
		SetProjectParameterFunc:    func(projectID, name, value string, secure bool) error { return nil },
		DeleteProjectParameterFunc: func(projectID, name string) error { return nil },
		GetBuildTypeParametersFunc: func(buildTypeID string) (*api.ParameterList, error) {
			return &api.ParameterList{
				Count:    1,
				Property: []api.Parameter{{Name: "param1", Value: "value1"}},
			}, nil
		},
		GetBuildTypeParameterFunc: func(buildTypeID, name string) (*api.Parameter, error) {
			return &api.Parameter{Name: name, Value: "value1"}, nil
		},
		SetBuildTypeParameterFunc:    func(buildTypeID, name, value string, secure bool) error { return nil },
		DeleteBuildTypeParameterFunc: func(buildTypeID, name string) error { return nil },
		GetParameterValueFunc: func(path string) (string, error) {
			return "parameter-value", nil
		},

		// Agents
		GetAgentsFunc: func(opts api.AgentsOptions) (*api.AgentList, error) {
			return &api.AgentList{Count: 0, Agents: []api.Agent{}}, nil
		},
		AuthorizeAgentFunc: func(id int, authorized bool) error { return nil },

		// Raw API access
		RawRequestFunc: func(method, path string, body io.Reader, headers map[string]string) (*api.RawResponse, error) {
			// Return sensible defaults for common endpoints
			switch {
			case path == "/app/rest/server":
				return &api.RawResponse{
					StatusCode: 200,
					Body:       []byte(`{"version":" (build 197398)","versionMajor":2025,"versionMinor":7,"buildNumber":"197398","webUrl":"http://mock.teamcity.test"}`),
				}, nil
			case path == "/app/rest/projects":
				return &api.RawResponse{
					StatusCode: 200,
					Body:       []byte(`{"count":1,"project":[{"id":"TestProject","name":"Test"}]}`),
				}, nil
			default:
				return &api.RawResponse{
					StatusCode: 200,
					Body:       []byte(`{}`),
				}, nil
			}
		},
	}
}

// setupMockClient sets up a mock client for testing and returns a cleanup function.
func setupMockClient(t *testing.T) *mocks.ClientInterfaceMock {
	t.Helper()

	mock := mockClient()

	// Store original and set up mock
	original := cmd.GetClientFunc
	cmd.GetClientFunc = func() (api.ClientInterface, error) {
		return mock, nil
	}

	// Set up minimal config
	t.Setenv("TEAMCITY_URL", "http://mock.teamcity.test")
	t.Setenv("TEAMCITY_TOKEN", "mock-token")
	config.Init()

	// Restore on cleanup
	t.Cleanup(func() {
		cmd.GetClientFunc = original
	})

	return mock
}
