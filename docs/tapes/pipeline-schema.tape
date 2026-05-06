Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 500
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false
Env TEAMCITY_URL "https://buildserver.labs.intellij.net"

# Inspect available top-level definitions
Type "teamcity pipeline schema | jq -r '.definitions | keys[]'"
Enter
Sleep 4s

# Save schema for editor / LSP integration
Type "teamcity pipeline schema > .teamcity-schema.json && wc -l .teamcity-schema.json"
Enter
Sleep 3s
