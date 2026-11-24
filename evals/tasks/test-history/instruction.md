Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

The test `com.acme.shop.it.CheckoutFlowIT.completesOrderWithSavedCard` keeps failing in the job `Shop_Backend_IntegrationTests`. Over the whole history of that job, on all branches: in how many runs did it fail, which was the first run where it failed, and on which branches has it failed?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"failed_runs": <number of runs>, "first_failed_run": "<run number>", "branches": ["<branch name>"]}
```

Run numbers are a job's build numbers (`#12` is `12`), not run IDs.
