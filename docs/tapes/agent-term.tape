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

# Open interactive terminal to an agent
Type "teamcity agent term 107418"
Enter
Sleep 4s

# Run commands on the remote agent
Type "hostname"
Enter
Sleep 2s

Type "uname -a"
Enter
Sleep 2s

Type "cat /etc/os-release | head -5"
Enter
Sleep 2s

# Exit the session
Type "exit"
Enter
Sleep 2s
