Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

What's going on with this build configuration? Look at its last 10 finished runs across all branches.

http://teamcity:8111/buildConfiguration/Shop_Backend_UnitTests

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"job": "<job ID>", "succeeded": <number of the 10>, "failed": <number of the 10>,
 "latest_status": "SUCCESS or FAILURE"}
```
