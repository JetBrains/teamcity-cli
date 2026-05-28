package cmd

import (
	"fmt"
	"os"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/tiulpin/instill"
	"golang.org/x/term"
)

// maybePromoteSkillInstall nudges the user to install the teamcity-cli skill when running from a detected AI agent that lacks it.
func maybePromoteSkillInstall(f *cmdutil.Factory, cmd *cobra.Command, runErr error) {
	if f == nil || cmd == nil || runErr != nil || f.Quiet || f.JSONOutput || f.NoInput {
		return
	}
	if p := cmd.Parent(); p != nil && p.Name() == "skill" {
		return
	}
	errFile, ok := f.Printer.ErrOut.(*os.File)
	if !ok || !term.IsTerminal(int(errFile.Fd())) {
		return
	}
	r := instill.DetectRuntime()
	if r == nil {
		return
	}
	opts := instill.Options{Agents: []string{r.Name}, ProjectDir: "."}
	for _, global := range []bool{true, false} {
		opts.Global = global
		if v, err := instill.InstalledVersion("teamcity-cli", opts); err != nil || v != "" {
			return
		}
	}
	_, _ = fmt.Fprintln(f.Printer.ErrOut, "\n"+output.FormatTip(output.TipInstallSkillFor(r.DisplayName)))
}
