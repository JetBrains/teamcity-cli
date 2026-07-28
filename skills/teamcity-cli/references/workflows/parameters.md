# Managing Job and Project Parameters

**List job parameters:**
```bash
teamcity job param list <job-id>
```

**Set a parameter:**
```bash
teamcity job param set <job-id> MY_PARAM "my value"
```

**Set a secure parameter:**
```bash
teamcity job param set <job-id> SECRET_KEY "****" --secure
```

**Get a parameter:**
```bash
teamcity job param get <job-id> MY_PARAM
```

**Delete a parameter:**
```bash
teamcity job param delete <job-id> MY_PARAM
```

Project parameters work the same way with `teamcity project param`.
