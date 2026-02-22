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

# Execute a remote command on an agent
Type "teamcity agent exec 107418 'uname -a'"
Enter
Sleep 5s

Type "teamcity agent exec 107418 'df -h'"
Enter
Sleep 5s
