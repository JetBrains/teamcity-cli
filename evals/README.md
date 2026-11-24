# TeamCity CLI Skill Evals

A/B evaluation of the `teamcity-cli` agent skill: same model, same tasks, same seeded
TeamCity server, and the skill is the only variable.

| Treatment | What the agent gets                      |
|-----------|------------------------------------------|
| `CONTROL` | the `teamcity` CLI, no skill (baseline)  |
| `CURRENT` | the CLI plus the `teamcity-cli` skill    |

Runs execute in [Harbor](https://github.com/harbor-framework/harbor) on local Docker. The
skill is not in this repository: it comes from the `github.com/JetBrains/teamcity-skills`
module that `go.mod` pins (`scaffold/skill.py` resolves it), so a dependency bump changes
what `CURRENT` measures.

Every task asks a question with a known answer. The agent writes the answer to a file, the
verifier compares it with a reference the agent never sees, and the share it got right is the
trial's **outcome score**. The headline metric is the **paired skill lift**: per-task
`CURRENT − CONTROL` outcome score, averaged across tasks, with a bootstrap 95% CI.

## Quick start

Needs Docker (about 3 GB of free memory for the server), Go, [uv](https://docs.astral.sh/uv/)
and `uv tool install harbor`.

```bash
just eval-seed                          # once: build the seeded server snapshot (about 25 min)
just eval-validate                      # no model: oracle scores 1 and no-op scores 0 on every task
just eval -k 2                          # all tasks, both arms, 2 attempts each
just eval -i flaky-tests -i why-queued  # a subset
just eval-unit                          # harness unit tests (no server, no API)
just eval-diff results/A results/B      # compare two experiments
harbor view evals/results/<experiment>/jobs
```

`just eval` resolves `evals/.env` through 1Password (`op run`) and calls
`scripts/run-harbor.sh`, which:

1. starts the seeded server from its snapshot (`seed/server.sh up`, about a minute)
2. builds the CLI for Linux, regenerates `cli_schema.json` and builds the `teamcity-cli-eval`
   base image (`harbor/Dockerfile`)
3. runs a `CONTROL` job and a `CURRENT` job (`--skill`) with the `claude-code` agent
4. imports the trials (`scripts/harbor_import.py`): reward, outcome, safety and process
   results, the LLM judge, Langfuse
5. gates them (`scripts/compare.py`) and writes `report.html` (`scripts/report.py`)

Any extra arguments go to `harbor run` (`-k` attempts, `-n` concurrency, `-i`/`-x` tasks).

## Layout

```
evals/
├── seed/                # The seeded TeamCity server
│   ├── seed.py          #   builds the fixture and snapshots it as a Docker image
│   ├── repo/            #   CI scripts of the fixture's git repository
│   ├── answers.py       #   reads ground truth back from the server
│   ├── manifest.json    #   GENERATED: facts about the fixture and every task's reference answer
│   └── server.sh        #   starts/stops the snapshot for a run
├── tasks/<id>/          # One Harbor task per directory:
│   ├── instruction.md   #   the prompt, with the answer file and its schema
│   ├── task.toml        #   Harbor settings; harness options under [metadata]
│   ├── environment/     #   Dockerfile FROM teamcity-cli-eval + workspace/ files
│   ├── solution/        #   solve.sh: the oracle, derives the answer with the CLI
│   └── tests/           #   expected.json (GENERATED reference), checks.toml, test.sh
├── harbor/              # Base image, entrypoint, verifier, network overlay for trials
├── grade.py             # Outcome grading: answer file against the reference
├── checks.py            # CHECK_REGISTRY: safety gates and process checks on the trajectory
├── cli_schema.json      # GENERATED command/flag allowlist (gitignored)
├── scaffold/
│   ├── events.py        # ATIF → commands, tool calls, tokens, skills
│   ├── runner.py        # EvalRunner: quote-aware command parsing, results
│   ├── graders.py       # LLM judge (advisory, fails closed)
│   ├── langfuse_log.py  # Langfuse traces + scores (never used for gating)
│   ├── skill.py         # Resolves the pinned teamcity-cli skill
│   └── tasks.py         # Task loader
├── scripts/             # run-harbor.sh, validate-tasks.sh, harbor_import.py, compare.py, report.py
├── tests/               # Unit tests for the harness itself
└── results/<experiment>/  # jobs/ (Harbor), per-trial JSON, gate_summary.json, report.html
```

## The seeded server

The tasks query a local TeamCity server with fixed contents, so answers do not drift and no
production server is involved. `seed/seed.py` starts a clean `jetbrains/teamcity-server` (pinned
version) and three agents, then:

- creates two top-level projects (`Shop` with `Backend`, `Frontend` and `Release` subprojects,
  and `Infra`), 16 jobs, four agent pools and five users
- creates a git repository inside the server container and commits to it with fixed authors and
  dates on `main`, `feature/cart-redesign` and `release/2.4`
- runs about 90 builds in a fixed order. Build steps are scripts from that repository that
  report tests through service messages: unit tests break with one commit, integration tests
  fail by run number (flaky), a compile error is committed and reverted, and a composite job
  fails once through a direct dependency and once through a nested one
- starts builds as different users, through the VCS trigger, a schedule trigger and snapshot
  dependencies
- leaves four runs queued that can never start (unmet agent requirements), disables one agent,
  stops the agents and commits the server container as `teamcity-cli-eval-server:seeded`

Trials get a token of `eval-viewer`, a user with the Project viewer role only, so writes are
rejected by the server. One server serves a whole run: `seed/server.sh up` starts the snapshot
on the `tc-eval` Docker network as `http://teamcity:8111`, and `harbor/compose.network.yaml`
joins every trial container to that network. No agents run during an eval: history is read from
the database and the queued runs stay queued.

`seed/answers.py` reads the ground truth back over REST (it does not trust the seed plan) and
writes `seed/manifest.json` and each task's `tests/expected.json`. The manifest's fingerprint, a
hash of the fixture facts, is also a label on the snapshot image; `server.sh` refuses a snapshot whose fingerprint differs from
the checked-in manifest. Seeding is sequential, so run numbers, branches, statuses and commit
hashes come out the same on every seed, and a fresh seed reproduces the checked-in fingerprint.

## How a trial is scored

The verifier (`harbor/verify.py`) reports three things separately in `checks.json`:

1. **Outcome** (the reward). `grade.py` compares the answer file with `tests/expected.json`,
   one result per top-level field: strings case-insensitively, numbers by value, lists as
   multisets unless the task marks them ordered, commit hashes by prefix. The reward is the
   share of fields that match. Harbor copies `tests/` into the container only after the agent
   has finished, so the reference is not readable during the trial.
2. **Safety gates.** Read from the agent's ATIF trajectory; any failure sets the reward to 0:
   an unknown command or invented flag (validated against `cli_schema.json`, regenerated from
   the binary's cobra tree on every run, aliases included), a command that would change the
   server, a hand-edited `teamcity.toml`. `negative-unrelated` gates on using the CLI or the
   skill at all.
3. **Process checks** (advisory). Idiom checks such as "listed runs with a status filter". They
   are reported per arm and never enter the reward: an agent that answers correctly another way
   loses nothing.

Commands are tokenized, never substring-matched: quotes, loops, `$(…)`, line continuations,
comments, `timeout`/`env` wrappers and heredocs are handled.

A trial is a measurement only if the agent ran and left a trajectory. The verifier writes no
reward when it finds agent logs without a trajectory, and `harbor_import.py` leaves out trials
that errored or have no graded trajectory, lists them in `invalid_trials.json` and aborts when
they outnumber the valid ones.

The LLM judge (`llm_grade` tasks) and the efficiency numbers (tokens, turns, tool calls) stay
advisory and are reported next to the lift.

The CLI in each trial is logged in through its config file, not `TEAMCITY_*` env vars, the way
most agent sessions authenticate (keyring). The task passes the server as `TC_EVAL_URL` and
`TC_EVAL_TOKEN`; `harbor/entrypoint.sh` writes them into the CLI config and expands
`${TEAMCITY_URL}` in workspace files. The verifier shares the agent's container because a
separate verifier does not receive the trajectory; `harbor lint` flags this as `shared-verifier`.

## Validating tasks without a model

`just eval-validate` runs Harbor's `oracle` agent (each task's `solution/solve.sh`) and its `nop`
agent on every task and fails unless the oracle scores 1 and the no-op scores 0 everywhere. The
oracle derives its answer with the CLI inside the trial container, using the viewer token, so it
also proves that the answer is reachable with read-only access.

## The gate (`scripts/compare.py`)

Computed from the experiment's own artifacts, with no cross-build baseline.

- **BLOCK** if the paired-lift CI lower bound < `GATE_LIFT_FLOOR` (default −5 pts).
- **BLOCK** if a guardrail task (`CURRENT` only, e.g. `negative-unrelated`) scores below 1 on at
  least half its attempts.
- `GATE_MODE=warn` (default) reports the verdict but exits 0; `enforce` fails the build. Needs
  at least `GATE_MIN_TASKS` (3) paired tasks, so partial local runs just report.

Also reported: per-task `pass^2` (both attempts fully correct), the process-check pass rate,
judge means and mean tokens, turns and tool calls per arm (all advisory).

## Tasks

| Task                       | Question                                                        |
|----------------------------|-----------------------------------------------------------------|
| `investigate-failure`      | Failing tests, first failing run and culprit commit             |
| `daily-loop`               | Last 5 runs on main and the commit that broke the build         |
| `composite-failure`        | Root-cause jobs and tests of a composite job's failed runs      |
| `inspect-url`              | Health of the job behind a TeamCity URL                         |
| `find-builds`              | The 10 most recent failed runs of a project, in order           |
| `cross-project`            | Per subproject: jobs, never-run jobs, failing jobs              |
| `explore-infrastructure`   | Top-level projects, subprojects, pool and agent counts          |
| `hallucination-resistance` | Three lookups phrased to invite flags that don't exist          |
| `flaky-tests`              | Fan-out: tests failing in some of the last 20 runs, with counts |
| `test-history`             | Fan-out: one test's failures over a job's whole history         |
| `why-queued`               | Oldest queued run of a project and the requirement it misses    |
| `pool-capacity`            | A project's pools with agent and enabled-agent counts           |
| `disabled-agents`          | Disabled agents and their pools                                 |
| `settings-search`          | Fan-out: Docker images used by build steps across a project     |
| `trigger-history`          | Trigger sources of the last 30 runs of a job                    |
| `branch-status`            | Latest run status per branch                                    |
| `locate-and-link`          | Bind the repository to a project (graded on `teamcity.toml`)    |
| `use-repository-link`      | Answer from the project an existing `teamcity.toml` links       |
| `negative-unrelated`       | Guardrail: skill must NOT fire on unrelated work                |

## Environment variables

| Variable             | Required | Description                                                  |
|----------------------|----------|--------------------------------------------------------------|
| `ANTHROPIC_API_KEY`  | Yes      | Agent (inside Harbor) and LLM judge                          |
| `EVAL_MODEL`         | No       | Harbor model slug (default `anthropic/claude-sonnet-5-5`)    |
| `BRANCH_NAME`        | No       | Names the experiment directory (default: a timestamp)        |
| `TC_EVAL_PORT`       | No       | Host port of the seeded server (default `18111`)             |
| `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`, `LANGFUSE_HOST` | No | Langfuse traces + scores |
| `LANGFUSE_TRACING_ENVIRONMENT` | No | Langfuse environment (default `eval`; CI sets `ci`)  |
| `GATE_MODE`          | No       | `warn` (default) or `enforce`                                |
| `GATE_LIFT_FLOOR`    | No       | Lift CI lower-bound threshold (default `-0.05`)              |

## CI pipeline

`TeamCity_TeamCityCLI_SkillEval` on teamcity-nightly; the pipeline YAML lives server-side
(`teamcity pipeline pull`, edit, `teamcity pipeline validate`, `teamcity pipeline push`). Per
run: build the CLI, skip with a `SKIPPED` status unless `evals/`, the schema generator or
`go.mod`/`go.sum` changed, validate the bundled skills, then `evals/scripts/run-harbor.sh -k 2`.
Results, `jobs/` and `report.html` are published as artifacts. The pipeline does not have the
snapshot image yet: it has to run `just eval-seed` on its agent or pull the image from a registry.

## Langfuse

When the Langfuse keys are set, each trial is pushed as a trace (name = task, `user_id` =
treatment, `session_id` = experiment) with one `tool` span per tool call and scores for
`pass_rate` (the reward), `passed`, `pass_bucket`, `skill_invoked`, `duration_sec`, `num_turns`,
`total_tokens`, `judge:<dim>` and one per outcome field, safety gate and process check.
Langfuse is observability only; gating runs on the local `results/` artifacts.

## Adding a task

1. Copy a directory under `tasks/`. In `instruction.md` ask for facts the fixture fixes and say
   where the answer goes and in what shape (`answer.json` with named fields). Set `[task] name`
   in `task.toml`; under `[metadata]`, `treatments = ["CURRENT"]` makes it a guardrail.
2. Add the reference to `task_answers()` in `seed/answers.py`, computed from the server, and
   regenerate the references: `evals/seed/server.sh up`, then `python3 evals/seed/answers.py`.
   If the fixture lacks the data, extend `seed/seed.py` and run `just eval-seed`; that changes
   the fingerprint, so everyone has to re-seed.
3. Write `solution/solve.sh` so that it finds the answer with the CLI, not by copying it.
4. List safety gates and optional process checks in `tests/checks.toml`. New check functions go
   in `checks.py`; use `runner.has_subcommand(...)`/`runner.has_flag(...)` rather than substring
   matching, and add a known-bad fixture for each in `tests/test_checks.py`.
5. Run `just eval-validate -i <id>` and `harbor lint evals/tasks/<id>/`.
