package api

import (
	"cmp"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBuildRevisions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		opts    RunBuildOptions
		want    []Revision
		wantErr string
		heads   bool
		state   string // overrides the default repository state response
	}{
		{name: "legacy pins every root", opts: RunBuildOptions{Revision: "abc", Branch: "feature"}, want: []Revision{
			{Version: "abc", VcsBranchName: "refs/heads/feature", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}},
			{Version: "abc", VcsBranchName: "refs/heads/feature", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "B"}},
		}},
		{name: "bare revision without branch uses each root's default branch", opts: RunBuildOptions{Revision: "abc"}, heads: true, want: []Revision{
			{Version: "abc", VcsBranchName: "refs/heads/main", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}},
			{Version: "abc", VcsBranchName: "refs/heads/master", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "B"}},
		}},
		{name: "pin only one root without inheriting logical branch", opts: RunBuildOptions{Branch: "unrelated", Revisions: []RevisionSpec{{VcsRootID: "A", Version: "abc"}}}, heads: true, want: []Revision{
			{Version: "abc", VcsBranchName: "refs/heads/main", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}},
		}},
		{name: "default branch name is sent as is", opts: RunBuildOptions{Revisions: []RevisionSpec{{VcsRootID: "A", Version: "abc"}}}, heads: true,
			state: `{"vcs-root-instance":[{"vcs-root-id":"A","repositoryState":{"branch":[{"name":"default","version":"h1","default":true}]}}]}`,
			want:  []Revision{{Version: "abc", VcsBranchName: "default", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}}}},
		{name: "empty state falls back to Git branch property", opts: RunBuildOptions{Revision: "abc"}, heads: true,
			state: `{"vcs-root-instance":[{"vcs-root-id":"A","vcsName":"jetbrains.git","properties":{"property":[{"name":"branch","value":"main"}]},"repositoryState":{"branch":[{"name":"","default":true}]}},{"vcs-root-id":"B","vcsName":"perforce","properties":{"property":[{"name":"branch","value":"main"}]},"repositoryState":{"branch":[{"name":"","default":true}]}}]}`,
			want: []Revision{
				{Version: "abc", VcsBranchName: "refs/heads/main", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}},
				{Version: "abc", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "B"}},
			}},
		{name: "root without default branch keeps no branch", opts: RunBuildOptions{Revision: "abc"}, heads: true,
			state: `{"vcs-root-instance":[{"vcs-root-id":"A","repositoryState":{"branch":[{"name":"","version":"42","default":true}]}}]}`,
			want: []Revision{
				{Version: "abc", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}},
				{Version: "abc", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "B"}},
			}},
		{name: "explicit branch and tag", opts: RunBuildOptions{Revisions: []RevisionSpec{{VcsRootID: "A", Version: "abc", Branch: "feature"}, {VcsRootID: "B", Version: "def", Branch: "refs/tags/v1"}}}, want: []Revision{
			{Version: "abc", VcsBranchName: "refs/heads/feature", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}},
			{Version: "def", VcsBranchName: "refs/tags/v1", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "B"}},
		}},
		{name: "two branch heads in one request", opts: RunBuildOptions{Revisions: []RevisionSpec{{VcsRootID: "A", Branch: "main"}, {VcsRootID: "B", Branch: "refs/tags/v1"}}}, heads: true, want: []Revision{
			{Version: "aaa", VcsBranchName: "refs/heads/main", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}},
			{Version: "bbb", VcsBranchName: "refs/tags/v1", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "B"}},
		}},
		{name: "mixed explicit and head", opts: RunBuildOptions{Revisions: []RevisionSpec{{VcsRootID: "A", Version: "older"}, {VcsRootID: "B", Branch: "refs/pull/12/merge"}}}, heads: true, want: []Revision{
			{Version: "older", VcsBranchName: "refs/heads/main", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "A"}},
			{Version: "pr", VcsBranchName: "refs/pull/12/merge", VcsRootInstance: &VcsRootInstanceRef{VcsRootID: "B"}},
		}},
		{name: "unknown root", opts: RunBuildOptions{Revisions: []RevisionSpec{{VcsRootID: "typo", Version: "abc"}}}, wantErr: "is not attached to Job"},
		{name: "duplicate root", opts: RunBuildOptions{Revisions: []RevisionSpec{{VcsRootID: "A", Version: "abc"}, {VcsRootID: "A", Version: "def"}}}, wantErr: "duplicate"},
		{name: "missing version and branch", opts: RunBuildOptions{Revisions: []RevisionSpec{{VcsRootID: "A"}}}, wantErr: "revision or branch required"},
		{name: "mixed legacy and keyed", opts: RunBuildOptions{Revision: "abc", Revisions: []RevisionSpec{{VcsRootID: "A", Version: "def"}}}, wantErr: "cannot mix"},
		{name: "unfetched branch", opts: RunBuildOptions{Revisions: []RevisionSpec{{VcsRootID: "A", Branch: "absent"}}}, heads: true, wantErr: "no fetched revision"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var captured TriggerBuildRequest
			posts, headRequests := 0, 0
			client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/app/rest/buildTypes/id:Job/vcs-root-entries":
					json.NewEncoder(w).Encode(VcsRootEntries{Count: 2, VcsRootEntry: []VcsRootEntry{{VcsRoot: &VcsRoot{ID: "A"}}, {VcsRoot: &VcsRoot{ID: "B"}}}})
				case "/app/rest/buildTypes/id:Job/vcs-root-instances":
					headRequests++
					assert.Equal(t, "vcs-root-instance(vcs-root-id,vcsName,properties(property(name,value)),repositoryState(branch(name,version,default)))", r.URL.Query().Get("fields"))
					w.Write([]byte(cmp.Or(tt.state, `{"vcs-root-instance":[{"vcs-root-id":"A","vcsName":"jetbrains.git","properties":{"property":[{"name":"branch","value":"refs/heads/other"}]},"repositoryState":{"branch":[{"name":"refs/heads/main","version":"aaa","default":true}]}},{"vcs-root-id":"B","repositoryState":{"branch":[{"name":"refs/heads/master","version":"ccc","default":true},{"name":"refs/tags/v1","version":"bbb"},{"name":"refs/pull/12/merge","version":"pr"}]}}]}`)))
				case "/app/rest/buildQueue":
					require.Equal(t, "POST", r.Method)
					posts++
					require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
					w.Write([]byte(`{"id":123}`))
				default:
					t.Errorf("unexpected request: %s", r.URL)
					http.NotFound(w, r)
				}
			})
			original, err := json.Marshal(tt.opts)
			require.NoError(t, err)
			_, err = client.RunBuild("Job", tt.opts)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Zero(t, posts)
			} else {
				require.NoError(t, err)
				require.NotNil(t, captured.Revisions)
				assert.Equal(t, tt.want, captured.Revisions.Revision)
				assert.Equal(t, 1, posts)
			}
			expectedHeads := 0
			if tt.heads {
				expectedHeads = 1
			}
			assert.Equal(t, expectedHeads, headRequests)
			after, err := json.Marshal(tt.opts)
			require.NoError(t, err)
			assert.JSONEq(t, string(original), string(after), "resolution must not mutate caller options")
		})
	}
}
