# Agent Instructions

Read [`CONTRIBUTING.md`](CONTRIBUTING.md) — it is the source of truth for
architecture, conventions, testing, linting, and the before-pushing checklist.
Everything below is additive guidance for AI agents only.

## Quick reference

```sh
just build          # go build → bin/teamcity
just install        # go install ./tc → $GOPATH/bin/teamcity
just lint           # go fmt + go fix + golangci-lint
just unit           # unit tests
just test           # unit + integration (testcontainers)
just acceptance     # e2e against cli.teamcity.com (-tags=acceptance)
just snapshot       # goreleaser local snapshot (all platforms)
just docs-generate  # regenerate CLI command reference
just record-gifs <name>  # record GIF from docs/tapes/<name>.tape → docs/images/
```

## Commits and PRs

- Don't commit unless asked.
- Conventional format: `feat(scope):`, `fix(scope):`, `refactor(scope):`.
- **Always respect `.github/PULL_REQUEST_TEMPLATE.md` when opening a PR.** Read the
  template before writing the body, fill in every section it defines (don't drop
  any), and don't invent extra sections. If a section doesn't apply, say so
  explicitly rather than silently omitting it.

## Terminology

| TeamCity concept    | CLI noun |
|---------------------|----------|
| Build               | `run`    |
| Build configuration | `job`    |
| Build agent         | `agent`  |
| Build queue         | `queue`  |
| Agent pool          | `pool`   |

## Filing Issues

- **Always check `.github/ISSUE_TEMPLATE/` before creating an issue.** This repo has
  `blank_issues_enabled: false` — every issue must use a template. Match the template
  to the issue type (bug, feature, eval task).
- **Follow the template structure exactly.** Fill in each section as defined in the YAML
  fields. Do not add extra sections, root-cause analysis, or fix suggestions unless the
  template asks for them.
- **Verify labels exist before using them.** Templates declare labels (e.g. `eval`) that
  may not yet exist in the repo. Run `gh label list` first; create missing labels only
  if the template requires them.

## Eval Issues (`eval_task.yml`)

Eval issues document real agent failures to turn into automated benchmarks. Keep them
focused on observable behavior:

- **Prompt**: what the agent was asked to do
- **What the agent did**: paste the actual commands and reasoning — no interpretation
- **Correct behavior**: numbered list of concrete steps / assertions
- **Failure type checkboxes**: select from the predefined list only
