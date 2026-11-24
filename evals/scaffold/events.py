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
    result_totals = [0, 0, 0, 0]  # turns, duration_ms, input_tokens, output_tokens
    saw_result = False

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
        # A session can emit several (a background task wakes it for another
        # turn), and each reports only its own segment, so they add up.
        if msg_type == "result":
            usage = obj.get("usage", {})
            result_totals[0] += obj.get("num_turns", 0)
            result_totals[1] += obj.get("duration_ms", 0)
            result_totals[2] += usage.get("input_tokens", 0)
            result_totals[3] += usage.get("output_tokens", 0)
            saw_result = True
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

    if saw_result:
        events.num_turns, events.duration_ms, events.input_tokens, events.output_tokens = result_totals

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


# Shell tools across harnesses (Claude Code, Codex, Gemini CLI, opencode, pi, Junie, ...).
_SHELL_TOOLS = {"bash", "shell", "exec_command", "run_shell_command", "execute_command", "terminal", "run_command"}


def _shell_command(arguments: dict) -> str:
    cmd = arguments.get("command") or arguments.get("cmd") or ""
    if isinstance(cmd, list):  # Codex passes argv, often ["bash", "-lc", "<script>"]
        cmd = cmd[-1] if len(cmd) >= 3 and cmd[1] in ("-c", "-lc") else " ".join(map(str, cmd))
    return str(cmd)


def _text(message) -> str:
    if isinstance(message, str):
        return message
    if isinstance(message, list):  # ContentPart array
        return "\n".join(p.get("text", "") for p in message if isinstance(p, dict) and p.get("type") == "text")
    return ""


def extract_events_atif(trajectory: dict) -> ClaudeEvents:
    """Parse a Harbor ATIF trajectory into ClaudeEvents, so the same checks grade every harness."""
    events = ClaudeEvents()
    for step in trajectory.get("steps", []):
        if step.get("source") != "agent":
            continue
        events.num_turns += 1
        if text := _text(step.get("message")):
            events.assistant_messages.append(text)
        for call in step.get("tool_calls") or []:
            name, args = call.get("function_name", ""), call.get("arguments") or {}
            events.tool_calls.append({"id": call.get("tool_call_id", ""), "name": name, "input": args})
            if name.lower() in _SHELL_TOOLS:
                _process_tool_call(events, "Bash", {"command": _shell_command(args)})
            else:
                _process_tool_call(events, name, args)
    final = trajectory.get("final_metrics") or {}
    events.input_tokens = final.get("total_prompt_tokens") or 0
    events.output_tokens = final.get("total_completion_tokens") or 0
    events.commands_mentioned = extract_teamcity_commands(events.full_text)
    return events
