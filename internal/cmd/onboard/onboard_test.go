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

func TestOnboardTokenIsOptional(t *testing.T) {
	ts := cmdtest.NewTestServer(t)

	// Gets past flag validation to the interactive check instead of failing on a missing --token.
	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "requires an interactive terminal", "onboard", ts.URL, "--no-input")
}

func TestOnboardRequiresServerURL(t *testing.T) {
	ts := cmdtest.NewTestServer(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "accepts 1 arg(s)", "onboard", "--token", "onboarding-token")
}
