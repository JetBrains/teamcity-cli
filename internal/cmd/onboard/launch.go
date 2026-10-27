package onboard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
)

// launchAgent runs ag in the foreground on this terminal with the staged context, offering a rerun when it exits with an error.
func launchAgent(f *cmdutil.Factory, ag agent, hc handoverContext) error {
	args := ag.promptArgs(handoverPrompt(hc))
	// TODO: copy pending design review
	f.Printer.Info("Handing over to %s. It uses this terminal; exit it to return to teamcity.\n", ag.label)
	for {
		err := runAgent(ag.path, args, hc.repo.dir)
		if err == nil {
			// A clean exit doesn't prove the setup is done, only that the agent ended normally.
			f.Printer.Info("\n%s exited.", ag.label)
			return nil
		}
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			return fmt.Errorf("start %s: %w", ag.label, err)
		}

		f.Printer.Warn("%s stopped with an error (%s). See its message above.", ag.label, exitErr.ProcessState)
		// TODO: copy pending design review
		rerun := false
		if err := cmdutil.Confirm("Run "+ag.label+" again? It may repeat changes it already made.", &rerun); err != nil {
			return err
		}
		if !rerun {
			return &cmdutil.ExitError{Code: max(exitErr.ExitCode(), 1)} // -1 when killed by a signal
		}
		// A manual repair or update may have replaced the executable.
		path, err := exec.LookPath(ag.bin)
		if err != nil {
			return api.Validation(ag.bin+" is no longer on PATH", "Fix the "+ag.label+" installation, then rerun teamcity onboard")
		}
		ag.path = path
	}
}

// runAgent runs path as a foreground child sharing this terminal; an *exec.ExitError means it ran and failed, any other error that it didn't start.
func runAgent(path string, args []string, dir string) error {
	// Not CommandContext: Ctrl+C reaches the agent through the terminal, and it handles it itself;
	// tc/main.go traps the signal, so teamcity waits instead of dying, and canceling would kill the agent mid-session.
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// promptArgs builds the arguments that start an interactive session with prompt already submitted.
func (a agent) promptArgs(prompt string) []string {
	if a.promptFlag == "" {
		return []string{prompt}
	}
	return []string{a.promptFlag, prompt}
}

// handoverPrompt passes the staged context and points at the onboard skill, which carries the steps.
// It stays on one line: Windows .cmd shims (npm installs) run through cmd.exe, which cuts arguments at a newline.
func handoverPrompt(hc handoverContext) string {
	// TODO: prompt wording pending design review
	parts := []string{
		"Use the " + onboardSkill + " skill to set up TeamCity for this repository.",
		"Server: " + hc.serverURL + ".",
		"Project: " + hc.projectID + ".",
	}
	if hc.repo.remoteURL != "" {
		parts = append(parts, "Repository: "+hc.repo.remoteURL+".")
	}
	if hc.repo.dir != "" {
		parts = append(parts, "Local checkout: "+hc.repo.dir+".")
	}
	if hc.repo.revision != "" {
		parts = append(parts, "Revision: "+hc.repo.revision+".")
	}
	if hc.authorized {
		parts = append(parts, "VCS root: exists.")
	} else {
		parts = append(parts, "VCS root: none.")
	}
	return strings.Join(parts, " ")
}
