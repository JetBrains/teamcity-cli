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

# List cloud images for a profile
Type "teamcity project cloud image list _Root --profile amazon-1"
Enter
Sleep 4s

# List running cloud instances
Type "teamcity project cloud instance list _Root --limit 10"
Enter
Sleep 4s
