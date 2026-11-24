Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

Which agent pools can the `Shop_Release` project run builds on? For each pool, how many agents does it have and how many of them are enabled?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"pools": [{"name": "<pool name>", "agents": <number>, "enabled_agents": <number>}]}
```
