package run

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRunListRequestFavorites(T *testing.T) {
	req, err := resolveRunListRequest(nil, &runListOptions{
		favorites: true,
		limit:     30,
	}, nil)
	require.NoError(T, err)

	assert.True(T, req.builds.Favorites)
	assert.Equal(T, "/favorite/builds", req.webPath)
	assert.Equal(T, "No favorite runs found", req.emptyMsg)
	assert.Equal(T, 30, req.builds.Limit)
}

func TestResolveRunListRequestAtMeUsesConfigUser(T *testing.T) {
	oldConfigUser := runListConfigCurrentUserFn
	T.Cleanup(func() {
		runListConfigCurrentUserFn = oldConfigUser
	})

	runListConfigCurrentUserFn = func() string { return "alice" }

	req, err := resolveRunListRequest(nil, &runListOptions{
		user:  "@me",
		limit: 30,
	}, nil)
	require.NoError(T, err)

	assert.Equal(T, "alice", req.builds.User)
}

func TestResolveRunListRequestAtMeFallsBackToAPIUser(T *testing.T) {
	oldConfigUser := runListConfigCurrentUserFn
	oldAPIUser := runListAPICurrentUserFn
	T.Cleanup(func() {
		runListConfigCurrentUserFn = oldConfigUser
		runListAPICurrentUserFn = oldAPIUser
	})

	runListConfigCurrentUserFn = func() string { return "" }
	runListAPICurrentUserFn = func(api.ClientInterface) (*api.User, error) {
		return &api.User{Username: "bob"}, nil
	}

	req, err := resolveRunListRequest(nil, &runListOptions{
		user:  "@me",
		limit: 30,
	}, nil)
	require.NoError(T, err)

	assert.Equal(T, "bob", req.builds.User)
}

func TestResolveRunListRequestBranchThisRequiresGitRepo(T *testing.T) {
	oldIsGitRepo := runListIsGitRepoFn
	T.Cleanup(func() { runListIsGitRepoFn = oldIsGitRepo })

	runListIsGitRepoFn = func() bool { return false }

	_, err := resolveRunListRequest(nil, &runListOptions{
		branch: "@this",
		limit:  30,
	}, nil)
	require.Error(T, err)
	assert.Contains(T, err.Error(), "git repository")
}
