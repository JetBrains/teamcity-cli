# Feature Parity Plan

Gap analysis between this VS Code extension and the IntelliJ TeamCity plugin spec (see internal `SPEC.md`).

Design constraints:

- **Thin UI wrapper.** Every feature must delegate to the `teamcity` CLI; no direct HTTP.
- **Lean.** Net TS lines per feature should be small; if a feature would require hundreds of TS lines, push it to the CLI first.
- **No CLI-side work unless called out.** The CLI already covers everything we need today except project linking and a few inspections.
- ✅ shipped · ⚠️ partial · ❌ not yet · 🟥 out of scope (platform mismatch or intentionally skipped)

---

## Current state

| Area | Status |
|---|---|
| Multi-server auth (`auth login`, `auth login --guest`, `auth logout`, `auth status`) | ⚠️ (CLI: multi-server; UI: picks one via `pickServer`) |
| Auto-reconnect with exponential backoff | ✅ |
| Pipelines / Runs / Queue / Agents / Favorites tree views | ✅ |
| Status bar with branch build status + notifications | ✅ |
| Run status filter QuickPick | ✅ |
| Trigger run + Remote run (`--local-changes`) with pipeline → job picker | ✅ |
| `.teamcity.yml` YAML schema validation | ✅ |
| CodeLens: Validate Pipeline / Push Pipeline / Run Job | ✅ |
| Notification action buttons (View Log / Open in Browser) | ✅ |
| BuildNotifier dedup (never spam same build twice) | ✅ |
| Test suite (`node:test`, 135 tests) | ✅ |

---

## Gaps, ordered by value / effort ratio

Each row estimates TS lines (excluding tests), effort, and dependency on CLI work.

### Tier 1 — ship next (high value, low effort, no CLI work)

| # | Feature | SPEC ref | Est. TS | Notes |
|---|---|---|---|---|
| 1 | **`.teamcity.yml` → auto-populate Remote Run / Trigger picker.** Workspace YAML doubles as "project linking". | §5 | ~50 | Reuses YAML parser from `pipelineCodeLens.ts`. Falls back to pipeline-list picker if no file. |
| 2 | **`pipeline validate` on save → `DiagnosticCollection`.** Replaces four IntelliJ inspections (undefined/duplicate/cyclic `needs:`, param↔secret clash) with a single CLI roundtrip. | §6.5 | ~80 | Debounce on `onDidSaveTextDocument`. Map CLI error positions to `vscode.Range`. |
| 3 | **`teamcity.switchServer` command.** Pops QuickPick of authenticated servers from `auth status --json`, updates `TEAMCITY_URL`, refreshes views. | §1 Multi-server | ~40 | |
| 4 | **Selection persistence** (`workspaceState`) for pipeline/job chosen in Remote Run. | §3 | ~20 | Chain after #1 — recall last choice, still offer QuickPick. |
| 5 | **"Credentials expired" status bar indicator.** CLI already surfaces `status: "expired"` in `auth status --json`. | §1 | ~15 | Add warning icon + "Re-login" action. |

**Subtotal: ~205 TS lines + tests. Closes the "no project linked" gap from today's session.**

### Tier 2 — ship after Tier 1 (high value, moderate effort)

| # | Feature | SPEC ref | Est. TS | Notes |
|---|---|---|---|---|
| 6 | **`needs:` Go-to-Definition in `.teamcity.yml`.** `DefinitionProvider` reusing the YAML parser from CodeLens. | §6.6 | ~60 | Ctrl+Click parity with IntelliJ. |
| 7 | **Agent name completion in YAML.** `CompletionItemProvider` scoped to `agent:` keys, fed by `agent list --json` (cached). | §6.4 | ~80 | Depends on reliable `agent list` output; invalidate cache on refresh. |
| 8 | **Build detail webview.** Replaces "no detail view" gap: status / agent / duration / triggered-by / comment / links. Opened on tree click. | §2.1 | ~150 | Renders markdown from `run view --json`. Keep it read-only. |
| 9 | **TaskProvider exposing pipeline jobs as VS Code Tasks.** `Run Task → TeamCity: <job>` runs `run start <job> --watch`. | §6.1 | ~70 | Cleaner than opening terminals for users who live in tasks.json. |

**Subtotal: ~360 TS lines.**

### Tier 3 — nice-to-have (lower value or requires CLI work)

| # | Feature | SPEC ref | Status | Notes |
|---|---|---|---|---|
| 10 | Pagination in Runs / Queue trees | §2.1 | ❌ | Today we use `--limit`. TreeItem "Load more…" — ~40 lines, but most users don't ask. |
| 11 | Hierarchical grouping of Runs by pipeline | §2.1 | ❌ | Preference, not parity. |
| 12 | Tool window state persistence (active tab, filters, pagination) | §2.3 | ❌ | Light lift once #10 lands. |
| 13 | Tests tab with filter (Passed/Failed/New Failure/Ignored) | §2.2 | ❌ | Requires `view --test-filter` or similar in CLI; otherwise we filter client-side. |
| 14 | Multi-configuration Remote Run | §3 | ❌ | Requires CLI `run start --jobs job1,job2 --local-changes` or parallel invocations. |
| 15 | Per-stage Remote Run progress notifications | §3 | 🟥 | Terminal output is sufficient in VS Code; unlikely ROI. |
| 16 | Rename refactoring across YAML | §6.6 | ❌ | `RenameProvider` — narrow audience in YAML. |

### Tier 4 — large effort or platform mismatch

| # | Feature | SPEC ref | Status | Notes |
|---|---|---|---|---|
| 17 | **VCS log / Source Control build status column.** Status per commit in the Git panel. | §4 | ❌ | `vscode.git` API is limited; would need `FileDecorationProvider` or custom SCM view. 5–10× the effort of Tier 1 items combined. Park until user demand is proven. |
| 18 | Custom expression language (lexer, parser, injection, completion) | §6.3 | 🟥 | IntelliJ-scale undertaking; skip unless JetBrains ships a language server we can reuse. |
| 19 | Analytics (FUS) | §11 | 🟥 | Intentionally skipped. |
| 20 | Per-module linking | §5 MODULE | 🟥 | VS Code has no "modules". N/A. |
| 21 | IntelliJ-style notification groups | §8 | 🟥 | VS Code has a single notification surface; user settings apply globally. |

---

## CLI gaps that would unlock more parity

Track these upstream in the CLI repo — none of them block Tier 1:

- `project link <jobId>` / `project link list` — native workspace linking, replaces our `.teamcity.yml` heuristic.
- `run start --jobs j1,j2 --local-changes` — multi-config remote run.
- `run view --tests --filter=failed` — server-side test filter for Tests tab.
- JDK list endpoint exposure — unblocks `PipelinesYamlJdkHomeCompletionProvider` parity.

---

## Out of scope (confirmed)

- VCS log integration (Tier 4 pending explicit ask).
- Expression language support.
- Analytics / telemetry.
- Per-module linking.
- Custom notification groups.

---

## Rough timeline

| Milestone | Scope | Estimated net TS |
|---|---|---|
| **v0.2 — Editor parity** | Tier 1 (items 1–5) | ~205 |
| **v0.3 — YAML & detail view** | Tier 2 (items 6–9) | ~360 |
| **v0.4 — Polish** | Selected Tier 3 items based on feedback | ~150 |
| **v1.0** | Tier 4 re-evaluated; CLI upstream items landed | varies |

Total from today through v0.4: **~700 TS lines + tests**, every line CLI-delegated. No new runtime dependencies expected.
