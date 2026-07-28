# Build Metadata & the Build Queue

## Build Metadata

**Pin a build (prevent cleanup):**
```bash
teamcity run pin <run-id> --comment "Release candidate"
```

**Unpin a build:**
```bash
teamcity run unpin <run-id>
```

**Tag a build:**
```bash
teamcity run tag <run-id> deployed production
```

**Remove tags:**
```bash
teamcity run untag <run-id> deployed
```

**Add a comment:**
```bash
teamcity run comment <run-id> "Verified by QA"
```

**View existing comment:**
```bash
teamcity run comment <run-id>
```

**Delete a comment:**
```bash
teamcity run comment <run-id> --delete
```

## Managing the Build Queue

**View queued builds:**
```bash
teamcity queue list
```

**Filter queue by job:**
```bash
teamcity queue list --job <job-id>
```

**Move a build to top of queue:**
```bash
teamcity queue top <run-id>
```

**Remove from queue:**
```bash
teamcity queue remove <run-id>
```

**Approve a build waiting for approval:**
```bash
teamcity queue approve <run-id>
```
