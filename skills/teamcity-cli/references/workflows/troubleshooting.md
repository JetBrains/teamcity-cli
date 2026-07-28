# Tips & Troubleshooting

## Tips

1. **Use `--json` for programmatic access** - Parse with `jq` for complex queries

1. **Use `teamcity api` as escape hatch** - When a specific command doesn't exist, use raw API access

1. **Environment variables** - If overriding with env vars, set both `TEAMCITY_URL` and `TEAMCITY_TOKEN`; `TEAMCITY_URL` alone bypasses stored auth

1. **Open in browser** - Most view commands support `-w` to open in web browser

1. **Auto-detection from DSL** – When working in a project with Kotlin DSL config, the server URL is auto-detected from `.teamcity/pom.xml`

1. **Multiple servers** - Use `TEAMCITY_URL` env var to switch between servers, or `teamcity auth login --server <url>` to add servers

## Troubleshooting

| Symptom                      | Likely Cause              | Action                                                                                  |
|------------------------------|---------------------------|-----------------------------------------------------------------------------------------|
| `401 Unauthorized`           | Invalid or expired token  | Run `teamcity auth status` to check; re-login with `teamcity auth login`                |
| `403 Forbidden`              | Insufficient permissions  | Build config may require different access rights; check with TeamCity admin             |
| `404 Not Found`              | Build deleted or wrong ID | Verify the build ID/URL; the build may have been cleaned up                             |
| Connection refused / timeout | Server unreachable        | Check if TeamCity instance is accessible; verify server URL with `teamcity auth status` |
| `Not authenticated`          | `TEAMCITY_URL` set without matching token, or no auth configured | Unset `TEAMCITY_URL` to use stored auth from `teamcity auth login`, or set both `TEAMCITY_URL` and `TEAMCITY_TOKEN` |
| `No server configured`       | Missing auth config       | Run `teamcity auth login -s <url>` or set `TEAMCITY_URL` and `TEAMCITY_TOKEN` env vars  |
| `Network access blocked by sandbox` | Sandbox proxy blocking outbound requests | Add the server domain to the sandbox `allowedDomains`, or exclude `teamcity` from sandboxing |
