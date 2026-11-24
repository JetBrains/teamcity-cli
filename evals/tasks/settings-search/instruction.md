Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

How many jobs in the Acme Shop project (ID `Shop`, subprojects included) run at least one build step inside a Docker image? Which images do they use, and how many jobs use each?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"jobs_with_docker_steps": <number>, "images": {"<image name with tag>": <number of jobs using it>}}
```
