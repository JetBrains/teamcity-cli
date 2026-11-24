Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

The job `Shop_Backend_UnitTests` is failing on the `main` branch. Investigate what went wrong: which tests fail in its latest run on main, in which run on main did they start failing, and which commit introduced the failure?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"failed_tests": ["<full test name>"], "first_failing_run": "<run number>",
 "culprit_commit": "<commit hash>", "culprit_author": "<VCS username>"}
```

Run numbers are a job's build numbers (`#12` is `12`), not run IDs.
