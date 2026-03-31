package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want InstallMethod
	}{
		{"homebrew_cellar", "/opt/homebrew/Cellar/teamcity/0.7.0/bin/teamcity", InstallHomebrew},
		{"homebrew_linuxbrew", "/home/user/.linuxbrew/homebrew/bin/teamcity", InstallHomebrew},
		{"scoop", `C:\Users\user\scoop\apps\teamcity\current\teamcity.exe`, InstallScoop},
		{"chocolatey", `C:\ProgramData\chocolatey\bin\teamcity.exe`, InstallChocolatey},
		{"winget", `C:\Users\user\AppData\Local\Microsoft\WindowsApps\teamcity.exe`, InstallWinGet},
		{"npm", "/usr/local/lib/node_modules/@jetbrains/teamcity-cli/bin/teamcity", InstallNPM},
		{"go_install", "/Users/user/go/bin/teamcity", InstallGoInstall},
		{"install_script", "/usr/local/bin/teamcity", InstallScript},
		{"install_script_local", "/home/user/.local/bin/teamcity", InstallScript},
		{"unknown", "/some/random/path/teamcity", InstallUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, detectFromPath(tt.path))
		})
	}
}
