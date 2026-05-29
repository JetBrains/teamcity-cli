<!-- Keep this lean. Every section stays — write "N/A — <reason>" instead of removing one. The diff carries the details; this body explains what you can't read from the diff. -->

## Summary

<!-- One paragraph. What does this PR do, and why? Link related issues with "Fixes #123". -->



## Changes

<!-- Short bullet list of key changes; group by area if useful. Keep terse. -->

-

## Design Decisions

<!-- Non-obvious choices and trade-offs. Write "Straightforward." if there are none. -->

## Example

<!-- For CLI changes: show command + output. Write "N/A — not user-visible." if not applicable. -->

```bash
teamcity <command>
```

## Test Plan

<!-- For conditional checkboxes that don't apply, leave unchecked and note "N/A — <reason>" inline. -->

- [ ] Unit tests pass (`just unit`)
- [ ] Linter passes (`just lint`)
- [ ] Acceptance tests pass (`just acceptance`)
- [ ] If adding a new command/flag: added `.txtar` test in `acceptance/testdata/`
- [ ] If adding a data-producing command: includes `--json` support
- [ ] If modifying `--json` output: no field removals/renames (additive only)
- [ ] If changing docs-visible behavior: updated `docs/`, `skills/`, and `README.md`
- [ ] External contributors: links a `status:finalized` issue (or trivial/docs/deps change)
