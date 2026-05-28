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

## Code style

- **Start lean.** First draft is the bare minimum — the observable behavior plus the guards needed to make it correct. Don't pre-emptively add throttle files, `*_NO_*` env knobs, marker state, helper helpers, or "in case" escape hatches. Add them when a real signal asks for them.
- **One-line comments by default.** Single-line godoc on exported symbols; only wrap when an invariant or trade-off truly needs the room.
- **Reuse what's there before inventing.** Output goes through `internal/output` — tip strings live in `output/tips.go` and render via `output.FormatTip`; status messages via `output.Printer`. Search for an existing helper before adding a parallel path.
- **Verify visible behavior before claiming done.** For runtime/UX changes, build (`just build`) and exercise the binary — the `verify` and `run` skills exist for this. Type-check passing ≠ feature works.

## Commits and PRs

- Don't commit unless asked.
- Conventional format: `feat(scope):`, `fix(scope):`, `refactor(scope):`.
- **Subject line only by default.** Recent commits are single-line; push the *why* into the PR description, not the commit body. Add a body only when context genuinely won't fit anywhere else.
- **Always respect `.github/PULL_REQUEST_TEMPLATE.md` when opening a PR.** Fill every section the template defines — its `<!-- Delete ... -->` hints are misleading, never drop a section. Write `N/A — <reason>` for sections that don't apply. Don't invent extra sections.
- **PR descriptions stay lean.** Summary fits a paragraph; Changes is a short bullet list; no marketing copy, no restated diff. If Design Decisions has nothing non-obvious, write `Straightforward.` and move on.

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
