Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 500
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false
Env TEAMCITY_URL "https://jetbrains-ai.internal.teamcity.cloud"

# List cloud profiles
Type "teamcity project cloud profile list _Root"
Enter
Sleep 4s

# View profile details
Type "teamcity project cloud profile view amazon-1"
Enter
Sleep 3s
