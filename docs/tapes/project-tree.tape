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

# Show full project hierarchy
Type "teamcity project tree"
Enter
Sleep 5s

# Show specific project
Type "teamcity project tree Sandbox"
Enter
Sleep 3s

# Hide jobs
Type "teamcity project tree CLI --no-jobs"
Enter
Sleep 3s
