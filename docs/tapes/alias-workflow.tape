Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 600
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false
Env TEAMCITY_URL "https://cli.teamcity.com"
Env TEAMCITY_GUEST "1"

# Create a few useful aliases
Type "teamcity alias set rl 'run list'"
Enter
Sleep 2s

Type "teamcity alias set fails 'run list --status=failure --since=24h'"
Enter
Sleep 2s

Type "teamcity alias set go 'run start $1 --watch'"
Enter
Sleep 2s

# List all aliases
Type "teamcity alias list"
Enter
Sleep 3s

# Use an alias
Type "teamcity rl --limit 5"
Enter
Sleep 4s
