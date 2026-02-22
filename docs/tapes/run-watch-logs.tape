Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 700
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false
Env TEAMCITY_URL "https://cli.teamcity.com"

# Watch a running build with live log streaming
Type "teamcity run watch {{RUN_ID}} --logs"
Enter
Sleep 20s
