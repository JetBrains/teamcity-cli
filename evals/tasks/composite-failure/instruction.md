`Shop_Release_Verify` is a composite job with snapshot dependencies. Look at every failed run it has had. For each one, find the job or jobs in its dependency chain that are the root cause (they failed on their own, not because one of their own dependencies failed) and the tests that failed in those root-cause runs.

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"failed_runs": [{"number": "<run number of the composite>", "branch": "<branch name>",
  "root_cause_jobs": ["<job ID>"], "failed_tests": ["<full test name>"]}]}
```

Run numbers are a job's build numbers (`#12` is `12`), not run IDs. Use an empty `failed_tests` list when the root cause has no failed tests.
