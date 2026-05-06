[//]: # (title: Anonymous usage statistics)

<show-structure for="chapter" depth="2"/>

<tip>

**Just want to opt out?** Set `DO_NOT_TRACK=1` in your shell, or run `teamcity config set analytics false`.

</tip>

TeamCity CLI sends anonymous usage data to JetBrains so we can see which commands matter to people, where things break, and where to spend our time next.

## What we collect

- The command you ran – for example, `run list`, `pipeline validate`, `agent term`.
- Whether you used common flags like `--json`, `--watch`, or `--web`.
- A salted hash of your TeamCity server URL, so we can count distinct servers without learning the URL.
- A per-machine session ID.
- OS, CLI version, terminal type, and whether the run was interactive or in CI.

## What we don't collect

- Tokens, passwords, SSH keys, or anything stored in the keyring or `config.yml`.
- Repository contents, build logs, or arguments you pass to commands.
- Project, job, agent, or pipeline names and IDs.
- Usernames, emails, hostnames, or IP addresses.

**Nothing we collect can be used to identify you, your organization, or your repositories.** Identifiers that could be sensitive – like the server URL – are hashed locally with a salt before they leave your machine, and we don't sell or share the data we receive.

## How to opt out

Pick whichever fits your workflow – checked in this order:

**Environment variable** (one-shot or per session):

```Shell
export DO_NOT_TRACK=1
# or, equivalently
export TEAMCITY_ANALYTICS=0
```

`DO_NOT_TRACK` follows the [industry convention](https://donottrack.sh/) used by other CLI tools. Either variable wins over the config file.

**Configuration** (persistent):

```Shell
teamcity config set analytics false
```

To re-enable later:

```Shell
teamcity config set analytics true
```

## Where the data goes

Through the JetBrains FUS (Feature Usage Statistics) pipeline – the same pipeline used by IntelliJ-based IDEs. Retention and processing details: [JetBrains Product Data Collection Terms](https://www.jetbrains.com/legal/docs/terms/product_data_collection/).

<seealso>
    <category ref="user-guide">
        <a href="teamcity-cli-configuration.md">Configuration</a>
    </category>
</seealso>
