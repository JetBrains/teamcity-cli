package onboard

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmd/skill"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/charmbracelet/huh"
)

// agent is a local AI coding agent onboarding can hand over to.
type agent struct {
	name    string // instill agent name, as `teamcity skill install --agent` expects
	label   string
	bin     string // executable looked up on PATH
	path    string // set by detectAgents
	version string // first line of `<bin> --version`, set by detectAgents
}

// supportedAgents are the only agents onboarding hands over to.
var supportedAgents = []agent{
	{name: "claude-code", label: "Claude Code", bin: "claude"},
	{name: "codex", label: "Codex", bin: "codex"},
	{name: "junie", label: "Junie", bin: "junie"},
}

// supportedAgentLabels lists supportedAgents for messages, e.g. "Claude Code, Codex, Junie".
func supportedAgentLabels() string {
	labels := make([]string, len(supportedAgents))
	for i, a := range supportedAgents {
		labels[i] = a.label
	}
	return strings.Join(labels, ", ")
}

// versionTimeout bounds `--version`, so a hanging binary can't stall onboarding.
const versionTimeout = 3 * time.Second

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
	authorized bool // a VCS root usable by projectID points at repo
}

// handOverToAgent passes the onboarding context (repo, its TeamCity project, VCS root status) to an AI agent to finish the setup.
func handOverToAgent(f *cmdutil.Factory, client api.ClientInterface, repo repository, projectID string, authorized bool) error {
	agents := detectAgents(f.Context())
	if len(agents) == 0 {
		f.Printer.Warn("No supported agents found, install one and try again.")
		f.Printer.Tip("Supported agents: %s", supportedAgentLabels())
		return nil
	}
	ag, err := pickAgent(f, agents)
	if err != nil {
		return err
	}

	// TODO: copy pending design review
	proceed := true
	if err := cmdutil.Confirm("Continue onboarding with "+ag.label+"?", &proceed); err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	scope, err := promptSkillScope(f, repo)
	if err != nil {
		return err
	}
	if err := installSkills(f, ag, scope); err != nil {
		return err
	}

	return launchAgent(f, ag, handoverContext{
		serverURL:  client.ServerURL(),
		projectID:  projectID,
		repo:       repo,
		authorized: authorized,
	})
}

// detectAgents returns the supportedAgents on PATH whose `--version` succeeds, so broken installs aren't offered.
func detectAgents(ctx context.Context) []agent {
	var found []agent
	for _, a := range supportedAgents {
		path, err := exec.LookPath(a.bin)
		if err != nil {
			continue
		}
		version, err := agentVersion(ctx, path)
		if err != nil {
			continue
		}
		a.path, a.version = path, version
		found = append(found, a)
	}
	return found
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
	// TODO: copy pending design review
	options := make([]huh.Option[agent], 0, len(agents))
	for _, a := range agents {
		options = append(options, huh.NewOption(a.label, a))
	}
	var picked agent
	if err := cmdutil.Select(f.Printer, "Choose an agent to finish onboarding", options, &picked); err != nil {
		return agent{}, err
	}
	return picked, nil
}

// promptSkillScope asks where to install the skills; per-project needs a local clone, so samples without one install globally.
func promptSkillScope(f *cmdutil.Factory, repo repository) (skillScope, error) {
	if repo.dir == "" {
		return scopeGlobal, nil
	}
	// TODO: copy pending design review
	options := []huh.Option[skillScope]{
		huh.NewOption("Globally, for all projects", scopeGlobal),
		huh.NewOption("Only for "+repo.dir, scopeProject),
	}
	var scope skillScope
	if err := cmdutil.Select(f.Printer, "Where should the TeamCity skills be installed?", options, &scope); err != nil {
		return "", err
	}
	return scope, nil
}

// installSkills runs `teamcity skill install` for ag in scope.
func installSkills(f *cmdutil.Factory, ag agent, scope skillScope) error {
	// TODO: per-project installs into the current directory, not repo.dir
	args := []string{"install", "--agent", ag.name}
	if scope == scopeProject {
		args = append(args, "--project")
	}
	cmd := skill.NewCmd(f)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.ExecuteContext(f.Context())
}

// launchAgent starts the agent with the staged context and hands it the terminal.
func launchAgent(_ *cmdutil.Factory, _ agent, _ handoverContext) error {
	// TODO: stub, launch agent handover
	return nil
}
