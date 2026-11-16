package analytics

import (
	"slices"
	"strings"
)

// allCommands enumerates every command path the CLI exposes for the `command` field; unknowns → "other".
func allCommands() []string {
	return []string{
		"auth.login", "auth.logout", "auth.status",
		"run.list", "run.view", "run.start", "run.cancel", "run.restart", "run.watch",
		"run.log", "run.download", "run.artifacts", "run.tests", "run.pin", "run.unpin",
		"run.tag", "run.untag", "run.comment", "run.changes", "run.tree", "run.diff",
		"run.analysis", "run.metadata", "run.git",
		"job.create", "job.list", "job.view", "job.tree", "job.pause", "job.resume",
		"job.param.list", "job.param.get", "job.param.set", "job.param.delete",
		"job.settings.list", "job.settings.get", "job.settings.set",
		"job.step.list", "job.step.view", "job.step.add", "job.step.delete",
		"project.list", "project.view", "project.tree", "project.create",
		"project.vcs.list", "project.vcs.view", "project.vcs.create", "project.vcs.test", "project.vcs.delete",
		"project.ssh.list", "project.ssh.upload", "project.ssh.generate", "project.ssh.delete",
		"project.cloud.profile.list", "project.cloud.profile.view",
		"project.cloud.image.list", "project.cloud.image.view", "project.cloud.image.start",
		"project.cloud.instance.list", "project.cloud.instance.view", "project.cloud.instance.stop",
		"project.connection.list", "project.connection.view", "project.connection.authorize", "project.connection.delete",
		"project.connection.create.docker", "project.connection.create.github-app",
		"project.token.put", "project.token.get",
		"project.settings.enable", "project.settings.status", "project.settings.export", "project.settings.validate",
		"project.param.list", "project.param.get", "project.param.set", "project.param.delete",
		"queue.list", "queue.remove", "queue.top", "queue.approve",
		"agent.list", "agent.view", "agent.jobs", "agent.move", "agent.enable",
		"agent.disable", "agent.authorize", "agent.deauthorize", "agent.term",
		"agent.exec", "agent.reboot",
		"pool.list", "pool.view", "pool.link", "pool.unlink",
		"pipeline.list", "pipeline.view", "pipeline.validate", "pipeline.create",
		"pipeline.delete", "pipeline.pull", "pipeline.push", "pipeline.schema",
		"api", "link", "migrate", "onboard", "server.plugin.upload",
		"alias.list", "alias.set", "alias.delete",
		"config.list", "config.get", "config.set",
		"skill.list", "skill.install", "skill.update", "skill.remove", "skill.guide", "skill.hook-event",
		"update", "other",
	}
}

// allAIAgents enumerates AI agent identifiers: every instill agent name with `-` → `_`, plus instill's runtime-only "cowork".
func allAIAgents() []string {
	return []string{
		"adal", "aider_desk", "amp", "antigravity", "antigravity_cli", "astrbot", "augment", "autohand_code",
		"bob", "claude_code", "cline", "codearts_agent", "codebuddy", "codemaker", "codestudio", "codex",
		"command_code", "continue", "cortex", "cowork", "crush", "cursor", "deepagents", "devin", "dexto",
		"droid", "eve", "firebender", "forgecode", "fx", "gemini_cli", "github_copilot", "goose", "grok",
		"hermes_agent", "iflow_cli", "inference_sh", "jazz", "junie", "kilo", "kimchi", "kimi_code_cli",
		"kiro_cli", "kode", "lingma", "loaf", "mcpjam", "minimax_code", "mistral_vibe", "moxby", "mux",
		"neovate", "ona", "openclaw", "opencode", "openhands", "pi", "pochi", "posit_assistant",
		"promptscript", "qoder", "qoder_cn", "qwen_code", "reasonix", "replit", "roo", "rovodev",
		"sarvam_code", "tabnine_cli", "terramind", "tinycloud", "trae", "trae_cn", "warp", "windsurf",
		"zcode", "zed", "zencoder", "zenflow",
		"other", "none",
	}
}

// allSkills enumerates the skills the CLI bundles from teamcity-skills.
func allSkills() []string {
	return []string{"teamcity-cli", "migrate-to-teamcity", "teamcity-onboard", "teamcity-migrate-remaining-gha", "other"}
}

// NormalizeSkill returns the wire-safe value for a bundled skill name; unknown → "other".
func NormalizeSkill(name string) string {
	if slices.Contains(allSkills(), name) {
		return name
	}
	return "other"
}

func skillAgentEnum() []string {
	return slices.DeleteFunc(allAIAgents(), func(a string) bool { return a == "none" })
}

// NormalizeCommand returns the wire-safe value for a cobra command path; unknown → "other".
func NormalizeCommand(path string) string {
	if slices.Contains(allCommands(), path) {
		return path
	}
	return "other"
}

// NormalizeAIAgent maps an instill agent name to the wire enum; "" → none, unknown → other.
func NormalizeAIAgent(instillName string) string {
	if instillName == "" {
		return "none"
	}
	v := strings.ReplaceAll(instillName, "-", "_")
	if slices.Contains(allAIAgents(), v) {
		return v
	}
	return "other"
}
