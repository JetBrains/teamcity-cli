Look at the last 20 finished runs of the job `Shop_Backend_IntegrationTests` across all branches. Which tests failed in more than one of those runs but not in all of them, and in how many of the 20 runs did each fail?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"flaky": [{"test": "<full test name as TeamCity reports it>", "failed_runs": <number of runs>}]}
```
