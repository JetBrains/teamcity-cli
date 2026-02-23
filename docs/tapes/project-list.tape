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

# List all projects
Type "teamcity project list"
Enter
Sleep 4s

# Filter by parent project
Type "teamcity project list --parent _Root"
Enter
Sleep 3s
