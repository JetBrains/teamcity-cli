Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 600
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false
Env TEAMCITY_URL "https://jetbrains-ai.internal.teamcity.cloud"

# List all agents
Type "teamcity agent list --limit 10"
Enter
Sleep 4s

# Filter by pool
Type "teamcity agent list --pool Default --connected --limit 5"
Enter
Sleep 3s
