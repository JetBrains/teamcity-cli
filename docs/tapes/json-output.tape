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

# Discover available fields
Type "teamcity run list --json="
Enter
Sleep 3s

# Select specific fields
Type "teamcity run list --json=id,status,buildType.name --limit 5"
Enter
Sleep 4s
