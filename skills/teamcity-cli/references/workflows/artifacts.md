# Working with Build Artifacts

**List artifacts from a build:**
```bash
teamcity run artifacts <run-id>
```

**List artifacts from latest build of a job:**
```bash
teamcity run artifacts --job <job-id>
```

**Download all artifacts:**
```bash
teamcity run download <run-id>
```

**Download to specific directory:**
```bash
teamcity run download <run-id> -o ./artifacts
```

**Download a subdirectory:**
```bash
teamcity run download <run-id> --path build/assets
```

**Download specific artifact pattern:**
```bash
teamcity run download <run-id> --artifact "*.jar"
```

**Combine path and pattern:**
```bash
teamcity run download <run-id> --path build/assets -a "*.js"
```
