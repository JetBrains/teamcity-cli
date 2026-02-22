[//]: # (title: Glossary)

<show-structure for="chapter" depth="2"/>

This glossary defines the key terms used in TeamCity CLI and maps them to their TeamCity equivalents.

## Run

A single execution of a build configuration. Equivalent to **build** in the TeamCity web interface.

```Shell
teamcity run list
teamcity run start MyProject_Build
```

## Job

A set of instructions for running a build. Equivalent to **build configuration** in the TeamCity web interface.

```Shell
teamcity job list
teamcity job view MyProject_Build
```

## Project

A collection of build configurations (jobs) with an associated name and description. Projects can be nested to form a hierarchy.

```Shell
teamcity project list
teamcity project view MyProject
```

## Build Queue

A list of builds waiting to be assigned to an available agent. Builds in the queue are distributed to compatible agents as resources become available. The queue can be reordered manually.

```Shell
teamcity queue list
teamcity queue top 12345
```

## Build Agent

A piece of software that executes builds. Agents are installed on separate machines and are assigned builds based on their compatibility with build configurations.

```Shell
teamcity agent list
teamcity agent view Agent-Linux-01
```

## Agent Pool

A named group of build agents. Projects can be linked to pools to control which agents run their builds.

```Shell
teamcity pool list
teamcity pool view 1
```

## Change

A modification to source code detected by TeamCity. A change is considered pending when it has been committed to the VCS but not yet included in a build.

```Shell
teamcity run changes 12345
```

## Personal Build

A build initiated by a developer to test local changes without affecting the main build history. In TeamCity CLI, personal builds are triggered using the `--local-changes` flag.

```Shell
teamcity run start MyProject_Build --local-changes
```

## Alias

A custom command shortcut that expands into a full `teamcity` command. Aliases support positional arguments and shell expressions.

```Shell
teamcity alias set rl 'run list'
teamcity alias list
```

## Skill

A bundled configuration file that teaches AI coding agents (Claude Code, Cursor, and others) how to use TeamCity CLI. Skills follow the [Agent Skills specification](https://agentskills.io).

```Shell
teamcity skill install
teamcity skill update
```

## Terminology mapping

<table>
<tr>
<td>

CLI term

</td>
<td>

TeamCity term

</td>
</tr>
<tr>
<td>

`run`

</td>
<td>

build

</td>
</tr>
<tr>
<td>

`job`

</td>
<td>

build configuration

</td>
</tr>
<tr>
<td>

`project`

</td>
<td>

project

</td>
</tr>
<tr>
<td>

`queue`

</td>
<td>

build queue

</td>
</tr>
<tr>
<td>

`agent`

</td>
<td>

build agent

</td>
</tr>
<tr>
<td>

`pool`

</td>
<td>

agent pool

</td>
</tr>
</table>
