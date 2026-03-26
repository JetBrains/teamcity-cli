package run

import (
	"context"
	"fmt"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlattenArtifacts(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name      string
		artifacts []api.Artifact
		wantNames []string
		wantSize  int64
	}{
		{
			name:      "empty list",
			artifacts: nil,
			wantNames: nil,
			wantSize:  0,
		},
		{
			name: "flat files",
			artifacts: []api.Artifact{
				{Name: "a.txt", Size: 100},
				{Name: "b.txt", Size: 200},
			},
			wantNames: []string{"a.txt", "b.txt"},
			wantSize:  300,
		},
		{
			name: "nested directory",
			artifacts: []api.Artifact{
				{Name: "dir", Children: &api.Artifacts{
					File: []api.Artifact{
						{Name: "inner.txt", Size: 50},
					},
				}},
				{Name: "root.txt", Size: 10},
			},
			wantNames: []string{"dir/inner.txt", "root.txt"},
			wantSize:  60,
		},
		{
			name: "deeply nested",
			artifacts: []api.Artifact{
				{Name: "a", Children: &api.Artifacts{
					File: []api.Artifact{
						{Name: "b", Children: &api.Artifacts{
							File: []api.Artifact{
								{Name: "c.txt", Size: 1},
							},
						}},
					},
				}},
			},
			wantNames: []string{"a/b/c.txt"},
			wantSize:  1,
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, size := flattenArtifacts(tc.artifacts, "")
			assert.Equal(t, tc.wantSize, size)
			var names []string
			for _, a := range got {
				names = append(names, a.Name)
			}
			assert.Equal(t, tc.wantNames, names)
		})
	}
}

type mockArtifactClient struct {
	api.ClientInterface
	responses map[string]*api.Artifacts
}

func (m *mockArtifactClient) GetArtifacts(buildID, path string) (*api.Artifacts, error) {
	key := fmt.Sprintf("%s:%s", buildID, path)
	resp, ok := m.responses[key]
	if !ok {
		return &api.Artifacts{}, nil
	}
	return resp, nil
}

func TestFetchAllArtifacts(T *testing.T) {
	T.Parallel()

	contentRef := new(api.Content{Href: "/download"})

	T.Run("flat files", func(t *testing.T) {
		t.Parallel()
		mock := &mockArtifactClient{responses: map[string]*api.Artifacts{
			"1:": {Count: 2, File: []api.Artifact{
				{Name: "a.txt", Size: 10, Content: contentRef},
				{Name: "b.txt", Size: 20, Content: contentRef},
			}},
		}}

		got, size, err := fetchAllArtifacts(t.Context(), mock, "1", "")
		require.NoError(t, err)
		assert.Equal(t, int64(30), size)
		assert.Len(t, got, 2)
		assert.Equal(t, "a.txt", got[0].Name)
		assert.Equal(t, "b.txt", got[1].Name)
	})

	T.Run("recursive directories", func(t *testing.T) {
		t.Parallel()
		mock := &mockArtifactClient{responses: map[string]*api.Artifacts{
			"1:": {Count: 2, File: []api.Artifact{
				{Name: "root.txt", Size: 5, Content: contentRef},
				{Name: "subdir"},
			}},
			"1:subdir": {Count: 1, File: []api.Artifact{
				{Name: "nested.txt", Size: 15, Content: contentRef},
			}},
		}}

		got, size, err := fetchAllArtifacts(t.Context(), mock, "1", "")
		require.NoError(t, err)
		assert.Equal(t, int64(20), size)
		require.Len(t, got, 2)
		assert.Equal(t, "root.txt", got[0].Name)
		assert.Equal(t, "subdir/nested.txt", got[1].Name)
	})

	T.Run("with base path", func(t *testing.T) {
		t.Parallel()
		mock := &mockArtifactClient{responses: map[string]*api.Artifacts{
			"1:build": {Count: 1, File: []api.Artifact{
				{Name: "app.jar", Size: 100, Content: contentRef},
			}},
		}}

		got, size, err := fetchAllArtifacts(t.Context(), mock, "1", "build")
		require.NoError(t, err)
		assert.Equal(t, int64(100), size)
		require.Len(t, got, 1)
		assert.Equal(t, "build/app.jar", got[0].Name)
	})

	T.Run("respects context cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		mock := &mockArtifactClient{responses: map[string]*api.Artifacts{
			"1:": {Count: 1, File: []api.Artifact{
				{Name: "a.txt", Size: 10, Content: contentRef},
			}},
		}}

		_, _, err := fetchAllArtifacts(ctx, mock, "1", "")
		assert.ErrorIs(t, err, context.Canceled)
	})
}
