package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestOccurrenceQueryBuildLocator(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		q    TestOccurrenceQuery
		want string
	}{
		{
			name: "build subject",
			q:    TestOccurrenceQuery{Build: "123"},
			want: "build:(id:123)",
		},
		{
			name: "test name subject",
			q:    TestOccurrenceQuery{TestName: "com.example.Foo.bar"},
			want: "test:(name:com.example.Foo.bar)",
		},
		{
			name: "status failed",
			q:    TestOccurrenceQuery{Build: "1", Status: "failed"},
			want: "build:(id:1),status:FAILURE",
		},
		{
			name: "status failed with muted false",
			q:    TestOccurrenceQuery{Build: "1", Status: "failed", Muted: new(false)},
			want: "build:(id:1),status:FAILURE,muted:false",
		},
		{
			name: "status failed with muted true",
			q:    TestOccurrenceQuery{Build: "1", Status: "failed", Muted: new(true)},
			want: "build:(id:1),status:FAILURE,muted:true",
		},
		{
			name: "status passed",
			q:    TestOccurrenceQuery{Build: "1", Status: "passed"},
			want: "build:(id:1),status:SUCCESS",
		},
		{
			name: "status ignored",
			q:    TestOccurrenceQuery{Build: "1", Status: "ignored"},
			want: "build:(id:1),ignored:true",
		},
		{
			name: "status new",
			q:    TestOccurrenceQuery{Build: "1", Status: "new"},
			want: "build:(id:1),newFailure:true",
		},
		{
			name: "muted only without status",
			q:    TestOccurrenceQuery{Build: "1", Muted: new(true)},
			want: "build:(id:1),muted:true",
		},
		{
			name: "test history scoped to a job (buildType + test)",
			q:    TestOccurrenceQuery{BuildType: "MyJob", TestName: "Foo.bar"},
			want: "buildType:(id:MyJob),test:(name:Foo.bar)",
		},
		{
			name: "job-scoped history only failures",
			q:    TestOccurrenceQuery{BuildType: "MyJob", TestName: "Foo.bar", Status: "failed"},
			want: "buildType:(id:MyJob),test:(name:Foo.bar),status:FAILURE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l, err := tc.q.buildLocator()
			require.NoError(t, err)
			assert.Equal(t, tc.want, l.String())
		})
	}
}

func TestTestOccurrenceQueryBuildLocatorErrors(t *testing.T) {
	t.Parallel()

	t.Run("empty scope", func(t *testing.T) {
		t.Parallel()
		_, err := TestOccurrenceQuery{}.buildLocator()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no test scope")
	})

	t.Run("invalid status", func(t *testing.T) {
		t.Parallel()
		_, err := TestOccurrenceQuery{Build: "1", Status: "bogus"}.buildLocator()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})
}

func TestListTestOccurrencesPagesThroughNextHref(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var detailPaths []string

	client := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/app/rest/testOccurrences" {
			http.NotFound(w, r)
			return
		}

		switch {
		case r.URL.Query().Get("page") == "2":
			mu.Lock()
			detailPaths = append(detailPaths, r.URL.RequestURI())
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(TestOccurrences{
				TestOccurrence: []TestOccurrence{{ID: "3", Name: "Baz"}},
			})
		case strings.Contains(r.URL.Query().Get("fields"), "testOccurrence("):
			mu.Lock()
			detailPaths = append(detailPaths, r.URL.Query().Get("locator"))
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(TestOccurrences{
				NextHref:       "/app/rest/testOccurrences?page=2",
				TestOccurrence: []TestOccurrence{{ID: "1", Name: "Foo"}, {ID: "2", Name: "Bar"}},
			})
		default:
			_ = json.NewEncoder(w).Encode(TestOccurrences{Count: 3, Failed: 3})
		}
	})

	tests, err := client.ListTestOccurrences(t.Context(), TestOccurrenceQuery{Build: "1"})
	require.NoError(t, err)

	require.Len(t, tests.TestOccurrence, 3, "should follow nextHref to collect all pages")
	assert.Equal(t, "3", tests.TestOccurrence[2].ID)
	assert.Equal(t, 3, tests.Failed, "aggregate summary is preserved")

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, detailPaths, 2, "should request a second page via nextHref")
	assert.Contains(t, detailPaths[0], "count:1000")
}
