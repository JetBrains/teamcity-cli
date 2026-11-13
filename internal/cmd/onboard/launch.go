package onboard

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/JetBrains/teamcity-cli/internal/onboarding"
)

// launchAgent runs ag in the foreground on this terminal with the staged context, offering a rerun when it exits with an error.
// It returns where the hand-over stopped.
func launchAgent(f *cmdutil.Factory, ag agent, hc handoverContext) (handoverEnd, error) {
	if ag.launchFlag.ifListed && !agentHasFlag(f.Context(), ag.path, ag.launchFlag.name) {
		ag.launchFlag = launchFlag{}
	}
	args := ag.promptArgs(handoverPrompt(hc), hc.sandbox.launchArgs)
	rt := repositoryType(hc.repo.source)
	f.Printer.Info("Starting %s in this terminal. Exit it to return to your shell.\n", ag.label)
	for {
		err := runAgent(ag.path, args, hc.repo.dir, rt)
		if err == nil {
			// A clean exit doesn't prove the setup is done, only that the agent ended normally.
			f.Printer.Info("\n%s exited.", ag.label)
			return endAgentExited, nil
		}
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if !ok {
			return endAgentStartFailed, fmt.Errorf("start %s: %w", ag.label, err)
		}
		code := max(exitErr.ExitCode(), 1) // -1 when killed by a signal
		// Ctrl+C or SIGTERM ended it; the user asked to stop, so don't offer a rerun.
		if f.Context().Err() != nil {
			return endAgentInterrupted, &cmdutil.ExitError{Code: code}
		}

		f.Printer.Warn("%s stopped with an error (%s). See its message above.", ag.label, exitErr.ProcessState)
		if ag.failTip != "" {
			f.Printer.Tip("%s", ag.failTip)
		}
		// TODO: copy pending design review
		rerun := false
		if err := confirm("Run "+ag.label+" again? It may repeat changes it already made.", &rerun); err != nil {
			return endFailed, err
		}
		if !rerun {
			return endRerunDeclined, &cmdutil.ExitError{Code: code}
		}
		// A manual repair or update may have replaced the executable.
		path, err := exec.LookPath(ag.bin)
		if err != nil {
			return endAgentError, api.Validation(ag.bin+" is no longer on PATH", "Fix the "+ag.label+" installation, then rerun teamcity onboard")
		}
		ag.path = path
	}
}

// runAgent runs path as a foreground child sharing this terminal, telling the teamcity commands it runs that they're inside the onboarding of an rt repository.
// An *exec.ExitError means it ran and failed, any other error that it didn't start.
func runAgent(path string, args []string, dir string, rt api.OnboardingRepositoryType) error {
	// Not CommandContext: Ctrl+C reaches the agent through the terminal, and it handles it itself;
	// tc/main.go traps the signal, so teamcity waits instead of dying, and canceling would kill the agent mid-session.
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), onboarding.Env(rt))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// promptArgs builds the arguments that start an interactive session with prompt already submitted, after the agent's launchArgs.
func (a agent) promptArgs(prompt string, launchArgs []string) []string {
	args := slices.Clone(launchArgs)
	if a.launchFlag.name != "" {
		args = append(args, a.launchFlag.name)
	}
	return append(args, prompt)
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
	if hc.sandbox.agentMustAsk {
		parts = append(parts, "Ask to run teamcity commands outside the sandbox: they need the system keyring and the server.")
	}
	if hc.repo.remoteURL != "" {
		parts = append(parts, "Repository: "+withoutUserinfo(hc.repo.remoteURL)+".")
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

// withoutUserinfo drops credentials from an http(s) remote, e.g. https://token@host/org/repo → https://host/org/repo.
// Not url.Redacted: it keeps the username, and hosts like GitHub accept a token there.
func withoutUserinfo(remote string) string {
	u, err := url.Parse(remote)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return remote // scp-style git@host:org/repo and ssh:// carry a username, not a secret
	}
	u.User = nil
	return u.String()
}
