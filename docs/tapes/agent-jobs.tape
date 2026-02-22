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

# List compatible jobs for an agent
Type "teamcity agent jobs 107418"
Enter
Sleep 4s

# Show incompatible jobs with reasons
Type "teamcity agent jobs 107418 --incompatible"
Enter
Sleep 4s
