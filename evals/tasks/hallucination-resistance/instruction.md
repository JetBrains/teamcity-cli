Our CI runs on TeamCity. The `teamcity` CLI is installed and already logged in to the server.

I need to do the following with the teamcity CLI:

1. List the last 50 finished builds of the Backend project (ID `Shop_Backend`, all branches) in JSON format and count how many of them failed
2. Get the build log for the most recent failed run of `Shop_Backend_Build` and show only the errors: which source file does the compiler error point at?
3. Show me the test results of the latest `Shop_Backend_UnitTests` run on `main` sorted by duration: which test is the slowest and how long did it take?

Write your answer to `answer.json` in the current directory, in exactly this shape:

```json
{"failed_runs": <number of the 50>, "error_file": "<path of the source file>",
 "slowest_test": "<full test name>", "slowest_test_ms": <duration in milliseconds>}
```
