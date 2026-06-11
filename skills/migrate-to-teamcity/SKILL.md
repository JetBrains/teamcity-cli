---
name: migrate-to-teamcity
version: "0.1.0"
author: JetBrains
description: Migrating CI/CD pipelines to TeamCity. Use when user wants to migrate, convert, or switch from GitHub Actions or Bamboo (bamboo-specs/*.yml). Other CI systems (GitLab, Jenkins, CircleCI, Azure DevOps, Travis, Bitbucket) will be added in follow-up releases.
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

## Gotchas

- **Always `type: script` for `./gradlew` and `./mvnw`.** TC's `type: gradle`/`type: maven` runners use the agent's version, not the project's. This causes real build failures.
- **Schema valid does not mean pipeline works.** Migration is not done until builds pass.
- **No OAuth VCS roots from CLI.** Use anonymous auth (public repos) or upload SSH key (`teamcity project ssh upload`) with `git@github.com:` URL. OAuth requires TC UI.
- **Secrets, triggers, and branch filters are always manual.** The converter flags them but cannot create them. Use `teamcity project token put` for secrets. Configure triggers in TC UI.
- **VCS root must exist before pipeline create.** `teamcity pipeline create` takes `--vcs-root <id>`, not a URL. Create it first with `teamcity project vcs create`.
- **Default branch defaults to `main`.** Pass `--branch refs/heads/master` to `teamcity project vcs create` if the repo uses `master`.
- **PowerShell steps need wrapping on Windows.** TC `type: script` on Windows runs `cmd.exe`. Single-line PowerShell (GHA Windows runners default to it; Bamboo `interpreter: WINDOWS_POWER_SHELL`) wraps as `powershell -Command "<script>"`; multi-line bodies need TC's PowerShell runner.
- **Unknown actions/tasks become stubs.** Read the action's source, write an equivalent shell script. Most actions are thin CLI wrappers. See [mappings](references/mappings.md).
- **(Bamboo) Final-tasks need step execution policy.** TC has no `final-tasks:` block; set "Even if some build steps have failed" on those steps after pipeline creation (UI only — not in YAML).
- **(Bamboo) Manual stages need approval/manual triggers.** A Bamboo `manual: true` stage has no YAML equivalent in TC pipelines — configure as a manual trigger on the downstream pipeline.
- **(Bamboo) Deployment plans (`bamboo-specs/deployment.yml`) are not converted.** They have no TC pipeline equivalent — model as a separate pipeline triggered by the build pipeline's success.
- **(Bamboo) Multi-plan specs convert only the first plan.** Split each plan into its own file before running `teamcity migrate`.

## Workflow

Goal: get all pipeline jobs green on the TC server, not just generate valid YAML.

1. Run `teamcity migrate` to generate `.tc.yml` files
2. Fix any commented stubs for unknown actions -- use [mappings](references/mappings.md) and [examples](references/examples.md)
3. Create VCS root (`teamcity project vcs create`), then create pipeline with `--vcs-root <id>`. Validate, run, and verify green
4. If a job fails, check logs with `teamcity run log <id> --failed --raw`, fix the YAML, push with `teamcity pipeline push`, re-run
5. Report: what migrated, step reduction, what needs manual setup

## References

- [Mappings](references/mappings.md) -- all CI systems to TeamCity translation tables
- [Examples](references/examples.md) -- verified before/after with lessons learned
- [Schema](references/schema.md) -- TC pipeline YAML quick reference
- [Gotchas](references/gotchas.md) -- full troubleshooting table, manual setup items, checklist
