[//]: # (title: Authentication)

<show-structure for="chapter" depth="2"/>

TeamCity CLI supports several authentication methods. This page covers interactive login, guest access, token-based authentication for CI/CD, multi-server setup, and automatic authentication within TeamCity builds.

> TeamCity CLI supports TeamCity server versions 2020.1 and later.
>
{style="tip"}

## Interactive login

The standard way to authenticate is with the `teamcity auth login` command:

```Shell
teamcity auth login
```

This starts an interactive flow:

1. Enter your TeamCity server URL (for example, `https://teamcity.example.com`).
2. If the server supports PKCE authentication, the CLI opens your browser to approve access directly — no token copying needed.
3. Otherwise, the CLI opens the TeamCity __Access Tokens__ page for you to create and paste a token manually.
4. The CLI validates the token and stores it securely.

To authenticate with a specific server URL:

```Shell
teamcity auth login --server https://teamcity.example.com
```

To skip browser-based authentication and enter a token manually:

```Shell
teamcity auth login --no-browser
```

To pass the token directly (for example, from a password manager):

```Shell
teamcity auth login --server https://teamcity.example.com --token <token>
```

### Browser-based login (PKCE)
{id="pkce"}

When PKCE is enabled on the TeamCity server, `teamcity auth login` authenticates via the browser automatically:

1. The CLI starts a temporary local server on your machine.
2. Your browser opens a TeamCity authorization page.
3. After you approve, the browser redirects back to the CLI with an authorization code.
4. The CLI exchanges the code for an access token.

This flow follows the [OAuth 2.0 PKCE standard (RFC 7636)](https://datatracker.ietf.org/doc/html/rfc7636) and does not require you to copy or paste any tokens.

PKCE tokens have an expiry date. The CLI tracks this and shows a warning when the token is about to expire or has expired:

```Shell
$ teamcity auth status
✓ Logged in to https://teamcity.example.com
  User: John Doe (john.doe) · system keyring
  ! Token expires 2 hours from now (on Mar 25, 2026)
```

> PKCE must be enabled on the TeamCity server. If it is not available, the CLI falls back to manual token entry automatically.
>
{style="note"}

### Check authentication status

View the current authentication state:

```Shell
teamcity auth status
```

This displays the server URL, server version, authenticated username, and token storage method.

### Log out

Remove stored credentials for the current server:

```Shell
teamcity auth logout
```

## Guest access

If the TeamCity server has guest access enabled, you can authenticate without a token:

```Shell
teamcity auth login --guest
```

With a specific server URL:

```Shell
teamcity auth login --server https://teamcity.example.com --guest
```

Guest authentication provides read-only access. It uses the `/guestAuth/` API prefix and does not require or store any credentials.

<img src="auth-login.gif" alt="Authenticating with guest access" border-effect="rounded"/>

> Guest access must be [enabled in the TeamCity server settings](enabling-guest-login.md). Otherwise, the login will fail.
>
{style="note"}

> The `--guest` and `--token` flags are mutually exclusive. Use either one or the other.
>
{style="warning"}

### Guest access via environment variable

For CI/CD environments where guest access is sufficient:

<tabs>
<tab title="macOS and Linux">

```Shell
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_GUEST=1
```

</tab>
<tab title="Windows">

PowerShell:

```PowerShell
$env:TEAMCITY_URL = "https://teamcity.example.com"
$env:TEAMCITY_GUEST = "1"
```

CMD:

```Shell
set TEAMCITY_URL=https://teamcity.example.com
set TEAMCITY_GUEST=1
```

</tab>
</tabs>

## Token storage

TeamCity CLI stores access tokens using the system keyring when available:

<table>
<tr>
<td>

Platform

</td>
<td>

Keyring

</td>
</tr>
<tr>
<td>

macOS

</td>
<td>

Keychain

</td>
</tr>
<tr>
<td>

Linux

</td>
<td>

GNOME Keyring (or compatible secret service)

</td>
</tr>
<tr>
<td>

Windows

</td>
<td>

Credential Manager

</td>
</tr>
</table>

If the system keyring is unavailable, the CLI falls back to storing the token in plain text in the configuration file at `~/.config/tc/config.yml`. To force plain text storage (for example, in headless environments), use the `--insecure-storage` flag:

```Shell
teamcity auth login --insecure-storage
```

> When using `--insecure-storage`, the token is saved as plain text in the config file. Protect this file with appropriate filesystem permissions.
>
{style="warning"}

## Environment variables
{id="auth-env-vars" help-id="auth-env-vars"}

For CI/CD pipelines and scripted environments, use environment variables instead of interactive login:

<tabs>
<tab title="macOS and Linux">

```Shell
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_TOKEN="your-access-token"
```

</tab>
<tab title="Windows">

PowerShell:

```PowerShell
$env:TEAMCITY_URL = "https://teamcity.example.com"
$env:TEAMCITY_TOKEN = "your-access-token"
```

CMD:

```Shell
set TEAMCITY_URL=https://teamcity.example.com
set TEAMCITY_TOKEN=your-access-token
```

</tab>
</tabs>

For guest access:

<tabs>
<tab title="macOS and Linux">

```Shell
export TEAMCITY_URL="https://teamcity.example.com"
export TEAMCITY_GUEST=1
```

</tab>
<tab title="Windows">

PowerShell:

```PowerShell
$env:TEAMCITY_URL = "https://teamcity.example.com"
$env:TEAMCITY_GUEST = "1"
```

CMD:

```Shell
set TEAMCITY_URL=https://teamcity.example.com
set TEAMCITY_GUEST=1
```

</tab>
</tabs>

Environment variables take precedence over the configuration file and keyring.

> To restrict the CLI to read-only operations (no builds triggered, no data modified), set `TEAMCITY_RO=1`. See [Read-only mode](teamcity-cli-scripting.md#Read-only+mode).
>
{style="tip"}

> Do not pass tokens as command-line flags in scripts — they may appear in process listings and shell history. Use environment variables instead.
>
{style="warning"}

## Advanced authentication scenarios

### Authentication inside TeamCity builds

When `teamcity` runs inside a TeamCity build, it automatically authenticates using build-level credentials from the build properties file. No additional configuration is required.

This allows you to use `teamcity` commands in build steps without storing or managing tokens:

```Shell
# Inside a TeamCity build step — no auth setup needed
teamcity run list --job MyProject_Build --limit 5
```

### Multiple servers

You can authenticate with several TeamCity servers. Each server's credentials are stored separately.

#### Adding servers

```Shell
# First server
teamcity auth login --server https://teamcity-prod.example.com

# Additional server (becomes the new default)
teamcity auth login --server https://teamcity-staging.example.com
```

#### Switching between servers

There are several ways to target a specific server:

**Environment variable (recommended for scripts):**

<tabs>
<tab title="macOS and Linux">

```Shell
TEAMCITY_URL=https://teamcity-prod.example.com teamcity run list
```

</tab>
<tab title="Windows">

PowerShell:

```PowerShell
$env:TEAMCITY_URL = "https://teamcity-prod.example.com"
teamcity run list
```

CMD:

```Shell
set TEAMCITY_URL=https://teamcity-prod.example.com
teamcity run list
```

</tab>
</tabs>

**Export for your session:**

<tabs>
<tab title="macOS and Linux">

```Shell
export TEAMCITY_URL=https://teamcity-prod.example.com
teamcity run list    # uses teamcity-prod
teamcity auth status # shows teamcity-prod
```

</tab>
<tab title="Windows">

PowerShell:

```PowerShell
$env:TEAMCITY_URL = "https://teamcity-prod.example.com"
teamcity run list    # uses teamcity-prod
teamcity auth status # shows teamcity-prod
```

CMD:

```Shell
set TEAMCITY_URL=https://teamcity-prod.example.com
teamcity run list    # uses teamcity-prod
teamcity auth status # shows teamcity-prod
```

</tab>
</tabs>

**Log in again to change the default:**

```Shell
teamcity auth login --server https://teamcity-prod.example.com
```

#### Server auto-detection from Kotlin DSL

When working in a project with TeamCity versioned settings, the CLI can detect the server URL from the Kotlin DSL `pom.xml`. It searches for `.teamcity/` or `.tc/` directories in the current folder and its parents (or uses `TEAMCITY_DSL_DIR` if set), and extracts the server URL from the DSL plugins repository URL. This auto-detected server URL is used when `TEAMCITY_URL` is not set. You still need credentials for that server.

### Credential precedence

Server URL resolution order (highest priority first):

1. `TEAMCITY_URL` environment variable
2. Kotlin DSL auto-detection (`TEAMCITY_DSL_DIR`, `.teamcity/`, or `.tc/`)
3. `default_server` from `~/.config/tc/config.yml`

Authentication resolution order (highest priority first):

1. Guest authentication (`TEAMCITY_GUEST` or a server configured with guest access)
2. `TEAMCITY_TOKEN` environment variable
3. Stored token for the resolved server URL (system keyring first, then plain text config if `--insecure-storage` was used)
4. Build-level credentials when running inside a TeamCity build

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-configuration.md">Configuration</a>
        <a href="teamcity-cli-commands.md">Command reference</a>
    </category>
    <category ref="user-guide">
        <a href="teamcity-cli-scripting.md">Scripting and automation</a>
    </category>
</seealso>
