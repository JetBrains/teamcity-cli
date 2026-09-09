package agent_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAgentTerminalReadOnly(t *testing.T) {
	for _, source := range []string{"env", "server"} {
		t.Run(source, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			ts := cmdtest.NewTestServer(t)
			t.Setenv("TEAMCITY_RO", "")
			if source == "env" {
				t.Setenv("TEAMCITY_RO", "1")
			} else {
				config.Get().Servers[ts.URL] = config.ServerConfig{RO: true}
				t.Cleanup(func() { delete(config.Get().Servers, ts.URL) })
			}
			ts.Factory.ClientFunc = func() (api.ClientInterface, error) {
				t.Fatal("read-only terminal access must fail before creating a client")
				return nil, nil
			}
			for _, args := range [][]string{{"agent", "exec", "1", "whoami"}, {"agent", "term", "1"}} {
				err := cmdtest.CaptureErr(t, ts.Factory, args...)
				require.ErrorIs(t, err, api.ErrReadOnly)
			}
		})
	}
}
