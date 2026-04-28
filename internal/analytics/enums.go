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
		"job.list", "job.view", "job.tree", "job.pause", "job.resume",
		"job.param.list", "job.param.get", "job.param.set", "job.param.delete",
		"project.list", "project.view", "project.tree", "project.create",
		"project.vcs.list", "project.vcs.view", "project.vcs.create", "project.vcs.test", "project.vcs.delete",
		"project.ssh.list", "project.ssh.upload", "project.ssh.generate", "project.ssh.delete",
		"project.cloud.profile.list", "project.cloud.profile.view",
		"project.cloud.image.list", "project.cloud.image.view",
		"project.cloud.instance.list", "project.cloud.instance.view",
		"project.connection.list", "project.token.put", "project.token.get",
		"project.settings.status", "project.settings.export", "project.settings.validate",
		"project.param.list", "project.param.get", "project.param.set", "project.param.delete",
		"queue.list", "queue.remove", "queue.top", "queue.approve",
		"agent.list", "agent.view", "agent.jobs", "agent.move", "agent.enable",
		"agent.disable", "agent.authorize", "agent.deauthorize", "agent.terminal",
		"agent.exec", "agent.reboot",
		"pool.list", "pool.view", "pool.link", "pool.unlink",
		"pipeline.list", "pipeline.view", "pipeline.validate", "pipeline.create",
		"pipeline.delete", "pipeline.pull", "pipeline.push",
		"api", "link",
		"alias.list", "alias.set", "alias.delete",
		"config.list", "config.get", "config.set",
		"skill.list", "skill.install", "skill.update", "skill.remove",
		"update", "other",
	}
}

// allAIAgents enumerates AI agent identifiers; mirrors instill names with `-` → `_`.
func allAIAgents() []string {
	return []string{
		"claude_code", "junie", "cursor", "gemini_cli", "codex", "goose",
		"augment", "github_copilot", "amp", "windsurf", "opencode", "trae",
		"roo", "other", "none",
	}
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
