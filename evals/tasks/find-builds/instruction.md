Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

Find the 10 most recent failed runs in the Backend project (project ID: `Shop_Backend`), across all branches.

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"runs": [{"job": "<job ID>", "number": "<run number>", "branch": "<branch name>"}]}
```

List the runs newest first. Run numbers are a job's build numbers (`#12` is `12`), not run IDs.
