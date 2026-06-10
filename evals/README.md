# TeamCity CLI Skill Evals

A/B evaluation of the `teamcity-cli` agent skill: same model, same tasks, same
live server — the skill is the only variable.

| Treatment | What Claude Code gets                   |
|-----------|------------------------------------------|
| `CONTROL` | the `teamcity` CLI, no skill — baseline  |
| `CURRENT` | the CLI **+** `skills/teamcity-cli/`     |

The headline metric is the **paired skill lift**: per-task
`CURRENT − CONTROL` deterministic pass rate, averaged across tasks, with a
bootstrap 95% CI. Because both arms run in the same session against the same
server, live-data drift cancels out of the difference.

## Quick start

```bash
just eval-setup                  # install dependencies
cp evals/.env.example evals/.env # configure (1Password refs or literals)

just eval --runs=2 -n 8          # full suite, both treatments
just eval --task=investigate-failure --treatment=CURRENT
just eval-unit                   # harness unit tests only (no server, no API)
just eval-diff                   # gate/report the current branch's results
just eval-diff results/A results/B   # informational A/B diff
```

## Architecture

```
evals/
├── tasks.json           # Task prompts + per-task check lists + treatments
├── checks.py            # CHECK_REGISTRY — deterministic check functions
├── cli_schema.json      # GENERATED command/flag allowlist (gitignored)
├── conftest.py          # Treatments, parametrization, provenance, schema gen
├── pipeline.yml         # Versioned source for the SkillEval CI pipeline
├── scaffold/
│   ├── claude.py        # Runs Claude Code with an isolated HOME per run
│   ├── events.py        # stream-json → commands/tools/tokens/skills
│   ├── runner.py        # EvalRunner: tokenized command matching, results
│   ├── graders.py       # LLM judge (advisory, fails closed)
│   └── sentry_log.py    # Observability traces (never used for gating)
├── scripts/compare.py   # Statistical gate: paired lift ± CI, pass^k
├── tests/test_tasks.py  # The eval runner (pytest parametrizes task×treatment)
├── tests/test_checks.py # Unit tests for the harness itself
└── results/<experiment>/  # Per-run JSON artifacts + gate_summary.json
```

## How a run is scored

1. **Deterministic checks** (gate-relevant). Executed commands are tokenized —
   never substring-matched — and validated against `cli_schema.json`, which is
   regenerated from the binary's cobra command tree on every session
   (`go run scripts/generate-cli-schema.go`). An invented flag is a failure;
   a real flag can never be misflagged, because the allowlist cannot rot.
2. **LLM judge** (advisory). Sonnet grades subjective dimensions with anchored
   rubrics. Judge scores are reported separately and **never** blend into the
   deterministic pass rate. If the judge is unavailable the dimension is
   recorded as *ungraded* — it fails closed, not open.
3. **Metadata**, not scores: whether the skill was available (treatment) vs
   actually invoked (observed), full provenance (git SHA, CLI/claude/skill
   versions, model, task-set hash).

**Pytest pass/fail means harness health only.** A CONTROL run scoring zero
checks is a *measurement* — the baseline being weak is the point, not a test
failure. Only harness breakage (claude crashed, unparseable output) fails
pytest.

## The gate (`scripts/compare.py`)

Self-contained: computed from the session's own artifacts, no cross-build
baseline, identical on `main` and branches.

- **BLOCK** if the paired-lift CI lower bound < `GATE_LIFT_FLOOR` (default
  −5 pts) — the skill stopped helping.
- **BLOCK** if a guardrail task (single-treatment, e.g. `negative-unrelated`)
  fails on ≥ half its reps — the skill misfires on unrelated work.
- `GATE_MODE=warn` (default) reports the verdict but exits 0; flip to
  `enforce` once thresholds have soaked. Needs ≥ `GATE_MIN_TASKS` (3) paired
  tasks, so partial local runs just report.

Also reported: per-task `pass^2` (probability both reps pass — reliability,
not just average), judge means per arm, and `gate_summary.json` for trend
tooling.

## Tasks

| Task                       | What It Tests                                       |
|----------------------------|-----------------------------------------------------|
| `investigate-failure`      | Full failure workflow: find → log → tests → changes |
| `daily-loop`               | Branch triage: list, drill into failures, changes   |
| `composite-failure`        | Dependency-tree drill-down on a composite build     |
| `inspect-url`              | Parse a TC URL and inspect the build/config         |
| `find-builds`              | Complex search with filters, JSON output            |
| `cross-project`            | Multi-project CI health summary                     |
| `explore-infrastructure`   | Navigate projects, pools, hierarchy                 |
| `hallucination-resistance` | Resist inventing flags that don't exist             |
| `negative-unrelated`       | Guardrail: skill must NOT fire on unrelated work    |

## Environment Variables

| Variable             | Required | Description                                              |
|----------------------|----------|----------------------------------------------------------|
| `ANTHROPIC_API_KEY`  | Yes      | Claude API key (agent + judge)                           |
| `TEAMCITY_URL`       | Yes      | TeamCity server URL                                      |
| `TEAMCITY_TOKEN`     | Yes      | TeamCity API token                                       |
| `SENTRY_DSN`         | No       | Ingest-only — sends observability traces to Sentry       |
| `SENTRY_ENVIRONMENT` | No       | Sentry environment tag (default: `eval`)                 |
| `BENCH_CC_MODEL`     | No       | Claude model (default: `claude-sonnet-4-5-20250929`)     |
| `BENCH_TIMEOUT`      | No       | Task timeout in seconds (default: 300)                   |
| `BENCH_LOCAL`        | No       | Set to `1` to skip Docker and run Claude locally         |
| `GATE_MODE`          | No       | `warn` (default) or `enforce`                            |
| `GATE_LIFT_FLOOR`    | No       | Lift CI lower-bound threshold (default `-0.05`)          |

## CI pipeline

`TeamCity_TeamCityCLI_SkillEval` on teamcity-nightly, defined by
`evals/pipeline.yml` (edit → `teamcity pipeline validate` → `teamcity
pipeline push`). Per run: build the CLI from the commit under test →
change-gate (skips with an explicit `SKIPPED` build status unless
`skills/teamcity-cli/`, `evals/`, or the schema generator changed) → pinned
`@anthropic-ai/claude-code` install → schema generation → `pytest tests/
--runs=2 -n 8` → gate. Pytest and the gate exit codes both propagate — nothing
is `|| true`'d away.

Known gaps (deliberate, next phases): no nightly schedule (pipelines YAML has
no trigger support — add via UI), live server instead of record/replay
cassettes, judge uncalibrated against human labels.

## Sentry

When `SENTRY_DSN` is set, each run is pushed as a `gen_ai.invoke_agent`
transaction (one `gen_ai.execute_tool` child span per tool call) tagged with
`experiment_id`/`task`/`treatment`/`skill_available`/`skill_invoked` plus full
provenance. Sentry is dashboards and drill-down only — gating always runs on
the local `results/` artifacts, which are the authoritative record.

## Adding a task

1. Add an entry to `tasks.json`: `id`, `prompt`, `checks` (IDs from
   `CHECK_REGISTRY`), optional `llm_grade`, optional `treatments` (omit for
   both arms; `["CURRENT"]` makes it a guardrail).
2. New check functions go in `checks.py` and the registry. Use
   `runner.has_subcommand(...)`/`runner.has_flag(...)` — don't substring-match
   command lines.
3. Add adversarial coverage in `tests/test_checks.py`: the check must fail on
   known-bad fixtures, not just pass on good ones.
