package run

import (
	"bytes"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

func init() { color.NoColor = true }

func TestBuildStatusSummary(t *testing.T) {
	tests := []struct {
		name string
		deps []RunTreeNode
		want string
	}{
		{
			name: "all passed",
			deps: []RunTreeNode{
				{Status: "SUCCESS", State: "finished"},
				{Status: "SUCCESS", State: "finished"},
			},
			want: "2 passed",
		},
		{
			name: "mixed results",
			deps: []RunTreeNode{
				{Status: "FAILURE", State: "finished"},
				{Status: "SUCCESS", State: "finished"},
				{Status: "FAILURE", State: "finished"},
				{Status: "SUCCESS", State: "finished"},
			},
			want: "2 failed · 2 passed",
		},
		{
			name: "with running and queued",
			deps: []RunTreeNode{
				{Status: "SUCCESS", State: "finished"},
				{State: "running"},
				{State: "queued"},
			},
			want: "1 passed · 1 running · 1 queued",
		},
		{
			name: "empty",
			deps: []RunTreeNode{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildStatusSummary(tt.deps)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPrintPipelineTree(t *testing.T) {
	var buf bytes.Buffer
	p := &output.Printer{Out: &buf, ErrOut: &buf}

	build := &api.Build{
		ID:         6708,
		Number:     "42",
		Status:     "FAILURE",
		State:      "finished",
		BranchName: "main",
	}
	pr := &api.PipelineRun{
		Pipeline: &api.PipelineRef{ID: "CLI_CiCd", Name: "CLI CI/CD"},
	}
	node := RunTreeNode{
		ID: 6708, Name: "Pipeline Head", Status: "FAILURE", State: "finished",
		Dependencies: []RunTreeNode{
			{ID: 6710, Name: "Test Linux ARM64", Status: "FAILURE", State: "finished", Dependencies: []RunTreeNode{}},
			{ID: 6712, Name: "Test macOS", Status: "FAILURE", State: "finished", Dependencies: []RunTreeNode{}},
			{ID: 6711, Name: "Lint", Status: "SUCCESS", State: "finished", Dependencies: []RunTreeNode{}},
			{ID: 6709, Name: "GoReleaser", Status: "SUCCESS", State: "finished", Dependencies: []RunTreeNode{}},
		},
	}

	printPipelineTree(p, build, pr, node)
	got := buf.String()

	assert.Contains(t, got, "CLI CI/CD ⬡")
	assert.Contains(t, got, "6708  #42")
	assert.Contains(t, got, "main")
	assert.Contains(t, got, "2 failed")
	assert.Contains(t, got, "2 passed")

	assert.Contains(t, got, "Test Linux ARM64  ")
	assert.Contains(t, got, "Test macOS        ")
	assert.Contains(t, got, "Lint              ")
	assert.Contains(t, got, "GoReleaser        ")
}
