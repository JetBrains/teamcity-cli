Show me the last 5 finished runs of `Shop_Backend_Build` on the `main` branch and check if any of them failed. If so, find the commit that broke the build.

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"runs": [{"number": "<run number>", "status": "SUCCESS or FAILURE"}],
 "breaking_commit": "<commit hash>", "breaking_commit_author": "<VCS username>"}
```

List the runs newest first. Run numbers are a job's build numbers (`#12` is `12`), not run IDs.
