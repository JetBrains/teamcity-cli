package project

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
)

func TestBuildTestRequestFromRoot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		root           *api.VcsRoot
		wantConnection string
		wantMissing    bool
		wantPrivate    bool
	}{
		{
			name: "anonymous",
			root: &api.VcsRoot{
				VcsName: "jetbrains.git",
				Properties: &api.PropertyList{Property: []api.Property{
					{Name: "url", Value: "https://github.com/owner/repo.git"},
					{Name: "authMethod", Value: "ANONYMOUS"},
				}},
			},
		},
		{
			name: "ssh-key sets isPrivate",
			root: &api.VcsRoot{
				VcsName: "jetbrains.git",
				Properties: &api.PropertyList{Property: []api.Property{
					{Name: "url", Value: "git@github.com:owner/repo.git"},
					{Name: "authMethod", Value: "TEAMCITY_SSH_KEY"},
					{Name: "teamcitySshKey", Value: "deploy"},
				}},
			},
			wantPrivate: true,
		},
		{
			name: "legacy pipelines.connectionId",
			root: &api.VcsRoot{
				VcsName: "jetbrains.git",
				Properties: &api.PropertyList{Property: []api.Property{
					{Name: "url", Value: "https://github.com/owner/repo.git"},
					{Name: "pipelines.connectionId", Value: "PROJECT_EXT_10"},
				}},
			},
			wantConnection: "PROJECT_EXT_10",
		},
		{
			name: "top-level connectionId wins over pipelines.connectionId",
			root: &api.VcsRoot{
				VcsName:      "jetbrains.git",
				ConnectionID: "PROJECT_EXT_77",
				Properties: &api.PropertyList{Property: []api.Property{
					{Name: "url", Value: "https://github.com/owner/repo.git"},
					{Name: "pipelines.connectionId", Value: "PROJECT_EXT_10"},
				}},
			},
			wantConnection: "PROJECT_EXT_77",
		},
		{
			name: "ACCESS_TOKEN root with no recoverable connection id flags missing",
			root: &api.VcsRoot{
				VcsName: "jetbrains.git",
				Properties: &api.PropertyList{Property: []api.Property{
					{Name: "url", Value: "https://github.com/owner/repo.git"},
					{Name: "authMethod", Value: "ACCESS_TOKEN"},
					{Name: "username", Value: "oauth2"},
					{Name: "tokenId", Value: "tc_token_id:CID_abc:-1:uuid"},
				}},
			},
			wantMissing: true,
		},
		{
			name: "ACCESS_TOKEN root with top-level connectionId is not missing",
			root: &api.VcsRoot{
				VcsName:      "jetbrains.git",
				ConnectionID: "PROJECT_EXT_77",
				Properties: &api.PropertyList{Property: []api.Property{
					{Name: "url", Value: "https://github.com/owner/repo.git"},
					{Name: "authMethod", Value: "ACCESS_TOKEN"},
					{Name: "username", Value: "oauth2"},
					{Name: "tokenId", Value: "tc_token_id:CID_abc:-1:uuid"},
				}},
			},
			wantConnection: "PROJECT_EXT_77",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, missing := buildTestRequestFromRoot(tc.root)
			assert.Equal(t, tc.wantConnection, req.ConnectionID)
			assert.Equal(t, tc.wantMissing, missing)
			assert.Equal(t, tc.wantPrivate, req.IsPrivate)
		})
	}
}
