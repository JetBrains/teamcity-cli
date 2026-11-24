Which run in the Acme Shop project (ID `Shop`, subprojects included) has been waiting in the build queue the longest, and why hasn't it started?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"job": "<job ID of that run>", "triggered_by": "<username>",
 "missing_agent_parameter": "<agent parameter the job requires and no agent provides>",
 "compatible_agents": <number of agents that can run it>,
 "queued_runs_in_project": <number of queued runs in the project>}
```
