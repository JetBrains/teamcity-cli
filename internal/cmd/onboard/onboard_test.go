package onboard_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

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
