# Managing Agents & Agent Pools

## Managing Agents

**List all agents:**
```bash
teamcity agent list
```

**List connected agents only:**
```bash
teamcity agent list --connected
```

**Filter agents by pool:**
```bash
teamcity agent list --pool Default
```

**View agent details:**
```bash
teamcity agent view <agent-id>
```

**See what jobs an agent can run:**
```bash
teamcity agent jobs <agent-id>
```

**See why jobs are incompatible with an agent:**
```bash
teamcity agent jobs <agent-id> --incompatible
```

**Enable/disable an agent:**
```bash
teamcity agent enable <agent-id>
teamcity agent disable <agent-id>
```

**Authorize/deauthorize an agent:**
```bash
teamcity agent authorize <agent-id>
teamcity agent deauthorize <agent-id>
```

**Move agent to a different pool:**
```bash
teamcity agent move <agent-id> <pool-id>
```

**Reboot an agent:**
```bash
teamcity agent reboot <agent-id>
```

**Reboot after current build finishes:**
```bash
teamcity agent reboot <agent-id> --graceful
```

## Remote Agent Access

**Open interactive shell on an agent:**
```bash
teamcity agent term <agent-id>
```

**Execute a command on an agent:**
```bash
teamcity agent exec <agent-id> "ls -la"
```

**Execute with timeout:**
```bash
teamcity agent exec <agent-id> --timeout 10m -- long-running-script.sh
```

## Managing Agent Pools

**List all pools:**
```bash
teamcity pool list
```

**View pool details:**
```bash
teamcity pool view <pool-id>
```

**Link a project to a pool:**
```bash
teamcity pool link <pool-id> <project-id>
```

**Unlink a project from a pool:**
```bash
teamcity pool unlink <pool-id> <project-id>
```
