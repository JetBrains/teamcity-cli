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

# Show full dependency chain from a leaf job
Type "teamcity job tree DeathStar_AssembleSuperlaser"
Enter
Sleep 5s

# Show only dependencies from the top
Type "teamcity job tree DeathStar_FullOperational --only dependencies"
Enter
Sleep 5s

# Show the full chain from a middle node
Type "teamcity job tree Sandbox_DeployStaging"
Enter
Sleep 5s
