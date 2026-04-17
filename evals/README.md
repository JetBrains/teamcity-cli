# TeamCity CLI Skill Evals

Evaluation pipeline for the `teamcity-cli` agent skill, based on the
[LangChain skills-benchmarks](https://blog.langchain.com/evaluating-skills/) pattern.

Measures how well Claude Code performs TeamCity tasks **with** vs **without** the skill.

## Quick Start

```bash
# Install dependencies
just eval-setup

# Configure (copy and fill in API keys)
cp evals/.env.example evals/.env

# Run all evals
just eval

# Run a specific task
just eval-task investigate-failure

# Compare CONTROL (no skill) vs CURRENT (with skill)
just eval-compare investigate-failure

# Multiple runs for confidence
just eval-bench investigate-failure 3
```

## Architecture

```
evals/
├── tasks/                 # Benchmark tasks (one dir per task)
│   ├── investigate-failure/
│   │   ├── task.toml      # Metadata, default treatments
│   │   ├── instruction.md # Prompt given to Claude
│   │   └── validation/    # Check functions
│   ├── inspect-url/
│   ├── find-builds/
│   ├── explore-infrastructure/
│   └── hallucination-resistance/
├── treatments/            # Skill configurations
│   └── common.yaml        # CONTROL (no skill), CURRENT (production skill)
├── scaffold/              # Framework: runner, events, validation
├── tests/test_tasks.py    # Pytest runner
├── conftest.py            # Fixtures, parametrization, Sentry init
├── Dockerfile             # Claude Code container
└── results/               # JSON artifacts per run
```

## How It Works

1. **Task** = a realistic TeamCity prompt (e.g., "find a recent failure and diagnose it")
2. **Treatment** = skill configuration (CONTROL = no skill, CURRENT = production skill)
3. **Execution** = Claude Code runs the task in Docker (or locally with `BENCH_LOCAL=1`)
4. **Validation** = check functions verify Claude used correct commands and produced useful output
5. **Tracking** = results logged to Sentry as AI-Agent traces (spans + measurements + tags)

## Tasks

| Task                       | What It Tests                                       |
|----------------------------|-----------------------------------------------------|
| `investigate-failure`      | Full failure workflow: find → log → tests → changes |
| `inspect-url`              | Parse a TC URL and inspect the build/config         |
| `find-builds`              | Complex search with filters, JSON output            |
| `explore-infrastructure`   | Navigate projects, pools, hierarchy                 |
| `hallucination-resistance` | Resist inventing flags that don't exist             |

## Treatments

| Treatment | Description                       |
|-----------|-----------------------------------|
| `CONTROL` | No skill loaded — baseline        |
| `CURRENT` | Production `skills/teamcity-cli/` |

Add variants in `treatments/common.yaml` to A/B test skill changes.

## Environment Variables

| Variable             | Required | Description                                                                          |
|----------------------|----------|--------------------------------------------------------------------------------------|
| `ANTHROPIC_API_KEY`  | Yes      | Claude API key                                                                       |
| `TEAMCITY_URL`       | Yes      | TeamCity server URL                                                                  |
| `TEAMCITY_TOKEN`     | Yes      | TeamCity API token                                                                   |
| `SENTRY_DSN`         | No       | Ingest-only — sends traces to Sentry. Cannot read back.                              |
| `SENTRY_ORG`         | No       | Org slug for `scripts/compare.py` (DSN doesn't carry org scope)                      |
| `SENTRY_AUTH_TOKEN`  | No       | Bearer token with `event:read` + `org:read` for compare                              |
| `SENTRY_ENVIRONMENT` | No       | Sentry environment tag (default: `eval`)                                             |
| `BENCH_CC_MODEL`     | No       | Claude model (default: `claude-sonnet-4-5-20250929`)                                 |
| `BENCH_TIMEOUT`      | No       | Task timeout in seconds (default: 300)                                               |
| `BENCH_LOCAL`        | No       | Set to `1` to skip Docker and run Claude locally                                     |

## Adding a Task

1. Create `tasks/<name>/task.toml`:
   ```toml
   [metadata]
   name = "my-task"
   description = "What this tests"
   default_treatments = ["CONTROL", "CURRENT"]

   [validation]
   checks_module = "test_my_task"
   ```

2. Create `tasks/<name>/instruction.md` with the prompt

3. Create `tasks/<name>/validation/test_my_task.py`:
   ```python
   from scaffold.runner import TestRunner

   def check_something(runner: TestRunner) -> None:
       if runner.has_command("run", "list"):
           runner.passed("Used correct command")
       else:
           runner.failed("Missing expected command")

   CHECKS = [check_something]
   ```

## Adding a Treatment

Add to `treatments/common.yaml`:

```yaml
MY_VARIANT:
  description: "Skill with improved failure workflow"
  skill_dir: "teamcity-cli"  # or a custom skill directory
```

## Sentry Integration

When `SENTRY_DSN` is set, each eval run is pushed to Sentry as a `gen_ai.invoke_agent`
transaction with one `gen_ai.execute_tool` child span per Claude tool call.

Per-run signal stored on the transaction:
- **Tags** (filterable): `experiment_id` (= branch), `task`, `treatment`, `skill_invoked`
- **Measurements** (aggregatable): `pass_rate`, `duration_sec`, `num_turns`, `total_tokens`
- **Data**: full instruction, response text, check results, LLM grades, tool inputs/outputs

### DSN vs. auth token

The **DSN is ingest-only** — it can send events to Sentry but cannot query them back.
To drive `scripts/compare.py` off Sentry you also need:

- `SENTRY_ORG` — your org slug (the DSN encodes an org *ID*, not a slug)
- `SENTRY_AUTH_TOKEN` — a Bearer token with `event:read` + `org:read` scopes
  (create at `https://{org}.sentry.io/settings/account/api/auth-tokens/`)

Without those, write still works (traces flow to Sentry for humans to browse in the
AI Agent Monitoring view); compare falls back to local-directory diff mode.

### Caveats

- Sentry event retention is 30–90d depending on plan. The local `results/<experiment_id>/*.json`
  dump is the authoritative cold store.
- Sampling is pinned to `1.0` for eval runs.
- Each transaction is capped at 10 measurements; we use 4.
