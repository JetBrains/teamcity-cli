package onboard

import (
	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
)

// handOverToAgent passes the onboarding context to an AI agent to finish the setup.
func handOverToAgent(_ *cmdutil.Factory, _ api.ClientInterface, _ repository, _ bool) error {
	// TODO: stub, launch agent handover
	return nil
}
