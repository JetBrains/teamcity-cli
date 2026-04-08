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
Env TEAMCITY_NO_UPDATE "1"

# List pipelines
Type "teamcity pipeline list --project CLI"
Enter
Sleep 3s

# View pipeline details
Type "teamcity pipeline view CLI_Demo"
Enter
Sleep 3s
