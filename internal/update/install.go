package update

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type InstallMethod int

const (
	InstallUnknown InstallMethod = iota
	InstallHomebrew
	InstallApt
	InstallRPM
	InstallArch
	InstallScoop
	InstallChocolatey
	InstallWinGet
	InstallNPM
	InstallGoInstall
	InstallScript
)

func DetectInstallMethod() InstallMethod {
	exe, err := os.Executable()
	if err != nil {
		return InstallUnknown
	}
	return detectFromPath(exe)
}

func detectFromPath(exe string) InstallMethod {
	exeLower := strings.ToLower(exe)

	if strings.Contains(exeLower, "/cellar/") || strings.Contains(exeLower, "/homebrew/") {
		return InstallHomebrew
	}
	if strings.Contains(exeLower, `\scoop\`) {
		return InstallScoop
	}
	if strings.Contains(exeLower, `\chocolatey\`) {
		return InstallChocolatey
	}
	if strings.Contains(exeLower, `\windowsapps\`) || strings.Contains(exeLower, `\winget\`) {
		return InstallWinGet
	}
	if strings.Contains(exeLower, "node_modules") {
		return InstallNPM
	}
	if isGoBin(exeLower) {
		return InstallGoInstall
	}

	if runtime.GOOS == "linux" {
		if cmdExists("dpkg") && isInstalledVia(exe, "dpkg", "-S") {
			return InstallApt
		}
		if cmdExists("rpm") && isInstalledVia(exe, "rpm", "-qf") {
			return InstallRPM
		}
		if cmdExists("pacman") && isInstalledVia(exe, "pacman", "-Qo") {
			return InstallArch
		}
	}

	if exeLower == "/usr/local/bin/teamcity" || strings.HasSuffix(exeLower, "/.local/bin/teamcity") {
		return InstallScript
	}

	return InstallUnknown
}

func isGoBin(exe string) bool {
	if strings.Contains(exe, "/go/bin/") || strings.Contains(exe, `\go\bin\`) {
		return true
	}
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		return strings.HasPrefix(strings.ToLower(exe), strings.ToLower(gopath))
	}
	return false
}

func cmdExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func isInstalledVia(exe string, cmd string, args ...string) bool {
	out, err := exec.Command(cmd, append(args, exe)...).CombinedOutput()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func (m InstallMethod) UpdateCommand() string {
	switch m {
	case InstallHomebrew:
		return "brew upgrade teamcity"
	case InstallApt:
		return "Visit https://github.com/JetBrains/teamcity-cli/releases/latest to download the latest .deb"
	case InstallRPM:
		return "Visit https://github.com/JetBrains/teamcity-cli/releases/latest to download the latest .rpm"
	case InstallArch:
		return "yay -Syu teamcity-bin"
	case InstallScoop:
		return "scoop update teamcity"
	case InstallChocolatey:
		return "choco upgrade TeamCityCLI"
	case InstallWinGet:
		return "winget upgrade JetBrains.TeamCityCLI"
	case InstallNPM:
		return "npm update -g @jetbrains/teamcity-cli"
	case InstallGoInstall:
		return "go install github.com/JetBrains/teamcity-cli/tc@latest"
	case InstallScript:
		if runtime.GOOS == "windows" {
			return `irm https://jb.gg/tc/install.ps1 | iex`
		}
		return "curl -fsSL https://jb.gg/tc/install | bash"
	default:
		return "Visit https://github.com/JetBrains/teamcity-cli/releases/latest"
	}
}

func (m InstallMethod) String() string {
	switch m {
	case InstallHomebrew:
		return "Homebrew"
	case InstallApt:
		return "apt/deb"
	case InstallRPM:
		return "rpm"
	case InstallArch:
		return "AUR"
	case InstallScoop:
		return "Scoop"
	case InstallChocolatey:
		return "Chocolatey"
	case InstallWinGet:
		return "WinGet"
	case InstallNPM:
		return "npm"
	case InstallGoInstall:
		return "go install"
	case InstallScript:
		return "install script"
	default:
		return "unknown"
	}
}
