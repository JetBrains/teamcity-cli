Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 400
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false
Env TEAMCITY_URL "https://jetbrains-ai.internal.teamcity.cloud"

# List all agent pools
Type "teamcity pool list"
Enter
Sleep 3s
