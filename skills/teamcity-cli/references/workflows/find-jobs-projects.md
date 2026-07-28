# Finding Jobs and Projects

**List all projects:**
```bash
teamcity project list
```

**List sub-projects:**
```bash
teamcity project list --parent <project-id>
```

**Create a project:**
```bash
teamcity project create <name>
teamcity project create <name> --id <id> --parent <parent-id>
```

**List jobs in a project:**
```bash
teamcity job list --project <project-id>
```

**View job details:**
```bash
teamcity job view <job-id>
```

**Search for a job by name:**
```bash
teamcity job list --json | jq '.buildType[] | select(.name | contains("deploy"))'
```
