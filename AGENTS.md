# Agent Instructions

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
