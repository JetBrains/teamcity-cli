I'm trying to understand the CI health of the Acme Shop project (ID `Shop`). For each of its direct subprojects, tell me how many jobs it has, how many of them have never had a finished run, and how many are failing right now (their latest finished run on the default branch failed).

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"subprojects": [{"project": "<project ID>", "jobs": <number>, "never_run": <number>, "failing": <number>}]}
```
