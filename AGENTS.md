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

- TeamCity CLI product reports belong in YouTrack project `TW`, assigned to Team `Onboarding`, with visibility set to All Users. Leave them unassigned unless explicitly requested.
- Use the matching prefilled YouTrack link in `.github/ISSUE_TEMPLATE/config.yml` for bugs, feature requests, migration issues, and eval reports. GitHub Discussions remain available for questions; code contributions still use GitHub pull requests.
- When creating issues with the YouTrack CLI, follow the matching description structure and verify the Type, Team, visibility, and assignee fields. Do not create replacement GitHub issues or apply GitHub labels to YouTrack tickets.

## Eval Issues

Eval issues document real agent failures to turn into automated benchmarks. Keep them focused on observable behavior:

- **Prompt**: what the agent was asked to do
- **What the agent did**: paste the actual commands and reasoning — no interpretation
- **Correct behavior**: numbered list of concrete steps / assertions
- **Failure types**: choose only from the predefined list in the eval issue link
