# Project Connections & VCS Roots

## Project Connections

Connections give jobs credentials for external services (GitHub, Docker registries, AWS, ...) without storing secrets per-job. Required before creating a VCS root that authenticates via OAuth.

**Inspect existing connections in a project:**
```bash
teamcity project connection list --project <project-id>
```

### Connecting a GitHub repository (GitHub App)

> **Always use this path for GitHub.** Don't `vcs create --auth password` with a personal access token — PATs tie infrastructure to one human, leak in job logs, and can't be revoked centrally. The four-step flow below produces a non-personal "Refreshable access token" tied to a service-identity App, which is what the TeamCity UI's "Sign in to GitHub App" button creates.

Creates a fresh GitHub App via GitHub's manifest flow — credentials are captured automatically, no PAT involved. Lets jobs clone, post commit statuses, and comment on PRs.

**1. Create the connection** (one browser click on github.com):

```bash
teamcity project connection create github-app -p <project-id>
# prompts: Connection name (default "GitHub App"), GitHub organization (blank for personal)
# browser auto-redirects to GitHub's "Create GitHub App" page; click Create.
# CLI captures App ID, client ID, secret, PEM, owner URL.
```

The output prints `Next steps:` with follow-up commands and the install link. Capture the `PROJECT_EXT_NN` from the success line.

**2. Authorize as the current TeamCity user** (stores a token for `(connection × user)`):

```bash
teamcity project connection authorize PROJECT_EXT_NN -p <project-id>
# browser opens TeamCity's OAuth page → click Authorize on GitHub → tab self-closes.
```

**3. Install the App on a repo** (one-time, per repo, on github.com):

Open the printed install link `https://github.com/apps/<slug>/installations/new`, pick repos, click Install.

> Steps 2 and 3 are independent — order doesn't matter. Both must complete before step 4: Authorize provides the user token TeamCity uses for API calls; Install grants the App access to the repo. `vcs create` will fail without either.

**4. Create the VCS root using the connection:**

```bash
teamcity project vcs create -p <project-id> \
  --auth token \
  --connection-id PROJECT_EXT_NN \
  --url https://github.com/<owner>/<repo>.git
```

TeamCity auto-fills `authMethod=ACCESS_TOKEN`, `username=oauth2`, and the proper `tokenId` from the connection's stored token. No manual property setup needed; the resulting VCS root uses a non-personal "Refreshable access token" — exactly what the UI's "Sign in to GitHub App" produces.

**Non-interactive (agent) variant — bring your own GitHub App credentials:**

```bash
echo "$GH_APP_CLIENT_SECRET" | teamcity project connection create github-app \
  -p <project-id> --no-manifest \
  --name "Backend" \
  --owner my-org \
  --app-id 1234567 \
  --client-id Iv1.abc \
  --private-key-file /path/to/key.pem \
  --stdin
```

Skips the manifest browser flow; use when a human has already registered the App and stored its credentials in a vault.

### Connecting a Docker registry

For pushing images to GHCR, Docker Hub, or a private registry. Uses static credentials — always use a service account / robot user, never a personal password.

```bash
echo "$REGISTRY_TOKEN" | teamcity project connection create docker \
  -p <project-id> \
  --name "GHCR" \
  --url https://ghcr.io \
  --username my-org \
  --stdin
```

Interactive variant prompts for each field; password is read via a secret prompt (never echoed). The connection is referenced from the Docker Image Builder runner and the `docker-support` build feature via its ID; configure those in the UI or Kotlin DSL.

### Removing a connection

```bash
teamcity project connection delete PROJECT_EXT_NN -p <project-id>
teamcity project connection delete PROJECT_EXT_NN -p <project-id> --force   # skip confirm
```

VCS roots and build features that reference the deleted connection break — clean those up first.

**Gotchas:**
- `vcs create --auth token` test connection returns "Malformed request" if the user hasn't authorized yet. The CLI prints a tip pointing at `connection authorize`. Run that, then retry.
- The App's per-repo install (step 3) is mandatory; without it, clones return 404 even with a valid connection.
- Connections in a parent project are inherited by sub-projects — don't recreate the same connection in nested projects.
- For Docker on AWS-managed ECR, prefer an AWS connection with role-based federation over Docker credentials.

## VCS Roots

For questions like "which repository URL and default branch does project `<id>` use", always discover attached VCS roots first, then inspect a concrete root.

**List VCS roots in a project:**
```bash
teamcity project vcs list --project <project-id>
```

**View VCS root details:**
```bash
teamcity project vcs view <vcs-root-id>
```

**Required sequence for project VCS inspection:**
1. Run `teamcity project vcs list --project <project-id>` to get valid root IDs.
2. Run `teamcity project vcs view <vcs-root-id>` for URL, default branch, auth method, and other properties.
3. Do not guess VCS root IDs.
4. Do not use `teamcity project view` or `teamcity project settings status` as a substitute for VCS root details.

**Create a VCS root:**
```bash
# Preferred for GitHub: use a GitHub App connection (see Project Connections above).
teamcity project vcs create -p <project-id> \
  --auth token --connection-id <connection-id> \
  --url https://github.com/<owner>/<repo>.git

# Other auth methods (use only when there is no usable connection).
teamcity project vcs create -p <project-id> --url <url> --auth anonymous
teamcity project vcs create -p <project-id> --url <url> --auth password --username U --stdin <<<"$PAT"
teamcity project vcs create -p <project-id> --url <url> --auth ssh-key --ssh-key-name my-key
```

> **For GitHub repositories, always prefer the GitHub App connection path** (`--auth token --connection-id <id>`). Pasting a personal access token via `--auth password` works but is an anti-pattern: PATs are tied to a single human, leak via job logs, and can't be revoked centrally. Use the [Connecting a GitHub repository](#connecting-a-github-repository-github-app) workflow before falling back to PAT auth.

**Delete a VCS root:**
```bash
teamcity project vcs delete <vcs-root-id>
teamcity project vcs delete <vcs-root-id> --yes   # skip confirmation
```
