# Project Settings & Secure Tokens

## Project Settings (Export & Status)

**Check versioned settings sync status (requires server connection):**
```bash
teamcity project settings status <project-id>
```

**Export project settings as Kotlin DSL:**
```bash
teamcity project settings export <project-id>
```

**Export as XML:**
```bash
teamcity project settings export <project-id> --xml -o settings.zip
```

## Secure Tokens

**Store a secret and get a token reference:**
```bash
teamcity project token put <project-id> "my-secret-password"
```

**Store from stdin (for piping):**
```bash
echo -n "my-secret" | teamcity project token put <project-id> --stdin
```

**Retrieve a token value (requires System Admin):**
```bash
teamcity project token get <project-id> "credentialsJSON:abc123..."
```
