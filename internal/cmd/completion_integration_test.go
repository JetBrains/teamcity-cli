package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStaticCompletions drives cobra's hidden __complete command to confirm each handler is wired to the right flag or positional.
func TestStaticCompletions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "run list --status",
			args: []string{"__complete", "run", "list", "--status", ""},
			want: []string{"success", "failure", "running", "queued", "error", "unknown", "canceled"},
		},
		{
			name: "job tree --only",
			args: []string{"__complete", "job", "tree", "--only", ""},
			want: []string{"dependents", "dependencies"},
		},
		{
			name: "api --method",
			args: []string{"__complete", "api", "--method", ""},
			want: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		},
		{
			name: "project ssh generate --type",
			args: []string{"__complete", "project", "ssh", "generate", "--type", ""},
			want: []string{"ed25519", "rsa"},
		},
		{
			name: "project vcs create --auth",
			args: []string{"__complete", "project", "vcs", "create", "--auth", ""},
			want: []string{"password", "ssh-key", "ssh-agent", "ssh-file", "token", "anonymous"},
		},
		{
			name: "config get <key>",
			args: []string{"__complete", "config", "get", ""},
			want: []string{"default_server", "guest", "ro", "token_expiry"},
		},
		{
			name: "config set <key>",
			args: []string{"__complete", "config", "set", ""},
			want: []string{"default_server", "guest", "ro", "token_expiry"},
		},
		{
			name: "config set guest <value>",
			args: []string{"__complete", "config", "set", "guest", ""},
			want: []string{"true", "false"},
		},
		{
			name: "skill install [name]",
			args: []string{"__complete", "skill", "install", ""},
			want: []string{"teamcity-cli"},
		},
		{
			name: "run list --user",
			args: []string{"__complete", "run", "list", "--user", ""},
			want: []string{"@me"},
		},
		{
			name: "run list --revision",
			args: []string{"__complete", "run", "list", "--revision", ""},
			want: []string{"@head"},
		},
		{
			name: "run list --branch includes @this",
			args: []string{"__complete", "run", "list", "--branch", ""},
			want: []string{"@this"},
		},
		{
			name: "project view positional includes _Root",
			args: []string{"__complete", "project", "view", ""},
			want: []string{"_Root"},
		},
		{
			name: "project token put positional includes _Root",
			args: []string{"__complete", "project", "token", "put", ""},
			want: []string{"_Root"},
		},
		{
			name: "pool link <project-id> second positional",
			args: []string{"__complete", "pool", "link", "1", ""},
			want: []string{"_Root"},
		},
		{
			// --local-changes has NoOptDefVal, so cobra needs --flag= form to request value completion.
			name: "run start --local-changes includes git",
			args: []string{"__complete", "run", "start", "Foo", "--local-changes="},
			want: []string{"git"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := runComplete(t, tc.args...)
			for _, value := range tc.want {
				assert.Contains(t, out, value, "expected %q in completion output:\n%s", value, out)
			}
		})
	}
}

func runComplete(t *testing.T, args ...string) string {
	t.Helper()
	rootCmd := cmd.NewCommand(nil)
	rootCmd.SetArgs(args)
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	require.NoError(t, rootCmd.Execute())
	return strings.TrimSpace(out.String())
}
