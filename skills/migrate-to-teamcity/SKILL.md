---
name: migrate-to-teamcity
version: "0.2.0"
description: Migrating CI/CD pipelines to TeamCity. Use when the user wants to migrate, convert, or switch to TeamCity from GitHub Actions (.github/workflows/) or Bamboo (bamboo-specs/*.yml), even if they only say "move our CI". Other CI systems (GitLab, Jenkins, CircleCI, Azure DevOps, Travis, Bitbucket) are not supported yet.
---

# Migrate to TeamCity

## Quick Start

```bash
teamcity migrate                    # detect + convert + write .tc.yml files
teamcity migrate --dry-run --json   # preview as structured JSON
teamcity pipeline validate f.tc.yml # schema check
teamcity project vcs create --url <repo-url> --auth anonymous -p ProjectId  # create VCS root first
teamcity pipeline create name -p ProjectId -f f.tc.yml --vcs-root <VcsRootId>
teamcity run start PipelineId --watch
```

Run `teamcity migrate` from the repo root -- detection scans `.github/workflows/` and `bamboo-specs/` relative to the current directory.

## Reading the report

- **Needs review** -- problems inside the generated YAML: TODO stubs, dropped steps, reusable-workflow placeholders. Fix these in the file before creating the pipeline.
- **Manual setup needed** -- work the converter could not do automatically; each item lands on one of two sides. YAML edits before `pipeline create`: secrets entries (see the checklist), matrix expansion, expression `runs-on`, `container:`/`services:` wiring (see gotchas.md). Server-side configuration after create: connections and `if:`-derived branch filters / execution conditions before the first run (they gate what executes); triggers and notifications at the end. Read each item and sort it accordingly.
- Exit code 1 means at least one source failed to convert *or* one generated file failed schema validation -- files that converted cleanly are still written. Read the per-file ✓/⚠/✗ lines instead of treating exit 1 as total failure.
- `--json` prints `{"sources": [...], "results": [...]}` to stdout; each result carries `outputFile`, `yaml`, `needsReview`, `manualSetup`, and `validationError`.

## Gotchas

- **Always `type: script` for `./gradlew` and `./mvnw`.** TC's `type: gradle`/`type: maven` runners use the agent's version, not the project's. This causes real build failures.
- **Schema valid does not mean pipeline works.** Migration is not done until builds pass.
- **Private repos: use a GitHub App connection, not a PAT.** `teamcity project connection create github-app -p <project>` → `teamcity project connection authorize <connection-id> -p <project>` → install the App on the repo/org (the create output prints the install link and these exact commands) → `teamcity project vcs create --url <repo-url> --auth token --connection-id <connection-id> -p <project>`. For public repos `--auth anonymous` works; SSH deploy keys (`teamcity project ssh upload` with a `git@github.com:` URL) are an alternative.
- **Secrets, triggers, and branch filters are always manual.** The converter flags them but cannot create them. Use `teamcity project token put` for secrets. Configure triggers in TC UI.
- **VCS root must exist before pipeline create.** `teamcity pipeline create` takes `--vcs-root <id>`, not a URL. Create it first with `teamcity project vcs create`.
- **Default branch defaults to `main`.** Pass `--branch refs/heads/master` to `teamcity project vcs create` if the repo uses `master`.
- **Unknown actions/tasks become stubs.** Read the action's source, write an equivalent shell script. Most actions are thin CLI wrappers. See [mappings](references/mappings.md).

## Workflow

Goal: get all pipeline jobs green on the TC server, not just generate valid YAML.

Copy this checklist and check off items as you complete them:

```
Migration progress:
- [ ] Convert: run `teamcity migrate` from the repo root
- [ ] Fix every "Needs review" item, plus "Manual setup" items needing YAML edits (matrix expansion, expression `runs-on`, container/services) -- see mappings.md and gotchas.md
- [ ] Wire up secrets in the YAML: the converter rewrites `${{ secrets.X }}` to `%X%` but does not define it -- store the value (`teamcity project token put <project> <value>`) and add `X: "credentialsJSON:<uuid>"` under the top-level `secrets:` block (see schema.md)
- [ ] Validate: `teamcity pipeline validate <file>` -- only proceed when it passes
- [ ] Create VCS root (`teamcity project vcs create`), then `teamcity pipeline create <name> -p <project> -f <file> --vcs-root <id>`
- [ ] Set up the remaining runtime "Manual setup needed" items before running: registry/cloud connections the steps reference (the first run fails without them), and any `if:`-condition items -- gate converted deploy/release steps via branch filter, execution condition, or a guard in the script so the first run cannot deploy from the wrong branch
- [ ] Run: `teamcity run start <id> --watch`; on failure read `teamcity run log <id> --failed --raw`, fix, `teamcity pipeline push`, re-run until green
- [ ] Do the trigger-only "Manual setup needed" items: triggers, notifications
- [ ] Report: what migrated, step reduction, what remains manual
```

## References

- [Mappings](references/mappings.md) -- GitHub Actions and Bamboo to TeamCity translation tables
- [Schema](references/schema.md) -- TC pipeline YAML quick reference
- [Gotchas](references/gotchas.md) -- skip list, matrix expansion, troubleshooting, manual setup items
