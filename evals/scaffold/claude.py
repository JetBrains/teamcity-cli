"""Claude Code execution — runs Claude CLI and captures output."""

from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path

# Claude binary path. Set CLAUDE_BIN env var in CI if claude isn't on PATH.
_CLAUDE_BIN = os.environ.get("CLAUDE_BIN") or shutil.which("claude") or "claude"

from typing import Protocol


class Treatment(Protocol):
    skill_dir: Path | None


@dataclass
class ClaudeResult:
    exit_code: int
    raw_output: str
    duration_sec: float = 0.0


# Env vars to propagate into Claude's subprocess
_PROPAGATE_KEYS = (
    "ANTHROPIC_API_KEY",
    "TEAMCITY_URL",
    "TEAMCITY_TOKEN",
    "LANGSMITH_TRACING",
    "LANGSMITH_API_KEY",
    "LANGSMITH_ENDPOINT",
    "LANGSMITH_PROJECT",
)


def _setup_workspace(
    work_dir: Path,
    treatment: Treatment,
) -> None:
    """Write skill files into the workspace."""
    if treatment.skill_dir and treatment.skill_dir.exists():
        dest = work_dir / ".claude" / "skills" / "teamcity-cli"
        shutil.copytree(treatment.skill_dir, dest)


def _build_isolated_env(work_dir: Path) -> dict[str, str]:
    """Build a clean env that isolates Claude from global config.

    Sets HOME and CLAUDE_CONFIG_DIR to the workspace so Claude Code
    does NOT pick up ~/.claude/ skills, plugins, or settings.
    This is critical for CONTROL runs — without isolation, globally
    installed skills leak into the baseline and invalidate comparisons.
    """
    env: dict[str, str] = {}

    # Minimal env for subprocess to work
    for key in ("PATH", "SHELL", "TERM", "USER", "LANG", "LC_ALL",
                "SSL_CERT_FILE", "NODE_EXTRA_CA_CERTS", "REQUESTS_CA_BUNDLE"):
        val = os.environ.get(key)
        if val:
            env[key] = val

    # Isolate Claude config — this is the key fix
    env["HOME"] = str(work_dir)
    env["USERPROFILE"] = str(work_dir)  # Windows

    # Propagate required API keys and tracing vars
    for key in _PROPAGATE_KEYS:
        val = os.environ.get(key)
        if val:
            env[key] = val

    # Suppress color for cleaner parsing
    env["NO_COLOR"] = "1"

    return env


def run_claude(
    prompt: str,
    treatment: Treatment,
    *,
    model: str | None = None,
    timeout: int = 300,
) -> ClaudeResult:
    """Run Claude Code CLI locally in a temp directory with isolated config."""
    model = model or os.environ.get("BENCH_CC_MODEL", "claude-sonnet-4-5-20250929")

    with tempfile.TemporaryDirectory(prefix="tc-eval-") as tmp:
        work_dir = Path(tmp)
        _setup_workspace(work_dir, treatment)
        env = _build_isolated_env(work_dir)

        cmd = [
            _CLAUDE_BIN,
            "-p", prompt,
            "--dangerously-skip-permissions",
            "--output-format", "stream-json",
            "--model", model,
            "--verbose",
        ]

        start = time.monotonic()
        try:
            result = subprocess.run(
                cmd,
                cwd=work_dir,
                capture_output=True,
                text=True,
                timeout=timeout + 30,
                env=env,
            )
            elapsed = time.monotonic() - start
            return ClaudeResult(
                exit_code=result.returncode,
                raw_output=result.stdout,
                duration_sec=elapsed,
            )
        except subprocess.TimeoutExpired:
            elapsed = time.monotonic() - start
            return ClaudeResult(
                exit_code=124,
                raw_output=f"Timeout after {timeout}s",
                duration_sec=elapsed,
            )


def run_claude_docker(
    prompt: str,
    treatment: Treatment,
    *,
    model: str | None = None,
    timeout: int = 300,
    image: str = "tc-skill-eval",
) -> ClaudeResult:
    """Run Claude Code CLI inside a Docker container (fully isolated)."""
    model = model or os.environ.get("BENCH_CC_MODEL", "claude-sonnet-4-5-20250929")

    with tempfile.TemporaryDirectory(prefix="tc-eval-") as tmp:
        work_dir = Path(tmp)
        _setup_workspace(work_dir, treatment)

        env_flags: list[str] = []
        for key in _PROPAGATE_KEYS:
            val = os.environ.get(key)
            if val:
                env_flags += ["-e", f"{key}={val}"]
        env_flags += ["-e", "NO_COLOR=1"]

        cmd = [
            "docker", "run", "--rm",
            "-v", f"{work_dir}:/workspace",
            "-w", "/workspace",
            *env_flags,
            image,
            "-p", prompt,
            "--dangerously-skip-permissions",
            "--output-format", "stream-json",
            "--model", model,
            "--verbose",
        ]

        start = time.monotonic()
        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=timeout + 60,
            )
            elapsed = time.monotonic() - start
            return ClaudeResult(
                exit_code=result.returncode,
                raw_output=result.stdout,
                duration_sec=elapsed,
            )
        except subprocess.TimeoutExpired:
            elapsed = time.monotonic() - start
            return ClaudeResult(
                exit_code=124,
                raw_output=f"Timeout after {timeout}s",
                duration_sec=elapsed,
            )


def build_docker_image(image: str = "tc-skill-eval") -> None:
    """Build the eval Docker image if not already present."""
    dockerfile = Path(__file__).resolve().parent.parent / "Dockerfile"
    subprocess.run(
        ["docker", "build", "-t", image, "-f", str(dockerfile), "."],
        cwd=dockerfile.parent,
        check=True,
        capture_output=True,
    )
