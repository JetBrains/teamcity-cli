Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

Some build agents on this server are disabled. Which ones, and which pool does each belong to?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"disabled_agents": [{"name": "<agent name>", "pool": "<pool name>"}]}
```
