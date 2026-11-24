Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

I'm new to this TeamCity server. Can you give me an overview?

1. What top-level projects are there (directly under the root project)?
2. What are the subprojects of Acme Shop?
3. How many agent pools exist, and how many agents are registered in total?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"top_level_projects": ["<project ID>"], "shop_subprojects": ["<project ID>"],
 "agent_pools": <number>, "agents": <number>}
```
