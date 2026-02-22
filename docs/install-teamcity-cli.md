[//]: # (title: Installing TeamCity CLI)

<show-structure for="chapter" depth="2"/>

This page describes how to install TeamCity CLI on macOS, Linux, and Windows.

## Prerequisites

TeamCity CLI requires a running TeamCity server (version 2020.1 or later) to connect to. Some features may require newer TeamCity versions (for example, 2024.04 or later). No additional runtime dependencies are needed — the CLI is distributed as a standalone binary.

## macOS and Linux

### Homebrew

Homebrew is the recommended installation method on macOS and Linux:

```Shell
brew install jetbrains/utils/teamcity
```

To update to the latest version:

```Shell
brew upgrade teamcity
```

### Install script

Download and run the install script:

```Shell
curl -fsSL https://jb.gg/tc/install | bash
```

The script detects your operating system and architecture automatically and installs the `teamcity` binary to a directory on your PATH.

### Debian and Ubuntu

Download and install the `.deb` package:

```Shell
curl -fsSLO https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.deb
sudo dpkg -i teamcity_linux_amd64.deb
```

### RHEL and Fedora

Install directly from the `.rpm` package:

```Shell
sudo rpm -i https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.rpm
```

### Arch Linux

Download and install the `.pkg.tar.zst` package:

```Shell
curl -fsSLO https://github.com/JetBrains/teamcity-cli/releases/latest/download/teamcity_linux_amd64.pkg.tar.zst
sudo pacman -U teamcity_linux_amd64.pkg.tar.zst
```

## Windows

### PowerShell

Download and run the install script in PowerShell:

```Shell
irm https://jb.gg/tc/install.ps1 | iex
```

### CMD

Download and run the install script in CMD:

```Shell
curl -fsSL https://jb.gg/tc/install.cmd -o install.cmd && install.cmd && del install.cmd
```

### Scoop

```Shell
scoop bucket add jetbrains https://github.com/JetBrains/scoop-utils
scoop install teamcity
```

## Go

If you have Go installed, you can install the CLI directly:

```Shell
go install github.com/JetBrains/teamcity-cli/tc@latest
```

This installs the `teamcity` binary to your `$GOPATH/bin` directory.

## Build from source

Clone the repository and build:

```Shell
git clone https://github.com/JetBrains/teamcity-cli.git
cd teamcity-cli
go build -o teamcity ./tc
```

The compiled binary is created in the current directory. Move it to a location on your PATH to use it globally.

## Verify the installation

After installing, verify that the CLI is available:

```Shell
teamcity --version
```

## Next steps

After installing TeamCity CLI, follow the [quickstart guide](get-started-with-teamcity-cli.md) to authenticate with your TeamCity server and run your first commands.

<seealso>
    <category ref="installation">
        <a href="get-started-with-teamcity-cli.md">Getting started with TeamCity CLI</a>
    </category>
    <category ref="reference">
        <a href="teamcity-cli-configuration.md">Configuration</a>
    </category>
</seealso>
