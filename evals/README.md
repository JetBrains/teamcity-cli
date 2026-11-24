# TeamCity CLI Skill Evals

A/B evaluation of the `teamcity-cli` agent skill: same model, same tasks, same
live server, and the skill is the only variable.

| Treatment | What the agent gets                      |
|-----------|------------------------------------------|
| `CONTROL` | the `teamcity` CLI, no skill (baseline)  |
| `CURRENT` | the CLI plus the `teamcity-cli` skill    |

Runs execute in [Harbor](https://github.com/harbor-framework/harbor) on local Docker. The
skill is not in this repository: it comes from the `github.com/JetBrains/teamcity-skills`
module that `go.mod` pins (`scaffold/skill.py` resolves it), so a dependency bump changes
what `CURRENT` measures.

The headline metric is the **paired skill lift**: per-task `CURRENT − CONTROL` deterministic
pass rate, averaged across tasks, with a bootstrap 95% CI. Both arms run back to back against
the same server, so live-data drift mostly cancels out of the difference.

## Quick start

Needs Docker, Go, [uv](https://docs.astral.sh/uv/) and `uv tool install harbor`.

```bash
just eval -k 2                          # all tasks, both arms, 2 attempts each
just eval -i flaky-tests -i why-queued  # a subset
just eval-unit                          # harness unit tests (no server, no API)
just eval-diff results/A results/B      # compare two experiments
harbor view evals/results/<experiment>/jobs
```

`just eval` resolves `evals/.env` through 1Password (`op run`) and calls
`scripts/run-harbor.sh`, which:

1. builds the CLI for Linux, regenerates `cli_schema.json` and builds the `teamcity-cli-eval`
   base image (`harbor/Dockerfile`)
2. runs a `CONTROL` job and a `CURRENT` job (`--skill`) with the `claude-code` agent
3. imports the trials (`scripts/harbor_import.py`): per-check results, the LLM judge, Langfuse
4. gates them (`scripts/compare.py`) and writes `report.html` (`scripts/report.py`)

Any extra arguments go to `harbor run` (`-k` attempts, `-n` concurrency, `-i`/`-x` tasks).

## Layout

```
evals/
├── tasks/<id>/          # One Harbor task per directory:
│   ├── instruction.md   #   the prompt
│   ├── task.toml        #   Harbor settings; harness options under [metadata]
│   ├── environment/     #   Dockerfile FROM teamcity-cli-eval + seeded workspace/ files
│   └── tests/           #   checks.toml (check IDs) + test.sh (verifier)
├── harbor/              # Base image, entrypoint, verifier (ATIF trajectory → checks → reward)
├── checks.py            # CHECK_REGISTRY: deterministic check functions
├── cli_schema.json      # GENERATED command/flag allowlist (gitignored)
├── scaffold/
│   ├── events.py        # ATIF → commands, tool calls, tokens, skills
│   ├── runner.py        # EvalRunner: quote-aware command parsing, results
│   ├── graders.py       # LLM judge (advisory, fails closed)
│   ├── langfuse_log.py  # Langfuse traces + scores (never used for gating)
│   ├── skill.py         # Resolves the pinned teamcity-cli skill
│   └── tasks.py         # Task loader
├── scripts/             # run-harbor.sh, harbor_import.py, compare.py, report.py
├── tests/test_checks.py # Unit tests for the harness itself
└── results/<experiment>/  # jobs/ (Harbor), per-trial JSON, gate_summary.json, report.html
```

## How a run is scored

1. **Deterministic checks** (gate-relevant). The verifier reads the agent's ATIF trajectory
   and tokenizes every shell command it ran, never substring-matching: quotes, loops,
   `$(…)`, line continuations, comments, `timeout`/`env` wrappers and heredocs are handled,
   and commands inside scripts the agent wrote count for fan-out checks. Flags are validated
   against `cli_schema.json`, regenerated from the binary's cobra tree on every run, so an
   invented flag fails and a real one can't be misflagged. The Harbor reward is the check
   pass rate.
2. **LLM judge** (advisory). For `llm_grade` tasks, Sonnet grades subjective dimensions with
   anchored rubrics after the run. Judge scores are reported separately and never blend into
   the pass rate; an unavailable judge leaves the dimension ungraded.
3. **Metadata**: whether the skill was available and actually invoked, model, agent, tokens,
   turns, duration.

The CLI in each trial is logged in through its config file, not `TEAMCITY_*` env vars, the way
most agent sessions authenticate (keyring). The task passes the server as `TC_EVAL_URL` and
`TC_EVAL_TOKEN`; `harbor/entrypoint.sh` writes them into the CLI config and expands
`${TEAMCITY_URL}` in seeded workspace files. The verifier shares the agent's container because a
separate verifier does not receive the trajectory; `harbor lint` flags this as `shared-verifier`.

## The gate (`scripts/compare.py`)

Computed from the experiment's own artifacts, with no cross-build baseline.

- **BLOCK** if the paired-lift CI lower bound < `GATE_LIFT_FLOOR` (default −5 pts).
- **BLOCK** if a guardrail task (`CURRENT` only, e.g. `negative-unrelated`) fails on at least
  half its attempts.
- `GATE_MODE=warn` (default) reports the verdict but exits 0; `enforce` fails the build. Needs
  at least `GATE_MIN_TASKS` (3) paired tasks, so partial local runs just report.

Also reported: per-task `pass^2`, judge means per arm, and mean tokens, turns and tool calls
per arm (advisory: a skill that documents more can cost more).

## Tasks

| Task                       | What it tests                                       |
|----------------------------|-----------------------------------------------------|
| `investigate-failure`      | Full failure workflow: find → log → tests → changes |
| `daily-loop`               | Branch triage: list, drill into failures, changes   |
| `composite-failure`        | Dependency-tree drill-down on a composite build     |
| `inspect-url`              | Parse a TC URL and inspect the build/config         |
| `find-builds`              | Complex search with filters, JSON output            |
| `cross-project`            | Multi-project CI health summary                     |
| `explore-infrastructure`   | Navigate projects, pools, hierarchy                 |
| `hallucination-resistance` | Resist inventing flags that don't exist             |
| `flaky-tests`              | Fan-out: failed tests aggregated across 20 runs     |
| `why-queued`               | Fan-out: oldest queued run, wait reason, agents     |
| `pool-capacity`            | Fan-out: a project's pools, agent load and OS       |
| `settings-search`          | Fan-out: build steps across every job in a project  |
| `trigger-history`          | Trigger sources; one run listing should suffice     |
| `negative-unrelated`       | Guardrail: skill must NOT fire on unrelated work    |

The fan-out tasks come from real agent sessions: questions whose answer spans many runs, jobs
or agents, where agents mostly wrote loops over the CLI or `teamcity api`.

## Environment variables

| Variable             | Required | Description                                                  |
|----------------------|----------|--------------------------------------------------------------|
| `ANTHROPIC_API_KEY`  | Yes      | Agent (inside Harbor) and LLM judge                          |
| `TEAMCITY_URL`       | Yes      | Eval server URL                                              |
| `TEAMCITY_TOKEN`     | Yes      | Eval server token                                            |
| `EVAL_MODEL`         | No       | Harbor model slug (default `anthropic/claude-sonnet-5-5`)    |
| `BRANCH_NAME`        | No       | Names the experiment directory (default: a timestamp)        |
| `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`, `LANGFUSE_HOST` | No | Langfuse traces + scores |
| `LANGFUSE_TRACING_ENVIRONMENT` | No | Langfuse environment (default `eval`; CI sets `ci`)  |
| `GATE_MODE`          | No       | `warn` (default) or `enforce`                                |
| `GATE_LIFT_FLOOR`    | No       | Lift CI lower-bound threshold (default `-0.05`)              |

## CI pipeline

`TeamCity_TeamCityCLI_SkillEval` on teamcity-nightly; the pipeline YAML lives server-side
(`teamcity pipeline pull`, edit, `teamcity pipeline validate`, `teamcity pipeline push`). Per
run: build the CLI, skip with a `SKIPPED` status unless `evals/`, the schema generator or
`go.mod`/`go.sum` changed, validate the bundled skills, then `evals/scripts/run-harbor.sh -k 2`.
Results, `jobs/` and `report.html` are published as artifacts.

## Langfuse

When the Langfuse keys are set, each trial is pushed as a trace (name = task, `user_id` =
treatment, `session_id` = experiment) with one `tool` span per tool call and scores for
`pass_rate`, `passed`, `pass_bucket`, `skill_invoked`, `duration_sec`, `num_turns`,
`total_tokens` and `judge:<dim>`. Langfuse is observability only; gating runs on the local
`results/` artifacts.

## Adding a task

1. Copy a directory under `tasks/` and edit `instruction.md`, `tests/checks.toml` (check IDs
   from `CHECK_REGISTRY`) and `task.toml` (`[task] name`; `[metadata]` `llm_grade`,
   `treatments`: omit for both arms, `["CURRENT"]` makes it a guardrail). Files the agent's
   working directory needs go in `environment/workspace/`.
2. New check functions go in `checks.py` and the registry. Use
   `runner.has_subcommand(...)`/`runner.has_flag(...)` rather than substring matching.
3. Add adversarial coverage in `tests/test_checks.py`: the check must fail on known-bad
   fixtures, not just pass on good ones.
4. Run `harbor lint evals/tasks/<id>/` and the no-op agent (`harbor run -p evals/tasks/<id> -a nop`).
