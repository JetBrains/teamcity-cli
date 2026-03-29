"""Parse Claude Code stream-json output into structured events."""

from __future__ import annotations

import json
import re
from dataclasses import dataclass, field


@dataclass
class ClaudeEvents:
    """Structured events extracted from a Claude Code session."""

    assistant_messages: list[str] = field(default_factory=list)
    tool_calls: list[dict] = field(default_factory=list)
    tool_results: dict[str, dict] = field(default_factory=dict)  # tool_use_id -> result
    skills_invoked: list[str] = field(default_factory=list)
    commands_run: list[str] = field(default_factory=list)
    files_read: list[str] = field(default_factory=list)
    files_created: list[str] = field(default_factory=list)
    files_modified: list[str] = field(default_factory=list)
    num_turns: int = 0
    duration_ms: int = 0
    input_tokens: int = 0
    output_tokens: int = 0
    commands_mentioned: list[str] = field(default_factory=list)

    @property
    def full_text(self) -> str:
        return "\n".join(self.assistant_messages)

    def summary(self) -> dict:
        return {
            "num_turns": self.num_turns,
            "duration_ms": self.duration_ms,
            "tool_calls_count": len(self.tool_calls),
            "skills_invoked": self.skills_invoked,
            "commands_run": self.commands_run[:20],
            "files_read": self.files_read[:20],
            "files_created": self.files_created[:10],
            "files_modified": self.files_modified[:10],
            "commands_mentioned": self.commands_mentioned[:20],
            "input_tokens": self.input_tokens,
            "output_tokens": self.output_tokens,
            "total_tokens": self.input_tokens + self.output_tokens,
        }


def extract_events(raw_output: str) -> ClaudeEvents:
    """Parse Claude's --output-format stream-json into ClaudeEvents."""
    events = ClaudeEvents()
    seen_tool_ids: set[str] = set()
    tool_id_to_index: dict[str, int] = {}

    for line in raw_output.strip().splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            obj = json.loads(line)
        except json.JSONDecodeError:
            continue

        msg_type = obj.get("type", "")

        # --- Result message: duration, turns, tokens ---
        if msg_type == "result":
            events.duration_ms = obj.get("duration_ms", 0)
            events.num_turns = obj.get("num_turns", events.num_turns)
            usage = obj.get("usage", {})
            events.input_tokens = usage.get("input_tokens", 0)
            events.output_tokens = usage.get("output_tokens", 0)
            continue

        # --- Assistant message: text + tool_use blocks ---
        if msg_type == "assistant" and "message" in obj:
            message = obj["message"]
            if isinstance(message, dict):
                for block in message.get("content", []):
                    if not isinstance(block, dict):
                        continue

                    if block.get("type") == "text":
                        events.assistant_messages.append(block["text"])

                    elif block.get("type") == "tool_use":
                        tool_id = block.get("id", "")
                        if tool_id in seen_tool_ids:
                            continue
                        seen_tool_ids.add(tool_id)

                        tool_name = block.get("name", "")
                        tool_input = block.get("input", {})
                        idx = len(events.tool_calls)
                        tool_id_to_index[tool_id] = idx
                        events.tool_calls.append({
                            "id": tool_id,
                            "name": tool_name,
                            "input": tool_input,
                        })

                        _process_tool_call(events, tool_name, tool_input)

                events.num_turns += 1

            elif isinstance(message, str):
                events.assistant_messages.append(message)
                events.num_turns += 1

        # --- User message: tool_result blocks ---
        if msg_type == "user" and "message" in obj:
            message = obj["message"]
            if isinstance(message, dict):
                for block in message.get("content", []):
                    if isinstance(block, dict) and block.get("type") == "tool_result":
                        tool_use_id = block.get("tool_use_id", "")
                        if tool_use_id:
                            events.tool_results[tool_use_id] = block

    # Extract teamcity commands from all text
    full = events.full_text
    events.commands_mentioned = extract_teamcity_commands(full)

    return events


def _process_tool_call(events: ClaudeEvents, name: str, inp: dict) -> None:
    """Classify a tool call and update the appropriate event lists."""
    if name == "Bash":
        cmd = inp.get("command", "")
        if cmd:
            events.commands_run.append(cmd)

    elif name == "Read":
        path = inp.get("file_path", "")
        if path:
            events.files_read.append(path)
            # Detect skill loading from file reads
            if ".claude/skills/" in path:
                skill_name = _extract_skill_name(path)
                if skill_name and skill_name not in events.skills_invoked:
                    events.skills_invoked.append(skill_name)

    elif name == "Write":
        path = inp.get("file_path", "")
        if path:
            events.files_created.append(path)

    elif name == "Edit":
        path = inp.get("file_path", "")
        if path:
            events.files_modified.append(path)

    elif name == "Skill":
        skill_name = inp.get("skill", "")
        if skill_name and skill_name not in events.skills_invoked:
            events.skills_invoked.append(skill_name)

    elif name == "Glob":
        pass  # file search, informational

    elif name == "Grep":
        pass  # content search, informational


def _extract_skill_name(path: str) -> str:
    """Extract skill name from a .claude/skills/<name>/... path."""
    marker = ".claude/skills/"
    idx = path.find(marker)
    if idx < 0:
        return ""
    rest = path[idx + len(marker):]
    parts = rest.split("/")
    return parts[0] if parts else ""


def extract_teamcity_commands(text: str) -> list[str]:
    """Find all `teamcity ...` command invocations in text."""
    pattern = r"(?:^|\n|`|\$\s*)(teamcity\s+[^\n`$]+)"
    matches = re.findall(pattern, text, re.MULTILINE)
    commands = []
    for m in matches:
        cmd = m.strip().rstrip("`").rstrip("\\").strip()
        if cmd and cmd not in commands:
            commands.append(cmd)
    return commands
