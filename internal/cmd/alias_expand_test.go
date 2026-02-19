package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandPositionalArgs(t *testing.T) {
	tests := []struct {
		name      string
		expansion string
		args      []string
		want      []string
	}{
		{
			name:      "no placeholders no args",
			expansion: "run list --status=failure",
			args:      nil,
			want:      []string{"run", "list", "--status=failure"},
		},
		{
			name:      "no placeholders with extra args",
			expansion: "run list --status=failure",
			args:      []string{"--limit=5"},
			want:      []string{"run", "list", "--status=failure", "--limit=5"},
		},
		{
			name:      "positional substitution",
			expansion: "run list --user=$1 --status=success",
			args:      []string{"@me"},
			want:      []string{"run", "list", "--user=@me", "--status=success"},
		},
		{
			name:      "multiple positional args",
			expansion: "run start --branch=$1 $2",
			args:      []string{"main", "MyJob"},
			want:      []string{"run", "start", "--branch=main", "MyJob"},
		},
		{
			name:      "positional plus extra args",
			expansion: "run list --user=$1",
			args:      []string{"@me", "--limit=5"},
			want:      []string{"run", "list", "--user=@me", "--limit=5"},
		},
		{
			name:      "unused positional placeholder",
			expansion: "run list --user=$1 --branch=$2",
			args:      []string{"@me"},
			want:      []string{"run", "list", "--user=@me", "--branch=$2"},
		},
		{
			name:      "double-digit placeholder not clobbered by single-digit",
			expansion: "$1 $10",
			args:      []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"},
			want:      []string{"A", "J", "B", "C", "D", "E", "F", "G", "H", "I"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandArgs(tt.expansion, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpandShellArgsQuotesUserInput(t *testing.T) {
	expansion := expandShellArgs("echo $1", []string{"hello world"})
	assert.Equal(t, `echo "hello world"`, expansion)

	expansion = expandShellArgs("echo $1", []string{"it's a test"})
	assert.Equal(t, `echo "it\'s a test"`, expansion)

	expansion = expandShellArgs("echo $1", []string{"; rm -rf /"})
	assert.Equal(t, `echo "\; rm -rf /"`, expansion)
}

// TestAwesomeAliasesExpand verifies that the example aliases from the README expand correctly.
func TestAwesomeAliasesExpand(t *testing.T) {
	tests := []struct {
		name      string
		expansion string
		args      []string
		want      []string
	}{
		{"rl", "run list", nil, []string{"run", "list"}},
		{"rv", "run view $1", []string{"672699"}, []string{"run", "view", "672699"}},
		{"rw", "run view $1 --web", []string{"672699"}, []string{"run", "view", "672699", "--web"}},
		{"mine", "run list --user=@me", nil, []string{"run", "list", "--user=@me"}},
		{"fails", "run list --status=failure --since=24h", nil, []string{"run", "list", "--status=failure", "--since=24h"}},
		{"running", "run list --status=running", nil, []string{"run", "list", "--status=running"}},
		{"morning", "run list --status=failure --since=12h", nil, []string{"run", "list", "--status=failure", "--since=12h"}},
		{"go", "run start $1 --watch", []string{"MyJob"}, []string{"run", "start", "MyJob", "--watch"}},
		{"try", "run start $1 --local-changes --watch", []string{"MyJob"}, []string{"run", "start", "MyJob", "--local-changes", "--watch"}},
		{"hotfix", "run start $1 --top --clean --watch", []string{"MyJob"}, []string{"run", "start", "MyJob", "--top", "--clean", "--watch"}},
		{"retry", "run restart $1 --watch", []string{"672653"}, []string{"run", "restart", "672653", "--watch"}},
		{"rush", "queue top $1", []string{"672699"}, []string{"queue", "top", "672699"}},
		{"ok", "queue approve $1", []string{"672699"}, []string{"queue", "approve", "672699"}},
		{"maint", "agent disable $1", []string{"107004"}, []string{"agent", "disable", "107004"}},
		{"unmaint", "agent enable $1", []string{"107004"}, []string{"agent", "enable", "107004"}},
		{"whoami", "api /app/rest/users/current", nil, []string{"api", "/app/rest/users/current"}},
		{"rl+extra", "run list", []string{"--limit", "5"}, []string{"run", "list", "--limit", "5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandArgs(tt.expansion, tt.args)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsHelpArg(t *testing.T) {
	assert.True(t, hasHelpFlag([]string{"--help"}))
	assert.True(t, hasHelpFlag([]string{"-h"}))
	assert.True(t, hasHelpFlag([]string{"foo", "--help"}))
	assert.False(t, hasHelpFlag([]string{"foo", "bar"}))
	assert.False(t, hasHelpFlag(nil))
}
