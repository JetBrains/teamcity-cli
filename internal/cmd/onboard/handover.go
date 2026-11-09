package onboard

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/charmbracelet/huh"
)

// agent is a local AI coding agent onboarding can hand over to.
type agent struct {
	name  string // instill agent name, as `teamcity skill install --agent` expects
	label string
	bin   string // executable looked up on PATH
	// promptFlag passes the initial prompt to an interactive session; empty means a positional argument.
	promptFlag string
	failTip    string // shown when the agent exits with an error
	path       string // set by detectAgents
	version    string // first line of `<bin> --version`, set by detectAgents
}

// supportedAgents are the only agents onboarding hands over to.
var supportedAgents = []agent{
	{name: "claude-code", label: "Claude Code", bin: "claude"},
	{name: "codex", label: "Codex", bin: "codex"},
	{
		name: "junie", label: "Junie", bin: "junie",
		promptFlag: "--prompt", // a positional arg runs a non-interactive task
		failTip:    "Older Junie CLI versions don't support --prompt; update Junie, then run it again",
	},
}

// supportedAgentLabels lists supportedAgents for messages, e.g. "Claude Code, Codex, Junie".
func supportedAgentLabels() string {
	labels := make([]string, len(supportedAgents))
	for i, a := range supportedAgents {
		labels[i] = a.label
	}
	return strings.Join(labels, ", ")
}

// versionTimeout bounds `--version` so a hanging binary can't stall onboarding; cold starts of npm installs and self-updating launchers can exceed 3s.
const versionTimeout = 10 * time.Second

type skillScope string

const (
	scopeGlobal  skillScope = "global"
	scopeProject skillScope = "project"
)

// handoverContext is what the deterministic part staged for the agent.
type handoverContext struct {
	serverURL  string
	projectID  string
	repo       repository
	authorized bool           // a VCS root usable by projectID points at repo
	sandbox    sandboxOutcome // how the agent runs teamcity; see allowOutsideSandbox
}

// confirm asks a yes/no question; tests replace it, since prompts need a terminal.
var confirm = cmdutil.Confirm

// handOverToAgent passes the onboarding context (repo, its TeamCity project, VCS root status) to the picked AI agent to finish the setup,
// and reports how the hand-over ended.
func handOverToAgent(f *cmdutil.Factory, client api.ClientInterface, repo repository, projectID string, authorized bool, agents []agent) error {
	end, err := handOver(f, client, repo, projectID, authorized, agents)
	notifyHandoverFinished(f, client, repositoryType(repo.source), handoverStatus(end))
	return err
}

// handOver runs the hand-over and returns where it stopped.
func handOver(f *cmdutil.Factory, client api.ClientInterface, repo repository, projectID string, authorized bool, agents []agent) (handoverEnd, error) {
	// The agent works in the checkout, and per-project skills land there; a sample left uncloned has none.
	if repo.dir == "" {
		return endNoCheckout, api.Validation("the agent needs a local copy of the repository", "Rerun teamcity onboard and clone the sample repository")
	}
	ag, err := pickAgent(f, agents)
	if err != nil {
		return endFailed, err
	}

	// TODO: copy pending design review
	proceed := true
	if err := confirm("Continue onboarding with "+ag.label+"?", &proceed); err != nil {
		return endFailed, err
	}
	if !proceed {
		return endDeclined, nil
	}

	scope, err := promptSkillScope(f, repo)
	if err != nil {
		return endFailed, err
	}
	if err := installSkills(f, ag, scope, repo.dir); err != nil {
		return endSkillInstallFailed, err
	}
	sandbox, err := allowOutsideSandbox(f, ag, repo.dir)
	if err != nil {
		return endFailed, err
	}

	notifyHandoverStarted(f, client, repositoryType(repo.source), ag)
	return launchAgent(f, ag, handoverContext{
		serverURL:  client.ServerURL(),
		projectID:  projectID,
		repo:       repo,
		authorized: authorized,
		sandbox:    sandbox,
	})
}

// detectAgents returns the supportedAgents on PATH whose `--version` succeeds, so broken installs aren't offered.
func detectAgents(ctx context.Context) ([]agent, error) {
	var found []agent
	for _, a := range supportedAgents {
		path, err := exec.LookPath(a.bin)
		if err != nil {
			continue
		}
		version, err := agentVersion(ctx, path)
		if err != nil {
			// An interrupted command isn't a broken install.
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		a.path, a.version = path, version
		found = append(found, a)
	}
	return found, nil
}

// agentVersion runs `path --version` within versionTimeout and returns its first output line.
func agentVersion(ctx context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.WaitDelay = time.Second // don't wait on children still holding stdout after the kill
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return first, nil
}

// pickAgent returns the only detected agent, or asks the user to pick one.
func pickAgent(f *cmdutil.Factory, agents []agent) (agent, error) {
	if len(agents) == 1 {
		return agents[0], nil
	}
	options := make([]huh.Option[agent], 0, len(agents))
	for _, a := range agents {
		options = append(options, huh.NewOption(a.label, a))
	}
	var picked agent
	if err := cmdutil.Select(f.Printer, "Which AI agent should set up your build?", options, &picked); err != nil {
		return agent{}, err
	}
	return picked, nil
}

// promptSkillScope asks whether to install the skills globally or only into repo.dir.
func promptSkillScope(f *cmdutil.Factory, repo repository) (skillScope, error) {
	options := []huh.Option[skillScope]{
		huh.NewOption("All my repositories", scopeGlobal),
		huh.NewOption("Only this repository ("+repo.dir+")", scopeProject),
	}
	var scope skillScope
	if err := cmdutil.Select(f.Printer, "Install TeamCity skills for all your repositories or just this one?", options, &scope); err != nil {
		return "", err
	}
	return scope, nil
}

// onboardSkill carries the agent's onboarding steps; the handover prompt only points at it.
const onboardSkill = "teamcity-onboard"

// installSkills installs the onboard skill and the teamcity-cli skill it builds on for ag, into dir when scope is per-project.
func installSkills(f *cmdutil.Factory, ag agent, scope skillScope, dir string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate teamcity executable: %w", err)
	}
	cmd := skillInstallCmd(f, exe, ag, scope, dir)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install skills: %w", err)
	}
	return nil
}

// skillInstallCmd runs this binary's own `skill install` in dir, since --project installs into the working directory.
// It passes on --quiet and --no-color; the color env vars reach the child on their own.
func skillInstallCmd(f *cmdutil.Factory, exe string, ag agent, scope skillScope, dir string) *exec.Cmd {
	args := []string{"skill", "install", "teamcity-cli", onboardSkill, "--agent", ag.name}
	if scope == scopeProject {
		args = append(args, "--project")
	}
	if f.Quiet {
		args = append(args, "--quiet")
	}
	if f.NoColor {
		args = append(args, "--no-color")
	}
	cmd := exec.CommandContext(f.Context(), exe, args...)
	cmd.Dir = dir
	return cmd
}
