package onboard_test

import (
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmd/onboard"
	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/stretchr/testify/assert"
)

func TestOnboardIsExperimental(t *testing.T) {
	cmd := onboard.NewCmd(cmdutil.NewFactory())

	assert.Equal(t, "true", cmd.Annotations["experimental"])
	assert.True(t, strings.HasPrefix(cmd.Short, "[experimental] "))
}

func TestOnboardRequiresInteractiveTerminal(t *testing.T) {
	ts := cmdtest.NewTestServer(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "requires an interactive terminal", "onboard", ts.URL, "--token", "onboarding-token", "--no-input")
}

func TestOnboardRequiresToken(t *testing.T) {
	ts := cmdtest.NewTestServer(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, `required flag(s) "token" not set`, "onboard", ts.URL)
}

func TestOnboardRequiresServerURL(t *testing.T) {
	ts := cmdtest.NewTestServer(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "accepts 1 arg(s)", "onboard", "--token", "onboarding-token")
}
