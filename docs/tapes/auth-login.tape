Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 500
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false

# Authenticate with guest access
Type "teamcity auth login -s https://cli.teamcity.com --guest"
Enter
Sleep 4s

# Verify authentication
Type "teamcity auth status"
Enter
Sleep 3s
