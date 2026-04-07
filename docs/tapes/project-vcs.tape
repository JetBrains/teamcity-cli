Output {{OUTPUT}}

Set Theme "{{THEME}}"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1200
Set Height 800
Set Padding 20
Set TypingSpeed 40ms
Set CursorBlink false
Env TEAMCITY_URL "https://cli.teamcity.com"
Env TEAMCITY_NO_UPDATE "1"

# List VCS roots in a project
Type "teamcity project vcs list --project CLI"
Enter
Sleep 4s

# View VCS root details
Type "teamcity project vcs view CLI_TeamCityCli"
Enter
Sleep 4s
