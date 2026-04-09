Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 700
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false
Env TEAMCITY_URL "https://buildserver.labs.intellij.net"
Env TEAMCITY_NO_UPDATE "1"

Type "teamcity run tree 917474183"
Enter
Sleep 5s

Type "teamcity run tree 917474183 --depth 1"
Enter
Sleep 4s
