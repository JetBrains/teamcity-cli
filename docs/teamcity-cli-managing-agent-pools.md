[//]: # (title: Managing Agent Pools)

<show-structure for="chapter" depth="2"/>

Agent pools group build agents and control which projects can use them. The `tc pool` command group lets you list pools, view pool details, and manage project-pool associations.

## Listing pools

View all configured agent pools:

```Shell
tc pool list
```

Output as JSON:

```Shell
tc pool list --json
tc pool list --json=id,name,maxAgents
```

## Viewing pool details

View details of a specific pool:

```Shell
tc pool view 0
tc pool view 1 --web
tc pool view 1 --json
```

## Linking projects to pools

Link a project to an agent pool, allowing the project's builds to run on agents in that pool:

```Shell
tc pool link 1 MyProject
```

The first argument is the pool ID, and the second is the project ID.

## Unlinking projects from pools

Remove a project's access to agents in a pool:

```Shell
tc pool unlink 1 MyProject
```

> Unlinking a project from a pool means builds from that project can no longer run on agents in the pool. Builds that are already running are not affected.
>
{style="note"}

<seealso>
    <category ref="reference">
        <a href="teamcity-cli-commands.md">Command reference</a>
        <a href="teamcity-cli-managing-agents.md">Managing agents</a>
    </category>
</seealso>
