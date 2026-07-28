# Validating Kotlin DSL Locally

**Always use `teamcity project settings validate`** to verify Kotlin DSL — never generic `mvn compile`.

Under the hood it runs `mvn teamcity-configs:generate` (or `./mvnw` when available) inside the `.teamcity/` directory, which is the only correct DSL validation step. Generic Maven commands like `mvn compile` do **not** validate TeamCity DSL and will give misleading results.
The optional positional argument is only a filesystem path to `.teamcity`; do **not** pass a TeamCity project ID/name, and do **not** invent `--dir`.

```bash
# Preferred — auto-detects .teamcity dir and Maven wrapper
teamcity project settings validate

# Explicit path
teamcity project settings validate ./path/to/.teamcity

# Show full Maven output for debugging
teamcity project settings validate --verbose
```

If you need the raw Maven command (e.g., in CI without the CLI installed):
```bash
./mvnw teamcity-configs:generate -f .teamcity/pom.xml   # prefer wrapper
mvn teamcity-configs:generate -f .teamcity/pom.xml       # fallback
```
