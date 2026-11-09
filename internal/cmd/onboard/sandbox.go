package onboard

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/pelletier/go-toml/v2"
)

// sandboxOutcome is what the sandbox step decided before the launch.
type sandboxOutcome struct {
	// launchArgs are the agent's flags that let it run teamcity commands outside its sandbox in this session, such as Claude Code's --settings.
	// Empty when the agent needs none, or a yes was applied another way, such as Codex's rule file.
	launchArgs []string
	// agentMustAsk means the agent will run teamcity commands in its sandbox: the user said no, or the yes
	// couldn't be applied. The handover prompt then tells the agent to ask the user to run them outside.
	agentMustAsk bool
}

// sandboxAccess lets teamcity run outside an agent's command sandbox, which hides the system keyring and the TeamCity server from it.
type sandboxAccess struct {
	sandboxed func(repoDir string) bool // whether the agent would run teamcity in its sandbox
	why       func() []string           // lines shown before the question: why it's needed, what a yes does
	question  string                    // a short yes/no question
	allow     func() ([]string, error)  // applies a yes; returns the launch args it needs, if any
}

// sandboxAccesses are keyed by agent name. Junie has none: its sandbox is off unless the user turns it on in the session.
var sandboxAccesses = map[string]sandboxAccess{
	"claude-code": {
		sandboxed: claudeSandboxEnabled,
		why: func() []string {
			return []string{
				"Claude Code's sandbox is on. Inside it, the TeamCity CLI can't access your TeamCity server.",
				"A yes lets TeamCity CLI commands run outside the sandbox in this session only. Other commands stay sandboxed.",
			}
		},
		question: "Run TeamCity CLI commands outside the sandbox?",
		allow: func() ([]string, error) {
			// A session-only setting that merges with the user's own sandbox settings.
			return []string{"--settings", `{"sandbox":{"excludedCommands":["teamcity *"]}}`}, nil
		},
	},
	"codex": {
		sandboxed: func(string) bool { return codexSandboxEnabled() && !codexTeamcityRuleExists() },
		why: func() []string {
			return []string{
				"Codex runs commands in a sandbox. Inside it, the TeamCity CLI can't access your TeamCity server.",
				"A yes adds a rule to " + filepath.Join(codexHome(), "rules", codexRuleFile) + ".",
				"Codex then runs TeamCity CLI commands outside its sandbox without asking, in this and future sessions. Other commands stay sandboxed.",
			}
		},
		question: "Let Codex run TeamCity CLI commands outside its sandbox?",
		allow:    func() ([]string, error) { return nil, writeCodexTeamcityRule() },
	},
}

// allowOutsideSandbox asks, when ag would run teamcity in its sandbox, whether teamcity may run outside it, and applies a yes.
func allowOutsideSandbox(f *cmdutil.Factory, ag agent, repoDir string) (sandboxOutcome, error) {
	// TODO(FUS): report this step per agent: not sandboxed, allowed, declined, or allow failed.
	access, ok := sandboxAccesses[ag.name]
	if !ok || !access.sandboxed(repoDir) {
		return sandboxOutcome{}, nil
	}
	for _, line := range access.why() {
		f.Printer.Info("%s", line)
	}
	allowed := true
	if err := confirm(access.question, &allowed); err != nil {
		return sandboxOutcome{}, err
	}
	if !allowed {
		f.Printer.Info("%s will ask you before it runs TeamCity CLI commands outside the sandbox.", ag.label)
		return sandboxOutcome{agentMustAsk: true}, nil
	}
	args, err := access.allow()
	if err != nil {
		f.Printer.Warn("Couldn't let the TeamCity CLI run outside the sandbox: %v. %s will ask you during the session.", err, ag.label)
		return sandboxOutcome{agentMustAsk: true}, nil
	}
	return sandboxOutcome{launchArgs: args}, nil
}

// claudeConfigDir is Claude Code's user config directory: $CLAUDE_CONFIG_DIR, or ~/.claude.
func claudeConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// claudeSandboxEnabled reads sandbox.enabled from the settings files in Claude Code's precedence order: local, project, user.
// Managed settings (files, MDM, server) aren't read; when they turn the sandbox on, the skill makes the agent ask instead.
func claudeSandboxEnabled(repoDir string) bool {
	for _, path := range []string{
		filepath.Join(repoDir, ".claude", "settings.local.json"),
		filepath.Join(repoDir, ".claude", "settings.json"),
		filepath.Join(claudeConfigDir(), "settings.json"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var s struct {
			Sandbox struct {
				Enabled *bool `json:"enabled"`
			} `json:"sandbox"`
		}
		if json.Unmarshal(data, &s) == nil && s.Sandbox.Enabled != nil {
			return *s.Sandbox.Enabled
		}
	}
	return false
}

// codexRuleFile holds the rule onboarding adds; Codex reads every .rules file in its rules folder at startup.
const codexRuleFile = "teamcity.rules"

// codexHome is Codex's config directory: $CODEX_HOME, or ~/.codex.
func codexHome() string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

// codexSandboxEnabled reports whether Codex sandboxes commands: it does unless config.toml sets sandbox_mode to danger-full-access.
func codexSandboxEnabled() bool {
	data, err := os.ReadFile(filepath.Join(codexHome(), "config.toml"))
	if err != nil {
		return true
	}
	var c struct {
		SandboxMode string `toml:"sandbox_mode"`
	}
	if toml.Unmarshal(data, &c) != nil {
		return true
	}
	return c.SandboxMode != "danger-full-access"
}

// codexTeamcityRuleExists reports whether a Codex rule already covers teamcity, for example one the user approved for good.
func codexTeamcityRuleExists() bool {
	files, _ := filepath.Glob(filepath.Join(codexHome(), "rules", "*.rules"))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err == nil && strings.Contains(string(data), `pattern=["teamcity"]`) {
			return true
		}
	}
	return false
}

// writeCodexTeamcityRule adds the rule that runs teamcity commands outside Codex's sandbox without asking.
func writeCodexTeamcityRule() error {
	dir := filepath.Join(codexHome(), "rules")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	rule := "# Added by teamcity onboard: teamcity needs the system keyring and the TeamCity server, which the Codex sandbox blocks.\n" +
		`prefix_rule(pattern=["teamcity"], decision="allow")` + "\n"
	path := filepath.Join(dir, codexRuleFile)
	if _, err := os.Stat(path); err == nil {
		return errors.New(path + " already exists")
	}
	return os.WriteFile(path, []byte(rule), 0o600)
}
