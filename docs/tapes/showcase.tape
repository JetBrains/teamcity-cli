Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 700
Set Padding 20
Set TypingSpeed 30ms
Set CursorBlink false
Env TEAMCITY_URL "https://cli.teamcity.com"
Env TEAMCITY_GUEST "1"

# Show the CLI welcome
Type "teamcity"
Enter
Sleep 3s

# List recent builds
Type "teamcity run list --limit 5"
Enter
Sleep 3s

# View build details
Type "teamcity run view 2016"
Enter
Sleep 3s

# Check the build queue
Type "teamcity queue list"
Enter
Sleep 3s

# JSON output for scripting
Type "teamcity run list --json=id,status,buildType.name --limit 3"
Enter
Sleep 3s

# Silently switch to internal server for agents
Hide
Type "export TEAMCITY_URL=https://jetbrains-ai.internal.teamcity.cloud"
Enter
Type "unset TEAMCITY_GUEST"
Enter
Sleep 1s
Show

# List connected agents
Type "teamcity agent list --connected --limit 5"
Enter
Sleep 3s

# Execute a remote command on an agent
Type "teamcity agent exec 107418 'hostname'"
Enter
Sleep 4s
